package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSpecKitCatalogHandlerReturnsAllEntries(t *testing.T) {
	svc := &Service{}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/harnesses/speckit/catalog", nil)
	svc.ListSpecKitCatalog(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d body=%s", rec.Code, rec.Body.String())
	}
	var got []SpecKitExtensionPublic
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v body=%s", err, rec.Body.String())
	}
	if len(got) != 6 {
		t.Errorf("entry count: got %d want 6", len(got))
	}
	if got[0].ID != "canonical" {
		t.Errorf("first id: got %q want canonical", got[0].ID)
	}
	// Stage clipping: the public shape carries name + slash_command + role,
	// NOT the full prompt template.
	if len(got[0].Stages) != 4 {
		t.Errorf("canonical stage count: got %d want 4", len(got[0].Stages))
	}
	if got[0].Stages[0].SlashCommand != "/speckit.specify" {
		t.Errorf("first stage slash: got %q", got[0].Stages[0].SlashCommand)
	}
}
