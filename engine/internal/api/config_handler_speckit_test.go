package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetSpecKitSettings_Default(t *testing.T) {
	srv := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/config/speckit-settings", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", w.Code, w.Body.String())
	}
	var got map[string]any
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got["version"] != "v0.9.1" {
		t.Errorf("version = %v, want v0.9.1", got["version"])
	}
}

func TestPutSpecKitSettings_RoundTrip(t *testing.T) {
	srv := newTestServer(t)
	body, _ := json.Marshal(map[string]string{"version": "v0.8.0"})
	req := httptest.NewRequest(http.MethodPut, "/api/config/speckit-settings", bytes.NewReader(body))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("PUT status = %d, want 200; body = %s", w.Code, w.Body.String())
	}
	var put map[string]any
	_ = json.NewDecoder(w.Body).Decode(&put)
	if put["version"] != "v0.8.0" {
		t.Errorf("PUT echoed version = %v, want v0.8.0", put["version"])
	}
	// GET reflects the saved value.
	req2 := httptest.NewRequest(http.MethodGet, "/api/config/speckit-settings", nil)
	w2 := httptest.NewRecorder()
	srv.ServeHTTP(w2, req2)
	var got map[string]any
	_ = json.NewDecoder(w2.Body).Decode(&got)
	if got["version"] != "v0.8.0" {
		t.Errorf("GET after PUT version = %v, want v0.8.0", got["version"])
	}
}

func TestPutSpecKitSettings_EmptyVersionRejected(t *testing.T) {
	srv := newTestServer(t)
	body, _ := json.Marshal(map[string]string{"version": "   "})
	req := httptest.NewRequest(http.MethodPut, "/api/config/speckit-settings", bytes.NewReader(body))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body = %s", w.Code, w.Body.String())
	}
}
