package harness

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mustafaselman/frameval/engine/internal/builtin/speckit"
	"github.com/mustafaselman/frameval/engine/pkg/executor"
	"github.com/mustafaselman/frameval/engine/pkg/harness"
	"github.com/mustafaselman/frameval/engine/pkg/task"
)

// speckitConstitutionFilename is the source file (in task harness_context/)
// that becomes .specify/memory/constitution.md inside the workspace.
const speckitConstitutionFilename = "constitution.md"

// speckitMemoryDir is the subdirectory spec-kit reads the constitution from.
const speckitMemoryDir = ".specify/memory"

// metadataKeyOwnsSpecify marks a HarnessRun whose Setup created the .specify/
// scaffold. Teardown only removes the directory when this flag is true.
const metadataKeyOwnsSpecify = "speckit.created_by_harness"

// metadataKeySpecKitExtension stashes the resolved catalog entry on the
// HarnessRun so Invoke can walk its stages without re-parsing cfg.
const metadataKeySpecKitExtension = "speckit.extension"

// ErrSpecKitExtensionMissing surfaces when cfg has no extension_id.
// The launcher's submit gate prevents this in normal flow; the sentinel
// catches direct API consumers.
var ErrSpecKitExtensionMissing = errors.New("speckit harness: cfg.speckit.extension_id is empty")

// ErrSpecKitExtensionNotFound surfaces when extension_id doesn't match
// any catalog entry. Usually a stale id from a deleted entry.
var ErrSpecKitExtensionNotFound = errors.New("speckit harness: extension_id does not match any catalog entry")

// SpecKit runs a spec-kit extension workflow driven by the catalog entry
// selected via cfg["speckit"]["extension_id"]. The canonical entry preserves
// today's 4-stage flow; other entries walk their own stage lists.
//
// Setup lays down `.specify/memory/constitution.md` from the task's
// harness_context bundle if present; if absent, the harness proceeds without
// a constitution (spec-kit treats it as optional).
type SpecKit struct{}

// NewSpecKit constructs the SpecKit harness. Stateless; safe to share.
func NewSpecKit() *SpecKit { return &SpecKit{} }

func (h *SpecKit) Name() string { return "speckit" }

func (h *SpecKit) Description() string {
	return "Canonical spec-kit workflow: /speckit.specify → /plan → /tasks → /implement (4 sequential agent calls)"
}

func (h *SpecKit) Setup(_ context.Context, ws harness.Workspace, t task.Task, b harness.Budget, cfg map[string]any) (harness.HarnessRun, error) {
	ext, err := extractSpecKitExtension(cfg)
	if err != nil {
		return harness.HarnessRun{}, err
	}

	memoryDir := filepath.Join(ws.Path, speckitMemoryDir)
	owned := false
	if _, err := os.Stat(filepath.Join(ws.Path, ".specify")); errors.Is(err, os.ErrNotExist) {
		if err := os.MkdirAll(memoryDir, 0o755); err != nil {
			return harness.HarnessRun{}, fmt.Errorf("speckit: mkdir .specify/memory: %w", err)
		}
		owned = true
	} else if err != nil {
		return harness.HarnessRun{}, fmt.Errorf("speckit: stat .specify: %w", err)
	}

	// Constitution is optional but strongly improves spec-kit's first stage.
	if t.TaskRootPath != "" {
		src := filepath.Join(t.TaskRootPath, "harness_context", speckitConstitutionFilename)
		if data, err := os.ReadFile(src); err == nil {
			dst := filepath.Join(memoryDir, "constitution.md")
			if err := os.WriteFile(dst, data, 0o644); err != nil {
				return harness.HarnessRun{}, fmt.Errorf("speckit: write constitution: %w", err)
			}
		} else if !errors.Is(err, os.ErrNotExist) {
			return harness.HarnessRun{}, fmt.Errorf("speckit: read constitution source: %w", err)
		}
	}

	return harness.HarnessRun{
		HarnessName: h.Name(),
		Task:        t,
		Workspace:   ws,
		Budget:      b,
		Metadata: map[string]any{
			metadataKeyOwnsSpecify:      owned,
			metadataKeySpecKitExtension: ext,
		},
	}, nil
}

// speckitStages names the four executable stages in execution order. Each
// stage's prompt embeds the task content via stagePrompt().
var speckitStages = []string{"specify", "plan", "tasks", "implement"}

func stagePrompt(stage string, t task.Task) string {
	switch stage {
	case "specify":
		return "/speckit.specify\n\n" + t.TaskPrompt
	case "plan":
		details := strings.TrimSpace(t.TechnicalDetail)
		if details == "" {
			details = "(no extra technical details supplied; infer from specify output)"
		}
		return "/speckit.plan\n\n" + details
	case "tasks":
		return "/speckit.tasks"
	case "implement":
		return "/speckit.implement"
	default:
		return "/speckit." + stage
	}
}

func (h *SpecKit) Invoke(ctx context.Context, run harness.HarnessRun, exec executor.AgentExecutor) (*executor.RunResult, error) {
	var stageResults []*executor.RunResult
	var stageErr error
stages:
	for _, stage := range speckitStages {
		// `break stages` (labeled) exits the for loop; a plain `break` inside
		// the select would only break the select.
		select {
		case <-ctx.Done():
			stageErr = ctx.Err()
			break stages
		default:
		}
		result, err := exec.Execute(ctx, harness.MergeConfig(run.BaseRunConfig, executor.RunConfig{
			Prompt:        stagePrompt(stage, run.Task),
			WorkspacePath: run.Workspace.Path,
			Stage:         stage,
		}))
		if result == nil {
			result = &executor.RunResult{}
		}
		stageResults = append(stageResults, result)
		if err != nil {
			stageErr = fmt.Errorf("speckit: stage %s: %w", stage, err)
			break stages
		}
	}
	return mergeStageTranscripts(speckitStages, stageResults), stageErr
}

// mergeStageTranscripts concatenates per-stage RunResults into a single
// transcript with `--- Stage: <name> ---` markers tagged on every turn.
//
// Stages that failed before producing turns still get a marker line so the
// downstream fingerprint extractor can see the gap.
func mergeStageTranscripts(stages []string, results []*executor.RunResult) *executor.RunResult {
	merged := &executor.RunResult{}
	var rawBuilder strings.Builder
	for i, result := range results {
		if i >= len(stages) {
			break
		}
		stage := stages[i]
		marker := "--- Stage: " + stage + " ---"
		rawBuilder.WriteString(marker)
		rawBuilder.WriteString("\n")
		if result != nil {
			rawBuilder.WriteString(result.RawOutput)
			if !strings.HasSuffix(result.RawOutput, "\n") {
				rawBuilder.WriteString("\n")
			}
			for _, turn := range result.ParsedTurns {
				turn.Stage = stage
				merged.ParsedTurns = append(merged.ParsedTurns, turn)
			}
		}
	}
	merged.RawOutput = rawBuilder.String()
	return merged
}

// Teardown removes the harness-created .specify scaffold. If the workspace
// already had a .specify/ directory before Setup (brownfield case), we leave
// it alone — Setup's `owned` flag in HarnessRun.Metadata tracks ownership.
func (h *SpecKit) Teardown(_ context.Context, run harness.HarnessRun) error {
	if run.Workspace.Path == "" {
		return nil
	}
	owned, _ := run.Metadata[metadataKeyOwnsSpecify].(bool)
	if !owned {
		return nil
	}
	target := filepath.Join(run.Workspace.Path, ".specify")
	if err := os.RemoveAll(target); err != nil {
		return fmt.Errorf("speckit: teardown remove %s: %w", target, err)
	}
	return nil
}

// extractSpecKitExtension reads and validates cfg["speckit"]["extension_id"],
// looks up the catalog, and returns the resolved extension.
func extractSpecKitExtension(cfg map[string]any) (speckit.SpecKitExtension, error) {
	if cfg == nil {
		return speckit.SpecKitExtension{}, ErrSpecKitExtensionMissing
	}
	sub, ok := cfg["speckit"].(map[string]any)
	if !ok {
		return speckit.SpecKitExtension{}, ErrSpecKitExtensionMissing
	}
	id, _ := sub["extension_id"].(string)
	if id == "" {
		return speckit.SpecKitExtension{}, ErrSpecKitExtensionMissing
	}
	ext, ok := speckit.Lookup(id)
	if !ok {
		return speckit.SpecKitExtension{}, fmt.Errorf("%w: %q", ErrSpecKitExtensionNotFound, id)
	}
	return ext, nil
}
