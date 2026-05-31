# Multi-agent harness Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the hardcoded two-role `planner_coder` harness with a user-configurable `multiagent` harness (1–5 sequential roles with `{{TASK}}` / `{{PREV_OUTPUT}}` substitution), and surface each turn's role in Run Inspect via a colored left-edge accent + small role badge on the existing `TurnGroupCard`.

**Architecture:** Backend rename + rewrite around a config-driven role loop (uses Project 1's `variants.harness_config_json`, no schema change). Frontend adds a `<MultiAgentForm>` panel registered in `HarnessConfigPanel`'s switch; launcher gates submit until every role is valid; per-cell payload already carries the config map per Project 1. Inspector picks the role off the group's first block and threads `roleAccent(role)` through `TurnGroupCard`'s rail dot + a new header badge.

**Tech Stack:** Go 1.22 (stdlib `testing`, no new deps), React 18 + TypeScript + TanStack Query + Tailwind (all existing).

**Spec:** [`docs/superpowers/specs/2026-05-31-multiagent-harness-design.md`](../specs/2026-05-31-multiagent-harness-design.md)

**Branch:** `feature/multiagent-harness` (created; spec committed at `f4c1bc1`).

---

## File map

| Layer | File | Action |
|---|---|---|
| Harness | `engine/internal/builtin/harness/planner_coder.go` | RENAME → `multiagent.go` + REWRITE body |
| Harness tests | `engine/internal/builtin/harness/planner_coder_test.go` | RENAME → `multiagent_test.go` + REWRITE body |
| Registry | `engine/internal/builtin/harness/registry.go` | MODIFY (`NewPlannerCoder` → `NewMultiAgent`) |
| Frontend types | `frontend/src/lib/types.ts` | MODIFY (add `MultiAgentRole`, `MultiAgentConfig`) |
| Panel switch | `frontend/src/components/launcher/HarnessConfigPanel.tsx` | MODIFY (add `multiagent` case) |
| Multi-agent form | `frontend/src/components/launcher/MultiAgentForm.tsx` | CREATE |
| Multi-agent form test | `frontend/src/components/launcher/MultiAgentForm.test.tsx` | CREATE |
| Validation helper | `frontend/src/components/launcher/multiagent-validate.ts` | CREATE |
| Validation test | `frontend/src/components/launcher/multiagent-validate.test.ts` | CREATE |
| Launcher gate | `frontend/src/pages/diagnostic/launch.tsx` | MODIFY (parallel gate + button label branch) |
| Role accent helper | `frontend/src/components/run-inspector/role-accent.ts` | CREATE |
| Role accent test | `frontend/src/components/run-inspector/role-accent.test.ts` | CREATE |
| Turn group struct | `frontend/src/components/run-inspector/group-turns.ts` | MODIFY (expose `role` on `TurnGroup`) |
| Group card | `frontend/src/components/run-inspector/TurnGroupCard.tsx` | MODIFY (render role badge + accent dot) |
| Group card test | `frontend/src/components/run-inspector/TurnGroupCard.test.tsx` | CREATE (or extend existing if present) |

No schema migration. No new dependencies.

---

## Task 1 — Rename `planner_coder` → `multiagent` (shell only, tests still pass)

This task does the file/identifier rename but keeps the old hardcoded two-role behavior alive temporarily so the suite stays green. Task 2 rewrites the body.

**Files:**
- Rename: `engine/internal/builtin/harness/planner_coder.go` → `multiagent.go`
- Rename: `engine/internal/builtin/harness/planner_coder_test.go` → `multiagent_test.go`
- Modify: `engine/internal/builtin/harness/registry.go`

- [ ] **Step 1: `git mv` both files**

```bash
cd /Users/mustafaselmangunes/Desktop/Frameval
git mv engine/internal/builtin/harness/planner_coder.go engine/internal/builtin/harness/multiagent.go
git mv engine/internal/builtin/harness/planner_coder_test.go engine/internal/builtin/harness/multiagent_test.go
```

- [ ] **Step 2: Rename the type, constructor, harness id, and constants inside the renamed source file**

In `engine/internal/builtin/harness/multiagent.go`, do the following find-and-replace within the file:

- `PlannerCoder` → `MultiAgent` (type name; every occurrence: struct, methods)
- `NewPlannerCoder` → `NewMultiAgent` (constructor name)
- `"planner_coder"` → `"multiagent"` (the Name() return value — exactly one literal)
- Leave the package-level doc comment and the `rolePlanner` / `roleCoder` constants alone for now; Task 2 rewrites them.

- [ ] **Step 3: Update the renamed test file's type references**

In `engine/internal/builtin/harness/multiagent_test.go`, replace:
- `NewPlannerCoder()` → `NewMultiAgent()`
- The `"planner_coder"` literal in `TestPlannerCoderIdentity` → `"multiagent"`
- Rename `TestPlannerCoderIdentity` to `TestMultiAgentIdentity` (just the function name; body keeps the same assertions but checks the new id)

- [ ] **Step 4: Update the registry**

In `engine/internal/builtin/harness/registry.go`, replace `mustRegister(r, NewPlannerCoder())` with `mustRegister(r, NewMultiAgent())`.

Then grep for any remaining production reference:

```bash
grep -rn 'NewPlannerCoder\|"planner_coder"' engine/ --include='*.go'
```

Expected output: empty. If a hit appears, fix it (likely a stale doc comment).

- [ ] **Step 5: Run the full engine test suite**

```bash
cd engine && go test ./...
```

Expected: every package green. The renamed `TestPlannerCoderInvokeIssuesTwoCallsInOrder` and friends still pass because the body still implements the same two-role flow.

- [ ] **Step 6: Commit**

```bash
git add engine/internal/builtin/harness/multiagent.go engine/internal/builtin/harness/multiagent_test.go engine/internal/builtin/harness/registry.go
git commit -m "Rename planner_coder harness to multiagent (shell only, body unchanged)"
```

---

## Task 2 — Multi-agent config + `expandPrompt` helper (TDD)

This adds the config schema, the prompt-substitution helper, and Setup validation. The Invoke loop is rewritten in Task 3.

**Files:**
- Modify: `engine/internal/builtin/harness/multiagent.go`
- Modify: `engine/internal/builtin/harness/multiagent_test.go`

- [ ] **Step 1: Write the failing tests**

Append to `engine/internal/builtin/harness/multiagent_test.go`:

```go
func TestExpandPromptSubstitutions(t *testing.T) {
	cases := []struct {
		name     string
		template string
		vars     map[string]string
		want     string
	}{
		{"replaces TASK", "do {{TASK}}", map[string]string{"TASK": "x"}, "do x"},
		{"replaces PREV_OUTPUT", "prev: {{PREV_OUTPUT}}", map[string]string{"PREV_OUTPUT": "y"}, "prev: y"},
		{"both tokens", "{{TASK}} after {{PREV_OUTPUT}}", map[string]string{"TASK": "a", "PREV_OUTPUT": "b"}, "a after b"},
		{"empty PREV_OUTPUT leaves blank", "{{PREV_OUTPUT}}!", map[string]string{"PREV_OUTPUT": ""}, "!"},
		{"unknown token preserved", "{{TASK}} then {{OTHER}}", map[string]string{"TASK": "x"}, "x then {{OTHER}}"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := expandPrompt(tc.template, tc.vars); got != tc.want {
				t.Errorf("expandPrompt: got %q want %q", got, tc.want)
			}
		})
	}
}

func TestSetupRejectsMissingRoles(t *testing.T) {
	h := NewMultiAgent()
	ws := pkgharness.Workspace{Path: t.TempDir()}
	cases := []map[string]any{
		nil,
		{},
		{"multiagent": map[string]any{}},
		{"multiagent": map[string]any{"roles": []any{}}},
	}
	for i, cfg := range cases {
		if _, err := h.Setup(context.Background(), ws, task.Task{}, pkgharness.Budget{}, cfg); !errors.Is(err, ErrMultiAgentRolesMissing) {
			t.Errorf("case %d: got %v, want ErrMultiAgentRolesMissing", i, err)
		}
	}
}

func TestSetupRejectsInvalidRoles(t *testing.T) {
	h := NewMultiAgent()
	ws := pkgharness.Workspace{Path: t.TempDir()}
	cases := []struct {
		name string
		cfg  map[string]any
	}{
		{"uppercase name", map[string]any{"multiagent": map[string]any{"roles": []any{
			map[string]any{"name": "Planner", "prompt": "x"},
		}}}},
		{"leading digit", map[string]any{"multiagent": map[string]any{"roles": []any{
			map[string]any{"name": "1bad", "prompt": "x"},
		}}}},
		{"empty prompt", map[string]any{"multiagent": map[string]any{"roles": []any{
			map[string]any{"name": "planner", "prompt": ""},
		}}}},
		{"duplicate names", map[string]any{"multiagent": map[string]any{"roles": []any{
			map[string]any{"name": "a", "prompt": "x"},
			map[string]any{"name": "a", "prompt": "y"},
		}}}},
		{"six roles", map[string]any{"multiagent": map[string]any{"roles": []any{
			map[string]any{"name": "a", "prompt": "x"},
			map[string]any{"name": "b", "prompt": "x"},
			map[string]any{"name": "c", "prompt": "x"},
			map[string]any{"name": "d", "prompt": "x"},
			map[string]any{"name": "e", "prompt": "x"},
			map[string]any{"name": "f", "prompt": "x"},
		}}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := h.Setup(context.Background(), ws, task.Task{}, pkgharness.Budget{}, tc.cfg); !errors.Is(err, ErrMultiAgentInvalidRole) {
				t.Errorf("got %v, want ErrMultiAgentInvalidRole", err)
			}
		})
	}
}

func TestSetupAcceptsValidRoles(t *testing.T) {
	h := NewMultiAgent()
	ws := pkgharness.Workspace{Path: t.TempDir()}
	cfg := map[string]any{
		"multiagent": map[string]any{
			"roles": []any{
				map[string]any{"name": "planner", "prompt": "plan {{TASK}}"},
				map[string]any{"name": "coder", "prompt": "code from {{PREV_OUTPUT}}"},
			},
		},
	}
	run, err := h.Setup(context.Background(), ws, task.Task{}, pkgharness.Budget{}, cfg)
	if err != nil {
		t.Fatalf("Setup: %v", err)
	}
	if run.HarnessName != "multiagent" {
		t.Errorf("HarnessName: got %q", run.HarnessName)
	}
}
```

- [ ] **Step 2: Run; tests must fail (helper + sentinels + new Setup don't exist yet)**

```bash
cd engine && go test ./internal/builtin/harness/ -run 'TestExpandPromptSubstitutions|TestSetupRejects|TestSetupAccepts' -v
```

Expected: compile errors about `expandPrompt`, `ErrMultiAgentRolesMissing`, `ErrMultiAgentInvalidRole`.

- [ ] **Step 3: Add the constants, sentinels, types, and the validating Setup body**

Replace the contents of `engine/internal/builtin/harness/multiagent.go` (everything after the `package harness` line) with:

```go
import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/mustafaselman/frameval/engine/pkg/executor"
	"github.com/mustafaselman/frameval/engine/pkg/harness"
	"github.com/mustafaselman/frameval/engine/pkg/task"
)

const (
	// MultiAgentHarnessID is the stable wire id for this harness.
	MultiAgentHarnessID = "multiagent"

	// multiagentConfigKey is the top-level key the harness reads from
	// the per-variant cfg map.
	multiagentConfigKey = "multiagent"

	// multiagentMinRoles / multiagentMaxRoles bound how many sequential
	// roles a single variant may declare. Five keeps the launcher form
	// and the merged transcript readable; lift in a follow-up if a real
	// research need ever emerges.
	multiagentMinRoles = 1
	multiagentMaxRoles = 5
)

// roleNamePattern enforces snake_case ASCII so role names show up
// predictably in logs, JSON, and the role-accent color hash. Empty
// is rejected separately for a clearer error message.
var roleNamePattern = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// ErrMultiAgentRolesMissing surfaces when the launcher's per-variant
// cfg has no roles at all (nil, missing key, empty array). The
// launcher's submit gate prevents this in normal flow; the sentinel
// is the last line of defense for direct API consumers.
var ErrMultiAgentRolesMissing = errors.New(
	"multiagent harness: cfg.multiagent.roles is empty; user must configure roles in the launcher")

// ErrMultiAgentInvalidRole surfaces for any per-role validation
// failure: bad name, empty prompt, duplicate, count outside the
// supported range. The error string carries the specific reason.
var ErrMultiAgentInvalidRole = errors.New("multiagent harness: invalid role")

// MultiAgent runs a user-configured sequence of agent roles. Each role
// has a name and a prompt template; substitutions {{TASK}} and
// {{PREV_OUTPUT}} are resolved before each call. Every emitted turn
// is tagged with the role name so the Inspector can render per-role
// visual cues.
type MultiAgent struct{}

func NewMultiAgent() *MultiAgent { return &MultiAgent{} }

func (h *MultiAgent) Name() string        { return MultiAgentHarnessID }
func (h *MultiAgent) Description() string {
	return "Configurable sequence of N agent roles (1-5). Each role gets its own prompt; outputs flow forward via {{PREV_OUTPUT}}."
}

// role is the internal, validated shape of one configured role.
type role struct {
	Name   string
	Prompt string
}

func (h *MultiAgent) Setup(_ context.Context, ws harness.Workspace, t task.Task, b harness.Budget, cfg map[string]any) (harness.HarnessRun, error) {
	roles, err := extractRoles(cfg)
	if err != nil {
		return harness.HarnessRun{}, err
	}
	return harness.HarnessRun{
		HarnessName: h.Name(),
		Task:        t,
		Workspace:   ws,
		Budget:      b,
		Metadata:    map[string]any{"multiagent.roles": roles},
	}, nil
}

// Invoke is rewritten in Task 3; the placeholder below keeps the
// build green while we land the validation + helper changes.
func (h *MultiAgent) Invoke(ctx context.Context, run harness.HarnessRun, exec executor.AgentExecutor) (*executor.RunResult, error) {
	return nil, errors.New("multiagent harness: Invoke not yet implemented in this task")
}

func (h *MultiAgent) Teardown(_ context.Context, _ harness.HarnessRun) error { return nil }

func extractRoles(cfg map[string]any) ([]role, error) {
	if cfg == nil {
		return nil, ErrMultiAgentRolesMissing
	}
	sub, ok := cfg[multiagentConfigKey].(map[string]any)
	if !ok {
		return nil, ErrMultiAgentRolesMissing
	}
	rawList, ok := sub["roles"].([]any)
	if !ok || len(rawList) == 0 {
		return nil, ErrMultiAgentRolesMissing
	}
	if len(rawList) < multiagentMinRoles || len(rawList) > multiagentMaxRoles {
		return nil, fmt.Errorf("%w: role count %d outside supported range [%d, %d]", ErrMultiAgentInvalidRole, len(rawList), multiagentMinRoles, multiagentMaxRoles)
	}
	seen := make(map[string]struct{}, len(rawList))
	out := make([]role, 0, len(rawList))
	for i, raw := range rawList {
		m, ok := raw.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("%w: role %d is not an object", ErrMultiAgentInvalidRole, i)
		}
		name, _ := m["name"].(string)
		prompt, _ := m["prompt"].(string)
		if !roleNamePattern.MatchString(name) {
			return nil, fmt.Errorf("%w: role %d name %q must match %s", ErrMultiAgentInvalidRole, i, name, roleNamePattern)
		}
		if strings.TrimSpace(prompt) == "" {
			return nil, fmt.Errorf("%w: role %d (%s) has empty prompt", ErrMultiAgentInvalidRole, i, name)
		}
		if _, dup := seen[name]; dup {
			return nil, fmt.Errorf("%w: role name %q appears more than once", ErrMultiAgentInvalidRole, name)
		}
		seen[name] = struct{}{}
		out = append(out, role{Name: name, Prompt: prompt})
	}
	return out, nil
}

// expandPrompt replaces {{TASK}} and {{PREV_OUTPUT}} literally with
// the corresponding entries in vars. Unknown {{...}} tokens are
// preserved as-is — the agent may legitimately write them and we
// don't want to silently mangle their text.
func expandPrompt(template string, vars map[string]string) string {
	out := template
	out = strings.ReplaceAll(out, "{{TASK}}", vars["TASK"])
	out = strings.ReplaceAll(out, "{{PREV_OUTPUT}}", vars["PREV_OUTPUT"])
	return out
}
```

Note: this replaces the existing `Invoke`/`plannerPrompt`/`coderPrompt` etc. with stubs. The old tests that exercise the planner+coder flow (`TestPlannerCoderInvokeIssuesTwoCallsInOrder`, etc.) will FAIL after this step. That's intentional — Task 3 rewrites them along with `Invoke`.

- [ ] **Step 4: Run the validation tests; they must pass**

```bash
cd engine && go test ./internal/builtin/harness/ -run 'TestExpandPromptSubstitutions|TestSetupRejects|TestSetupAccepts' -v
```

Expected: all 4 PASS.

- [ ] **Step 5: Confirm the pre-existing two-role flow tests now FAIL (expected, Task 3 fixes them)**

```bash
cd engine && go test ./internal/builtin/harness/ -run 'TestPlannerCoder' -v
```

Expected: every old test errors with "Invoke not yet implemented in this task" or "compile error". Note them; Task 3 deletes / rewrites them.

- [ ] **Step 6: Commit**

```bash
git add engine/internal/builtin/harness/multiagent.go engine/internal/builtin/harness/multiagent_test.go
git commit -m "multiagent: add config schema + validation + expandPrompt helper"
```

(Yes, the harness tests are red right now. Task 3 lands immediately after.)

---

## Task 3 — Multi-agent `Invoke` loop + transcript role tagging (TDD)

**Files:**
- Modify: `engine/internal/builtin/harness/multiagent.go`
- Modify: `engine/internal/builtin/harness/multiagent_test.go`

- [ ] **Step 1: Delete the old two-role tests, write the new loop tests**

In `engine/internal/builtin/harness/multiagent_test.go`, **delete** every test that references `rolePlanner`, `roleCoder`, or the `twoStageExecutor` type. Then **delete** the `twoStageExecutor` type definition itself.

Replace with a generic N-role test fixture + the new tests. Append to the file:

```go
// recordingExecutor lets a test assert what prompts the harness sent
// to each role and what the merged transcript looks like.
type recordingExecutor struct {
	calls    []executor.RunConfig
	results  map[string]*executor.RunResult // keyed by role
	errs     map[string]error
}

func (e *recordingExecutor) Name() string                                          { return "recording" }
func (e *recordingExecutor) SupportedModes() []executor.ExecutionMode              { return []executor.ExecutionMode{executor.ExecutionModeCLI} }
func (e *recordingExecutor) ParseTranscript(raw []byte) ([]executor.ParsedTurn, error) { return nil, nil }
func (e *recordingExecutor) Execute(_ context.Context, cfg executor.RunConfig) (*executor.RunResult, error) {
	e.calls = append(e.calls, cfg)
	if r, ok := e.results[cfg.Role]; ok {
		return r, e.errs[cfg.Role]
	}
	return &executor.RunResult{}, e.errs[cfg.Role]
}

func cfgWithTwoRoles() map[string]any {
	return map[string]any{
		"multiagent": map[string]any{
			"roles": []any{
				map[string]any{"name": "planner", "prompt": "plan: {{TASK}}"},
				map[string]any{"name": "coder", "prompt": "code from {{PREV_OUTPUT}} for {{TASK}}"},
			},
		},
	}
}

func TestInvokeRunsRolesSequentially(t *testing.T) {
	h := NewMultiAgent()
	ws := pkgharness.Workspace{Path: t.TempDir()}
	tk := task.Task{TaskPrompt: "scaffold a CLI"}
	exec := &recordingExecutor{
		results: map[string]*executor.RunResult{
			"planner": {
				RawOutput:   "planner-raw",
				ParsedTurns: []executor.ParsedTurn{{Role: "assistant", Content: "## plan\nuse click"}},
			},
			"coder": {RawOutput: "coder-raw"},
		},
	}
	run, _ := h.Setup(context.Background(), ws, tk, pkgharness.Budget{}, cfgWithTwoRoles())
	if _, err := h.Invoke(context.Background(), run, exec); err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	if len(exec.calls) != 2 {
		t.Fatalf("call count: got %d, want 2", len(exec.calls))
	}
	if exec.calls[0].Role != "planner" || exec.calls[1].Role != "coder" {
		t.Errorf("role order: got %q, %q", exec.calls[0].Role, exec.calls[1].Role)
	}
	if exec.calls[0].Prompt != "plan: scaffold a CLI" {
		t.Errorf("planner prompt: got %q", exec.calls[0].Prompt)
	}
	wantCoderPrompt := "code from ## plan\nuse click for scaffold a CLI"
	if exec.calls[1].Prompt != wantCoderPrompt {
		t.Errorf("coder prompt: got %q want %q", exec.calls[1].Prompt, wantCoderPrompt)
	}
}

func TestInvokeTagsTurnsWithRole(t *testing.T) {
	h := NewMultiAgent()
	ws := pkgharness.Workspace{Path: t.TempDir()}
	exec := &recordingExecutor{
		results: map[string]*executor.RunResult{
			"planner": {
				RawOutput:   "p",
				ParsedTurns: []executor.ParsedTurn{{Role: "assistant", Content: "plan"}},
			},
			"coder": {
				RawOutput:   "c",
				ParsedTurns: []executor.ParsedTurn{{Role: "assistant", Content: "code"}},
			},
		},
	}
	run, _ := h.Setup(context.Background(), ws, task.Task{TaskPrompt: "t"}, pkgharness.Budget{}, cfgWithTwoRoles())
	merged, err := h.Invoke(context.Background(), run, exec)
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	if len(merged.ParsedTurns) != 2 {
		t.Fatalf("merged turns: got %d", len(merged.ParsedTurns))
	}
	if merged.ParsedTurns[0].Role != "planner" || merged.ParsedTurns[0].Stage != "planner" {
		t.Errorf("first turn role/stage: got %+v", merged.ParsedTurns[0])
	}
	if merged.ParsedTurns[1].Role != "coder" || merged.ParsedTurns[1].Stage != "coder" {
		t.Errorf("second turn role/stage: got %+v", merged.ParsedTurns[1])
	}
	if !strings.Contains(merged.RawOutput, "--- Role: planner ---") || !strings.Contains(merged.RawOutput, "--- Role: coder ---") {
		t.Errorf("merged raw output missing role markers: %q", merged.RawOutput)
	}
}

func TestInvokeContinuesPastSingleRoleFailure(t *testing.T) {
	h := NewMultiAgent()
	ws := pkgharness.Workspace{Path: t.TempDir()}
	exec := &recordingExecutor{
		results: map[string]*executor.RunResult{
			"planner": {RawOutput: "p"},
			"coder":   {RawOutput: "c"},
		},
		errs: map[string]error{"planner": errors.New("boom")},
	}
	run, _ := h.Setup(context.Background(), ws, task.Task{TaskPrompt: "t"}, pkgharness.Budget{}, cfgWithTwoRoles())
	merged, err := h.Invoke(context.Background(), run, exec)
	if err == nil {
		t.Fatal("expected accumulated error from planner failure")
	}
	if !strings.Contains(err.Error(), "planner") {
		t.Errorf("error string should mention planner: %v", err)
	}
	if len(exec.calls) != 2 {
		t.Errorf("coder should still run after planner failure: got %d calls", len(exec.calls))
	}
	if merged == nil {
		t.Fatal("merged result should not be nil even on partial failure")
	}
}

func TestInvokeAbortsOnContextCancel(t *testing.T) {
	h := NewMultiAgent()
	ws := pkgharness.Workspace{Path: t.TempDir()}
	exec := &recordingExecutor{
		errs: map[string]error{"planner": context.Canceled},
	}
	run, _ := h.Setup(context.Background(), ws, task.Task{}, pkgharness.Budget{}, cfgWithTwoRoles())
	_, err := h.Invoke(context.Background(), run, exec)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
	if len(exec.calls) != 1 {
		t.Errorf("should bail before coder: got %d calls", len(exec.calls))
	}
}
```

- [ ] **Step 2: Run; the four new Invoke tests must fail**

```bash
cd engine && go test ./internal/builtin/harness/ -run 'TestInvoke' -v
```

Expected: every test fails with "Invoke not yet implemented in this task" (the Task 2 stub).

- [ ] **Step 3: Implement the Invoke loop**

In `engine/internal/builtin/harness/multiagent.go`, replace the stub `Invoke` method with the loop, and add the helpers it uses (paste them right above `Invoke` if helpful — keep them unexported):

```go
func (h *MultiAgent) Invoke(ctx context.Context, run harness.HarnessRun, exec executor.AgentExecutor) (*executor.RunResult, error) {
	roles, ok := run.Metadata["multiagent.roles"].([]role)
	if !ok || len(roles) == 0 {
		return nil, ErrMultiAgentRolesMissing
	}

	merged := &executor.RunResult{}
	var rawBuilder strings.Builder
	var prevOutput string
	var roleErrors []error

	for i, r := range roles {
		prompt := expandPrompt(r.Prompt, map[string]string{
			"TASK":        run.Task.TaskPrompt,
			"PREV_OUTPUT": prevOutput,
		})
		result, err := exec.Execute(ctx, harness.MergeConfig(run.BaseRunConfig, executor.RunConfig{
			Prompt:        prompt,
			WorkspacePath: run.Workspace.Path,
			Role:          r.Name,
			Stage:         r.Name,
		}))
		if result == nil {
			result = &executor.RunResult{}
		}

		// Always merge whatever this role produced (raw + tagged turns)
		// so the user has a transcript even on partial failure.
		rawBuilder.WriteString("--- Role: ")
		rawBuilder.WriteString(r.Name)
		rawBuilder.WriteString(" ---\n")
		rawBuilder.WriteString(result.RawOutput)
		if !strings.HasSuffix(result.RawOutput, "\n") {
			rawBuilder.WriteString("\n")
		}
		for _, turn := range result.ParsedTurns {
			turn.Role = r.Name
			turn.Stage = r.Name
			merged.ParsedTurns = append(merged.ParsedTurns, turn)
		}

		// ctx cancellation aborts immediately — no point spinning up
		// the next role when the user / scheduler told us to stop.
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			merged.RawOutput = rawBuilder.String()
			return merged, err
		}

		if err != nil {
			roleErrors = append(roleErrors, fmt.Errorf("role %q: %w", r.Name, err))
			prevOutput = fmt.Sprintf("(role %s failed; proceeding without its output)", r.Name)
			continue
		}

		prevOutput = extractLastAssistantOutput(result)
		_ = i // index reserved for future per-role policy
	}

	merged.RawOutput = rawBuilder.String()
	if len(roleErrors) > 0 {
		return merged, fmt.Errorf("multiagent: %w", errors.Join(roleErrors...))
	}
	return merged, nil
}

// extractLastAssistantOutput returns the last assistant turn's content
// from a RunResult, falling back to RawOutput when no assistant turn is
// present. Used to feed the next role's {{PREV_OUTPUT}} substitution.
func extractLastAssistantOutput(r *executor.RunResult) string {
	if r == nil {
		return ""
	}
	for i := len(r.ParsedTurns) - 1; i >= 0; i-- {
		if strings.EqualFold(r.ParsedTurns[i].Role, "assistant") {
			content := strings.TrimSpace(r.ParsedTurns[i].Content)
			if content != "" {
				return content
			}
		}
	}
	return strings.TrimSpace(r.RawOutput)
}
```

Also delete the now-unused `plannerPrompt`, `coderPrompt`, `extractPlanFromResult`, and `mergeRoleTranscripts` functions if they're still in the file (the Task 2 rewrite left them out; if any survived, drop them now). Also delete the `rolePlanner` / `roleCoder` constants.

- [ ] **Step 4: Run the Invoke tests; they must pass**

```bash
cd engine && go test ./internal/builtin/harness/ -run 'TestInvoke' -v
```

Expected: all 4 PASS.

- [ ] **Step 5: Run the full engine test suite**

```bash
cd engine && go test ./...
```

Expected: every package green. If anything else still references the deleted `plannerPrompt` / `coderPrompt` / `mergeRoleTranscripts`, the compiler tells you exactly where; fix it (no production references should exist).

- [ ] **Step 6: Commit**

```bash
git add engine/internal/builtin/harness/multiagent.go engine/internal/builtin/harness/multiagent_test.go
git commit -m "multiagent: implement N-role sequential Invoke loop with role-tagged transcript"
```

---

## Task 4 — Frontend types

**Files:**
- Modify: `frontend/src/lib/types.ts`

- [ ] **Step 1: Add `MultiAgentRole` and `MultiAgentConfig` types**

Append to `frontend/src/lib/types.ts` (anywhere — convention is near the other launcher request types):

```ts
export type MultiAgentRole = {
  name: string;
  prompt: string;
};

export type MultiAgentConfig = {
  roles: MultiAgentRole[];
};
```

- [ ] **Step 2: Typecheck**

```bash
cd frontend && npm run lint
```

Expected: exits 0.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/lib/types.ts
git commit -m "Add MultiAgentRole + MultiAgentConfig types"
```

---

## Task 5 — Validation helper + tests (TDD)

The launcher needs to know when the multiagent config is "ready" (matches the same constraints the backend's Setup enforces). Pull that into a pure helper so it's testable without rendering.

**Files:**
- Create: `frontend/src/components/launcher/multiagent-validate.ts`
- Create: `frontend/src/components/launcher/multiagent-validate.test.ts`

- [ ] **Step 1: Write the failing tests**

Create `frontend/src/components/launcher/multiagent-validate.test.ts`:

```ts
import { describe, expect, it } from 'vitest';
import { validateMultiAgentConfig } from './multiagent-validate';

describe('validateMultiAgentConfig', () => {
  it('rejects undefined or empty configs', () => {
    expect(validateMultiAgentConfig(undefined)).toBe(false);
    expect(validateMultiAgentConfig({ roles: [] })).toBe(false);
  });

  it('rejects more than 5 roles', () => {
    const six = Array.from({ length: 6 }, (_, i) => ({ name: `r${i}`, prompt: 'x' }));
    expect(validateMultiAgentConfig({ roles: six })).toBe(false);
  });

  it('rejects invalid role names', () => {
    expect(validateMultiAgentConfig({ roles: [{ name: 'Planner', prompt: 'x' }] })).toBe(false); // uppercase
    expect(validateMultiAgentConfig({ roles: [{ name: '1bad', prompt: 'x' }] })).toBe(false); // leading digit
    expect(validateMultiAgentConfig({ roles: [{ name: '', prompt: 'x' }] })).toBe(false);
    expect(validateMultiAgentConfig({ roles: [{ name: 'has space', prompt: 'x' }] })).toBe(false);
  });

  it('rejects empty / whitespace-only prompts', () => {
    expect(validateMultiAgentConfig({ roles: [{ name: 'a', prompt: '' }] })).toBe(false);
    expect(validateMultiAgentConfig({ roles: [{ name: 'a', prompt: '   \n\t  ' }] })).toBe(false);
  });

  it('rejects duplicate role names', () => {
    expect(validateMultiAgentConfig({ roles: [
      { name: 'a', prompt: 'x' },
      { name: 'a', prompt: 'y' },
    ] })).toBe(false);
  });

  it('accepts a clean 1-5 role config', () => {
    expect(validateMultiAgentConfig({ roles: [{ name: 'planner', prompt: 'x' }] })).toBe(true);
    expect(validateMultiAgentConfig({ roles: [
      { name: 'planner', prompt: 'plan {{TASK}}' },
      { name: 'coder', prompt: 'code from {{PREV_OUTPUT}}' },
    ] })).toBe(true);
  });
});
```

- [ ] **Step 2: Run; tests must fail (module missing)**

```bash
cd frontend && npx vitest run src/components/launcher/multiagent-validate.test.ts
```

Expected: "Cannot find module './multiagent-validate'".

- [ ] **Step 3: Implement the helper**

Create `frontend/src/components/launcher/multiagent-validate.ts`:

```ts
import type { MultiAgentConfig } from '../../lib/types';

const NAME_PATTERN = /^[a-z][a-z0-9_]*$/;
const MIN_ROLES = 1;
const MAX_ROLES = 5;

/**
 * True when the supplied config matches every backend constraint:
 * - between 1 and 5 roles
 * - every name is snake_case ASCII (`^[a-z][a-z0-9_]*$`)
 * - every prompt is non-empty after trimming
 * - all names are unique
 *
 * Pure function — no DOM, no React, no side effects. The launcher
 * uses it as the submit gate; mirror the same validation rules the
 * backend's extractRoles helper enforces so the UI never lets users
 * submit configs the harness would reject.
 */
export function validateMultiAgentConfig(value: MultiAgentConfig | undefined): boolean {
  if (!value || !Array.isArray(value.roles)) return false;
  const { roles } = value;
  if (roles.length < MIN_ROLES || roles.length > MAX_ROLES) return false;
  const seen = new Set<string>();
  for (const r of roles) {
    if (!r || typeof r.name !== 'string' || typeof r.prompt !== 'string') return false;
    if (!NAME_PATTERN.test(r.name)) return false;
    if (r.prompt.trim().length === 0) return false;
    if (seen.has(r.name)) return false;
    seen.add(r.name);
  }
  return true;
}
```

- [ ] **Step 4: Re-run; tests must pass**

```bash
cd frontend && npx vitest run src/components/launcher/multiagent-validate.test.ts
```

Expected: all PASS (6 cases).

- [ ] **Step 5: Commit**

```bash
git add frontend/src/components/launcher/multiagent-validate.ts frontend/src/components/launcher/multiagent-validate.test.ts
git commit -m "Add validateMultiAgentConfig pure helper + Vitest cases"
```

---

## Task 6 — `<MultiAgentForm>` component (TDD)

**Files:**
- Create: `frontend/src/components/launcher/MultiAgentForm.tsx`
- Create: `frontend/src/components/launcher/MultiAgentForm.test.tsx`

- [ ] **Step 1: Write the failing test**

Create `frontend/src/components/launcher/MultiAgentForm.test.tsx`:

```tsx
import { describe, expect, it, vi } from 'vitest';
import { render, screen, fireEvent, within } from '@testing-library/react';
import { MultiAgentForm } from './MultiAgentForm';

describe('MultiAgentForm', () => {
  it('seeds planner + coder roles when value is undefined', () => {
    const onChange = vi.fn();
    render(<MultiAgentForm value={undefined} onChange={onChange} />);
    // The first render should call onChange once with the seed value
    // so the parent picks up the default.
    expect(onChange).toHaveBeenCalledTimes(1);
    const seed = onChange.mock.calls[0][0];
    expect(seed.roles).toHaveLength(2);
    expect(seed.roles[0].name).toBe('planner');
    expect(seed.roles[1].name).toBe('coder');
  });

  it('renders one row per role and edits name / prompt round-trip', () => {
    const onChange = vi.fn();
    render(<MultiAgentForm value={{ roles: [{ name: 'planner', prompt: 'p1' }] }} onChange={onChange} />);
    const nameInput = screen.getByLabelText(/role 1 name/i) as HTMLInputElement;
    expect(nameInput.value).toBe('planner');
    fireEvent.change(nameInput, { target: { value: 'plotter' } });
    expect(onChange).toHaveBeenCalledWith({ roles: [{ name: 'plotter', prompt: 'p1' }] });

    const promptArea = screen.getByLabelText(/role 1 prompt/i) as HTMLTextAreaElement;
    fireEvent.change(promptArea, { target: { value: 'new prompt' } });
    expect(onChange).toHaveBeenLastCalledWith({ roles: [{ name: 'planner', prompt: 'new prompt' }] });
  });

  it('adds a new empty role up to the 5-role cap', () => {
    const onChange = vi.fn();
    const four = Array.from({ length: 4 }, (_, i) => ({ name: `r${i}`, prompt: 'x' }));
    render(<MultiAgentForm value={{ roles: four }} onChange={onChange} />);
    fireEvent.click(screen.getByRole('button', { name: /add role/i }));
    expect(onChange).toHaveBeenCalled();
    const last = onChange.mock.calls[onChange.mock.calls.length - 1][0];
    expect(last.roles).toHaveLength(5);

    // Re-render with the new five-role value; Add button should now be disabled.
    onChange.mockClear();
    render(<MultiAgentForm value={last} onChange={onChange} />);
    expect(screen.getByRole('button', { name: /add role/i })).toBeDisabled();
  });

  it('removes a role; Remove is disabled when only one role remains', () => {
    const onChange = vi.fn();
    const two = { roles: [{ name: 'a', prompt: 'x' }, { name: 'b', prompt: 'y' }] };
    const { rerender } = render(<MultiAgentForm value={two} onChange={onChange} />);
    const removeButtons = screen.getAllByRole('button', { name: /remove role/i });
    fireEvent.click(removeButtons[0]);
    const after = onChange.mock.calls[onChange.mock.calls.length - 1][0];
    expect(after.roles).toEqual([{ name: 'b', prompt: 'y' }]);

    rerender(<MultiAgentForm value={after} onChange={onChange} />);
    expect(screen.getByRole('button', { name: /remove role/i })).toBeDisabled();
  });

  it('reorders roles with up / down arrows', () => {
    const onChange = vi.fn();
    const two = { roles: [{ name: 'a', prompt: 'x' }, { name: 'b', prompt: 'y' }] };
    render(<MultiAgentForm value={two} onChange={onChange} />);
    fireEvent.click(screen.getByRole('button', { name: /move role 1 down/i }));
    expect(onChange).toHaveBeenLastCalledWith({ roles: [{ name: 'b', prompt: 'y' }, { name: 'a', prompt: 'x' }] });
  });
});
```

- [ ] **Step 2: Run; tests must fail (module missing)**

```bash
cd frontend && npx vitest run src/components/launcher/MultiAgentForm.test.tsx
```

Expected: "Cannot find module './MultiAgentForm'".

- [ ] **Step 3: Implement the component**

Create `frontend/src/components/launcher/MultiAgentForm.tsx`:

```tsx
import { useEffect } from 'react';
import type { MultiAgentConfig, MultiAgentRole } from '../../lib/types';

const MAX_ROLES = 5;

const SEED: MultiAgentConfig = {
  roles: [
    {
      name: 'planner',
      prompt: [
        'You are the PLANNER role in a sequential workflow. Output must be a written plan only — do not write code or modify files yet.',
        'Produce a markdown response with three sections:',
        '',
        '## Approach',
        '(one paragraph: what overall strategy will solve this task?)',
        '',
        '## Files to change',
        '(bulleted list of file paths the implementer should touch)',
        '',
        '## Test strategy',
        '(bulleted list of how the implementer should verify correctness)',
        '',
        'Task:',
        '{{TASK}}',
      ].join('\n'),
    },
    {
      name: 'coder',
      prompt: [
        'You are the CODER role. A previous role has produced an implementation plan; follow it where reasonable, deviate with justification if you find it is wrong.',
        '',
        '## Plan from previous role',
        '{{PREV_OUTPUT}}',
        '',
        '## Task',
        '{{TASK}}',
      ].join('\n'),
    },
  ],
};

interface FormProps {
  value: MultiAgentConfig | undefined;
  onChange: (next: MultiAgentConfig) => void;
}

/**
 * MultiAgentForm — N-role configuration panel rendered inside
 * HarnessConfigPanel when `multiagent` is selected. Each role row has
 * a name input, a prompt textarea, reorder buttons, and a remove
 * button. The form enforces the 1..5 role cap visually (Add disabled
 * at 5; Remove disabled at 1). It does NOT do format validation
 * inline — validateMultiAgentConfig gates the Launch button instead.
 *
 * On first render with undefined value, the form emits a default
 * planner+coder pair via onChange so the parent picks up the seed.
 */
export function MultiAgentForm({ value, onChange }: FormProps) {
  useEffect(() => {
    if (value === undefined) {
      onChange(SEED);
    }
    // Only fire on the mount; once `value` is defined the parent owns it.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const roles = value?.roles ?? SEED.roles;

  const update = (next: MultiAgentRole[]) => onChange({ roles: next });
  const editName = (i: number, name: string) =>
    update(roles.map((r, j) => (i === j ? { ...r, name } : r)));
  const editPrompt = (i: number, prompt: string) =>
    update(roles.map((r, j) => (i === j ? { ...r, prompt } : r)));
  const remove = (i: number) => update(roles.filter((_, j) => j !== i));
  const add = () =>
    update([...roles, { name: `role${roles.length + 1}`, prompt: '' }]);
  const move = (i: number, dir: 'up' | 'down') => {
    const j = dir === 'up' ? i - 1 : i + 1;
    if (j < 0 || j >= roles.length) return;
    const next = [...roles];
    [next[i], next[j]] = [next[j], next[i]];
    update(next);
  };

  return (
    <div className="mt-2 rounded-md border border-border bg-bg-elev-1 p-3">
      <div className="mb-2 text-xs uppercase tracking-wider text-fg-muted">
        Multi-agent roles
      </div>
      <p className="mb-3 text-xs text-fg-muted">
        Sequential roles — each runs after the previous one finishes. Available substitutions in prompts:{' '}
        <code className="font-mono text-fg">{'{{TASK}}'}</code> and{' '}
        <code className="font-mono text-fg">{'{{PREV_OUTPUT}}'}</code>.
      </p>
      <div className="space-y-3">
        {roles.map((r, i) => (
          <div key={i} className="rounded-md border border-border bg-bg p-3">
            <div className="mb-2 flex items-center gap-2">
              <span className="text-xs uppercase tracking-wider text-fg-muted">Role {i + 1}</span>
              <input
                aria-label={`Role ${i + 1} name`}
                className="flex-1 rounded-md border border-border bg-bg-elev-1 px-2 py-1 font-mono text-xs text-fg"
                placeholder="planner"
                value={r.name}
                onChange={(e) => editName(i, e.target.value)}
              />
              <button
                type="button"
                aria-label={`Move role ${i + 1} up`}
                disabled={i === 0}
                onClick={() => move(i, 'up')}
                className="rounded-md border border-border bg-bg-elev-1 px-2 py-1 text-xs text-fg disabled:opacity-40"
              >
                ↑
              </button>
              <button
                type="button"
                aria-label={`Move role ${i + 1} down`}
                disabled={i === roles.length - 1}
                onClick={() => move(i, 'down')}
                className="rounded-md border border-border bg-bg-elev-1 px-2 py-1 text-xs text-fg disabled:opacity-40"
              >
                ↓
              </button>
              <button
                type="button"
                aria-label={`Remove role ${i + 1}`}
                disabled={roles.length === 1}
                onClick={() => remove(i)}
                className="rounded-md border border-border bg-bg-elev-1 px-2 py-1 text-xs text-fg disabled:opacity-40"
              >
                Remove
              </button>
            </div>
            <textarea
              aria-label={`Role ${i + 1} prompt`}
              className="min-h-32 w-full rounded-md border border-border bg-bg-elev-1 p-2 font-mono text-xs text-fg"
              placeholder="Prompt template — use {{TASK}} and {{PREV_OUTPUT}}"
              value={r.prompt}
              onChange={(e) => editPrompt(i, e.target.value)}
            />
          </div>
        ))}
      </div>
      <button
        type="button"
        onClick={add}
        disabled={roles.length >= MAX_ROLES}
        className="mt-3 rounded-md border border-border bg-bg-elev-2 px-3 py-1.5 text-xs text-fg transition hover:bg-bg-elev-1 disabled:opacity-40"
      >
        {roles.length >= MAX_ROLES ? `Max ${MAX_ROLES} roles` : '+ Add role'}
      </button>
    </div>
  );
}
```

- [ ] **Step 4: Re-run; tests must pass**

```bash
cd frontend && npx vitest run src/components/launcher/MultiAgentForm.test.tsx
```

Expected: 5/5 PASS.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/components/launcher/MultiAgentForm.tsx frontend/src/components/launcher/MultiAgentForm.test.tsx
git commit -m "Add MultiAgentForm component with seed roles, reorder, add/remove"
```

---

## Task 7 — Wire the form into `HarnessConfigPanel`

**Files:**
- Modify: `frontend/src/components/launcher/HarnessConfigPanel.tsx`

- [ ] **Step 1: Add the `multiagent` case to the switch**

In `frontend/src/components/launcher/HarnessConfigPanel.tsx`, add the import:

```tsx
import { MultiAgentForm } from './MultiAgentForm';
import type { MultiAgentConfig } from '../../lib/types';
```

Then extend the switch in the `HarnessConfigPanel` function. The existing structure is:

```tsx
switch (harnessId) {
  case 'agent_instructions':
    return <AgentInstructionsForm value={...} onChange={onChange} />;
  default:
    return null;
}
```

Add the new case before `default`:

```tsx
case 'multiagent':
  return (
    <MultiAgentForm
      value={value as MultiAgentConfig | undefined}
      onChange={(next) => onChange(next as unknown as Record<string, unknown>)}
    />
  );
```

- [ ] **Step 2: Run the full frontend test suite**

```bash
cd frontend && npm run lint && npm test -- --run 2>&1 | tail -8
```

Expected: typecheck clean; all tests PASS (including the new MultiAgentForm + validate tests).

- [ ] **Step 3: Commit**

```bash
git add frontend/src/components/launcher/HarnessConfigPanel.tsx
git commit -m "Register multiagent in HarnessConfigPanel switch"
```

---

## Task 8 — Launcher submit gate + button label

**Files:**
- Modify: `frontend/src/pages/diagnostic/launch.tsx`

- [ ] **Step 1: Add the multiagent gate parallel to the agent_instructions gate**

In `frontend/src/pages/diagnostic/launch.tsx`, find the existing `agentInstructionsReady` derivation. Right below it, add the multiagent equivalent:

```ts
import { validateMultiAgentConfig } from '../../components/launcher/multiagent-validate';
import type { MultiAgentConfig } from '../../lib/types';
```

(Add the imports at the top with the other component imports.)

In the body of `DiagnosticLaunchPage`, find the gate block (looks like the snippet below) and replace it:

```ts
const needsAgentInstructions = selectedHarnesses.includes('agent_instructions');
const agentInstructionsContent =
  (harnessConfigs.agent_instructions as { content?: string } | undefined)?.content?.trim() ?? '';
const agentInstructionsReady = !needsAgentInstructions || agentInstructionsContent.length > 0;

const needsMultiAgent = selectedHarnesses.includes('multiagent');
const multiagentConfig = harnessConfigs.multiagent as MultiAgentConfig | undefined;
const multiagentReady = !needsMultiAgent || validateMultiAgentConfig(multiagentConfig);

const canSubmit =
  taskIDs.length > 0
  && variants.length > 0
  && agentInstructionsReady
  && multiagentReady
  && !launch.isPending;
```

- [ ] **Step 2: Add the button-label branch**

Locate the Launch button's label expression (a ternary chain that picks among `'Launching…'`, `'Pick a task'`, `'Pick a variant'`, `'Type agent instructions'`, `'Launch · 1 task'`, `'Launch suite · N experiments'`). Add a `'Configure multiagent roles'` branch immediately after the agent_instructions branch:

```tsx
: !agentInstructionsReady
? 'Type agent instructions'
: !multiagentReady
? 'Configure multiagent roles'
: totalExperiments === 1
? 'Launch · 1 task'
: `Launch suite · ${totalExperiments} experiments`
```

- [ ] **Step 3: Update the display name override**

In the same file, locate `harnessDisplayName` (a small helper added in Project 1). Extend the overrides map to include the new harness id:

```ts
const overrides: Record<string, string> = {
  agent_instructions: 'Agent instructions',
  multiagent: 'Multi-agent',
};
```

- [ ] **Step 4: Typecheck + tests + build**

```bash
cd frontend && npm run lint && npm test -- --run 2>&1 | tail -8 && npm run build 2>&1 | tail -3
```

Expected: every check clean.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/pages/diagnostic/launch.tsx
git commit -m "Launcher gates multiagent submit + adds chip display name"
```

---

## Task 9 — Role accent helper + tests (TDD)

**Files:**
- Create: `frontend/src/components/run-inspector/role-accent.ts`
- Create: `frontend/src/components/run-inspector/role-accent.test.ts`

- [ ] **Step 1: Write the failing tests**

Create `frontend/src/components/run-inspector/role-accent.test.ts`:

```ts
import { describe, expect, it } from 'vitest';
import { roleAccent } from './role-accent';

describe('roleAccent', () => {
  it('returns a stable fallback for undefined / empty roles', () => {
    expect(roleAccent(undefined)).toBe('border-l-border');
    expect(roleAccent('')).toBe('border-l-border');
  });

  it('returns the same class for the same role name across calls', () => {
    expect(roleAccent('planner')).toBe(roleAccent('planner'));
    expect(roleAccent('coder')).toBe(roleAccent('coder'));
  });

  it('produces at least 3 distinct classes across 5 distinct names (palette coverage)', () => {
    const samples = ['planner', 'coder', 'reviewer', 'critic', 'tester'];
    const distinct = new Set(samples.map(roleAccent));
    expect(distinct.size).toBeGreaterThanOrEqual(3);
  });

  it('every returned class is one of the published palette entries', () => {
    const allowed = new Set([
      'border-l-border',
      'border-l-info-fg/50',
      'border-l-success-fg/50',
      'border-l-warning-fg/50',
      'border-l-accent-fg/50',
      'border-l-fg-muted/50',
    ]);
    for (const name of ['x', 'y', 'z', 'planner', 'coder', 'reviewer']) {
      expect(allowed.has(roleAccent(name))).toBe(true);
    }
  });
});
```

- [ ] **Step 2: Run; tests must fail**

```bash
cd frontend && npx vitest run src/components/run-inspector/role-accent.test.ts
```

Expected: "Cannot find module './role-accent'".

- [ ] **Step 3: Implement the helper**

Create `frontend/src/components/run-inspector/role-accent.ts`:

```ts
/**
 * roleAccent maps a role name to a stable Tailwind left-border class.
 * Same role → same color across all runs and pages. The palette reuses
 * existing semantic tokens with /50 alpha so it sits naturally next to
 * the rest of the design language; no new tokens are introduced.
 *
 * Empty / undefined → the neutral default border class (the absence of
 * a role should look like no accent at all).
 */

const PALETTE: readonly string[] = [
  'border-l-info-fg/50',
  'border-l-success-fg/50',
  'border-l-warning-fg/50',
  'border-l-accent-fg/50',
  'border-l-fg-muted/50',
];

export function roleAccent(role: string | undefined): string {
  if (!role) return 'border-l-border';
  let h = 0;
  for (let i = 0; i < role.length; i++) {
    h = (h * 31 + role.charCodeAt(i)) >>> 0;
  }
  return PALETTE[h % PALETTE.length];
}
```

- [ ] **Step 4: Re-run; tests must pass**

```bash
cd frontend && npx vitest run src/components/run-inspector/role-accent.test.ts
```

Expected: 4/4 PASS.

- [ ] **Step 5: Token check (verify palette entries are valid)**

```bash
cd frontend && npm run check:tokens
```

Expected: clean. If the token checker flags any of the palette classes, replace with an equivalent existing token (read `frontend/tailwind.config.ts` for the registered semantic colors).

- [ ] **Step 6: Commit**

```bash
git add frontend/src/components/run-inspector/role-accent.ts frontend/src/components/run-inspector/role-accent.test.ts
git commit -m "Add roleAccent pure helper + Vitest cases for stable per-role colors"
```

---

## Task 10 — `TurnGroup.role` + `TurnGroupCard` badge + accent

**Files:**
- Modify: `frontend/src/components/run-inspector/group-turns.ts`
- Modify: `frontend/src/components/run-inspector/TurnGroupCard.tsx`

- [ ] **Step 1: Expose `role` on `TurnGroup`**

Read `frontend/src/components/run-inspector/group-turns.ts` first to confirm the current shape. Find the `TurnGroup` interface (around line 10) and add a new field:

```ts
export interface TurnGroup {
  // …existing fields…
  role: string;
}
```

Find the `groupTurns` function (around line 60). At the spot where it constructs each group (the `.map(([parent, blocks]) => { ... })` block), compute the role from the first block whose role is non-empty:

```ts
const role = blocks.find((b) => (b.role ?? '').length > 0)?.role ?? '';
```

Add `role` to the returned group object.

- [ ] **Step 2: Render the badge + accent in `TurnGroupCard`**

Read `frontend/src/components/run-inspector/TurnGroupCard.tsx`. At the top of the file with the other imports, add:

```ts
import { roleAccent } from './role-accent';
```

Find the outer `<div role="button" ...>` (around line 113). Append `roleAccent(group.role)` to the existing className. Wrap that in a Tailwind border-left to make the accent visible — change the className to:

```tsx
className={cn(
  'group/turn relative grid w-full cursor-pointer grid-cols-[28px_1fr] gap-3 border-l-4 px-2 py-2 text-left transition focus:outline-none hover:bg-bg-elev-1/40 focus-visible:bg-bg-elev-1/60',
  roleAccent(group.role),
)}
```

(Adds `border-l-4` and the dynamic class. The fallback `border-l-border` from `roleAccent('')` keeps the look unchanged for runs without role tagging.)

Then, inside the body column (the `<div className="min-w-0 space-y-1.5">` block), at the top — right above the optional symptom glyphs line — add a small role badge that only renders when `group.role` is non-empty:

```tsx
{group.role && (
  <div className="text-xs uppercase tracking-wider text-fg-muted">
    Role: <span className="font-mono text-fg">{group.role}</span>
  </div>
)}
```

(Keeps the style consistent with the rest of the Inspector header labels.)

- [ ] **Step 3: Typecheck + run all tests (no regression)**

```bash
cd frontend && npm run lint && npm test -- --run 2>&1 | tail -10
```

Expected: lint clean, all tests pass. If any existing `group-turns.test.ts` test now fails because it asserts the exact `TurnGroup` shape (missing `role`), update its fixture rows to include `role: ''` so the shape matches; do NOT change the assertions themselves.

- [ ] **Step 4: Commit**

```bash
git add frontend/src/components/run-inspector/group-turns.ts frontend/src/components/run-inspector/TurnGroupCard.tsx
git commit -m "Inspector: expose role on TurnGroup, render badge + role accent on TurnGroupCard"
```

---

## Task 11 — Full sanity pass

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

## Task 12 — Manual verification

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

- [ ] **Step 2: Browser walkthrough**

Visit `http://localhost:5173/diagnostic/launch`. Confirm:

1. The harness chip list shows `Multi-agent` (NOT `planner_coder`).
2. Selecting `Multi-agent` reveals an inline panel with two pre-seeded role rows (`planner`, `coder`).
3. Without editing anything else, the Launch button reads `Configure multiagent roles` and is disabled if Multi-agent is the only selected harness AND a model isn't picked, otherwise it reads `Launch · 1 task` once a model is also selected. (Try both.)
4. Add a third role (`+ Add role`) → it appears as `role3` with an empty prompt. Launch button now reads `Configure multiagent roles` because the new prompt is empty. Type any content into the new prompt → button enables.
5. Hit the 5-role cap → Add button disables with `Max 5 roles`.
6. Click `Remove` on a role → the row vanishes; Remove disables once only one role remains.
7. Use `↑` / `↓` arrows → roles reorder.
8. Launch a run with a 1-task × 1-harness × 1-exec × 1-model selection where Multi-agent is the harness. After the run completes (or while in progress), open the Inspector for that run.
9. In Inspector, every turn-group card has a small `Role: <name>` line at the top of its body, and a colored left border. Two groups from the same role share the same color.

- [ ] **Step 3: API smoke (direct curl, no UI)**

```bash
curl -s -X POST http://localhost:8080/api/diagnostic/launch \
  -H 'Content-Type: application/json' \
  -d '{
    "task_id":"brownfield-fix-async-race",
    "executor_id":"opencode",
    "harness_ids":["multiagent"],
    "model":"opencode/deepseek-v4-flash-free",
    "runs_per_variant":1,
    "harness_configs": {
      "multiagent": {
        "roles": [
          {"name":"planner", "prompt":"plan: {{TASK}}"},
          {"name":"reviewer", "prompt":"review the plan: {{PREV_OUTPUT}}"}
        ]
      }
    }
  }'
```

Expected: `{"experiment_id":"..."}`. The variant persists with the role list; the run executes both roles sequentially.

- [ ] **Step 4: Empty-config rejection smoke**

```bash
curl -s -X POST http://localhost:8080/api/diagnostic/launch \
  -H 'Content-Type: application/json' \
  -d '{
    "task_id":"brownfield-fix-async-race",
    "executor_id":"opencode",
    "harness_ids":["multiagent"],
    "model":"opencode/deepseek-v4-flash-free",
    "runs_per_variant":1
  }'
```

Expected: 202 returned, but the run transitions to `failed` within ~5s with an error message mentioning `ErrMultiAgentRolesMissing` or "cfg.multiagent.roles is empty".

- [ ] **Step 5: Stop servers**

```bash
lsof -ti tcp:8080 | xargs -r kill 2>/dev/null
lsof -ti tcp:5173 | xargs -r kill 2>/dev/null
```

---

## Task 13 — Push, open PR, request review, watch CI, merge

**Files:** none

- [ ] **Step 1: Push**

```bash
git push -u origin feature/multiagent-harness
```

- [ ] **Step 2: Open PR**

```bash
gh pr create --title "Multi-agent harness: rename planner_coder, user-config N roles, Inspector role accent" --body "$(cat <<'EOF'
## Summary

Project 2 of the harness refresh. Replaces the hardcoded two-role \`planner_coder\` harness with a user-configurable \`multiagent\` harness (1–5 sequential roles), and surfaces per-role visual demarcation in Run Inspect.

- **Harness:** rename \`planner_coder\` → \`multiagent\` (wire id + Go type + tests + registry). Old id is dropped (clean break, mirrors Project 1's claudemd → agent_instructions pattern).
- **Config schema:** role list lives in \`variants.harness_config_json\` under the \`multiagent\` key (Project 1's plumbing — no schema change). Each role has a snake_case name and a prompt template with \`{{TASK}}\` and \`{{PREV_OUTPUT}}\` substitutions. Setup validates count (1–5), name regex, prompt non-empty, uniqueness. Failures surface as \`ErrMultiAgentRolesMissing\` / \`ErrMultiAgentInvalidRole\`.
- **Invoke loop:** walks roles in array order, substitutes the previous role's last-assistant output into \`{{PREV_OUTPUT}}\`, tags every emitted ParsedTurn with the role name (both \`Role\` and \`Stage\` fields), continues past single-role failures, bails immediately on context cancel.
- **Launcher:** new \`<MultiAgentForm>\` panel registered in \`HarnessConfigPanel\`. Seeded with planner+coder rows on first render. Add/Remove/↑/↓ controls; 5-role cap enforced. Submit gate refuses launch until \`validateMultiAgentConfig\` returns true; button label reads \"Configure multiagent roles\".
- **Inspector:** pure \`roleAccent\` helper hashes the role name to one of 5 palette entries; \`TurnGroupCard\` picks up the accent as a 4-px left border AND shows a small \"Role: <name>\" header above the group body. Two groups from the same role share the same color.

Spec: [\`docs/superpowers/specs/2026-05-31-multiagent-harness-design.md\`](docs/superpowers/specs/2026-05-31-multiagent-harness-design.md)
Plan: [\`docs/superpowers/plans/2026-05-31-multiagent-harness.md\`](docs/superpowers/plans/2026-05-31-multiagent-harness.md)

## Test plan

- [x] \`go test ./...\` clean — new \`expandPrompt\`, validation, sequential-invoke, role-tagging, partial-failure, ctx-cancel tests
- [x] \`npm run lint\` / \`build\` / \`test\` / \`check:tokens\` clean — new \`MultiAgentForm\`, \`validateMultiAgentConfig\`, \`roleAccent\` Vitest cases
- [x] Manual: launcher seeds planner+coder, add/remove/reorder work, Launch gate fires correctly; running a multiagent experiment surfaces per-role badges + colors in Inspector; CLI \`curl\` with an empty config fails the run cleanly with the sentinel message
EOF
)"
```

- [ ] **Step 3: Dispatch the code reviewer**

Per memory `feedback_github_workflow`, every non-trivial PR goes through `feature-dev:code-reviewer`. Dispatch the agent on the branch HEAD and address findings.

- [ ] **Step 4: Watch CI**

Monitor checks until all pass. Push fixes if needed.

- [ ] **Step 5: Squash merge once green**

```bash
gh pr merge <PR#> --squash --delete-branch
git fetch origin && git checkout main && git reset --hard origin/main
```

---

## Self-review

**Spec coverage:**
- Harness rename + clean break → Task 1
- Config schema + validation + `expandPrompt` substitution + sentinels → Task 2
- Sequential Invoke loop with role-tagged transcript + partial failure + ctx cancel → Task 3
- Frontend types → Task 4
- `validateMultiAgentConfig` pure helper → Task 5
- `<MultiAgentForm>` component (seed, add/remove, reorder, max cap) → Task 6
- Wire form into `HarnessConfigPanel` → Task 7
- Launcher submit gate + button label + chip display name → Task 8
- `roleAccent` helper → Task 9
- `TurnGroup.role` + `TurnGroupCard` accent + badge → Task 10
- Sanity → Task 11
- Manual + API smoke → Task 12
- PR + review + CI + merge → Task 13
- Out-of-scope items (per-role model, Compare side-by-side, turn filtering) — explicitly excluded by spec; no tasks needed.

**No placeholders** — every step shows complete code, exact commands, exact expected output. Manual steps cite concrete URLs and curl payloads.

**Type consistency:**
- Go: `MultiAgent` type, `NewMultiAgent` constructor, `MultiAgentHarnessID = "multiagent"` constant, internal `role` struct (Name, Prompt), `extractRoles` / `expandPrompt` / `extractLastAssistantOutput` helpers, `ErrMultiAgentRolesMissing` / `ErrMultiAgentInvalidRole` sentinels — used consistently across Tasks 1–3.
- TypeScript: `MultiAgentRole` / `MultiAgentConfig` types, `validateMultiAgentConfig` helper, `MultiAgentForm` component, `roleAccent(role)` helper — consistent across Tasks 4–10.
- The `multiagentConfigKey = "multiagent"` Go constant matches the TS panel-switch case `'multiagent'` matches the backend wire id `MultiAgentHarnessID` matches the harness registry id.

**Ordering** — Rename → schema/validation → loop → frontend types → validator → component → panel switch → launcher gate → role accent → inspector card → sanity → manual → PR. Each task is testable on its own.
