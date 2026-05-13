-- Align SQL schema with the pkg/task.Task and pkg/task.TestCase public types.
--
-- These columns are referenced by storage/task_repo.go but were never added to
-- the schema. The seeder failed loudly on a fresh DB with
-- "table tasks has no column named external_source" / "table test_cases has
-- no column named visibility".
--
-- Although the benchmark importer that originally introduced these fields has
-- been removed (#6), keeping the columns aligns the SQL schema with the public
-- pkg/task contract and allows future external task sources (e.g., user-imported
-- brownfield tasks) to populate them without another migration.

-- template_kind was already added by migration 003; not re-added here.
ALTER TABLE tasks ADD COLUMN external_source TEXT;
ALTER TABLE tasks ADD COLUMN external_id TEXT;
ALTER TABLE tasks ADD COLUMN external_url TEXT;
ALTER TABLE tasks ADD COLUMN metadata_json TEXT;

ALTER TABLE test_cases ADD COLUMN visibility TEXT;
ALTER TABLE test_cases ADD COLUMN timeout_seconds INTEGER;
ALTER TABLE test_cases ADD COLUMN setup_script TEXT;
