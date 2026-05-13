package harness

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mustafaselman/frameval/engine/pkg/executor"
	pkgharness "github.com/mustafaselman/frameval/engine/pkg/harness"
	"github.com/mustafaselman/frameval/engine/pkg/task"
)

// recordingExecutor captures every Execute call so tests can assert the
// per-stage prompt and Stage routing.
type recordingExecutor struct {
	calls []executor.RunConfig
	retErr error
}

func (r *recordingExecutor) Name() string                                          { return "recorder" }
func (r *recordingExecutor) SupportedModes() []executor.ExecutionMode              { return []executor.ExecutionMode{executor.ExecutionModeCLI} }
func (r *recordingExecutor) ParseTranscript(raw []byte) ([]executor.ParsedTurn, error) { return nil, nil }
func (r *recordingExecutor) Execute(_ context.Context, cfg executor.RunConfig) (*executor.RunResult, error) {
	r.calls = append(r.calls, cfg)
	if r.retErr != nil {
		return &executor.RunResult{RawOutput: "stage-failed"}, r.retErr
	}
	return &executor.RunResult{
		RawOutput:   "stage-output for " + cfg.Stage,
		ParsedTurns: []executor.ParsedTurn{{Role: "assistant", Content: cfg.Stage + "-turn"}},
	}, nil
}

func TestSpecKitIdentity(t *testing.T) {
	h := NewSpecKit()
	if h.Name() != "speckit" {
		t.Errorf("Name = %q", h.Name())
	}
	if h.Description() == "" {
		t.Error("Description should not be empty")
	}
}

func TestSpecKitSetupCreatesSpecifyDir(t *testing.T) {
	h := NewSpecKit()
	ws := pkgharness.Workspace{Path: t.TempDir()}
	tk := task.Task{ID: "fixture"}

	run, err := h.Setup(context.Background(), ws, tk, pkgharness.Budget{})
	if err != nil {
		t.Fatalf("Setup: %v", err)
	}
	if owned, _ := run.Metadata[metadataKeyOwnsSpecify].(bool); !owned {
		t.Error("Setup should mark .specify as owned on fresh workspace")
	}
	memoryDir := filepath.Join(ws.Path, ".specify", "memory")
	if _, err := os.Stat(memoryDir); err != nil {
		t.Errorf(".specify/memory not created: %v", err)
	}
}

func TestSpecKitSetupCopiesConstitutionWhenPresent(t *testing.T) {
	h := NewSpecKit()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "harness_context"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	want := "## Frameval Constitution\nbe deterministic"
	if err := os.WriteFile(filepath.Join(root, "harness_context", "constitution.md"), []byte(want), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	ws := pkgharness.Workspace{Path: t.TempDir()}
	tk := task.Task{ID: "fixture", TaskRootPath: root}

	_, err := h.Setup(context.Background(), ws, tk, pkgharness.Budget{})
	if err != nil {
		t.Fatalf("Setup: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(ws.Path, ".specify", "memory", "constitution.md"))
	if err != nil {
		t.Fatalf("read constitution: %v", err)
	}
	if string(got) != want {
		t.Errorf("constitution content mismatch: got %q", string(got))
	}
}

func TestSpecKitSetupNoConstitutionIsOK(t *testing.T) {
	h := NewSpecKit()
	ws := pkgharness.Workspace{Path: t.TempDir()}
	tk := task.Task{ID: "no-constitution", TaskRootPath: t.TempDir()}

	if _, err := h.Setup(context.Background(), ws, tk, pkgharness.Budget{}); err != nil {
		t.Fatalf("Setup should succeed without constitution: %v", err)
	}
}

func TestSpecKitInvokeIssuesFourStages(t *testing.T) {
	h := NewSpecKit()
	rec := &recordingExecutor{}
	tk := task.Task{ID: "fixture", TaskPrompt: "build CLI", TechnicalDetail: "use click"}
	ws := pkgharness.Workspace{Path: t.TempDir()}
	run, _ := h.Setup(context.Background(), ws, tk, pkgharness.Budget{})

	result, err := h.Invoke(context.Background(), run, rec)
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	if len(rec.calls) != 4 {
		t.Fatalf("expected 4 stage invocations, got %d", len(rec.calls))
	}
	wantStages := []string{"specify", "plan", "tasks", "implement"}
	for i, want := range wantStages {
		if rec.calls[i].Stage != want {
			t.Errorf("call %d Stage = %q, want %q", i, rec.calls[i].Stage, want)
		}
	}
	if !strings.HasPrefix(rec.calls[0].Prompt, "/speckit.specify") {
		t.Errorf("specify prompt should start with /speckit.specify; got %q", rec.calls[0].Prompt)
	}
	if !strings.Contains(rec.calls[0].Prompt, "build CLI") {
		t.Errorf("specify prompt should inline task; got %q", rec.calls[0].Prompt)
	}
	if !strings.Contains(rec.calls[1].Prompt, "use click") {
		t.Errorf("plan prompt should inline technical details; got %q", rec.calls[1].Prompt)
	}
	// Merged transcript has 4 stage markers
	for _, stage := range wantStages {
		if !strings.Contains(result.RawOutput, "--- Stage: "+stage+" ---") {
			t.Errorf("merged transcript missing marker for stage %q", stage)
		}
	}
	// Turns tagged with stage
	for _, turn := range result.ParsedTurns {
		if turn.Stage == "" {
			t.Errorf("turn missing Stage tag: %+v", turn)
		}
	}
}

func TestSpecKitInvokeStopsOnStageError(t *testing.T) {
	h := NewSpecKit()
	rec := &recordingExecutor{retErr: errors.New("stage 2 broken")}
	tk := task.Task{TaskPrompt: "x"}
	ws := pkgharness.Workspace{Path: t.TempDir()}
	run, _ := h.Setup(context.Background(), ws, tk, pkgharness.Budget{})

	_, err := h.Invoke(context.Background(), run, rec)
	if err == nil {
		t.Fatal("expected stage error to surface")
	}
	// First stage runs, then error halts the pipeline
	if len(rec.calls) != 1 {
		t.Errorf("expected halt after first failing stage; got %d calls", len(rec.calls))
	}
}

func TestSpecKitInvokeRespectsContextCancellation(t *testing.T) {
	h := NewSpecKit()
	// recordingExecutor cancels the parent ctx mid-flight via a closure
	calls := 0
	ctx, cancel := context.WithCancel(context.Background())
	fake := &cancellingExecutor{cancel: cancel, callsBeforeCancel: 2, calls: &calls}
	tk := task.Task{TaskPrompt: "x"}
	ws := pkgharness.Workspace{Path: t.TempDir()}
	run, _ := h.Setup(context.Background(), ws, tk, pkgharness.Budget{})

	_, err := h.Invoke(ctx, run, fake)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
	// fake.Execute increments calls for each invocation. After cancellation
	// triggered on the 2nd call, no further stage Execute should happen.
	if calls > 2 {
		t.Errorf("expected at most 2 stage invocations before honoring cancellation, got %d", calls)
	}
}

type cancellingExecutor struct {
	cancel            context.CancelFunc
	callsBeforeCancel int
	calls             *int
}

func (c *cancellingExecutor) Name() string                                          { return "cancelling" }
func (c *cancellingExecutor) SupportedModes() []executor.ExecutionMode              { return []executor.ExecutionMode{executor.ExecutionModeCLI} }
func (c *cancellingExecutor) ParseTranscript(raw []byte) ([]executor.ParsedTurn, error) { return nil, nil }
func (c *cancellingExecutor) Execute(_ context.Context, cfg executor.RunConfig) (*executor.RunResult, error) {
	*c.calls++
	if *c.calls >= c.callsBeforeCancel {
		c.cancel()
	}
	return &executor.RunResult{RawOutput: "ok", ParsedTurns: []executor.ParsedTurn{{Role: "assistant", Content: cfg.Stage}}}, nil
}

func TestSpecKitTeardownRemovesOwnedDir(t *testing.T) {
	h := NewSpecKit()
	ws := pkgharness.Workspace{Path: t.TempDir()}
	run, _ := h.Setup(context.Background(), ws, task.Task{}, pkgharness.Budget{})

	if err := h.Teardown(context.Background(), run); err != nil {
		t.Fatalf("Teardown: %v", err)
	}
	if _, err := os.Stat(filepath.Join(ws.Path, ".specify")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf(".specify should be removed; got stat err %v", err)
	}
}

func TestSpecKitTeardownPreservesUnownedDir(t *testing.T) {
	h := NewSpecKit()
	ws := pkgharness.Workspace{Path: t.TempDir()}
	// Pre-create .specify so Setup's "did this directory exist" check sees it.
	if err := os.MkdirAll(filepath.Join(ws.Path, ".specify", "memory"), 0o755); err != nil {
		t.Fatalf("seed: %v", err)
	}
	run, err := h.Setup(context.Background(), ws, task.Task{}, pkgharness.Budget{})
	if err != nil {
		t.Fatalf("Setup: %v", err)
	}
	if owned, _ := run.Metadata[metadataKeyOwnsSpecify].(bool); owned {
		t.Error("Setup should NOT mark .specify as owned when it pre-existed")
	}
	if err := h.Teardown(context.Background(), run); err != nil {
		t.Fatalf("Teardown: %v", err)
	}
	if _, err := os.Stat(filepath.Join(ws.Path, ".specify")); err != nil {
		t.Errorf("pre-existing .specify should be preserved; got stat err %v", err)
	}
}

func TestRegistryListsSpecKit(t *testing.T) {
	r := NewRegistry()
	if _, err := r.Get("speckit"); err != nil {
		t.Errorf("expected speckit in default registry: %v", err)
	}
}
