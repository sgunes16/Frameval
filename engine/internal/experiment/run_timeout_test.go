package experiment

import (
	"context"
	"testing"
	"time"

	bh "github.com/mustafaselman/frameval/engine/internal/builtin/harness"
	pkgharness "github.com/mustafaselman/frameval/engine/pkg/harness"
	"github.com/mustafaselman/frameval/engine/pkg/task"
	"github.com/mustafaselman/frameval/engine/test/support"
)

func bareRun(t *testing.T, maxWallSeconds int) (*bh.Bare, pkgharness.HarnessRun) {
	t.Helper()
	bare := bh.NewBare()
	hRun, err := bare.Setup(
		context.Background(),
		pkgharness.Workspace{Path: t.TempDir()},
		task.Task{TaskPrompt: "do the thing"},
		pkgharness.Budget{MaxWallSeconds: maxWallSeconds},
		nil,
	)
	if err != nil {
		t.Fatalf("bare Setup: %v", err)
	}
	return bare, hRun
}

// A stalled agent (FakeModeSlow blocks until ctx cancel) must be cut off at
// the run's wall-clock budget instead of hanging forever. Before the fix,
// Invoke ran on an undeadlined context and would block for the full 30s.
func TestInvokeWithTimeout_CancelsStalledAgent(t *testing.T) {
	bare, hRun := bareRun(t, 1) // 1s budget
	slow := support.NewFakeExecutor(support.FakeExecutorConfig{
		Mode:  support.FakeModeSlow,
		Delay: 30 * time.Second, // would hang well past the budget
	})

	start := time.Now()
	_, err, timedOut := invokeWithTimeout(context.Background(), bare, hRun, slow)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected an error from the timed-out invoke, got nil")
	}
	if !timedOut {
		t.Errorf("expected timedOut=true, got false (err=%v)", err)
	}
	if elapsed > 5*time.Second {
		t.Errorf("invoke did not honor the 1s budget; took %v", elapsed)
	}
}

// A fast-completing agent must not be flagged as timed out.
func TestInvokeWithTimeout_FastAgentNotFlagged(t *testing.T) {
	bare, hRun := bareRun(t, 30)
	fast := support.NewFakeExecutor(support.FakeExecutorConfig{
		Mode:      support.FakeModeSuccess,
		RawOutput: "done",
	})

	_, err, timedOut := invokeWithTimeout(context.Background(), bare, hRun, fast)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if timedOut {
		t.Error("fast agent wrongly flagged as timedOut")
	}
}

// A parent cancellation (e.g. experiment cancel) is not a per-run timeout —
// timedOut must stay false so the orchestrator surfaces the real cause.
func TestInvokeWithTimeout_ParentCancelNotTimeout(t *testing.T) {
	bare, hRun := bareRun(t, 30)
	slow := support.NewFakeExecutor(support.FakeExecutorConfig{
		Mode:  support.FakeModeSlow,
		Delay: 30 * time.Second,
	})

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()
	_, err, timedOut := invokeWithTimeout(ctx, bare, hRun, slow)
	if err == nil {
		t.Fatal("expected an error from the cancelled invoke")
	}
	if timedOut {
		t.Error("parent cancel wrongly reported as a per-run timeout")
	}
}
