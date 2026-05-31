# Spec-kit catalog Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the hardcoded canonical 4-stage `speckit` harness with a 6-entry catalog of curated spec-kit extensions (canonical, lite, tdd-first, research-first, rigorous, dual-role) selectable from the launcher as a multi-select. Each selected extension becomes its own variant via a new 4th axis on the matrix expansion. Run Inspect renders the chosen extension.

**Architecture:** A new `engine/internal/builtin/speckit` package holds the catalog as Go data. The existing speckit harness reads `cfg["speckit"]["extension_id"]` from Project 1's per-variant config plumbing, looks up the catalog, and walks the entry's `Stages` in order — passing each stage's `Role` through `RunConfig.Role` so Project 2's Inspector role accent fires automatically for the dual-role extension. The launcher gains a `<SpecKitForm>` panel that calls a new `/api/harnesses/speckit/catalog` endpoint and converts the user's multi-id selection into a 4th matrix axis right before posting.

**Tech Stack:** Go 1.22 (stdlib `testing`, no new deps), React 18 + TypeScript + TanStack Query + Tailwind (all existing).

**Spec:** [`docs/superpowers/specs/2026-05-31-speckit-catalog-design.md`](../specs/2026-05-31-speckit-catalog-design.md)

**Branch:** `feature/speckit-catalog` (created; spec committed at `dfb13b2`).

---

## File map

| Layer | File | Action |
|---|---|---|
| Catalog package | `engine/internal/builtin/speckit/catalog.go` | CREATE |
| Catalog tests | `engine/internal/builtin/speckit/catalog_test.go` | CREATE |
| Harness rewrite | `engine/internal/builtin/harness/speckit.go` | MODIFY (drop hardcoded stages, consume catalog) |
| Harness tests | `engine/internal/builtin/harness/speckit_test.go` | MODIFY (extension-aware cases) |
| API handler | `engine/internal/api/speckit_catalog_handler.go` | CREATE |
| API handler test | `engine/internal/api/speckit_catalog_handler_test.go` | CREATE |
| Router | `engine/internal/api/router.go` | MODIFY (register new route) |
| Frontend types | `frontend/src/lib/types.ts` | MODIFY (add `SpecKitExtensionPublic`, `SpecKitConfig`) |
| Frontend hook | `frontend/src/lib/hooks.ts` | MODIFY (add `useSpecKitCatalog`) |
| SpecKitForm component | `frontend/src/components/launcher/SpecKitForm.tsx` | CREATE |
| SpecKitForm test | `frontend/src/components/launcher/SpecKitForm.test.tsx` | CREATE |
| HarnessConfigPanel switch | `frontend/src/components/launcher/HarnessConfigPanel.tsx` | MODIFY |
| Matrix helper | `frontend/src/pages/diagnostic/launch-matrix.ts` | MODIFY (4th axis) |
| Matrix tests | `frontend/src/pages/diagnostic/launch-matrix.test.ts` | MODIFY (new axis cases) |
| Launcher page | `frontend/src/pages/diagnostic/launch.tsx` | MODIFY (selectedExtensions derive, per-cell payload narrow, submit gate, button label) |
| Inspector card | `frontend/src/pages/runs/inspect.tsx` | MODIFY (render `<SpecKitExtensionCard>`) |

No schema migration. No new dependencies.

---

## Task 1 — Catalog package (TDD)

**Files:**
- Create: `engine/internal/builtin/speckit/catalog.go`
- Create: `engine/internal/builtin/speckit/catalog_test.go`

- [ ] **Step 1: Write the failing tests**

Create `engine/internal/builtin/speckit/catalog_test.go`:

```go
package speckit

import (
	"strings"
	"testing"
)

func TestListReturnsAllEntries(t *testing.T) {
	got := List()
	if len(got) != 6 {
		t.Fatalf("entry count: got %d want 6", len(got))
	}
	// Canonical must come first; rest alphabetical by ID.
	if got[0].ID != "canonical" {
		t.Errorf("first entry: got %q want %q", got[0].ID, "canonical")
	}
	wantIDs := []string{"canonical", "dual-role", "lite", "research-first", "rigorous", "tdd-first"}
	for i, w := range wantIDs {
		if got[i].ID != w {
			t.Errorf("entry %d: got %q want %q", i, got[i].ID, w)
		}
	}
}

func TestLookupKnownAndUnknown(t *testing.T) {
	ext, ok := Lookup("canonical")
	if !ok || ext.ID != "canonical" {
		t.Errorf("known: ok=%v id=%q", ok, ext.ID)
	}
	if _, ok := Lookup("nope"); ok {
		t.Error("unknown should return ok=false")
	}
	if _, ok := Lookup(""); ok {
		t.Error("empty should return ok=false")
	}
}

func TestCanonicalEntryPreservesOldStagePrompts(t *testing.T) {
	ext, ok := Lookup("canonical")
	if !ok {
		t.Fatal("canonical missing")
	}
	if len(ext.Stages) != 4 {
		t.Fatalf("stage count: got %d want 4", len(ext.Stages))
	}
	expect := []struct {
		name         string
		slashCommand string
		promptStart  string
	}{
		{"specify", "/speckit.specify", "/speckit.specify\n\n{{TASK}}"},
		{"plan", "/speckit.plan", "/speckit.plan\n\n{{TECHNICAL_DETAILS}}"},
		{"tasks", "/speckit.tasks", "/speckit.tasks"},
		{"implement", "/speckit.implement", "/speckit.implement"},
	}
	for i, want := range expect {
		st := ext.Stages[i]
		if st.Name != want.name {
			t.Errorf("stage %d name: got %q want %q", i, st.Name, want.name)
		}
		if st.SlashCommand != want.slashCommand {
			t.Errorf("stage %d slash: got %q want %q", i, st.SlashCommand, want.slashCommand)
		}
		if !strings.Contains(st.PromptTemplate, want.promptStart) {
			t.Errorf("stage %d prompt should contain %q; got %q", i, want.promptStart, st.PromptTemplate)
		}
		if st.Role != "" {
			t.Errorf("canonical stage %d should have empty role; got %q", i, st.Role)
		}
	}
}

func TestDualRoleEntryHasRoleTags(t *testing.T) {
	ext, ok := Lookup("dual-role")
	if !ok {
		t.Fatal("dual-role missing")
	}
	if !ext.MultiAgent {
		t.Error("dual-role should set MultiAgent=true")
	}
	wantRoles := []string{"architect", "architect", "coder", "coder"}
	if len(ext.Stages) != len(wantRoles) {
		t.Fatalf("stage count: got %d want %d", len(ext.Stages), len(wantRoles))
	}
	for i, want := range wantRoles {
		if ext.Stages[i].Role != want {
			t.Errorf("stage %d role: got %q want %q", i, ext.Stages[i].Role, want)
		}
	}
}
```

- [ ] **Step 2: Run; tests must fail (package missing)**

```bash
cd engine && go test ./internal/builtin/speckit/...
```

Expected: build error — package doesn't exist yet.

- [ ] **Step 3: Implement the catalog**

Create `engine/internal/builtin/speckit/catalog.go`:

```go
// Package speckit holds the curated catalog of spec-kit extensions the
// launcher exposes to users. Each extension is a small ordered list of
// stages with prompt templates; the harness walks them in sequence at
// invocation time. The dual-role entry tags its stages with role names
// so Project 2's Inspector role accent fires for those runs.
package speckit

import "sort"

type Stage struct {
	Name           string // stable id used in transcripts ("specify", "plan", ...)
	SlashCommand   string // "/speckit.specify"
	PromptTemplate string // text with {{TASK}} / {{TECHNICAL_DETAILS}} substitutions
	Role           string // optional; non-empty only for dual-role
}

type SpecKitExtension struct {
	ID          string
	Name        string
	Description string
	Stages      []Stage
	MultiAgent  bool
	SourceURL   string
}

var entries = []SpecKitExtension{
	{
		ID:          "canonical",
		Name:        "Canonical (4-stage)",
		Description: "specify → plan → tasks → implement; the upstream spec-kit baseline.",
		Stages: []Stage{
			{Name: "specify", SlashCommand: "/speckit.specify", PromptTemplate: "/speckit.specify\n\n{{TASK}}"},
			{Name: "plan", SlashCommand: "/speckit.plan", PromptTemplate: "/speckit.plan\n\n{{TECHNICAL_DETAILS}}"},
			{Name: "tasks", SlashCommand: "/speckit.tasks", PromptTemplate: "/speckit.tasks"},
			{Name: "implement", SlashCommand: "/speckit.implement", PromptTemplate: "/speckit.implement"},
		},
		SourceURL: "https://github.github.io/spec-kit/",
	},
	{
		ID:          "lite",
		Name:        "Lite (2-stage)",
		Description: "specify → implement; the minimal-ceremony baseline.",
		Stages: []Stage{
			{Name: "specify", SlashCommand: "/speckit.specify", PromptTemplate: "/speckit.specify\n\n{{TASK}}"},
			{Name: "implement", SlashCommand: "/speckit.implement", PromptTemplate: "/speckit.implement"},
		},
	},
	{
		ID:          "tdd-first",
		Name:        "TDD-first",
		Description: "specify → tests → plan → implement → verify; write tests before the plan.",
		Stages: []Stage{
			{Name: "specify", SlashCommand: "/speckit.specify", PromptTemplate: "/speckit.specify\n\n{{TASK}}"},
			{Name: "tests", SlashCommand: "/speckit.tests", PromptTemplate: "/speckit.tests\n\nWrite failing tests that pin every requirement from the specify stage."},
			{Name: "plan", SlashCommand: "/speckit.plan", PromptTemplate: "/speckit.plan\n\n{{TECHNICAL_DETAILS}}"},
			{Name: "implement", SlashCommand: "/speckit.implement", PromptTemplate: "/speckit.implement"},
			{Name: "verify", SlashCommand: "/speckit.verify", PromptTemplate: "/speckit.verify\n\nRun the test suite and report any failures."},
		},
	},
	{
		ID:          "research-first",
		Name:        "Research-first",
		Description: "research → specify → plan → tasks → implement; gather context before specifying.",
		Stages: []Stage{
			{Name: "research", SlashCommand: "/speckit.research", PromptTemplate: "/speckit.research\n\nSurvey the codebase and external context relevant to:\n{{TASK}}"},
			{Name: "specify", SlashCommand: "/speckit.specify", PromptTemplate: "/speckit.specify\n\n{{TASK}}"},
			{Name: "plan", SlashCommand: "/speckit.plan", PromptTemplate: "/speckit.plan\n\n{{TECHNICAL_DETAILS}}"},
			{Name: "tasks", SlashCommand: "/speckit.tasks", PromptTemplate: "/speckit.tasks"},
			{Name: "implement", SlashCommand: "/speckit.implement", PromptTemplate: "/speckit.implement"},
		},
	},
	{
		ID:          "rigorous",
		Name:        "Rigorous review",
		Description: "specify → plan → tasks → implement → review; post-implement review pass.",
		Stages: []Stage{
			{Name: "specify", SlashCommand: "/speckit.specify", PromptTemplate: "/speckit.specify\n\n{{TASK}}"},
			{Name: "plan", SlashCommand: "/speckit.plan", PromptTemplate: "/speckit.plan\n\n{{TECHNICAL_DETAILS}}"},
			{Name: "tasks", SlashCommand: "/speckit.tasks", PromptTemplate: "/speckit.tasks"},
			{Name: "implement", SlashCommand: "/speckit.implement", PromptTemplate: "/speckit.implement"},
			{Name: "review", SlashCommand: "/speckit.review", PromptTemplate: "/speckit.review\n\nReview the implementation against the spec; flag deviations and unhandled cases."},
		},
	},
	{
		ID:          "dual-role",
		Name:        "Dual-role (multi-agent)",
		Description: "Architect (specify, plan) hands off to coder (tasks, implement); role-tagged transcript.",
		MultiAgent:  true,
		Stages: []Stage{
			{Name: "specify", SlashCommand: "/speckit.specify", PromptTemplate: "/speckit.specify\n\n{{TASK}}", Role: "architect"},
			{Name: "plan", SlashCommand: "/speckit.plan", PromptTemplate: "/speckit.plan\n\n{{TECHNICAL_DETAILS}}", Role: "architect"},
			{Name: "tasks", SlashCommand: "/speckit.tasks", PromptTemplate: "/speckit.tasks", Role: "coder"},
			{Name: "implement", SlashCommand: "/speckit.implement", PromptTemplate: "/speckit.implement", Role: "coder"},
		},
	},
}

// List returns every catalog entry. Canonical is always first; remaining
// entries follow alphabetical id order so the picker UI is deterministic.
func List() []SpecKitExtension {
	out := make([]SpecKitExtension, len(entries))
	copy(out, entries)
	sort.SliceStable(out[1:], func(i, j int) bool {
		return out[1+i].ID < out[1+j].ID
	})
	return out
}

// Lookup returns the entry matching id, or (zero, false) if none.
// Empty id is treated as unknown.
func Lookup(id string) (SpecKitExtension, bool) {
	if id == "" {
		return SpecKitExtension{}, false
	}
	for _, e := range entries {
		if e.ID == id {
			return e, true
		}
	}
	return SpecKitExtension{}, false
}
```

- [ ] **Step 4: Re-run; all tests must pass**

```bash
cd engine && go test ./internal/builtin/speckit/... -v
```

Expected: 4/4 PASS.

- [ ] **Step 5: Commit**

```bash
git add engine/internal/builtin/speckit/
git commit -m "Add curated 6-entry spec-kit catalog (canonical, lite, tdd-first, research-first, rigorous, dual-role)"
```

---

## Task 2 — Spec-kit harness extension-aware Setup (TDD)

This task adds Setup validation and pulls the catalog entry out of the cfg, stashing it on the HarnessRun. The Invoke rewrite is Task 3 so the cycle stays bite-sized.

**Files:**
- Modify: `engine/internal/builtin/harness/speckit.go`
- Modify: `engine/internal/builtin/harness/speckit_test.go`

- [ ] **Step 1: Read the existing test file to see its package-level fixtures**

Skim `engine/internal/builtin/harness/speckit_test.go` to learn what test executor / helpers it already exposes — you'll re-use the same pattern. If it uses a `recordingExecutor` (PR #146 introduced one for multiagent), check whether speckit's tests have their own variant. Don't introduce a duplicate.

- [ ] **Step 2: Write the failing Setup tests**

Append to `engine/internal/builtin/harness/speckit_test.go`:

```go
func TestSpecKitSetupRejectsMissingExtensionID(t *testing.T) {
	h := NewSpecKit()
	ws := pkgharness.Workspace{Path: t.TempDir()}
	cases := []map[string]any{
		nil,
		{},
		{"speckit": map[string]any{}},
		{"speckit": map[string]any{"extension_id": ""}},
	}
	for i, cfg := range cases {
		if _, err := h.Setup(context.Background(), ws, task.Task{}, pkgharness.Budget{}, cfg); !errors.Is(err, ErrSpecKitExtensionMissing) {
			t.Errorf("case %d: got %v want ErrSpecKitExtensionMissing", i, err)
		}
	}
}

func TestSpecKitSetupRejectsUnknownExtensionID(t *testing.T) {
	h := NewSpecKit()
	ws := pkgharness.Workspace{Path: t.TempDir()}
	cfg := map[string]any{"speckit": map[string]any{"extension_id": "does-not-exist"}}
	if _, err := h.Setup(context.Background(), ws, task.Task{}, pkgharness.Budget{}, cfg); !errors.Is(err, ErrSpecKitExtensionNotFound) {
		t.Errorf("got %v want ErrSpecKitExtensionNotFound", err)
	}
}

func TestSpecKitSetupAcceptsKnownExtensionID(t *testing.T) {
	h := NewSpecKit()
	ws := pkgharness.Workspace{Path: t.TempDir()}
	cfg := map[string]any{"speckit": map[string]any{"extension_id": "canonical"}}
	run, err := h.Setup(context.Background(), ws, task.Task{}, pkgharness.Budget{}, cfg)
	if err != nil {
		t.Fatalf("Setup: %v", err)
	}
	ext, ok := run.Metadata["speckit.extension"].(speckit.SpecKitExtension)
	if !ok || ext.ID != "canonical" {
		t.Errorf("stashed extension: got %+v ok=%v", run.Metadata["speckit.extension"], ok)
	}
}
```

(Add the import `"github.com/mustafaselman/frameval/engine/internal/builtin/speckit"` at the top of the test file. If it's not there yet, add it now.)

- [ ] **Step 3: Run; tests must fail (sentinels + extraction don't exist yet)**

```bash
cd engine && go test ./internal/builtin/harness/ -run 'TestSpecKitSetup' -v
```

Expected: compile errors about `ErrSpecKitExtensionMissing`, `ErrSpecKitExtensionNotFound`.

- [ ] **Step 4: Add the sentinels + extension extraction to Setup**

In `engine/internal/builtin/harness/speckit.go`, at the top of the file with the other consts/vars, add:

```go
import (
	// …existing imports…
	"github.com/mustafaselman/frameval/engine/internal/builtin/speckit"
)

// metadataKeySpecKitExtension stashes the resolved catalog entry on the
// HarnessRun so Invoke can walk its stages without re-parsing cfg.
const metadataKeySpecKitExtension = "speckit.extension"

// ErrSpecKitExtensionMissing surfaces when cfg has no extension_id.
// The launcher's submit gate prevents this in normal flow; the sentinel
// catches direct API consumers.
var ErrSpecKitExtensionMissing = errors.New("speckit harness: cfg.speckit.extension_id is empty")

// ErrSpecKitExtensionNotFound surfaces when extension_id doesn't match
// any catalog entry. Usually a stale id from a deleted entry.
var ErrSpecKitExtensionNotFound = errors.New("speckit harness: extension_id does not match any catalog entry")
```

Now modify `Setup` to extract + validate the extension. Replace the existing Setup body's start (before the constitution-handling block) with:

```go
func (h *SpecKit) Setup(_ context.Context, ws harness.Workspace, t task.Task, b harness.Budget, cfg map[string]any) (harness.HarnessRun, error) {
	ext, err := extractSpecKitExtension(cfg)
	if err != nil {
		return harness.HarnessRun{}, err
	}
	memoryDir := filepath.Join(ws.Path, speckitMemoryDir)
	// …existing .specify scaffold + constitution handling stays exactly as it is…
```

At the end of Setup, where the existing return constructs the HarnessRun, stash both the existing `owned` flag AND the resolved extension:

```go
	return harness.HarnessRun{
		HarnessName: h.Name(),
		Task:        t,
		Workspace:   ws,
		Budget:      b,
		Metadata: map[string]any{
			metadataKeyOwnsSpecify:        owned,
			metadataKeySpecKitExtension:   ext,
		},
	}, nil
```

Add the helper at the bottom of the file:

```go
func extractSpecKitExtension(cfg map[string]any) (speckit.SpecKitExtension, error) {
	if cfg == nil {
		return speckit.SpecKitExtension{}, ErrSpecKitExtensionMissing
	}
	sub, ok := cfg["speckit"].(map[string]any)
	if !ok {
		return speckit.SpecKitExtension{}, ErrSpecKitExtensionMissing
	}
	id, _ := sub["extension_id"].(string)
	if id == "" {
		return speckit.SpecKitExtension{}, ErrSpecKitExtensionMissing
	}
	ext, ok := speckit.Lookup(id)
	if !ok {
		return speckit.SpecKitExtension{}, fmt.Errorf("%w: %q", ErrSpecKitExtensionNotFound, id)
	}
	return ext, nil
}
```

- [ ] **Step 5: Re-run Setup tests; must pass**

```bash
cd engine && go test ./internal/builtin/harness/ -run 'TestSpecKitSetup' -v
```

Expected: 3/3 PASS.

- [ ] **Step 6: Confirm the pre-existing speckit Invoke tests still pass (Invoke unchanged in this task)**

```bash
cd engine && go test ./internal/builtin/harness/ -v
```

Expected: every test in the package green. (The old TestSpecKitInvoke* tests still use the hardcoded stages — they'll keep passing because Invoke hasn't been rewritten yet.)

- [ ] **Step 7: Commit**

```bash
git add engine/internal/builtin/harness/speckit.go engine/internal/builtin/harness/speckit_test.go
git commit -m "speckit harness: extract extension from cfg in Setup (canonical pass-through, no Invoke change)"
```

---

## Task 3 — Spec-kit harness extension-aware Invoke (TDD)

**Files:**
- Modify: `engine/internal/builtin/harness/speckit.go`
- Modify: `engine/internal/builtin/harness/speckit_test.go`

- [ ] **Step 1: Delete the old Invoke tests that hardcode the 4-stage flow**

Read the test file. Identify any test that asserts the canonical 4-stage flow (likely `TestSpecKitInvokeWalksFourStagesInOrder` or similar). Delete those tests — Task 3 replaces them with extension-aware versions.

Also delete any helper like a fixed `speckitStages` reference in the test file if it exists.

- [ ] **Step 2: Write the new Invoke tests**

Append to `engine/internal/builtin/harness/speckit_test.go`:

```go
// speckitRecordingExec captures every RunConfig the harness emits so a
// test can assert per-stage prompts, stage names, and role threading.
type speckitRecordingExec struct {
	calls []executor.RunConfig
}

func (e *speckitRecordingExec) Name() string                                          { return "speckit-record" }
func (e *speckitRecordingExec) SupportedModes() []executor.ExecutionMode              { return []executor.ExecutionMode{executor.ExecutionModeCLI} }
func (e *speckitRecordingExec) ParseTranscript(raw []byte) ([]executor.ParsedTurn, error) { return nil, nil }
func (e *speckitRecordingExec) Execute(_ context.Context, cfg executor.RunConfig) (*executor.RunResult, error) {
	e.calls = append(e.calls, cfg)
	return &executor.RunResult{RawOutput: cfg.Stage + "-done\n"}, nil
}

func TestSpecKitInvokeWalksExtensionStages(t *testing.T) {
	h := NewSpecKit()
	ws := pkgharness.Workspace{Path: t.TempDir()}
	exec := &speckitRecordingExec{}
	cfg := map[string]any{"speckit": map[string]any{"extension_id": "lite"}}
	run, err := h.Setup(context.Background(), ws, task.Task{TaskPrompt: "scaffold"}, pkgharness.Budget{}, cfg)
	if err != nil {
		t.Fatalf("Setup: %v", err)
	}
	if _, err := h.Invoke(context.Background(), run, exec); err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	if len(exec.calls) != 2 {
		t.Fatalf("call count: got %d want 2", len(exec.calls))
	}
	if exec.calls[0].Stage != "specify" || exec.calls[1].Stage != "implement" {
		t.Errorf("stage order: got %q,%q want specify,implement", exec.calls[0].Stage, exec.calls[1].Stage)
	}
	if !strings.Contains(exec.calls[0].Prompt, "scaffold") {
		t.Errorf("specify prompt should contain task content; got %q", exec.calls[0].Prompt)
	}
}

func TestSpecKitDualRoleSetsRoleOnRunConfig(t *testing.T) {
	h := NewSpecKit()
	ws := pkgharness.Workspace{Path: t.TempDir()}
	exec := &speckitRecordingExec{}
	cfg := map[string]any{"speckit": map[string]any{"extension_id": "dual-role"}}
	run, _ := h.Setup(context.Background(), ws, task.Task{TaskPrompt: "x"}, pkgharness.Budget{}, cfg)
	if _, err := h.Invoke(context.Background(), run, exec); err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	if len(exec.calls) != 4 {
		t.Fatalf("call count: got %d want 4", len(exec.calls))
	}
	wantRoles := []string{"architect", "architect", "coder", "coder"}
	for i, want := range wantRoles {
		if exec.calls[i].Role != want {
			t.Errorf("call %d role: got %q want %q", i, exec.calls[i].Role, want)
		}
	}
}

func TestExpandSpecKitPrompt(t *testing.T) {
	cases := []struct {
		name     string
		template string
		vars     map[string]string
		want     string
	}{
		{"replaces TASK", "do {{TASK}}", map[string]string{"TASK": "x"}, "do x"},
		{"replaces TECHNICAL_DETAILS", "{{TECHNICAL_DETAILS}}!", map[string]string{"TECHNICAL_DETAILS": "y"}, "y!"},
		{"both", "{{TASK}} - {{TECHNICAL_DETAILS}}", map[string]string{"TASK": "a", "TECHNICAL_DETAILS": "b"}, "a - b"},
		{"empty values leave blank", "{{TASK}}", map[string]string{"TASK": ""}, ""},
		{"unknown token preserved", "{{TASK}} {{OTHER}}", map[string]string{"TASK": "a"}, "a {{OTHER}}"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := expandSpecKitPrompt(tc.template, tc.vars); got != tc.want {
				t.Errorf("got %q want %q", got, tc.want)
			}
		})
	}
}
```

- [ ] **Step 3: Run; new Invoke tests must fail (helper + new Invoke don't exist yet)**

```bash
cd engine && go test ./internal/builtin/harness/ -run 'TestSpecKitInvoke|TestSpecKitDualRole|TestExpandSpecKitPrompt' -v
```

Expected: compile error (`expandSpecKitPrompt`) and/or assertion failures (old Invoke hardcoded canonical stages).

- [ ] **Step 4: Rewrite Invoke + add expandSpecKitPrompt helper**

In `engine/internal/builtin/harness/speckit.go`:

Delete the existing `speckitStages` slice, `stagePrompt` function, and the existing `Invoke` body. Replace `Invoke` with:

```go
func (h *SpecKit) Invoke(ctx context.Context, run harness.HarnessRun, exec executor.AgentExecutor) (*executor.RunResult, error) {
	ext, ok := run.Metadata[metadataKeySpecKitExtension].(speckit.SpecKitExtension)
	if !ok {
		return nil, ErrSpecKitExtensionMissing
	}
	stageNames := make([]string, 0, len(ext.Stages))
	stageResults := make([]*executor.RunResult, 0, len(ext.Stages))
	var stageErr error
stages:
	for _, stage := range ext.Stages {
		select {
		case <-ctx.Done():
			stageErr = ctx.Err()
			break stages
		default:
		}
		prompt := expandSpecKitPrompt(stage.PromptTemplate, map[string]string{
			"TASK":              run.Task.TaskPrompt,
			"TECHNICAL_DETAILS": run.Task.TechnicalDetail,
		})
		result, err := exec.Execute(ctx, harness.MergeConfig(run.BaseRunConfig, executor.RunConfig{
			Prompt:        prompt,
			WorkspacePath: run.Workspace.Path,
			Stage:         stage.Name,
			Role:          stage.Role,
		}))
		if result == nil {
			result = &executor.RunResult{}
		}
		stageNames = append(stageNames, stage.Name)
		stageResults = append(stageResults, result)
		if err != nil {
			stageErr = fmt.Errorf("speckit: stage %s: %w", stage.Name, err)
			break stages
		}
	}
	return mergeStageTranscripts(stageNames, stageResults), stageErr
}

// expandSpecKitPrompt replaces {{TASK}} and {{TECHNICAL_DETAILS}}
// literally; unknown tokens are preserved (a stage might genuinely
// want to emit literal {{X}}).
func expandSpecKitPrompt(template string, vars map[string]string) string {
	out := template
	out = strings.ReplaceAll(out, "{{TASK}}", vars["TASK"])
	out = strings.ReplaceAll(out, "{{TECHNICAL_DETAILS}}", vars["TECHNICAL_DETAILS"])
	return out
}
```

Verify `mergeStageTranscripts` is still defined in the file; if not, restore it (it was the existing helper that wraps `--- Stage: <name> ---` markers).

Verify the imports still include `strings`, `fmt`, `errors`, and now `speckit`.

- [ ] **Step 5: Re-run Invoke tests; must pass**

```bash
cd engine && go test ./internal/builtin/harness/ -run 'TestSpecKit|TestExpandSpecKitPrompt' -v
```

Expected: every PASS.

- [ ] **Step 6: Run the full engine suite**

```bash
cd engine && go test ./...
```

Expected: all packages green.

- [ ] **Step 7: Commit**

```bash
git add engine/internal/builtin/harness/speckit.go engine/internal/builtin/harness/speckit_test.go
git commit -m "speckit harness: catalog-driven Invoke with role threading + expandSpecKitPrompt"
```

---

## Task 4 — Catalog API endpoint (TDD)

**Files:**
- Create: `engine/internal/api/speckit_catalog_handler.go`
- Create: `engine/internal/api/speckit_catalog_handler_test.go`
- Modify: `engine/internal/api/router.go`

- [ ] **Step 1: Write the failing handler test**

Create `engine/internal/api/speckit_catalog_handler_test.go`:

```go
package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSpecKitCatalogHandlerReturnsAllEntries(t *testing.T) {
	svc := &Service{}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/harnesses/speckit/catalog", nil)
	svc.ListSpecKitCatalog(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d body=%s", rec.Code, rec.Body.String())
	}
	var got []SpecKitExtensionPublic
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v body=%s", err, rec.Body.String())
	}
	if len(got) != 6 {
		t.Errorf("entry count: got %d want 6", len(got))
	}
	if got[0].ID != "canonical" {
		t.Errorf("first id: got %q want canonical", got[0].ID)
	}
	// Stage clipping: the public shape carries name + slash_command + role,
	// NOT the full prompt template.
	if len(got[0].Stages) != 4 {
		t.Errorf("canonical stage count: got %d want 4", len(got[0].Stages))
	}
	if got[0].Stages[0].SlashCommand != "/speckit.specify" {
		t.Errorf("first stage slash: got %q", got[0].Stages[0].SlashCommand)
	}
}
```

- [ ] **Step 2: Run; tests must fail (handler + types don't exist)**

```bash
cd engine && go test ./internal/api/ -run TestSpecKitCatalog -v
```

Expected: compile errors about `SpecKitExtensionPublic`, `ListSpecKitCatalog`.

- [ ] **Step 3: Implement the handler**

Create `engine/internal/api/speckit_catalog_handler.go`:

```go
package api

import (
	"net/http"

	"github.com/mustafaselman/frameval/engine/internal/builtin/speckit"
)

// SpecKitExtensionPublic is the wire shape exposed to the frontend. It
// clips Stage.PromptTemplate (server-only — keeps prompt engineering
// out of public API surface) but carries everything the launcher needs
// to render the catalog picker.
type SpecKitExtensionPublic struct {
	ID          string              `json:"id"`
	Name        string              `json:"name"`
	Description string              `json:"description"`
	Stages      []StagePublic       `json:"stages"`
	MultiAgent  bool                `json:"multi_agent"`
	SourceURL   string              `json:"source_url,omitempty"`
}

type StagePublic struct {
	Name         string `json:"name"`
	SlashCommand string `json:"slash_command"`
	Role         string `json:"role,omitempty"`
}

// ListSpecKitCatalog returns every curated extension. Catalog is static
// within a process so no caching concerns at this layer.
func (s *Service) ListSpecKitCatalog(w http.ResponseWriter, _ *http.Request) {
	entries := speckit.List()
	out := make([]SpecKitExtensionPublic, 0, len(entries))
	for _, e := range entries {
		stages := make([]StagePublic, 0, len(e.Stages))
		for _, st := range e.Stages {
			stages = append(stages, StagePublic{
				Name:         st.Name,
				SlashCommand: st.SlashCommand,
				Role:         st.Role,
			})
		}
		out = append(out, SpecKitExtensionPublic{
			ID:          e.ID,
			Name:        e.Name,
			Description: e.Description,
			Stages:      stages,
			MultiAgent:  e.MultiAgent,
			SourceURL:   e.SourceURL,
		})
	}
	JSON(w, http.StatusOK, out)
}
```

- [ ] **Step 4: Register the route**

In `engine/internal/api/router.go`, find the existing harness endpoints (`r.Get("/config/harnesses", ...)` around line 123) and add immediately below:

```go
		r.Get("/harnesses/speckit/catalog", service.ListSpecKitCatalog)
```

- [ ] **Step 5: Re-run the handler test; must pass**

```bash
cd engine && go test ./internal/api/ -run TestSpecKitCatalog -v
```

Expected: PASS.

- [ ] **Step 6: Run the full engine suite**

```bash
cd engine && go test ./...
```

Expected: all packages green.

- [ ] **Step 7: Commit**

```bash
git add engine/internal/api/speckit_catalog_handler.go engine/internal/api/speckit_catalog_handler_test.go engine/internal/api/router.go
git commit -m "Add GET /api/harnesses/speckit/catalog endpoint"
```

---

## Task 5 — Frontend types + hook

**Files:**
- Modify: `frontend/src/lib/types.ts`
- Modify: `frontend/src/lib/hooks.ts`

- [ ] **Step 1: Add the types**

Append to `frontend/src/lib/types.ts` (near the other catalog-like types — `ExecutorInfo`, `ModelConfig` etc.):

```ts
export type SpecKitStagePublic = {
  name: string;
  slash_command: string;
  role?: string;
};

export type SpecKitExtensionPublic = {
  id: string;
  name: string;
  description: string;
  stages: SpecKitStagePublic[];
  multi_agent: boolean;
  source_url?: string;
};

export type SpecKitConfig = {
  // Per-cell wire shape. Launcher state holds a multi-id list; matrix
  // expansion narrows it to one id per cell before posting.
  extension_id: string;
};
```

- [ ] **Step 2: Add the hook**

In `frontend/src/lib/hooks.ts`, locate `useHarnesses` (around line 249). Add a sibling immediately below:

```ts
export function useSpecKitCatalog() {
  return useQuery({
    queryKey: ['speckit', 'catalog'] as const,
    queryFn: () => api.get<SpecKitExtensionPublic[]>('/harnesses/speckit/catalog'),
    staleTime: 24 * 60 * 60 * 1000, // catalog is static within a process
  });
}
```

Extend the type import at the top of `hooks.ts` to include `SpecKitExtensionPublic` if it doesn't already. Check the existing `import type {...} from './types'` block — if it's a wildcard `import * as types from './types'`, no edit is needed.

- [ ] **Step 3: Typecheck**

```bash
cd frontend && npm run lint
```

Expected: exits 0.

- [ ] **Step 4: Commit**

```bash
git add frontend/src/lib/types.ts frontend/src/lib/hooks.ts
git commit -m "Add SpecKit catalog types + useSpecKitCatalog hook"
```

---

## Task 6 — SpecKitForm component (TDD)

**Files:**
- Create: `frontend/src/components/launcher/SpecKitForm.tsx`
- Create: `frontend/src/components/launcher/SpecKitForm.test.tsx`

- [ ] **Step 1: Write the failing test**

Create `frontend/src/components/launcher/SpecKitForm.test.tsx`:

```tsx
import { describe, expect, it, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import type { SpecKitExtensionPublic } from '../../lib/types';
import { SpecKitForm } from './SpecKitForm';

const CATALOG: SpecKitExtensionPublic[] = [
  { id: 'canonical', name: 'Canonical (4-stage)', description: 'baseline', stages: [], multi_agent: false },
  { id: 'lite', name: 'Lite (2-stage)', description: 'minimal', stages: [], multi_agent: false },
  { id: 'dual-role', name: 'Dual-role (multi-agent)', description: 'role-tagged', stages: [], multi_agent: true },
];

function setupQuery(initialData: SpecKitExtensionPublic[]) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  client.setQueryData(['speckit', 'catalog'], initialData);
  return client;
}

function Wrap({ children, client }: { children: React.ReactNode; client: QueryClient }) {
  return <QueryClientProvider client={client}>{children}</QueryClientProvider>;
}

describe('SpecKitForm', () => {
  it('seeds canonical on mount when value is undefined', () => {
    const onChange = vi.fn();
    const client = setupQuery(CATALOG);
    render(
      <Wrap client={client}>
        <SpecKitForm value={undefined} onChange={onChange} />
      </Wrap>,
    );
    expect(onChange).toHaveBeenCalledTimes(1);
    expect(onChange).toHaveBeenCalledWith({ extension_ids: ['canonical'] });
  });

  it('renders one chip per catalog entry', () => {
    const onChange = vi.fn();
    const client = setupQuery(CATALOG);
    render(
      <Wrap client={client}>
        <SpecKitForm value={{ extension_ids: ['canonical'] }} onChange={onChange} />
      </Wrap>,
    );
    expect(screen.getByRole('button', { name: /Canonical/i })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /Lite/i })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /Dual-role/i })).toBeInTheDocument();
  });

  it('shows the Multi-agent badge on the dual-role chip', () => {
    const onChange = vi.fn();
    const client = setupQuery(CATALOG);
    render(
      <Wrap client={client}>
        <SpecKitForm value={{ extension_ids: ['canonical'] }} onChange={onChange} />
      </Wrap>,
    );
    const dualBtn = screen.getByRole('button', { name: /Dual-role/i });
    expect(dualBtn.textContent).toMatch(/Multi-agent/i);
  });

  it('toggles selection on chip click', () => {
    const onChange = vi.fn();
    const client = setupQuery(CATALOG);
    render(
      <Wrap client={client}>
        <SpecKitForm value={{ extension_ids: ['canonical'] }} onChange={onChange} />
      </Wrap>,
    );
    fireEvent.click(screen.getByRole('button', { name: /Lite/i }));
    expect(onChange).toHaveBeenLastCalledWith({ extension_ids: ['canonical', 'lite'] });
  });

  it('shows the empty-catalog fallback message', () => {
    const onChange = vi.fn();
    const client = setupQuery([]);
    render(
      <Wrap client={client}>
        <SpecKitForm value={{ extension_ids: [] }} onChange={onChange} />
      </Wrap>,
    );
    expect(screen.getByText(/could not load/i)).toBeInTheDocument();
  });
});
```

- [ ] **Step 2: Run; tests must fail (component missing)**

```bash
cd frontend && npx vitest run src/components/launcher/SpecKitForm.test.tsx
```

Expected: "Cannot find module './SpecKitForm'".

- [ ] **Step 3: Implement the component**

Create `frontend/src/components/launcher/SpecKitForm.tsx`:

```tsx
import { useEffect } from 'react';
import { useSpecKitCatalog } from '../../lib/hooks';
import type { SpecKitExtensionPublic } from '../../lib/types';

interface FormValue {
  extension_ids?: string[];
}

interface FormProps {
  value: FormValue | undefined;
  onChange: (next: { extension_ids: string[] }) => void;
}

/**
 * SpecKitForm — multi-select chip list of curated spec-kit extensions.
 *
 * The launcher tracks `extension_ids` (multi-select). Matrix expansion
 * inside launch.tsx narrows that list to one id per launch cell before
 * posting (so the per-variant wire shape stays `{ extension_id: <one> }`).
 *
 * Seeds `{ extension_ids: ['canonical'] }` on first render with an
 * undefined value so the gate doesn't trip the moment the user picks
 * the speckit harness chip.
 */
export function SpecKitForm({ value, onChange }: FormProps) {
  const query = useSpecKitCatalog();
  const selected = value?.extension_ids ?? [];

  useEffect(() => {
    if (value === undefined) {
      onChange({ extension_ids: ['canonical'] });
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const toggle = (id: string) => {
    const next = selected.includes(id)
      ? selected.filter((x) => x !== id)
      : [...selected, id];
    onChange({ extension_ids: next });
  };

  if (query.isError) {
    return (
      <div className="mt-2 rounded-md border border-border bg-bg-elev-1 p-3 text-xs text-fg-muted">
        Could not load spec-kit catalog. Check the engine logs.
      </div>
    );
  }
  const catalog = query.data ?? [];
  if (catalog.length === 0) {
    return (
      <div className="mt-2 rounded-md border border-border bg-bg-elev-1 p-3 text-xs text-fg-muted">
        Could not load spec-kit catalog.
      </div>
    );
  }

  return (
    <div className="mt-2 rounded-md border border-border bg-bg-elev-1 p-3">
      <div className="mb-2 text-xs uppercase tracking-wider text-fg-muted">
        Spec-kit extensions
      </div>
      <p className="mb-3 text-xs text-fg-muted">
        Each selected extension becomes its own variant. N selections × tasks × executors × models = N×… experiments.
      </p>
      <div className="flex flex-wrap gap-2">
        {catalog.map((ext) => renderChip(ext, selected.includes(ext.id), () => toggle(ext.id)))}
      </div>
    </div>
  );
}

function renderChip(ext: SpecKitExtensionPublic, isSelected: boolean, onClick: () => void) {
  const stateClasses = isSelected
    ? 'border-fg bg-bg-elev-2 text-fg'
    : 'border-border bg-bg-elev-1 text-fg-muted hover:border-border-strong';
  return (
    <button
      key={ext.id}
      type="button"
      onClick={onClick}
      title={ext.description}
      className={`flex items-center gap-2 rounded-md border px-2.5 py-1.5 text-xs transition ${stateClasses}`}
    >
      <span className="font-medium">{ext.name}</span>
      {ext.multi_agent && (
        <span className="rounded-full border border-border bg-bg-elev-2 px-1.5 py-0.5 text-[10px] uppercase tracking-wider text-fg-muted">
          Multi-agent
        </span>
      )}
    </button>
  );
}
```

- [ ] **Step 4: Re-run; tests must pass**

```bash
cd frontend && npx vitest run src/components/launcher/SpecKitForm.test.tsx
```

Expected: 5/5 PASS.

- [ ] **Step 5: Run full Vitest suite — no regression**

```bash
cd frontend && npm test -- --run 2>&1 | tail -10
```

Expected: every test green.

- [ ] **Step 6: Commit**

```bash
git add frontend/src/components/launcher/SpecKitForm.tsx frontend/src/components/launcher/SpecKitForm.test.tsx
git commit -m "Add SpecKitForm component (multi-select chip list backed by /api/harnesses/speckit/catalog)"
```

---

## Task 7 — HarnessConfigPanel switch case

**Files:**
- Modify: `frontend/src/components/launcher/HarnessConfigPanel.tsx`

- [ ] **Step 1: Add the case**

In `frontend/src/components/launcher/HarnessConfigPanel.tsx`, add the import (alphabetical with the others):

```ts
import { SpecKitForm } from './SpecKitForm';
```

In the `switch (harnessId)` block, add a new case before `default`:

```tsx
case 'speckit':
  return (
    <SpecKitForm
      value={value as { extension_ids?: string[] } | undefined}
      onChange={(next) => onChange(next as unknown as Record<string, unknown>)}
    />
  );
```

- [ ] **Step 2: Typecheck + test**

```bash
cd frontend && npm run lint && npm test -- --run 2>&1 | tail -8
```

Expected: clean.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/components/launcher/HarnessConfigPanel.tsx
git commit -m "Register speckit case in HarnessConfigPanel switch"
```

---

## Task 8 — Matrix expansion 4th axis (TDD)

**Files:**
- Modify: `frontend/src/pages/diagnostic/launch-matrix.ts`
- Modify: `frontend/src/pages/diagnostic/launch-matrix.test.ts`

- [ ] **Step 1: Append failing tests for the new axis**

Append to `frontend/src/pages/diagnostic/launch-matrix.test.ts`:

```ts
describe('expandLaunchMatrix — speckit axis', () => {
  it('multiplies cells by speckitExtensions count', () => {
    const cells = expandLaunchMatrix({
      taskIds: ['t'],
      executorIds: ['e'],
      modelIds: ['m'],
      speckitExtensions: ['canonical', 'lite', 'dual-role'],
    });
    expect(cells).toHaveLength(3);
    expect(cells.map((c) => c.speckitExtension)).toEqual(['canonical', 'lite', 'dual-role']);
  });

  it('collapses speckit axis to one empty-string cell when not provided', () => {
    const cells = expandLaunchMatrix({
      taskIds: ['t'],
      executorIds: ['e'],
      modelIds: ['m'],
      speckitExtensions: [''],
    });
    expect(cells).toHaveLength(1);
    expect(cells[0].speckitExtension).toBe('');
  });

  it('countExperiments multiplies by speckitExtensions', () => {
    expect(countExperiments({
      taskIds: ['a', 'b'],
      executorIds: ['e'],
      modelIds: ['m'],
      speckitExtensions: ['canonical', 'lite'],
    })).toBe(4);
  });
});
```

Also update the *existing* `expandLaunchMatrix` tests in the same file: every existing `ExpansionInput` literal must add `speckitExtensions: ['']` so the type matches. Read the file, find each existing test, append the new field.

- [ ] **Step 2: Run; new tests fail; existing tests now hit compile errors about the missing field**

```bash
cd frontend && npx vitest run src/pages/diagnostic/launch-matrix.test.ts
```

Expected: compile/type errors.

- [ ] **Step 3: Extend the helper**

Replace the body of `frontend/src/pages/diagnostic/launch-matrix.ts` with:

```ts
/**
 * Pure expansion of the launcher's variant matrix into experiment cells.
 *
 * Frameval distinguishes two axes:
 *   - Across experiments: task × executor × model × speckit-extension.
 *     Each cell becomes one experiment. Multiple cells form a batch.
 *   - Within an experiment: harnesses become variants of that single
 *     experiment so the Compare view can score them side-by-side.
 *
 * The speckit-extension axis collapses to a single empty-string entry
 * when the user hasn't selected the speckit harness or hasn't picked
 * any extensions — so non-speckit selections keep producing the same
 * cell counts they did before this axis existed.
 *
 * The launcher uses this expansion to decide how many `/diagnostic/launch`
 * calls to fire and what batch identity to share across them.
 */

export interface LaunchCell {
  taskId: string;
  executorId: string;
  modelId: string;
  speckitExtension: string;
}

export interface ExpansionInput {
  taskIds: string[];
  executorIds: string[];
  modelIds: string[];
  speckitExtensions: string[];
}

/** Number of experiments the current selection will produce. */
export function countExperiments(input: ExpansionInput): number {
  return Math.max(input.taskIds.length, 1)
    * Math.max(input.executorIds.length, 1)
    * Math.max(input.modelIds.length, 1)
    * Math.max(input.speckitExtensions.length, 1);
}

/**
 * Expand the (task × executor × model × speckit-extension) cross-product
 * into one cell per experiment. Order is stable: tasks outermost,
 * speckit-extensions innermost — matches the order the variant preview
 * list already uses.
 */
export function expandLaunchMatrix(input: ExpansionInput): LaunchCell[] {
  const out: LaunchCell[] = [];
  for (const taskId of input.taskIds) {
    for (const executorId of input.executorIds) {
      for (const modelId of input.modelIds) {
        for (const speckitExtension of input.speckitExtensions) {
          out.push({ taskId, executorId, modelId, speckitExtension });
        }
      }
    }
  }
  return out;
}
```

- [ ] **Step 4: Re-run tests; must pass**

```bash
cd frontend && npx vitest run src/pages/diagnostic/launch-matrix.test.ts
```

Expected: every test PASS.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/pages/diagnostic/launch-matrix.ts frontend/src/pages/diagnostic/launch-matrix.test.ts
git commit -m "Add speckit-extension as 4th axis to expandLaunchMatrix"
```

---

## Task 9 — Launcher page: selectedExtensions derive + per-cell payload + gate + label

**Files:**
- Modify: `frontend/src/pages/diagnostic/launch.tsx`

- [ ] **Step 1: Derive `selectedExtensions`**

In `frontend/src/pages/diagnostic/launch.tsx`, find the block where `handleLaunch` builds `cells`. Just above the `expandLaunchMatrix` call (whichever line that is), insert:

```ts
const speckitConfig = harnessConfigs.speckit as { extension_ids?: string[] } | undefined;
const selectedExtensions = selectedHarnesses.includes('speckit')
  ? (speckitConfig?.extension_ids ?? []).filter((id) => id.length > 0)
  : [''];
const speckitAxis = selectedExtensions.length > 0 ? selectedExtensions : [''];
```

Then update the `expandLaunchMatrix({ ... })` call to include the 4th axis:

```ts
const cells = expandLaunchMatrix({
  taskIds: taskIDs,
  executorIds: selectedExecutors,
  modelIds: selectedModels,
  speckitExtensions: speckitAxis,
});
```

- [ ] **Step 2: Narrow the per-cell harness_configs.speckit shape**

In both the single-cell and multi-cell branches of `handleLaunch`, just before the `await launch.mutateAsync({...})` call, build a cell-specific configs map and use it in the payload:

```ts
const cellConfigs: Record<string, unknown> = { ...harnessConfigs };
if (cell.speckitExtension) {
  cellConfigs.speckit = { extension_id: cell.speckitExtension };
} else if ('speckit' in cellConfigs) {
  delete cellConfigs.speckit;
}
```

Use `harness_configs: cellConfigs` (instead of `harness_configs: harnessConfigs`) in the payload. Do this in BOTH the single-cell `await launch.mutateAsync({...})` and the multi-cell `Promise.allSettled(cells.map((cell) => launch.mutateAsync({...})))` paths.

- [ ] **Step 3: Add the submit gate**

Find the gate block (where `agentInstructionsReady` and `multiagentReady` are derived). Add a parallel speckit gate:

```ts
const needsSpecKit = selectedHarnesses.includes('speckit');
const speckitReady = !needsSpecKit
  || ((harnessConfigs.speckit as { extension_ids?: string[] } | undefined)?.extension_ids?.length ?? 0) > 0;
```

Update `canSubmit` to AND in `speckitReady`:

```ts
const canSubmit =
  taskIDs.length > 0
  && variants.length > 0
  && agentInstructionsReady
  && multiagentReady
  && speckitReady
  && !launch.isPending;
```

- [ ] **Step 4: Add the button label branch**

Find the Launch button label ternary chain. Add a branch right after `!multiagentReady ? 'Configure multiagent roles' : ...`:

```tsx
: !speckitReady
? 'Pick a spec-kit extension'
: totalExperiments === 1
? 'Launch · 1 task'
: `Launch suite · ${totalExperiments} experiments`
```

- [ ] **Step 5: Multiply `totalExperiments` by the speckit axis count**

Find the existing `totalExperiments` derivation. Add the speckit factor:

```ts
const speckitFactor = needsSpecKit
  ? Math.max(((harnessConfigs.speckit as { extension_ids?: string[] } | undefined)?.extension_ids?.length ?? 0), 1)
  : 1;
const totalExperiments = countExperiments({
  taskIds: taskIDs,
  executorIds: selectedExecutors,
  modelIds: selectedModels,
  speckitExtensions: needsSpecKit && (harnessConfigs.speckit as { extension_ids?: string[] } | undefined)?.extension_ids?.length
    ? (harnessConfigs.speckit as { extension_ids?: string[] }).extension_ids!
    : [''],
});
```

(If the existing code computed `totalExperiments` from `Math.max(...)` chains rather than `countExperiments`, switch to the `countExperiments` helper now that the 4th axis exists. Keep behavior consistent with the matrix expansion.)

- [ ] **Step 6: Typecheck + tests + build**

```bash
cd frontend && npm run lint && npm test -- --run 2>&1 | tail -8 && npm run build 2>&1 | tail -3
```

Expected: all clean.

- [ ] **Step 7: Commit**

```bash
git add frontend/src/pages/diagnostic/launch.tsx
git commit -m "Launcher: speckit multi-select drives 4th matrix axis + gate + button label"
```

---

## Task 10 — Run Inspect SpecKitExtensionCard

**Files:**
- Modify: `frontend/src/pages/runs/inspect.tsx`

- [ ] **Step 1: Add the card render**

In `frontend/src/pages/runs/inspect.tsx`, find the existing `<AgentInstructionsCard>` render block (added in Project 1). Add a sibling render right below it:

```tsx
{variant?.harness_config?.speckit?.extension_id && (
  <div className="mt-3">
    <SpecKitExtensionCard
      extensionId={String(variant.harness_config.speckit.extension_id)}
    />
  </div>
)}
```

- [ ] **Step 2: Add the helper component at the bottom of the file**

Append below the existing `AgentInstructionsCard` definition:

```tsx
function SpecKitExtensionCard({ extensionId }: { extensionId: string }) {
  const [open, setOpen] = useState(true);
  const query = useSpecKitCatalog();
  const ext = query.data?.find((e) => e.id === extensionId);
  if (!ext) {
    return (
      <Card>
        <div className="text-sm font-semibold text-fg">Spec-kit extension</div>
        <div className="text-xs text-fg-muted">Loading catalog or extension <code className="font-mono">{extensionId}</code> not found.</div>
      </Card>
    );
  }
  return (
    <Card>
      <div className="flex items-center justify-between">
        <div>
          <div className="flex items-center gap-2 text-sm font-semibold text-fg">
            Spec-kit extension
            {ext.multi_agent && (
              <span className="rounded-full border border-border bg-bg-elev-2 px-1.5 py-0.5 text-[10px] uppercase tracking-wider text-fg-muted">
                Multi-agent
              </span>
            )}
          </div>
          <div className="text-xs text-fg-muted">{ext.name} — {ext.description}</div>
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
        <ol className="mt-3 space-y-1 text-xs text-fg-muted">
          {ext.stages.map((st, i) => (
            <li key={st.name} className="font-mono">
              {i + 1}. {st.name}{' '}
              <span className="text-fg-subtle">({st.slash_command})</span>
              {st.role && <span className="ml-2 text-fg-muted">role: {st.role}</span>}
            </li>
          ))}
        </ol>
      )}
    </Card>
  );
}
```

Make sure the file's import block includes `useSpecKitCatalog` from `../../lib/hooks` and `useState` from `react`.

- [ ] **Step 3: Typecheck + tests + build**

```bash
cd frontend && npm run lint && npm test -- --run 2>&1 | tail -8 && npm run build 2>&1 | tail -3 && npm run check:tokens 2>&1 | tail -3
```

Expected: all clean.

- [ ] **Step 4: Commit**

```bash
git add frontend/src/pages/runs/inspect.tsx
git commit -m "Run inspect: surface the spec-kit extension used in the run"
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

- [ ] **Step 2: Verify the catalog endpoint**

```bash
curl -s http://localhost:8080/api/harnesses/speckit/catalog | python3 -c "
import json,sys
catalog = json.load(sys.stdin)
print(f'entries: {len(catalog)}')
for e in catalog:
    flag = ' [multi-agent]' if e.get('multi_agent') else ''
    print(f\"  {e['id']:<14} {e['name']}{flag}\")
"
```

Expected: 6 entries, canonical first, dual-role flagged.

- [ ] **Step 3: Browser walkthrough**

Visit `http://localhost:5173/diagnostic/launch`. Confirm:

1. Pick the `speckit` chip. A "Spec-kit extensions" panel appears below with 6 chips. Canonical is pre-selected (highlighted).
2. Click `Lite` and `Dual-role` to multi-select. The dual-role chip shows the "Multi-agent" badge.
3. The bottom preview reads "3 experiments × N runs each" (3 selections, single task/exec/model).
4. Deselect all 3 → button reads "Pick a spec-kit extension" + disabled.
5. Re-select canonical → button reads "Launch · 1 task".
6. Launch with 3 selected (canonical + lite + dual-role) and a 1-task / 1-exec / 1-model selection. URL redirects to `/experiments?batch=<id>`. 3 variants visible in the batched group.

- [ ] **Step 4: Inspector verification**

After the runs complete (or while they're in progress):

1. Open the canonical run in the Inspector. The right-side aside shows "Spec-kit extension" card with `Canonical (4-stage)` and its 4 stages listed.
2. Open the dual-role run. The card shows `Dual-role (multi-agent)` with the Multi-agent badge and 4 stages tagged with `role: architect / coder`.
3. Scroll through the dual-role run's turn groups. Groups corresponding to architect stages have one accent color; coder-stage groups have a different accent color. Both groups show the small "Role: architect" / "Role: coder" header above their content.

- [ ] **Step 5: Empty-config rejection smoke**

```bash
curl -s -X POST http://localhost:8080/api/diagnostic/launch \
  -H 'Content-Type: application/json' \
  -d '{
    "task_id":"brownfield-fix-async-race",
    "executor_id":"opencode",
    "harness_ids":["speckit"],
    "model":"opencode/deepseek-v4-flash-free",
    "runs_per_variant":1
  }'
```

Expected: 202 returned, but the run transitions to `failed` within ~5s with an error message mentioning `ErrSpecKitExtensionMissing`.

- [ ] **Step 6: Stop servers**

```bash
lsof -ti tcp:8080 | xargs -r kill 2>/dev/null
lsof -ti tcp:5173 | xargs -r kill 2>/dev/null
```

---

## Task 13 — Push, open PR, request review, watch CI, merge

**Files:** none

- [ ] **Step 1: Push**

```bash
git push -u origin feature/speckit-catalog
```

- [ ] **Step 2: Open PR**

```bash
gh pr create --title "Spec-kit catalog: 6 curated extensions, multi-select 4th matrix axis" --body "$(cat <<'EOF'
## Summary

Project 3 (and final) of the harness refresh. Replaces the hardcoded canonical 4-stage spec-kit with a curated 6-entry catalog (canonical, lite, tdd-first, research-first, rigorous, dual-role), exposed in the launcher as a multi-select. Each selected extension becomes its own variant via a new 4th matrix axis. Run Inspect surfaces the chosen extension on every run.

- **Catalog:** new \`engine/internal/builtin/speckit\` package with 6 hand-curated entries. Canonical preserves today's exact stage prompts (regression-guarded). Dual-role tags its stages with role names so Project 2's Inspector role accent renders automatically.
- **Harness:** \`speckit\` now reads \`cfg[\"speckit\"][\"extension_id\"]\` (via Project 1's per-variant plumbing). Empty / unknown → \`ErrSpecKitExtensionMissing\` / \`ErrSpecKitExtensionNotFound\`. Setup stashes the resolved entry; Invoke walks \`extension.Stages\` with \`{{TASK}}\` / \`{{TECHNICAL_DETAILS}}\` substitution.
- **API:** new \`GET /api/harnesses/speckit/catalog\` returns the public catalog shape (id, name, description, stages, multi-agent flag, optional source URL). Prompt templates stay server-only.
- **Launcher:** new \`<SpecKitForm>\` multi-select chip list backed by \`useSpecKitCatalog()\`. Seeds canonical on mount. Matrix expansion gains a 4th axis (\`speckitExtensions\`) — N selections × tasks × executors × models = N×… experiments, all batched together via PR #143's group plumbing.
- **Inspector:** \`<SpecKitExtensionCard>\` mirrors Project 1's pattern in the aside panel; shows the resolved extension's name, multi-agent badge, and ordered stage list with slash commands and role tags.

Spec: [\`docs/superpowers/specs/2026-05-31-speckit-catalog-design.md\`](docs/superpowers/specs/2026-05-31-speckit-catalog-design.md)
Plan: [\`docs/superpowers/plans/2026-05-31-speckit-catalog.md\`](docs/superpowers/plans/2026-05-31-speckit-catalog.md)

## Test plan

- [x] \`go test ./...\` clean — catalog list/lookup/canonical-prompts/dual-role-roles, harness Setup + Invoke + dual-role + expandSpecKitPrompt, catalog handler endpoint
- [x] \`npm run lint\` / \`build\` / \`test\` / \`check:tokens\` clean — \`SpecKitForm\` cases, \`launch-matrix\` new-axis cases
- [x] Manual: catalog endpoint returns 6 entries; launcher multi-select with 3 extensions → 3 batched variants; Inspector renders the SpecKitExtensionCard and the dual-role run shows per-role accents on turn groups
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
- Catalog data + Lookup + canonical regression guard → Task 1
- Harness extension-aware Setup → Task 2
- Harness extension-aware Invoke + expandSpecKitPrompt → Task 3
- API endpoint → Task 4
- Frontend types + hook → Task 5
- SpecKitForm multi-select component → Task 6
- HarnessConfigPanel switch case → Task 7
- Matrix expansion 4th axis → Task 8
- Launcher gate + button label + per-cell payload narrowing → Task 9
- Inspector card → Task 10
- Sanity → Task 11
- Manual + API smoke → Task 12
- PR + review + CI + merge → Task 13
- Out-of-scope items (live fetch, custom extensions, per-stage timing, compare side-by-side) — explicitly excluded by spec; no tasks needed.

**No placeholders** — every step shows complete code (catalog struct, harness Setup body, Invoke loop, React component, matrix expansion, page integration, Inspector card), exact commands, exact expected output. Manual steps cite concrete URLs and curl payloads.

**Type consistency:**
- Go: `speckit.SpecKitExtension`, `speckit.Stage`, `speckit.List()`, `speckit.Lookup()`, `ErrSpecKitExtensionMissing`, `ErrSpecKitExtensionNotFound`, `metadataKeySpecKitExtension`, `expandSpecKitPrompt`. Used consistently across Tasks 1–4.
- TypeScript: `SpecKitStagePublic`, `SpecKitExtensionPublic`, `SpecKitConfig`, `useSpecKitCatalog`, `<SpecKitForm>`, `<SpecKitExtensionCard>`, `selectedExtensions`, `speckitAxis`, `cellConfigs`. Used consistently across Tasks 5–10.
- Wire shape `harness_configs.speckit.extension_id` (singular) on POST. Launcher state holds `extension_ids` (plural) — narrowed per-cell in Task 9.

**Ordering** — catalog → harness setup → harness invoke → API → frontend types/hook → form → panel switch → matrix expansion → launcher integration → inspector → sanity → manual → PR. Each task is testable on its own.
