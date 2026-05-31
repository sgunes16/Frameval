# Experiment batches Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let multiple experiments launched together (from the UI or a CLI script) share a `batch_id` so the Experiments list renders them as a single collapsible group instead of N orphan rows.

**Architecture:** Two new TEXT columns on `experiments` (`batch_id`, `batch_label`) thread through repo + JSON + existing single-launch endpoint as optional pass-through fields. A new endpoint `POST /api/diagnostic/launch-suite` accepts N `task_ids` and creates all the resulting experiments with one server-minted `batch_id`. Frontend launcher turns its single-task picker into a multi-select chip list and calls the suite endpoint when ≥2 tasks are picked; the Experiments list buckets rows by `batch_id`.

**Tech Stack:** Go 1.22 (stdlib `database/sql` + go-sqlite3 + Chi router + uuid), SQLite migrations, React 18 + TypeScript + TanStack Query (existing). No new dependencies on either side.

**Spec:** [`docs/superpowers/specs/2026-05-31-experiment-batches-design.md`](../specs/2026-05-31-experiment-batches-design.md)

**Branch:** `feature/experiment-batches` (already created and spec committed at `b8be88b`).

---

## File map

| Layer | File | Action |
|---|---|---|
| Schema | `engine/internal/storage/migrations/019_experiment_batches.sql` | CREATE |
| Model | `engine/internal/models/experiment.go` | MODIFY (add `BatchID`, `BatchLabel` to `Experiment` and `ExperimentRequest`) |
| Repo | `engine/internal/storage/experiment_repo.go` | MODIFY (INSERT + 2× SELECT + `scanExperiment` carry the new cols) |
| Repo tests | `engine/internal/storage/experiment_repo_test.go` | CREATE (covers batch round-trip + NULL case) |
| Launch endpoint | `engine/internal/api/diagnostic_launch.go` | MODIFY (add `BatchID`/`BatchLabel` pass-through to `LaunchDiagnosticRequest`) + ADD `LaunchDiagnosticSuite` handler |
| Router | `engine/internal/api/router.go` | MODIFY (1-line `r.Post("/diagnostic/launch-suite", ...)`) |
| Launch tests | `engine/internal/api/diagnostic_launch_test.go` | CREATE (suite happy path + partial failure + empty body + pass-through) |
| Frontend types | `frontend/src/lib/types.ts` | MODIFY (extend `Experiment`, `LaunchDiagnosticRequest`; add suite request/response types) |
| Frontend hooks | `frontend/src/lib/hooks.ts` | MODIFY (add `useLaunchDiagnosticSuite`) |
| Launcher page | `frontend/src/pages/diagnostic/launch.tsx` | MODIFY (`taskID` → `taskIDs[]`, multi-chip picker, suite-vs-single branching) |
| Experiments page | `frontend/src/pages/experiments/index.tsx` | MODIFY (group-by-batch render + remove in-table "New diagnostic run" + harness-aware runs label + sticky filter + URL `?batch=` auto-expand) |
| Grouping helper | `frontend/src/pages/experiments/grouping.ts` | CREATE (pure function `groupByBatch`) |
| Grouping tests | `frontend/src/pages/experiments/grouping.test.ts` | CREATE (4 cases) |

---

## Task 1 — Schema migration

**Files:**
- Create: `engine/internal/storage/migrations/019_experiment_batches.sql`

- [ ] **Step 1: Write the migration SQL**

Create `engine/internal/storage/migrations/019_experiment_batches.sql` with:

```sql
-- 019_experiment_batches.sql
--
-- Adds batch identity to experiments so the Diagnostic launcher can
-- create N experiments under one logical batch and the Experiments
-- list can group them. Both columns are NULL-able so pre-existing
-- experiments stay unbatched; no backfill.
--
-- SQLite ADD COLUMN is O(1) metadata-only on populated tables; the
-- index creation is fast on an empty column.

ALTER TABLE experiments ADD COLUMN batch_id TEXT;
ALTER TABLE experiments ADD COLUMN batch_label TEXT;
CREATE INDEX IF NOT EXISTS idx_experiments_batch_id ON experiments(batch_id);
```

- [ ] **Step 2: Verify migration applies cleanly**

Run from the repo root:

```bash
rm -f /tmp/frameval-migrate.db
cd engine && FRAMEVAL_DB_PATH=/tmp/frameval-migrate.db go run cmd/server/main.go &
SERVER_PID=$!
sleep 4
sqlite3 /tmp/frameval-migrate.db '.schema experiments' | grep -E 'batch_id|batch_label' && echo "MIGRATION APPLIED"
kill $SERVER_PID 2>/dev/null
rm -f /tmp/frameval-migrate.db
```

Expected: `batch_id TEXT` and `batch_label TEXT` appear in the schema dump, plus the index. The "MIGRATION APPLIED" marker confirms both columns exist.

- [ ] **Step 3: Commit**

```bash
git add engine/internal/storage/migrations/019_experiment_batches.sql
git commit -m "Add batch_id/batch_label columns to experiments (migration 019)"
```

---

## Task 2 — Go model

**Files:**
- Modify: `engine/internal/models/experiment.go`

- [ ] **Step 1: Add fields to `Experiment`**

In `engine/internal/models/experiment.go`, find the `type Experiment struct` block. Add these two fields right before the closing `Variants` field (so they sit with the other identifying / metadata fields):

```go
	BatchID             string             `json:"batch_id,omitempty"`
	BatchLabel          string             `json:"batch_label,omitempty"`
```

- [ ] **Step 2: Add fields to `ExperimentRequest`**

In the same file, find `type ExperimentRequest struct` (further down — it's the create/update payload that `CreateExperiment` accepts). Add the same two fields anywhere in the struct (convention: near the end, before `Variants`):

```go
	BatchID        string                 `json:"batch_id,omitempty"`
	BatchLabel     string                 `json:"batch_label,omitempty"`
```

- [ ] **Step 3: Verify the build still compiles**

```bash
cd engine && go build ./...
```

Expected: exits 0 with no output.

- [ ] **Step 4: Commit**

```bash
git add engine/internal/models/experiment.go
git commit -m "Add BatchID/BatchLabel to Experiment + ExperimentRequest models"
```

---

## Task 3 — Repo round-trip (TDD)

**Files:**
- Test: `engine/internal/storage/experiment_repo_test.go` (CREATE)
- Modify: `engine/internal/storage/experiment_repo.go`

- [ ] **Step 1: Write the failing test**

Create `engine/internal/storage/experiment_repo_test.go` with:

```go
package storage

import (
	"context"
	"testing"

	"github.com/mustafaselman/frameval/engine/internal/models"
	"github.com/mustafaselman/frameval/engine/test/support"
)

func seedTaskForExperimentTest(t *testing.T, store *Store, taskID string) {
	t.Helper()
	if _, err := store.DB.ExecContext(context.Background(), `
		INSERT INTO tasks (id, name, description, category, complexity_score, codebase_type, task_prompt)
		VALUES (?, 'Test task', 'desc', 'greenfield', 1.0, 'fresh', 'do it')
	`, taskID); err != nil {
		t.Fatalf("seed task: %v", err)
	}
}

func TestCreateExperimentPersistsBatchFields(t *testing.T) {
	store := support.TmpStore(t)
	seedTaskForExperimentTest(t, store, "task-batch-1")

	created, err := store.CreateExperiment(context.Background(), models.ExperimentRequest{
		Name:           "batched",
		TaskID:         "task-batch-1",
		Model:          "claude",
		AgentCLI:       "claude",
		RunsPerVariant: 1,
		BatchID:        "batch-abc",
		BatchLabel:     "Calibration suite v1",
	})
	if err != nil {
		t.Fatalf("CreateExperiment: %v", err)
	}
	if created.BatchID != "batch-abc" {
		t.Fatalf("batch_id round-trip: got %q want %q", created.BatchID, "batch-abc")
	}
	if created.BatchLabel != "Calibration suite v1" {
		t.Fatalf("batch_label round-trip: got %q want %q", created.BatchLabel, "Calibration suite v1")
	}

	listed, err := store.ListExperiments(context.Background())
	if err != nil {
		t.Fatalf("ListExperiments: %v", err)
	}
	if len(listed) != 1 {
		t.Fatalf("list size: got %d want 1", len(listed))
	}
	if listed[0].BatchID != "batch-abc" || listed[0].BatchLabel != "Calibration suite v1" {
		t.Fatalf("list batch fields: got id=%q label=%q", listed[0].BatchID, listed[0].BatchLabel)
	}
}

func TestCreateExperimentNullBatchFields(t *testing.T) {
	store := support.TmpStore(t)
	seedTaskForExperimentTest(t, store, "task-batch-2")

	created, err := store.CreateExperiment(context.Background(), models.ExperimentRequest{
		Name:           "unbatched",
		TaskID:         "task-batch-2",
		Model:          "claude",
		AgentCLI:       "claude",
		RunsPerVariant: 1,
	})
	if err != nil {
		t.Fatalf("CreateExperiment: %v", err)
	}
	if created.BatchID != "" {
		t.Fatalf("batch_id should be empty: got %q", created.BatchID)
	}
	if created.BatchLabel != "" {
		t.Fatalf("batch_label should be empty: got %q", created.BatchLabel)
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

```bash
cd engine && go test ./internal/storage/ -run TestCreateExperiment -v
```

Expected: both tests fail. The first likely fails on the assertion `batch_id round-trip: got "" want "batch-abc"` because the INSERT statement doesn't carry the new columns yet.

- [ ] **Step 3: Update the INSERT to carry the new columns**

In `engine/internal/storage/experiment_repo.go`, find the INSERT statement inside `CreateExperiment`. Replace it with the version below (added `batch_id, batch_label` to the column list, two extra `?` placeholders, and two extra Exec args at the end):

```go
	_, err = tx.ExecContext(ctx, `
		INSERT INTO experiments (
			id, name, description, status, task_id, workspace_source_type, local_path, git_url, git_ref, model, agent_cli, execution_mode,
			runs_per_variant, temperature, timeout_seconds, max_concurrent, judge_model, seed, composite_weights_json,
			batch_id, batch_label
		) VALUES (?, ?, ?, 'draft', ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, experimentID, req.Name, req.Description, req.TaskID, fallbackExperimentWorkspace(req.WorkspaceSourceType), nullableString(req.LocalPath), nullableString(req.GitURL), nullableString(req.GitRef), req.Model, req.AgentCLI, req.ExecutionMode,
		req.RunsPerVariant, req.Temperature, req.TimeoutSeconds, req.MaxConcurrent, req.JudgeModel, req.Seed, marshalJSON(weights),
		nullableString(req.BatchID), nullableString(req.BatchLabel))
```

- [ ] **Step 4: Update both SELECTs to fetch the new columns**

Same file, in `ListExperiments` (around line 64) replace the query string with:

```go
	rows, err := s.DB.QueryContext(ctx, `
		SELECT id, name, description, status, task_id, workspace_source_type, local_path, git_url, git_ref, model, agent_cli, execution_mode,
		       runs_per_variant, temperature, timeout_seconds, max_concurrent, judge_model,
		       seed, estimated_cost_usd, actual_cost_usd, composite_weights_json,
		       created_at, started_at, completed_at,
		       batch_id, batch_label
		FROM experiments ORDER BY created_at DESC
	`)
```

In `GetExperiment` (around line 88) replace the query string with:

```go
	row := s.DB.QueryRowContext(ctx, `
		SELECT id, name, description, status, task_id, workspace_source_type, local_path, git_url, git_ref, model, agent_cli, execution_mode,
		       runs_per_variant, temperature, timeout_seconds, max_concurrent, judge_model,
		       seed, estimated_cost_usd, actual_cost_usd, composite_weights_json,
		       created_at, started_at, completed_at,
		       batch_id, batch_label
		FROM experiments WHERE id = ?
	`, experimentID)
```

- [ ] **Step 5: Update `scanExperiment` to scan the new columns**

Same file, replace the entire `scanExperiment` function with:

```go
func scanExperiment(scanner interface{ Scan(dest ...any) error }) (models.Experiment, error) {
	var experiment models.Experiment
	var description, judgeModel, startedAt, completedAt, localPath, gitURL, gitRef sql.NullString
	var workspaceSourceType string
	var seed sql.NullInt64
	var estimated, actual sql.NullFloat64
	var weightsRaw string
	var batchID, batchLabel sql.NullString
	if err := scanner.Scan(
		&experiment.ID, &experiment.Name, &description, &experiment.Status, &experiment.TaskID, &workspaceSourceType, &localPath, &gitURL, &gitRef, &experiment.Model,
		&experiment.AgentCLI, &experiment.ExecutionMode, &experiment.RunsPerVariant, &experiment.Temperature,
		&experiment.TimeoutSeconds, &experiment.MaxConcurrent, &judgeModel, &seed, &estimated, &actual,
		&weightsRaw, &experiment.CreatedAt, &startedAt, &completedAt,
		&batchID, &batchLabel,
	); err != nil {
		return experiment, fmt.Errorf("scan experiment: %w", err)
	}
	experiment.Description = description.String
	experiment.WorkspaceSourceType = workspaceSourceType
	experiment.LocalPath = localPath.String
	experiment.GitURL = gitURL.String
	experiment.GitRef = gitRef.String
	experiment.JudgeModel = judgeModel.String
	experiment.StartedAt = startedAt.String
	experiment.CompletedAt = completedAt.String
	if seed.Valid {
		value := int(seed.Int64)
		experiment.Seed = &value
	}
	if estimated.Valid {
		value := estimated.Float64
		experiment.EstimatedCostUSD = &value
	}
	if actual.Valid {
		value := actual.Float64
		experiment.ActualCostUSD = &value
	}
	experiment.CompositeWeights = unmarshalJSON(weightsRaw, map[string]float64{"code": 0.3, "judge": 0.3, "process": 0.2, "adherence": 0.2})
	experiment.BatchID = batchID.String
	experiment.BatchLabel = batchLabel.String
	return experiment, nil
}
```

- [ ] **Step 6: Re-run the tests; they must now pass**

```bash
cd engine && go test ./internal/storage/ -run TestCreateExperiment -v
```

Expected: both tests PASS.

- [ ] **Step 7: Run the full engine test suite to confirm no regression**

```bash
cd engine && go test ./...
```

Expected: every package green.

- [ ] **Step 8: Commit**

```bash
git add engine/internal/storage/experiment_repo.go engine/internal/storage/experiment_repo_test.go
git commit -m "Persist batch_id/batch_label through experiments repo"
```

---

## Task 4 — Single-launch endpoint accepts batch pass-through (TDD)

**Files:**
- Test: `engine/internal/api/diagnostic_launch_test.go` (CREATE)
- Modify: `engine/internal/api/diagnostic_launch.go`

- [ ] **Step 1: Write the failing pass-through test**

Create `engine/internal/api/diagnostic_launch_test.go` with:

```go
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mustafaselman/frameval/engine/internal/models"
	"github.com/mustafaselman/frameval/engine/test/support"
)

func newLaunchTestService(t *testing.T) *Service {
	t.Helper()
	store := support.TmpStore(t)
	if _, err := store.DB.ExecContext(context.Background(), `
		INSERT INTO tasks (id, name, description, category, complexity_score, codebase_type, task_prompt)
		VALUES ('t-launch', 'Launch task', 'desc', 'greenfield', 1.0, 'fresh', 'do it')
	`); err != nil {
		t.Fatalf("seed task: %v", err)
	}
	return &Service{
		store:        store,
		harnesses:    support.NewStaticHarnessRegistry("bare", "ralph"),
		executors:    support.NewStaticExecutorRegistry("opencode"),
		orchestrator: support.NewNoopOrchestrator(),
	}
}

func TestLaunchDiagnosticAcceptsBatchPassThrough(t *testing.T) {
	svc := newLaunchTestService(t)

	body, _ := json.Marshal(map[string]any{
		"task_id":     "t-launch",
		"executor_id": "opencode",
		"harness_ids": []string{"bare"},
		"model":       "anything",
		"batch_id":    "batch-xyz",
		"batch_label": "Suite from CLI",
	})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/diagnostic/launch", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	svc.LaunchDiagnostic(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status: got %d body=%s", rec.Code, rec.Body.String())
	}

	var resp LaunchDiagnosticResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v body=%s", err, rec.Body.String())
	}
	exp, err := svc.store.GetExperiment(context.Background(), resp.ExperimentID)
	if err != nil {
		t.Fatalf("fetch experiment: %v", err)
	}
	if exp.BatchID != "batch-xyz" {
		t.Fatalf("BatchID: got %q want %q", exp.BatchID, "batch-xyz")
	}
	if exp.BatchLabel != "Suite from CLI" {
		t.Fatalf("BatchLabel: got %q want %q", exp.BatchLabel, "Suite from CLI")
	}
	_ = models.Experiment{}
}
```

If `support.NewStaticHarnessRegistry`, `support.NewStaticExecutorRegistry`, or `support.NewNoopOrchestrator` don't exist yet, add them at the end of `engine/test/support/*.go` (use the same pattern as existing `TmpStore`). Inspect `engine/test/support/` first to see what's there:

```bash
ls engine/test/support/
grep -n 'TmpStore\|FakeExecutor\|FakeGrader' engine/test/support/*.go
```

If a helper called something similar already exists (e.g., `FakeExecutor`, `FakeHarness`), use it instead of creating a duplicate.

- [ ] **Step 2: Run to verify the test fails**

```bash
cd engine && go test ./internal/api/ -run TestLaunchDiagnosticAcceptsBatchPassThrough -v
```

Expected: FAIL. The handler reads `BatchID`/`BatchLabel` off the request but doesn't pass them to `CreateExperiment`, so the experiment's batch fields stay empty.

- [ ] **Step 3: Wire BatchID/BatchLabel through `LaunchDiagnostic`**

In `engine/internal/api/diagnostic_launch.go`, add the two fields to `LaunchDiagnosticRequest`:

```go
type LaunchDiagnosticRequest struct {
	TaskID         string   `json:"task_id"`
	ExecutorID     string   `json:"executor_id"`
	HarnessIDs     []string `json:"harness_ids"`
	Model          string   `json:"model"`
	RunsPerVariant int      `json:"runs_per_variant"`
	TimeoutSeconds int      `json:"timeout_seconds"`
	Name           string   `json:"name"`
	BatchID        string   `json:"batch_id"`
	BatchLabel     string   `json:"batch_label"`
}
```

Then in `LaunchDiagnostic`, inside the `models.ExperimentRequest{...}` literal that's passed to `CreateExperiment`, add the two trailing fields:

```go
	experiment, err := s.store.CreateExperiment(r.Context(), models.ExperimentRequest{
		Name:           name,
		Description:    fmt.Sprintf("Diagnostic launcher: %d harness(es), executor=%s", len(req.HarnessIDs), req.ExecutorID),
		TaskID:         req.TaskID,
		Model:          req.Model,
		AgentCLI:       req.ExecutorID,
		ExecutionMode:  "cli",
		RunsPerVariant: runsPerVariant,
		TimeoutSeconds: timeout,
		MaxConcurrent:  1,
		Variants:       variants,
		BatchID:        req.BatchID,
		BatchLabel:     req.BatchLabel,
	})
```

- [ ] **Step 4: Run the test; it must pass**

```bash
cd engine && go test ./internal/api/ -run TestLaunchDiagnosticAcceptsBatchPassThrough -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add engine/internal/api/diagnostic_launch.go engine/internal/api/diagnostic_launch_test.go engine/test/support/
git commit -m "Single-launch endpoint accepts batch_id + batch_label pass-through"
```

(Only `git add engine/test/support/` if you actually added helpers there.)

---

## Task 5 — Suite-launch endpoint (TDD)

**Files:**
- Test: `engine/internal/api/diagnostic_launch_test.go` (MODIFY — append cases)
- Modify: `engine/internal/api/diagnostic_launch.go`
- Modify: `engine/internal/api/router.go`

- [ ] **Step 1: Append three failing tests for the suite endpoint**

Append to `engine/internal/api/diagnostic_launch_test.go`:

```go
func TestLaunchDiagnosticSuiteHappyPath(t *testing.T) {
	svc := newLaunchTestService(t)
	// Seed a second task so we can launch 2 in one batch.
	if _, err := svc.store.DB.ExecContext(context.Background(), `
		INSERT INTO tasks (id, name, description, category, complexity_score, codebase_type, task_prompt)
		VALUES ('t-launch-2', 'Second task', 'desc', 'greenfield', 1.0, 'fresh', 'do it')
	`); err != nil {
		t.Fatalf("seed second task: %v", err)
	}

	body, _ := json.Marshal(map[string]any{
		"task_ids":    []string{"t-launch", "t-launch-2"},
		"executor_id": "opencode",
		"harness_ids": []string{"bare"},
		"model":       "anything",
		"batch_label": "Happy path suite",
	})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/diagnostic/launch-suite", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	svc.LaunchDiagnosticSuite(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status: got %d body=%s", rec.Code, rec.Body.String())
	}

	var resp LaunchDiagnosticSuiteResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.BatchID == "" {
		t.Fatal("BatchID empty")
	}
	if len(resp.ExperimentIDs) != 2 {
		t.Fatalf("ExperimentIDs: got %d want 2", len(resp.ExperimentIDs))
	}
	if len(resp.Failures) != 0 {
		t.Fatalf("Failures: got %+v want none", resp.Failures)
	}

	// Both experiments share the batch_id and label.
	for _, eid := range resp.ExperimentIDs {
		exp, err := svc.store.GetExperiment(context.Background(), eid)
		if err != nil {
			t.Fatalf("fetch %s: %v", eid, err)
		}
		if exp.BatchID != resp.BatchID {
			t.Errorf("exp %s BatchID=%q want %q", eid, exp.BatchID, resp.BatchID)
		}
		if exp.BatchLabel != "Happy path suite" {
			t.Errorf("exp %s BatchLabel=%q want %q", eid, exp.BatchLabel, "Happy path suite")
		}
	}
}

func TestLaunchDiagnosticSuitePartialFailure(t *testing.T) {
	svc := newLaunchTestService(t)

	body, _ := json.Marshal(map[string]any{
		"task_ids":    []string{"t-launch", "does-not-exist"},
		"executor_id": "opencode",
		"harness_ids": []string{"bare"},
		"model":       "anything",
		"batch_label": "Partial",
	})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/diagnostic/launch-suite", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	svc.LaunchDiagnosticSuite(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status: got %d body=%s", rec.Code, rec.Body.String())
	}
	var resp LaunchDiagnosticSuiteResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.ExperimentIDs) != 1 {
		t.Fatalf("ExperimentIDs: got %d want 1", len(resp.ExperimentIDs))
	}
	if len(resp.Failures) != 1 {
		t.Fatalf("Failures: got %d want 1", len(resp.Failures))
	}
	if resp.Failures[0].TaskID != "does-not-exist" {
		t.Errorf("failure task_id: got %q want %q", resp.Failures[0].TaskID, "does-not-exist")
	}
	if resp.BatchID == "" {
		t.Fatal("BatchID empty")
	}
}

func TestLaunchDiagnosticSuiteRejectsEmptyTaskIDs(t *testing.T) {
	svc := newLaunchTestService(t)

	body, _ := json.Marshal(map[string]any{
		"task_ids":    []string{},
		"executor_id": "opencode",
		"harness_ids": []string{"bare"},
		"model":       "anything",
	})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/diagnostic/launch-suite", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	svc.LaunchDiagnosticSuite(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d body=%s want 400", rec.Code, rec.Body.String())
	}
}
```

- [ ] **Step 2: Run; they must fail (handler doesn't exist yet)**

```bash
cd engine && go test ./internal/api/ -run TestLaunchDiagnosticSuite -v
```

Expected: compilation error (the type and method don't exist), or undefined-method failures. That's the desired red.

- [ ] **Step 3: Implement the suite handler**

Append to `engine/internal/api/diagnostic_launch.go`:

```go
// LaunchDiagnosticSuiteRequest launches N experiments in one POST,
// all sharing a server-minted batch_id. Display-only batch_label
// helps the Experiments list show a readable group title.
type LaunchDiagnosticSuiteRequest struct {
	TaskIDs        []string `json:"task_ids"`
	ExecutorID     string   `json:"executor_id"`
	HarnessIDs     []string `json:"harness_ids"`
	Model          string   `json:"model"`
	RunsPerVariant int      `json:"runs_per_variant"`
	TimeoutSeconds int      `json:"timeout_seconds"`
	BatchLabel     string   `json:"batch_label"`
}

// LaunchDiagnosticSuiteResponse returns the new batch identity and
// per-task outcomes. Partial failure is reported in Failures and
// does not abort the rest of the batch.
type LaunchDiagnosticSuiteResponse struct {
	BatchID       string                `json:"batch_id"`
	ExperimentIDs []string              `json:"experiment_ids"`
	Failures      []SuiteLaunchFailure  `json:"failures,omitempty"`
}

type SuiteLaunchFailure struct {
	TaskID  string `json:"task_id"`
	Message string `json:"message"`
}

// LaunchDiagnosticSuite is the multi-task launch endpoint. It iterates
// over TaskIDs and creates one experiment per task with a shared
// batch_id. One bad task ID doesn't fail the rest — failures land in
// the response's Failures array.
func (s *Service) LaunchDiagnosticSuite(w http.ResponseWriter, r *http.Request) {
	var req LaunchDiagnosticSuiteRequest
	if err := decodeJSON(r, &req); err != nil {
		renderError(w, r.Context(), http.StatusBadRequest, ErrCodeBadRequest, "invalid request", err)
		return
	}
	if len(req.TaskIDs) == 0 {
		renderError(w, r.Context(), http.StatusBadRequest, ErrCodeBadRequest, "task_ids must contain at least one task", nil)
		return
	}
	if strings.TrimSpace(req.ExecutorID) == "" {
		renderError(w, r.Context(), http.StatusBadRequest, ErrCodeBadRequest, "executor_id is required", nil)
		return
	}
	if len(req.HarnessIDs) == 0 {
		renderError(w, r.Context(), http.StatusBadRequest, ErrCodeBadRequest, "harness_ids must contain at least one harness", nil)
		return
	}
	for _, hid := range req.HarnessIDs {
		if _, err := s.harnesses.Get(hid); err != nil {
			renderError(w, r.Context(), http.StatusBadRequest, ErrCodeBadRequest, fmt.Sprintf("unknown harness %q", hid), err)
			return
		}
	}
	if _, err := s.executors.Get(req.ExecutorID); err != nil {
		renderError(w, r.Context(), http.StatusBadRequest, ErrCodeBadRequest, fmt.Sprintf("unknown executor %q", req.ExecutorID), err)
		return
	}

	runsPerVariant := req.RunsPerVariant
	if runsPerVariant <= 0 {
		runsPerVariant = 5
	}
	timeout := req.TimeoutSeconds
	if timeout <= 0 {
		timeout = 600
	}
	batchID := uuid.NewString()
	label := strings.TrimSpace(req.BatchLabel)
	if label == "" {
		label = fmt.Sprintf("Diagnostic suite · %s", time.Now().UTC().Format("2006-01-02 15:04"))
	}

	resp := LaunchDiagnosticSuiteResponse{BatchID: batchID}

	for _, tid := range req.TaskIDs {
		task, err := s.store.GetTask(r.Context(), tid)
		if err != nil {
			resp.Failures = append(resp.Failures, SuiteLaunchFailure{TaskID: tid, Message: fmt.Sprintf("task not found: %v", err)})
			continue
		}

		variants := make([]models.VariantRequest, 0, len(req.HarnessIDs))
		for idx, hid := range req.HarnessIDs {
			variants = append(variants, models.VariantRequest{
				Name:        hid,
				Description: fmt.Sprintf("Harness: %s", hid),
				IsControl:   idx == 0,
				Ordering:    idx,
				HarnessID:   hid,
			})
		}

		experiment, err := s.store.CreateExperiment(r.Context(), models.ExperimentRequest{
			Name:           fmt.Sprintf("%s · %s", label, task.Name),
			Description:    fmt.Sprintf("Diagnostic suite (%d task(s)), executor=%s", len(req.TaskIDs), req.ExecutorID),
			TaskID:         tid,
			Model:          req.Model,
			AgentCLI:       req.ExecutorID,
			ExecutionMode:  "cli",
			RunsPerVariant: runsPerVariant,
			TimeoutSeconds: timeout,
			MaxConcurrent:  1,
			Variants:       variants,
			BatchID:        batchID,
			BatchLabel:     label,
		})
		if err != nil {
			resp.Failures = append(resp.Failures, SuiteLaunchFailure{TaskID: tid, Message: fmt.Sprintf("create experiment: %v", err)})
			continue
		}
		if err := s.orchestrator.StartExperiment(r.Context(), experiment.ID); err != nil {
			_ = s.store.UpdateExperimentStatus(r.Context(), experiment.ID, "failed")
			resp.Failures = append(resp.Failures, SuiteLaunchFailure{TaskID: tid, Message: fmt.Sprintf("start experiment: %v", err)})
			continue
		}
		resp.ExperimentIDs = append(resp.ExperimentIDs, experiment.ID)
	}

	JSON(w, http.StatusAccepted, resp)
}
```

Make sure these imports are present at the top of `diagnostic_launch.go`:

```go
import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/mustafaselman/frameval/engine/internal/models"
)
```

(`google/uuid` is already in `go.mod` — used by repo layer.)

- [ ] **Step 4: Register the route**

In `engine/internal/api/router.go`, find the existing `r.Post("/diagnostic/launch", ...)` line (around line 94). Add immediately below it:

```go
		r.Post("/diagnostic/launch-suite", service.LaunchDiagnosticSuite)
```

- [ ] **Step 5: Run the suite tests; they must pass**

```bash
cd engine && go test ./internal/api/ -run TestLaunchDiagnosticSuite -v
```

Expected: 3/3 PASS.

- [ ] **Step 6: Run the full engine test suite to confirm no regression**

```bash
cd engine && go test ./...
```

Expected: all packages green.

- [ ] **Step 7: Commit**

```bash
git add engine/internal/api/diagnostic_launch.go engine/internal/api/diagnostic_launch_test.go engine/internal/api/router.go
git commit -m "Add POST /api/diagnostic/launch-suite for batched experiment launches"
```

---

## Task 6 — Frontend types + hook

**Files:**
- Modify: `frontend/src/lib/types.ts`
- Modify: `frontend/src/lib/hooks.ts`

- [ ] **Step 1: Extend the Experiment type with batch fields**

In `frontend/src/lib/types.ts`, find `export type Experiment = {` (around line 114). Append the two new optional fields right before the closing `}`:

```ts
  batch_id?: string;
  batch_label?: string;
```

- [ ] **Step 2: Extend `LaunchDiagnosticRequest` with batch pass-through**

In the same file, find `export type LaunchDiagnosticRequest` (around line 284). Replace with:

```ts
export type LaunchDiagnosticRequest = {
  task_id: string;
  executor_id: string;
  harness_ids: string[];
  model?: string;
  runs_per_variant?: number;
  timeout_seconds?: number;
  name?: string;
  batch_id?: string;
  batch_label?: string;
};
```

- [ ] **Step 3: Add suite request/response types**

Append right after `LaunchDiagnosticResponse`:

```ts
export type LaunchDiagnosticSuiteRequest = {
  task_ids: string[];
  executor_id: string;
  harness_ids: string[];
  model?: string;
  runs_per_variant?: number;
  timeout_seconds?: number;
  batch_label?: string;
};

export type SuiteLaunchFailure = {
  task_id: string;
  message: string;
};

export type LaunchDiagnosticSuiteResponse = {
  batch_id: string;
  experiment_ids: string[];
  failures?: SuiteLaunchFailure[];
};
```

- [ ] **Step 4: Add `useLaunchDiagnosticSuite` hook**

In `frontend/src/lib/hooks.ts`, find the existing `useLaunchDiagnostic` function. Add a sibling immediately below:

```ts
export function useLaunchDiagnosticSuite() {
  const client = useQueryClient();
  return useMutation({
    mutationFn: (payload: LaunchDiagnosticSuiteRequest) =>
      api.post<LaunchDiagnosticSuiteResponse>('/diagnostic/launch-suite', payload),
    onSuccess: () => {
      client.invalidateQueries({ queryKey: ['experiments'] });
      client.invalidateQueries({ queryKey: ['runs'] });
    },
  });
}
```

Make sure the import line near the top of `hooks.ts` that already imports types from `./types` is extended to include the two new ones:

```ts
import type {
  // …existing list…
  LaunchDiagnosticSuiteRequest,
  LaunchDiagnosticSuiteResponse,
} from './types';
```

If the file uses a wildcard `import * as types from './types'` pattern instead, no edit is needed — re-check by reading the import block.

- [ ] **Step 5: Typecheck**

```bash
cd frontend && npm run lint
```

Expected: exits 0 (this project's `lint` is `tsc --noEmit`).

- [ ] **Step 6: Commit**

```bash
git add frontend/src/lib/types.ts frontend/src/lib/hooks.ts
git commit -m "Add suite-launch types + useLaunchDiagnosticSuite hook"
```

---

## Task 7 — Grouping helper + tests (TDD)

**Files:**
- Test: `frontend/src/pages/experiments/grouping.test.ts` (CREATE)
- Create: `frontend/src/pages/experiments/grouping.ts`

- [ ] **Step 1: Write the failing test**

Create `frontend/src/pages/experiments/grouping.test.ts`:

```ts
import { describe, expect, it } from 'vitest';
import { groupByBatch, type GroupedExperiment } from './grouping';
import type { Experiment } from '../../lib/types';

function exp(id: string, batchId: string | undefined, createdAt: string, status = 'completed'): Experiment {
  return {
    id,
    name: id,
    status,
    task_id: 't',
    model: 'm',
    agent_cli: 'a',
    execution_mode: 'cli',
    runs_per_variant: 1,
    temperature: 0,
    timeout_seconds: 600,
    max_concurrent: 1,
    created_at: createdAt,
    batch_id: batchId,
  };
}

describe('groupByBatch', () => {
  it('groups two experiments that share a batch_id', () => {
    const items = [exp('a', 'b1', '2026-05-31T10:00:00Z'), exp('b', 'b1', '2026-05-31T10:01:00Z')];
    const out = groupByBatch(items);
    expect(out).toHaveLength(1);
    expect(out[0].kind).toBe('group');
    if (out[0].kind === 'group') {
      expect(out[0].experiments.map((e) => e.id).sort()).toEqual(['a', 'b']);
    }
  });

  it('renders a singleton batch as solo (avoid 1-item groups)', () => {
    const items = [exp('a', 'b1', '2026-05-31T10:00:00Z')];
    const out = groupByBatch(items);
    expect(out).toHaveLength(1);
    expect(out[0].kind).toBe('solo');
  });

  it('renders experiments without a batch_id as solo', () => {
    const items = [exp('a', undefined, '2026-05-31T10:00:00Z')];
    const out = groupByBatch(items);
    expect(out).toHaveLength(1);
    expect(out[0].kind).toBe('solo');
  });

  it('mixed input: 3 batched + 1 solo-batched + 1 unbatched → 1 group + 2 solos, sorted by recency', () => {
    const items = [
      exp('older', 'A', '2026-05-31T09:00:00Z'),
      exp('newer', 'A', '2026-05-31T11:00:00Z'),
      exp('mid', 'A', '2026-05-31T10:00:00Z'),
      exp('lonelyBatch', 'B', '2026-05-31T12:00:00Z'),
      exp('noBatch', undefined, '2026-05-31T13:00:00Z'),
    ];
    const out = groupByBatch(items);
    // ordering: noBatch (13:00) → lonelyBatch (12:00) → group A (max=11:00)
    expect(out.map((g: GroupedExperiment) => (g.kind === 'group' ? `group:${g.batchId}` : `solo:${g.experiment.id}`))).toEqual([
      'solo:noBatch',
      'solo:lonelyBatch',
      'group:A',
    ]);
    const groupA = out.find((g) => g.kind === 'group' && g.batchId === 'A');
    expect(groupA).toBeDefined();
    if (groupA && groupA.kind === 'group') {
      // children sorted newest-first within the group
      expect(groupA.experiments.map((e) => e.id)).toEqual(['newer', 'mid', 'older']);
    }
  });
});
```

- [ ] **Step 2: Run; the test must fail (module doesn't exist)**

```bash
cd frontend && npx vitest run src/pages/experiments/grouping.test.ts
```

Expected: FAIL — "Cannot find module './grouping'".

- [ ] **Step 3: Implement the helper**

Create `frontend/src/pages/experiments/grouping.ts`:

```ts
import type { Experiment } from '../../lib/types';

export type GroupedExperiment =
  | { kind: 'solo'; experiment: Experiment }
  | { kind: 'group'; batchId: string; batchLabel: string; experiments: Experiment[] };

/**
 * Bucket experiments by their batch_id.
 *
 *   - Two or more experiments sharing a batch_id form a `group`.
 *   - A batch_id with only one experiment in the current view, or an
 *     experiment without a batch_id at all, renders as `solo`.
 *
 * Within a group, members sort newest-first. Across groups + solos,
 * the position of each unit is the most-recent created_at among its
 * members (a group's anchor is its newest child).
 */
export function groupByBatch(experiments: Experiment[]): GroupedExperiment[] {
  const byBatch = new Map<string, Experiment[]>();
  const solos: Experiment[] = [];

  for (const exp of experiments) {
    if (exp.batch_id) {
      const bucket = byBatch.get(exp.batch_id);
      if (bucket) bucket.push(exp);
      else byBatch.set(exp.batch_id, [exp]);
    } else {
      solos.push(exp);
    }
  }

  const units: GroupedExperiment[] = solos.map((experiment) => ({ kind: 'solo' as const, experiment }));

  for (const [batchId, members] of byBatch.entries()) {
    if (members.length < 2) {
      // Singleton "batch" — render flat to avoid visual noise.
      units.push({ kind: 'solo', experiment: members[0] });
      continue;
    }
    // Newest child first within the group.
    members.sort((a, b) => (b.created_at ?? '').localeCompare(a.created_at ?? ''));
    const batchLabel = members[0].batch_label ?? batchId;
    units.push({ kind: 'group', batchId, batchLabel, experiments: members });
  }

  // Anchor each unit by its newest member, then sort descending.
  const anchor = (g: GroupedExperiment): string =>
    g.kind === 'solo'
      ? g.experiment.created_at ?? ''
      : g.experiments[0]?.created_at ?? '';

  units.sort((a, b) => anchor(b).localeCompare(anchor(a)));
  return units;
}
```

- [ ] **Step 4: Re-run; tests must pass**

```bash
cd frontend && npx vitest run src/pages/experiments/grouping.test.ts
```

Expected: 4/4 PASS.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/pages/experiments/grouping.ts frontend/src/pages/experiments/grouping.test.ts
git commit -m "Add groupByBatch helper for experiment list grouping"
```

---

## Task 8 — Experiments list: render groups

**Files:**
- Modify: `frontend/src/pages/experiments/index.tsx`

- [ ] **Step 1: Update imports + add group expand state**

In `frontend/src/pages/experiments/index.tsx`, add the import below the existing ones:

```ts
import { useSearchParams } from 'react-router-dom';
import { groupByBatch, type GroupedExperiment } from './grouping';
```

Inside `ExperimentsPage`, add a `useState` for expand state and a `useSearchParams` hook to read `?batch=`:

```ts
  const [searchParams] = useSearchParams();
  const focusBatch = searchParams.get('batch') ?? '';
  const [expandedBatches, setExpandedBatches] = useState<Record<string, boolean>>({});
```

- [ ] **Step 2: Build the grouped view**

Right below the existing `filtered` `useMemo`, add:

```ts
  const grouped = useMemo(() => groupByBatch(filtered), [filtered]);
```

- [ ] **Step 3: Initialize default expand state**

Add after the `grouped` memo:

```ts
  useEffect(() => {
    setExpandedBatches((prev) => {
      const next = { ...prev };
      for (const unit of grouped) {
        if (unit.kind !== 'group') continue;
        if (next[unit.batchId] !== undefined) continue;
        // Default: expanded for small groups, collapsed for large ones.
        // The URL-focused batch always defaults to expanded.
        next[unit.batchId] = unit.experiments.length <= 3 || unit.batchId === focusBatch;
      }
      return next;
    });
  }, [grouped, focusBatch]);
```

Add `useEffect` to the React imports if missing.

- [ ] **Step 4: Drop the in-table "New diagnostic run" button**

In the filter Card's right cluster (the `<div className="flex items-center gap-2">` block), remove the `<Link to="/diagnostic/launch"><Button size="sm">New diagnostic run</Button></Link>` element. The `<Input>` stays — it becomes the only element in the right cluster. The empty-state CTA keeps its own button.

- [ ] **Step 5: Make the filter Card sticky**

Replace the filter `<Card>` opener with:

```tsx
      <Card className="sticky top-0 z-10">
```

- [ ] **Step 6: Compute a `runsLabel` helper**

Below the `ExperimentsPage` function (alongside `SummaryChips`), add:

```tsx
function runsLabel(experiment: Experiment): string {
  const variantCount = experiment.variants?.length ?? 0;
  if (variantCount === 0) {
    return `${experiment.runs_per_variant} run${experiment.runs_per_variant === 1 ? '' : 's'}`;
  }
  const harnessNames = (experiment.variants ?? []).map((v) => v.harness_id ?? v.name).join(', ');
  return `${variantCount}v × ${experiment.runs_per_variant}r${harnessNames ? ` · ${harnessNames}` : ''}`;
}
```

Make sure `Experiment` is imported from `../../lib/types`.

Replace the inline `runsLabel` calculation in each row (currently `const runsLabel = \`${variantCount}v × ${experiment.runs_per_variant}r\`;`) with a call to the helper: `runsLabel(experiment)`.

- [ ] **Step 7: Replace the `<tbody>` body to render groups + solos**

The existing `<tbody>` maps `filtered.map((experiment) => ...)`. Replace that whole map with one that walks `grouped` and emits a group header row (for `'group'` units) followed by the children (when expanded), or a single row (for `'solo'` units):

```tsx
            <tbody>
              {grouped.map((unit) => {
                if (unit.kind === 'solo') {
                  return <ExperimentRow key={unit.experiment.id} experiment={unit.experiment} />;
                }
                const expanded = expandedBatches[unit.batchId] ?? false;
                const counts = countByStatus(unit.experiments);
                return (
                  <GroupBlock
                    key={unit.batchId}
                    unit={unit}
                    expanded={expanded}
                    counts={counts}
                    onToggle={() =>
                      setExpandedBatches((prev) => ({ ...prev, [unit.batchId]: !expanded }))
                    }
                  />
                );
              })}
            </tbody>
```

- [ ] **Step 8: Extract `ExperimentRow` and add `GroupBlock` + `countByStatus`**

Append to the file:

```tsx
function countByStatus(experiments: Experiment[]): Record<string, number> {
  const out: Record<string, number> = {};
  for (const e of experiments) {
    out[e.status] = (out[e.status] ?? 0) + 1;
  }
  return out;
}

function statusSummary(counts: Record<string, number>, total: number): string {
  const completed = counts['completed'] ?? 0;
  if (completed === total) return `${total}/${total} completed`;
  const parts: string[] = [];
  if (counts['running']) parts.push(`${counts['running']} running`);
  if ((counts['draft'] ?? 0) + (counts['queued'] ?? 0)) {
    parts.push(`${(counts['draft'] ?? 0) + (counts['queued'] ?? 0)} queued`);
  }
  if (completed) parts.push(`${completed} completed`);
  if (counts['failed']) parts.push(`${counts['failed']} failed`);
  return parts.length ? parts.join(' · ') : `${total} experiments`;
}

function ExperimentRow({ experiment, nested = false }: { experiment: Experiment; nested?: boolean }) {
  const cost = experiment.estimated_cost_usd ?? 0;
  return (
    <tr className={`border-t border-border ${nested ? 'bg-bg-elev-1/60' : 'bg-bg-elev-1'} hover:bg-bg-elev-2/60`}>
      <td className={`px-4 py-3 ${nested ? 'pl-10' : ''}`}>
        <div className="font-medium text-fg">{experiment.name}</div>
        {experiment.description && (
          <div className="mt-0.5 line-clamp-1 text-xs text-fg-muted">{experiment.description}</div>
        )}
      </td>
      <td className="px-4 py-3">
        <Badge tone={statusTone(experiment.status)}>{statusLabel(experiment.status)}</Badge>
      </td>
      <td className="px-4 py-3 text-fg-muted">
        {experiment.agent_cli}
        <span className="text-fg-subtle"> · </span>
        {experiment.model}
      </td>
      <td className="px-4 py-3">
        <div className="text-fg">{runsLabel(experiment)}</div>
        {cost > 0 && <div className="text-xs text-fg-subtle">~{formatCurrency(cost)}</div>}
      </td>
      <td className="px-4 py-3 text-right text-fg-muted">
        <div className="flex items-center justify-end gap-3">
          <span>{formatTimeAgo(experiment.created_at)}</span>
          <Link to={`/experiments/${experiment.id}/monitor`} className="text-fg-muted hover:text-fg">
            Open →
          </Link>
        </div>
      </td>
    </tr>
  );
}

function GroupBlock({
  unit,
  expanded,
  counts,
  onToggle,
}: {
  unit: Extract<GroupedExperiment, { kind: 'group' }>;
  expanded: boolean;
  counts: Record<string, number>;
  onToggle: () => void;
}) {
  const summary = statusSummary(counts, unit.experiments.length);
  const newest = unit.experiments[0];
  return (
    <>
      <tr className="border-t border-border bg-bg-elev-2 hover:bg-bg-elev-2/80">
        <td colSpan={5} className="px-4 py-2">
          <button
            type="button"
            onClick={onToggle}
            className="flex w-full items-center justify-between text-left"
          >
            <div className="flex items-center gap-2">
              <span className={`text-fg-muted transition ${expanded ? 'rotate-90' : ''}`}>▶</span>
              <span className="font-medium text-fg">{unit.batchLabel}</span>
              <span className="text-xs text-fg-subtle">·</span>
              <span className="text-xs text-fg-muted">{unit.experiments.length} experiments</span>
              <span className="text-xs text-fg-subtle">·</span>
              <span className="text-xs text-fg-muted">{summary}</span>
            </div>
            <span className="text-xs text-fg-subtle">{formatTimeAgo(newest.created_at)}</span>
          </button>
        </td>
      </tr>
      {expanded &&
        unit.experiments.map((experiment) => (
          <ExperimentRow key={experiment.id} experiment={experiment} nested />
        ))}
    </>
  );
}
```

`Variant` needs a `harness_id` field accessible via TypeScript. If the existing `Variant` type doesn't expose `harness_id`, peek at `frontend/src/lib/types.ts` for the `Variant` shape and either use the field name that exists (likely `harness_id`) or fall back to `v.name` which is also a string.

- [ ] **Step 9: Add a scroll-to-focus-batch effect**

Right after the existing initializer effect from Step 3, append:

```tsx
  useEffect(() => {
    if (!focusBatch) return;
    const el = document.querySelector(`[data-batch-id="${focusBatch}"]`);
    if (el && 'scrollIntoView' in el) {
      el.scrollIntoView({ behavior: 'smooth', block: 'start' });
    }
  }, [focusBatch, grouped]);
```

And in `GroupBlock`'s outer `<tr>` add a `data-batch-id={unit.batchId}` attribute so the query selector resolves.

- [ ] **Step 10: Typecheck + tests + dev sanity**

```bash
cd frontend && npm run lint && npm test -- --run 2>&1 | tail -10
```

Expected: lint exits 0; tests all pass (including the new `grouping.test.ts`).

- [ ] **Step 11: Commit**

```bash
git add frontend/src/pages/experiments/index.tsx
git commit -m "Render experiment list with batch groups, sticky filter, harness-aware runs label"
```

---

## Task 9 — Diagnostic launcher: multi-task picker

**Files:**
- Modify: `frontend/src/pages/diagnostic/launch.tsx`

- [ ] **Step 1: Convert single `taskID` to `taskIDs[]`**

In `frontend/src/pages/diagnostic/launch.tsx`, replace:

```tsx
  const initialTask = searchParams.get('task') ?? '';
  const [taskID, setTaskID] = useState(initialTask);
```

with:

```tsx
  const initialTask = searchParams.get('task') ?? '';
  const [taskIDs, setTaskIDs] = useState<string[]>(initialTask ? [initialTask] : []);
```

And replace every other `taskID` usage in the file: the existing tasks-list onClick handler should toggle membership rather than set a single value:

```tsx
  const toggleTask = (id: string) => {
    setTaskIDs((prev) => (prev.includes(id) ? prev.filter((t) => t !== id) : [...prev, id]));
  };
```

Wire it into the same UI block that currently selects a single task — change the `onClick={() => setTaskID(task.id)}` to `onClick={() => toggleTask(task.id)}` and change the "is selected" check from `taskID === task.id` to `taskIDs.includes(task.id)`.

- [ ] **Step 2: Add the suite label input (only when ≥2 selected)**

Below the task chip list, add:

```tsx
  {taskIDs.length >= 2 && (
    <div className="mt-3">
      <label className="block text-xs uppercase tracking-wider text-fg-muted">Suite label</label>
      <Input
        value={suiteLabel}
        onChange={(e) => setSuiteLabel(e.target.value)}
        placeholder="e.g. FMT calibration v6"
        className="mt-1 w-full"
      />
    </div>
  )}
```

And add state for it next to `taskIDs`:

```tsx
  const [suiteLabel, setSuiteLabel] = useState('');
```

- [ ] **Step 3: Bring in the suite launch hook**

Add to the existing import from `../../lib/hooks`:

```tsx
  useLaunchDiagnosticSuite,
```

And in the body:

```tsx
  const launchSuite = useLaunchDiagnosticSuite();
```

- [ ] **Step 4: Branch the submit handler**

Find the existing handler that calls `launch.mutate(...)` (or similar). Replace it with a branching version:

```tsx
  const handleLaunch = async () => {
    if (taskIDs.length === 0) {
      setPartialError('Pick at least one task.');
      return;
    }
    setPartialError(null);
    const baseFields = {
      executor_id: selectedExecutors[0],
      harness_ids: selectedHarnesses,
      model: selectedModels[0],
      runs_per_variant: runsPerVariant,
    };
    if (taskIDs.length === 1) {
      const res = await launch.mutateAsync({
        ...baseFields,
        task_id: taskIDs[0],
        name: name.trim() || undefined,
      });
      navigate(`/diagnostic/compare?experiment=${res.experiment_id}`);
      return;
    }
    const res = await launchSuite.mutateAsync({
      ...baseFields,
      task_ids: taskIDs,
      batch_label: suiteLabel.trim() || undefined,
    });
    if (res.failures && res.failures.length > 0) {
      const summary = `Started ${res.experiment_ids.length}/${taskIDs.length}. Failures: ${res.failures.map((f) => f.task_id).join(', ')}`;
      setPartialError(summary);
    }
    navigate(`/experiments?batch=${res.batch_id}`);
  };
```

Match the existing UI button's `onClick` to point at `handleLaunch`. Update the disabled-state check to use `taskIDs.length === 0` instead of `!taskID`.

- [ ] **Step 5: Update the variant counter**

Find the existing counter label that reads `harness × executor × model × runs`. Multiply by `taskIDs.length` and relabel "experiments to launch":

```tsx
  const totalExperiments =
    Math.max(taskIDs.length, 1) *
    Math.max(selectedHarnesses.length, 1) *
    Math.max(selectedExecutors.length, 1) *
    Math.max(selectedModels.length, 1);
  // …display: `{totalExperiments} experiments × {runsPerVariant} runs each`
```

(Use whatever wording the existing counter already uses — only the multiplication and the label change.)

- [ ] **Step 6: Typecheck**

```bash
cd frontend && npm run lint
```

Expected: exits 0.

- [ ] **Step 7: Commit**

```bash
git add frontend/src/pages/diagnostic/launch.tsx
git commit -m "Diagnostic launcher: multi-task picker, suite endpoint, batch redirect"
```

---

## Task 10 — Full sanity pass

**Files:** none

- [ ] **Step 1: Lint, build, test, tokens**

```bash
cd frontend && npm run lint && npm run build && npm test -- --run 2>&1 | tail -10 && npm run check:tokens
```

```bash
cd engine && go test ./...
```

Expected: all four frontend commands exit 0, every Go package green.

- [ ] **Step 2: Manual walkthrough**

1. Start engine + frontend:
   ```bash
   cd engine && go run cmd/server/main.go &
   cd ../frontend && npm run dev
   ```
2. Browse to `/diagnostic/launch`. Pick 3 tasks, fill harness (`bare`) + executor + model + suite label `"Sanity batch"` → Launch.
3. URL redirects to `/experiments?batch=<uuid>`. The new group appears at the top, expanded (≤3 children rule). Group header reads `"Sanity batch · 3 experiments · …"`.
4. Pick 1 task, launch → URL redirects to `/diagnostic/compare?experiment=<id>` as today (no batch).
5. CLI suite tag: run a quick `curl` to confirm pass-through works:
   ```bash
   BATCH=$(uuidgen)
   for tid in brownfield-misread-hidden-contract brownfield-hal-api-pydantic-version; do
     curl -s -X POST http://localhost:8080/api/diagnostic/launch \
       -H 'Content-Type: application/json' \
       -d "{\"task_id\":\"$tid\",\"executor_id\":\"opencode\",\"harness_ids\":[\"bare\"],\"model\":\"opencode/deepseek-v4-flash-free\",\"runs_per_variant\":1,\"batch_id\":\"$BATCH\",\"batch_label\":\"CLI suite\"}"
   done
   ```
   Refresh `/experiments` — two new rows under a single `"CLI suite"` group.

- [ ] **Step 3: Stop the dev server + engine**

Ctrl-C both. No commit; manual verification is a gate, not a delivery.

---

## Task 11 — Push branch, open PR, request review, watch CI

**Files:** none

- [ ] **Step 1: Push**

```bash
git push -u origin feature/experiment-batches
```

- [ ] **Step 2: Open PR**

```bash
gh pr create --title "Experiment batches: group launched-together runs" --body "$(cat <<'EOF'
## Summary

Lets the user launch multiple experiments in one diagnostic run (or
script-loop) and see them as one group in the Experiments list,
instead of N orphan rows.

- Backend: migration 019 adds nullable \`batch_id\` + \`batch_label\` columns to \`experiments\`
- Backend: existing \`POST /api/diagnostic/launch\` accepts optional \`batch_id\`/\`batch_label\` pass-through (lets shell scripts tag a batch with one UUID across N calls)
- Backend: new \`POST /api/diagnostic/launch-suite\` takes \`task_ids\`, mints a single \`batch_id\`, creates N experiments under it, tolerates per-task failures via a \`failures\` array
- Frontend: Diagnostic launcher's task picker becomes multi-select; 1 → existing endpoint, ≥2 → suite endpoint, redirects to \`/experiments?batch=<id>\`
- Frontend: Experiments list groups rows by \`batch_id\` into a collapsible block (≤3 children expanded by default, URL-focused batch always expanded + scrolled into view)
- Frontend: sticky filter row, harness-aware runs label (\`1 run · bare\` instead of \`0v × 1r\`), \"New diagnostic run\" button removed from the in-table toolbar (sidebar still has it)
- Pre-existing experiments stay ungrouped (no backfill — out of scope per spec)

Spec: [\`docs/superpowers/specs/2026-05-31-experiment-batches-design.md\`](docs/superpowers/specs/2026-05-31-experiment-batches-design.md)
Plan: [\`docs/superpowers/plans/2026-05-31-experiment-batches.md\`](docs/superpowers/plans/2026-05-31-experiment-batches.md)

## Test plan

- [x] Go: \`go test ./...\` clean (new repo + handler tests)
- [x] Frontend: \`npm run lint\`, \`npm run build\`, \`npm test\`, \`npm run check:tokens\` all clean
- [x] Manual: multi-task launch → batch group appears + auto-expand; single launch → unchanged compare redirect; CLI \`curl\` pass-through tags a batch
EOF
)"
```

- [ ] **Step 3: Dispatch the project's code reviewer subagent on the branch HEAD**

(Per memory `feedback_github_workflow`: every non-trivial PR goes through `feature-dev:code-reviewer`.)

- [ ] **Step 4: Monitor CI**

Watch checks until all pass (or fix-and-push for any failure).

- [ ] **Step 5: Merge once green**

```bash
gh pr merge <PR#> --squash --delete-branch
```

---

## Self-review

**Spec coverage:**
- Schema migration → Task 1
- Go model fields → Task 2
- Repo CRUD round-trip → Task 3
- Existing endpoint pass-through → Task 4
- Suite endpoint → Task 5
- Frontend types + hook → Task 6
- Grouping logic + tests → Task 7
- Experiments list rendering (groups, sticky filter, runs label, button drop) → Task 8
- Launcher multi-task picker + branching submit → Task 9
- Sanity + manual → Task 10
- PR + review + CI + merge → Task 11
- Out-of-scope items (cross-task aggregation, batch re-run, backfill) — explicitly excluded by spec; no tasks needed.

**No placeholders** — every step has runnable commands, exact file paths, and complete code blocks.

**Type consistency** — `BatchID`/`BatchLabel` Go field names match the JSON tags `batch_id`/`batch_label` and the frontend types and the SQL column names. `GroupedExperiment` discriminated union (`kind: 'solo' | 'group'`) is used identically in Tasks 7 and 8. `runsLabel(experiment)` helper signature is referenced consistently. The `Variant` type's `harness_id` field is checked in Task 8 step 8 before use; a fallback to `v.name` is documented.

**Ordering** — Schema → Go model → repo → API → frontend types → frontend logic → frontend pages → sanity → PR. Each step is testable on its own; nothing references a not-yet-built dependency.
