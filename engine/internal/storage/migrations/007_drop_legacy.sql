-- Drop legacy tables superseded by the AgentDx pivot.
--
-- The grades table is recreated to remove the now-dangling
-- baseline_id FK declaration (the parent baselines table is dropped here).
-- Leaving the FK declaration in place would silently misrepresent the schema
-- and cause runtime FOREIGN KEY errors if any future code ever set baseline_id.
--
-- experiment_stats is intentionally KEPT for now: stats_repo / orchestrator
-- still write to it on completion, and the GetExperimentStats handler still
-- reads it. The stats pipeline is retired entirely when AgentDx diagnostic
-- supersedes the grade-based comparison view (Epic 4).
--
-- PRAGMA foreign_keys = OFF is required so we can recreate grades while
-- keeping the existing run FK pointing at the still-live runs table without
-- triggering deferred-constraint validation during the swap.

PRAGMA foreign_keys = OFF;

CREATE TABLE grades_new (
    id                          TEXT PRIMARY KEY,
    run_id                      TEXT NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
    test_pass_rate              REAL,
    test_pass_count             INTEGER,
    test_fail_count             INTEGER,
    lint_score                  REAL,
    type_check_pass             INTEGER,
    file_state_valid            INTEGER,
    turn_count                  INTEGER,
    total_tokens                INTEGER,
    cost_usd                    REAL,
    token_efficiency            REAL,
    backtrack_count             INTEGER,
    self_validation_rate        REAL,
    premature_completion        INTEGER,
    idle_turns                  INTEGER,
    error_recovery_count        INTEGER,
    tool_call_accuracy          REAL,
    context_utilization         REAL,
    judge_correctness           REAL,
    judge_maintainability       REAL,
    judge_completeness          REAL,
    judge_best_practices        REAL,
    judge_error_handling        REAL,
    judge_irr_alpha             REAL,
    raw_judge_responses_json    TEXT,
    spec_instruction_compliance REAL,
    spec_constraint_violations  INTEGER,
    spec_convention_adherence   REAL,
    spec_per_instruction_json   TEXT,
    composite_score             REAL,
    graded_at                   TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    test_results_json           TEXT
);

INSERT INTO grades_new (
    id, run_id, test_pass_rate, test_pass_count, test_fail_count, lint_score, type_check_pass,
    file_state_valid, turn_count, total_tokens, cost_usd, token_efficiency, backtrack_count,
    self_validation_rate, premature_completion, idle_turns, error_recovery_count, tool_call_accuracy,
    context_utilization, judge_correctness, judge_maintainability, judge_completeness, judge_best_practices,
    judge_error_handling, judge_irr_alpha, raw_judge_responses_json, spec_instruction_compliance,
    spec_constraint_violations, spec_convention_adherence, spec_per_instruction_json, composite_score,
    graded_at, test_results_json
)
SELECT
    id, run_id, test_pass_rate, test_pass_count, test_fail_count, lint_score, type_check_pass,
    file_state_valid, turn_count, total_tokens, cost_usd, token_efficiency, backtrack_count,
    self_validation_rate, premature_completion, idle_turns, error_recovery_count, tool_call_accuracy,
    context_utilization, judge_correctness, judge_maintainability, judge_completeness, judge_best_practices,
    judge_error_handling, judge_irr_alpha, raw_judge_responses_json, spec_instruction_compliance,
    spec_constraint_violations, spec_convention_adherence, spec_per_instruction_json, composite_score,
    graded_at, test_results_json
FROM grades
WHERE run_id IS NOT NULL;

DROP TABLE grades;
ALTER TABLE grades_new RENAME TO grades;

DROP INDEX IF EXISTS idx_baselines_task;
DROP TABLE IF EXISTS baselines;

PRAGMA foreign_keys = ON;
