ALTER TABLE experiments ADD COLUMN workspace_source_type TEXT NOT NULL DEFAULT 'task_codebase';
ALTER TABLE experiments ADD COLUMN local_path TEXT;
ALTER TABLE experiments ADD COLUMN git_url TEXT;
ALTER TABLE experiments ADD COLUMN git_ref TEXT;

ALTER TABLE tasks ADD COLUMN template_kind TEXT NOT NULL DEFAULT 'builtin';
ALTER TABLE tasks ADD COLUMN workspace_mode TEXT NOT NULL DEFAULT 'task_codebase';
ALTER TABLE tasks ADD COLUMN task_root_path TEXT;

ALTER TABLE artifact_versions ADD COLUMN source_kind TEXT NOT NULL DEFAULT 'custom_file';
ALTER TABLE artifact_versions ADD COLUMN display_name TEXT;
ALTER TABLE artifact_versions ADD COLUMN source_ref TEXT;

UPDATE tasks
SET task_root_path = CASE
    WHEN codebase_path IS NOT NULL AND codebase_path != '' THEN substr(codebase_path, 1, length(codebase_path) - 9)
    ELSE task_root_path
END
WHERE task_root_path IS NULL OR task_root_path = '';
