-- spec-kit CLI version used by the spec-kit harness.
--
-- The spec-kit harness installs `specify` (spec-kit) in the sandbox at run
-- time; this setting pins which release tag it installs. Editable from the
-- frontend Settings page; falls back to FRAMEVAL_SPECKIT_RELEASE env then a
-- hard-coded default in the engine.
INSERT OR IGNORE INTO app_settings (key, value) VALUES
  ('speckit.version', 'v0.9.1');
