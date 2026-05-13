package harness

import (
	"fmt"

	pkgharness "github.com/mustafaselman/frameval/engine/pkg/harness"
)

// Registry resolves a harness adapter by name. Built-in harnesses are
// pre-registered by NewRegistry; external harnesses can be added via Register.
type Registry struct {
	adapters map[string]pkgharness.Harness
}

// NewRegistry constructs a Registry pre-populated with all built-in harnesses.
//
// Each AgentDx release ships at minimum the bare harness; additional built-ins
// (claudemd, speckit, ralph, planner_coder) land in their respective stories
// and self-register here. Built-in registration failures are programmer errors
// (a name collision between built-ins indicates the constructor itself is
// broken) and panic immediately so the engine refuses to start in an
// inconsistent state.
func NewRegistry() *Registry {
	r := &Registry{adapters: map[string]pkgharness.Harness{}}
	mustRegister(r, NewBare())
	mustRegister(r, NewClaudeMd())
	mustRegister(r, NewSpecKit())
	mustRegister(r, NewRalph())
	mustRegister(r, NewPlannerCoder())
	return r
}

func mustRegister(r *Registry, h pkgharness.Harness) {
	if err := r.Register(h); err != nil {
		panic(fmt.Sprintf("harness: failed to register built-in %q: %v", h.Name(), err))
	}
}

// Register adds an adapter. Returns an error if the name collides with an
// existing registration.
func (r *Registry) Register(h pkgharness.Harness) error {
	if _, exists := r.adapters[h.Name()]; exists {
		return fmt.Errorf("harness %q already registered", h.Name())
	}
	r.adapters[h.Name()] = h
	return nil
}

// Get returns the registered adapter for name, or an error if unknown.
func (r *Registry) Get(name string) (pkgharness.Harness, error) {
	h, ok := r.adapters[name]
	if !ok {
		return nil, fmt.Errorf("harness %q not registered", name)
	}
	return h, nil
}

// List returns the names of all registered harnesses (sorted is not guaranteed).
func (r *Registry) List() []string {
	names := make([]string, 0, len(r.adapters))
	for name := range r.adapters {
		names = append(names, name)
	}
	return names
}
