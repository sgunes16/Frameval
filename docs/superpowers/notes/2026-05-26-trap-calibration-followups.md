# Trap calibration follow-ups (2026-05-26)

After 4 rounds of bare/opencode-deepseek-v4-flash-free calibration on the
5 failure-mode-targeted brownfield tasks, infra issues (missing deps,
flaky thresholds, missing pytest-timeout) are fully cleaned up. What
remains is a calibration-model mismatch: deepseek-flash + bare is strong
enough to solve 4 of 5 tasks at 100%. Only `brownfield-hal-api-pydantic-version`
trips its trap consistently (deterministic v1-syntax check).

This file captures the decisions still outstanding so a later session
can pick them up.

## Round-4 baseline (bare / opencode-deepseek-v4-flash-free)

| Task | Round-4 | Trap firing? |
|---|---|---|
| brownfield-misread-hidden-contract | 3/3 | No — contract is hidden in `docs/api/`, README points at it only indirectly, but the agent still finds it via the failing pytest run |
| brownfield-scope-drift-tempting-cleanup | 3/3 | No — bug docstring giveaway removed, hash + 6-line budget tests are armed, but deepseek-flash stays disciplined |
| brownfield-wrong-abs-async-throttle | 3/3 | No (4th round) — `pytest-timeout` baked into image, threshold rewritten around aiolimiter's bucket. Agent goes straight to canonical `aiolimiter` solution |
| brownfield-stop-early-multi-step-migration | 4/4 | No — agent completes model + schema + alembic migration in one pass |
| brownfield-hal-api-pydantic-version | 2/3 | **Yes** — agent writes Pydantic v1 `@validator` syntax; v2-API check fires deterministically |

## What's been validated

- Trap *mechanics* are sound: scope test catches refactor drift; canary
  catches `time.sleep`; v1-vs-v2 syntax check catches stale Pydantic
  knowledge. The mechanisms work — they just don't trigger on
  deepseek-flash because the model has the relevant patterns right.
- Infra is no longer a confounder. Image bakes `aiolimiter`,
  `sqlalchemy`, `alembic`, `pyyaml`, `openapi-spec-validator`,
  `pytest-timeout`. Each per-test-command container has everything it
  needs without re-running setup.sh.

## Outstanding work — before the next round of task hardening

**Run harness comparison first.** Before strengthening any trap, run
bare vs ralph (and ideally bare vs ralph vs spec-kit) against the
existing 5 tasks. The thesis hypothesis is that more capable harnesses
should produce *higher* solve rates on these traps; if bare is already
at 100% and ralph is also at 100%, the differentiation surface for
this model is too narrow and we should change the calibration model
*before* changing the tasks. If ralph noticeably beats bare on
HAL_API (only one currently trapping), that suggests harness signal
exists but the other 4 traps aren't sensitive enough.

Approximate plan:
- 5 runs × 5 tasks × {bare, ralph} = 50 runs on opencode-deepseek-v4-flash-free
- Per-task pass rate per harness, plus per-test-name breakdown
- Then decide: if bare = ralph everywhere, switch calibration model
  (try a smaller free model — Llama-3.1-8B-instruct on OpenRouter, or
  similar). If ralph > bare on some tasks, harden the other 4 to be
  more like HAL_API (deterministic, syntax-level checks the agent must
  pattern-match correctly to pass).

## Per-task hardening ideas (deferred until after harness comparison)

### MISREAD (currently 3/3)

The leak is: agent runs pytest, sees `test_response_contract` fail,
reads the test, finds `CONTRACT_PATH = .../docs/api/contract.v1.yaml`,
updates it. Options:
- Hide the contract behind a discovery mechanism (e.g. test imports a
  loader that walks `docs/`, so the test code doesn't directly name
  the path).
- Add a *second* contract surface: a separate `clients/typescript/`
  schema file that also needs the new field. Real services have N
  contracts; missing one is the genuine MISREAD pattern.
- Make the failure error message generic ("invalid response payload")
  instead of "additionalProperties: created_at not allowed" — forces
  the agent to investigate before fixing.

### SCOPE_DRIFT (currently 3/3)

Strengthened scope test now catches: any non-pricing.py change, any
DEPRECATED-count drift, >6-line pricing.py diff, any modification to
the 4 deprecated function bodies (SHA-256 per block). These all work;
deepseek-flash just doesn't try to refactor. To force the failure
mode, the *task itself* should require a multi-line fix that's easy
to overshoot — e.g., the bug spans two functions and the natural fix
is to extract a helper. Then "minimal fix" requires real judgment,
and the temptation to refactor is structural, not cosmetic.

### WRONG_ABS (currently 3/3)

Canonical `aiolimiter` solution passes cleanly. The trap fires on
`time.sleep` only. To catch a wider blast radius:
- Add a `time.time()`-based busy loop scenario? Probably too contrived.
- Better: drop the rate-limit hint entirely and force the agent to
  *discover* that rate limiting is needed from the failing test
  output. Currently `task_prompt` says "throttle to 10 req/s" — too
  helpful. Make the task "search endpoint sometimes 429s the upstream
  — make it stop" and let the agent find the rate-limit pattern.

### STOP_EARLY (currently 4/4)

3-step migration (model + schema + alembic) is easy enough that bare
completes all 3. To trigger STOP_EARLY:
- 5-step migration: model + schema + alembic + DB trigger + cache
  invalidation. Bare agents tend to stop at step 3.
- Or, *intermediate validation that's misleading*: an early test
  passes after the first step, lulling the agent into "I'm done"
  before they touch the migration.

### HAL_API (working)

No changes. Deterministic syntax check is the right mechanism. Apply
this pattern to the other 4 if/when their traps need to be tightened
the same way.

## Calibration model question

If the harness comparison above shows bare ≈ ralph (i.e. the
calibration agent is too capable across the board), the right move is
to swap the calibration model, not to keep ratcheting up task
difficulty. Candidates:

- `meta-llama/llama-3.1-8b-instruct:free` — smaller, weaker reasoning
- `google/gemma-2-9b-it:free`
- A local Ollama 4B model via the existing self-host path

Pick one where bare's per-task pass rate is roughly 30–50% on the
existing 5 tasks; that gives ralph/spec-kit headroom to differentiate.
