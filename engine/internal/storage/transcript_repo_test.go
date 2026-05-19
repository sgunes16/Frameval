package storage

import (
	"context"
	"testing"

	"github.com/mustafaselman/frameval/engine/internal/models"
)

// seedTranscript inserts a transcript for the given run. seedRun must
// have been called first (it sets up the FK chain). Used by ListTurns
// and ListTurnsByExperiment tests to avoid duplicating the boilerplate.
//
// Uses models.ParsedTurn (the storage-layer public type) so the test
// matches the boundary the storage layer publishes, not the underlying
// pkg/executor alias.
func seedTranscript(t *testing.T, store *Store, runID string, turns []models.ParsedTurn) {
	t.Helper()
	ctx := context.Background()
	transcript := models.Transcript{
		RunID:       runID,
		RawOutput:   "fixture",
		ParsedTurns: turns,
		TotalTurns:  len(turns),
	}
	if err := store.SaveTranscript(ctx, transcript); err != nil {
		t.Fatalf("seedTranscript: %v", err)
	}
}

func TestListTurns_ReturnsParsedTurnsForRun(t *testing.T) {
	store := newTestStore(t)
	seedRun(t, store, "run-turns-1")
	seedTranscript(t, store, "run-turns-1", []models.ParsedTurn{
		{TurnIndex: 0, BlockKind: "thinking", Content: "let me check"},
		{TurnIndex: 1, BlockKind: "tool_use", ToolName: "Read", Content: "Read main.go"},
	})

	got, err := store.ListTurns(context.Background(), "run-turns-1")
	if err != nil {
		t.Fatalf("ListTurns: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("want 2 turns, got %d", len(got))
	}
	if got[0].BlockKind != "thinking" || got[1].ToolName != "Read" {
		t.Errorf("fields not preserved: %+v", got)
	}
}

func TestListTurns_MissingRunReturnsEmpty(t *testing.T) {
	store := newTestStore(t)
	got, err := store.ListTurns(context.Background(), "nonexistent")
	if err != nil {
		t.Errorf("missing run should not error; got %v", err)
	}
	if len(got) != 0 {
		t.Errorf("missing run should return empty slice; got %d", len(got))
	}
}

func TestListTurnsByExperiment_GroupsByRunID(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	// Bootstrap the shared task/experiment/variant via seedRun for the
	// first run; insert the second run directly with a distinct
	// run_number to satisfy the (experiment_id, variant_id, run_number)
	// uniqueness constraint.
	seedRun(t, store, "exp-multi-a")
	if _, err := store.DB.ExecContext(ctx,
		`INSERT INTO runs (id, experiment_id, variant_id, run_number, status)
		 VALUES ('exp-multi-b', 'exp-1', 'var-1', 1, 'completed')`); err != nil {
		t.Fatalf("insert second run: %v", err)
	}
	seedTranscript(t, store, "exp-multi-a", []models.ParsedTurn{
		{TurnIndex: 0, BlockKind: "text", Content: "from a"},
	})
	seedTranscript(t, store, "exp-multi-b", []models.ParsedTurn{
		{TurnIndex: 0, BlockKind: "text", Content: "from b"},
		{TurnIndex: 1, BlockKind: "text", Content: "more from b"},
	})

	got, err := store.ListTurnsByExperiment(context.Background(), "exp-1")
	if err != nil {
		t.Fatalf("ListTurnsByExperiment: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("want 2 runs in map, got %d (keys=%v)", len(got), keysOf(got))
	}
	if len(got["exp-multi-a"]) != 1 {
		t.Errorf("run a expected 1 turn, got %d", len(got["exp-multi-a"]))
	}
	if len(got["exp-multi-b"]) != 2 {
		t.Errorf("run b expected 2 turns, got %d", len(got["exp-multi-b"]))
	}
}

func keysOf(m map[string][]models.ParsedTurn) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
