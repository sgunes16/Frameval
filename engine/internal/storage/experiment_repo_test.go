package storage_test

import (
	"context"
	"database/sql"
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

func TestVariantHarnessConfigNilStoresAsNull(t *testing.T) {
	store := support.TmpStore(t)
	seedTaskForExperimentTest(t, store, "task-hc-null")

	exp, err := store.CreateExperiment(context.Background(), models.ExperimentRequest{
		Name:           "no-config",
		TaskID:         "task-hc-null",
		Model:          "claude",
		AgentCLI:       "claude",
		RunsPerVariant: 1,
		Variants: []models.VariantRequest{
			{Name: "bare", HarnessID: "bare"},
		},
	})
	if err != nil {
		t.Fatalf("CreateExperiment: %v", err)
	}
	// Assert the raw column is SQL NULL, not the literal string "null".
	var raw sql.NullString
	row := store.DB.QueryRowContext(context.Background(), `SELECT harness_config_json FROM variants WHERE id = ?`, exp.Variants[0].ID)
	if err := row.Scan(&raw); err != nil {
		t.Fatalf("raw scan: %v", err)
	}
	if raw.Valid {
		t.Errorf("harness_config_json: expected NULL, got valid string %q", raw.String)
	}
	// And the model round-trip still produces a nil map.
	if exp.Variants[0].HarnessConfig != nil {
		t.Errorf("HarnessConfig: expected nil, got %v", exp.Variants[0].HarnessConfig)
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

func TestListExperimentsIncludesVariants(t *testing.T) {
	store := support.TmpStore(t)
	seedTaskForExperimentTest(t, store, "task-listvar")

	_, err := store.CreateExperiment(context.Background(), models.ExperimentRequest{
		Name:           "with-variants",
		TaskID:         "task-listvar",
		Model:          "m",
		AgentCLI:       "opencode",
		RunsPerVariant: 1,
		Variants: []models.VariantRequest{
			{Name: "bare", HarnessID: "bare", Ordering: 0},
			{Name: "speckit/canonical", HarnessID: "speckit", Ordering: 1},
			{Name: "speckit/lite", HarnessID: "speckit", Ordering: 2},
		},
	})
	if err != nil {
		t.Fatalf("CreateExperiment: %v", err)
	}

	listed, err := store.ListExperiments(context.Background())
	if err != nil {
		t.Fatalf("ListExperiments: %v", err)
	}
	var found *models.Experiment
	for i := range listed {
		if listed[i].Name == "with-variants" {
			found = &listed[i]
			break
		}
	}
	if found == nil {
		t.Fatal("experiment not in list")
	}
	if len(found.Variants) != 3 {
		t.Fatalf("variant count in list: got %d want 3", len(found.Variants))
	}
	// Ordered by `ordering`, names preserved.
	if found.Variants[0].Name != "bare" || found.Variants[2].Name != "speckit/lite" {
		t.Errorf("variant order/names wrong: %+v", found.Variants)
	}
}

func TestCancelPendingRunsLeavesStartedRunsAlone(t *testing.T) {
	store := support.TmpStore(t)
	seedTaskForExperimentTest(t, store, "task-cancel")
	ctx := context.Background()

	exp, err := store.CreateExperiment(ctx, models.ExperimentRequest{
		Name: "cancel-test", TaskID: "task-cancel", Model: "claude", AgentCLI: "claude", RunsPerVariant: 1,
		Variants: []models.VariantRequest{
			{Name: "a", HarnessID: "bare"},
			{Name: "b", HarnessID: "bare"},
			{Name: "c", HarnessID: "bare"},
		},
	})
	if err != nil {
		t.Fatalf("CreateExperiment: %v", err)
	}
	if err := store.EnsureRunsForExperiment(ctx, exp.ID); err != nil {
		t.Fatalf("EnsureRunsForExperiment: %v", err)
	}
	runs, err := store.ListRunsByExperiment(ctx, exp.ID)
	if err != nil || len(runs) != 3 {
		t.Fatalf("want 3 runs, got %d (err %v)", len(runs), err)
	}
	// One in-flight, one finished, one still pending.
	_ = store.UpdateRunStatus(ctx, runs[0].ID, "running", "")
	_ = store.UpdateRunStatus(ctx, runs[1].ID, "completed", "")

	if err := store.CancelPendingRuns(ctx, exp.ID); err != nil {
		t.Fatalf("CancelPendingRuns: %v", err)
	}

	after, _ := store.ListRunsByExperiment(ctx, exp.ID)
	got := map[string]string{}
	for _, r := range after {
		got[r.ID] = r.Status
	}
	if got[runs[0].ID] != "running" {
		t.Errorf("running run must be untouched, got %q", got[runs[0].ID])
	}
	if got[runs[1].ID] != "completed" {
		t.Errorf("completed run must be untouched, got %q", got[runs[1].ID])
	}
	if got[runs[2].ID] != "cancelled" {
		t.Errorf("pending run must be cancelled, got %q", got[runs[2].ID])
	}
}

func TestDeleteExperimentCascadesAndLeavesNoOrphans(t *testing.T) {
	store := support.TmpStore(t)
	seedTaskForExperimentTest(t, store, "task-del")
	ctx := context.Background()

	exp, err := store.CreateExperiment(ctx, models.ExperimentRequest{
		Name: "del-test", TaskID: "task-del", Model: "claude", AgentCLI: "claude", RunsPerVariant: 1,
		Variants: []models.VariantRequest{
			{Name: "a", HarnessID: "bare"},
			{Name: "b", HarnessID: "bare"},
		},
	})
	if err != nil {
		t.Fatalf("CreateExperiment: %v", err)
	}
	if err := store.EnsureRunsForExperiment(ctx, exp.ID); err != nil {
		t.Fatalf("EnsureRunsForExperiment: %v", err)
	}
	runs, err := store.ListRunsByExperiment(ctx, exp.ID)
	if err != nil || len(runs) == 0 {
		t.Fatalf("expected runs, got %d (err %v)", len(runs), err)
	}
	// A transcript hangs off a run — deleting the experiment must take it
	// too, not leave it dangling by run_id.
	if err := store.SaveTranscript(ctx, models.Transcript{ID: "tr-del-1", RunID: runs[0].ID, RawOutput: "x"}); err != nil {
		t.Fatalf("SaveTranscript: %v", err)
	}

	if err := store.DeleteExperiment(ctx, exp.ID); err != nil {
		t.Fatalf("DeleteExperiment: %v", err)
	}

	count := func(query, arg string) int {
		t.Helper()
		var n int
		if err := store.DB.QueryRowContext(ctx, query, arg).Scan(&n); err != nil {
			t.Fatalf("count %q: %v", query, err)
		}
		return n
	}
	if n := count(`SELECT count(*) FROM experiments WHERE id = ?`, exp.ID); n != 0 {
		t.Errorf("experiment row remains: %d", n)
	}
	if n := count(`SELECT count(*) FROM variants WHERE experiment_id = ?`, exp.ID); n != 0 {
		t.Errorf("orphan variants remain: %d", n)
	}
	if n := count(`SELECT count(*) FROM runs WHERE experiment_id = ?`, exp.ID); n != 0 {
		t.Errorf("orphan runs remain: %d", n)
	}
	if n := count(`SELECT count(*) FROM transcripts WHERE run_id = ?`, runs[0].ID); n != 0 {
		t.Errorf("orphan transcripts remain: %d", n)
	}
}
