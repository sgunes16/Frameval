package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRubrics_GetListIncludes5Seeded(t *testing.T) {
	srv := newTestServer(t)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/config/rubrics", nil))
	if w.Code != 200 { t.Fatalf("status %d body %s", w.Code, w.Body.String()) }
	var got []map[string]any
	_ = json.NewDecoder(w.Body).Decode(&got)
	if len(got) != 5 { t.Errorf("got %d, want 5", len(got)) }
}

func TestRubrics_PostCreatesNew(t *testing.T) {
	srv := newTestServer(t)
	body, _ := json.Marshal(map[string]string{
		"key": "security", "display_name": "Security",
		"prompt": "score security: input validation, secrets, dep risks. " + strings.Repeat("x", 100),
	})
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/api/config/rubrics", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	srv.ServeHTTP(w, r)
	if w.Code != 201 { t.Fatalf("status %d body %s", w.Code, w.Body.String()) }
}

func TestRubrics_PostDuplicateReturns409(t *testing.T) {
	srv := newTestServer(t)
	body, _ := json.Marshal(map[string]string{
		"key": "correctness", "display_name": "x",
		"prompt": strings.Repeat("p", 100),
	})
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/api/config/rubrics", bytes.NewReader(body))
	srv.ServeHTTP(w, r)
	if w.Code != 409 { t.Errorf("want 409, got %d", w.Code) }
}

func TestRubrics_DeleteBuiltinReturns400(t *testing.T) {
	srv := newTestServer(t)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, httptest.NewRequest(http.MethodDelete, "/api/config/rubrics/correctness", nil))
	if w.Code != 400 { t.Errorf("want 400, got %d", w.Code) }
}

func TestRubrics_DeleteCustomReturns204(t *testing.T) {
	srv := newTestServer(t)
	// First POST a custom rubric.
	body, _ := json.Marshal(map[string]string{
		"key": "tmp", "display_name": "Tmp", "prompt": strings.Repeat("p", 100),
	})
	r := httptest.NewRequest(http.MethodPost, "/api/config/rubrics", bytes.NewReader(body))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, r)
	if w.Code != 201 { t.Fatalf("create failed: %d %s", w.Code, w.Body.String()) }
	// Then DELETE.
	w = httptest.NewRecorder()
	srv.ServeHTTP(w, httptest.NewRequest(http.MethodDelete, "/api/config/rubrics/tmp", nil))
	if w.Code != 204 { t.Errorf("want 204, got %d", w.Code) }
}

func TestRubrics_PostInvalidKeyReturns400(t *testing.T) {
	srv := newTestServer(t)
	body, _ := json.Marshal(map[string]string{
		"key": "Bad-Key!", "display_name": "x", "prompt": strings.Repeat("p", 100),
	})
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/api/config/rubrics", bytes.NewReader(body))
	srv.ServeHTTP(w, r)
	if w.Code != 400 { t.Errorf("want 400, got %d", w.Code) }
}
