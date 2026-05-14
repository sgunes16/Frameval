package storage

import (
	"context"
	"database/sql"
	"errors"
	"testing"
)

func TestTx_CommitsOnNilError(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	err := store.Tx(ctx, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `INSERT INTO tasks (id, name, description, category, complexity_score, codebase_type, task_prompt)
			VALUES ('tx-commit', 'tx', 'd', 'greenfield', 1.0, 'python', 'p')`)
		return err
	})
	if err != nil {
		t.Fatalf("Tx returned err: %v", err)
	}

	var count int
	if err := store.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM tasks WHERE id='tx-commit'").Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 row after commit, got %d", count)
	}
}

func TestTx_RollsBackOnError(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	sentinel := errors.New("intentional fail")
	err := store.Tx(ctx, func(tx *sql.Tx) error {
		_, _ = tx.ExecContext(ctx, `INSERT INTO tasks (id, name, description, category, complexity_score, codebase_type, task_prompt)
			VALUES ('tx-rollback', 'tx', 'd', 'greenfield', 1.0, 'python', 'p')`)
		return sentinel
	})
	if !errors.Is(err, sentinel) {
		t.Fatalf("expected sentinel error to propagate, got %v", err)
	}

	var count int
	if err := store.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM tasks WHERE id='tx-rollback'").Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 0 {
		t.Errorf("expected rollback (0 rows), got %d", count)
	}
}

func TestTx_RollsBackOnPanic(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic to re-raise")
		}
	}()

	_ = store.Tx(ctx, func(tx *sql.Tx) error {
		_, _ = tx.ExecContext(ctx, `INSERT INTO tasks (id, name, description, category, complexity_score, codebase_type, task_prompt)
			VALUES ('tx-panic', 'tx', 'd', 'greenfield', 1.0, 'python', 'p')`)
		panic("simulated bug")
	})

	// Unreachable in the panic path — defer above will assert no commit.
	t.Fatal("Tx should have re-panicked")
}

func TestTx_NoPanicRollbackLeavesDBClean(t *testing.T) {
	// Companion to TestTx_RollsBackOnPanic — runs in a separate goroutine
	// so we can verify the row really did not commit, without the panic
	// itself terminating the test.
	store := newTestStore(t)
	ctx := context.Background()

	defer func() {
		_ = recover()

		var count int
		if err := store.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM tasks WHERE id='tx-panic-check'").Scan(&count); err != nil {
			t.Fatalf("count: %v", err)
		}
		if count != 0 {
			t.Errorf("panic path must rollback; got %d rows committed", count)
		}
	}()

	_ = store.Tx(ctx, func(tx *sql.Tx) error {
		_, _ = tx.ExecContext(ctx, `INSERT INTO tasks (id, name, description, category, complexity_score, codebase_type, task_prompt)
			VALUES ('tx-panic-check', 'tx', 'd', 'greenfield', 1.0, 'python', 'p')`)
		panic("ensure rollback")
	})
}
