# Spec-kit extension as intra-experiment variant Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Move spec-kit extension from a cross-experiment matrix axis to an intra-experiment variant dimension, so `1 task × 1 exec × 1 model × {4 non-speckit harnesses + 6 spec-kit extensions}` produces **one experiment with 10 variants / 10 runs**, not 6 experiments × 5 variants.

**Architecture:** Backend gains an additive `variants []LaunchVariant` field on the launch request (each variant carries its own harness id + config); when present it replaces the legacy `harness_ids` + shared-config path. Per-variant config already persists and flows to the harness at run time (PR #145), so storage/orchestrator are untouched. Frontend reverts the matrix to 3 axes (task × exec × model), builds the intra-experiment variant list via a pure helper (non-speckit harnesses + one speckit variant per selected extension), and sends it. Compare and the Experiments list are fixed to label/count per variant.

**Tech Stack:** Go 1.22 (stdlib `database/sql`, Chi, no new deps), React 18 + TypeScript + TanStack Query + Vitest.

**Spec:** [`docs/superpowers/specs/2026-06-02-extension-as-variant-rework-design.md`](../specs/2026-06-02-extension-as-variant-rework-design.md)

**Branch:** `feature/extension-as-variant-rework` (created; spec committed at `a3aa64a`).

---

## File map

| Layer | File | Action |
|---|---|---|
| Launch API | `engine/internal/api/diagnostic_launch.go` | MODIFY (`LaunchVariant` type + `variants` field + handler branch) |
| Launch API tests | `engine/internal/api/diagnostic_launch_test.go` | MODIFY (append 3 cases) |
| Experiment repo | `engine/internal/storage/variant_repo.go` | MODIFY (add `ListVariantsByExperiments` batched loader) |
| Experiment repo | `engine/internal/storage/experiment_repo.go` | MODIFY (`ListExperiments` attaches variants) |
| Experiment repo tests | `engine/internal/storage/experiment_repo_test.go` | MODIFY (append 1 case) |
| Frontend types | `frontend/src/lib/types.ts` | MODIFY (`LaunchVariant` + `LaunchDiagnosticRequest.variants`) |
| Variant builder | `frontend/src/pages/diagnostic/build-variants.ts` | CREATE |
| Variant builder tests | `frontend/src/pages/diagnostic/build-variants.test.ts` | CREATE |
| Matrix helper | `frontend/src/pages/diagnostic/launch-matrix.ts` | MODIFY (drop speckit axis → 3 axes) |
| Matrix tests | `frontend/src/pages/diagnostic/launch-matrix.test.ts` | MODIFY (drop speckit-axis cases) |
| Launcher page | `frontend/src/pages/diagnostic/launch.tsx` | MODIFY (variant memo, totalRuns, handleLaunch, preview) |
| Compare page | `frontend/src/pages/diagnostic/compare.tsx` | MODIFY (per-variant label) |

No schema migration.

---

## Task 1 — Backend: `LaunchVariant` + per-variant launch path (TDD)

**Files:**
- Modify: `engine/internal/api/diagnostic_launch.go`
- Modify: `engine/internal/api/diagnostic_launch_test.go`

- [ ] **Step 1: Append failing tests**

Append to `engine/internal/api/diagnostic_launch_test.go`:

```go
func TestLaunchWithVariantsArrayCreatesPerVariantConfig(t *testing.T) {
	svc := newLaunchTestService(t)

	body, _ := json.Marshal(map[string]any{
		"task_id":     "t-launch",
		"executor_id": "opencode",
		"model":       "anything",
		"variants": []map[string]any{
			{"harness_id": "speckit", "name": "speckit/canonical",
				"harness_config": map[string]any{"speckit": map[string]any{"extension_id": "canonical"}}},
			{"harness_id": "speckit", "name": "speckit/lite",
				"harness_config": map[string]any{"speckit": map[string]any{"extension_id": "lite"}}},
		},
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
		t.Fatalf("decode: %v", err)
	}
	exp, err := svc.store.GetExperiment(context.Background(), resp.ExperimentID)
	if err != nil {
		t.Fatalf("fetch experiment: %v", err)
	}
	if len(exp.Variants) != 2 {
		t.Fatalf("variant count: got %d want 2", len(exp.Variants))
	}
	// Each variant keeps its own extension_id, AND both are the speckit harness.
	gotExt := map[string]string{}
	for _, v := range exp.Variants {
		if v.HarnessID != "speckit" {
			t.Errorf("variant %q harness: got %q want speckit", v.Name, v.HarnessID)
		}
		sub, _ := v.HarnessConfig["speckit"].(map[string]any)
		ext, _ := sub["extension_id"].(string)
		gotExt[v.Name] = ext
	}
	if gotExt["speckit/canonical"] != "canonical" || gotExt["speckit/lite"] != "lite" {
		t.Errorf("per-variant extension_id wrong: %v", gotExt)
	}
}

func TestLaunchVariantsRejectsUnknownHarness(t *testing.T) {
	svc := newLaunchTestService(t)
	body, _ := json.Marshal(map[string]any{
		"task_id":     "t-launch",
		"executor_id": "opencode",
		"model":       "anything",
		"variants": []map[string]any{
			{"harness_id": "does-not-exist", "name": "x"},
		},
	})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/diagnostic/launch", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	svc.LaunchDiagnostic(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d want 400 body=%s", rec.Code, rec.Body.String())
	}
}

func TestLaunchVariantsEmptyFallsBackToHarnessIds(t *testing.T) {
	svc := newLaunchTestService(t)
	body, _ := json.Marshal(map[string]any{
		"task_id":     "t-launch",
		"executor_id": "opencode",
		"model":       "anything",
		"harness_ids": []string{"bare"},
		"variants":    []map[string]any{}, // empty → legacy path
	})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/diagnostic/launch", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	svc.LaunchDiagnostic(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status: got %d want 202 body=%s", rec.Code, rec.Body.String())
	}
	var resp LaunchDiagnosticResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	exp, _ := svc.store.GetExperiment(context.Background(), resp.ExperimentID)
	if len(exp.Variants) != 1 || exp.Variants[0].HarnessID != "bare" {
		t.Errorf("legacy fallback variants wrong: %+v", exp.Variants)
	}
}
```

`newLaunchTestService` already exists in this file (registers `bare`, `ralph`; executor `opencode`). Add `speckit` to its harness registry if it isn't already registered — check the helper and, if needed, update the `support.NewStaticHarnessRegistry(...)` call to include `"speckit"`.

- [ ] **Step 2: Run; tests must fail**

```bash
cd engine && go test ./internal/api/ -run 'TestLaunchWithVariants|TestLaunchVariants' -v
```
Expected: compile error / failures — the `variants` field doesn't exist yet.

- [ ] **Step 3: Add the `LaunchVariant` type + request field**

In `engine/internal/api/diagnostic_launch.go`, add the type and extend the request:

```go
// LaunchVariant is one explicit intra-experiment variant: a harness with
// its own per-variant config. Used by the launcher to put N spec-kit
// extensions (and the other harnesses) into a SINGLE experiment, each
// carrying its own harness_config, rather than crossing extensions with
// non-speckit harnesses across separate experiments.
type LaunchVariant struct {
	HarnessID     string         `json:"harness_id"`
	Name          string         `json:"name"`
	HarnessConfig map[string]any `json:"harness_config,omitempty"`
}

type LaunchDiagnosticRequest struct {
	TaskID         string          `json:"task_id"`
	ExecutorID     string          `json:"executor_id"`
	HarnessIDs     []string        `json:"harness_ids"`
	Model          string          `json:"model"`
	RunsPerVariant int             `json:"runs_per_variant"`
	TimeoutSeconds int             `json:"timeout_seconds"`
	Name           string          `json:"name"`
	BatchID        string          `json:"batch_id"`
	BatchLabel     string          `json:"batch_label"`
	HarnessConfigs map[string]any  `json:"harness_configs,omitempty"`
	Variants       []LaunchVariant `json:"variants,omitempty"`
}
```

- [ ] **Step 4: Branch the handler's validation + variant build**

In `LaunchDiagnostic`, replace the harness-id validation block AND the variant-build block. The current code (around lines 55-99) validates `harness_ids` then builds `variants` from them. Replace with a branch:

Replace this:
```go
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
```
with:
```go
	useVariants := len(req.Variants) > 0
	if !useVariants && len(req.HarnessIDs) == 0 {
		renderError(w, r.Context(), http.StatusBadRequest, ErrCodeBadRequest, "harness_ids or variants required", nil)
		return
	}
	// Validate every harness id referenced (either path).
	if useVariants {
		for _, v := range req.Variants {
			if _, err := s.harnesses.Get(v.HarnessID); err != nil {
				renderError(w, r.Context(), http.StatusBadRequest, ErrCodeBadRequest, fmt.Sprintf("unknown harness %q", v.HarnessID), err)
				return
			}
		}
	} else {
		for _, hid := range req.HarnessIDs {
			if _, err := s.harnesses.Get(hid); err != nil {
				renderError(w, r.Context(), http.StatusBadRequest, ErrCodeBadRequest, fmt.Sprintf("unknown harness %q", hid), err)
				return
			}
		}
	}
```

Then replace the variant-build block:
```go
	variants := make([]models.VariantRequest, 0, len(req.HarnessIDs))
	for idx, hid := range req.HarnessIDs {
		variants = append(variants, models.VariantRequest{
			Name:          hid,
			Description:   fmt.Sprintf("Harness: %s", hid),
			IsControl:     idx == 0,
			Ordering:      idx,
			HarnessID:     hid,
			HarnessConfig: req.HarnessConfigs,
		})
	}
```
with:
```go
	var variants []models.VariantRequest
	if useVariants {
		variants = make([]models.VariantRequest, 0, len(req.Variants))
		for idx, v := range req.Variants {
			name := v.Name
			if name == "" {
				name = v.HarnessID
			}
			variants = append(variants, models.VariantRequest{
				Name:          name,
				Description:   fmt.Sprintf("Harness: %s", v.HarnessID),
				IsControl:     idx == 0,
				Ordering:      idx,
				HarnessID:     v.HarnessID,
				HarnessConfig: v.HarnessConfig,
			})
		}
	} else {
		variants = make([]models.VariantRequest, 0, len(req.HarnessIDs))
		for idx, hid := range req.HarnessIDs {
			variants = append(variants, models.VariantRequest{
				Name:          hid,
				Description:   fmt.Sprintf("Harness: %s", hid),
				IsControl:     idx == 0,
				Ordering:      idx,
				HarnessID:     hid,
				HarnessConfig: req.HarnessConfigs,
			})
		}
	}
```

Also update the experiment `Description` string that references `len(req.HarnessIDs)` so it doesn't say "0 harness(es)" on the variants path. Change:
```go
		Description:    fmt.Sprintf("Diagnostic launcher: %d harness(es), executor=%s", len(req.HarnessIDs), req.ExecutorID),
```
to:
```go
		Description:    fmt.Sprintf("Diagnostic launcher: %d variant(s), executor=%s", len(variants), req.ExecutorID),
```

- [ ] **Step 5: Re-run the new tests; must pass**

```bash
cd engine && go test ./internal/api/ -run 'TestLaunchWithVariants|TestLaunchVariants' -v
```
Expected: all 3 PASS.

- [ ] **Step 6: Run the full engine suite**

```bash
cd engine && go test ./...
```
Expected: all packages green (existing legacy launch tests still pass via the fallback path).

- [ ] **Step 7: Commit**

```bash
git add engine/internal/api/diagnostic_launch.go engine/internal/api/diagnostic_launch_test.go engine/test/support/
git commit -m "launch: accept explicit per-variant config via variants[] (additive)"
```
(Only `git add engine/test/support/` if you had to register speckit in the static harness registry.)

---

## Task 2 — Backend: `ListExperiments` returns variants (TDD)

**Files:**
- Modify: `engine/internal/storage/variant_repo.go`
- Modify: `engine/internal/storage/experiment_repo.go`
- Modify: `engine/internal/storage/experiment_repo_test.go`

- [ ] **Step 1: Append the failing test**

Append to `engine/internal/storage/experiment_repo_test.go`:

```go
func TestListExperimentsIncludesVariants(t *testing.T) {
	store := support.TmpStore(t)
	seedTaskForExperimentTest(t, store, "task-listvar")

	_, err := store.CreateExperiment(context.Background(), models.ExperimentRequest{
		Name:           "with-variants",
		TaskID:         "task-listvar",
		Model:          "m",
		AgentCLI:       "opencode",
		RunsPerVariant: 1,
		Variants: []models.VariantRequest{
			{Name: "bare", HarnessID: "bare", Ordering: 0},
			{Name: "speckit/canonical", HarnessID: "speckit", Ordering: 1},
			{Name: "speckit/lite", HarnessID: "speckit", Ordering: 2},
		},
	})
	if err != nil {
		t.Fatalf("CreateExperiment: %v", err)
	}

	listed, err := store.ListExperiments(context.Background())
	if err != nil {
		t.Fatalf("ListExperiments: %v", err)
	}
	var found *models.Experiment
	for i := range listed {
		if listed[i].Name == "with-variants" {
			found = &listed[i]
			break
		}
	}
	if found == nil {
		t.Fatal("experiment not in list")
	}
	if len(found.Variants) != 3 {
		t.Fatalf("variant count in list: got %d want 3", len(found.Variants))
	}
	// Ordered by `ordering`, names preserved.
	if found.Variants[0].Name != "bare" || found.Variants[2].Name != "speckit/lite" {
		t.Errorf("variant order/names wrong: %+v", found.Variants)
	}
}
```

- [ ] **Step 2: Run; must fail**

```bash
cd engine && go test ./internal/storage/ -run TestListExperimentsIncludesVariants -v
```
Expected: FAIL — `found.Variants` is empty (ListExperiments doesn't load them).

- [ ] **Step 3: Add a batched variant loader**

In `engine/internal/storage/variant_repo.go`, add (after `ListVariantsByExperiment`):

```go
// ListVariantsByExperiments batch-loads variants for many experiments in
// one query and groups them by experiment_id. Unlike
// ListVariantsByExperiment it does NOT eager-load artifact versions —
// callers that need only variant identity (the Experiments list's run
// label) skip that extra per-variant query. Returns a map keyed by
// experiment_id; experiments with no variants are simply absent.
func (s *Store) ListVariantsByExperiments(ctx context.Context, experimentIDs []string) (map[string][]models.Variant, error) {
	out := make(map[string][]models.Variant)
	if len(experimentIDs) == 0 {
		return out, nil
	}
	placeholders := make([]string, len(experimentIDs))
	args := make([]any, len(experimentIDs))
	for i, id := range experimentIDs {
		placeholders[i] = "?"
		args[i] = id
	}
	query := `SELECT id, experiment_id, name, description, is_control, ordering, harness_id, harness_config_json
		FROM variants WHERE experiment_id IN (` + strings.Join(placeholders, ",") + `) ORDER BY experiment_id, ordering ASC`
	rows, err := s.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list variants by experiments: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		variant, scanErr := scanVariant(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		out[variant.ExperimentID] = append(out[variant.ExperimentID], variant)
	}
	return out, rows.Err()
}
```

Add `"strings"` to the import block of `variant_repo.go` if it isn't already there (check the top of the file).

- [ ] **Step 4: Wire it into `ListExperiments`**

In `engine/internal/storage/experiment_repo.go`, replace the `ListExperiments` body's tail (after the rows loop, before `return`):

Current:
```go
	experiments := make([]models.Experiment, 0)
	for rows.Next() {
		experiment, scanErr := scanExperiment(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		experiments = append(experiments, experiment)
	}
	return experiments, rows.Err()
}
```
Replace with:
```go
	experiments := make([]models.Experiment, 0)
	for rows.Next() {
		experiment, scanErr := scanExperiment(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		experiments = append(experiments, experiment)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	// Attach variants in one batched query so the Experiments list can
	// render the per-experiment variant count + harness names without an
	// N+1 round-trip.
	ids := make([]string, len(experiments))
	for i := range experiments {
		ids[i] = experiments[i].ID
	}
	byExp, err := s.ListVariantsByExperiments(ctx, ids)
	if err != nil {
		return nil, err
	}
	for i := range experiments {
		experiments[i].Variants = byExp[experiments[i].ID]
	}
	return experiments, nil
}
```

The `defer rows.Close()` is already present; closing it before the second query runs is fine because we've fully drained `rows` in the loop. (Go's database/sql allows a new query on the same DB while a closed/drained rows exists.)

- [ ] **Step 5: Re-run the test; must pass**

```bash
cd engine && go test ./internal/storage/ -run TestListExperimentsIncludesVariants -v
```
Expected: PASS.

- [ ] **Step 6: Full engine suite**

```bash
cd engine && go test ./...
```
Expected: green.

- [ ] **Step 7: Commit**

```bash
git add engine/internal/storage/variant_repo.go engine/internal/storage/experiment_repo.go engine/internal/storage/experiment_repo_test.go
git commit -m "storage: ListExperiments eagerly attaches variants (batched, no N+1)"
```

---

## Task 3 — Frontend types

**Files:**
- Modify: `frontend/src/lib/types.ts`

- [ ] **Step 1: Add `LaunchVariant` + extend the request**

In `frontend/src/lib/types.ts`, find `export type LaunchDiagnosticRequest`. Add a new type above it and a field to it:

```ts
export type LaunchVariant = {
  harness_id: string;
  name: string;
  harness_config?: Record<string, unknown>;
};
```

Add to `LaunchDiagnosticRequest`:
```ts
  variants?: LaunchVariant[];
```

- [ ] **Step 2: Typecheck**

```bash
cd frontend && npm run lint
```
Expected: exits 0.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/lib/types.ts
git commit -m "Add LaunchVariant type + variants field on LaunchDiagnosticRequest"
```

---

## Task 4 — Frontend: `buildLaunchVariants` pure helper (TDD)

**Files:**
- Create: `frontend/src/pages/diagnostic/build-variants.ts`
- Create: `frontend/src/pages/diagnostic/build-variants.test.ts`

- [ ] **Step 1: Write the failing test**

Create `frontend/src/pages/diagnostic/build-variants.test.ts`:

```ts
import { describe, expect, it } from 'vitest';
import { buildLaunchVariants } from './build-variants';

describe('buildLaunchVariants', () => {
  it('emits one variant per non-speckit harness with its own config block', () => {
    const out = buildLaunchVariants(
      ['bare', 'agent_instructions', 'multiagent', 'ralph'],
      {
        agent_instructions: { content: 'rules' },
        multiagent: { roles: [{ name: 'planner', prompt: 'p' }] },
      },
      [],
    );
    expect(out.map((v) => v.name)).toEqual(['bare', 'agent_instructions', 'multiagent', 'ralph']);
    expect(out.every((v) => v.harness_id === v.name)).toBe(true);
    // bare / ralph carry no config; the configured ones carry only their own block.
    expect(out[0].harness_config).toBeUndefined();
    expect(out[1].harness_config).toEqual({ agent_instructions: { content: 'rules' } });
    expect(out[2].harness_config).toEqual({ multiagent: { roles: [{ name: 'planner', prompt: 'p' }] } });
    expect(out[3].harness_config).toBeUndefined();
  });

  it('emits one speckit variant per extension, named speckit/<ext>', () => {
    const out = buildLaunchVariants(['speckit'], {}, ['canonical', 'lite', 'dual-role']);
    expect(out).toEqual([
      { harness_id: 'speckit', name: 'speckit/canonical', harness_config: { speckit: { extension_id: 'canonical' } } },
      { harness_id: 'speckit', name: 'speckit/lite', harness_config: { speckit: { extension_id: 'lite' } } },
      { harness_id: 'speckit', name: 'speckit/dual-role', harness_config: { speckit: { extension_id: 'dual-role' } } },
    ]);
  });

  it('mixes non-speckit harnesses and speckit extensions in selection order', () => {
    const out = buildLaunchVariants(
      ['bare', 'speckit', 'ralph'],
      {},
      ['canonical', 'lite'],
    );
    expect(out.map((v) => v.name)).toEqual([
      'bare', 'speckit/canonical', 'speckit/lite', 'ralph',
    ]);
  });

  it('emits no speckit variants when speckit is selected but no extensions chosen', () => {
    const out = buildLaunchVariants(['bare', 'speckit'], {}, []);
    expect(out.map((v) => v.name)).toEqual(['bare']);
  });
});
```

- [ ] **Step 2: Run; must fail (module missing)**

```bash
cd frontend && npx vitest run src/pages/diagnostic/build-variants.test.ts
```
Expected: "Cannot find module './build-variants'".

- [ ] **Step 3: Implement the helper**

Create `frontend/src/pages/diagnostic/build-variants.ts`:

```ts
import type { LaunchVariant } from '../../lib/types';

/**
 * Build the intra-experiment variant list from the launcher's harness +
 * spec-kit-extension selection.
 *
 * Each non-speckit harness becomes exactly one variant carrying only its
 * own config block. The spec-kit harness, when selected, fans out into
 * one variant per chosen extension (named `speckit/<ext>`), each carrying
 * `{ speckit: { extension_id } }`. We do NOT cross non-speckit harnesses
 * with extensions — extensions are variants of the spec-kit harness only.
 *
 * Variants appear in `selectedHarnesses` order; spec-kit's slot expands
 * in place into its extension variants.
 */
export function buildLaunchVariants(
  selectedHarnesses: string[],
  harnessConfigs: Record<string, unknown>,
  speckitExtensions: string[],
): LaunchVariant[] {
  const out: LaunchVariant[] = [];
  for (const h of selectedHarnesses) {
    if (h === 'speckit') {
      for (const ext of speckitExtensions) {
        out.push({
          harness_id: 'speckit',
          name: `speckit/${ext}`,
          harness_config: { speckit: { extension_id: ext } },
        });
      }
      continue;
    }
    const cfg = configForHarness(h, harnessConfigs);
    out.push({ harness_id: h, name: h, harness_config: cfg });
  }
  return out;
}

// Return only the harness's own config block, or undefined when the
// harness needs none (bare, ralph) or the user hasn't supplied it yet.
function configForHarness(
  harnessId: string,
  harnessConfigs: Record<string, unknown>,
): Record<string, unknown> | undefined {
  const block = harnessConfigs[harnessId];
  if (block === undefined || block === null) return undefined;
  return { [harnessId]: block };
}
```

- [ ] **Step 4: Re-run; must pass**

```bash
cd frontend && npx vitest run src/pages/diagnostic/build-variants.test.ts
```
Expected: 4/4 PASS.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/pages/diagnostic/build-variants.ts frontend/src/pages/diagnostic/build-variants.test.ts
git commit -m "Add buildLaunchVariants pure helper (harnesses + speckit extensions → variant list)"
```

---

## Task 5 — Frontend: matrix back to 3 axes (TDD)

**Files:**
- Modify: `frontend/src/pages/diagnostic/launch-matrix.ts`
- Modify: `frontend/src/pages/diagnostic/launch-matrix.test.ts`

- [ ] **Step 1: Replace the matrix test file with the 3-axis version**

Overwrite `frontend/src/pages/diagnostic/launch-matrix.test.ts` with:

```ts
import { describe, expect, it } from 'vitest';
import { countExperiments, expandLaunchMatrix } from './launch-matrix';

describe('countExperiments', () => {
  it('returns 1 when every dimension is empty (so the gate stays sane)', () => {
    expect(countExperiments({ taskIds: [], executorIds: [], modelIds: [] })).toBe(1);
  });

  it('multiplies tasks × executors × models', () => {
    expect(countExperiments({ taskIds: ['a', 'b'], executorIds: ['x'], modelIds: ['m1', 'm2'] })).toBe(4);
  });
});

describe('expandLaunchMatrix', () => {
  it('yields one cell per (task × executor × model) combination', () => {
    const cells = expandLaunchMatrix({
      taskIds: ['t1', 't2'],
      executorIds: ['e1'],
      modelIds: ['m1', 'm2'],
    });
    expect(cells).toEqual([
      { taskId: 't1', executorId: 'e1', modelId: 'm1' },
      { taskId: 't1', executorId: 'e1', modelId: 'm2' },
      { taskId: 't2', executorId: 'e1', modelId: 'm1' },
      { taskId: 't2', executorId: 'e1', modelId: 'm2' },
    ]);
  });

  it('returns a single cell for a fully scalar selection', () => {
    expect(expandLaunchMatrix({ taskIds: ['t'], executorIds: ['e'], modelIds: ['m'] }))
      .toEqual([{ taskId: 't', executorId: 'e', modelId: 'm' }]);
  });

  it('returns an empty list when any dimension is empty', () => {
    expect(expandLaunchMatrix({ taskIds: [], executorIds: ['e'], modelIds: ['m'] })).toEqual([]);
    expect(expandLaunchMatrix({ taskIds: ['t'], executorIds: [], modelIds: ['m'] })).toEqual([]);
    expect(expandLaunchMatrix({ taskIds: ['t'], executorIds: ['e'], modelIds: [] })).toEqual([]);
  });
});
```

- [ ] **Step 2: Run; fails to compile (speckitExtension still in source)**

```bash
cd frontend && npx vitest run src/pages/diagnostic/launch-matrix.test.ts
```
Expected: type errors / the source still requires `speckitExtensions`.

- [ ] **Step 3: Rewrite the matrix source to 3 axes**

Overwrite `frontend/src/pages/diagnostic/launch-matrix.ts` with:

```ts
/**
 * Pure expansion of the launcher's cross-experiment matrix into cells.
 *
 * Cross-experiment axes (each combination → one experiment, batched):
 *   task × executor × model
 *
 * Harnesses and spec-kit extensions are NOT axes here — they are
 * intra-experiment variants (see build-variants.ts). Every cell produces
 * one experiment carrying the same variant list.
 */

export interface LaunchCell {
  taskId: string;
  executorId: string;
  modelId: string;
}

export interface ExpansionInput {
  taskIds: string[];
  executorIds: string[];
  modelIds: string[];
}

/** Number of experiments the current selection will produce. */
export function countExperiments(input: ExpansionInput): number {
  return Math.max(input.taskIds.length, 1)
    * Math.max(input.executorIds.length, 1)
    * Math.max(input.modelIds.length, 1);
}

/**
 * Expand the (task × executor × model) cross-product into one cell per
 * experiment. Order is stable: tasks outermost, models innermost.
 */
export function expandLaunchMatrix(input: ExpansionInput): LaunchCell[] {
  const out: LaunchCell[] = [];
  for (const taskId of input.taskIds) {
    for (const executorId of input.executorIds) {
      for (const modelId of input.modelIds) {
        out.push({ taskId, executorId, modelId });
      }
    }
  }
  return out;
}
```

- [ ] **Step 4: Re-run; must pass**

```bash
cd frontend && npx vitest run src/pages/diagnostic/launch-matrix.test.ts
```
Expected: all PASS. (launch.tsx will not compile yet — that's Task 6.)

- [ ] **Step 5: Commit**

```bash
git add frontend/src/pages/diagnostic/launch-matrix.ts frontend/src/pages/diagnostic/launch-matrix.test.ts
git commit -m "matrix: drop speckit-extension axis, back to task×executor×model"
```

---

## Task 6 — Frontend: rewire the launcher page

**Files:**
- Modify: `frontend/src/pages/diagnostic/launch.tsx`

This is the integration task. Read the file fully first; the edits below reference current line regions but verify against the live file.

- [ ] **Step 1: Add imports**

Add near the other local imports:
```ts
import { buildLaunchVariants } from './build-variants';
import type { LaunchVariant } from '../../lib/types';
```

- [ ] **Step 2: Replace the `variants` useMemo with `experimentVariants`**

Replace the existing `variants` useMemo (the `harness × executor × model` one, ~lines 119-138) with:

```ts
  // The intra-experiment variant list — the same for every experiment
  // the matrix produces. Non-speckit harnesses → 1 each; speckit → 1 per
  // selected extension. Executor/model are cross-experiment axes, not
  // part of this list.
  const experimentVariants = useMemo<LaunchVariant[]>(
    () => buildLaunchVariants(selectedHarnesses, harnessConfigs, validSpecKitExtensions),
    [selectedHarnesses, harnessConfigs, validSpecKitExtensions],
  );
```

Note: `validSpecKitExtensions` is defined further down (line ~166). Move the `experimentVariants` memo to AFTER `validSpecKitExtensions` is declared, or hoist `validSpecKitExtensions` above it. Cleanest: place `experimentVariants` immediately after the `validSpecKitExtensions` / `speckitReady` block.

Delete the now-unused `Variant` interface (line ~38) and `allowedProvidersFor` usage inside the old memo only if nothing else references them — `allowedProvidersFor` is still used by `visibleModels`, so KEEP it. Only the `Variant` interface and the old memo go.

- [ ] **Step 3: Fix `totalRuns` and the submit gate**

Replace:
```ts
  const totalRuns = totalExperiments * Math.max(selectedHarnesses.length, 1) * runsPerVariant;
```
with:
```ts
  const totalRuns = totalExperiments * experimentVariants.length * runsPerVariant;
```

In `countExperiments(...)` call (line ~174), remove the `speckitExtensions` argument so it matches the new 3-axis signature:
```ts
  const totalExperiments = countExperiments({
    taskIds: taskIDs,
    executorIds: selectedExecutors,
    modelIds: selectedModels,
  });
```

Replace the `canSubmit` condition's `variants.length > 0` with `experimentVariants.length > 0`.

- [ ] **Step 4: Rewrite `handleLaunch`**

Replace the whole body from the `setPartialError(null)` line through the multi-cell block (the speckit-axis expansion + per-cell `cellConfigs` logic, ~lines 203-280) with:

```ts
    setPartialError(null);

    const cells = expandLaunchMatrix({
      taskIds: taskIDs,
      executorIds: selectedExecutors,
      modelIds: selectedModels,
    });
    const variants = buildLaunchVariants(selectedHarnesses, harnessConfigs, validSpecKitExtensions);
    if (variants.length === 0) {
      setPartialError('Pick at least one harness (and a spec-kit extension if spec-kit is selected).');
      return;
    }

    // Single cell → one experiment, no batch, land on Compare.
    if (cells.length === 1) {
      const cell = cells[0];
      const res = await launch.mutateAsync({
        task_id: cell.taskId,
        executor_id: cell.executorId,
        model: cell.modelId,
        runs_per_variant: runsPerVariant,
        name: name.trim() || undefined,
        variants,
      });
      navigate(`/diagnostic/compare?experiment=${res.experiment_id}`);
      return;
    }

    // Multi-cell → batch the experiments; every experiment gets the same
    // variant list.
    const batchId = crypto.randomUUID();
    const label = suiteLabel.trim()
      || `Diagnostic suite · ${new Date().toISOString().slice(0, 16).replace('T', ' ')}`;
    const results = await Promise.allSettled(cells.map((cell) =>
      launch.mutateAsync({
        task_id: cell.taskId,
        executor_id: cell.executorId,
        model: cell.modelId,
        runs_per_variant: runsPerVariant,
        batch_id: batchId,
        batch_label: label,
        variants,
      }),
    ));
    const failures = results
      .map((r, i) => r.status === 'rejected'
        ? { cell: cells[i], reason: r.reason instanceof Error ? r.reason.message : String(r.reason) }
        : null)
      .filter((x): x is { cell: typeof cells[number]; reason: string } => x !== null);
    if (failures.length > 0) {
      setPartialError(`Started ${cells.length - failures.length}/${cells.length}. Failed: ${failures.map((f) => f.cell.taskId).join(', ')}`);
    }
    navigate(`/experiments?batch=${batchId}`);
```

Keep the existing `taskIDs.length === 0` / `selectedExecutors.length === 0` / `selectedModels.length === 0` guards above this block as they are.

- [ ] **Step 5: Update `<VariantPreview>` to render the variant list**

Change the call site (line ~438) from `<VariantPreview variants={variants} />` to `<VariantPreview variants={experimentVariants} />`.

Replace the `VariantPreview` component (lines ~621-648) with:

```tsx
function VariantPreview({ variants }: { variants: LaunchVariant[] }) {
  if (variants.length === 0) {
    return (
      <div className="rounded-md border border-dashed border-border bg-bg-elev-1 px-3 py-3 text-center text-xs text-fg-muted">
        No variants yet. Tick at least one harness (and a spec-kit extension if spec-kit is on).
      </div>
    );
  }
  return (
    <ul className="max-h-56 space-y-0.5 overflow-auto rounded-md border border-border bg-bg-elev-1 px-2 py-1.5 font-mono text-xs">
      {variants.map((v, i) => (
        <li
          key={`${v.name}-${i}`}
          className="flex items-baseline gap-2 px-1 py-0.5 leading-5"
        >
          <span className="w-6 select-none text-right text-fg-subtle">
            {String(i + 1).padStart(2, '0')}
          </span>
          <span className="text-fg">{v.name}</span>
        </li>
      ))}
    </ul>
  );
}
```

- [ ] **Step 6: Typecheck + tests + build**

```bash
cd frontend && npm run lint && npm test -- --run 2>&1 | tail -10 && npm run build 2>&1 | tail -3 && npm run check:tokens 2>&1 | tail -3
```
Expected: lint clean (no unused `Variant` interface, no leftover `speckitExtensions` references); tests green; build succeeds; tokens clean. If lint flags an unused import or the removed `Variant` type, clean it up.

- [ ] **Step 7: Commit**

```bash
git add frontend/src/pages/diagnostic/launch.tsx
git commit -m "launcher: spec-kit extensions become intra-experiment variants (one experiment, N variants)"
```

---

## Task 7 — Frontend: Compare per-variant label

**Files:**
- Modify: `frontend/src/pages/diagnostic/compare.tsx`

- [ ] **Step 1: Fetch experiment detail for every id (not just matrix)**

In `compare.tsx`, replace the `matrixExperiments` fetch + `expIndex` memo (the `useExperimentsForIds(isMatrix ? experimentIDs : [])` line and the `expIndex` useMemo that branches on `isMatrix`) with:

```ts
  // Always fetch detail (variants populated) for every selected
  // experiment — the list endpoint's variants are now populated too,
  // but the per-run harness label needs each experiment's variant set
  // regardless of single vs matrix mode.
  const { data: detailExperiments } = useExperimentsForIds(experimentIDs);

  const expIndex = useMemo<Map<string, Experiment>>(() => {
    const m = new Map<string, Experiment>();
    if (detailExperiments) {
      for (const e of detailExperiments) {
        if (e) m.set(e.id, e);
      }
    }
    return m;
  }, [detailExperiments]);
```

If `useExperiments()` (the full-list hook) is now unused in the file after this change, remove its import and call. (It may still be used by the experiment picker dropdown — grep before removing; keep it if referenced.)

- [ ] **Step 2: Make `variantSignature` variant-aware**

Replace `variantSignature`:

```ts
function variantSignature(exp: Experiment, variantId?: string): string {
  const dot = ' · ';
  const idx = exp.name.lastIndexOf(dot);
  if (idx >= 0) {
    const tail = exp.name.slice(idx + dot.length).trim();
    if (tail.includes('/')) return tail;
  }
  // Resolve the run's specific variant by id — an experiment with N
  // variants (one per harness / spec-kit extension) would otherwise
  // label every column with variants[0].
  const variant = variantId
    ? exp.variants?.find((v) => v.id === variantId)
    : exp.variants?.[0];
  const harness = variant?.name ?? variant?.harness_id ?? '?';
  return `${harness}/${exp.agent_cli}/${exp.model}`;
}
```

- [ ] **Step 3: Thread `variant_id` through the callers**

- In `shortLabel`, widen the `runs` param type to include `variant_id?: string` and pass it:
  ```ts
  function shortLabel(
    runs: Array<{ id: string; run_number: number; experiment_id?: string; variant_id?: string }>,
    runId: string,
    expIndex?: Map<string, Experiment>,
  ): string {
    const found = runs.find((r) => r.id === runId);
    if (found && expIndex && found.experiment_id) {
      const exp = expIndex.get(found.experiment_id);
      if (exp) return variantSignature(exp, found.variant_id);
    }
    if (found) return `Run ${found.run_number}`;
    return runId.length > 12 ? runId.slice(0, 8) + '…' : runId;
  }
  ```
- In the RunPicker row render (the `const variantSig = exp ? variantSignature(exp) : null;` line), change to `variantSignature(exp, run.variant_id)`.
- In `GradeComparisonTableProps`, widen `runs` to include `variant_id?: string`.
- In the grade-table `rawCoords` builder, resolve the variant by id:
  ```ts
  const rawCoords = runIds.map((id) => {
    const run = runs.find((r) => r.id === id);
    const exp = run?.experiment_id ? expIndex?.get(run.experiment_id) : undefined;
    const variant = exp?.variants?.find((v) => v.id === run?.variant_id);
    return {
      harness: variant?.name ?? variant?.harness_id ?? '—',
      agent: exp?.agent_cli ?? '—',
      model: exp?.model ?? '—',
    } satisfies Record<Dim, string>;
  });
  ```

- [ ] **Step 4: Typecheck + tests + build**

```bash
cd frontend && npm run lint && npm test -- --run 2>&1 | tail -8 && npm run build 2>&1 | tail -3
```
Expected: clean.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/pages/diagnostic/compare.tsx
git commit -m "compare: label each column by its run's actual variant (harness / extension)"
```

---

## Task 8 — Full sanity pass

**Files:** none

- [ ] **Step 1: Backend**

```bash
cd engine && go test ./...
```
Expected: every package green.

- [ ] **Step 2: Frontend**

```bash
cd frontend && npm run lint && npm run build && npm test -- --run 2>&1 | tail -10 && npm run check:tokens
```
Expected: all four exit 0.

---

## Task 9 — Manual verification

**Files:** none (runtime).

- [ ] **Step 1: Restart engine + frontend**

```bash
lsof -ti tcp:8080 | xargs -r kill 2>/dev/null
lsof -ti tcp:5173 | xargs -r kill 2>/dev/null
(cd engine && nohup go run cmd/server/main.go > /tmp/engine.log 2>&1 &)
(cd frontend && nohup npm run dev > /tmp/vite.log 2>&1 &)
sleep 8
curl -s http://localhost:8080/api/health | python3 -c "import json,sys; print('engine=',json.load(sys.stdin).get('ok'))"
curl -s -o /dev/null -w 'vite=%{http_code}\n' http://localhost:5173/
```

- [ ] **Step 2: Browser walkthrough**

Visit `/diagnostic/launch`. With 1 task, 1 executor, 1 model:
1. Tick all 5 harnesses; tick all 6 spec-kit extensions.
2. The Variant Preview lists **10** rows: `bare`, `agent_instructions` (after typing its content), `multiagent` (after configuring roles), `ralph`, `speckit/canonical … speckit/dual-role`.
3. The total-runs line reads **10 total runs** (1 experiment × 10 variants × 1 run).
4. Launch → redirects to Compare for **one** experiment.
5. Backend check:
   ```bash
   curl -s http://localhost:8080/api/experiments | python3 -c "
   import json,sys,urllib.request
   exps=json.load(sys.stdin)
   e=exps[0]
   full=json.loads(urllib.request.urlopen(f'http://localhost:8080/api/experiments/{e[\"id\"]}').read())
   print('variants:', len(full['variants']))
   for v in full['variants']: print(' ', v['name'], v['harness_id'])
   "
   ```
   Expected: 10 variants, the 6 speckit ones named `speckit/<ext>` each with their own extension_id.
6. Experiments list (`/experiments`): the row shows **10v × 1r** (not "1 run").
7. Compare: 10 distinctly-labelled columns (`speckit/canonical` ≠ `speckit/lite`).

- [ ] **Step 3: Multi-cell check**

Pick 2 tasks (same harness/extension selection) → preview "2 experiments … 20 total runs"; launch → `/experiments?batch=<id>` with 2 experiments grouped, each 10 variants.

- [ ] **Step 4: Stop servers**

```bash
lsof -ti tcp:8080 | xargs -r kill 2>/dev/null
lsof -ti tcp:5173 | xargs -r kill 2>/dev/null
```

---

## Task 10 — Push, PR, review, CI, merge

**Files:** none.

- [ ] **Step 1: Push**

```bash
git push -u origin feature/extension-as-variant-rework
```

- [ ] **Step 2: Open PR**

```bash
gh pr create --title "Spec-kit extension as intra-experiment variant (matrix rework)" --body "$(cat <<'EOF'
## Summary

Fixes the launcher matrix semantics: spec-kit extension was a cross-experiment axis, so 1 task × 1 exec × 1 model × 6 extensions + 4 other harnesses produced 6 experiments × 5 variants = 30 runs (bare/ralph/etc. ran 6× pointlessly). Now extensions are intra-experiment variants of the spec-kit harness: the same selection produces **one experiment with 10 variants / 10 runs**.

- **Backend (additive):** \`/diagnostic/launch\` accepts an optional \`variants[]\` array, each with its own \`harness_id\` + \`harness_config\`. When present it builds one variant per entry (each its own config); when absent the legacy \`harness_ids\` + shared-config path runs unchanged (CLI scripts / suite endpoint preserved). Storage + orchestrator already persist and apply per-variant config (PR #145), so no schema or run-path change.
- **Backend:** \`ListExperiments\` now eagerly attaches variants via one batched query, so the Experiments list shows the real variant count instead of "1 run".
- **Frontend:** matrix reverts to 3 axes (task × exec × model). New \`buildLaunchVariants\` pure helper turns the harness + extension selection into the intra-experiment variant list (non-speckit harnesses + one \`speckit/<ext>\` per extension). The launcher sends \`variants\`; preview + run count reflect the per-experiment variant list.
- **Frontend:** Compare labels each column by its run's actual variant (\`speckit/canonical\` vs \`speckit/lite\`), resolved by \`variant_id\` — previously every column showed \`variants[0]\`'s harness and the list endpoint returned no variants so the label was blank.

Spec: [\`docs/superpowers/specs/2026-06-02-extension-as-variant-rework-design.md\`](docs/superpowers/specs/2026-06-02-extension-as-variant-rework-design.md)
Plan: [\`docs/superpowers/plans/2026-06-02-extension-as-variant-rework.md\`](docs/superpowers/plans/2026-06-02-extension-as-variant-rework.md)

## Test plan

- [x] \`go test ./...\` clean — new per-variant launch cases, unknown-harness rejection, legacy fallback, ListExperiments-includes-variants
- [x] \`npm run lint\` / \`build\` / \`test\` / \`check:tokens\` clean — new buildLaunchVariants cases, 3-axis matrix cases
- [x] Manual via Playwright: 5 harnesses + 6 extensions → 10-variant single experiment; Compare 10 distinct columns; Experiments list "10v × 1r"
EOF
)"
```

- [ ] **Step 3: Dispatch the code reviewer**

Per memory `feedback_github_workflow`, every non-trivial PR goes through `feature-dev:code-reviewer`. Dispatch on the branch HEAD and address findings.

- [ ] **Step 4: Watch CI**, fix failures if any.

- [ ] **Step 5: Squash merge once green**

```bash
gh pr merge <PR#> --squash --delete-branch
git fetch origin && git checkout main && git reset --hard origin/main
```

---

## Self-review

**Spec coverage:**
- Backend additive `variants[]` path → Task 1
- `ListExperiments` returns variants → Task 2
- Frontend types → Task 3
- `buildLaunchVariants` helper → Task 4
- Matrix back to 3 axes → Task 5
- Launcher rewiring (variant memo, totalRuns, handleLaunch, preview) → Task 6
- Compare per-variant label → Task 7
- Sanity / manual / PR → Tasks 8-10
- Out-of-scope (suite endpoint, CLI, run execution path) — explicitly preserved by the additive design; no tasks.

**No placeholders** — every step shows full code, exact commands, expected output.

**Type consistency:** Go `LaunchVariant{HarnessID, Name, HarnessConfig}` mirrors TS `LaunchVariant{harness_id, name, harness_config}`. `buildLaunchVariants(selectedHarnesses, harnessConfigs, speckitExtensions)` signature is identical in the helper, its tests, and both call sites (memo + handleLaunch). `expandLaunchMatrix`/`countExperiments` use the 3-field `ExpansionInput` consistently in Task 5 and Task 6's call sites. `variantSignature(exp, variantId)` and the `variant_id`-widened run types are consistent across Task 7's edits.

**Ordering:** backend first (independently testable + mergeable), then frontend types → helper → matrix → launcher integration → compare, then sanity/manual/PR. launch.tsx won't compile between Task 5 and Task 6 — both are frontend and land before the Task 8 sanity gate, so no broken intermediate is ever tested in isolation against the full suite.
