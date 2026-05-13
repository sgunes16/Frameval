// Smoke test for the public pkg/ API surface.
//
// Demonstrates that a third party can implement Harness using *only* the public
// engine/pkg/ packages, with no dependency on engine/internal/. If this file
// compiles, the framework's public contract is intact.
//
// This file is intentionally tiny and standalone. The fully worked third-party
// harness example lives at examples/03-add-your-own-harness/ (post-MVP).
package main

import (
	"context"
	"fmt"

	"github.com/mustafaselman/frameval/engine/pkg/executor"
	"github.com/mustafaselman/frameval/engine/pkg/harness"
	"github.com/mustafaselman/frameval/engine/pkg/task"
)

// NoopHarness is the simplest possible Harness: it never calls the executor.
// Used here only to prove the interface can be satisfied from outside the module's
// internal/ tree.
type NoopHarness struct{}

func (h *NoopHarness) Name() string        { return "noop" }
func (h *NoopHarness) Description() string { return "Does nothing; exists to verify pkg/harness compiles externally." }

func (h *NoopHarness) Setup(_ context.Context, ws harness.Workspace, t task.Task, b harness.Budget) (harness.HarnessRun, error) {
	return harness.HarnessRun{HarnessName: h.Name(), Task: t, Workspace: ws, Budget: b}, nil
}

func (h *NoopHarness) Invoke(_ context.Context, _ harness.HarnessRun, _ executor.AgentExecutor) (*executor.RunResult, error) {
	return &executor.RunResult{RawOutput: "noop", ParsedTurns: nil}, nil
}

func (h *NoopHarness) Teardown(_ context.Context, _ harness.HarnessRun) error { return nil }

func main() {
	var h harness.Harness = &NoopHarness{}
	fmt.Printf("AgentDx public API smoke test OK — harness %q satisfies pkg/harness.Harness\n", h.Name())
}
