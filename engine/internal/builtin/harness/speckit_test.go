package harness

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mustafaselman/frameval/engine/internal/builtin/speckit"
	"github.com/mustafaselman/frameval/engine/pkg/executor"
	pkgharness "github.com/mustafaselman/frameval/engine/pkg/harness"
	"github.com/mustafaselman/frameval/engine/pkg/task"
)

// canonicalCfg is a valid speckit cfg that selects the canonical extension.
// Used by pre-existing setup/invoke tests that don't care which extension is
// chosen — they just need Setup to succeed.
var canonicalCfg = map[string]any{"speckit": map[string]any{"extension_id": "canonical"}}

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

	run, err := h.Setup(context.Background(), ws, tk, pkgharness.Budget{}, canonicalCfg)
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

	_, err := h.Setup(context.Background(), ws, tk, pkgharness.Budget{}, canonicalCfg)
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

	if _, err := h.Setup(context.Background(), ws, tk, pkgharness.Budget{}, canonicalCfg); err != nil {
		t.Fatalf("Setup should succeed without constitution: %v", err)
	}
}

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
	run, _ := h.Setup(context.Background(), ws, task.Task{}, pkgharness.Budget{}, canonicalCfg)

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
	run, err := h.Setup(context.Background(), ws, task.Task{}, pkgharness.Budget{}, canonicalCfg)
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
