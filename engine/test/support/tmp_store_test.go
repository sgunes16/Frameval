package support_test

import (
	"context"
	"testing"

	"github.com/mustafaselman/frameval/engine/test/support"
)

func TestTmpStore_OpensFreshStoreWithMigrationsApplied(t *testing.T) {
	store := support.TmpStore(t)

	if store == nil || store.DB == nil {
		t.Fatal("TmpStore returned nil")
	}

	// Asserts at least one expected table from 001_initial_schema.sql exists.
	// If migrations failed to run, the experiments table would be absent.
	var name string
	row := store.DB.QueryRowContext(context.Background(),
		"SELECT name FROM sqlite_master WHERE type='table' AND name='experiments'")
	if err := row.Scan(&name); err != nil {
		t.Fatalf("experiments table missing — migrations did not run: %v", err)
	}
	if name != "experiments" {
		t.Errorf("unexpected scan: %q", name)
	}
}

func TestTmpStore_EachCallIsIsolated(t *testing.T) {
	a := support.TmpStore(t)
	b := support.TmpStore(t)

	// Two separate stores must not share state. Inserting into one must
	// not be visible in the other.
	ctx := context.Background()
	if _, err := a.DB.ExecContext(ctx,
		`INSERT INTO tasks (id, name, description, category, complexity_score, codebase_type, task_prompt)
		 VALUES ('isolated', 't', 'd', 'greenfield', 1.0, 'python', 'p')`); err != nil {
		t.Fatalf("insert into store a: %v", err)
	}

	var count int
	if err := b.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM tasks WHERE id='isolated'").Scan(&count); err != nil {
		t.Fatalf("count from store b: %v", err)
	}
	if count != 0 {
		t.Errorf("expected isolated stores, store b saw %d rows from store a", count)
	}
}
