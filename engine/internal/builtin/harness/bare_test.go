package harness

import (
	"context"
	"errors"
	"testing"

	"github.com/mustafaselman/frameval/engine/pkg/executor"
	pkgharness "github.com/mustafaselman/frameval/engine/pkg/harness"
	"github.com/mustafaselman/frameval/engine/pkg/task"
)

// fakeExecutor records the RunConfig it received and returns a canned result.
type fakeExecutor struct {
	received    executor.RunConfig
	returnError error
	returnTurns []executor.ParsedTurn
}

func (f *fakeExecutor) Name() string                                 { return "fake" }
func (f *fakeExecutor) SupportedModes() []executor.ExecutionMode     { return []executor.ExecutionMode{executor.ExecutionModeCLI} }
func (f *fakeExecutor) ParseTranscript(raw []byte) ([]executor.ParsedTurn, error) {
	return nil, nil
}
func (f *fakeExecutor) Execute(_ context.Context, cfg executor.RunConfig) (*executor.RunResult, error) {
	f.received = cfg
	return &executor.RunResult{RawOutput: "fake", ParsedTurns: f.returnTurns}, f.returnError
}

func TestBareIdentity(t *testing.T) {
	b := NewBare()
	if b.Name() != "bare" {
		t.Errorf("Name() = %q, want %q", b.Name(), "bare")
	}
	if b.Description() == "" {
		t.Error("Description should not be empty")
	}
}

func TestBareSetupDoesNotMutateWorkspace(t *testing.T) {
	b := NewBare()
	ws := pkgharness.Workspace{Path: "/tmp/ws", TestsDir: "/tmp/tests"}
	tk := task.Task{ID: "t-1", TaskPrompt: "build a CLI"}
	run, err := b.Setup(context.Background(), ws, tk, pkgharness.Budget{MaxIterations: 1})
	if err != nil {
		t.Fatalf("Setup: %v", err)
	}
	if run.HarnessName != "bare" {
		t.Errorf("HarnessName = %q, want bare", run.HarnessName)
	}
	if run.Task.ID != "t-1" {
		t.Errorf("Task not carried: got %q", run.Task.ID)
	}
	if run.Workspace.Path != "/tmp/ws" {
		t.Errorf("Workspace.Path not carried: %+v", run.Workspace)
	}
	if run.Workspace.TestsDir != "/tmp/tests" {
		t.Errorf("Workspace.TestsDir not carried: %+v", run.Workspace)
	}
}

func TestBareInvokeForwardsPrompt(t *testing.T) {
	b := NewBare()
	fake := &fakeExecutor{returnTurns: []executor.ParsedTurn{{Role: "assistant", Content: "ok"}}}
	tk := task.Task{ID: "t-1", TaskPrompt: "do the thing"}
	ws := pkgharness.Workspace{Path: "/sandbox/ws"}
	run, _ := b.Setup(context.Background(), ws, tk, pkgharness.Budget{})

	result, err := b.Invoke(context.Background(), run, fake)
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if fake.received.Prompt != "do the thing" {
		t.Errorf("Prompt = %q, want %q", fake.received.Prompt, "do the thing")
	}
	if fake.received.WorkspacePath != "/sandbox/ws" {
		t.Errorf("WorkspacePath = %q, want %q", fake.received.WorkspacePath, "/sandbox/ws")
	}
}

func TestBareInvokeSurfacesExecutorError(t *testing.T) {
	b := NewBare()
	wantErr := errors.New("boom")
	fake := &fakeExecutor{returnError: wantErr}
	tk := task.Task{TaskPrompt: "x"}
	run, _ := b.Setup(context.Background(), pkgharness.Workspace{}, tk, pkgharness.Budget{})

	_, err := b.Invoke(context.Background(), run, fake)
	if !errors.Is(err, wantErr) {
		t.Errorf("expected wrapped boom, got %v", err)
	}
}

func TestBareTeardownIsNoop(t *testing.T) {
	b := NewBare()
	run, _ := b.Setup(context.Background(), pkgharness.Workspace{}, task.Task{}, pkgharness.Budget{})
	if err := b.Teardown(context.Background(), run); err != nil {
		t.Errorf("Teardown should be a no-op, got error %v", err)
	}
}

func TestRegistryListsBareByDefault(t *testing.T) {
	r := NewRegistry()
	names := r.List()
	found := false
	for _, n := range names {
		if n == "bare" {
			found = true
		}
	}
	if !found {
		t.Errorf("bare should be in default registry; got %v", names)
	}
}

func TestRegistryGetAndDuplicate(t *testing.T) {
	r := NewRegistry()
	if _, err := r.Get("bare"); err != nil {
		t.Errorf("Get(bare): %v", err)
	}
	if _, err := r.Get("nonexistent"); err == nil {
		t.Error("expected error for unknown harness")
	}
	if err := r.Register(NewBare()); err == nil {
		t.Error("expected error registering duplicate")
	}
}
