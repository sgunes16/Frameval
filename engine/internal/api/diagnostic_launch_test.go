package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mustafaselman/frameval/engine/internal/models"
	"github.com/mustafaselman/frameval/engine/test/support"
)

func newLaunchTestService(t *testing.T) *Service {
	t.Helper()
	store := support.TmpStore(t)
	if _, err := store.DB.ExecContext(context.Background(), `
		INSERT INTO tasks (id, name, description, category, complexity_score, codebase_type, task_prompt)
		VALUES ('t-launch', 'Launch task', 'desc', 'greenfield', 1.0, 'fresh', 'do it')
	`); err != nil {
		t.Fatalf("seed task: %v", err)
	}
	return &Service{
		store:        store,
		harnesses:    support.NewStaticHarnessRegistry("bare", "ralph"),
		executors:    support.NewStaticExecutorRegistry("opencode"),
		orchestrator: support.NewNoopOrchestrator(),
	}
}

func TestLaunchDiagnosticAcceptsBatchPassThrough(t *testing.T) {
	svc := newLaunchTestService(t)

	body, _ := json.Marshal(map[string]any{
		"task_id":     "t-launch",
		"executor_id": "opencode",
		"harness_ids": []string{"bare"},
		"model":       "anything",
		"batch_id":    "batch-xyz",
		"batch_label": "Suite from CLI",
	})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/diagnostic/launch", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	svc.LaunchDiagnostic(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status: got %d body=%s", rec.Code, rec.Body.String())
	}

	var resp LaunchDiagnosticResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v body=%s", err, rec.Body.String())
	}
	exp, err := svc.store.GetExperiment(context.Background(), resp.ExperimentID)
	if err != nil {
		t.Fatalf("fetch experiment: %v", err)
	}
	if exp.BatchID != "batch-xyz" {
		t.Fatalf("BatchID: got %q want %q", exp.BatchID, "batch-xyz")
	}
	if exp.BatchLabel != "Suite from CLI" {
		t.Fatalf("BatchLabel: got %q want %q", exp.BatchLabel, "Suite from CLI")
	}
	_ = models.Experiment{}
}
