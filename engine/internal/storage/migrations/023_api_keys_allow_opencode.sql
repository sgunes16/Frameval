-- Relax the provider CHECK constraint on api_keys to add 'opencode'.
--
-- The opencode executor can run against opencode's hosted "zen" gateway
-- (opencode/* models), which authenticates via the OPENCODE_API_KEY env
-- var. Storing that key in api_keys lets the orchestrator inject it into
-- the sandbox at run time (see Orchestrator.agentStoredKeyEnv) instead of
-- requiring it on the host environment.
--
-- SQLite does not support ALTER TABLE ... DROP CONSTRAINT, so we use the
-- standard 5-step table rebuild: create new, copy data, drop old, rename.

CREATE TABLE api_keys_new (
    id            TEXT PRIMARY KEY,
    provider      TEXT NOT NULL UNIQUE CHECK (provider IN ('anthropic', 'openai', 'google', 'openrouter', 'zai', 'ollama', 'cursor', 'opencode')),
    encrypted_key TEXT NOT NULL,
    created_at    TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    updated_at    TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);

INSERT INTO api_keys_new SELECT * FROM api_keys;

DROP TABLE api_keys;

ALTER TABLE api_keys_new RENAME TO api_keys;
