package storage

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/mustafaselman/frameval/engine/pkg/diagnostic"
)

// newTestStore opens a fresh on-disk SQLite + applies all migrations.
// On-disk (not :memory:) so multiple connections see the same schema —
// the migration runner uses SetMaxOpenConns(1) but the tests connect
// through the same Store.
func newTestStore(t *testing.T) *Store {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "diagnostic_test.db")
	store, err := Open(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() {
		_ = store.Close()
		_ = os.Remove(dbPath)
	})
	return store
}

// seedRun makes a minimal tasks → experiments → variants → runs chain so
// the diagnostic table's FK to runs is satisfiable. Order matters: tasks
// must exist before experiments reference them.
//
// All three tests share the same task/experiment/variant IDs but distinct
// run IDs — the INSERT OR IGNORE clauses make the parent inserts safe to
// repeat across tests using the same store instance.
func seedRun(t *testing.T, store *Store, runID string) {
	t.Helper()
	ctx := context.Background()
	if _, err := store.DB.ExecContext(ctx,
		`INSERT OR IGNORE INTO tasks (id, name, description, category, complexity_score, codebase_type, task_prompt)
		 VALUES ('task-1','t','test task','greenfield',1.0,'python','x')`); err != nil {
		t.Fatalf("insert task: %v", err)
	}
	if _, err := store.DB.ExecContext(ctx,
		`INSERT OR IGNORE INTO experiments (id, name, status, task_id, model, agent_cli, runs_per_variant)
		 VALUES ('exp-1','test','draft','task-1','m','aider', 5)`); err != nil {
		t.Fatalf("insert experiment: %v", err)
	}
	if _, err := store.DB.ExecContext(ctx,
		`INSERT OR IGNORE INTO variants (id, experiment_id, name) VALUES ('var-1','exp-1','v1')`); err != nil {
		t.Fatalf("insert variant: %v", err)
	}
	if _, err := store.DB.ExecContext(ctx,
		`INSERT INTO runs (id, experiment_id, variant_id, run_number, status)
		 VALUES (?, 'exp-1', 'var-1', 0, 'completed')`, runID); err != nil {
		t.Fatalf("insert run: %v", err)
	}
}

func TestSaveAndGetDiagnostic_FullRoundTrip(t *testing.T) {
	store := newTestStore(t)
	seedRun(t, store, "run-1")

	failureLabel := &diagnostic.FailureClassification{
		Primary:    diagnostic.FailureHalAPI,
		Secondary:  []diagnostic.FailureCode{diagnostic.FailureDepMiss},
		Confidence: 0.82,
		Rationale:  "agent imported nonexistent attribute",
		Evidence: []diagnostic.EvidenceSpan{
			{Code: diagnostic.FailureHalAPI, Quote: "AttributeError", TurnIndex: 3},
		},
	}
	rec := DiagnosticRecord{
		RunID: "run-1",
		Fingerprint: diagnostic.Fingerprint{
			PlanningDepth: 0.5, ToolCallDiversity: 0.7,
		},
		Symptoms: diagnostic.Symptoms{
			TestsPassed: 2, TestsFailed: 1, TestsTotal: 3,
			DeclaredCompletion: true,
		},
		Recovery: diagnostic.RecoveryProfile{
			ErrorAcknowledgmentRate: 0.5,
			SilentSkipCount:         1,
		},
		FailureLabel:        failureLabel,
		ClassifierModel:     "claude-haiku-4-5-20251001",
		ClassifierLatencyMs: 1234,
	}
	if err := store.SaveDiagnostic(context.Background(), rec); err != nil {
		t.Fatalf("SaveDiagnostic: %v", err)
	}
	got, err := store.GetDiagnosticByRun(context.Background(), "run-1")
	if err != nil {
		t.Fatalf("GetDiagnosticByRun: %v", err)
	}
	if got.Fingerprint.PlanningDepth != 0.5 || got.Fingerprint.ToolCallDiversity != 0.7 {
		t.Errorf("fingerprint roundtrip mismatch: %+v", got.Fingerprint)
	}
	if got.Symptoms.TestsPassed != 2 || !got.Symptoms.DeclaredCompletion {
		t.Errorf("symptoms roundtrip mismatch: %+v", got.Symptoms)
	}
	if got.Recovery.ErrorAcknowledgmentRate != 0.5 || got.Recovery.SilentSkipCount != 1 {
		t.Errorf("recovery roundtrip mismatch: %+v", got.Recovery)
	}
	if got.FailureLabel == nil {
		t.Fatal("failure label dropped on round-trip")
	}
	if got.FailureLabel.Primary != diagnostic.FailureHalAPI || got.FailureLabel.Confidence != 0.82 {
		t.Errorf("failure label fields mismatch: %+v", got.FailureLabel)
	}
	if got.ClassifierModel != "claude-haiku-4-5-20251001" || got.ClassifierLatencyMs != 1234 {
		t.Errorf("classifier metadata mismatch: model=%q latency=%d", got.ClassifierModel, got.ClassifierLatencyMs)
	}
}

func TestSaveDiagnostic_NilFailureLabelPersistedAsNull(t *testing.T) {
	store := newTestStore(t)
	seedRun(t, store, "run-noclass")
	rec := DiagnosticRecord{RunID: "run-noclass"}
	if err := store.SaveDiagnostic(context.Background(), rec); err != nil {
		t.Fatalf("SaveDiagnostic: %v", err)
	}
	got, err := store.GetDiagnosticByRun(context.Background(), "run-noclass")
	if err != nil {
		t.Fatalf("GetDiagnosticByRun: %v", err)
	}
	if got.FailureLabel != nil {
		t.Errorf("expected nil FailureLabel, got %+v", got.FailureLabel)
	}
}

func TestSaveDiagnostic_Idempotent(t *testing.T) {
	store := newTestStore(t)
	seedRun(t, store, "run-rerun")

	rec := DiagnosticRecord{RunID: "run-rerun"}
	if err := store.SaveDiagnostic(context.Background(), rec); err != nil {
		t.Fatalf("first SaveDiagnostic: %v", err)
	}
	// Re-grade with different fingerprint → second save replaces first.
	rec.Fingerprint.PlanningDepth = 0.9
	if err := store.SaveDiagnostic(context.Background(), rec); err != nil {
		t.Fatalf("second SaveDiagnostic: %v", err)
	}
	got, err := store.GetDiagnosticByRun(context.Background(), "run-rerun")
	if err != nil {
		t.Fatalf("GetDiagnosticByRun: %v", err)
	}
	if got.Fingerprint.PlanningDepth != 0.9 {
		t.Errorf("re-grade did not overwrite; got %v", got.Fingerprint.PlanningDepth)
	}
}

func TestGetDiagnostic_MissingRunReturnsErrNoRows(t *testing.T) {
	store := newTestStore(t)
	_, err := store.GetDiagnosticByRun(context.Background(), "no-such-run")
	if !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("expected ErrNoRows, got %v", err)
	}
}
