package storage

import (
	"context"
	"database/sql"
	"fmt"
)

// Tx runs fn inside a single SQLite transaction. The transaction commits
// when fn returns nil; rolls back otherwise. A panic inside fn rolls back
// the transaction and re-panics, so callers see the original panic value
// after the DB is in a consistent state.
//
// Used by the orchestrator's run-finalization path so that transcript +
// grade + diagnostic + run-status updates are all-or-nothing: a crash or
// error mid-finalization no longer leaves a half-graded run row in the
// DB. Use Tx whenever a multi-step write to the DB must be atomic from
// the consumer's point of view.
func (s *Store) Tx(ctx context.Context, fn func(*sql.Tx) error) (err error) {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("storage.Tx: begin: %w", err)
	}

	defer func() {
		if p := recover(); p != nil {
			// Rollback first so the DB returns to a consistent state, then
			// re-panic. The caller's recover (or lack thereof) is preserved.
			_ = tx.Rollback()
			panic(p)
		}
		if err != nil {
			if rbErr := tx.Rollback(); rbErr != nil && rbErr != sql.ErrTxDone {
				// Wrap both errors: %w preserves errors.Is on the original
				// fn error (the one callers route on), %v surfaces the
				// rollback failure as supplementary diagnostic info.
				err = fmt.Errorf("storage.Tx: rollback after %w: %v", err, rbErr)
			}
		}
	}()

	if err = fn(tx); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("storage.Tx: commit: %w", err)
	}
	return nil
}
