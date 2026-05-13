# Example 02 — Evaluate your own CLAUDE.md

You have a CLAUDE.md you've been refining at $work. You want to know if it
makes Aider+Qwen behave differently than the bare baseline on a realistic
brownfield bug-fix.

This example uses the `claudemd` harness against the `brownfield-fix-async-race`
task. `claudemd` is identical to `bare` except it lays down a CLAUDE.md into
the workspace before the agent runs — so any observed behavior difference is
directly attributable to your CLAUDE.md.

## Setup

1. Drop your CLAUDE.md into the `my-claudemd/` subdirectory:
   ```bash
   cp ~/work/repo/CLAUDE.md my-claudemd/CLAUDE.md
   ```
2. (Optional) point `experiment.yaml` at a different task in `../../tasks/`
   or one you authored using [`../../docs/task-authoring.md`](../../docs/task-authoring.md).

## What you'll see

After 3 replicas of each harness complete, the Diagnostic Compare view should
show:

- **BehavioralRadar**: `context_reference_rate` will rise for claudemd if
  your CLAUDE.md actually gets read. If it doesn't move, your agent isn't
  picking it up.
- **FailureBreakdown**: if your CLAUDE.md is reducing a specific failure
  mode (e.g., agent kept hitting HAL_API; CLAUDE.md tells them to verify
  imports), the bar for that code should shrink under claudemd.
- **TranscriptEvidence**: literally see which lines the failure classifier
  attributed each failure to. Lets you go back to your CLAUDE.md and tune
  the rules that aren't landing.

## Anti-pattern warning

The brownfield task has scope discipline tests (`tests/test_scope.sh`). If
your CLAUDE.md says things like "always update docstrings" or "rewrite
adjacent functions", you'll TANK the SCOPE_DRIFT metric. That's a real
finding — your CLAUDE.md may be making your agent worse at constrained
brownfield work, even if it's net-positive on greenfield. AgentDx is
designed to surface exactly this kind of asymmetry.

## How to run it

Same shape as Example 01: create the experiments through the UI for now
(the YAML schema in this directory is the orchestrator-follow-up target,
not a runnable command today), then assemble the run IDs into the
Diagnostic Compare URL:

```bash
docker compose up -d
open http://localhost:5173/experiments
# create one bare experiment + one claudemd experiment (point claudemd at
# my-claudemd/CLAUDE.md), wait for runs to complete (~10–15 min on
# 16 GB Mac for 5 replicas × 2 harnesses), then:
open "http://localhost:5173/diagnostic/compare?runs=<id1>,<id2>,<id3>,<id4>,<id5>,<id6>"
```
