# Demo checklist

Pre-flight script for the 8-10 minute AgentDx demo. Designed for the
4-week-MVP demo session (Story #28). Use as a literal checklist the
day-of; don't skip ahead until each box is green.

## T-24 hours

- [ ] Pull the latest `main` and re-build the sandbox image:
      `docker build -t frameval-sandbox:local -f docker/sandbox/Dockerfile .`
- [ ] Confirm Ollama is running on host with `qwen2.5-coder:7b` pulled:
      `ollama list | grep qwen2.5-coder:7b`
- [ ] Run all three example tasks end-to-end once with `bare` to sanity-check
      the sandbox / Ollama path. Record total wall time per task — this is
      your demo timing budget.
- [ ] If the calibration study (Story #25) has run, copy the markdown
      report into `docs/calibration/2026-XX-validation.md` so you can
      flash it during the methodology section.

## T-1 hour

- [ ] Close every other Docker container and Aider session; the demo
      Mac should have ~10 GB free RAM headroom for Ollama.
- [ ] **Verify the Cursor backup path is ready** before you discover it
      isn't mid-demo. Export the key and sanity-check:
      `export CURSOR_API_KEY=<your-key> && env | grep CURSOR_API_KEY`
      Without this, the recovery move described in the bottom section
      will fail.
- [ ] Restart the stack from a clean DB:
      `rm frameval.db && docker compose up`
- [ ] In a separate terminal, tail engine logs so you can see runs as
      they complete: `docker compose logs -f engine`
- [ ] Open the **two** browser tabs you'll switch between:
      1. The Run Monitor for whichever experiment you'll be watching live
      2. The Diagnostic Compare URL with the run IDs from your dry-run

## During (run-of-show)

1. **00:00–00:30 — Why this exists.** One sentence: "Benchmarks give you
   a number, not a diagnosis." Frame the gap.
2. **00:30–01:00 — Set up the comparison.** On the Compare page,
   paste 6 run IDs (3× bare + 3× ralph on wordfreq). Hit "Share link"
   so the URL is in the bar — useful if you crash and need to recover.
3. **01:00–04:00 — Walk the radar.** Live-narrate which dimensions
   differ. Hover the legend to isolate. The key reveal: ralph wins on
   self_validation_rate; bare wins on turn_efficiency. Both can be right
   for different definitions of "good".
4. **04:00–06:00 — Failure breakdown.** Click into a stacked segment to
   surface the transcript evidence panel. THIS is the "aha" moment that
   distinguishes AgentDx from scalar benchmarks.
5. **06:00–07:30 — Recovery timeline.** The gantt rows make the
   bare-vs-ralph cost-vs-quality trade-off geometric, not numerical.
6. **07:30–08:30 — Cost-quality scatter.** Land the Pareto-frontier
   reading. Hand-wave that "ralph wins iff your time budget is generous".
7. **08:30–10:00 — How to extend.** Three commands:
   - `agentdx run --task ./my-task --harness bare,my-harness`
     (the orchestrator-follow-up CLI, currently scaffolded)
   - Show `examples/02-evaluate-your-own-claudemd/` (paste-in CLAUDE.md)
   - Reference `docs/task-authoring.md` for the 4 recipes

## Recovery moves (when something breaks live)

- **Ollama hangs:** kill the model with `ollama stop qwen2.5-coder:7b`,
  re-start it. The stack will pick up the next invocation.
- **Engine crashes mid-demo:** the diagnostic data is already in
  `frameval.db`. Restart the engine; the Compare page reloads with the
  same run IDs.
- **Nothing renders:** check the browser console for 404s on
  `/api/runs/<id>/diagnostic`. If 404 — orchestrator hasn't wired the
  pipeline yet (out-of-scope follow-up). Fall back to the screencast at
  `docs/demo/recording.mp4` (record at T-24h).

## Backup plan

If live Ollama is unrecoverable mid-demo:

1. Switch the executor for the next live demo step to `cursor` (requires
   `CURSOR_API_KEY` env var set on the demo box) — same harness shape,
   same diagnostic output.
2. If even Cursor is down: play the pre-recorded screencast and narrate
   over it. Plan to record this at T-24h regardless; it's also the
   thesis-defense backup.

## After

- [ ] Save the live-demo run IDs into `docs/demo/last-run-ids.txt` so
      the report is reproducible.
- [ ] If you noticed any UI bug during the demo, file an issue
      immediately while it's fresh.
