package harness

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mustafaselman/frameval/engine/pkg/executor"
	"github.com/mustafaselman/frameval/engine/pkg/harness"
	"github.com/mustafaselman/frameval/engine/pkg/task"
)

const (
	// AgentInstructionsHarnessID is the stable wire id for this harness.
	AgentInstructionsHarnessID = "agent_instructions"

	// agentInstructionsConfigKey is the top-level key the harness reads
	// from the per-variant cfg map.
	agentInstructionsConfigKey = "agent_instructions"

	// agentInstructionsTargetFile is the filename written into the
	// workspace. Stays CLAUDE.md so existing agents that read it by name
	// keep working.
	agentInstructionsTargetFile = "CLAUDE.md"

	metadataKeyOwnsTarget = "agent_instructions.created_by_harness"
)

// ErrAgentInstructionsContentMissing surfaces when the launcher's
// per-variant cfg has no usable text. The launcher's submit gate
// prevents this in normal flow; the sentinel is the last line of
// defense for direct API consumers.
var ErrAgentInstructionsContentMissing = errors.New(
	"agent_instructions harness: cfg.agent_instructions.content is empty; user must supply text in the launcher")

// AgentInstructions lays the user-typed CLAUDE.md into the workspace
// from the per-variant config blob. Replaces the older `claudemd`
// harness that read from task-bundled files.
type AgentInstructions struct{}

func NewAgentInstructions() *AgentInstructions { return &AgentInstructions{} }

func (h *AgentInstructions) Name() string { return AgentInstructionsHarnessID }

func (h *AgentInstructions) Description() string {
	return "Lay down user-supplied CLAUDE.md (typed in the launcher) into the workspace before the agent runs"
}

func (h *AgentInstructions) Setup(_ context.Context, ws harness.Workspace, t task.Task, b harness.Budget, cfg map[string]any) (harness.HarnessRun, error) {
	content, ok := extractAgentInstructionsContent(cfg)
	if !ok || strings.TrimSpace(content) == "" {
		return harness.HarnessRun{}, ErrAgentInstructionsContentMissing
	}
	dst := filepath.Join(ws.Path, agentInstructionsTargetFile)
	if _, statErr := os.Stat(dst); statErr == nil {
		return harness.HarnessRun{}, fmt.Errorf("agent_instructions harness: workspace already contains %s; refusing to overwrite", agentInstructionsTargetFile)
	} else if !errors.Is(statErr, os.ErrNotExist) {
		return harness.HarnessRun{}, fmt.Errorf("agent_instructions harness: stat dst: %w", statErr)
	}
	if err := os.WriteFile(dst, []byte(content), 0o644); err != nil {
		return harness.HarnessRun{}, fmt.Errorf("agent_instructions harness: write %s: %w", agentInstructionsTargetFile, err)
	}
	return harness.HarnessRun{
		HarnessName: h.Name(),
		Task:        t,
		Workspace:   ws,
		Budget:      b,
		Metadata:    map[string]any{metadataKeyOwnsTarget: true},
	}, nil
}

func (h *AgentInstructions) Invoke(ctx context.Context, run harness.HarnessRun, exec executor.AgentExecutor) (*executor.RunResult, error) {
	return exec.Execute(ctx, harness.MergeConfig(run.BaseRunConfig, executor.RunConfig{
		Prompt:        run.Task.TaskPrompt,
		WorkspacePath: run.Workspace.Path,
	}))
}

func (h *AgentInstructions) Teardown(_ context.Context, run harness.HarnessRun) error {
	if run.Workspace.Path == "" {
		return nil
	}
	owned, _ := run.Metadata[metadataKeyOwnsTarget].(bool)
	if !owned {
		return nil
	}
	target := filepath.Join(run.Workspace.Path, agentInstructionsTargetFile)
	if err := os.Remove(target); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("agent_instructions harness: teardown remove %s: %w", target, err)
	}
	return nil
}

// extractAgentInstructionsContent safely walks cfg["agent_instructions"]["content"].
// Returns ("", false) on any structural mismatch; never panics.
func extractAgentInstructionsContent(cfg map[string]any) (string, bool) {
	if cfg == nil {
		return "", false
	}
	sub, ok := cfg[agentInstructionsConfigKey].(map[string]any)
	if !ok {
		return "", false
	}
	content, ok := sub["content"].(string)
	if !ok {
		return "", false
	}
	return content, true
}
