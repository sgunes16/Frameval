-- Compare V2 anchor cache (story #66).
--
-- The Tape tab in Compare V2 builds its alignment scaffold from
-- per-run anchors. For experiments with <= 5 runs the engine can
-- recompute on-the-fly from the bulk turns endpoint, but at 6+
-- runs the round-trip cost grows quadratically. This column caches
-- the JSON-serialised AnchorBundle keyed by experiment, refreshed
-- by the orchestrator on each run finalize.
--
-- Empty / NULL is the documented "not computed yet" state — the
-- handler returns the empty bundle in that case rather than 404,
-- so the frontend can treat "no data" and "still building" the same.

ALTER TABLE experiments ADD COLUMN anchors_json TEXT NOT NULL DEFAULT '{}';
