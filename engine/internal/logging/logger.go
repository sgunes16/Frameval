// Package logging is the engine's structured-logging boundary. Production
// code never imports "log" directly; instead it constructs a root logger
// once at startup and derives per-request / per-run child loggers via
// FromContext.
//
// Trace IDs flow through context.Context. Middleware at the API boundary
// stuffs an X-Frameval-Trace value (or a fresh UUIDv7) into the context;
// every layer below just calls logging.FromContext(ctx, root) and the
// child logger carries trace_id, run_id, and experiment_id as structured
// attributes without each call site having to remember them.
package logging

import (
	"context"
	"io"
	"log/slog"
	"os"
)

// Config controls how the root logger is constructed at startup.
//
// Format == "pretty" produces human-readable text (one line per record);
// any other value produces JSON. Level defaults to Info if zero. Output
// defaults to os.Stderr when nil.
type Config struct {
	Level  slog.Level
	Format string
	Output io.Writer
}

// New returns a root *slog.Logger configured from cfg.
func New(cfg Config) *slog.Logger {
	out := cfg.Output
	if out == nil {
		out = os.Stderr
	}
	opts := &slog.HandlerOptions{Level: cfg.Level}

	var handler slog.Handler
	if cfg.Format == "pretty" {
		handler = slog.NewTextHandler(out, opts)
	} else {
		handler = slog.NewJSONHandler(out, opts)
	}
	return slog.New(handler)
}

// contextKey is a private named type so other packages cannot collide on
// the same context keys by accident.
type contextKey string

const (
	traceIDKey      contextKey = "trace_id"
	runIDKey        contextKey = "run_id"
	experimentIDKey contextKey = "experiment_id"
)

// WithTraceID returns a derived context carrying a request-scoped trace ID.
// The middleware at the API boundary is the canonical place to call this;
// downstream code should pass the context through and let FromContext do
// the right thing on log emission.
func WithTraceID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, traceIDKey, id)
}

// WithRunID returns a derived context carrying the current run's ID for
// every subsequent log emission.
func WithRunID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, runIDKey, id)
}

// WithExperimentID returns a derived context carrying the current
// experiment's ID for every subsequent log emission.
func WithExperimentID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, experimentIDKey, id)
}

// TraceIDFromContext extracts the trace ID set by WithTraceID. Returns ""
// if no trace ID is on the context.
func TraceIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if v, ok := ctx.Value(traceIDKey).(string); ok {
		return v
	}
	return ""
}

// FromContext returns a child logger that has trace_id, run_id, and
// experiment_id attached as structured attributes for every record. Any
// of the three that is not on the context is simply omitted from the
// attribute list.
//
// Pass the root logger explicitly to avoid singleton state; callers are
// expected to hold one root logger constructed at startup and thread it
// through their constructors.
func FromContext(ctx context.Context, root *slog.Logger) *slog.Logger {
	if root == nil {
		root = slog.Default()
	}
	if ctx == nil {
		return root
	}

	attrs := make([]any, 0, 6)
	if v := TraceIDFromContext(ctx); v != "" {
		attrs = append(attrs, "trace_id", v)
	}
	if v, ok := ctx.Value(runIDKey).(string); ok && v != "" {
		attrs = append(attrs, "run_id", v)
	}
	if v, ok := ctx.Value(experimentIDKey).(string); ok && v != "" {
		attrs = append(attrs, "experiment_id", v)
	}
	if len(attrs) == 0 {
		return root
	}
	return root.With(attrs...)
}
