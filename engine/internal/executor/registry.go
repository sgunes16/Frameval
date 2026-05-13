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
