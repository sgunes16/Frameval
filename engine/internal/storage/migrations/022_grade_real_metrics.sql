-- Migration 022: add real process + harness-adherence metric columns to grades.
-- These columns back the metrics redesign; existing rows default to zero / NULL
-- so no data migration is required.

ALTER TABLE grades ADD COLUMN tool_call_count INTEGER NOT NULL DEFAULT 0;
ALTER TABLE grades ADD COLUMN tool_error_rate REAL NOT NULL DEFAULT 0;
ALTER TABLE grades ADD COLUMN ran_validation INTEGER NOT NULL DEFAULT 0;
ALTER TABLE grades ADD COLUMN harness_adherence_score REAL NOT NULL DEFAULT 0;
ALTER TABLE grades ADD COLUMN harness_adherence_json TEXT;
