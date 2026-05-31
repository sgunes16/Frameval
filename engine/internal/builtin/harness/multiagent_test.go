package harness

import (
	"context"
	"errors"
	"testing"

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
