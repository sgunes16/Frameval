package harness

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/mustafaselman/frameval/engine/pkg/executor"
	"github.com/mustafaselman/frameval/engine/pkg/harness"
	"github.com/mustafaselman/frameval/engine/pkg/task"
)

const (
	rolePlanner = "planner"
	roleCoder   = "coder"
)

// MultiAgent is the multi-agent harness: two sequential agent invocations
// where the first produces an implementation plan and the second implements
// against it. Both calls go through the same executor; only the prompt and
// the RunConfig.Role field differ. The merged transcript tags every turn
// with its originating role so downstream AgentDx fingerprint extraction
// can compute per-role behavioral metrics.
type MultiAgent struct{}

// NewMultiAgent constructs the harness. Stateless; safe to share.
func NewMultiAgent() *MultiAgent { return &MultiAgent{} }

func (h *MultiAgent) Name() string { return "multiagent" }

func (h *MultiAgent) Description() string {
	return "Two-role multi-agent: planner emits a written plan, coder implements against it"
}

func (h *MultiAgent) Setup(_ context.Context, ws harness.Workspace, t task.Task, b harness.Budget, _ map[string]any) (harness.HarnessRun, error) {
	return harness.HarnessRun{
		HarnessName: h.Name(),
		Task:        t,
		Workspace:   ws,
		Budget:      b,
	}, nil
}

func (h *MultiAgent) Invoke(ctx context.Context, run harness.HarnessRun, exec executor.AgentExecutor) (*executor.RunResult, error) {
	plannerResult, plannerErr := exec.Execute(ctx, harness.MergeConfig(run.BaseRunConfig, executor.RunConfig{
		Prompt:        plannerPrompt(run.Task),
		WorkspacePath: run.Workspace.Path,
		Role:          rolePlanner,
		Stage:         rolePlanner,
	}))
	if plannerResult == nil {
		plannerResult = &executor.RunResult{}
	}

	// Plan extraction — falls back to a stub if the planner errored or produced no usable output.
	plan := extractPlanFromResult(plannerResult)
	if plannerErr != nil {
		plan = "(planner stage failed; proceeding without a plan)"
	}

	// Even if planner failed, run the coder with the fallback plan so downstream
	// AgentDx still has a coder transcript to compare against. The error is
	// surfaced after coder completes.
	if errors.Is(plannerErr, context.Canceled) || errors.Is(plannerErr, context.DeadlineExceeded) {
		return mergeRoleTranscripts(plannerResult, &executor.RunResult{}), plannerErr
	}

	coderResult, coderErr := exec.Execute(ctx, harness.MergeConfig(run.BaseRunConfig, executor.RunConfig{
		Prompt:        coderPrompt(run.Task, plan),
		WorkspacePath: run.Workspace.Path,
		Role:          roleCoder,
		Stage:         roleCoder,
	}))
	if coderResult == nil {
		coderResult = &executor.RunResult{}
	}

	combined := mergeRoleTranscripts(plannerResult, coderResult)
	switch {
	case plannerErr != nil && coderErr != nil:
		// Go 1.20+ fmt.Errorf supports multiple %w verbs; both errors are
		// unwrappable via errors.Is/errors.As.
		return combined, fmt.Errorf("planner_coder: planner: %w; coder: %w", plannerErr, coderErr)
	case plannerErr != nil:
		return combined, fmt.Errorf("planner_coder: planner: %w", plannerErr)
	case coderErr != nil:
		return combined, fmt.Errorf("planner_coder: coder: %w", coderErr)
	}
	return combined, nil
}

func (h *MultiAgent) Teardown(_ context.Context, _ harness.HarnessRun) error { return nil }

// plannerPrompt instructs the agent to produce a markdown-structured plan
// WITHOUT writing code. Structure matches typical spec-driven workflows so
// the extracted plan is readable both by the next-role agent and by humans.
func plannerPrompt(t task.Task) string {
	return strings.Join([]string{
		"You are the PLANNER role in a two-stage workflow. Your output must be a written plan only — do not write code or modify files yet.",
		"Produce a markdown response with three sections:",
		"",
		"## Approach",
		"(one paragraph: what overall strategy will solve this task?)",
		"",
		"## Files to change",
		"(bulleted list of file paths the implementer should touch)",
		"",
		"## Test strategy",
		"(bulleted list of how the implementer should verify correctness)",
		"",
		"Task:",
		t.TaskPrompt,
	}, "\n")
}

// coderPrompt builds the implementation prompt. The plan is inlined so the
// coder role has it in context; the prompt explicitly authorizes deviation
// when the plan is wrong (rare but important for resilience).
func coderPrompt(t task.Task, plan string) string {
	return strings.Join([]string{
		"You are the CODER role in a two-stage workflow. A planner role has produced an implementation plan; follow it where reasonable, deviate with justification if you discover the plan is wrong.",
		"",
		"## Plan from planner",
		plan,
		"",
		"## Task",
		t.TaskPrompt,
	}, "\n")
}

// extractPlanFromResult returns the planner's most useful output. Preference:
//  1. The content of the last assistant ParsedTurn (typical Aider output).
//  2. The raw stdout if no parsed turns exist.
//  3. A stub when nothing useful was produced.
func extractPlanFromResult(r *executor.RunResult) string {
	if r == nil {
		return "(planner produced no output)"
	}
	for i := len(r.ParsedTurns) - 1; i >= 0; i-- {
		if strings.ToLower(r.ParsedTurns[i].Role) == "assistant" {
			content := strings.TrimSpace(r.ParsedTurns[i].Content)
			if content != "" {
				return content
			}
		}
	}
	if raw := strings.TrimSpace(r.RawOutput); raw != "" {
		return raw
	}
	return "(planner produced no output)"
}

// mergeRoleTranscripts concatenates planner + coder RunResults with
// `--- Role: <name> ---` markers and tags each ParsedTurn with its
// originating role (also set as Stage so existing stage-aware
// downstream consumers see it).
func mergeRoleTranscripts(planner, coder *executor.RunResult) *executor.RunResult {
	merged := &executor.RunResult{}
	var b strings.Builder
	emit := func(role string, r *executor.RunResult) {
		b.WriteString("--- Role: " + role + " ---\n")
		if r == nil {
			return
		}
		b.WriteString(r.RawOutput)
		if !strings.HasSuffix(r.RawOutput, "\n") {
			b.WriteString("\n")
		}
		for _, turn := range r.ParsedTurns {
			turn.Role = role
			turn.Stage = role
			merged.ParsedTurns = append(merged.ParsedTurns, turn)
		}
	}
	emit(rolePlanner, planner)
	emit(roleCoder, coder)
	merged.RawOutput = b.String()
	return merged
}
