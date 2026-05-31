package support

import (
	"context"
	"fmt"

	"github.com/mustafaselman/frameval/engine/internal/models"
	pkgexecutor "github.com/mustafaselman/frameval/engine/pkg/executor"
	pkgharness "github.com/mustafaselman/frameval/engine/pkg/harness"
	pkgtask "github.com/mustafaselman/frameval/engine/pkg/task"
)

// staticHarness is a stub pkgharness.Harness that satisfies the interface
// with no-op implementations. Only Name() is meaningful for test assertions.
type staticHarness struct{ name string }

func (h *staticHarness) Name() string        { return h.name }
func (h *staticHarness) Description() string { return "stub harness " + h.name }
func (h *staticHarness) Setup(_ context.Context, _ pkgharness.Workspace, _ pkgtask.Task, _ pkgharness.Budget) (pkgharness.HarnessRun, error) {
	return pkgharness.HarnessRun{}, nil
}
func (h *staticHarness) Invoke(_ context.Context, _ pkgharness.HarnessRun, _ pkgexecutor.AgentExecutor) (*pkgexecutor.RunResult, error) {
	return &pkgexecutor.RunResult{}, nil
}
func (h *staticHarness) Teardown(_ context.Context, _ pkgharness.HarnessRun) error { return nil }

// StaticHarnessRegistry is a stub harness lookup that responds to Get/Entries
// using a fixed set of names. Satisfies the api.harnessLookup interface.
type StaticHarnessRegistry struct {
	harnesses map[string]pkgharness.Harness
}

// NewStaticHarnessRegistry returns a stub registry that recognises the given
// names and returns a no-op harness for each. Any other name returns an error.
func NewStaticHarnessRegistry(names ...string) *StaticHarnessRegistry {
	m := make(map[string]pkgharness.Harness, len(names))
	for _, n := range names {
		m[n] = &staticHarness{name: n}
	}
	return &StaticHarnessRegistry{harnesses: m}
}

func (r *StaticHarnessRegistry) Get(name string) (pkgharness.Harness, error) {
	h, ok := r.harnesses[name]
	if !ok {
		return nil, fmt.Errorf("stub harness registry: %q not registered", name)
	}
	return h, nil
}

func (r *StaticHarnessRegistry) Entries() []pkgharness.Harness {
	out := make([]pkgharness.Harness, 0, len(r.harnesses))
	for _, h := range r.harnesses {
		out = append(out, h)
	}
	return out
}

// staticExecutor is a stub AgentExecutor that satisfies the interface with
// no-op implementations. Only Name() is meaningful for test assertions.
type staticExecutor struct{ name string }

func (e *staticExecutor) Name() string { return e.name }
func (e *staticExecutor) SupportedModes() []pkgexecutor.ExecutionMode {
	return []pkgexecutor.ExecutionMode{pkgexecutor.ExecutionModeCLI}
}
func (e *staticExecutor) Execute(_ context.Context, _ pkgexecutor.RunConfig) (*pkgexecutor.RunResult, error) {
	return &pkgexecutor.RunResult{}, nil
}
func (e *staticExecutor) ParseTranscript(_ []byte) ([]pkgexecutor.ParsedTurn, error) {
	return nil, nil
}

// StaticExecutorRegistry is a stub executor lookup that responds to Get/Entries
// using a fixed set of names. Satisfies the api.executorLookup interface.
type StaticExecutorRegistry struct {
	executors map[string]pkgexecutor.AgentExecutor
}

// NewStaticExecutorRegistry returns a stub registry that recognises the given
// names and returns a no-op executor for each. Any other name returns an error.
func NewStaticExecutorRegistry(names ...string) *StaticExecutorRegistry {
	m := make(map[string]pkgexecutor.AgentExecutor, len(names))
	for _, n := range names {
		m[n] = &staticExecutor{name: n}
	}
	return &StaticExecutorRegistry{executors: m}
}

func (r *StaticExecutorRegistry) Get(name string) (pkgexecutor.AgentExecutor, error) {
	e, ok := r.executors[name]
	if !ok {
		return nil, fmt.Errorf("stub executor registry: %q not registered", name)
	}
	return e, nil
}

func (r *StaticExecutorRegistry) Entries() []pkgexecutor.AgentExecutor {
	out := make([]pkgexecutor.AgentExecutor, 0, len(r.executors))
	for _, e := range r.executors {
		out = append(out, e)
	}
	return out
}

// NoopOrchestrator is a stub orchestrator that satisfies the api.orchestratorIface
// interface. StartExperiment returns nil; all other methods are no-ops.
type NoopOrchestrator struct{}

// NewNoopOrchestrator returns a NoopOrchestrator stub.
func NewNoopOrchestrator() *NoopOrchestrator { return &NoopOrchestrator{} }

func (o *NoopOrchestrator) StartExperiment(_ context.Context, _ string) error  { return nil }
func (o *NoopOrchestrator) CancelExperiment(_ context.Context, _ string) error  { return nil }
func (o *NoopOrchestrator) RetryRun(_ context.Context, _ string) error          { return nil }
func (o *NoopOrchestrator) RegradeRun(_ context.Context, _ string) error        { return nil }
func (o *NoopOrchestrator) ReparseRunTranscript(_ context.Context, _ string) (int, error) {
	return 0, nil
}
func (o *NoopOrchestrator) EstimateExperiment(_ context.Context, _ string) (float64, error) {
	return 0, nil
}
func (o *NoopOrchestrator) QueueSnapshot() models.QueueStatus   { return models.QueueStatus{} }
func (o *NoopOrchestrator) SandboxHealth(_ context.Context) map[string]any {
	return map[string]any{"healthy": true}
}
