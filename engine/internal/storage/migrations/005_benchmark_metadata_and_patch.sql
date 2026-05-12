ALTER TABLE tasks ADD COLUMN external_source TEXT;
ALTER TABLE tasks ADD COLUMN external_id TEXT;
ALTER TABLE tasks ADD COLUMN external_url TEXT;
ALTER TABLE tasks ADD COLUMN metadata_json TEXT NOT NULL DEFAULT '{}';

ALTER TABLE test_cases ADD COLUMN visibility TEXT NOT NULL DEFAULT 'public' CHECK (visibility IN ('public', 'hidden'));
ALTER TABLE test_cases ADD COLUMN timeout_seconds INTEGER NOT NULL DEFAULT 120;
ALTER TABLE test_cases ADD COLUMN setup_script TEXT;

ALTER TABLE transcripts ADD COLUMN patch TEXT;
