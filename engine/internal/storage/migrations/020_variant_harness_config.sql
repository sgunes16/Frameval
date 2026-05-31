-- 020_variant_harness_config.sql
--
-- Adds an opaque per-variant harness config blob. Keyed by harness id;
-- each value is whatever shape that harness expects. Today this carries
-- `agent_instructions.content` (the user-typed CLAUDE.md). Future
-- harnesses (multiagent, speckit) reuse this column without further
-- schema work.

ALTER TABLE variants ADD COLUMN harness_config_json TEXT;
