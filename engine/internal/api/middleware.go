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
			// UUIDv7 is time-sortable, so log consumers can order records
			// chronologically by trace_id when timestamps are coarse.
			generated, err := uuid.NewV7()
			if err != nil {
				// NewV7 only errors on a clock-source failure; fall back to
				// random v4 so requests still get a stable ID.
				generated = uuid.New()
			}
			id = generated.String()
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

// WithBodyCap limits inbound request bodies to maxBytes. Reads past the
// cap return *http.MaxBytesError, which handlers' JSON decoders surface
// as a normal decode error — renderError then maps it to a 400 with the
// ErrCodeBadRequest code. Prevents a malicious or accidental large POST
// from filling the engine's heap.
func WithBodyCap(maxBytes int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Body != nil {
				r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			}
			next.ServeHTTP(w, r)
		})
	}
}

func corsMiddleware() func(http.Handler) http.Handler {
	return cors.Handler(cors.Options{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders: []string{"Accept", "Authorization", "Content-Type", traceHeader},
		// ExposedHeaders lets browser JS read the response header (otherwise
		// the browser strips X-Frameval-Trace from the JS-visible Headers
		// map). Without this the "echo on response" design goal in
		// WithTraceID is invisible to cross-origin SPA clients.
		ExposedHeaders: []string{traceHeader},
	})
}
