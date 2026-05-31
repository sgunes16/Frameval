# Agent instructions harness + per-variant config foundation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Rename `claudemd` to `agent_instructions`, source its CLAUDE.md content from a user-typed textarea in the launcher (per variant), and install the per-variant harness-config plumbing that Multi-Agent (Project 2) and Spec-Kit catalog (Project 3) will reuse.

**Architecture:** Three layers in lockstep. (1) SQLite migration 020 adds `variants.harness_config_json`. (2) Go `Harness` interface gains a `cfg map[string]any` parameter on `Setup`; the renamed `agent_instructions` harness reads `cfg["agent_instructions"]["content"]` and writes it as `CLAUDE.md` in the workspace; the other four harnesses keep working unchanged with a no-op signature update. The orchestrator passes each variant's persisted config into Setup. (3) The frontend launcher renders a generic `<HarnessConfigPanel>` below each selected harness chip; for `agent_instructions` that panel is a textarea; an empty textarea blocks Launch. Per-cell launch payloads carry the configs map. Run Inspect surfaces the active content above the transcript.

**Tech Stack:** Go 1.22 (stdlib `database/sql`, Chi router, no new deps), SQLite, React 18 + TypeScript + TanStack Query (existing).

**Spec:** [`docs/superpowers/specs/2026-05-31-agent-instructions-harness-foundation-design.md`](../specs/2026-05-31-agent-instructions-harness-foundation-design.md)

**Branch:** `feature/agent-instructions-harness` (created; spec committed at `2038af5`).

---

## File map

| Layer | File | Action |
|---|---|---|
| Schema | `engine/internal/storage/migrations/020_variant_harness_config.sql` | CREATE |
| Model | `engine/internal/models/experiment.go` | MODIFY (`Variant.HarnessConfig`, `VariantRequest.HarnessConfig`) |
| Variant repo | `engine/internal/storage/variant_repo.go` | MODIFY (INSERT + SELECTs + `scanVariant`) |
| Experiment repo | `engine/internal/storage/experiment_repo.go` | MODIFY (`CreateExperiment` passes `HarnessConfig` from `VariantRequest`) |
| Repo tests | `engine/internal/storage/experiment_repo_test.go` | MODIFY (append `TestVariantHarnessConfigRoundTrip`) |
| Harness interface | `engine/pkg/harness/harness.go` | MODIFY (add `cfg map[string]any` to `Setup`) |
| Bare harness | `engine/internal/builtin/harness/bare.go` | MODIFY (signature only) |
| Ralph harness | `engine/internal/builtin/harness/ralph.go` | MODIFY (signature only) |
| PlannerCoder harness | `engine/internal/builtin/harness/planner_coder.go` | MODIFY (signature only) |
| SpecKit harness | `engine/internal/builtin/harness/speckit.go` | MODIFY (signature only) |
| Harness tests | `engine/internal/builtin/harness/{bare,ralph,planner_coder,speckit}_test.go` | MODIFY (call sites pass `nil` cfg) |
| Old claudemd file | `engine/internal/builtin/harness/claudemd.go` | RENAME → `agent_instructions.go` + REWRITE body |
| Old claudemd test | `engine/internal/builtin/harness/claudemd_test.go` | RENAME → `agent_instructions_test.go` + REWRITE body |
| Registry | `engine/internal/builtin/harness/registry.go` | MODIFY (`NewClaudeMd` → `NewAgentInstructions`) |
| Task bundles | `tasks/{brownfield-fix-async-race,greenfield-cli-wordfreq,greenfield-rate-limiter-fastapi}/harness_context/` | DELETE whole dir |
| Orchestrator | `engine/internal/experiment/orchestrator.go:224` | MODIFY (unmarshal variant cfg, pass to `Setup`) |
| Launch API | `engine/internal/api/diagnostic_launch.go` | MODIFY (`HarnessConfigs` field on both request types + propagate to `VariantRequest`) |
| Launch API tests | `engine/internal/api/diagnostic_launch_test.go` | MODIFY (append round-trip test) |
| Frontend types | `frontend/src/lib/types.ts` | MODIFY (`Variant.harness_config`, request types) |
| Launcher panel component | `frontend/src/components/launcher/HarnessConfigPanel.tsx` | CREATE |
| Launcher panel test | `frontend/src/components/launcher/HarnessConfigPanel.test.tsx` | CREATE |
| Launcher page | `frontend/src/pages/diagnostic/launch.tsx` | MODIFY (state + render + gate + payload) |
| Run inspect | `frontend/src/pages/runs/inspect.tsx` | MODIFY (add agent-instructions card) |

---

## Task 1 — Schema migration

**Files:**
- Create: `engine/internal/storage/migrations/020_variant_harness_config.sql`

- [ ] **Step 1: Write the migration**

Create `engine/internal/storage/migrations/020_variant_harness_config.sql`:

```sql
-- 020_variant_harness_config.sql
--
-- Adds an opaque per-variant harness config blob. Keyed by harness id;
-- each value is whatever shape that harness expects. Today this carries
-- `agent_instructions.content` (the user-typed CLAUDE.md). Future
-- harnesses (multiagent, speckit) reuse this column without further
-- schema work.

ALTER TABLE variants ADD COLUMN harness_config_json TEXT;
```

No index — every read happens by `variant_id` which already has a PK index.

- [ ] **Step 2: Apply migration once and confirm**

```bash
rm -f /tmp/frameval-migrate-020.db
cd engine && FRAMEVAL_DB_PATH=/tmp/frameval-migrate-020.db go run cmd/server/main.go &
PID=$!
sleep 4
sqlite3 /tmp/frameval-migrate-020.db '.schema variants' | grep harness_config_json && echo "OK"
kill $PID 2>/dev/null
rm -f /tmp/frameval-migrate-020.db
```
Expected: `harness_config_json TEXT` appears in the schema dump.

- [ ] **Step 3: Commit**

```bash
git add engine/internal/storage/migrations/020_variant_harness_config.sql
git commit -m "Add variants.harness_config_json column (migration 020)"
```

---

## Task 2 — Model field

**Files:**
- Modify: `engine/internal/models/experiment.go`

- [ ] **Step 1: Add `HarnessConfig` to `Variant` and `VariantRequest`**

In `engine/internal/models/experiment.go`, locate `type Variant struct` (around line 30) and add a new field before `ArtifactVersions`:

```go
    HarnessConfig    map[string]any    `json:"harness_config,omitempty"`
```

Then find `type VariantRequest struct` in the same file. Add the same field:

```go
    HarnessConfig    map[string]any    `json:"harness_config,omitempty"`
```

If you can't find `VariantRequest`, search via:

```bash
grep -n 'type VariantRequest' engine/internal/models/*.go
```

- [ ] **Step 2: Build**

```bash
cd engine && go build ./...
```
Expected: exits 0.

- [ ] **Step 3: Commit**

```bash
git add engine/internal/models/experiment.go
git commit -m "Add HarnessConfig field to Variant + VariantRequest"
```

---

## Task 3 — Variant repo round-trip (TDD)

**Files:**
- Modify: `engine/internal/storage/experiment_repo_test.go`
- Modify: `engine/internal/storage/variant_repo.go`
- Modify: `engine/internal/storage/experiment_repo.go`

- [ ] **Step 1: Write the failing test**

Append to `engine/internal/storage/experiment_repo_test.go`:

```go
func TestVariantHarnessConfigRoundTrip(t *testing.T) {
	store := support.TmpStore(t)
	seedTaskForExperimentTest(t, store, "task-hc-1")

	exp, err := store.CreateExperiment(context.Background(), models.ExperimentRequest{
		Name:           "harness-config",
		TaskID:         "task-hc-1",
		Model:          "claude",
		AgentCLI:       "claude",
		RunsPerVariant: 1,
		Variants: []models.VariantRequest{
			{
				Name:      "bare",
				HarnessID: "bare",
				HarnessConfig: map[string]any{
					"agent_instructions": map[string]any{"content": "rule one"},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("CreateExperiment: %v", err)
	}
	if len(exp.Variants) != 1 {
		t.Fatalf("variant count: got %d", len(exp.Variants))
	}
	gotCfg, _ := exp.Variants[0].HarnessConfig["agent_instructions"].(map[string]any)
	if got, _ := gotCfg["content"].(string); got != "rule one" {
		t.Fatalf("HarnessConfig round-trip: got %q want %q", got, "rule one")
	}
}
```

- [ ] **Step 2: Run; the test must fail**

```bash
cd engine && go test ./internal/storage/ -run TestVariantHarnessConfigRoundTrip -v
```
Expected: FAIL with a nil-pointer / empty map assertion. The INSERT doesn't carry the column, the SELECT doesn't fetch it, the scan doesn't populate the field — first failure is the assertion at `gotCfg["content"]`.

- [ ] **Step 3: Update `CreateExperiment` to propagate `HarnessConfig` into the variant INSERT**

In `engine/internal/storage/experiment_repo.go`, find the variant INSERT inside `CreateExperiment` (around line 47):

```go
		_, err = tx.ExecContext(ctx, `
			INSERT INTO variants (id, experiment_id, name, description, is_control, ordering, harness_id)
			VALUES (?, ?, ?, ?, ?, ?, ?)
		`, variantID, experimentID, variantReq.Name, variantReq.Description, boolToInt(variantReq.IsControl), maxInt(variantReq.Ordering, idx), fallbackHarnessID(variantReq.HarnessID))
```

Replace with:

```go
		_, err = tx.ExecContext(ctx, `
			INSERT INTO variants (id, experiment_id, name, description, is_control, ordering, harness_id, harness_config_json)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		`, variantID, experimentID, variantReq.Name, variantReq.Description, boolToInt(variantReq.IsControl), maxInt(variantReq.Ordering, idx), fallbackHarnessID(variantReq.HarnessID), nullableString(marshalJSON(variantReq.HarnessConfig)))
```

`marshalJSON` is the existing helper in the same package. Wrapping in `nullableString` converts the literal "null" string back to NULL so empty configs stay clean.

- [ ] **Step 4: Update `CreateVariant` to also carry the column**

In `engine/internal/storage/variant_repo.go`, replace the body of `CreateVariant`:

```go
func (s *Store) CreateVariant(ctx context.Context, variant models.Variant) (*models.Variant, error) {
	if variant.ID == "" {
		variant.ID = uuid.NewString()
	}
	_, err := s.DB.ExecContext(ctx, `
		INSERT INTO variants (id, experiment_id, name, description, is_control, ordering, harness_id, harness_config_json)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, variant.ID, variant.ExperimentID, variant.Name, variant.Description, boolToInt(variant.IsControl), variant.Ordering, fallbackHarnessID(variant.HarnessID), nullableString(marshalJSON(variant.HarnessConfig)))
	if err != nil {
		return nil, fmt.Errorf("create variant: %w", err)
	}
	return s.GetVariant(ctx, variant.ID)
}
```

- [ ] **Step 5: Update both SELECTs to fetch the new column**

In `engine/internal/storage/variant_repo.go`, replace the `ListVariantsByExperiment` query:

```go
	rows, err := s.DB.QueryContext(ctx, `SELECT id, experiment_id, name, description, is_control, ordering, harness_id, harness_config_json FROM variants WHERE experiment_id = ? ORDER BY ordering ASC`, experimentID)
```

And `GetVariant`:

```go
	row := s.DB.QueryRowContext(ctx, `SELECT id, experiment_id, name, description, is_control, ordering, harness_id, harness_config_json FROM variants WHERE id = ?`, variantID)
```

- [ ] **Step 6: Update `scanVariant` to scan the new column**

Replace `scanVariant` with:

```go
func scanVariant(scanner interface{ Scan(dest ...any) error }) (models.Variant, error) {
	var variant models.Variant
	var description sql.NullString
	var harnessID sql.NullString
	var harnessConfigJSON sql.NullString
	var isControl int
	if err := scanner.Scan(&variant.ID, &variant.ExperimentID, &variant.Name, &description, &isControl, &variant.Ordering, &harnessID, &harnessConfigJSON); err != nil {
		return variant, fmt.Errorf("scan variant: %w", err)
	}
	variant.Description = description.String
	variant.IsControl = isControl == 1
	variant.HarnessID = fallbackHarnessID(harnessID.String)
	if harnessConfigJSON.Valid && harnessConfigJSON.String != "" {
		variant.HarnessConfig = unmarshalJSON(harnessConfigJSON.String, map[string]any{})
	}
	return variant, nil
}
```

Don't forget: also update `UpdateVariant` to carry the column. Replace its body:

```go
func (s *Store) UpdateVariant(ctx context.Context, variantID string, variant models.Variant) (*models.Variant, error) {
	_, err := s.DB.ExecContext(ctx, `UPDATE variants SET name = ?, description = ?, is_control = ?, ordering = ?, harness_id = ?, harness_config_json = ? WHERE id = ?`, variant.Name, variant.Description, boolToInt(variant.IsControl), variant.Ordering, fallbackHarnessID(variant.HarnessID), nullableString(marshalJSON(variant.HarnessConfig)), variantID)
	if err != nil {
		return nil, fmt.Errorf("update variant: %w", err)
	}
	return s.GetVariant(ctx, variantID)
}
```

- [ ] **Step 7: Re-run the test; it must pass**

```bash
cd engine && go test ./internal/storage/ -run TestVariantHarnessConfigRoundTrip -v
```
Expected: PASS.

- [ ] **Step 8: Run the full storage suite — no regression**

```bash
cd engine && go test ./internal/storage/...
```
Expected: every test green.

- [ ] **Step 9: Commit**

```bash
git add engine/internal/storage/variant_repo.go engine/internal/storage/experiment_repo.go engine/internal/storage/experiment_repo_test.go
git commit -m "Persist Variant.HarnessConfig through CRUD"
```

---

## Task 4 — Harness interface change

**Files:**
- Modify: `engine/pkg/harness/harness.go`
- Modify: `engine/internal/builtin/harness/{bare,ralph,planner_coder,speckit,claudemd}.go`
- Modify: `engine/internal/builtin/harness/{bare,ralph,planner_coder,speckit,claudemd}_test.go`

(`claudemd` will be renamed in Task 5; for now just update its signature so the interface change can land.)

- [ ] **Step 1: Update the interface**

In `engine/pkg/harness/harness.go`, replace the `Harness` interface block:

```go
type Harness interface {
	Name() string
	Description() string

	// Setup prepares the workspace before agent invocation. cfg carries
	// per-variant configuration the launcher supplied, keyed by harness
	// id. A harness that doesn't need config can ignore it.
	Setup(ctx context.Context, ws Workspace, t task.Task, budget Budget, cfg map[string]any) (HarnessRun, error)

	Invoke(ctx context.Context, run HarnessRun, exec executor.AgentExecutor) (*executor.RunResult, error)
	Teardown(ctx context.Context, run HarnessRun) error
}
```

- [ ] **Step 2: Add the parameter to every harness implementation**

For each of `bare.go`, `ralph.go`, `planner_coder.go`, `speckit.go`, `claudemd.go` in `engine/internal/builtin/harness/`, find the `Setup` method and append `, cfg map[string]any` to the signature. The body stays the same — the new parameter is ignored for now (except claudemd, which Task 5 rewrites).

Example diff for `bare.go`:

```go
// Before:
func (h *Bare) Setup(_ context.Context, ws harness.Workspace, t task.Task, b harness.Budget) (harness.HarnessRun, error) {

// After:
func (h *Bare) Setup(_ context.Context, ws harness.Workspace, t task.Task, b harness.Budget, _ map[string]any) (harness.HarnessRun, error) {
```

Repeat verbatim for `ralph.go`, `planner_coder.go`, `speckit.go`, `claudemd.go`.

- [ ] **Step 3: Update the harness test files**

Every harness test that calls `Setup(...)` directly needs the new `nil` arg. The compiler will tell you exactly where. Fix mechanically. Locations (one each):

```bash
grep -n '\.Setup(' engine/internal/builtin/harness/*_test.go
```

Replace each call like `h.Setup(ctx, ws, t, b)` with `h.Setup(ctx, ws, t, b, nil)`.

- [ ] **Step 4: Update the orchestrator call site**

In `engine/internal/experiment/orchestrator.go` around line 224:

```go
	hRun, setupErr := harnessImpl.Setup(ctx, harnessWorkspace, *task, pkgharness.Budget{
		MaxIterations:  3,
		MaxWallSeconds: experiment.TimeoutSeconds,
	}, variant.HarnessConfig)
```

(`variant` is in scope at that point — verify with `grep -n 'variant\s*,?\s*err' engine/internal/experiment/orchestrator.go` and check the surrounding 30 lines if needed.)

- [ ] **Step 5: Build the whole engine — must compile cleanly**

```bash
cd engine && go build ./...
```
Expected: exits 0.

- [ ] **Step 6: Run all engine tests**

```bash
cd engine && go test ./...
```
Expected: all green. If any test fails because of the signature change (likely just compile errors that vanish after Step 3), fix the missed call sites.

- [ ] **Step 7: Commit**

```bash
git add engine/pkg/harness/harness.go engine/internal/builtin/harness/ engine/internal/experiment/orchestrator.go
git commit -m "Add cfg map[string]any parameter to Harness.Setup (no-op for 4 of 5 harnesses)"
```

---

## Task 5 — Rename claudemd → agent_instructions and rewrite body (TDD)

**Files:**
- Rename: `engine/internal/builtin/harness/claudemd.go` → `agent_instructions.go`
- Rename: `engine/internal/builtin/harness/claudemd_test.go` → `agent_instructions_test.go`
- Modify: `engine/internal/builtin/harness/registry.go`
- Delete: 3 task `harness_context/claudemd.md` files

- [ ] **Step 1: Rewrite the test file first (TDD)**

`git mv engine/internal/builtin/harness/claudemd_test.go engine/internal/builtin/harness/agent_instructions_test.go`

Replace the entire body of `agent_instructions_test.go` with:

```go
package harness

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	pkgharness "github.com/mustafaselman/frameval/engine/pkg/harness"
	"github.com/mustafaselman/frameval/engine/pkg/task"
)

func TestAgentInstructionsNameAndDescription(t *testing.T) {
	h := NewAgentInstructions()
	if h.Name() != "agent_instructions" {
		t.Errorf("Name = %q, want %q", h.Name(), "agent_instructions")
	}
	if h.Description() == "" {
		t.Error("Description should not be empty")
	}
}

func TestAgentInstructionsSetupWritesClaudeMD(t *testing.T) {
	h := NewAgentInstructions()
	ws := pkgharness.Workspace{Path: t.TempDir()}
	cfg := map[string]any{
		"agent_instructions": map[string]any{"content": "# rules\nbe concise"},
	}
	run, err := h.Setup(context.Background(), ws, task.Task{ID: "fixture"}, pkgharness.Budget{}, cfg)
	if err != nil {
		t.Fatalf("Setup: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(ws.Path, "CLAUDE.md"))
	if err != nil {
		t.Fatalf("read CLAUDE.md: %v", err)
	}
	if string(got) != "# rules\nbe concise" {
		t.Errorf("CLAUDE.md content: got %q", string(got))
	}
	if run.HarnessName != "agent_instructions" {
		t.Errorf("HarnessName: got %q", run.HarnessName)
	}
}

func TestAgentInstructionsSetupRejectsEmptyContent(t *testing.T) {
	h := NewAgentInstructions()
	ws := pkgharness.Workspace{Path: t.TempDir()}
	cases := []map[string]any{
		nil,
		{},
		{"agent_instructions": map[string]any{}},
		{"agent_instructions": map[string]any{"content": ""}},
		{"agent_instructions": map[string]any{"content": "   \n\t  "}},
	}
	for i, cfg := range cases {
		if _, err := h.Setup(context.Background(), ws, task.Task{ID: "fixture"}, pkgharness.Budget{}, cfg); !errors.Is(err, ErrAgentInstructionsContentMissing) {
			t.Errorf("case %d: got err=%v, want ErrAgentInstructionsContentMissing", i, err)
		}
	}
}

func TestAgentInstructionsSetupRefusesExistingClaudeMD(t *testing.T) {
	h := NewAgentInstructions()
	ws := pkgharness.Workspace{Path: t.TempDir()}
	if err := os.WriteFile(filepath.Join(ws.Path, "CLAUDE.md"), []byte("existing"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	cfg := map[string]any{"agent_instructions": map[string]any{"content": "x"}}
	if _, err := h.Setup(context.Background(), ws, task.Task{ID: "fixture"}, pkgharness.Budget{}, cfg); err == nil {
		t.Fatal("Setup should refuse to overwrite existing CLAUDE.md")
	}
}

func TestAgentInstructionsTeardownRemovesOwnedFile(t *testing.T) {
	h := NewAgentInstructions()
	ws := pkgharness.Workspace{Path: t.TempDir()}
	cfg := map[string]any{"agent_instructions": map[string]any{"content": "x"}}
	run, err := h.Setup(context.Background(), ws, task.Task{ID: "fixture"}, pkgharness.Budget{}, cfg)
	if err != nil {
		t.Fatalf("Setup: %v", err)
	}
	if err := h.Teardown(context.Background(), run); err != nil {
		t.Fatalf("Teardown: %v", err)
	}
	if _, err := os.Stat(filepath.Join(ws.Path, "CLAUDE.md")); !os.IsNotExist(err) {
		t.Error("Teardown should have removed CLAUDE.md")
	}
}
```

- [ ] **Step 2: Run; tests must fail (constructor doesn't exist yet, old constructor `NewClaudeMd` still in registry)**

```bash
cd engine && go test ./internal/builtin/harness/ -run TestAgentInstructions -v
```
Expected: compile errors about `NewAgentInstructions` / `ErrAgentInstructionsContentMissing`.

- [ ] **Step 3: Rename the source file and replace its body**

`git mv engine/internal/builtin/harness/claudemd.go engine/internal/builtin/harness/agent_instructions.go`

Replace the entire body of `agent_instructions.go` with:

```go
package harness

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mustafaselman/frameval/engine/pkg/executor"
	"github.com/mustafaselman/frameval/engine/pkg/harness"
	"github.com/mustafaselman/frameval/engine/pkg/task"
)

const (
	// AgentInstructionsHarnessID is the stable wire id for this harness.
	AgentInstructionsHarnessID = "agent_instructions"

	// agentInstructionsConfigKey is the top-level key the harness reads
	// from the per-variant cfg map.
	agentInstructionsConfigKey = "agent_instructions"

	// agentInstructionsTargetFile is the filename written into the
	// workspace. Stays CLAUDE.md so existing agents that read it by name
	// keep working.
	agentInstructionsTargetFile = "CLAUDE.md"

	metadataKeyOwnsTarget = "agent_instructions.created_by_harness"
)

// ErrAgentInstructionsContentMissing surfaces when the launcher's
// per-variant cfg has no usable text. The launcher's submit gate
// prevents this in normal flow; the sentinel is the last line of
// defense for direct API consumers.
var ErrAgentInstructionsContentMissing = errors.New(
	"agent_instructions harness: cfg.agent_instructions.content is empty; user must supply text in the launcher")

// AgentInstructions lays the user-typed CLAUDE.md into the workspace
// from the per-variant config blob. Replaces the older `claudemd`
// harness that read from task-bundled files.
type AgentInstructions struct{}

func NewAgentInstructions() *AgentInstructions { return &AgentInstructions{} }

func (h *AgentInstructions) Name() string { return AgentInstructionsHarnessID }

func (h *AgentInstructions) Description() string {
	return "Lay down user-supplied CLAUDE.md (typed in the launcher) into the workspace before the agent runs"
}

func (h *AgentInstructions) Setup(_ context.Context, ws harness.Workspace, t task.Task, b harness.Budget, cfg map[string]any) (harness.HarnessRun, error) {
	content, ok := extractAgentInstructionsContent(cfg)
	if !ok || strings.TrimSpace(content) == "" {
		return harness.HarnessRun{}, ErrAgentInstructionsContentMissing
	}
	dst := filepath.Join(ws.Path, agentInstructionsTargetFile)
	if _, statErr := os.Stat(dst); statErr == nil {
		return harness.HarnessRun{}, fmt.Errorf("agent_instructions harness: workspace already contains %s; refusing to overwrite", agentInstructionsTargetFile)
	} else if !errors.Is(statErr, os.ErrNotExist) {
		return harness.HarnessRun{}, fmt.Errorf("agent_instructions harness: stat dst: %w", statErr)
	}
	if err := os.WriteFile(dst, []byte(content), 0o644); err != nil {
		return harness.HarnessRun{}, fmt.Errorf("agent_instructions harness: write %s: %w", agentInstructionsTargetFile, err)
	}
	return harness.HarnessRun{
		HarnessName: h.Name(),
		Task:        t,
		Workspace:   ws,
		Budget:      b,
		Metadata:    map[string]any{metadataKeyOwnsTarget: true},
	}, nil
}

func (h *AgentInstructions) Invoke(ctx context.Context, run harness.HarnessRun, exec executor.AgentExecutor) (*executor.RunResult, error) {
	return exec.Execute(ctx, harness.MergeConfig(run.BaseRunConfig, executor.RunConfig{
		Prompt:        run.Task.TaskPrompt,
		WorkspacePath: run.Workspace.Path,
	}))
}

func (h *AgentInstructions) Teardown(_ context.Context, run harness.HarnessRun) error {
	if run.Workspace.Path == "" {
		return nil
	}
	owned, _ := run.Metadata[metadataKeyOwnsTarget].(bool)
	if !owned {
		return nil
	}
	target := filepath.Join(run.Workspace.Path, agentInstructionsTargetFile)
	if err := os.Remove(target); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("agent_instructions harness: teardown remove %s: %w", target, err)
	}
	return nil
}

// extractAgentInstructionsContent safely walks cfg["agent_instructions"]["content"].
// Returns ("", false) on any structural mismatch; never panics.
func extractAgentInstructionsContent(cfg map[string]any) (string, bool) {
	if cfg == nil {
		return "", false
	}
	sub, ok := cfg[agentInstructionsConfigKey].(map[string]any)
	if !ok {
		return "", false
	}
	content, ok := sub["content"].(string)
	if !ok {
		return "", false
	}
	return content, true
}
```

- [ ] **Step 4: Update the registry**

In `engine/internal/builtin/harness/registry.go`, replace the constructor call:

```go
// Before:
	mustRegister(r, NewClaudeMd())

// After:
	mustRegister(r, NewAgentInstructions())
```

- [ ] **Step 5: Delete the 3 task harness_context bundles**

```bash
rm -rf tasks/brownfield-fix-async-race/harness_context
rm -rf tasks/greenfield-cli-wordfreq/harness_context
rm -rf tasks/greenfield-rate-limiter-fastapi/harness_context
```

- [ ] **Step 6: Re-run all harness tests**

```bash
cd engine && go test ./internal/builtin/harness/ -v
```
Expected: all PASS, including the 5 new `TestAgentInstructions*` cases.

- [ ] **Step 7: Run the full engine suite**

```bash
cd engine && go test ./...
```
Expected: all green. If anything else references `NewClaudeMd` or the `claudemd` id at the wire level, fix it now — grep:

```bash
grep -rn 'NewClaudeMd\|"claudemd"' engine/ --include='*.go'
```

If any production hit appears (tests excluded), update it to `NewAgentInstructions` / `"agent_instructions"`.

- [ ] **Step 8: Commit**

```bash
git add engine/internal/builtin/harness/agent_instructions.go engine/internal/builtin/harness/agent_instructions_test.go engine/internal/builtin/harness/registry.go tasks/brownfield-fix-async-race tasks/greenfield-cli-wordfreq tasks/greenfield-rate-limiter-fastapi
git commit -m "Rename claudemd harness → agent_instructions, drop task-bundled files

The harness now consumes per-variant cfg[\"agent_instructions\"][\"content\"]
supplied by the launcher. The 3 task harness_context/ bundles for the
old claudemd are deleted — a launch without user-supplied content fails
Setup with ErrAgentInstructionsContentMissing.

Wire id and registered name both change to 'agent_instructions'; the
target filename in the workspace stays CLAUDE.md."
```

---

## Task 6 — Launch API accepts `harness_configs` (TDD)

**Files:**
- Modify: `engine/internal/api/diagnostic_launch.go`
- Modify: `engine/internal/api/diagnostic_launch_test.go`

- [ ] **Step 1: Write the failing test**

Append to `engine/internal/api/diagnostic_launch_test.go`:

```go
func TestLaunchDiagnosticPersistsHarnessConfig(t *testing.T) {
	svc := newLaunchTestService(t)

	body, _ := json.Marshal(map[string]any{
		"task_id":     "t-launch",
		"executor_id": "opencode",
		"harness_ids": []string{"bare"},
		"model":       "anything",
		"harness_configs": map[string]any{
			"agent_instructions": map[string]any{"content": "# rules\nbe concise"},
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
	if len(exp.Variants) != 1 {
		t.Fatalf("variant count: got %d", len(exp.Variants))
	}
	sub, _ := exp.Variants[0].HarnessConfig["agent_instructions"].(map[string]any)
	if got, _ := sub["content"].(string); got != "# rules\nbe concise" {
		t.Errorf("HarnessConfig: got %q want %q", got, "# rules\nbe concise")
	}
}
```

- [ ] **Step 2: Run; the test must fail**

```bash
cd engine && go test ./internal/api/ -run TestLaunchDiagnosticPersistsHarnessConfig -v
```
Expected: FAIL — the variant's `HarnessConfig` is nil because the handler doesn't read the field.

- [ ] **Step 3: Add the field to `LaunchDiagnosticRequest`**

In `engine/internal/api/diagnostic_launch.go`, extend the struct:

```go
type LaunchDiagnosticRequest struct {
	TaskID         string         `json:"task_id"`
	ExecutorID     string         `json:"executor_id"`
	HarnessIDs     []string       `json:"harness_ids"`
	Model          string         `json:"model"`
	RunsPerVariant int            `json:"runs_per_variant"`
	TimeoutSeconds int            `json:"timeout_seconds"`
	Name           string         `json:"name"`
	BatchID        string         `json:"batch_id"`
	BatchLabel     string         `json:"batch_label"`
	HarnessConfigs map[string]any `json:"harness_configs,omitempty"`
}
```

- [ ] **Step 4: Pass it into every variant**

Find the loop that builds `[]models.VariantRequest` (lines ~88-97). Replace with:

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

- [ ] **Step 5: Mirror the field on the suite request + handler**

Add to `LaunchDiagnosticSuiteRequest`:

```go
	HarnessConfigs map[string]any `json:"harness_configs,omitempty"`
```

In `LaunchDiagnosticSuite`'s per-task loop, find the variants build and add `HarnessConfig: req.HarnessConfigs` to each `VariantRequest` literal. (Same one-line change as Step 4, in a different function.)

- [ ] **Step 6: Re-run the test; it must pass**

```bash
cd engine && go test ./internal/api/ -run TestLaunchDiagnosticPersistsHarnessConfig -v
```
Expected: PASS.

- [ ] **Step 7: Run the full engine suite**

```bash
cd engine && go test ./...
```
Expected: all green.

- [ ] **Step 8: Commit**

```bash
git add engine/internal/api/diagnostic_launch.go engine/internal/api/diagnostic_launch_test.go
git commit -m "Launch endpoints accept and persist harness_configs per variant"
```

---

## Task 7 — Frontend types + new HarnessConfigPanel component (TDD)

**Files:**
- Modify: `frontend/src/lib/types.ts`
- Create: `frontend/src/components/launcher/HarnessConfigPanel.tsx`
- Create: `frontend/src/components/launcher/HarnessConfigPanel.test.tsx`

- [ ] **Step 1: Extend types**

In `frontend/src/lib/types.ts`:

1. Find `export type Variant = {` (around line 90). Append a new optional field before the closing `}`:

```ts
  harness_config?: Record<string, unknown>;
```

2. Find `export type LaunchDiagnosticRequest = {` (around line 284). Append:

```ts
  harness_configs?: Record<string, unknown>;
```

3. Find `export type LaunchDiagnosticSuiteRequest = {` (added in PR #143). Append the same field:

```ts
  harness_configs?: Record<string, unknown>;
```

4. Run typecheck to confirm no other consumer breaks:

```bash
cd frontend && npm run lint
```
Expected: exits 0.

- [ ] **Step 2: Write the panel test (TDD)**

Create `frontend/src/components/launcher/HarnessConfigPanel.test.tsx`:

```tsx
import { describe, expect, it, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { HarnessConfigPanel } from './HarnessConfigPanel';

describe('HarnessConfigPanel', () => {
  it('renders nothing for an unknown harness id', () => {
    const { container } = render(
      <HarnessConfigPanel harnessId="unknown" value={undefined} onChange={() => {}} />,
    );
    expect(container.firstChild).toBeNull();
  });

  it('renders a textarea for agent_instructions and reports edits', () => {
    const onChange = vi.fn();
    render(
      <HarnessConfigPanel
        harnessId="agent_instructions"
        value={{ content: 'hello' }}
        onChange={onChange}
      />,
    );
    const ta = screen.getByLabelText(/agent instructions/i) as HTMLTextAreaElement;
    expect(ta.value).toBe('hello');
    fireEvent.change(ta, { target: { value: 'updated' } });
    expect(onChange).toHaveBeenCalledWith({ content: 'updated' });
  });

  it('renders empty textarea when value has no content', () => {
    render(
      <HarnessConfigPanel harnessId="agent_instructions" value={undefined} onChange={() => {}} />,
    );
    const ta = screen.getByLabelText(/agent instructions/i) as HTMLTextAreaElement;
    expect(ta.value).toBe('');
  });
});
```

- [ ] **Step 3: Run; the test must fail (module missing)**

```bash
cd frontend && npx vitest run src/components/launcher/HarnessConfigPanel.test.tsx
```
Expected: FAIL — "Cannot find module './HarnessConfigPanel'".

- [ ] **Step 4: Implement the panel**

Create `frontend/src/components/launcher/HarnessConfigPanel.tsx`:

```tsx
export type HarnessConfigValue = Record<string, unknown>;

interface PanelProps {
  harnessId: string;
  value: HarnessConfigValue | undefined;
  onChange: (next: HarnessConfigValue) => void;
}

/**
 * Per-harness config form. The launcher renders one of these below
 * every selected harness chip. Harnesses that don't need config
 * (bare, ralph) render nothing. Future harnesses (multiagent,
 * speckit) add their own switch case here without touching the
 * launcher page itself.
 */
export function HarnessConfigPanel({ harnessId, value, onChange }: PanelProps) {
  switch (harnessId) {
    case 'agent_instructions':
      return (
        <AgentInstructionsForm
          value={value as { content?: string } | undefined}
          onChange={onChange}
        />
      );
    default:
      return null;
  }
}

function AgentInstructionsForm({
  value,
  onChange,
}: {
  value: { content?: string } | undefined;
  onChange: (next: HarnessConfigValue) => void;
}) {
  return (
    <div className="mt-2 rounded-md border border-border bg-bg-elev-1 p-3">
      <label
        htmlFor="agent-instructions-content"
        className="block text-xs uppercase tracking-wider text-fg-muted"
      >
        Agent instructions (laid down as CLAUDE.md)
      </label>
      <textarea
        id="agent-instructions-content"
        className="mt-1 min-h-32 w-full rounded-md border border-border bg-bg p-2 font-mono text-xs text-fg"
        placeholder="# Project rules&#10;&#10;Keep changes focused..."
        value={value?.content ?? ''}
        onChange={(e) => onChange({ content: e.target.value })}
      />
    </div>
  );
}
```

- [ ] **Step 5: Re-run; tests must pass**

```bash
cd frontend && npx vitest run src/components/launcher/HarnessConfigPanel.test.tsx
```
Expected: 3/3 PASS.

- [ ] **Step 6: Run full Vitest suite — no regression**

```bash
cd frontend && npm test -- --run 2>&1 | tail -10
```
Expected: all PASS.

- [ ] **Step 7: Commit**

```bash
git add frontend/src/lib/types.ts frontend/src/components/launcher/HarnessConfigPanel.tsx frontend/src/components/launcher/HarnessConfigPanel.test.tsx
git commit -m "Add HarnessConfigPanel component + agent_instructions textarea"
```

---

## Task 8 — Launcher: harness chips render the panel, submit gate, payload

**Files:**
- Modify: `frontend/src/pages/diagnostic/launch.tsx`

- [ ] **Step 1: Import the panel and add state**

In `frontend/src/pages/diagnostic/launch.tsx`, add the import (sort alphabetically with the other component imports):

```ts
import { HarnessConfigPanel, type HarnessConfigValue } from '../../components/launcher/HarnessConfigPanel';
```

Inside `DiagnosticLaunchPage`, add the state right next to the other launcher state (near `selectedHarnesses`):

```ts
  const [harnessConfigs, setHarnessConfigs] = useState<Record<string, HarnessConfigValue>>({});
```

- [ ] **Step 2: Render a panel below each selected harness chip**

Locate the JSX block that renders the harness chips. It's the section with `selectedHarnesses` toggles, similar in structure to executors / models. Below the chip row (after the chip-list `</div>`), add:

```tsx
              {selectedHarnesses.map((hid) => (
                <HarnessConfigPanel
                  key={hid}
                  harnessId={hid}
                  value={harnessConfigs[hid]}
                  onChange={(next) => setHarnessConfigs((prev) => ({ ...prev, [hid]: next }))}
                />
              ))}
```

For harnesses that don't need config the component returns null — invisible. For `agent_instructions` it renders the textarea inline.

- [ ] **Step 3: Update the submit gate**

Locate the `canSubmit` derivation. Replace it with:

```ts
  const needsAgentInstructions = selectedHarnesses.includes('agent_instructions');
  const agentInstructionsContent =
    (harnessConfigs.agent_instructions as { content?: string } | undefined)?.content?.trim() ?? '';
  const agentInstructionsReady = !needsAgentInstructions || agentInstructionsContent.length > 0;
  const canSubmit =
    taskIDs.length > 0
    && variants.length > 0
    && agentInstructionsReady
    && !launch.isPending;
```

In the Launch button's `disabled` props and label logic, add a branch so the user sees why the button is disabled when agent_instructions is empty:

```tsx
              {launch.isPending
                ? 'Launching…'
                : taskIDs.length === 0
                ? 'Pick a task'
                : variants.length === 0
                ? 'Pick a variant'
                : !agentInstructionsReady
                ? 'Type agent instructions'
                : totalExperiments === 1
                ? 'Launch · 1 task'
                : `Launch suite · ${totalExperiments} experiments`}
```

(Match the exact existing structure — only add the `!agentInstructionsReady` branch.)

- [ ] **Step 4: Send `harness_configs` in every launch payload**

Inside `handleLaunch`, in BOTH the single-cell branch and the multi-cell loop, every `launch.mutateAsync(...)` payload gets an extra field:

```ts
        harness_configs: harnessConfigs,
```

Concrete: for the single-cell branch:

```ts
      const res = await launch.mutateAsync({
        task_id: cell.taskId,
        executor_id: cell.executorId,
        harness_ids: selectedHarnesses,
        model: cell.modelId,
        runs_per_variant: runsPerVariant,
        name: name.trim() || undefined,
        harness_configs: harnessConfigs,
      });
```

And inside the multi-cell `Promise.allSettled(cells.map(...))`:

```ts
      launch.mutateAsync({
        task_id: cell.taskId,
        executor_id: cell.executorId,
        harness_ids: selectedHarnesses,
        model: cell.modelId,
        runs_per_variant: runsPerVariant,
        batch_id: batchId,
        batch_label: label,
        harness_configs: harnessConfigs,
      }),
```

- [ ] **Step 5: Update the harness chip label so the user sees the new name**

In the same file find the JSX that prints each harness chip's label. The chip currently uses the raw `harness.id` (or similar). Either:

- If the harness list from `useHarnesses()` already exposes a display name field (check by reading 3-4 lines of the chip render block first), no change needed beyond confirming `agent_instructions` will display as the backend's `Description()` — which after Task 5 says "Lay down user-supplied CLAUDE.md…".
- Otherwise, add a simple display-name overlay:

```ts
function harnessDisplayName(id: string): string {
  if (id === 'agent_instructions') return 'Agent instructions';
  return id;
}
```

…and call `harnessDisplayName(h.id)` in the chip label.

Read the chip render code (`grep -n 'selectedHarnesses\|harness\.id' frontend/src/pages/diagnostic/launch.tsx`) to decide which approach the existing structure prefers.

- [ ] **Step 6: Run typecheck + test + build**

```bash
cd frontend && npm run lint && npm test -- --run 2>&1 | tail -5 && npm run build 2>&1 | tail -3
```
Expected: lint exits 0; tests all PASS; build succeeds.

- [ ] **Step 7: Commit**

```bash
git add frontend/src/pages/diagnostic/launch.tsx
git commit -m "Launcher renders harness config panels, gates submit, ships configs"
```

---

## Task 9 — Run inspect card

**Files:**
- Modify: `frontend/src/pages/runs/inspect.tsx`

- [ ] **Step 1: Locate where the page already fetches the run + its variant**

```bash
grep -n 'useRun\|variant\.\|harness_config' frontend/src/pages/runs/inspect.tsx | head -20
```

If `useRun` or a similar hook already loads the variant, you have variant in scope (likely via `run.variant` or a separate `useVariant`). If only the run is available, fetch the variant with the existing `useVariant` hook (`frontend/src/lib/hooks.ts` — verify by grepping).

- [ ] **Step 2: Add the card**

Above the transcript section in `inspect.tsx`, insert:

```tsx
      {variant?.harness_config?.agent_instructions != null && (
        <AgentInstructionsCard
          content={String((variant.harness_config.agent_instructions as { content?: string }).content ?? '')}
        />
      )}
```

And at the bottom of the file add the helper component:

```tsx
function AgentInstructionsCard({ content }: { content: string }) {
  const [open, setOpen] = useState(true);
  if (!content.trim()) return null;
  return (
    <Card>
      <div className="flex items-center justify-between">
        <div>
          <div className="text-sm font-semibold text-fg">Agent instructions used in this run</div>
          <div className="text-xs text-fg-muted">Laid down as CLAUDE.md in the workspace before the agent ran.</div>
        </div>
        <button
          type="button"
          onClick={() => setOpen((v) => !v)}
          className="text-xs text-fg-muted hover:text-fg"
        >
          {open ? 'Hide' : 'Show'}
        </button>
      </div>
      {open && (
        <pre className="mt-3 max-h-64 overflow-auto rounded-md border border-border bg-bg p-3 font-mono text-xs text-fg">
          {content}
        </pre>
      )}
    </Card>
  );
}
```

(Match existing import patterns — `Card` import probably already exists. If `useState` isn't imported in this file, add it.)

- [ ] **Step 3: Run typecheck + tests**

```bash
cd frontend && npm run lint && npm test -- --run 2>&1 | tail -5
```
Expected: lint clean, tests pass.

- [ ] **Step 4: Commit**

```bash
git add frontend/src/pages/runs/inspect.tsx
git commit -m "Run inspect: surface the agent_instructions content used in the run"
```

---

## Task 10 — Full sanity pass

**Files:** none

- [ ] **Step 1: All backend tests**

```bash
cd engine && go test ./...
```
Expected: every package green.

- [ ] **Step 2: All frontend checks**

```bash
cd frontend && npm run lint && npm run build && npm test -- --run 2>&1 | tail -10 && npm run check:tokens
```
Expected: all four exit 0.

---

## Task 11 — Manual verification

**Files:** none (runtime verification)

- [ ] **Step 1: Start engine + frontend**

```bash
lsof -ti tcp:8080 | xargs -r kill 2>/dev/null
lsof -ti tcp:5173 | xargs -r kill 2>/dev/null
(cd engine && nohup go run cmd/server/main.go > /tmp/engine.log 2>&1 &)
(cd frontend && nohup npm run dev > /tmp/vite.log 2>&1 &)
sleep 8
curl -s http://localhost:8080/api/health | python3 -c "import json,sys; print('engine=',json.load(sys.stdin).get('ok'))"
curl -s -o /dev/null -w 'vite=%{http_code}\n' http://localhost:5173/
```
Expected: `engine= True`, `vite=200`.

- [ ] **Step 2: Browser walkthrough**

Visit `http://localhost:5173/diagnostic/launch`. Confirm:

1. The harness list shows `agent_instructions` (label preferably "Agent instructions"). The old `claudemd` chip is gone.
2. Selecting `agent_instructions` reveals an "Agent instructions (laid down as CLAUDE.md)" textarea below the harness chip row.
3. Without typing anything in the textarea, the Launch button is disabled with the label "Type agent instructions".
4. After typing `# rules\nbe concise` in the textarea, the Launch button enables. Launch with a 1-task × 1-harness × 1-exec × 1-model selection.
5. Open the resulting run in the Inspector. The "Agent instructions used in this run" card appears above the transcript, showing the typed content.
6. Confirm the sandbox actually got the file by spot-checking the log (`/tmp/engine.log` should mention `CLAUDE.md` written), or by visiting the Compare view if it now shows the artifact.

- [ ] **Step 3: API smoke**

```bash
curl -s -X POST http://localhost:8080/api/diagnostic/launch \
  -H 'Content-Type: application/json' \
  -d '{
    "task_id":"brownfield-fix-async-race",
    "executor_id":"opencode",
    "harness_ids":["agent_instructions"],
    "model":"opencode/deepseek-v4-flash-free",
    "runs_per_variant":1,
    "harness_configs": {"agent_instructions": {"content": "# rules\nbe concise"}}
  }'
```
Expected: `{"experiment_id":"..."}`. The experiment runs with the user-supplied CLAUDE.md.

- [ ] **Step 4: Empty-content rejection**

```bash
curl -s -X POST http://localhost:8080/api/diagnostic/launch \
  -H 'Content-Type: application/json' \
  -d '{
    "task_id":"brownfield-fix-async-race",
    "executor_id":"opencode",
    "harness_ids":["agent_instructions"],
    "model":"opencode/deepseek-v4-flash-free",
    "runs_per_variant":1
  }'
```
Expected: 202 returned, but the resulting run quickly transitions to `failed` with `error_message` mentioning `ErrAgentInstructionsContentMissing` or "cfg.agent_instructions.content is empty".

- [ ] **Step 5: Stop servers**

```bash
lsof -ti tcp:8080 | xargs -r kill 2>/dev/null
lsof -ti tcp:5173 | xargs -r kill 2>/dev/null
```

---

## Task 12 — Push, open PR, request review, watch CI, merge

**Files:** none

- [ ] **Step 1: Push**

```bash
git push -u origin feature/agent-instructions-harness
```

- [ ] **Step 2: Open PR**

```bash
gh pr create --title "Agent instructions harness + per-variant config foundation" --body "$(cat <<'EOF'
## Summary

Project 1 of the harness refresh: replaces the task-bundled \`claudemd\` harness with a user-typed \`agent_instructions\` harness, and lays down the per-variant config plumbing that Multi-Agent (Project 2) and Spec-Kit catalog (Project 3) will reuse.

- **Schema**: migration 020 adds nullable \`variants.harness_config_json\` (opaque JSON, keyed by harness id).
- **Backend**: \`Harness.Setup\` gains a \`cfg map[string]any\` parameter; the 4 unchanged harnesses ignore it. The renamed \`agent_instructions\` harness (formerly \`claudemd\`) reads \`cfg[\"agent_instructions\"][\"content\"]\` and writes it as CLAUDE.md in the workspace; an empty/missing content fails Setup with \`ErrAgentInstructionsContentMissing\`. Three task \`harness_context/claudemd.md\` bundles are deleted (no longer the source of truth).
- **Launch API**: \`/api/diagnostic/launch\` and \`/api/diagnostic/launch-suite\` accept an optional \`harness_configs\` field that is persisted on every spawned variant.
- **Frontend**: new \`<HarnessConfigPanel>\` component; the launcher renders it below every selected harness chip. For \`agent_instructions\` it's an inline textarea. Launch button is disabled until the textarea is non-empty.
- **Inspector**: Run Inspect shows the actual agent instructions used above the transcript.

Spec: [\`docs/superpowers/specs/2026-05-31-agent-instructions-harness-foundation-design.md\`](docs/superpowers/specs/2026-05-31-agent-instructions-harness-foundation-design.md)
Plan: [\`docs/superpowers/plans/2026-05-31-agent-instructions-harness-foundation.md\`](docs/superpowers/plans/2026-05-31-agent-instructions-harness-foundation.md)

## Test plan

- [x] \`go test ./...\` clean (new variant repo round-trip, agent_instructions harness tests, launch API persistence test)
- [x] \`npm run lint\` / \`build\` / \`test\` / \`check:tokens\` clean
- [x] Manual: launcher textarea → CLAUDE.md ends up in the sandbox; empty textarea → Launch disabled; API smoke shows the run failing cleanly with the missing-content sentinel
EOF
)"
```

- [ ] **Step 3: Dispatch the code reviewer**

Per memory `feedback_github_workflow`, every non-trivial PR goes through `feature-dev:code-reviewer`. Dispatch the agent on the branch HEAD and address findings.

- [ ] **Step 4: Watch CI**

Monitor checks until all pass. Push fixes if needed.

- [ ] **Step 5: Squash merge**

```bash
gh pr merge <PR#> --squash --delete-branch
```

Sync local main:

```bash
git fetch origin && git checkout main && git reset --hard origin/main
```

---

## Self-review

**Spec coverage:**
- Schema migration → Task 1
- Go model field → Task 2
- Repo round-trip + variant CRUD → Task 3
- Harness interface change → Task 4
- Rename claudemd → agent_instructions, registry update, drop bundles → Task 5
- Launch API harness_configs pass-through → Task 6
- Frontend types + panel component → Task 7
- Launcher integration (state, render, gate, payload, label) → Task 8
- Inspector card → Task 9
- Sanity → Task 10
- Manual → Task 11
- PR + review + CI + merge → Task 12
- Compare view diff → explicitly out of scope per spec; no task.

**No placeholders** — every step has runnable commands, exact file paths, and complete code blocks.

**Type consistency** — `HarnessConfig map[string]any` is the canonical Go type; `harness_config?: Record<string, unknown>` is the matching TS shape. The harness id literal `"agent_instructions"` and constant `AgentInstructionsHarnessID` are used consistently. The cfg key in the panel (`'agent_instructions'`), in the harness (`agentInstructionsConfigKey = "agent_instructions"`), and in the test JSON (`"agent_instructions"`) all match.

**Ordering** — Schema → model → repo → harness interface → harness rename → API → frontend types/component → launcher integration → inspector → sanity/manual/PR. Each task is testable on its own and never references a not-yet-built dependency.
