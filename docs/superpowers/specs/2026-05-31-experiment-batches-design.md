# Experiment batches: launch many tasks together, group them in the list

**Date:** 2026-05-31
**Scope:** Backend (engine + schema) + frontend (Diagnostic launcher + Experiments list). No grader changes.

## Goal

When a user kicks off "the failure-mode calibration suite" from the
Diagnostic launcher (or from a CLI script), all the spawned
experiments share a single batch identity, and the Experiments list
renders them as one collapsible group instead of N indistinguishable
rows scattered down the page.

## Motivation

Today the Diagnostic launcher takes one `task_id` per POST. Suite
runs are done via shell scripts (`/tmp/run-fmt-suite.sh`) that loop
over tasks and call the endpoint N times. The result is N orphan
experiments with no relationship in the data model — the
Experiments list shows them as 5 visually identical rows. The user
just merged the 5-task failure-mode corpus and immediately ran into
this scanning problem ("FMT calibration: …" × 5 → noise).

A batch identity at the data layer fixes both problems:
- The Experiments list can group by `batch_id`
- CLI scripts can pre-mint a batch UUID and tag all calls in the
  loop, getting the same grouping behavior

## Out of scope

- Grader changes
- Cross-task aggregation metrics ("how did harness X do across all 5
  tasks in this batch?") — separate work. Batch groups in this PR
  are organizational only.
- Per-batch status rollup beyond a simple count (e.g., "3/5
  completed") — anything richer is later.
- Re-running a whole batch with one click. Listed batches are
  view-only.
- Backfilling `batch_id` for the existing 80+ orphan experiments.
  Migration adds the column NULL-able; pre-existing rows stay
  ungrouped. The user can delete them or live with them.

---

## Architecture

Three layers touched in lockstep:

```
SQLite migration 019  →  Go models + repo + API  →  React launcher + list
```

Storage adds two columns (`batch_id`, `batch_label`). The launch
endpoints accept an optional batch identity from the caller (no
server-side mint to keep round-trips small and let CLI scripts own
the UUID). A new endpoint `POST /api/diagnostic/launch-suite` is the
thin sugar that creates N experiments in one POST sharing a single
server-generated `batch_id`. The frontend launcher's task field
becomes a multi-select: 1 selection → existing single endpoint,
≥2 → suite endpoint.

The Experiments list groups rows whose `batch_id` is set and shared
with at least one other row. Solo rows (no batch_id or unique
batch_id) render flat as today.

---

## Detailed design

### 1. Schema migration

New file: `engine/internal/storage/migrations/019_experiment_batches.sql`

```sql
ALTER TABLE experiments ADD COLUMN batch_id TEXT;
ALTER TABLE experiments ADD COLUMN batch_label TEXT;

-- Group lookups in the list (and any future aggregation queries) hit
-- this column a lot. Keep the index narrow: just batch_id, since
-- batch_label is display-only and never filtered on.
CREATE INDEX IF NOT EXISTS idx_experiments_batch_id ON experiments(batch_id);
```

Migration is forward-only. SQLite can ADD COLUMN without rewriting
the table, so the migration is fast even on the existing local DB.

### 2. Go model + repo

`engine/internal/models/experiment.go`:

```go
type Experiment struct {
    // …existing fields…
    BatchID    string `json:"batch_id,omitempty"`
    BatchLabel string `json:"batch_label,omitempty"`
}
```

`engine/internal/models/experiment.go` (request):

```go
type ExperimentRequest struct {
    // …existing fields…
    BatchID    string `json:"batch_id,omitempty"`
    BatchLabel string `json:"batch_label,omitempty"`
}
```

Storage repo updates:
- `engine/internal/storage/experiment_repo.go` — `INSERT` includes
  the two new columns; `SELECT` includes them in every projection
  that returns `Experiment` (list, get-by-id).
- Scan helpers updated to handle NULL → empty string the same way
  the existing `description`/`local_path` fields do.

### 3. Existing launch endpoint accepts batch identity

`engine/internal/api/diagnostic_launch.go`:

```go
type LaunchDiagnosticRequest struct {
    // …existing fields…
    BatchID    string `json:"batch_id,omitempty"`
    BatchLabel string `json:"batch_label,omitempty"`
}
```

Pass-through to `models.ExperimentRequest`. No server-side mint; if
the caller (CLI script or frontend single-task launch) leaves both
empty, the experiment stays unbatched (NULL columns).

### 4. New suite-launch endpoint

`engine/internal/api/diagnostic_launch.go` adds:

```go
type LaunchDiagnosticSuiteRequest struct {
    TaskIDs        []string `json:"task_ids"`
    ExecutorID     string   `json:"executor_id"`
    HarnessIDs     []string `json:"harness_ids"`
    Model          string   `json:"model"`
    RunsPerVariant int      `json:"runs_per_variant"`
    TimeoutSeconds int      `json:"timeout_seconds"`
    BatchLabel     string   `json:"batch_label"` // display name for the group
}

type LaunchDiagnosticSuiteResponse struct {
    BatchID       string   `json:"batch_id"`
    ExperimentIDs []string `json:"experiment_ids"`
    Failures      []SuiteLaunchFailure `json:"failures,omitempty"`
}

type SuiteLaunchFailure struct {
    TaskID  string `json:"task_id"`
    Message string `json:"message"`
}
```

Handler:
1. Validates `TaskIDs` non-empty, all task IDs exist, exactly one
   executor, ≥1 harness, model present.
2. Mints `batch_id = uuid.NewString()`.
3. Loops over `TaskIDs`. For each task: builds the same
   `ExperimentRequest` the single-task path uses, with the shared
   `batch_id` + `batch_label`, sequential per-task name
   (`"<batch_label> · <task.Name>"` if `batch_label` non-empty,
   else `"Diagnostic suite · <task.Name>"`).
4. `CreateExperiment` + `StartExperiment` for each, accumulating
   IDs and any per-task failures into the response (one bad task
   doesn't abort the others — the response lets the UI show
   partial-success state).
5. Returns 202 with the batch ID + the IDs that were successfully
   started + any failure rows. Empty `Failures` means full success.

The handler is wired in `engine/internal/api/router.go`:

```go
r.Post("/diagnostic/launch-suite", service.LaunchDiagnosticSuite)
```

### 5. Frontend — launcher: multi-task picker

`frontend/src/pages/diagnostic/launch.tsx`:

- Replace the single `taskID` state with `taskIDs: string[]`.
- The "Task" section becomes a multi-select chip list (same visual
  pattern as the existing harness/executor chip selectors on the
  same page — consistency over inventiveness).
- A small "Suite label" text input appears below the task list once
  ≥2 tasks are selected, defaulting to `"Calibration suite · YYYY-MM-DD HH:MM"`.
- `useLaunchDiagnostic` hook stays (single-task path). New
  `useLaunchDiagnosticSuite` hook hits the new endpoint.
- Launch handler:
  - If `taskIDs.length === 1`: call `useLaunchDiagnostic` as today,
    pass no batch fields. Redirect to `/diagnostic/compare?experiment=<id>`.
  - If `taskIDs.length >= 2`: call `useLaunchDiagnosticSuite`,
    redirect to `/experiments?batch=<batch_id>` (list page can scroll
    to and highlight the group via the query param).
- Variant counter at the bottom of the form already shows
  `harness × executor × model × runs`; now it multiplies by
  `taskIDs.length` and labels itself "experiments to launch".

### 6. Frontend — Experiments list: group by batch_id

`frontend/src/pages/experiments/index.tsx`:

- Replace the single `filtered` array with a `grouped` structure:

  ```ts
  type GroupedExperiment =
    | { kind: 'solo'; experiment: Experiment }
    | { kind: 'group'; batchId: string; batchLabel: string; experiments: Experiment[] };
  ```

  Build `grouped` by walking `filtered` once: bucket by `batch_id`
  (when present and shared by ≥2 visible experiments), emit solo
  rows otherwise. Preserve created-at descending sort: a group's
  position is the most-recent member's created_at.

- Render each group as:
  - A header row (`<tr>` styled differently — `bg-bg-elev-2`,
    chevron indicator, monospace `batch_label` or first 8 chars of
    `batch_id` if no label, plus `{N} experiments · {summary}`
    where summary is `"3/5 completed"` or `"2 running · 3 queued"`.
  - Below it: the N child rows, indented (`pl-8`) and with
    `bg-bg-elev-1/60` to read as nested.

- Default expand state: collapsed when the group has >3 children,
  expanded otherwise. Stored in `useState<Record<string, boolean>>`
  inside the page component (no persistence — resets on reload).

- If the URL has `?batch=<id>` (from suite launch redirect): force
  that batch's group to expanded on mount and scroll into view.

- Remove the in-table "New diagnostic run" button from the filter
  row (user request — the sidebar's Run diagnostic link is the
  canonical entry). Keep the empty-state CTA.

### 7. UI polish (small, scoped)

- `runsLabel` in the Experiments table: when `variants.length === 0`
  AND the experiment has variants representing harnesses (from
  diagnostic launcher), show `"{N} run · {harnessNames.join(', ')}"`
  instead of `"0v × Nr"`. When variants > 0, keep `"{V}v × {N}r"`.

- Status badge tones: bump `completed` (more saturated green),
  `running` (cyan with subtle pulse — pure CSS, no JS), `draft`
  (dimmer). Use existing Tailwind tokens — no new tokens.

- Group header indentation rhythm: child rows indent with `pl-6` so
  the nesting is visible at a glance.

- Filter bar gets `sticky top-0 z-10` so it stays visible when
  scrolling a long batch — small but high-value change.

---

## Data flow

```
[User picks 5 tasks in launcher]
    ↓
POST /api/diagnostic/launch-suite { task_ids: [...], batch_label: "..." }
    ↓
[engine] mints batch_id = UUID; loops over task_ids:
    for each → CreateExperiment(batch_id, batch_label) + StartExperiment
    ↓
[engine] returns { batch_id, experiment_ids: [...], failures: [] }
    ↓
[frontend] redirects to /experiments?batch=<batch_id>
    ↓
[Experiments list] groups by batch_id, expands and scrolls to the new group
```

CLI scripts can do the same thing manually:

```bash
BATCH=$(uuidgen)
LABEL="FMT calibration v6"
for tid in "${TASKS[@]}"; do
  curl -X POST .../api/diagnostic/launch \
    -d "{\"task_id\":\"$tid\", \"batch_id\":\"$BATCH\", \"batch_label\":\"$LABEL\", ...}"
done
```

…and they land in the UI as one group automatically.

---

## Error handling

- Schema migration: forward-only, idempotent (`ADD COLUMN` + `IF NOT EXISTS` index). No rollback path needed for the local SQLite use case.
- Suite endpoint per-task failure: one task failing CreateExperiment or StartExperiment doesn't abort the rest. The response's `failures` array carries `{task_id, message}` for each. Frontend shows a soft warning toast like `"Started 4/5 — 1 failed: <task> (msg)"` and still redirects to the batch group.
- Existing single-launch endpoint: behavior unchanged if caller omits batch fields. If both `batch_id` and `batch_label` are sent, both persist. If only one is sent (e.g. label without ID), it persists as-is — no validation, since the column is informational.
- Frontend list: a `batch_id` referenced by only one visible experiment (after filter) renders as a solo row, not a single-item group. Avoids visual noise when filter narrows a 5-experiment batch down to 1 visible row.

---

## Testing

**Backend (Go, table-driven, stdlib testing per CLAUDE.md):**

- `engine/internal/storage/experiment_repo_test.go`:
  - `TestCreateExperimentPersistsBatchFields` — create with non-empty `batch_id`/`batch_label`, fetch back, assert round-trip.
  - `TestCreateExperimentNullBatchFields` — create without batch fields, fetch back, assert empty strings (NULL handling).
  - `TestListExperimentsByBatch` (only if a helper is added; otherwise skip — frontend filters list-side).

- `engine/internal/api/diagnostic_launch_test.go`:
  - `TestLaunchDiagnosticSuiteHappyPath` — 3 task IDs, valid harnesses, returns 3 experiments + 0 failures + shared batch_id.
  - `TestLaunchDiagnosticSuitePartialFailure` — 3 task IDs where the middle one is unknown, returns 2 successful IDs + 1 failure row, batch_id still shared.
  - `TestLaunchDiagnosticSuiteRejectsEmptyTaskIDs` — `task_ids: []` → 400.
  - `TestLaunchDiagnosticAcceptsBatchPassThrough` — existing endpoint with `batch_id` + `batch_label` fields, persists them.

**Frontend (Vitest):**

- `frontend/src/pages/experiments/grouping.test.ts` (extract pure grouping function `groupByBatch(experiments)` so it's testable without rendering):
  - Two experiments with same `batch_id` → one group of two.
  - One experiment with `batch_id`, no other matches → solo.
  - Experiment with no `batch_id` → solo.
  - Mixed: 5 total, 3 share batch A, 1 shares batch B with itself only (solo), 1 has no batch → 1 group of 3 + 2 solos. Verify sort order (group position = max created_at of members).

- No need to add a launcher-page test (high UI churn, low ROI). The end-to-end smoke test (existing root-redirect test) already covers that the page loads.

**Manual:**

- Pick 1 task → launch → land on Compare (same as today).
- Pick 3 tasks → launch → land on Experiments with `?batch=<id>`, group expanded, scrolled into view.
- Trigger a partial failure (e.g. pick 3 tasks, kill one's backend by editing the row to invalid). Verify the soft toast and that the successful ones still grouped.

---

## Risks and mitigations

| Risk | Mitigation |
|---|---|
| Migration locks DB on a populated table | SQLite `ADD COLUMN` is O(1) metadata-only; index creation on an existing empty column is also fast. Tested on local 80-experiment DB during implementation. |
| Existing experiments stay ungrouped forever | Acceptable. Out of scope. Users can delete or live with them. |
| Suite endpoint creates orphan experiments if mid-loop crash | Each `CreateExperiment` is independent; failures are reported per-task. Up to the caller to delete partial-success orphans if they want. |
| Multi-select task picker too dense visually | Reuse the existing chip-list pattern from harness/executor selectors. If 80+ tasks ever land in the picker, add a search input (deferred — current task library is ~10 tasks). |
| `?batch=<id>` query param collides with future routing | `useSearchParams` strips it on consumption; never persisted in state. No conflict. |
| Group header summary ("3/5 completed") needs status counts per group | Computed client-side from the children — O(N) per group, where N ≤ ~10. Negligible. |

---

## Open questions

None remaining — multi-task picker scope confirmed during brainstorming.
