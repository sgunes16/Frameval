-- 019_experiment_batches.sql
--
-- Adds batch identity to experiments so the Diagnostic launcher can
-- create N experiments under one logical batch and the Experiments
-- list can group them. Both columns are NULL-able so pre-existing
-- experiments stay unbatched; no backfill.
--
-- SQLite ADD COLUMN is O(1) metadata-only on populated tables; the
-- index creation is fast on an empty column.

ALTER TABLE experiments ADD COLUMN batch_id TEXT;
ALTER TABLE experiments ADD COLUMN batch_label TEXT;
CREATE INDEX IF NOT EXISTS idx_experiments_batch_id ON experiments(batch_id);
