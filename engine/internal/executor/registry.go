package executor

import (
	"fmt"

	"github.com/mustafaselman/frameval/engine/internal/sandbox"
)

type Registry struct {
	executors map[string]AgentExecutor
}

func NewRegistry(manager *sandbox.Manager) *Registry {
	// opencode is the canonical local-Ollama executor: its --format
	// json output gives us pre-stamped ParsedTurns (no heuristic
	// parser). aider stays registered for back-compat with existing
	// experiments / transcripts but is no longer the recommended
	// path — new experiments should default to opencode.
	return &Registry{executors: map[string]AgentExecutor{
		"opencode": NewOpenCodeExecutor(manager),
		"aider":    NewAiderExecutor(manager),
		"cursor":   NewCursorExecutor(manager),
	}}
}

func (r *Registry) Get(name string) (AgentExecutor, error) {
	executor, ok := r.executors[name]
	if !ok {
		return nil, fmt.Errorf("executor %q not registered", name)
	}
	return executor, nil
}

// Entries returns all registered executors. Used by the API config endpoint
// so the frontend can list pickable executors without hard-coding names.
func (r *Registry) Entries() []AgentExecutor {
	out := make([]AgentExecutor, 0, len(r.executors))
	for _, e := range r.executors {
		out = append(out, e)
	}
	return out
}
