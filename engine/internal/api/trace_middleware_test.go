package api

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
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

func TestRequestLogger_CapturesStatusCode(t *testing.T) {
	cases := []struct {
		name       string
		handler    http.HandlerFunc
		wantStatus int
	}{
		{
			name: "200_default",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte("ok"))
			},
			wantStatus: 200,
		},
		{
			name: "201_created",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusCreated)
			},
			wantStatus: 201,
		},
		{
			name: "500_explicit",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			},
			wantStatus: 500,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			buf := &bytes.Buffer{}
			logger := slog.New(slog.NewJSONHandler(buf, &slog.HandlerOptions{Level: slog.LevelInfo}))

			wrapped := requestLogger(logger)(tc.handler)

			req := httptest.NewRequest(http.MethodGet, "/x", nil)
			rec := httptest.NewRecorder()
			wrapped.ServeHTTP(rec, req)

			// One JSON log line should have been emitted.
			line := strings.TrimSpace(buf.String())
			if line == "" {
				t.Fatal("requestLogger did not emit a log line")
			}
			var record map[string]any
			if err := json.Unmarshal([]byte(line), &record); err != nil {
				t.Fatalf("log line not JSON: %v (raw=%q)", err, line)
			}
			gotStatus, ok := record["status"].(float64)
			if !ok {
				t.Fatalf("status field missing or wrong type; record=%v", record)
			}
			if int(gotStatus) != tc.wantStatus {
				t.Errorf("status: want %d, got %d", tc.wantStatus, int(gotStatus))
			}
		})
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
