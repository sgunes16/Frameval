package harness

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/mustafaselman/frameval/engine/pkg/executor"
	pkgharness "github.com/mustafaselman/frameval/engine/pkg/harness"
	"github.com/mustafaselman/frameval/engine/pkg/task"
)

func TestRalphIdentity(t *testing.T) {
	h := NewRalph()
	if h.Name() != "ralph" {
		t.Errorf("Name = %q", h.Name())
	}
	if h.Description() == "" {
		t.Error("Description should not be empty")
	}
}

func TestRalphSetupAppliesDefaultMaxIterations(t *testing.T) {
	h := NewRalph()
	run, err := h.Setup(context.Background(), pkgharness.Workspace{Path: t.TempDir()}, task.Task{}, pkgharness.Budget{}, nil)
	if err != nil {
		t.Fatalf("Setup: %v", err)
	}
	if run.Budget.MaxIterations != defaultRalphMaxIterations {
		t.Errorf("expected default %d iterations, got %d", defaultRalphMaxIterations, run.Budget.MaxIterations)
	}
}

func TestRalphSetupPreservesExplicitMaxIterations(t *testing.T) {
	h := NewRalph()
	run, err := h.Setup(context.Background(), pkgharness.Workspace{Path: t.TempDir()}, task.Task{}, pkgharness.Budget{MaxIterations: 3}, nil)
	if err != nil {
		t.Fatalf("Setup: %v", err)
	}
	if run.Budget.MaxIterations != 3 {
		t.Errorf("expected 3 iterations, got %d", run.Budget.MaxIterations)
	}
}

// progressingExecutor writes a unique file on each call so the workspace
// fingerprint changes between iterations. Used to verify the loop reaches
// MaxIterations.
type progressingExecutor struct {
	calls int
	dir   string
}

func (p *progressingExecutor) Name() string                                          { return "progressing" }
func (p *progressingExecutor) SupportedModes() []executor.ExecutionMode              { return []executor.ExecutionMode{executor.ExecutionModeCLI} }
func (p *progressingExecutor) ParseTranscript(raw []byte) ([]executor.ParsedTurn, error) { return nil, nil }
func (p *progressingExecutor) Execute(_ context.Context, cfg executor.RunConfig) (*executor.RunResult, error) {
	p.calls++
	if err := os.WriteFile(filepath.Join(p.dir, "out-"+cfg.Stage+".txt"), []byte("progress"), 0o644); err != nil {
		return nil, err
	}
	return &executor.RunResult{
		RawOutput:   "iter " + cfg.Stage,
		ParsedTurns: []executor.ParsedTurn{{Role: "assistant", Content: cfg.Stage}},
	}, nil
}

func TestRalphReachesMaxIterationsWhenProgressing(t *testing.T) {
	h := NewRalph()
	ws := pkgharness.Workspace{Path: t.TempDir()}
	rec := &progressingExecutor{dir: ws.Path}
	tk := task.Task{TaskPrompt: "fix it"}
	run, _ := h.Setup(context.Background(), ws, tk, pkgharness.Budget{MaxIterations: 3}, nil)

	result, err := h.Invoke(context.Background(), run, rec)
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	if rec.calls != 3 {
		t.Errorf("expected 3 calls, got %d", rec.calls)
	}
	for i := 0; i < 3; i++ {
		marker := "--- Iteration: " + strconv.Itoa(i) + " ---"
		if !strings.Contains(result.RawOutput, marker) {
			t.Errorf("merged transcript missing %q", marker)
		}
	}
}

// idleExecutor never modifies the workspace so the fingerprint never changes;
// Ralph should halt after the no-progress streak threshold.
type idleExecutor struct{ calls int }

func (e *idleExecutor) Name() string                                          { return "idle" }
func (e *idleExecutor) SupportedModes() []executor.ExecutionMode              { return []executor.ExecutionMode{executor.ExecutionModeCLI} }
func (e *idleExecutor) ParseTranscript(raw []byte) ([]executor.ParsedTurn, error) { return nil, nil }
func (e *idleExecutor) Execute(_ context.Context, cfg executor.RunConfig) (*executor.RunResult, error) {
	e.calls++
	return &executor.RunResult{RawOutput: "noop"}, nil
}

func TestRalphHaltsOnNoProgress(t *testing.T) {
	h := NewRalph()
	ws := pkgharness.Workspace{Path: t.TempDir()}
	rec := &idleExecutor{}
	tk := task.Task{TaskPrompt: "fix it"}
	// Use a high MaxIterations so the no-progress detector is what halts us.
	run, _ := h.Setup(context.Background(), ws, tk, pkgharness.Budget{MaxIterations: 10}, nil)

	_, err := h.Invoke(context.Background(), run, rec)
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	// Expect: iter 0 (no prevFingerprint baseline yet), iter 1 (1st no-progress),
	// iter 2 (streak hits threshold, halt). So 3 calls.
	if rec.calls < 2 || rec.calls > 4 {
		t.Errorf("expected halt around iteration 2-3, got %d calls", rec.calls)
	}
	if rec.calls >= 10 {
		t.Errorf("loop should have halted before MaxIterations; got %d calls", rec.calls)
	}
}

func TestRalphInvokeRespectsContextCancellation(t *testing.T) {
	h := NewRalph()
	ws := pkgharness.Workspace{Path: t.TempDir()}
	tk := task.Task{TaskPrompt: "x"}
	ctx, cancel := context.WithCancel(context.Background())
	calls := 0
	rec := &cancellingExecutor{cancel: cancel, callsBeforeCancel: 2, calls: &calls}

	run, _ := h.Setup(context.Background(), ws, tk, pkgharness.Budget{MaxIterations: 10}, nil)
	_, err := h.Invoke(ctx, run, rec)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
	if calls > 3 {
		t.Errorf("expected at most a few iterations before cancellation honored, got %d", calls)
	}
}

func TestRalphPromptShapesFollowUpIterations(t *testing.T) {
	tk := task.Task{TaskPrompt: "scaffold a CLI"}
	first := ralphPrompt(tk, 0, 5)
	later := ralphPrompt(tk, 2, 5)

	if first != "scaffold a CLI" {
		t.Errorf("first iteration should use raw prompt; got %q", first)
	}
	if !strings.Contains(later, "Iteration 3 of 5") {
		t.Errorf("follow-up iteration prompt should announce position; got %q", later)
	}
	if !strings.Contains(later, "scaffold a CLI") {
		t.Errorf("follow-up iteration should still carry task prompt; got %q", later)
	}
}

func TestRalphIterationStageTagging(t *testing.T) {
	h := NewRalph()
	ws := pkgharness.Workspace{Path: t.TempDir()}
	rec := &progressingExecutor{dir: ws.Path}
	tk := task.Task{TaskPrompt: "t"}
	run, _ := h.Setup(context.Background(), ws, tk, pkgharness.Budget{MaxIterations: 2}, nil)

	result, _ := h.Invoke(context.Background(), run, rec)
	if len(result.ParsedTurns) != 2 {
		t.Fatalf("expected 2 turns (one per iteration), got %d", len(result.ParsedTurns))
	}
	if result.ParsedTurns[0].Stage != "iteration-0" {
		t.Errorf("turn 0 Stage = %q", result.ParsedTurns[0].Stage)
	}
	if result.ParsedTurns[1].Stage != "iteration-1" {
		t.Errorf("turn 1 Stage = %q", result.ParsedTurns[1].Stage)
	}
}

func TestRegistryListsRalph(t *testing.T) {
	r := NewRegistry()
	if _, err := r.Get("ralph"); err != nil {
		t.Errorf("expected ralph in default registry: %v", err)
	}
}
