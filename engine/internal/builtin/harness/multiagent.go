package harness

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/mustafaselman/frameval/engine/pkg/executor"
	"github.com/mustafaselman/frameval/engine/pkg/harness"
	"github.com/mustafaselman/frameval/engine/pkg/task"
)

const (
	// MultiAgentHarnessID is the stable wire id for this harness.
	MultiAgentHarnessID = "multiagent"

	// multiagentConfigKey is the top-level key the harness reads from
	// the per-variant cfg map.
	multiagentConfigKey = "multiagent"

	// multiagentMinRoles / multiagentMaxRoles bound how many sequential
	// roles a single variant may declare. Five keeps the launcher form
	// and the merged transcript readable; lift in a follow-up if a real
	// research need ever emerges.
	multiagentMinRoles = 1
	multiagentMaxRoles = 5
)

// roleNamePattern enforces snake_case ASCII so role names show up
// predictably in logs, JSON, and the role-accent color hash. Empty
// is rejected separately for a clearer error message.
var roleNamePattern = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// ErrMultiAgentRolesMissing surfaces when the launcher's per-variant
// cfg has no roles at all (nil, missing key, empty array). The
// launcher's submit gate prevents this in normal flow; the sentinel
// is the last line of defense for direct API consumers.
var ErrMultiAgentRolesMissing = errors.New(
	"multiagent harness: cfg.multiagent.roles is empty; user must configure roles in the launcher")

// ErrMultiAgentInvalidRole surfaces for any per-role validation
// failure: bad name, empty prompt, duplicate, count outside the
// supported range. The error string carries the specific reason.
var ErrMultiAgentInvalidRole = errors.New("multiagent harness: invalid role")

// MultiAgent runs a user-configured sequence of agent roles. Each role
// has a name and a prompt template; substitutions {{TASK}} and
// {{PREV_OUTPUT}} are resolved before each call. Every emitted turn
// is tagged with the role name so the Inspector can render per-role
// visual cues.
type MultiAgent struct{}

func NewMultiAgent() *MultiAgent { return &MultiAgent{} }

func (h *MultiAgent) Name() string { return MultiAgentHarnessID }
func (h *MultiAgent) Description() string {
	return "Configurable sequence of N agent roles (1-5). Each role gets its own prompt; outputs flow forward via {{PREV_OUTPUT}}."
}

// role is the internal, validated shape of one configured role.
type role struct {
	Name   string
	Prompt string
}

func (h *MultiAgent) Setup(_ context.Context, ws harness.Workspace, t task.Task, b harness.Budget, cfg map[string]any) (harness.HarnessRun, error) {
	roles, err := extractRoles(cfg)
	if err != nil {
		return harness.HarnessRun{}, err
	}
	return harness.HarnessRun{
		HarnessName: h.Name(),
		Task:        t,
		Workspace:   ws,
		Budget:      b,
		Metadata:    map[string]any{"multiagent.roles": roles},
	}, nil
}

func (h *MultiAgent) Invoke(ctx context.Context, run harness.HarnessRun, exec executor.AgentExecutor) (*executor.RunResult, error) {
	roles, ok := run.Metadata["multiagent.roles"].([]role)
	if !ok || len(roles) == 0 {
		return nil, ErrMultiAgentRolesMissing
	}

	merged := &executor.RunResult{}
	var rawBuilder strings.Builder
	var prevOutput string
	var roleErrors []error

	for i, r := range roles {
		prompt := expandPrompt(r.Prompt, map[string]string{
			"TASK":        run.Task.TaskPrompt,
			"PREV_OUTPUT": prevOutput,
		})
		result, err := exec.Execute(ctx, harness.MergeConfig(run.BaseRunConfig, executor.RunConfig{
			Prompt:        prompt,
			WorkspacePath: run.Workspace.Path,
			Role:          r.Name,
			Stage:         r.Name,
		}))
		if result == nil {
			result = &executor.RunResult{}
		}

		// Always merge whatever this role produced (raw + tagged turns)
		// so the user has a transcript even on partial failure.
		rawBuilder.WriteString("--- Role: ")
		rawBuilder.WriteString(r.Name)
		rawBuilder.WriteString(" ---\n")
		rawBuilder.WriteString(result.RawOutput)
		if !strings.HasSuffix(result.RawOutput, "\n") {
			rawBuilder.WriteString("\n")
		}
		for _, turn := range result.ParsedTurns {
			turn.Role = r.Name
			turn.Stage = r.Name
			merged.ParsedTurns = append(merged.ParsedTurns, turn)
		}

		// ctx cancellation aborts immediately — no point spinning up
		// the next role when the user / scheduler told us to stop.
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			merged.RawOutput = rawBuilder.String()
			return merged, err
		}

		if err != nil {
			roleErrors = append(roleErrors, fmt.Errorf("role %q: %w", r.Name, err))
			prevOutput = fmt.Sprintf("(role %s failed; proceeding without its output)", r.Name)
			continue
		}

		prevOutput = extractLastAssistantOutput(result)
		_ = i // index reserved for future per-role policy
	}

	merged.RawOutput = rawBuilder.String()
	if len(roleErrors) > 0 {
		return merged, fmt.Errorf("multiagent: %w", errors.Join(roleErrors...))
	}
	return merged, nil
}

// extractLastAssistantOutput returns the last assistant turn's content
// from a RunResult, falling back to RawOutput when no assistant turn is
// present. Used to feed the next role's {{PREV_OUTPUT}} substitution.
func extractLastAssistantOutput(r *executor.RunResult) string {
	if r == nil {
		return ""
	}
	for i := len(r.ParsedTurns) - 1; i >= 0; i-- {
		if strings.EqualFold(r.ParsedTurns[i].Role, "assistant") {
			content := strings.TrimSpace(r.ParsedTurns[i].Content)
			if content != "" {
				return content
			}
		}
	}
	return strings.TrimSpace(r.RawOutput)
}

func (h *MultiAgent) Teardown(_ context.Context, _ harness.HarnessRun) error { return nil }

func extractRoles(cfg map[string]any) ([]role, error) {
	if cfg == nil {
		return nil, ErrMultiAgentRolesMissing
	}
	sub, ok := cfg[multiagentConfigKey].(map[string]any)
	if !ok {
		return nil, ErrMultiAgentRolesMissing
	}
	rawList, ok := sub["roles"].([]any)
	if !ok || len(rawList) == 0 {
		return nil, ErrMultiAgentRolesMissing
	}
	if len(rawList) < multiagentMinRoles || len(rawList) > multiagentMaxRoles {
		return nil, fmt.Errorf("%w: role count %d outside supported range [%d, %d]", ErrMultiAgentInvalidRole, len(rawList), multiagentMinRoles, multiagentMaxRoles)
	}
	seen := make(map[string]struct{}, len(rawList))
	out := make([]role, 0, len(rawList))
	for i, raw := range rawList {
		m, ok := raw.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("%w: role %d is not an object", ErrMultiAgentInvalidRole, i)
		}
		name, _ := m["name"].(string)
		prompt, _ := m["prompt"].(string)
		if !roleNamePattern.MatchString(name) {
			return nil, fmt.Errorf("%w: role %d name %q must match %s", ErrMultiAgentInvalidRole, i, name, roleNamePattern)
		}
		if strings.TrimSpace(prompt) == "" {
			return nil, fmt.Errorf("%w: role %d (%s) has empty prompt", ErrMultiAgentInvalidRole, i, name)
		}
		if _, dup := seen[name]; dup {
			return nil, fmt.Errorf("%w: role name %q appears more than once", ErrMultiAgentInvalidRole, name)
		}
		seen[name] = struct{}{}
		out = append(out, role{Name: name, Prompt: prompt})
	}
	return out, nil
}

// expandPrompt replaces {{TASK}} and {{PREV_OUTPUT}} literally with
// the corresponding entries in vars. Unknown {{...}} tokens are
// preserved as-is — the agent may legitimately write them and we
// don't want to silently mangle their text.
func expandPrompt(template string, vars map[string]string) string {
	out := template
	out = strings.ReplaceAll(out, "{{TASK}}", vars["TASK"])
	out = strings.ReplaceAll(out, "{{PREV_OUTPUT}}", vars["PREV_OUTPUT"])
	return out
}
