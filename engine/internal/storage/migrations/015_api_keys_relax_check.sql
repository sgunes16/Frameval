-- Relax the provider CHECK constraint on api_keys to include additional
-- providers needed by the LLM judge: openrouter, zai, ollama, cursor.
--
-- SQLite does not support ALTER TABLE ... DROP CONSTRAINT, so we use the
-- standard 5-step table rebuild: create new, copy data, drop old, rename.

CREATE TABLE api_keys_new (
    id            TEXT PRIMARY KEY,
    provider      TEXT NOT NULL UNIQUE CHECK (provider IN ('anthropic', 'openai', 'google', 'openrouter', 'zai', 'ollama', 'cursor')),
    encrypted_key TEXT NOT NULL,
    created_at    TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    updated_at    TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);

INSERT INTO api_keys_new SELECT * FROM api_keys;

DROP TABLE api_keys;

ALTER TABLE api_keys_new RENAME TO api_keys;
