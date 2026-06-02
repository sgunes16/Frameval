# Spec-kit extension as intra-experiment variant (matrix semantic rework)

**Date:** 2026-06-02
**Scope:** Backend launch endpoint + frontend launcher + Compare + Experiments list. No schema migration (per-variant config already persisted since PR #145).

## Problem

The Diagnostic launcher currently treats spec-kit extension as a **cross-experiment** matrix axis: `task × executor × model × speckit-extension`. Each cell becomes its own experiment, and every experiment gets *all* selected harnesses as variants.

Consequence: selecting 1 task × 1 executor × 1 model × 6 spec-kit extensions + 4 non-speckit harnesses produces **6 experiments × 5 variants = 30 runs** — and `bare`, `ralph`, `agent_instructions`, `multiagent` each run 6 pointless times (once per extension cell) even though extensions are meaningless to them.

The user's mental model — and the correct one for the thesis — is: "I picked 6 spec-kit extensions and 4 other harnesses; that's **10 runs**, one per variant, in **one experiment**." Spec-kit extensions are *variants of the spec-kit harness*, not separate experiments. We do not cross non-speckit harnesses with extensions.

## Goal

Move spec-kit extension from a cross-experiment axis to an **intra-experiment variant** dimension:

- **Cross-experiment axes** (each combination → one experiment, batched): `task × executor × model`
- **Intra-experiment variants** (within each experiment):
  - each selected non-speckit harness → exactly 1 variant
  - the spec-kit harness, if selected → 1 variant per selected extension

So `1 task × 1 exec × 1 model` with `{bare, agent_instructions, multiagent, ralph}` + spec-kit extensions `{canonical, lite, tdd-first, research-first, rigorous, dual-role}` = **1 experiment, 10 variants, 10 runs**.

## Why this is small

Per-variant harness config already flows end to end:

- `models.VariantRequest` carries `HarnessConfig map[string]any` (PR #145).
- `CreateExperiment` persists each variant's config to `variants.harness_config_json` (PR #145).
- The orchestrator fetches each variant fresh (`GetVariant`, orchestrator.go:186) and passes `variant.HarnessConfig` to `harness.Setup` (orchestrator.go:227).

The **only** thing forcing every variant to share one config is the launch handler building all `VariantRequest`s from the single `req.HarnessConfigs` map (diagnostic_launch.go:89-99). The rework gives each variant its own config.

## Out of scope

- `/diagnostic/launch-suite` endpoint (dead from the UI; left untouched, still harness_ids-based).
- CLI calibration scripts that POST `harness_ids` (kept working via additive back-compat — see below).
- Any change to how a single variant's run executes, grades, or is inspected.
- Per-variant executor/model overrides (executor & model remain experiment-level / cross-experiment axes).

## Architecture

### 1. Backend — additive `variants` array on the launch request

`engine/internal/api/diagnostic_launch.go`:

```go
type LaunchVariant struct {
    HarnessID     string         `json:"harness_id"`
    Name          string         `json:"name"`          // display label, e.g. "speckit/canonical"
    HarnessConfig map[string]any `json:"harness_config,omitempty"`
}

type LaunchDiagnosticRequest struct {
    TaskID         string          `json:"task_id"`
    ExecutorID     string          `json:"executor_id"`
    HarnessIDs     []string        `json:"harness_ids"`      // legacy path (CLI); used only when Variants is empty
    Model          string          `json:"model"`
    RunsPerVariant int             `json:"runs_per_variant"`
    TimeoutSeconds int             `json:"timeout_seconds"`
    Name           string          `json:"name"`
    BatchID        string          `json:"batch_id"`
    BatchLabel     string          `json:"batch_label"`
    HarnessConfigs map[string]any  `json:"harness_configs,omitempty"` // legacy path
    Variants       []LaunchVariant `json:"variants,omitempty"`        // new path
}
```

Handler logic:

- Validate `task_id`, `executor_id`, `model` as today.
- **If `req.Variants` is non-empty** (new path):
  - require ≥1 variant
  - each variant's `harness_id` must resolve via `s.harnesses.Get` (400 on unknown)
  - build one `models.VariantRequest` per `LaunchVariant`, each carrying its own `Name` and `HarnessConfig`; `IsControl = (idx == 0)`, `Ordering = idx`
- **Else** (legacy path): require `harness_ids` non-empty; build variants from `harness_ids` + the shared `harness_configs` exactly as today.

Everything downstream (`CreateExperiment`, orchestrator, harness Setup) is unchanged.

### 2. Frontend — matrix back to 3 axes

`frontend/src/pages/diagnostic/launch-matrix.ts`:

- Remove `speckitExtension` from `LaunchCell` and `ExpansionInput`.
- `countExperiments` = `task × executor × model`.
- `expandLaunchMatrix` = triple loop (task × executor × model).
- Update `launch-matrix.test.ts` to drop the speckit-axis cases and the `speckitExtension` field.

### 3. Frontend — per-experiment variant builder

`frontend/src/pages/diagnostic/launch.tsx` gains a pure helper (extracted + unit-tested):

```ts
// build-variants.ts
export interface LaunchVariant {
  harness_id: string;
  name: string;
  harness_config?: Record<string, unknown>;
}

export function buildLaunchVariants(
  selectedHarnesses: string[],
  harnessConfigs: Record<string, unknown>,
  speckitExtensions: string[],   // already filtered to non-empty
): LaunchVariant[]
```

Rules:
- For each `h` in `selectedHarnesses` in selection order:
  - if `h === 'speckit'`: emit one variant per extension in `speckitExtensions`:
    `{ harness_id: 'speckit', name: 'speckit/' + ext, harness_config: { speckit: { extension_id: ext } } }`
  - else: emit `{ harness_id: h, name: h, harness_config: configForHarness(h, harnessConfigs) }`
    where `configForHarness` returns just that harness's own block:
    - `agent_instructions` → `{ agent_instructions: harnessConfigs.agent_instructions }`
    - `multiagent` → `{ multiagent: harnessConfigs.multiagent }`
    - `bare` / `ralph` → `undefined`

### 4. Frontend — launcher wiring

`launch.tsx`:

- The `variants` useMemo (currently `harness × executor × model`) is replaced by **two** memos:
  - `experimentVariants` = `buildLaunchVariants(selectedHarnesses, harnessConfigs, validSpecKitExtensions)` — the intra-experiment variant list (used for the preview + the launch payload).
  - the cross-experiment count comes from `countExperiments({taskIds, executorIds, modelIds})`.
- `totalRuns = totalExperiments × experimentVariants.length × runsPerVariant`.
- `canSubmit` requires `experimentVariants.length > 0` (plus the existing per-harness readiness gates).
- `handleLaunch`:
  - compute `cells = expandLaunchMatrix({taskIds, executorIds, modelIds})`
  - compute `variants = buildLaunchVariants(...)` **once**
  - single cell → one `launch.mutateAsync({ task_id, executor_id, model, runs_per_variant, name, variants })`, redirect to Compare
  - multi-cell → mint batch id, fire one call per cell, each with the same `variants` array + `batch_id`/`batch_label`, redirect to `/experiments?batch=<id>`
- The Variant Preview list shows the intra-experiment variants (e.g. `bare`, `agent_instructions`, `speckit/canonical`, …), not the old harness×exec×model rows.

### 5. Frontend — types + hook

`frontend/src/lib/types.ts`: add `LaunchVariant` and extend `LaunchDiagnosticRequest` with `variants?: LaunchVariant[]`.

(`useLaunchDiagnostic` is unchanged — it already posts the whole request body.)

### 6. Compare — correct per-variant label

`frontend/src/pages/diagnostic/compare.tsx`:

- Fetch experiment **detail** (variants populated) for every experiment id, not just in matrix mode (the list endpoint omits variants → harness label was always blank). Use `useExperimentsForIds(experimentIDs)` unconditionally for `expIndex`.
- `variantSignature(exp, variantId)` resolves the run's specific variant by `variant_id` (not `variants[0]`) and uses `variant.name` (so `speckit/canonical` vs `speckit/lite` are distinct columns). Falls back to `harness_id` then `'?'`.
- `shortLabel` and the grade-table `rawCoords` pass `run.variant_id` through.

### 7. Experiments list — show variant count

`engine/internal/storage/experiment_repo.go`:

- `ListExperiments` eagerly attaches variants via a single batched query `ListVariantsByExperiments(ctx, ids)` (one `SELECT … WHERE experiment_id IN (…)` grouped in memory — not N+1).

This makes the Experiments list render `10v × 1r · bare, …, speckit/canonical` instead of the current `1 run` (which happens because the list endpoint returns no variants and `runsLabel` falls back to the runs-only label).

## Data flow (after rework)

```
[User: 1 task, 1 exec, 1 model, harnesses {bare, agent_instructions, multiagent, ralph, speckit},
       speckit extensions {canonical, lite, tdd-first, research-first, rigorous, dual-role}]
   ↓
buildLaunchVariants → 10 variants:
   bare, agent_instructions, multiagent, ralph,
   speckit/canonical, speckit/lite, speckit/tdd-first,
   speckit/research-first, speckit/rigorous, speckit/dual-role
   ↓
expandLaunchMatrix(task×exec×model) → 1 cell
   ↓
POST /diagnostic/launch { task, exec, model, runs_per_variant, variants: [...10...] }
   ↓
1 experiment, 10 variant rows (each its own harness_config), 10 runs
   ↓
Compare: 10 columns, each labelled by variant.name
Experiments list: "10v × 1r · bare, agent_instructions, …, speckit/canonical"
```

Multi-cell example: 3 tasks × 1 exec × 1 model, same harness/extension selection → 3 experiments (batched), each with the identical 10-variant list → 30 runs total, grouped under one batch.

## Error handling

- New path with empty `variants` AND empty `harness_ids` → 400 "harness_ids or variants required".
- Unknown harness id in a variant → 400 "unknown harness %q".
- A speckit variant whose config lacks `extension_id` → the speckit harness Setup already returns `ErrSpecKitExtensionMissing`; the run fails cleanly (unchanged).
- Launcher submit gate blocks the no-variant and unready-config cases before posting (unchanged gates, recomputed against `experimentVariants`).

## Testing

**Backend (Go):**
- `diagnostic_launch_test.go`:
  - `TestLaunchWithVariantsArrayCreatesPerVariantConfig` — POST with 2 variants (`speckit/canonical`, `speckit/lite`) → experiment has 2 variants, each with its own `extension_id`.
  - `TestLaunchVariantsRejectsUnknownHarness` — a variant with a bogus harness id → 400.
  - `TestLaunchVariantsEmptyFallsBackToHarnessIds` — `variants: []` + `harness_ids` → legacy behavior preserved.
  - existing legacy tests stay green.
- `experiment_repo_test.go`:
  - `TestListExperimentsIncludesVariants` — create an experiment with 3 variants, `ListExperiments` returns them populated.

**Frontend (Vitest):**
- `build-variants.test.ts`:
  - non-speckit harnesses → 1 variant each with own config block
  - speckit + 3 extensions → 3 speckit variants named `speckit/<ext>`
  - mixed selection → 4 non-speckit + N speckit, correct order
  - bare/ralph get `undefined` config
- `launch-matrix.test.ts`: updated to 3-axis (speckit cases removed).

**Manual (Playwright):**
- Select 1 task, 1 exec, 1 model, all 5 harnesses, all 6 extensions → preview shows 10 variants, "10 runs"; launch → 1 experiment, 10 variants, Compare shows 10 distinctly-labelled columns; Experiments list shows "10v × 1r".

## Risks and mitigations

| Risk | Mitigation |
|---|---|
| CLI scripts / suite endpoint break | Additive: `variants` empty → legacy `harness_ids` path runs unchanged. |
| `ListExperiments` N+1 on variant load | Single batched `WHERE experiment_id IN (…)` query, grouped in memory. |
| Eager variant load slows the list page | Variants table is tiny (≤~10 rows/experiment); one extra query per list call is negligible for local SQLite. |
| Old experiments created under the cross-product model still exist | They render fine — they're just experiments with their old variant sets. No migration needed. |

## Open questions

None — design confirmed with the user.
