package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/mustafaselman/frameval/engine/internal/diagnostic"
	"github.com/mustafaselman/frameval/engine/test/support"
)

// TestGetExperimentAnchors_EmptyBundleForNeverComputed pins the
// "no anchors yet" contract: when the experiment exists but its
// orchestrator hasn't refreshed anchors yet, the handler returns
// 200 with the canonical empty-bundle shape ({}) so the frontend
// can render "still building" without distinguishing 404 from a
// real empty response.
func TestGetExperimentAnchors_EmptyBundleForNeverComputed(t *testing.T) {
	store := support.TmpStore(t)
	svc := &Service{store: store}

	ctx := context.Background()
	expID := "exp-anchors-empty"
	if _, err := store.DB.ExecContext(ctx, `INSERT INTO tasks (id, name, description, category, complexity_score, codebase_type, task_prompt) VALUES ('t1', 'Test', 'desc', 'greenfield', 1.0, 'fresh', 'do it')`); err != nil {
		t.Fatalf("seed task: %v", err)
	}
	if _, err := store.DB.ExecContext(ctx, `
		INSERT INTO experiments (id, name, status, task_id, workspace_source_type, model, agent_cli, execution_mode, runs_per_variant, temperature, timeout_seconds, max_concurrent, composite_weights_json)
		VALUES (?, 'test', 'draft', 't1', 'task-builtin', 'claude', 'claude', 'cli', 1, 0, 600, 1, '{}')
	`, expID); err != nil {
		t.Fatalf("seed: %v", err)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/experiments/"+expID+"/anchors", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", expID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	svc.GetExperimentAnchors(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, body=%s", rec.Code, rec.Body.String())
	}

	var bundle diagnostic.AnchorBundle
	if err := json.Unmarshal(rec.Body.Bytes(), &bundle); err != nil {
		t.Fatalf("response not JSON: %v (raw=%q)", err, rec.Body.String())
	}
	if len(bundle.Runs) != 0 {
		t.Fatalf("expected 0 runs in empty bundle, got %d", len(bundle.Runs))
	}
}

// TestGetExperimentAnchors_ReturnsPersistedBundle pins the round-
// trip: the handler returns exactly what SetExperimentAnchors wrote.
func TestGetExperimentAnchors_ReturnsPersistedBundle(t *testing.T) {
	store := support.TmpStore(t)
	svc := &Service{store: store}

	ctx := context.Background()
	expID := "exp-anchors-roundtrip"
	if _, err := store.DB.ExecContext(ctx, `INSERT INTO tasks (id, name, description, category, complexity_score, codebase_type, task_prompt) VALUES ('t1', 'Test', 'desc', 'greenfield', 1.0, 'fresh', 'do it')`); err != nil {
		t.Fatalf("seed task: %v", err)
	}
	if _, err := store.DB.ExecContext(ctx, `
		INSERT INTO experiments (id, name, status, task_id, workspace_source_type, model, agent_cli, execution_mode, runs_per_variant, temperature, timeout_seconds, max_concurrent, composite_weights_json)
		VALUES (?, 'test', 'draft', 't1', 'task-builtin', 'claude', 'claude', 'cli', 1, 0, 600, 1, '{}')
	`, expID); err != nil {
		t.Fatalf("seed: %v", err)
	}

	bundle := diagnostic.AnchorBundle{
		Runs: []diagnostic.RunAnchors{
			{
				RunID: "r1",
				Anchors: []diagnostic.Anchor{
					{Key: "Edit|src/main.go", TurnIndex: 3, ParentTurnIndex: 3},
				},
			},
		},
	}
	payload, _ := json.Marshal(bundle)
	if err := store.SetExperimentAnchors(ctx, expID, string(payload)); err != nil {
		t.Fatalf("persist: %v", err)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/experiments/"+expID+"/anchors", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", expID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	svc.GetExperimentAnchors(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, body=%s", rec.Code, rec.Body.String())
	}

	var got diagnostic.AnchorBundle
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("response not JSON: %v (raw=%q)", err, rec.Body.String())
	}
	if len(got.Runs) != 1 || got.Runs[0].RunID != "r1" {
		t.Fatalf("unexpected bundle: %+v", got)
	}
	if got.Runs[0].Anchors[0].Key != "Edit|src/main.go" {
		t.Fatalf("unexpected anchor key: %q", got.Runs[0].Anchors[0].Key)
	}
}

// TestGetExperimentAnchors_MissingExperimentReturns404 pins the
// not-found path so the frontend can distinguish a typo'd ID from
// a row that simply hasn't computed anchors yet.
func TestGetExperimentAnchors_MissingExperimentReturns404(t *testing.T) {
	store := support.TmpStore(t)
	svc := &Service{store: store}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/experiments/does-not-exist/anchors", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "does-not-exist")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	svc.GetExperimentAnchors(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}
