package executor

import (
	"fmt"

	"github.com/mustafaselman/frameval/engine/internal/sandbox"
)

type Registry struct {
	executors map[string]AgentExecutor
}

func NewRegistry(manager *sandbox.Manager) *Registry {
	return &Registry{executors: map[string]AgentExecutor{
		"aider":  NewAiderExecutor(manager),
		"cursor": NewCursorExecutor(manager),
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
