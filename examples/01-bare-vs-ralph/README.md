# Example 01 — `bare` vs `ralph` on the wordfreq task

The simplest meaningful comparison AgentDx supports: a single-shot agent
(`bare`) against the same agent wrapped in a Ralph Wiggum loop (`ralph`).
Same task. Same executor. Same model. The only thing that changes is the
harness around the agent.

## What you'll see

After the runs complete, the Diagnostic Compare view should show:

- **BehavioralRadar**: Ralph typically has higher `self_validation_rate`
  (it re-checks its work each iteration) but also higher `idle_thinking_ratio`
  on later iterations.
- **FailureBreakdown**: bare often hits `STOP_EARLY` on edge cases. Ralph
  rarely does, but can fall into `LOOP_INF` on tasks where the agent can't
  see its own mistake.
- **CostQualityScatter**: Ralph is up-and-to-the-right of bare on
  pass-rate, but also further right on wall-clock seconds — the textbook
  Pareto trade-off.

## How to run it

End-to-end orchestration of a harness-aware experiment from one YAML file
is the active follow-up (Story #23's "out of scope" note). Until that lands,
use the existing `/api/experiments` UI to create two experiments — one per
harness — and reference the same task / replica count, then assemble the
run IDs into the Diagnostic Compare URL:

```bash
# Bring up the stack (sandbox image must be pre-built)
docker compose up -d

# Create the experiments through the UI for now
open http://localhost:5173/experiments

# Once both sets of runs complete, open the compare view with the
# completed run IDs
open "http://localhost:5173/diagnostic/compare?runs=<bare-run-id-1>,<bare-run-id-2>,<bare-run-id-3>,<ralph-run-id-1>,<ralph-run-id-2>,<ralph-run-id-3>"
```

The `experiment.yaml` in this directory documents the harness-aware schema
the orchestrator follow-up will consume directly. Until then it's a planning
document, not a runnable command.

## Configuration

See `experiment.yaml` in this directory. Key fields:

- `task`: pointer to `../../tasks/greenfield-cli-wordfreq` (relative path
  is resolved by the engine).
- `executor`: `aider` — talks to local Ollama. Set `OLLAMA_BASE_URL` if
  your Ollama instance isn't on `host.docker.internal:11434`.
- `harnesses`: the two adapters to compare. Add `claudemd` or `speckit` here
  to widen the comparison.
- `replicas`: how many times to run each harness. 3 is the sweet spot for
  demo (low variance, modest wall time).

## Expected runtime

On a 16 GB M-series Mac running Ollama with Qwen2.5-Coder-7B:

- bare × 3: ~3 min total
- ralph × 3 (k=8): ~15 min total
- diagnostic pipeline (deterministic + Haiku classifier): ~10 s per run

Total: ~20 min for the full 6-run comparison.

## What to inspect afterward

1. Open `/diagnostic/compare?runs=<id1>,<id2>,<id3>,<id4>,<id5>,<id6>`.
2. Hover the BehavioralRadar legend to highlight one harness at a time.
3. Click into a FailureBreakdown segment to surface the corresponding
   transcript evidence in the panel below.
4. Use the CostQualityScatter to spot which harness is on the Pareto
   frontier for this task — typically Ralph wins on success at the cost
   of wall time.
