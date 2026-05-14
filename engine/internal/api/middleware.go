package api

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/cors"
	"github.com/google/uuid"

	"github.com/mustafaselman/frameval/engine/internal/logging"
)

// traceHeader is the request/response header that carries the trace ID.
// Clients can set it on outgoing requests to thread their own ID through
// the engine; we echo whatever lands (or a fresh ID) on the response so
// the client can correlate.
const traceHeader = "X-Frameval-Trace"

// WithTraceID is the HTTP middleware that injects a trace_id into the
// request context and echoes it on the response. Subsequent handlers
// (and anything downstream that calls logging.FromContext) get
// structured log records tagged with the trace_id automatically.
//
// If the request arrives with an X-Frameval-Trace header the middleware
// trusts it (the caller is asserting "this work is part of trace X").
// Otherwise a fresh UUID is generated.
func WithTraceID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get(traceHeader)
		if id == "" {
			id = uuid.NewString()
		}
		w.Header().Set(traceHeader, id)
		next.ServeHTTP(w, r.WithContext(logging.WithTraceID(r.Context(), id)))
	})
}

// requestLogger logs every HTTP request with method, path, duration, and
// the request's trace_id. Takes the engine's root *slog.Logger so we can
// avoid singleton state.
func requestLogger(root *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			started := time.Now()
			next.ServeHTTP(w, r)
			logging.FromContext(r.Context(), root).Info(
				"http_request",
				"method", r.Method,
				"path", r.URL.Path,
				"duration_ms", time.Since(started).Milliseconds(),
			)
		})
	}
}

func corsMiddleware() func(http.Handler) http.Handler {
	return cors.Handler(cors.Options{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders: []string{"Accept", "Authorization", "Content-Type"},
	})
}
