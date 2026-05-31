// Package harness ships the built-in Harness adapters bundled with AgentDx.
//
// Third parties should import engine/pkg/harness to implement their own
// adapter; this package contains the reference implementations the framework
// ships with (bare, claudemd, speckit, ralph, planner_coder).
package harness

import (
	"context"

	"github.com/mustafaselman/frameval/engine/pkg/executor"
	"github.com/mustafaselman/frameval/engine/pkg/harness"
	"github.com/mustafaselman/frameval/engine/pkg/task"
)

// Bare is the baseline Harness: no setup, single agent invocation with the
// task's raw prompt, no teardown. Every other built-in harness is differentiated
// against `bare` in the Diagnostic Compare view.
type Bare struct{}

// NewBare constructs the Bare harness. Stateless; safe to share.
func NewBare() *Bare { return &Bare{} }

func (h *Bare) Name() string { return "bare" }

func (h *Bare) Description() string {
	return "Single agent invocation with task prompt only — no instruction files, no orchestration"
}

func (h *Bare) Setup(_ context.Context, ws harness.Workspace, t task.Task, b harness.Budget, _ map[string]any) (harness.HarnessRun, error) {
	return harness.HarnessRun{
		HarnessName: h.Name(),
		Task:        t,
		Workspace:   ws,
		Budget:      b,
	}, nil
}

func (h *Bare) Invoke(ctx context.Context, run harness.HarnessRun, exec executor.AgentExecutor) (*executor.RunResult, error) {
	return exec.Execute(ctx, harness.MergeConfig(run.BaseRunConfig, executor.RunConfig{
		Prompt:        run.Task.TaskPrompt,
		WorkspacePath: run.Workspace.Path,
	}))
}

func (h *Bare) Teardown(_ context.Context, _ harness.HarnessRun) error { return nil }
