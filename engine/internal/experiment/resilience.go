package experiment

import (
	"context"
	"errors"
	"expvar"
	"fmt"
	"time"

	"github.com/sony/gobreaker/v2"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ErrGraderUnavailable signals that the grader circuit breaker is open
// (or a half-open probe was rejected). Callers use errors.Is to detect it
// and route the run to the "grading_failed" status rather than dragging
// the orchestrator through a long timeout chain.
var ErrGraderUnavailable = errors.New("grader unavailable")

// graderRetryConfig controls how aggressively we retry transient gRPC
// failures. The defaults (3 attempts, 100ms initial, 4× multiplier)
// give 0.1s + 0.4s + 1.6s of backoff for ~2.1s of total wait — short
// enough not to block other runs, long enough to absorb a single grader
// restart.
type graderRetryConfig struct {
	MaxAttempts  int
	InitialDelay time.Duration
	Multiplier   float64
}

var defaultGraderRetry = graderRetryConfig{
	MaxAttempts:  3,
	InitialDelay: 100 * time.Millisecond,
	Multiplier:   4,
}

// isRetryableGRPC reports whether a gRPC error represents a transient
// fault worth retrying. Unavailable means the server (or a proxy) is
// momentarily down; DeadlineExceeded usually means an in-flight call
// has stalled — a fresh attempt may succeed. Every other code (logic
// errors, protocol violations) retries cost more than they help.
func isRetryableGRPC(err error) bool {
	if err == nil {
		return false
	}
	s, ok := status.FromError(err)
	if !ok {
		return false
	}
	switch s.Code() {
	case codes.Unavailable, codes.DeadlineExceeded:
		return true
	}
	return false
}

// retryGrader runs op up to cfg.MaxAttempts times, retrying only on
// transient codes (see isRetryableGRPC). Backoff grows by cfg.Multiplier
// each attempt. Returns the last error verbatim — wrapping is the
// caller's job if they want to attach a stable error code.
func retryGrader[T any](ctx context.Context, cfg graderRetryConfig, op func(ctx context.Context) (T, error)) (T, error) {
	var zero T
	var lastErr error
	delay := cfg.InitialDelay
	for attempt := 0; attempt < cfg.MaxAttempts; attempt++ {
		result, err := op(ctx)
		if err == nil {
			return result, nil
		}
		lastErr = err
		if !isRetryableGRPC(err) || attempt == cfg.MaxAttempts-1 {
			break
		}
		select {
		case <-time.After(delay):
		case <-ctx.Done():
			return zero, ctx.Err()
		}
		delay = time.Duration(float64(delay) * cfg.Multiplier)
	}
	return zero, lastErr
}

// graderBreakerState is an expvar-exposed string ("closed", "open",
// "half-open") so operators can see the current breaker state on
// /debug/vars. Updated by the breaker's OnStateChange callback.
var graderBreakerState *expvar.String

func init() {
	if v := expvar.Get("frameval_grader_breaker_state"); v != nil {
		graderBreakerState = v.(*expvar.String)
	} else {
		graderBreakerState = expvar.NewString("frameval_grader_breaker_state")
		graderBreakerState.Set("closed")
	}
}

// newGraderBreaker returns a circuit breaker tuned for the grader RPC
// surface: opens after 5 consecutive failures, stays open for 30s,
// allows a single probe in half-open. The state callback keeps the
// expvar metric in sync so external monitoring picks the transition up
// without polling Go internals.
func newGraderBreaker() *gobreaker.CircuitBreaker[any] {
	return gobreaker.NewCircuitBreaker[any](gobreaker.Settings{
		Name:        "grader",
		MaxRequests: 1,
		Timeout:     30 * time.Second,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			return counts.ConsecutiveFailures >= 5
		},
		OnStateChange: func(_ string, _, to gobreaker.State) {
			graderBreakerState.Set(to.String())
		},
	})
}

// breakerExec wraps op with the breaker. On a breaker rejection
// (open state, or too many half-open requests) the returned error
// wraps ErrGraderUnavailable so callers can route the failure via
// errors.Is without scraping strings.
func breakerExec[T any](b *gobreaker.CircuitBreaker[any], op func() (T, error)) (T, error) {
	var zero T
	raw, err := b.Execute(func() (any, error) {
		return op()
	})
	if err != nil {
		if errors.Is(err, gobreaker.ErrOpenState) || errors.Is(err, gobreaker.ErrTooManyRequests) {
			return zero, fmt.Errorf("%w: %v", ErrGraderUnavailable, err)
		}
		return zero, err
	}
	v, ok := raw.(T)
	if !ok {
		return zero, fmt.Errorf("breakerExec: unexpected value type %T", raw)
	}
	return v, nil
}
