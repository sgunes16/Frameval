-- Drop legacy tables superseded by the AgentDx pivot.
--
-- The baseline_id column on grades is left in place as an inert orphan
-- (always NULL post-pivot) to avoid the cost of a full grades table
-- recreation. The grades table itself will be retired entirely once the
-- AgentDx diagnostic table is the authoritative grade store (Epic 4).
--
-- PRAGMA foreign_keys = OFF allows DROP TABLE to succeed even though the
-- grades.baseline_id FK declaration references the parent table — the
-- declaration is kept at the schema level but no longer enforced.

PRAGMA foreign_keys = OFF;

DROP INDEX IF EXISTS idx_baselines_task;
DROP INDEX IF EXISTS idx_experiment_stats_experiment;

DROP TABLE IF EXISTS baselines;
DROP TABLE IF EXISTS experiment_stats;

PRAGMA foreign_keys = ON;
