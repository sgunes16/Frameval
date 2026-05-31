package harness

import (
	"context"
	"time"

	"github.com/mustafaselman/frameval/engine/pkg/executor"
	"github.com/mustafaselman/frameval/engine/pkg/task"
)

// Workspace represents the filesystem environment the agent sees inside the
// sandbox. Tests live outside Workspace.Path and are mounted read-only at
// Workspace.TestsDir — the agent's working directory excludes the tests dir.
type Workspace struct {
	// Path is the writable workspace root the agent edits.
	Path string

	// TestsDir is mounted read-only and inaccessible from the agent's container view.
	// Used by eval.sh (run by the orchestrator, not the agent) to assess outputs.
	TestsDir string

	// GitRef is set for brownfield-git workspaces; useful for scope-discipline tests
	// that diff against an initial commit.
	GitRef string
}

// Budget bounds how long, how much, and how many iterations a harness may consume.
//
// Different harnesses use different fields:
//   - Ralph uses MaxIterations + StopOnSuccess (test-pass check between iterations)
//   - SpecKit uses MaxWallSeconds across the 4-stage pipeline
//   - PlannerCoder uses MaxWallSeconds for both planner + coder calls combined
type Budget struct {
	MaxIterations  int
	MaxTokens      int
	MaxWallSeconds int
	StopOnSuccess  bool
}

// WallTimeout returns Budget.MaxWallSeconds as a time.Duration, or 10 minutes
// if unset.
func (b Budget) WallTimeout() time.Duration {
	if b.MaxWallSeconds <= 0 {
		return 10 * time.Minute
	}
	return time.Duration(b.MaxWallSeconds) * time.Second
}

// HarnessRun is the per-run handle a Harness creates during Setup and threads
// through Invoke and Teardown. Implementations may stash internal state in
// Metadata; external consumers should treat it as opaque.
//
// BaseRunConfig carries orchestrator-supplied defaults — model, environment,
// streaming callback — that the harness should preserve when building its
// own per-sub-call RunConfigs. Use MergeConfig to overlay harness-specific
// fields (Prompt, Stage, Role) onto the base.
type HarnessRun struct {
	HarnessName   string
	Task          task.Task
	Workspace     Workspace
	Budget        Budget
	BaseRunConfig executor.RunConfig
	Metadata      map[string]any
}

// MergeConfig overlays override onto base, returning a merged RunConfig.
// Non-zero override fields win; zero override fields keep the base value.
// Harnesses use this so they don't have to know about orchestrator defaults
// (Model, Environment, OnOutput) — they just declare what they care about
// (Prompt, Stage, Role).
func MergeConfig(base, override executor.RunConfig) executor.RunConfig {
	merged := base
	if override.WorkspacePath != "" {
		merged.WorkspacePath = override.WorkspacePath
	}
	if override.Prompt != "" {
		merged.Prompt = override.Prompt
	}
	if override.Model != "" {
		merged.Model = override.Model
	}
	if override.Environment != nil {
		merged.Environment = override.Environment
	}
	if override.OnOutput != nil {
		merged.OnOutput = override.OnOutput
	}
	if override.Stage != "" {
		merged.Stage = override.Stage
	}
	if override.Role != "" {
		merged.Role = override.Role
	}
	return merged
}

// Harness scaffolds an agent run.
//
// A Harness describes "how to invoke an executor on a task". Built-in harnesses
// include bare, agent_instructions, speckit, ralph, and planner_coder. Third parties
// implement this interface to plug in their own harness pattern (e.g., a
// Reflexion variant, a debate workflow, a custom skill bundle) without forking
// the framework.
//
// Lifecycle:
//  1. Setup: prepare the workspace (lay down context files, init tooling)
//  2. Invoke: call the executor one or more times; capture transcript
//  3. Teardown: clean up workspace artifacts (tests directory is never touched)
type Harness interface {
	// Name is the stable identifier used in experiment configs.
	Name() string

	// Description is a one-line human-readable summary shown in selectors.
	Description() string

	// Setup prepares the workspace before agent invocation. cfg carries
	// per-variant configuration the launcher supplied, keyed by harness
	// id. A harness that doesn't need config can ignore it.
	Setup(ctx context.Context, ws Workspace, t task.Task, budget Budget, cfg map[string]any) (HarnessRun, error)

	// Invoke runs the agent (possibly multiple times). Returns the merged transcript
	// covering all sub-invocations.
	Invoke(ctx context.Context, run HarnessRun, exec executor.AgentExecutor) (*executor.RunResult, error)

	// Teardown removes harness-specific files. Workspace files written by the
	// agent are preserved for eval.sh.
	Teardown(ctx context.Context, run HarnessRun) error
}
