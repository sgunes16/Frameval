-- AgentDx differentiates runs by harness (bare, claudemd, speckit, ralph,
-- planner_coder). Until this migration, harness selection only existed in
-- the registry and was never persisted — every run silently used a direct
-- executor call. Adding harness_id to variants makes the selection durable
-- and queryable. Default 'bare' so pre-AgentDx variants keep working.
ALTER TABLE variants ADD COLUMN harness_id TEXT NOT NULL DEFAULT 'bare';
