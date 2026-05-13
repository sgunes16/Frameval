-- AgentDx diagnostic pipeline tables.
--
-- diagnostic        — per-run output of the AgentDx pipeline
--                     (fingerprint + symptoms + recovery + optional failure classification)
-- harness_registry  — built-in and third-party harness adapter registrations
-- validation_label  — hand-labeled ground truth for the failure classifier
--                     (calibration study, Story #25)

CREATE TABLE IF NOT EXISTS diagnostic (
    id                    TEXT PRIMARY KEY,
    run_id                TEXT NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
    fingerprint           TEXT NOT NULL,
    symptoms              TEXT NOT NULL,
    recovery              TEXT NOT NULL,
    failure_label         TEXT,
    classifier_model      TEXT,
    classifier_latency_ms INTEGER,
    created_at            TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);

CREATE INDEX IF NOT EXISTS idx_diagnostic_run ON diagnostic(run_id);

CREATE TABLE IF NOT EXISTS harness_registry (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL UNIQUE,
    kind        TEXT NOT NULL CHECK (kind IN ('builtin', 'external')),
    description TEXT,
    config_json TEXT,
    created_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);

CREATE TABLE IF NOT EXISTS validation_label (
    id               TEXT PRIMARY KEY,
    run_id           TEXT NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
    primary_label    TEXT NOT NULL,
    secondary_labels TEXT,
    labeler          TEXT,
    notes            TEXT,
    labeled_at       TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);

CREATE INDEX IF NOT EXISTS idx_validation_label_run ON validation_label(run_id);
