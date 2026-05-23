package api

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mustafaselman/frameval/engine/test/support"
)

// newTestServer builds a fully wired http.Handler for handler tests that
// need real route resolution. It wires support.TmpStore as the store so
// all migrations (including app_settings defaults) are applied.
func newTestServer(t *testing.T) http.Handler {
	t.Helper()
	store := support.TmpStore(t)
	svc := &Service{store: store}
	return NewRouter(svc, slog.Default())
}

func TestGetLLMSettings_Defaults(t *testing.T) {
	srv := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/config/llm-settings", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", w.Code, w.Body.String())
	}
	var got map[string]any
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got["provider"] != "openrouter" {
		t.Errorf("provider = %v, want openrouter", got["provider"])
	}
	if got["enabled"] != true {
		t.Errorf("enabled = %v, want true", got["enabled"])
	}
	if _, ok := got["api_key_present"]; !ok {
		t.Errorf("missing api_key_present in response")
	}
}

func TestPutLLMSettings_Roundtrip(t *testing.T) {
	srv := newTestServer(t)

	body := map[string]any{"provider": "zai", "model": "glm-4.6", "enabled": false}
	buf, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPut, "/api/config/llm-settings", bytes.NewReader(buf))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("PUT status = %d, body = %s", w.Code, w.Body.String())
	}
	req2 := httptest.NewRequest(http.MethodGet, "/api/config/llm-settings", nil)
	w2 := httptest.NewRecorder()
	srv.ServeHTTP(w2, req2)
	var got map[string]any
	_ = json.NewDecoder(w2.Body).Decode(&got)
	if got["provider"] != "zai" {
		t.Errorf("after PUT, provider = %v, want zai", got["provider"])
	}
	if got["enabled"] != false {
		t.Errorf("after PUT, enabled = %v, want false", got["enabled"])
	}
}

func TestPutLLMSettings_RejectsUnknownProvider(t *testing.T) {
	srv := newTestServer(t)

	body := map[string]any{"provider": "totally-not-real", "model": "x", "enabled": true}
	buf, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPut, "/api/config/llm-settings", bytes.NewReader(buf))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body = %s", w.Code, w.Body.String())
	}
}
