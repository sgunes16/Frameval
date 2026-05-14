package experiment

import (
	"context"
	"errors"
	"testing"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestRetry_SucceedsOnFirstAttempt(t *testing.T) {
	calls := 0
	got, err := retryGrader(context.Background(), defaultGraderRetry, func(_ context.Context) (int, error) {
		calls++
		return 42, nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != 42 || calls != 1 {
		t.Errorf("got=%d calls=%d (want 42 / 1)", got, calls)
	}
}

func TestRetry_RetriesUnavailableThenSucceeds(t *testing.T) {
	calls := 0
	got, err := retryGrader(context.Background(), graderRetryConfig{MaxAttempts: 3, InitialDelay: 1 * time.Millisecond, Multiplier: 2}, func(_ context.Context) (int, error) {
		calls++
		if calls < 3 {
			return 0, status.Error(codes.Unavailable, "transient")
		}
		return 7, nil
	})
	if err != nil {
		t.Fatalf("unexpected error after retries: %v", err)
	}
	if got != 7 || calls != 3 {
		t.Errorf("expected success after 3 attempts; got=%d calls=%d", got, calls)
	}
}

func TestRetry_DoesNotRetryNonRetryableCode(t *testing.T) {
	calls := 0
	_, err := retryGrader(context.Background(), defaultGraderRetry, func(_ context.Context) (int, error) {
		calls++
		return 0, status.Error(codes.InvalidArgument, "client bug")
	})
	if err == nil {
		t.Fatal("expected an error")
	}
	if calls != 1 {
		t.Errorf("InvalidArgument should not retry; got %d calls", calls)
	}
}

func TestRetry_GivesUpAfterMaxAttempts(t *testing.T) {
	calls := 0
	_, err := retryGrader(context.Background(), graderRetryConfig{MaxAttempts: 2, InitialDelay: 1 * time.Millisecond, Multiplier: 2}, func(_ context.Context) (int, error) {
		calls++
		return 0, status.Error(codes.Unavailable, "always down")
	})
	if err == nil {
		t.Fatal("expected error after exhausting retries")
	}
	if calls != 2 {
		t.Errorf("expected 2 attempts; got %d", calls)
	}
}

func TestRetry_ContextCancellationStopsRetries(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already cancelled
	calls := 0
	_, err := retryGrader(ctx, defaultGraderRetry, func(_ context.Context) (int, error) {
		calls++
		return 0, status.Error(codes.Unavailable, "transient")
	})
	if err == nil {
		t.Fatal("expected error on cancelled context")
	}
	// At least one attempt fires before backoff sleeps on the cancelled ctx.
	if calls > 1 {
		t.Errorf("expected ≤ 1 attempt before ctx cancel; got %d", calls)
	}
}

func TestBreaker_OpensAfterConsecutiveFailures(t *testing.T) {
	b := newGraderBreaker()

	// Hit the breaker with 5 failures back-to-back. The 6th call must
	// short-circuit with ErrGraderUnavailable rather than invoke op.
	for i := 0; i < 5; i++ {
		_, err := breakerExec(b, func() (int, error) {
			return 0, errors.New("grader down")
		})
		if err == nil {
			t.Fatalf("attempt %d should have errored", i+1)
		}
	}

	calls := 0
	_, err := breakerExec(b, func() (int, error) {
		calls++
		return 0, nil
	})
	if !errors.Is(err, ErrGraderUnavailable) {
		t.Errorf("breaker open path should wrap ErrGraderUnavailable, got %v", err)
	}
	if calls != 0 {
		t.Errorf("breaker open should not invoke op; got %d calls", calls)
	}
}

func TestBreaker_ContextCancelDoesNotCountAsFailure(t *testing.T) {
	b := newGraderBreaker()

	// Hit the breaker with 6 context-cancelled calls — well above the
	// 5-failure threshold. The breaker must stay closed; otherwise an
	// operator cancelling experiments could degrade grading for 30s.
	for i := 0; i < 6; i++ {
		_, _ = breakerExec(b, func() (int, error) {
			return 0, context.Canceled
		})
	}

	// A subsequent success-path call must invoke op (breaker not open).
	calls := 0
	_, err := breakerExec(b, func() (int, error) {
		calls++
		return 1, nil
	})
	if err != nil {
		t.Fatalf("breaker should still be closed; got %v", err)
	}
	if calls != 1 {
		t.Errorf("op should have been invoked (breaker closed); calls=%d", calls)
	}
}

func TestRetry_DoesNotRetryDeadlineExceeded(t *testing.T) {
	// DeadlineExceeded is no longer retried — retrying on a context
	// whose deadline has elapsed wastes work. The test asserts the
	// classifier surfaces the first error immediately.
	calls := 0
	_, err := retryGrader(context.Background(), defaultGraderRetry, func(_ context.Context) (int, error) {
		calls++
		return 0, status.Error(codes.DeadlineExceeded, "ran out of time")
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if calls != 1 {
		t.Errorf("DeadlineExceeded must not retry; calls=%d", calls)
	}
}

func TestBreaker_PassesThroughWhenClosed(t *testing.T) {
	b := newGraderBreaker()
	got, err := breakerExec(b, func() (int, error) {
		return 5, nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != 5 {
		t.Errorf("breaker should return op's value verbatim; got %d", got)
	}
}
