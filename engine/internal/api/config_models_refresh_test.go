package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mustafaselman/frameval/engine/internal/models"
	"github.com/mustafaselman/frameval/engine/test/support"
)

type modelsRunnerStub struct {
	output string
}

func (m *modelsRunnerStub) RunShell(_ context.Context, _ string, _ map[string]string, _ string) (string, error) {
	return m.output, nil
}

func TestRefreshOpenCodeModelsSyncsCatalog(t *testing.T) {
	store := support.TmpStore(t)
	runner := &modelsRunnerStub{
		output: "opencode/foo-free\nopencode/bar-free\n",
	}
	svc := &Service{store: store, modelsRunner: runner}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/config/models/refresh", nil)
	svc.RefreshOpenCodeModels(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d body=%s", rec.Code, rec.Body.String())
	}
	var got []models.ModelConfig
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	ocCount := 0
	for _, m := range got {
		if m.Provider == "opencode" {
			ocCount++
		}
	}
	if ocCount != 2 {
		t.Errorf("opencode count: got %d want 2 (full list: %v)", ocCount, got)
	}
}

func TestRefreshOpenCodeModelsWithoutRunnerJustReturnsCurrent(t *testing.T) {
	store := support.TmpStore(t)
	if err := store.SeedModelConfigs(context.Background()); err != nil {
		t.Fatalf("seed defaults: %v", err)
	}
	svc := &Service{store: store, modelsRunner: nil}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/config/models/refresh", nil)
	svc.RefreshOpenCodeModels(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d body=%s", rec.Code, rec.Body.String())
	}
	var got []models.ModelConfig
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got) == 0 {
		t.Error("expected default models in response when runner is nil")
	}
}
