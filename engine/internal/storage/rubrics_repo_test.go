package storage

import (
	"context"
	"database/sql"
	"errors"
	"testing"
)

func TestRubrics_ListSeedHas5Builtins(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	rows, err := store.ListRubrics(ctx)
	if err != nil {
		t.Fatalf("ListRubrics: %v", err)
	}
	if len(rows) != 5 {
		t.Errorf("got %d rubrics, want 5 seeded", len(rows))
	}
	keys := map[string]bool{}
	for _, r := range rows {
		keys[r.Key] = true
		if !r.IsBuiltin {
			t.Errorf("rubric %q should be builtin", r.Key)
		}
	}
	for _, want := range []string{"correctness", "maintainability", "completeness", "best_practices", "error_handling"} {
		if !keys[want] {
			t.Errorf("missing seeded rubric %q", want)
		}
	}
}

func TestRubrics_GetMissingReturnsErrNoRows(t *testing.T) {
	store := newTestStore(t)
	_, err := store.GetRubric(context.Background(), "nope")
	if !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("want sql.ErrNoRows, got %v", err)
	}
}

func TestRubrics_UpsertNewThenReadBack(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	r := Rubric{Key: "security", DisplayName: "Security", Prompt: "score security only", SortOrder: 99, IsBuiltin: false}
	if err := store.UpsertRubric(ctx, r); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	got, err := store.GetRubric(ctx, "security")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.DisplayName != "Security" || got.Prompt != "score security only" || got.IsBuiltin {
		t.Errorf("roundtrip mismatch: %+v", got)
	}
}

func TestRubrics_UpsertExistingPreservesIsBuiltin(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	// Edit a builtin rubric — display_name change should stick, is_builtin must remain 1.
	r := Rubric{Key: "correctness", DisplayName: "Correctness (edited)", Prompt: "new prompt body", IsBuiltin: false}
	if err := store.UpsertRubric(ctx, r); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	got, err := store.GetRubric(ctx, "correctness")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if !got.IsBuiltin {
		t.Errorf("is_builtin must be preserved as true on upsert of a builtin row")
	}
	if got.Prompt != "new prompt body" {
		t.Errorf("prompt not updated")
	}
}

func TestRubrics_DeleteRemovesRow(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	_ = store.UpsertRubric(ctx, Rubric{Key: "tmp", DisplayName: "Tmp", Prompt: "x", IsBuiltin: false})
	if err := store.DeleteRubric(ctx, "tmp"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	_, err := store.GetRubric(ctx, "tmp")
	if !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("expected ErrNoRows after delete, got %v", err)
	}
}
