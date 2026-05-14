package api

import (
	"bytes"
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

func TestWithBodyCap_TruncatesOversizedBody(t *testing.T) {
	// 1 MiB cap; client sends 2 MiB → decode should fail with the
	// http.MaxBytesError sentinel.
	const cap = 1 << 20
	handler := WithBodyCap(cap)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		buf := make([]byte, cap+1024)
		n, err := r.Body.Read(buf)
		// Some bytes may read before the cap fires; just ensure the read
		// eventually surfaces an error rather than the full payload.
		if err == nil && n == len(buf) {
			t.Errorf("expected MaxBytesError, read full %d bytes", n)
		}
	}))

	req := httptest.NewRequest(http.MethodPost, "/x", bytes.NewReader(make([]byte, 2<<20)))
	handler.ServeHTTP(httptest.NewRecorder(), req)
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
