package storage_test

import (
	"context"
	"testing"

	"github.com/mustafaselman/frameval/engine/internal/models"
	"github.com/mustafaselman/frameval/engine/internal/storage"
	"github.com/mustafaselman/frameval/engine/test/support"
)

func seedTaskForExperimentTest(t *testing.T, store *storage.Store, taskID string) {
	t.Helper()
	if _, err := store.DB.ExecContext(context.Background(), `
		INSERT INTO tasks (id, name, description, category, complexity_score, codebase_type, task_prompt)
		VALUES (?, 'Test task', 'desc', 'greenfield', 1.0, 'fresh', 'do it')
	`, taskID); err != nil {
		t.Fatalf("seed task: %v", err)
	}
}

func TestCreateExperimentPersistsBatchFields(t *testing.T) {
	store := support.TmpStore(t)
	seedTaskForExperimentTest(t, store, "task-batch-1")

	created, err := store.CreateExperiment(context.Background(), models.ExperimentRequest{
		Name:           "batched",
		TaskID:         "task-batch-1",
		Model:          "claude",
		AgentCLI:       "claude",
		RunsPerVariant: 1,
		BatchID:        "batch-abc",
		BatchLabel:     "Calibration suite v1",
	})
	if err != nil {
		t.Fatalf("CreateExperiment: %v", err)
	}
	if created.BatchID != "batch-abc" {
		t.Fatalf("batch_id round-trip: got %q want %q", created.BatchID, "batch-abc")
	}
	if created.BatchLabel != "Calibration suite v1" {
		t.Fatalf("batch_label round-trip: got %q want %q", created.BatchLabel, "Calibration suite v1")
	}

	listed, err := store.ListExperiments(context.Background())
	if err != nil {
		t.Fatalf("ListExperiments: %v", err)
	}
	if len(listed) != 1 {
		t.Fatalf("list size: got %d want 1", len(listed))
	}
	if listed[0].BatchID != "batch-abc" || listed[0].BatchLabel != "Calibration suite v1" {
		t.Fatalf("list batch fields: got id=%q label=%q", listed[0].BatchID, listed[0].BatchLabel)
	}
}

func TestCreateExperimentNullBatchFields(t *testing.T) {
	store := support.TmpStore(t)
	seedTaskForExperimentTest(t, store, "task-batch-2")

	created, err := store.CreateExperiment(context.Background(), models.ExperimentRequest{
		Name:           "unbatched",
		TaskID:         "task-batch-2",
		Model:          "claude",
		AgentCLI:       "claude",
		RunsPerVariant: 1,
	})
	if err != nil {
		t.Fatalf("CreateExperiment: %v", err)
	}
	if created.BatchID != "" {
		t.Fatalf("batch_id should be empty: got %q", created.BatchID)
	}
	if created.BatchLabel != "" {
		t.Fatalf("batch_label should be empty: got %q", created.BatchLabel)
	}
}

func TestVariantHarnessConfigRoundTrip(t *testing.T) {
	store := support.TmpStore(t)
	seedTaskForExperimentTest(t, store, "task-hc-1")

	exp, err := store.CreateExperiment(context.Background(), models.ExperimentRequest{
		Name:           "harness-config",
		TaskID:         "task-hc-1",
		Model:          "claude",
		AgentCLI:       "claude",
		RunsPerVariant: 1,
		Variants: []models.VariantRequest{
			{
				Name:      "bare",
				HarnessID: "bare",
				HarnessConfig: map[string]any{
					"agent_instructions": map[string]any{"content": "rule one"},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("CreateExperiment: %v", err)
	}
	if len(exp.Variants) != 1 {
		t.Fatalf("variant count: got %d", len(exp.Variants))
	}
	gotCfg, _ := exp.Variants[0].HarnessConfig["agent_instructions"].(map[string]any)
	if got, _ := gotCfg["content"].(string); got != "rule one" {
		t.Fatalf("HarnessConfig round-trip: got %q want %q", got, "rule one")
	}
}
