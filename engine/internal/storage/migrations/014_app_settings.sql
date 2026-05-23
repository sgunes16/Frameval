-- App-level settings for judge configuration.
--
-- Non-secret settings (judge provider, model, enable flag) are stored in this
-- key-value table. API keys continue to use the encrypted api_keys table.
-- Settings can be edited from the frontend Settings page.

CREATE TABLE app_settings (
  key        TEXT PRIMARY KEY,
  value      TEXT NOT NULL,
  updated_at DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);

INSERT INTO app_settings (key, value) VALUES
  ('judge.provider', 'openrouter'),
  ('judge.model',    'deepseek/deepseek-chat-v3-0324:free'),
  ('judge.enabled',  'true');
