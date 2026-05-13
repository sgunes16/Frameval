# Demo scenarios

Three scripted demo scenarios for the AgentDx demo. Pick one for the
live performance, keep the other two as recorded fallbacks in case of
Ollama / network failure.

The numbers below are placeholders — they get filled in after the
dress-rehearsal runs on T-24h, so the screen matches what you'll narrate.

## Scenario A — The bare-vs-ralph showcase

**Task:** `greenfield-cli-wordfreq` (≤ 2 min per bare run, ≤ 5 min per
ralph run with k=8).

**Story:** "Both end at the same pass rate, but the diagnostic profiles
disagree on *how* they got there."

**Expected shape:**
- bare pass: ~`__%`, ralph pass: ~`__%`
- bare wall: ~`__s`, ralph wall: ~`__s`
- Dominant bare failure: typically `STOP_EARLY` (skipped `-c` flag)
- Dominant ralph failure: typically `LOOP_INF` (no-progress halt)
- `self_validation_rate` delta: ralph ≫ bare (it re-tests each iteration)

**One-line takeaway:** "Same outcome, different shape. The radar tells
you which agent you'd trust on the next task."

## Scenario B — The claudemd reveal

**Task:** `brownfield-fix-async-race` (≤ 3 min per run, requires the
git-baseline tag from setup.sh).

**Harnesses:** `bare` + `claudemd` (CLAUDE.md from
`examples/02-evaluate-your-own-claudemd/my-claudemd/CLAUDE.md`).

**Story:** "Adding CLAUDE.md *can* improve scope discipline. It can also
make it worse. Here's the data."

**Expected shape:**
- A well-written CLAUDE.md cuts `SCOPE_DRIFT` failures by ~`__%`.
- A too-eager CLAUDE.md ("always update docstrings") *introduces*
  SCOPE_DRIFT because the agent obeys it on a constrained brownfield task.
- The radar shows `context_reference_rate` rising for claudemd — proof
  the CLAUDE.md is being read.

**One-line takeaway:** "Wording matters. Measure it."

## Scenario C — The multi-agent ablation

**Task:** `greenfield-rate-limiter-fastapi` (≤ 4 min per run; uses
freezegun so the 60-s window check is fast).

**Harnesses:** `bare` + `planner_coder`.

**Story:** "Multi-agent isn't free — but on this task the planner role
prevents the WRONG_ABS failure that bare regularly hits."

**Expected shape:**
- bare passes ~`__%`; planner_coder passes ~`__%`.
- bare's failures are split across `HAL_API`, `MISREAD`,
  `WRONG_ABS`. planner_coder's failures are mostly `MISREAD` (the planner
  occasionally sends the coder down the wrong abstraction path).
- Wall clock: planner_coder ≈ 2× bare (two sequential LLM calls).

**One-line takeaway:** "Two agents > one when the failure mode is
abstraction-level. Same cost as ralph(k=2), different failure profile."

## Choosing for the live performance

Default: **Scenario A** — most contrast, shortest wall time, easiest
narrative. Keep Scenario B in your back pocket if the audience is the
"CLAUDE.md crowd"; lead with Scenario C for an audience that's been
arguing about multi-agent on Twitter.
