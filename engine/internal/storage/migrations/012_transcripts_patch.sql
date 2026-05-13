-- transcript_repo.go writes a `patch` column (captured `git diff` output from
-- the sandbox) but the original transcripts schema in 001 never declared
-- it. Pre-AgentDx the column existed as a transient migration 005 that was
-- never committed; the column has been silently missing ever since, so any
-- run that reaches CapturePatch fails with "table transcripts has no column
-- named patch". Add the column nullable so historical rows still load.
ALTER TABLE transcripts ADD COLUMN patch TEXT;
