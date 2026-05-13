-- AgentDx pivot retired pairwise variant statistics in favor of the per-run
-- Diagnostic Profile (see migration 006_agentdx.sql). The Go stats_repo.go
-- and the ExperimentStat model/handler were already removed in PR #50; this
-- migration drops the orphaned SQL table.
DROP TABLE IF EXISTS experiment_stats;
