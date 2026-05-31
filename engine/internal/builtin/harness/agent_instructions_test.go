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
