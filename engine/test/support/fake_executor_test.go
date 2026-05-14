package support_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/mustafaselman/frameval/engine/pkg/executor"
	"github.com/mustafaselman/frameval/engine/test/support"
)

func TestFakeExecutor_SuccessReturnsCannedResult(t *testing.T) {
	fe := support.NewFakeExecutor(support.FakeExecutorConfig{
		Mode:      support.FakeModeSuccess,
		RawOutput: "assistant: ok\nDone.",
		Turns: []executor.ParsedTurn{
			{Role: "assistant", Content: "ok"},
		},
	})

	if fe.Name() == "" {
		t.Error("Name() must be non-empty")
	}
	modes := fe.SupportedModes()
	if len(modes) == 0 {
		t.Error("SupportedModes must declare at least one mode")
	}

	result, err := fe.Execute(context.Background(), executor.RunConfig{Prompt: "p"})
	if err != nil {
		t.Fatalf("expected nil error in success mode, got %v", err)
	}
	if !strings.Contains(result.RawOutput, "Done") {
		t.Errorf("RawOutput not propagated: %q", result.RawOutput)
	}
	if len(result.ParsedTurns) != 1 || result.ParsedTurns[0].Role != "assistant" {
		t.Errorf("ParsedTurns not propagated: %+v", result.ParsedTurns)
	}
}

func TestFakeExecutor_PanicModeRecoversToError(t *testing.T) {
	fe := support.NewFakeExecutor(support.FakeExecutorConfig{
		Mode:      support.FakeModePanic,
		PanicWith: "simulated executor crash",
	})

	// The fake must convert its planned panic into an error rather than crash
	// the test runner — production code is what we're stressing, not the harness.
	_, err := fe.Execute(context.Background(), executor.RunConfig{})
	if err == nil {
		t.Fatal("expected error from panic mode, got nil")
	}
	if !strings.Contains(err.Error(), "simulated executor crash") {
		t.Errorf("error should carry panic value, got %v", err)
	}
}

func TestFakeExecutor_SlowModeRespectsContextCancel(t *testing.T) {
	fe := support.NewFakeExecutor(support.FakeExecutorConfig{
		Mode:  support.FakeModeSlow,
		Delay: 5 * time.Second,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err := fe.Execute(ctx, executor.RunConfig{})
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected context error, got nil")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("expected DeadlineExceeded, got %v", err)
	}
	if elapsed > 1*time.Second {
		t.Errorf("Execute did not return promptly on ctx cancel: %v", elapsed)
	}
}

func TestFakeExecutor_PartialThenStopStreamsThenReturns(t *testing.T) {
	fe := support.NewFakeExecutor(support.FakeExecutorConfig{
		Mode:         support.FakeModePartialThenStop,
		StreamedLogs: []string{"line one", "line two", "line three"},
		RawOutput:    "line one\nline two\nline three",
	})

	var captured []string
	cfg := executor.RunConfig{
		OnOutput: func(line string) { captured = append(captured, line) },
	}
	result, err := fe.Execute(context.Background(), cfg)
	if err != nil {
		t.Fatalf("partial-then-stop should complete cleanly, got %v", err)
	}
	if len(captured) != 3 {
		t.Errorf("expected 3 streamed lines, got %d: %v", len(captured), captured)
	}
	if !result.StreamedOutput {
		t.Error("StreamedOutput flag should be true for partial mode")
	}
}
