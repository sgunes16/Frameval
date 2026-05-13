package harness

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/mustafaselman/frameval/engine/pkg/executor"
	pkgharness "github.com/mustafaselman/frameval/engine/pkg/harness"
	"github.com/mustafaselman/frameval/engine/pkg/task"
)

// twoStageExecutor returns a different transcript per Role so tests can
// assert plan extraction and prompt routing.
type twoStageExecutor struct {
	calls         []executor.RunConfig
	plannerOutput string
	coderOutput   string
	plannerErr    error
	coderErr      error
}

func (e *twoStageExecutor) Name() string                                          { return "two-stage" }
func (e *twoStageExecutor) SupportedModes() []executor.ExecutionMode              { return []executor.ExecutionMode{executor.ExecutionModeCLI} }
func (e *twoStageExecutor) ParseTranscript(raw []byte) ([]executor.ParsedTurn, error) { return nil, nil }
func (e *twoStageExecutor) Execute(_ context.Context, cfg executor.RunConfig) (*executor.RunResult, error) {
	e.calls = append(e.calls, cfg)
	if cfg.Role == rolePlanner {
		return &executor.RunResult{
			RawOutput:   "raw planner",
			ParsedTurns: []executor.ParsedTurn{{Role: "assistant", Content: e.plannerOutput}},
		}, e.plannerErr
	}
	return &executor.RunResult{
		RawOutput:   "raw coder",
		ParsedTurns: []executor.ParsedTurn{{Role: "assistant", Content: e.coderOutput}},
	}, e.coderErr
}

func TestPlannerCoderIdentity(t *testing.T) {
	h := NewPlannerCoder()
	if h.Name() != "planner_coder" {
		t.Errorf("Name = %q", h.Name())
	}
	if h.Description() == "" {
		t.Error("Description should not be empty")
	}
}

func TestPlannerCoderInvokeIssuesTwoCallsInOrder(t *testing.T) {
	h := NewPlannerCoder()
	ws := pkgharness.Workspace{Path: t.TempDir()}
	tk := task.Task{TaskPrompt: "scaffold a CLI"}
	exec := &twoStageExecutor{
		plannerOutput: "## Approach\nuse click\n## Files to change\n- main.py",
		coderOutput:   "done",
	}
	run, _ := h.Setup(context.Background(), ws, tk, pkgharness.Budget{})

	result, err := h.Invoke(context.Background(), run, exec)
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	if len(exec.calls) != 2 {
		t.Fatalf("expected 2 calls, got %d", len(exec.calls))
	}
	if exec.calls[0].Role != rolePlanner {
		t.Errorf("first call Role = %q, want planner", exec.calls[0].Role)
	}
	if exec.calls[1].Role != roleCoder {
		t.Errorf("second call Role = %q, want coder", exec.calls[1].Role)
	}
	// Planner prompt should mention PLANNER + 3 markdown sections.
	if !strings.Contains(exec.calls[0].Prompt, "PLANNER") {
		t.Errorf("planner prompt missing role label: %q", exec.calls[0].Prompt)
	}
	if !strings.Contains(exec.calls[0].Prompt, "## Approach") {
		t.Errorf("planner prompt missing Approach section: %q", exec.calls[0].Prompt)
	}
	// Coder prompt should inline the planner's output.
	if !strings.Contains(exec.calls[1].Prompt, "use click") {
		t.Errorf("coder prompt does not inline planner plan: %q", exec.calls[1].Prompt)
	}
	if !strings.Contains(exec.calls[1].Prompt, "scaffold a CLI") {
		t.Errorf("coder prompt missing original task: %q", exec.calls[1].Prompt)
	}
	// Merged transcript carries both role markers.
	for _, marker := range []string{"--- Role: planner ---", "--- Role: coder ---"} {
		if !strings.Contains(result.RawOutput, marker) {
			t.Errorf("merged transcript missing %q", marker)
		}
	}
	if len(result.ParsedTurns) != 2 {
		t.Fatalf("expected 2 turns, got %d", len(result.ParsedTurns))
	}
	if result.ParsedTurns[0].Role != "planner" || result.ParsedTurns[1].Role != "coder" {
		t.Errorf("turn roles = %q, %q; want planner, coder",
			result.ParsedTurns[0].Role, result.ParsedTurns[1].Role)
	}
}

func TestPlannerCoderPlannerErrorStillRunsCoder(t *testing.T) {
	h := NewPlannerCoder()
	ws := pkgharness.Workspace{Path: t.TempDir()}
	tk := task.Task{TaskPrompt: "x"}
	wantErr := errors.New("planner crashed")
	exec := &twoStageExecutor{plannerErr: wantErr, coderOutput: "fallback completed"}
	run, _ := h.Setup(context.Background(), ws, tk, pkgharness.Budget{})

	_, err := h.Invoke(context.Background(), run, exec)
	if err == nil {
		t.Fatal("expected planner error to surface")
	}
	if !errors.Is(err, wantErr) {
		t.Errorf("planner error not wrapped: %v", err)
	}
	if len(exec.calls) != 2 {
		t.Errorf("coder should still be invoked even on planner failure; got %d calls", len(exec.calls))
	}
	if !strings.Contains(exec.calls[1].Prompt, "planner stage failed") {
		t.Errorf("coder should see fallback plan note; got %q", exec.calls[1].Prompt)
	}
}

func TestPlannerCoderContextCancellationAbortsBeforeCoder(t *testing.T) {
	h := NewPlannerCoder()
	ws := pkgharness.Workspace{Path: t.TempDir()}
	tk := task.Task{TaskPrompt: "x"}
	exec := &twoStageExecutor{plannerErr: context.Canceled}
	run, _ := h.Setup(context.Background(), ws, tk, pkgharness.Budget{})

	_, err := h.Invoke(context.Background(), run, exec)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
	if len(exec.calls) != 1 {
		t.Errorf("expected only planner call on ctx cancel; got %d", len(exec.calls))
	}
}

// Parallel to the Canceled test: DeadlineExceeded also aborts before coder.
func TestPlannerCoderDeadlineExceededAbortsBeforeCoder(t *testing.T) {
	h := NewPlannerCoder()
	ws := pkgharness.Workspace{Path: t.TempDir()}
	tk := task.Task{TaskPrompt: "x"}
	exec := &twoStageExecutor{plannerErr: context.DeadlineExceeded}
	run, _ := h.Setup(context.Background(), ws, tk, pkgharness.Budget{})

	_, err := h.Invoke(context.Background(), run, exec)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected context.DeadlineExceeded, got %v", err)
	}
	if len(exec.calls) != 1 {
		t.Errorf("expected only planner call on deadline; got %d", len(exec.calls))
	}
}

// When BOTH planner and coder fail, both errors must be unwrappable so
// downstream errors.Is checks can detect either sentinel.
func TestPlannerCoderBothErrorsAreWrapped(t *testing.T) {
	h := NewPlannerCoder()
	ws := pkgharness.Workspace{Path: t.TempDir()}
	tk := task.Task{TaskPrompt: "x"}
	plannerSentinel := errors.New("planner blew up")
	coderSentinel := errors.New("coder blew up")
	exec := &twoStageExecutor{plannerErr: plannerSentinel, coderErr: coderSentinel}
	run, _ := h.Setup(context.Background(), ws, tk, pkgharness.Budget{})

	_, err := h.Invoke(context.Background(), run, exec)
	if err == nil {
		t.Fatal("expected error from both stages")
	}
	if !errors.Is(err, plannerSentinel) {
		t.Errorf("planner sentinel not unwrappable: %v", err)
	}
	if !errors.Is(err, coderSentinel) {
		t.Errorf("coder sentinel not unwrappable: %v", err)
	}
}

func TestExtractPlanFromResultPrefersAssistantTurn(t *testing.T) {
	r := &executor.RunResult{
		RawOutput: "fallback",
		ParsedTurns: []executor.ParsedTurn{
			{Role: "user", Content: "ignore"},
			{Role: "assistant", Content: "the plan"},
		},
	}
	if got := extractPlanFromResult(r); got != "the plan" {
		t.Errorf("got %q", got)
	}
}

func TestExtractPlanFallsBackToRawOutput(t *testing.T) {
	r := &executor.RunResult{RawOutput: "raw fallback"}
	if got := extractPlanFromResult(r); got != "raw fallback" {
		t.Errorf("got %q", got)
	}
}

func TestExtractPlanFromEmpty(t *testing.T) {
	if got := extractPlanFromResult(nil); !strings.Contains(got, "no output") {
		t.Errorf("nil → %q", got)
	}
	if got := extractPlanFromResult(&executor.RunResult{}); !strings.Contains(got, "no output") {
		t.Errorf("empty → %q", got)
	}
}

func TestRegistryListsPlannerCoder(t *testing.T) {
	r := NewRegistry()
	if _, err := r.Get("planner_coder"); err != nil {
		t.Errorf("expected planner_coder in default registry: %v", err)
	}
}
