package api

import (
	"encoding/json"
	"expvar"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMetricsEndpoint_ExposesExpvar(t *testing.T) {
	// Register a probe expvar so the test asserts the route plumbing
	// rather than the specific set of counters that exist today. Probe
	// is registered with a unique name to avoid colliding with the
	// engine's real counters across multiple test runs in one binary.
	const probeName = "frameval_metrics_test_probe"
	var probe *expvar.Int
	if v := expvar.Get(probeName); v != nil {
		probe = v.(*expvar.Int)
	} else {
		probe = expvar.NewInt(probeName)
	}
	probe.Set(42)

	router := NewRouter(&Service{}, slog.Default())

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/debug/vars", nil)
	router.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("status: got %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "application/json") {
		t.Errorf("Content-Type: got %q", ct)
	}

	// Body must be parseable JSON and must include our probe key.
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("expvar response not JSON: %v (raw=%q)", err, rec.Body.String()[:200])
	}
	if v, ok := body[probeName]; !ok {
		t.Errorf("probe %s missing from /debug/vars output", probeName)
	} else if f, ok := v.(float64); !ok || f != 42 {
		t.Errorf("probe value: got %v", v)
	}
}
