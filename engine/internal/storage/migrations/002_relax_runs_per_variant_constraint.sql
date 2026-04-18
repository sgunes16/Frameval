PRAGMA foreign_keys = OFF;

CREATE TABLE IF NOT EXISTS experiments_new (
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
    runs_per_variant INTEGER NOT NULL DEFAULT 5 CHECK (runs_per_variant >= 1),
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

DELETE FROM experiments_new;

INSERT INTO experiments_new (
    id, name, description, status, task_id, model, agent_cli, execution_mode,
    runs_per_variant, temperature, timeout_seconds, max_concurrent, judge_model,
    seed, estimated_cost_usd, actual_cost_usd, composite_weights_json,
    created_at, started_at, completed_at
)
SELECT
    id, name, description, status, task_id, model, agent_cli, execution_mode,
    runs_per_variant, temperature, timeout_seconds, max_concurrent, judge_model,
    seed, estimated_cost_usd, actual_cost_usd, composite_weights_json,
    created_at, started_at, completed_at
FROM experiments;

DROP TABLE experiments;
ALTER TABLE experiments_new RENAME TO experiments;

PRAGMA foreign_keys = ON;
