package support

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/mustafaselman/frameval/engine/pkg/executor"
)

// FakeMode selects the deterministic behavior FakeExecutor exhibits during Execute.
type FakeMode int

const (
	// FakeModeSuccess returns the configured RunResult immediately with no error.
	FakeModeSuccess FakeMode = iota
	// FakeModePanic deliberately panics inside Execute and recovers the panic
	// into an error. Lets integration tests assert the orchestrator turns a
	// crashing executor into a failed run rather than wedging.
	FakeModePanic
	// FakeModeSlow blocks until the configured Delay elapses OR the context
	// is canceled, whichever comes first. Returns ctx.Err() on cancel.
	FakeModeSlow
	// FakeModePartialThenStop streams StreamedLogs through OnOutput line by
	// line, then returns RawOutput/ParsedTurns successfully. Used to assert
	// the orchestrator handles partial output that ends cleanly.
	FakeModePartialThenStop
)

// FakeExecutorConfig is the canned behavior bundle. All zero values are valid;
// the executor defaults to Success mode returning empty output.
type FakeExecutorConfig struct {
	Mode         FakeMode
	RawOutput    string
	Turns        []executor.ParsedTurn
	PanicWith    string
	Delay        time.Duration
	StreamedLogs []string
}

// FakeExecutor implements executor.AgentExecutor with deterministic, canned
// behavior controlled by FakeMode. Use NewFakeExecutor to construct.
type FakeExecutor struct {
	cfg FakeExecutorConfig
}

// NewFakeExecutor returns a FakeExecutor with the given config.
func NewFakeExecutor(cfg FakeExecutorConfig) *FakeExecutor {
	return &FakeExecutor{cfg: cfg}
}

// Name returns the stable identifier used for executor registry lookups.
func (f *FakeExecutor) Name() string { return "fake" }

// SupportedModes reports which execution modes the fake supports.
func (f *FakeExecutor) SupportedModes() []executor.ExecutionMode {
	return []executor.ExecutionMode{executor.ExecutionModeCLI}
}

// Execute runs the fake's canned behavior according to its mode.
func (f *FakeExecutor) Execute(ctx context.Context, cfg executor.RunConfig) (result *executor.RunResult, err error) {
	switch f.cfg.Mode {
	case FakeModePanic:
		// Recover the planned panic into an error so the test runner is not
		// killed. Production code is the unit under stress — not the harness.
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("fake executor panic: %v", r)
				result = nil
			}
		}()
		panic(f.cfg.PanicWith)

	case FakeModeSlow:
		select {
		case <-time.After(f.cfg.Delay):
			return f.successResult(false), nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}

	case FakeModePartialThenStop:
		for _, line := range f.cfg.StreamedLogs {
			if cfg.OnOutput != nil {
				cfg.OnOutput(line)
			}
		}
		return f.successResult(true), nil

	default:
		return f.successResult(false), nil
	}
}

// ParseTranscript returns the configured ParsedTurns unchanged. The raw byte
// slice is ignored so tests can drive both the cached transcript and the
// re-parse path with the same canned data.
func (f *FakeExecutor) ParseTranscript(_ []byte) ([]executor.ParsedTurn, error) {
	return append([]executor.ParsedTurn(nil), f.cfg.Turns...), nil
}

func (f *FakeExecutor) successResult(streamed bool) *executor.RunResult {
	return &executor.RunResult{
		RawOutput:      strings.TrimRight(f.cfg.RawOutput, "\n"),
		ParsedTurns:    append([]executor.ParsedTurn(nil), f.cfg.Turns...),
		StreamedOutput: streamed,
	}
}
