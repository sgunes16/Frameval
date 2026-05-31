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

func TestLaunchDiagnosticSuiteHappyPath(t *testing.T) {
	svc := newLaunchTestService(t)
	// Seed a second task so we can launch 2 in one batch.
	if _, err := svc.store.DB.ExecContext(context.Background(), `
		INSERT INTO tasks (id, name, description, category, complexity_score, codebase_type, task_prompt)
		VALUES ('t-launch-2', 'Second task', 'desc', 'greenfield', 1.0, 'fresh', 'do it')
	`); err != nil {
		t.Fatalf("seed second task: %v", err)
	}

	body, _ := json.Marshal(map[string]any{
		"task_ids":    []string{"t-launch", "t-launch-2"},
		"executor_id": "opencode",
		"harness_ids": []string{"bare"},
		"model":       "anything",
		"batch_label": "Happy path suite",
	})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/diagnostic/launch-suite", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	svc.LaunchDiagnosticSuite(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status: got %d body=%s", rec.Code, rec.Body.String())
	}

	var resp LaunchDiagnosticSuiteResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.BatchID == "" {
		t.Fatal("BatchID empty")
	}
	if len(resp.ExperimentIDs) != 2 {
		t.Fatalf("ExperimentIDs: got %d want 2", len(resp.ExperimentIDs))
	}
	if len(resp.Failures) != 0 {
		t.Fatalf("Failures: got %+v want none", resp.Failures)
	}

	// Both experiments share the batch_id and label.
	for _, eid := range resp.ExperimentIDs {
		exp, err := svc.store.GetExperiment(context.Background(), eid)
		if err != nil {
			t.Fatalf("fetch %s: %v", eid, err)
		}
		if exp.BatchID != resp.BatchID {
			t.Errorf("exp %s BatchID=%q want %q", eid, exp.BatchID, resp.BatchID)
		}
		if exp.BatchLabel != "Happy path suite" {
			t.Errorf("exp %s BatchLabel=%q want %q", eid, exp.BatchLabel, "Happy path suite")
		}
	}
}

func TestLaunchDiagnosticSuitePartialFailure(t *testing.T) {
	svc := newLaunchTestService(t)

	body, _ := json.Marshal(map[string]any{
		"task_ids":    []string{"t-launch", "does-not-exist"},
		"executor_id": "opencode",
		"harness_ids": []string{"bare"},
		"model":       "anything",
		"batch_label": "Partial",
	})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/diagnostic/launch-suite", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	svc.LaunchDiagnosticSuite(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status: got %d body=%s", rec.Code, rec.Body.String())
	}
	var resp LaunchDiagnosticSuiteResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.ExperimentIDs) != 1 {
		t.Fatalf("ExperimentIDs: got %d want 1", len(resp.ExperimentIDs))
	}
	if len(resp.Failures) != 1 {
		t.Fatalf("Failures: got %d want 1", len(resp.Failures))
	}
	if resp.Failures[0].TaskID != "does-not-exist" {
		t.Errorf("failure task_id: got %q want %q", resp.Failures[0].TaskID, "does-not-exist")
	}
	if resp.BatchID == "" {
		t.Fatal("BatchID empty")
	}
}

func TestLaunchDiagnosticSuiteRejectsEmptyTaskIDs(t *testing.T) {
	svc := newLaunchTestService(t)

	body, _ := json.Marshal(map[string]any{
		"task_ids":    []string{},
		"executor_id": "opencode",
		"harness_ids": []string{"bare"},
		"model":       "anything",
	})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/diagnostic/launch-suite", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	svc.LaunchDiagnosticSuite(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d body=%s want 400", rec.Code, rec.Body.String())
	}
}

// TestLaunchDiagnosticPersistsHarnessConfig verifies that harness_configs
// supplied on a launch request are persisted onto every spawned variant for
// both the single-task and multi-task (suite) endpoints.
func TestLaunchDiagnosticPersistsHarnessConfig(t *testing.T) {
	// Canonical wire shape — matches what the agent_instructions harness's
	// extractAgentInstructionsContent helper consumes (cfg[key].(map)["content"]).
	// A flat string at the top level would silently fail the harness at runtime.
	cfg := map[string]any{
		"agent_instructions": map[string]any{"content": "# rules\nbe concise"},
	}

	t.Run("single endpoint", func(t *testing.T) {
		svc := newLaunchTestService(t)

		body, _ := json.Marshal(map[string]any{
			"task_id":         "t-launch",
			"executor_id":     "opencode",
			"harness_ids":     []string{"bare"},
			"model":           "anything",
			"harness_configs": cfg,
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
			t.Fatalf("decode: %v", err)
		}
		exp, err := svc.store.GetExperiment(context.Background(), resp.ExperimentID)
		if err != nil {
			t.Fatalf("fetch experiment: %v", err)
		}
		if len(exp.Variants) == 0 {
			t.Fatal("no variants returned")
		}
		sub, ok := exp.Variants[0].HarnessConfig["agent_instructions"].(map[string]any)
		if !ok {
			t.Fatalf("HarnessConfig.agent_instructions: got %v want map", exp.Variants[0].HarnessConfig)
		}
		if got, _ := sub["content"].(string); got != "# rules\nbe concise" {
			t.Fatalf("HarnessConfig.agent_instructions.content: got %q want %q", got, "# rules\nbe concise")
		}
	})

	t.Run("suite endpoint", func(t *testing.T) {
		svc := newLaunchTestService(t)

		body, _ := json.Marshal(map[string]any{
			"task_ids":        []string{"t-launch"},
			"executor_id":     "opencode",
			"harness_ids":     []string{"bare"},
			"model":           "anything",
			"harness_configs": cfg,
		})
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/diagnostic/launch-suite", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		svc.LaunchDiagnosticSuite(rec, req)
		if rec.Code != http.StatusAccepted {
			t.Fatalf("status: got %d body=%s", rec.Code, rec.Body.String())
		}

		var resp LaunchDiagnosticSuiteResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if len(resp.ExperimentIDs) == 0 {
			t.Fatal("no experiment IDs returned")
		}
		exp, err := svc.store.GetExperiment(context.Background(), resp.ExperimentIDs[0])
		if err != nil {
			t.Fatalf("fetch experiment: %v", err)
		}
		if len(exp.Variants) == 0 {
			t.Fatal("no variants returned")
		}
		sub, ok := exp.Variants[0].HarnessConfig["agent_instructions"].(map[string]any)
		if !ok {
			t.Fatalf("HarnessConfig.agent_instructions: got %v want map", exp.Variants[0].HarnessConfig)
		}
		if got, _ := sub["content"].(string); got != "# rules\nbe concise" {
			t.Fatalf("HarnessConfig.agent_instructions.content: got %q want %q", got, "# rules\nbe concise")
		}
	})
}
