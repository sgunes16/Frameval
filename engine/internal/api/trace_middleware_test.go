package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mustafaselman/frameval/engine/internal/logging"
)

func TestWithTraceID_GeneratesIDWhenHeaderAbsent(t *testing.T) {
	var captured string
	handler := WithTraceID(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		captured = logging.TraceIDFromContext(r.Context())
	}))

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if captured == "" {
		t.Fatal("handler should observe a non-empty trace_id on the request context")
	}
	if got := rec.Header().Get("X-Frameval-Trace"); got != captured {
		t.Errorf("response should echo the trace_id: header=%q ctx=%q", got, captured)
	}
}

func TestWithTraceID_HonorsIncomingHeader(t *testing.T) {
	var captured string
	handler := WithTraceID(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		captured = logging.TraceIDFromContext(r.Context())
	}))

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set("X-Frameval-Trace", "supplied-by-caller-7")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if captured != "supplied-by-caller-7" {
		t.Errorf("trace_id should mirror the inbound header, got %q", captured)
	}
	if got := rec.Header().Get("X-Frameval-Trace"); got != "supplied-by-caller-7" {
		t.Errorf("response should echo the inbound trace_id: got %q", got)
	}
}
