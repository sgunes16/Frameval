// Package harness defines the public Harness interface used by AgentDx to wrap
// agent executors with a particular methodological style.
//
// AgentDx ships five built-in harnesses under engine/internal/builtin/harness:
//   - bare:          single executor call, no setup, baseline reference
//   - claudemd:      lays down CLAUDE.md from task harness_context, then bare invocation
//   - speckit:       full spec-kit workflow (constitution -> specify -> plan -> tasks -> implement)
//   - ralph:         while-loop wrapping bare until test-pass or budget exhausted
//   - planner_coder: two sequential role-tagged invocations (plan first, code second)
//
// Third parties implement this interface to plug in their own harness pattern
// (Reflexion, debate, skill-bundle variants, etc.). See examples/03-add-your-own-harness/
// for a worked walkthrough.
//
// # Example: minimal external harness
//
//	package mybarness
//
//	import (
//	    "context"
//	    "github.com/mustafaselman/frameval/engine/pkg/executor"
//	    "github.com/mustafaselman/frameval/engine/pkg/harness"
//	    "github.com/mustafaselman/frameval/engine/pkg/task"
//	)
//
//	type Bare struct{}
//
//	func (h *Bare) Name() string        { return "my-bare" }
//	func (h *Bare) Description() string { return "Single invocation, custom prompt prefix" }
//
//	func (h *Bare) Setup(ctx context.Context, ws harness.Workspace, t task.Task, b harness.Budget) (harness.HarnessRun, error) {
//	    return harness.HarnessRun{HarnessName: h.Name(), Task: t, Workspace: ws, Budget: b}, nil
//	}
//
//	func (h *Bare) Invoke(ctx context.Context, run harness.HarnessRun, exec executor.AgentExecutor) (*executor.RunResult, error) {
//	    return exec.Execute(ctx, executor.RunConfig{
//	        Prompt:        "Please be concise.\n\n" + run.Task.TaskPrompt,
//	        WorkspacePath: run.Workspace.Path,
//	    })
//	}
//
//	func (h *Bare) Teardown(ctx context.Context, run harness.HarnessRun) error { return nil }
package harness
