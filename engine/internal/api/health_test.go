package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealth_ReturnsSubcomponentMap(t *testing.T) {
	// The health handler builds its response from primitives passed in via
	// the Service constructor (in production those are wired through
	// main.go). The test calls the route handler directly with a stub
	// service to assert the response shape and not the wiring.
	svc := &Service{}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)

	svc.GetHealth(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d", rec.Code)
	}

	var body struct {
		OK         bool                       `json:"ok"`
		Version    string                     `json:"version"`
		Components map[string]json.RawMessage `json:"components"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("response not JSON: %v (raw=%q)", err, rec.Body.String())
	}
	if !body.OK {
		t.Errorf("ok should be true; got %v", body.OK)
	}
	if body.Version == "" {
		t.Errorf("version should be non-empty")
	}
	for _, name := range []string{"db", "docker", "grader", "queue"} {
		if _, present := body.Components[name]; !present {
			t.Errorf("components.%s missing; raw=%s", name, rec.Body.String())
		}
	}
}
