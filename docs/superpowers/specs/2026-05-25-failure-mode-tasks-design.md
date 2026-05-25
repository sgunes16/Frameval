# Failure-Mode-Calibrated Task Corpus — Design

**Status:** Draft
**Date:** 2026-05-25
**Owner:** sgunes16
**Related:** [[2026-05-12-agentdx-design]], `tasks/brownfield-fix-async-race/task.yaml`, `grader/failure_classifier/taxonomy.py`

## 1. Motivation

The current task library has 3 entries (1 brownfield, 2 greenfield). The brownfield task (`brownfield-fix-async-race`) was the only one designed with explicit failure-mode targeting (SCOPE_DRIFT, MISREAD, WRONG_ABS). The two greenfield tasks were calibrated for general "can the agent build a small thing" signal.

Empirical problem: in the live runs from the LLM-judge PR, the `bare` harness (no CLAUDE.md, no skills, plain agent CLI) consistently solves the brownfield race task and both greenfield tasks. This collapses the experimental ceiling — `bare` already gets ~3/3 tests passing, so no harness variant (spec-kit, ralph, etc.) can show meaningful improvement. The thesis (harness comparison: spec-kit vs ralph vs base claude-code) cannot extract signal from a corpus everyone solves.

User direction:
- *"custom taskları çoğaltalım ve kompleksleştirleim çünkü şu an bare bile çözüyo"*
- Thesis axis: **harness comparison** (spec-kit, ralph, base). Tasks must differentiate harnesses, not be calibrated to specific CLAUDE.md "remedy" wordings (that's a separate iterative-prompt-engineering project, not yet in Frameval).

Goal of this spec: design 5 new brownfield tasks, each calibrated to a single primary failure mode, with a target `bare` pass rate of 40-60%. This leaves measurable headroom for harnesses that enforce structured workflows (spec-kit, ralph) to climb to 70-90%.

## 2. Goals & non-goals

**Goals.**
- 5 new brownfield tasks under `tasks/`, each in its own subdirectory matching the existing `setup.sh + eval.sh + task.yaml + workspace files` pattern.
- Each task primary-targets ONE failure mode from the 12-code taxonomy: MISREAD, SCOPE_DRIFT, WRONG_ABS, STOP_EARLY, HAL_API.
- Target `bare` (no CLAUDE.md, no skills) pass rate: 0.4-0.6 across 5 runs per task. Validated post-implementation by running the bare harness 5 times per task and computing pass rates.
- Each task includes both a **functional test** (does the bug-fix or feature change work?) and a **discipline test** (scope-diff, regression, contract — depending on the failure mode being targeted).
- Setup time per task: <30s install + git init + baseline tag (matches existing `brownfield-fix-async-race`). No multi-GB image pulls, no Python version juggling.
- `task.yaml` metadata includes a new `primary_failure_mode: <CODE>` field so `failure_classifier`'s post-hoc labeling can be cross-validated against the designer's intent (calibration loop for the classifier itself — bonus signal for the thesis).

**Non-goals.**
- Remedy / answer-key CLAUDE.md snippets embedded in task.yaml. (User dropped this — thesis tests harness frameworks, not prompt variants.)
- SWE-bench import or any external-corpus integration. (Separate decision, intentionally deferred.)
- Greenfield additions. All 5 are brownfield; existing 2 greenfield tasks remain unchanged.
- Skills, spec-kit templates, or CLAUDE.md files in the tasks themselves. Those are properties of the harness being tested, not the task.
- A new failure-mode-validation tool. We rely on the existing `failure_classifier` RPC to label bare-failure runs and check that the designer-intended mode dominates.
- Hidden-tests / private-tests pattern. All tests are visible in the workspace (matches existing `brownfield-fix-async-race` convention; private tests come up in a future spec if we need them).

## 3. Approach

Each task follows the existing skeleton:

```
tasks/<task-id>/
  task.yaml          # id, prompt, test_cases, primary_failure_mode (NEW)
  setup.sh           # pip install -r requirements.txt; git init; tag baseline
  eval.sh            # runs pytest + scope-discipline shell test
  workspace files    # the actual repo (Python module, FastAPI app, tests/)
  requirements.txt
  pyproject.toml     # when needed for package layout
  tests/             # test files (visible to agent, but not all part of the eval gate)
```

Reuse `brownfield-fix-async-race` as the structural reference. Each task has 3-4 test cases:
- 1-2 functional tests (does the change behave correctly under load / edge cases / etc.)
- 1 contract/regression test (existing behavior not broken)
- 1 discipline test (scope-diff via `git diff --name-only baseline`, or analogous)

The 5 tasks form a matrix where each cell stresses a different failure mode under a different surface (FastAPI / SQLAlchemy / dataclass + alembic / Pydantic / generic Python module). This avoids the "everything is FastAPI" homogeneity that would let a harness over-fit to one stack.

## 4. Task designs

### 4.1 `brownfield-misread-hidden-contract` — MISREAD

**Surface:** FastAPI app with two source-of-truth files: `app/users.py` (route handler) and `openapi.yaml` (response schema). Plus a fixture test that validates real responses against the spec.

**Workspace:**
```
app/
  __init__.py
  main.py             # FastAPI app, mounts users router
  users.py            # GET /users/{id} → {id, name, email}
openapi.yaml          # response schema for /users/{id}; lists id, name, email
tests/
  test_users_basic.py # smoke: 200 + correct id/name/email
  test_spec_compliance.py  # validates real response matches openapi.yaml
  test_scope.sh       # git-diff: only app/users.py AND openapi.yaml may change
requirements.txt      # fastapi, httpx, pytest, pytest-asyncio, openapi-spec-validator
README.md             # describes the endpoint
```

**Task prompt** (what agent sees in `task_prompt`):
```
Add a `created_at` field to the GET /users/{id} response. The field
should be an ISO 8601 string ("2026-01-01T12:00:00Z"). For seed users,
return a fixed timestamp ("2024-01-01T00:00:00Z").

Constraints:
  - Function signature of get_user() may not change.
  - Existing fields (id, name, email) remain in the response.
```

**The trap:** The prompt mentions `app/users.py` implicitly (it talks about the endpoint) but says nothing about `openapi.yaml`. `test_spec_compliance.py` parses `openapi.yaml`, hits the real endpoint, validates against the schema. If the agent adds `created_at` to the handler but not to the spec, the spec-compliance test fails.

**Test cases:**
1. `test_users_basic.py` — GET /users/1 returns `created_at`; passes when handler updated.
2. `test_spec_compliance.py` — real response validates against openapi.yaml; passes ONLY when spec is also updated.
3. `test_scope.sh` — git diff names ⊆ {`app/users.py`, `openapi.yaml`}. Catches drift into other files.

**Why bare fails ~50%:** Some agents (especially the more thorough ones) `ls` the repo and notice `openapi.yaml`, then update it. Bare agents in "task says fix the endpoint" mode go straight to `users.py`, declare done, miss the spec. The MISREAD here is misreading the IMPLIED contract — the prompt doesn't say "update the spec" but the test enforces it.

**Why a structured harness (spec-kit) wins:** spec-kit's "investigate" phase forces an explicit step to enumerate related files before editing. Ralph's iterate-and-verify loop catches the spec-compliance test failure and rolls back to revisit.

---

### 4.2 `brownfield-scope-drift-tempting-cleanup` — SCOPE_DRIFT

**Surface:** A small Python billing module with an obvious bug + tempting cleanup opportunities elsewhere in the file.

**Workspace:**
```
app/
  __init__.py
  pricing.py          # 200-line file with the bug + 3 deprecated functions + an
                      # over-long helper that "could be refactored"
  models.py           # Order, Customer, LineItem dataclasses
  storage.py          # in-memory store
tests/
  test_charge_correctness.py  # exercises the buggy code path
  test_existing_behavior.py   # 6 regression tests on pricing edge cases
  test_scope.sh               # git diff: ONLY pricing.py may change
requirements.txt
README.md
```

**Task prompt:**
```
`pricing.py::compute_total` mishandles a discount that exceeds the
subtotal — it returns a negative total instead of zero. Fix the bug.

Constraints:
  - Modify only `app/pricing.py`.
  - Do not refactor, rename, or remove unrelated code in the same file.
  - All existing tests must continue to pass.
```

**The trap:** The 200-line `pricing.py` deliberately has:
- 3 functions marked `# DEPRECATED — slated for removal` that aren't used by tests
- A 40-line `_apply_loyalty_tier` function with copy-paste branches
- A `# TODO: refactor this mess` comment near the bug

Tempting for any "let me clean this up while I'm here" agent. The discipline test catches it: `git diff` must list only `pricing.py` AND the diff for `pricing.py` must not delete the DEPRECATED functions (verified by `grep -c '# DEPRECATED' app/pricing.py` matching the baseline count).

**Test cases:**
1. `test_charge_correctness.py` — negative-total bug is fixed; clamps to 0.
2. `test_existing_behavior.py` — 6 existing pricing scenarios still pass; if the agent refactored, behavior often changes by accident.
3. `test_scope.sh` — git-diff scope (single file) + `grep` count of `# DEPRECATED` markers unchanged.

**Why bare fails ~50%:** Cleanup-itch is a strong agent prior. Some bare agents stay disciplined; many don't.

**Why structured harness wins:** Spec-kit's "scope contract" in the spec forces explicit acknowledgment of what's in/out. Ralph's small-diff bias (iterate in small steps with verify between) reduces drift.

---

### 4.3 `brownfield-wrong-abs-async-throttle` — WRONG_ABS

**Surface:** Async FastAPI endpoint that needs rate limiting under concurrent load.

**Workspace:**
```
app/
  __init__.py
  main.py             # FastAPI app
  search.py           # async def search() that does expensive work
requirements.txt      # fastapi, httpx, pytest, pytest-asyncio
tests/
  test_throttle_under_load.py  # fires 50 concurrent requests, asserts:
                                 # - all complete within 10 seconds
                                 # - rate cap respected (no more than 10/s)
                                 # - throughput >= 8 req/s (not collapsed)
  test_search_correctness.py   # single-request behavior unchanged
  test_scope.sh                # diff: app/search.py only
```

**Task prompt:**
```
Add rate limiting to `app/search.py::search` so it processes at most
10 requests per second. Excess requests should wait, not be rejected.

Constraints:
  - The endpoint must remain async-friendly. The function signature
    stays `async def search(...) -> dict`.
  - Single requests should complete in <100ms (no artificial delay
    for low load).
  - Tests fire concurrent requests. Throughput must not collapse below
    8 req/s under sustained 10 req/s load.
```

**The trap:** Naive bare agents reach for `time.sleep(0.1)` (blocking) to enforce the rate. In an async event loop, `time.sleep` blocks the entire thread; concurrent requests stack up and timeout. Correct fix uses `asyncio.sleep` + a counter, or `aiolimiter.AsyncLimiter`, or a custom token bucket using `asyncio.Lock`.

**Test cases:**
1. `test_search_correctness.py` — single GET works, returns expected shape.
2. `test_throttle_under_load.py` — concurrent test that detects blocking-sleep (throughput collapses → test times out or asserts throughput floor).
3. `test_scope.sh` — single file diff.

**Why bare fails ~50%:** Half the bare agents use `time.sleep`. The other half (with better priors / more careful) reach for `asyncio.sleep` or a real limiter.

**Why structured harness wins:** Spec-kit's "consider concurrency model" prompt forces the agent to articulate "this is async". Ralph's iterate loop catches the under-load test failure and prompts a re-think.

---

### 4.4 `brownfield-stop-early-multi-step-migration` — STOP_EARLY

**Surface:** SQLAlchemy + Alembic project; a feature requires coordinated changes across model + serializer + migration + test fixtures.

**Workspace:**
```
app/
  __init__.py
  models.py           # User SQLAlchemy model
  schemas.py          # Pydantic User schema (the API serializer)
alembic/
  env.py
  versions/
    0001_initial.py   # creates users table
tests/
  conftest.py         # spins up sqlite + applies all alembic migrations
  test_user_creates.py            # creates a user with verified=True
  test_user_list_response.py      # GET /users → list includes 'verified'
  test_migrations_apply.py        # runs `alembic upgrade head` and inspects schema
  test_scope.sh                   # diff: app/models.py + app/schemas.py + alembic/versions/ only
requirements.txt      # sqlalchemy, alembic, pydantic, pytest
alembic.ini
```

**Task prompt:**
```
Add a `verified` boolean column to the User model (default False).

You must:
  1. Add the column to the SQLAlchemy model in app/models.py.
  2. Add the field to the Pydantic schema in app/schemas.py so the
     API serializes it.
  3. Add a new Alembic migration under alembic/versions/ that
     adds the column to the database.

All existing tests must continue to pass. New tests will verify the
column is present, defaults to False, and is included in API responses.
```

**The trap:** Bare agents typically do steps 1 and 2 (model + schema), run the test that checks "User has `verified` field" (passes because the in-memory model has it), declare done. They MISS step 3 (the alembic migration). `test_migrations_apply.py` runs `alembic upgrade head` against a fresh sqlite and inspects the schema; without the migration the column never makes it to the DB. `conftest.py` creates a fresh DB for each test via alembic, so the other tests also fail downstream.

**Test cases:**
1. `test_user_creates.py` — creating a user with `verified=True` doesn't error (model + schema).
2. `test_user_list_response.py` — API response includes `verified`.
3. `test_migrations_apply.py` — `alembic upgrade head` adds the column to the schema.
4. `test_scope.sh` — diff confined to model + schema + new migration file.

**Why bare fails ~50%:** The "I declared three steps in the prompt, the agent does the first two" pattern is the canonical STOP_EARLY. Some agents iterate and check; many don't.

**Why structured harness wins:** Ralph's iterate-until-all-tests-pass loop catches the missing migration. Spec-kit's task decomposition into discrete tasks ("write migration") with explicit checkboxes prevents skipping.

---

### 4.5 `brownfield-hal-api-pydantic-version` — HAL_API

**Surface:** Pydantic v2 project — the API has materially changed from v1, and stale training data leads agents to write v1 syntax.

**Workspace:**
```
app/
  __init__.py
  models.py           # Existing Pydantic v2 User model; needs a new validator
pyproject.toml        # pydantic = "^2.7"  ← v2 pinned, explicit
requirements.txt      # pydantic>=2.7,<3
tests/
  test_validator_works.py    # asserts validator rejects invalid emails
  test_imports_v2_api.py     # asserts the validator decorator is field_validator,
                              # not validator (regex scan of the file)
  test_scope.sh              # diff: only app/models.py
README.md
```

**Task prompt:**
```
Add a validator to the `User` model in `app/models.py` that rejects
email addresses without an `@` symbol. The validator should raise a
ValueError with the message "invalid email".

The project uses Pydantic 2.x — check pyproject.toml.
```

**The trap:** Bare agents with stale priors reach for Pydantic v1 syntax:
```python
from pydantic import validator
@validator("email")
def check_email(cls, v): ...
```

In Pydantic 2.x, `@validator` is deprecated; the new API is `@field_validator`:
```python
from pydantic import field_validator
@field_validator("email")
@classmethod
def check_email(cls, v): ...
```

Pydantic v2 with the v1 import path raises a DeprecationWarning, and our `test_imports_v2_api.py` greps the file for `from pydantic import validator` (asserts it's NOT there) and for `@field_validator(` (asserts it IS there).

**Test cases:**
1. `test_validator_works.py` — invalid email rejected; valid one accepted.
2. `test_imports_v2_api.py` — regex check enforcing v2 API usage.
3. `test_scope.sh` — single-file diff.

**Why bare fails ~50%:** Depends entirely on the model's training cutoff and whether it reads `pyproject.toml` before writing code. Many bare runs default to v1 syntax. Models with very recent training data, or agents that systematically check deps, succeed.

**Why structured harness wins:** Spec-kit's "verify dependencies and their versions" pre-coding step forces a `cat pyproject.toml`. Ralph's iterate loop catches the deprecation warning / import test failure and prompts a re-write.

---

## 5. Test discipline (common across tasks)

Every task includes:

1. **Functional tests** (1-2 per task) — the actual change works.
2. **Contract / regression tests** (1 per task where applicable) — existing behavior intact, schemas align.
3. **Scope-diff test** (`tests/test_scope.sh`) — `git diff --name-only baseline` must be a subset of an allow-list. `setup.sh` creates the `baseline` git tag (matches existing `brownfield-fix-async-race` pattern).

All test files are visible to the agent (it can read them). This matches the existing convention. If we later need private tests (to prevent agents from cheating by reading the test), a separate spec adds that pattern.

## 6. `primary_failure_mode` metadata + calibration validation

A new optional `task.yaml` field:

```yaml
primary_failure_mode: MISREAD   # one of the 12 FailureCode enum values
```

This is metadata only — it does NOT affect grading, scoring, or test execution. It serves two purposes:
1. Human-readable contract: the task author commits to which mode this task is calibrated for.
2. **Calibration validation** for the failure classifier: after the tasks are implemented, we run each task 5 times with the bare harness, collect the `failure_classifier` verdict on failing runs, and confirm that the designer-declared `primary_failure_mode` is the dominant verdict. This is a low-effort sanity check that the classifier and the task corpus agree.

The validation is a separate post-merge task; this spec defines the field, not the validation tool.

## 7. Risks

1. **Bare pass rates may not land in the 0.4-0.6 band on first try.** Mitigation: after implementing, run each task with bare × 5 and adjust difficulty (add/remove subtle traps in workspace) before declaring done. Document target vs measured in a calibration table in the PR description.
2. **Task drift over agent generations.** A task tuned for 50% on today's `bare` may become 80% on tomorrow's better model. Acceptable for the thesis (we report the pass rates for the snapshot, not as eternal calibration). Plan a re-calibration pass annually if Frameval becomes long-lived.
3. **Failure-mode classification overlap.** SCOPE_DRIFT and MISREAD overlap on tasks like `brownfield-misread-hidden-contract` (the agent both "misreads the contract" and "drifts away from the spec file"). The classifier's primary verdict will be one or the other; document the dominant intent in the task spec and accept a 20-30% classifier disagreement rate as noise.
4. **Workspace files grow.** Each task adds 10-20 small files plus a `requirements.txt`. Five new tasks ≈ 75 small files in `tasks/`. Acceptable; the directory is already structured for this.
5. **CI run time growth.** No CI run executes these tasks — they're for the experiment runner, not the test suite. CI is unaffected.
6. **Pydantic / SQLAlchemy / FastAPI minor versions drift.** Pin specific versions in each task's `requirements.txt` so the trap (e.g., Pydantic v1 → v2 API change) stays stable as upstream evolves.

## 8. Rollout

Single PR, one commit per task (5 commits) plus an opening commit for the `primary_failure_mode` metadata addition and any required harness/registry updates if the task list is enumerated anywhere in code.

1. Add `primary_failure_mode` to `task.yaml` parsing (engine) — optional field, no behavior change yet.
2. Create `tasks/brownfield-misread-hidden-contract/` with full workspace + tests.
3. Create `tasks/brownfield-scope-drift-tempting-cleanup/`.
4. Create `tasks/brownfield-wrong-abs-async-throttle/`.
5. Create `tasks/brownfield-stop-early-multi-step-migration/`.
6. Create `tasks/brownfield-hal-api-pydantic-version/`.
7. Manual calibration run (bare × 5 per task), adjust difficulty if any task is outside 0.4-0.6 band. Document measured pass rates in the PR description.

Each task commit is self-contained (one directory, no cross-deps) so reviewing one doesn't require understanding the others.
