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

func TestMultiAgentIdentity(t *testing.T) {
	h := NewMultiAgent()
	if h.Name() != "multiagent" {
		t.Errorf("Name = %q", h.Name())
	}
	if h.Description() == "" {
		t.Error("Description should not be empty")
	}
}

func TestRegistryListsMultiAgent(t *testing.T) {
	r := NewRegistry()
	if _, err := r.Get("multiagent"); err != nil {
		t.Errorf("expected multiagent in default registry: %v", err)
	}
}

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

// multiagentRecordingExec lets a test assert what prompts the harness sent
// to each role and what the merged transcript looks like.
type multiagentRecordingExec struct {
	calls   []executor.RunConfig
	results map[string]*executor.RunResult // keyed by role
	errs    map[string]error
}

func (e *multiagentRecordingExec) Name() string                     { return "recording" }
func (e *multiagentRecordingExec) SupportedModes() []executor.ExecutionMode {
	return []executor.ExecutionMode{executor.ExecutionModeCLI}
}
func (e *multiagentRecordingExec) ParseTranscript(raw []byte) ([]executor.ParsedTurn, error) {
	return nil, nil
}
func (e *multiagentRecordingExec) Execute(_ context.Context, cfg executor.RunConfig) (*executor.RunResult, error) {
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
	exec := &multiagentRecordingExec{
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
	exec := &multiagentRecordingExec{
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
	exec := &multiagentRecordingExec{
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
	exec := &multiagentRecordingExec{
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
