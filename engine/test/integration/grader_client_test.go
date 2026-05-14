//go:build integration

// Package integration_test contains cross-component slices that exercise real
// engine code paths against the FakeGrader and TmpStore helpers. The tests
// here are gated behind the "integration" build tag so the default
// `go test ./...` run stays fast; CI invokes them via
// `go test -tags=integration ./engine/test/integration/...`.
package integration_test

import (
	"context"
	"testing"
	"time"

	"github.com/mustafaselman/frameval/engine/internal/experiment"
	"github.com/mustafaselman/frameval/engine/internal/models"
	"github.com/mustafaselman/frameval/engine/pkg/diagnostic"
	"github.com/mustafaselman/frameval/engine/test/support"
	"go.uber.org/goleak"
)

// TestMain runs goleak after every test in this package. Long-lived goroutines
// (gRPC server loops, queue workers, hub goroutines) leaking past test
// completion are a class of bug we want CI to catch.
func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m,
		// GraderClient currently opens a fresh *grpc.ClientConn on every RPC
		// (grader_client.go:68, 125). Each connection spawns a balance-watcher
		// goroutine that does not always shut down within bounded time at
		// grpc-go v1.80 (see grpc/grpc-go#5321). Tracked as a follow-up in
		// #86 — once GraderClient holds a persistent connection, both
		// ignores can be removed.
		goleak.IgnoreTopFunction("google.golang.org/grpc.(*ccBalancerWrapper).watcher"),
		goleak.IgnoreAnyFunction("google.golang.org/grpc/internal/grpcsync.(*CallbackSerializer).run"),
	)
}

func TestGraderClient_GradeRun_RoundTripsThroughFakeGrader(t *testing.T) {
	// Float32-exact dyadic fractions to keep assertions stable across the
	// proto float32 wire round-trip back into float64.
	addr := support.StartFakeGrader(t, support.FakeGraderConfig{
		CompositeScore: 6.5,
		TestPassRate:   0.75,
	})

	client := experiment.NewGraderClient(addr)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	task := models.Task{ID: "task-1", TaskPrompt: "p", CodebaseType: "python"}
	transcript := models.Transcript{
		RunID:     "run-1",
		RawOutput: "fake",
	}

	grade, err := client.GradeRun(ctx, task, nil, transcript)
	if err != nil {
		t.Fatalf("GradeRun: %v", err)
	}
	if grade.CompositeScore != 6.5 {
		t.Errorf("CompositeScore: want 6.5, got %v", grade.CompositeScore)
	}
	if grade.TestPassRate != 0.75 {
		t.Errorf("TestPassRate: want 0.75, got %v", grade.TestPassRate)
	}
}

func TestGraderClient_ClassifyFailure_RoundTripsThroughFakeGrader(t *testing.T) {
	addr := support.StartFakeGrader(t, support.FakeGraderConfig{
		FailurePrimary:    "HAL_API",
		FailureConfidence: 0.875,
		FailureRationale:  "agent called nonexistent method",
	})

	client := experiment.NewGraderClient(addr)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	verdict := client.ClassifyFailure(ctx, "run-1", diagnostic.Symptoms{}, "task description", "tail", "claude-haiku-4-5-20251001")

	if verdict.Classification.Primary != "HAL_API" {
		t.Errorf("Primary: want HAL_API, got %q", verdict.Classification.Primary)
	}
	if verdict.Classification.Confidence != 0.875 {
		t.Errorf("Confidence: want 0.875, got %v", verdict.Classification.Confidence)
	}
}

func TestGraderClient_FallsBackWhenGraderUnreachable(t *testing.T) {
	// Empty addr forces the GraderClient's "no grader configured" branch:
	// it must return a fallback grade rather than blocking on a dial that
	// never resolves.
	client := experiment.NewGraderClient("")

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	task := models.Task{ID: "task-1", TaskPrompt: "p"}
	transcript := models.Transcript{RunID: "run-1", TotalTokens: 100}

	grade, err := client.GradeRun(ctx, task, nil, transcript)
	if err != nil {
		t.Fatalf("expected fallback, got error: %v", err)
	}
	if grade.TokenEfficiency == 0 {
		t.Error("fallback grade should compute a non-zero token efficiency")
	}
}

