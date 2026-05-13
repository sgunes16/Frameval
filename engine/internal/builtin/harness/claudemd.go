package harness

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mustafaselman/frameval/engine/pkg/executor"
	"github.com/mustafaselman/frameval/engine/pkg/harness"
	"github.com/mustafaselman/frameval/engine/pkg/task"
)

// Source filename inside the task's harness_context directory that becomes
// the workspace's CLAUDE.md.
const claudeMdSourceFilename = "claudemd.md"

// claudemdTargetFilename is the canonical instruction-file path the agent
// reads in the workspace.
const claudemdTargetFilename = "CLAUDE.md"

// metadataKeyOwnsClaudemd marks a HarnessRun whose Setup created the
// target file. Teardown only removes the file when this flag is true,
// so a brownfield workspace that already had a CLAUDE.md is preserved
// (and Setup actually errors out in that case, see below).
const metadataKeyOwnsClaudemd = "claudemd.created_by_harness"

// ErrClaudemdSourceMissing surfaces when the task's harness_context bundle
// does not include claudemd.md. Returning this from Setup prevents silent
// degradation to a bare run.
var ErrClaudemdSourceMissing = errors.New("claudemd harness: task harness_context/claudemd.md not found")

// ErrClaudemdWouldStompExisting surfaces when the workspace already carries a
// CLAUDE.md (typical for brownfield tasks whose codebase ships one). The
// harness refuses to overwrite; operators can move the conflicting file aside
// or pick a different harness.
var ErrClaudemdWouldStompExisting = errors.New("claudemd harness: workspace already contains CLAUDE.md; refusing to overwrite")

// ClaudeMd is the wording-isolation harness: it lays down a CLAUDE.md file
// in the workspace from the task's harness_context bundle, then runs the
// agent with the raw task prompt. Difference vs `bare` is exactly the
// instruction file, making it the natural baseline for measuring the
// effect of CLAUDE.md content on agent behavior.
type ClaudeMd struct{}

// NewClaudeMd constructs the ClaudeMd harness. Stateless; safe to share.
func NewClaudeMd() *ClaudeMd { return &ClaudeMd{} }

func (h *ClaudeMd) Name() string { return "claudemd" }

func (h *ClaudeMd) Description() string {
	return "Lay down task harness_context/claudemd.md into the workspace, then run agent with task prompt"
}

func (h *ClaudeMd) Setup(_ context.Context, ws harness.Workspace, t task.Task, b harness.Budget) (harness.HarnessRun, error) {
	if t.TaskRootPath == "" {
		return harness.HarnessRun{}, fmt.Errorf("%w: task has no TaskRootPath", ErrClaudemdSourceMissing)
	}
	src := filepath.Join(t.TaskRootPath, "harness_context", claudeMdSourceFilename)
	dst := filepath.Join(ws.Path, claudemdTargetFilename)

	content, err := os.ReadFile(src)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return harness.HarnessRun{}, fmt.Errorf("%w (looked for %s)", ErrClaudemdSourceMissing, src)
		}
		return harness.HarnessRun{}, fmt.Errorf("claudemd harness: read source: %w", err)
	}

	// Brownfield safety: do not stomp on an existing CLAUDE.md.
	if _, statErr := os.Stat(dst); statErr == nil {
		return harness.HarnessRun{}, fmt.Errorf("%w (at %s)", ErrClaudemdWouldStompExisting, dst)
	} else if !errors.Is(statErr, os.ErrNotExist) {
		return harness.HarnessRun{}, fmt.Errorf("claudemd harness: stat destination: %w", statErr)
	}

	if err := os.WriteFile(dst, content, 0o644); err != nil {
		return harness.HarnessRun{}, fmt.Errorf("claudemd harness: write target: %w", err)
	}

	return harness.HarnessRun{
		HarnessName: h.Name(),
		Task:        t,
		Workspace:   ws,
		Budget:      b,
		Metadata:    map[string]any{metadataKeyOwnsClaudemd: true},
	}, nil
}

func (h *ClaudeMd) Invoke(ctx context.Context, run harness.HarnessRun, exec executor.AgentExecutor) (*executor.RunResult, error) {
	return exec.Execute(ctx, executor.RunConfig{
		Prompt:        run.Task.TaskPrompt,
		WorkspacePath: run.Workspace.Path,
	})
}

// Teardown removes the laid-down CLAUDE.md if and only if Setup created it.
// Pre-existing CLAUDE.md files (which Setup would have refused) are not
// possible to reach here, but the metadata flag guards against future
// refactors that might change Setup behavior.
func (h *ClaudeMd) Teardown(_ context.Context, run harness.HarnessRun) error {
	if run.Workspace.Path == "" {
		return nil
	}
	owned, _ := run.Metadata[metadataKeyOwnsClaudemd].(bool)
	if !owned {
		return nil
	}
	target := filepath.Join(run.Workspace.Path, claudemdTargetFilename)
	if err := os.Remove(target); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("claudemd harness: teardown remove %s: %w", target, err)
	}
	return nil
}
