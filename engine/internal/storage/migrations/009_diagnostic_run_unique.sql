-- Enforce that each run has at most one diagnostic row.
--
-- The diagnostic_repo's SaveDiagnostic uses delete-then-insert for
-- idempotency, which is safe in the single-orchestrator-instance MVP. As
-- soon as two threads / two orchestrators concurrently re-grade the same
-- run, both can pass the DELETE and both INSERT, leaving duplicate rows.
-- A UNIQUE index closes that race at the database level — GetDiagnosticByRun
-- already uses QueryRowContext + WHERE run_id = ? and would silently drop
-- the duplicate, so this also protects readers from inconsistency.
--
-- SQLite supports CREATE UNIQUE INDEX after the fact; no table rebuild
-- required.

CREATE UNIQUE INDEX IF NOT EXISTS idx_diagnostic_run_unique ON diagnostic(run_id);
