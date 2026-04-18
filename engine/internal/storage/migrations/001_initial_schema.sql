PRAGMA foreign_keys = ON;

CREATE TABLE IF NOT EXISTS experiments (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT,
    status TEXT NOT NULL DEFAULT 'draft'
        CHECK (status IN ('draft', 'estimating', 'ready', 'running', 'completed', 'failed', 'cancelled')),
    task_id TEXT NOT NULL REFERENCES tasks(id),
    model TEXT NOT NULL,
    agent_cli TEXT NOT NULL,
    execution_mode TEXT NOT NULL DEFAULT 'cli'
        CHECK (execution_mode IN ('cli', 'api', 'manual')),
    runs_per_variant INTEGER NOT NULL DEFAULT 5 CHECK (runs_per_variant >= 5),
    temperature REAL NOT NULL DEFAULT 0.0,
    timeout_seconds INTEGER NOT NULL DEFAULT 600,
    max_concurrent INTEGER NOT NULL DEFAULT 3,
    judge_model TEXT,
    seed INTEGER,
    estimated_cost_usd REAL,
    actual_cost_usd REAL,
    composite_weights_json TEXT NOT NULL DEFAULT '{"code":0.3,"judge":0.3,"process":0.2,"adherence":0.2}',
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    started_at TEXT,
    completed_at TEXT
);

CREATE TABLE IF NOT EXISTS variants (
    id TEXT PRIMARY KEY,
    experiment_id TEXT NOT NULL REFERENCES experiments(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    description TEXT,
    is_control INTEGER NOT NULL DEFAULT 0,
    ordering INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS artifact_versions (
    id TEXT PRIMARY KEY,
    variant_id TEXT NOT NULL REFERENCES variants(id) ON DELETE CASCADE,
    artifact_type TEXT NOT NULL,
    file_path TEXT NOT NULL,
    content TEXT NOT NULL,
    content_hash TEXT NOT NULL,
    dimensions_json TEXT,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);

CREATE TABLE IF NOT EXISTS runs (
    id TEXT PRIMARY KEY,
    experiment_id TEXT NOT NULL REFERENCES experiments(id) ON DELETE CASCADE,
    variant_id TEXT NOT NULL REFERENCES variants(id) ON DELETE CASCADE,
    run_number INTEGER NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'queued', 'running', 'grading', 'completed', 'failed', 'cancelled', 'timeout')),
    container_id TEXT,
    environment_fingerprint_json TEXT,
    started_at TEXT,
    completed_at TEXT,
    duration_seconds REAL,
    error_message TEXT,
    UNIQUE(experiment_id, variant_id, run_number)
);

CREATE TABLE IF NOT EXISTS transcripts (
    id TEXT PRIMARY KEY,
    run_id TEXT NOT NULL UNIQUE REFERENCES runs(id) ON DELETE CASCADE,
    raw_output TEXT NOT NULL,
    parsed_turns_json TEXT,
    filesystem_diff TEXT,
    total_turns INTEGER,
    total_tokens INTEGER,
    cost_usd REAL,
    output_files_path TEXT
);

CREATE TABLE IF NOT EXISTS grades (
    id TEXT PRIMARY KEY,
    run_id TEXT REFERENCES runs(id) ON DELETE CASCADE,
    baseline_id TEXT REFERENCES baselines(id) ON DELETE CASCADE,
    test_pass_rate REAL,
    test_pass_count INTEGER,
    test_fail_count INTEGER,
    lint_score REAL,
    type_check_pass INTEGER,
    file_state_valid INTEGER,
    turn_count INTEGER,
    total_tokens INTEGER,
    cost_usd REAL,
    token_efficiency REAL,
    backtrack_count INTEGER,
    self_validation_rate REAL,
    premature_completion INTEGER,
    idle_turns INTEGER,
    error_recovery_count INTEGER,
    tool_call_accuracy REAL,
    context_utilization REAL,
    judge_correctness REAL,
    judge_maintainability REAL,
    judge_completeness REAL,
    judge_best_practices REAL,
    judge_error_handling REAL,
    judge_irr_alpha REAL,
    raw_judge_responses_json TEXT,
    spec_instruction_compliance REAL,
    spec_constraint_violations INTEGER,
    spec_convention_adherence REAL,
    spec_per_instruction_json TEXT,
    composite_score REAL,
    graded_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    test_results_json TEXT,
    CHECK (run_id IS NOT NULL OR baseline_id IS NOT NULL)
);

CREATE TABLE IF NOT EXISTS tasks (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT NOT NULL,
    category TEXT NOT NULL CHECK (category IN ('greenfield', 'brownfield', 'bug-fix', 'refactor')),
    complexity_score REAL NOT NULL CHECK (complexity_score >= 1.0 AND complexity_score <= 10.0),
    codebase_type TEXT NOT NULL,
    task_prompt TEXT NOT NULL,
    technical_details TEXT,
    setup_script TEXT,
    codebase_path TEXT,
    is_builtin INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);

CREATE TABLE IF NOT EXISTS test_cases (
    id TEXT PRIMARY KEY,
    task_id TEXT NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    test_command TEXT NOT NULL,
    expected_result TEXT,
    ordering INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS baselines (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT,
    source TEXT NOT NULL CHECK (source IN ('control', 'official', 'community', 'custom')),
    artifact_type TEXT,
    artifact_content TEXT,
    task_id TEXT NOT NULL REFERENCES tasks(id),
    model TEXT NOT NULL,
    agent_cli TEXT NOT NULL,
    total_runs INTEGER NOT NULL,
    evaluated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS experiment_stats (
    id TEXT PRIMARY KEY,
    experiment_id TEXT NOT NULL REFERENCES experiments(id) ON DELETE CASCADE,
    variant_a_id TEXT NOT NULL REFERENCES variants(id),
    variant_b_id TEXT NOT NULL REFERENCES variants(id),
    metric_name TEXT NOT NULL,
    mean_a REAL,
    mean_b REAL,
    median_a REAL,
    median_b REAL,
    std_a REAL,
    std_b REAL,
    mann_whitney_u REAL,
    p_value REAL,
    cohens_d REAL,
    ci_lower REAL,
    ci_upper REAL,
    is_significant INTEGER NOT NULL DEFAULT 0,
    observed_power REAL,
    UNIQUE(experiment_id, variant_a_id, variant_b_id, metric_name)
);

CREATE TABLE IF NOT EXISTS api_keys (
    id TEXT PRIMARY KEY,
    provider TEXT NOT NULL UNIQUE CHECK (provider IN ('anthropic', 'openai', 'google')),
    encrypted_key TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);

CREATE TABLE IF NOT EXISTS model_configs (
    id TEXT PRIMARY KEY,
    provider TEXT NOT NULL,
    model_id TEXT NOT NULL UNIQUE,
    display_name TEXT NOT NULL,
    input_price_per_1k REAL NOT NULL,
    output_price_per_1k REAL NOT NULL,
    max_context_tokens INTEGER,
    supports_structured_output INTEGER NOT NULL DEFAULT 0,
    supports_seed INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_variants_experiment ON variants(experiment_id);
CREATE INDEX IF NOT EXISTS idx_artifact_versions_variant ON artifact_versions(variant_id);
CREATE INDEX IF NOT EXISTS idx_runs_experiment ON runs(experiment_id);
CREATE INDEX IF NOT EXISTS idx_runs_variant ON runs(variant_id);
CREATE INDEX IF NOT EXISTS idx_runs_status ON runs(status);
CREATE INDEX IF NOT EXISTS idx_grades_run ON grades(run_id);
CREATE INDEX IF NOT EXISTS idx_grades_baseline ON grades(baseline_id);
CREATE INDEX IF NOT EXISTS idx_baselines_task ON baselines(task_id);
CREATE INDEX IF NOT EXISTS idx_experiment_stats_experiment ON experiment_stats(experiment_id);
