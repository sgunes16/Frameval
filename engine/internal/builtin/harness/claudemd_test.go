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

// setupClaudeMdFixture creates a temp task root with a harness_context/claudemd.md
// payload and a separate workspace directory. Returns task, workspace, and the
// expected CLAUDE.md content.
func setupClaudeMdFixture(t *testing.T, content string) (task.Task, pkgharness.Workspace, string) {
	t.Helper()
	root := t.TempDir()
	ws := t.TempDir()
	hcDir := filepath.Join(root, "harness_context")
	if err := os.MkdirAll(hcDir, 0o755); err != nil {
		t.Fatalf("mkdir harness_context: %v", err)
	}
	src := filepath.Join(hcDir, "claudemd.md")
	if err := os.WriteFile(src, []byte(content), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}
	return task.Task{ID: "fixture", TaskRootPath: root, TaskPrompt: "do x"},
		pkgharness.Workspace{Path: ws}, content
}

func TestClaudeMdIdentity(t *testing.T) {
	h := NewClaudeMd()
	if h.Name() != "claudemd" {
		t.Errorf("Name = %q", h.Name())
	}
	if h.Description() == "" {
		t.Error("Description should not be empty")
	}
}

func TestClaudeMdSetupCopiesSourceToWorkspace(t *testing.T) {
	h := NewClaudeMd()
	tk, ws, want := setupClaudeMdFixture(t, "## CLAUDE.md\nrule: be careful")

	run, err := h.Setup(context.Background(), ws, tk, pkgharness.Budget{})
	if err != nil {
		t.Fatalf("Setup: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(ws.Path, "CLAUDE.md"))
	if err != nil {
		t.Fatalf("read target: %v", err)
	}
	if string(got) != want {
		t.Errorf("content mismatch: got %q want %q", string(got), want)
	}
	if owned, _ := run.Metadata[metadataKeyOwnsClaudemd].(bool); !owned {
		t.Error("metadata flag should mark file as owned by harness")
	}
}

func TestClaudeMdSetupReturnsErrorWhenSourceMissing(t *testing.T) {
	h := NewClaudeMd()
	tk := task.Task{ID: "fixture", TaskRootPath: t.TempDir()}
	ws := pkgharness.Workspace{Path: t.TempDir()}

	_, err := h.Setup(context.Background(), ws, tk, pkgharness.Budget{})
	if !errors.Is(err, ErrClaudemdSourceMissing) {
		t.Fatalf("expected ErrClaudemdSourceMissing, got %v", err)
	}
}

func TestClaudeMdSetupRefusesToStompExistingFile(t *testing.T) {
	h := NewClaudeMd()
	tk, ws, _ := setupClaudeMdFixture(t, "harness content")
	preExisting := []byte("brownfield codebase already ships CLAUDE.md")
	if err := os.WriteFile(filepath.Join(ws.Path, "CLAUDE.md"), preExisting, 0o644); err != nil {
		t.Fatalf("seed existing CLAUDE.md: %v", err)
	}

	_, err := h.Setup(context.Background(), ws, tk, pkgharness.Budget{})
	if !errors.Is(err, ErrClaudemdWouldStompExisting) {
		t.Fatalf("expected ErrClaudemdWouldStompExisting, got %v", err)
	}

	got, _ := os.ReadFile(filepath.Join(ws.Path, "CLAUDE.md"))
	if string(got) != string(preExisting) {
		t.Errorf("pre-existing file got overwritten: %q", string(got))
	}
}

func TestClaudeMdInvokeForwardsPrompt(t *testing.T) {
	h := NewClaudeMd()
	tk, ws, _ := setupClaudeMdFixture(t, "rules")
	fake := &fakeExecutor{}

	run, err := h.Setup(context.Background(), ws, tk, pkgharness.Budget{})
	if err != nil {
		t.Fatalf("Setup: %v", err)
	}
	if _, err := h.Invoke(context.Background(), run, fake); err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	if fake.received.Prompt != "do x" {
		t.Errorf("Prompt forwarded = %q, want %q", fake.received.Prompt, "do x")
	}
	if fake.received.WorkspacePath != ws.Path {
		t.Errorf("WorkspacePath forwarded = %q, want %q", fake.received.WorkspacePath, ws.Path)
	}
}

func TestClaudeMdTeardownRemovesOwnedFile(t *testing.T) {
	h := NewClaudeMd()
	tk, ws, _ := setupClaudeMdFixture(t, "x")
	run, _ := h.Setup(context.Background(), ws, tk, pkgharness.Budget{})

	if err := h.Teardown(context.Background(), run); err != nil {
		t.Fatalf("Teardown: %v", err)
	}
	if _, err := os.Stat(filepath.Join(ws.Path, "CLAUDE.md")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("CLAUDE.md should be removed, got stat err %v", err)
	}
}

func TestClaudeMdTeardownLeavesUnownedFile(t *testing.T) {
	h := NewClaudeMd()
	ws := pkgharness.Workspace{Path: t.TempDir()}
	preExisting := []byte("brownfield")
	if err := os.WriteFile(filepath.Join(ws.Path, "CLAUDE.md"), preExisting, 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	run := pkgharness.HarnessRun{HarnessName: "claudemd", Workspace: ws, Metadata: map[string]any{metadataKeyOwnsClaudemd: false}}

	if err := h.Teardown(context.Background(), run); err != nil {
		t.Fatalf("Teardown: %v", err)
	}
	got, _ := os.ReadFile(filepath.Join(ws.Path, "CLAUDE.md"))
	if string(got) != string(preExisting) {
		t.Errorf("Teardown removed unowned file; got %q", string(got))
	}
}

func TestRegistryListsClaudeMd(t *testing.T) {
	r := NewRegistry()
	if _, err := r.Get("claudemd"); err != nil {
		t.Errorf("expected claudemd in default registry: %v", err)
	}
}
