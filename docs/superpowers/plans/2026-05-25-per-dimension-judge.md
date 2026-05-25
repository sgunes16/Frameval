# Per-Dimension Judge Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development. Steps use checkbox (`- [ ]`) syntax.

**Goal:** Replace the single all-dimensions judge call with five independent concurrent LLM calls (one per dimension) using detailed per-dim rubric prompts, so the judge actually distinguishes dimensions instead of anchoring to one number.

**Architecture:** `instructor.from_openai(AsyncOpenAI(...))` for the OpenAI-compatible providers, `instructor.from_anthropic(AsyncAnthropic(...))` for Anthropic. `asyncio.gather` over 5 per-dim calls inside the otherwise-sync `grade()` entry point via `asyncio.run(...)`. Each call returns a `DimensionVerdict(score, rationale)`. Raw responses tagged `dim=<name>;<payload>` so the frontend can group rationales per dim. Public dict shape preserved.

**Tech Stack:** Python 3.11 stdlib `asyncio`, `instructor`, `openai` SDK (already async-capable), Pydantic. React + TypeScript for the rationale rendering update.

**Spec:** [`docs/superpowers/specs/2026-05-25-per-dimension-judge-design.md`](../specs/2026-05-25-per-dimension-judge-design.md)

---

## File Structure

**Modify (Python):**
- `grader/llm_client.py` — add `async_client: bool = False` kwarg to `build_client`
- `grader/llm_judge/prompts.py` — replace `SYSTEM_PROMPT` with `DIMENSION_RUBRICS` dict (5 detailed per-dim prompts)
- `grader/llm_judge/grader.py` — rewrite to async fan-out
- `grader/llm_judge/tests/test_judge.py` — update mocks + assertions

**Modify (Frontend):**
- `frontend/src/components/grading-inspector/LLMJudgeCard.tsx` — per-dim rationales (parse `dim=<name>;` tag)
- `frontend/src/pages/runs/grading.test.tsx` — update mock `raw_judge_responses` shape

---

## Task 1: `grader/llm_client.py` — async client option

**Files:** Modify `grader/llm_client.py`

- [ ] **Step 1: Read the current file** to confirm structure

`cat /Users/mustafaselmangunes/Desktop/Frameval/grader/llm_client.py`

- [ ] **Step 2: Replace `build_client` with the dual sync/async version**

Find:
```python
def build_client(cfg: LLMClientConfig):
    """Returns an instructor-wrapped chat completions surface.
    ...
    """
    if cfg.provider == "anthropic":
        import instructor
        from anthropic import Anthropic
        if not cfg.api_key:
            raise RuntimeError("anthropic provider needs api_key")
        return instructor.from_anthropic(Anthropic(api_key=cfg.api_key)).messages

    import instructor
    from openai import OpenAI
    client_kwargs: dict[str, object] = {}
    if cfg.base_url:
        client_kwargs["base_url"] = cfg.base_url
    client_kwargs["api_key"] = cfg.api_key or "not-needed"
    return instructor.from_openai(OpenAI(**client_kwargs)).chat.completions
```

Replace with:

```python
def build_client(cfg: LLMClientConfig, *, async_client: bool = False):
    """Return an instructor-wrapped chat completions surface.

    When async_client=True, the returned `.create(...)` is awaitable —
    required for per-dimension judge calls run concurrently via
    asyncio.gather. The default (sync) surface preserves single-call
    callers (failure_classifier) without changes.
    """
    if cfg.provider == "anthropic":
        import instructor
        if not cfg.api_key:
            raise RuntimeError("anthropic provider needs api_key")
        if async_client:
            from anthropic import AsyncAnthropic
            return instructor.from_anthropic(AsyncAnthropic(api_key=cfg.api_key)).messages
        from anthropic import Anthropic
        return instructor.from_anthropic(Anthropic(api_key=cfg.api_key)).messages

    import instructor
    client_kwargs: dict[str, object] = {}
    if cfg.base_url:
        client_kwargs["base_url"] = cfg.base_url
    # Ollama accepts any non-empty key; OpenRouter / Z.ai / OpenAI require real keys.
    client_kwargs["api_key"] = cfg.api_key or "not-needed"
    if async_client:
        from openai import AsyncOpenAI
        return instructor.from_openai(AsyncOpenAI(**client_kwargs)).chat.completions
    from openai import OpenAI
    return instructor.from_openai(OpenAI(**client_kwargs)).chat.completions
```

- [ ] **Step 3: Verify existing tests still pass** (sync default unchanged):

`cd grader && uv run pytest tests/test_llm_client.py -v`
Expected: 5/5 PASS (no test changes).

- [ ] **Step 4: Verify failure_classifier still works** (uses sync default):

`cd grader && uv run pytest failure_classifier/ -v`
Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add grader/llm_client.py
git commit -m "Add async_client option to build_client for concurrent per-dim judge calls"
```

---

## Task 2: `grader/llm_judge/prompts.py` — detailed per-dim rubrics

**Files:** Modify `grader/llm_judge/prompts.py`

- [ ] **Step 1: Replace entire file contents**

Use the Write tool to replace `grader/llm_judge/prompts.py` with:

```python
from __future__ import annotations

import json
from typing import Any


_SHARED_TAIL = """## Output format

Return a JSON object with two fields:
- score: a float in [0.0, 10.0]
- rationale: a string up to 600 chars citing specific evidence from the
  output files, test results, or transcript. Reference concrete file
  names, function names, line numbers, or specific snippets where
  possible. Generic praise or generic criticism without evidence is a
  red flag in your own scoring — push yourself to be specific.

## Calibration

Be strict and calibrated. Use the full 0-10 range, not just 7-9. Do NOT
default to round numbers; if a dim feels like a 6.5 or 7.3, return that.
Most real-world agent outputs land between 3 and 7. Reserve 8-10 for
work you would ship to production with no modifications. Reserve 0-2
for output that does not address this dimension at all. If you find
yourself wanting to give the same score you gave to the previous run,
double-check that you aren't anchoring.

## Score anchors (use these to calibrate)

- 0-2: the output completely fails on this dimension (e.g. no error
  handling at all, code is unreadable mess, abandoned mid-implementation).
- 3-4: significant deficiency, multiple obvious problems, would not
  pass a junior code review.
- 5-6: acceptable baseline; works but has clear gaps a reviewer would
  flag.
- 7-8: solid professional work with minor polish issues.
- 9-10: production-ready, hard to find anything to improve."""


DIMENSION_RUBRICS: dict[str, str] = {
    "correctness": f"""You are a strict senior code reviewer scoring ONE
dimension of an AI coding agent's output: **CORRECTNESS**.

You will receive (a) the task the agent was asked to complete, (b) the
files the agent produced, (c) summary code metrics (test pass rate,
lint, type-check), and (d) a tail of the conversation transcript. Score
only correctness — ignore style, idioms, and error-handling polish
(those are scored by other reviewers in parallel).

## What CORRECTNESS measures

- Does the implementation actually do what the task asked, given the
  inputs specified, producing the outputs required?
- Does it pass the test cases supplied? (Use `test_pass_rate` in
  metrics; 1.0 = all pass, 0.0 = all fail.)
- Would an independent reviewer who knows the requirements verify the
  agent's logic as correct, not just plausible?
- Does the agent introduce regressions in unrelated code paths
  (especially relevant for brownfield tasks where existing tests
  matter)?

## Specific things to look for

- Tests claimed to pass but the implementation skips them, mocks them
  away, or only handles the happy path → big correctness penalty.
- Logic that "looks right" but has off-by-one, wrong comparator, wrong
  default, or wrong branch ordering → moderate penalty.
- Hallucinated APIs / non-existent functions the agent called → severe
  penalty (the code can't actually run as written).
- Solution that targets the wrong requirement (misread the spec) →
  severe penalty even if its own logic is internally consistent.
- Test_pass_rate near 1.0 + premature_completion=false + lint clean is
  strong (but not sufficient) evidence of correctness.

## What NOT to penalize here

- Ugly code, poor naming, lack of comments → that's maintainability.
- Missing error handling → that's error_handling.
- Failure to handle edge cases the task did not specify → that's
  completeness, not correctness.

{_SHARED_TAIL}""",

    "maintainability": f"""You are a strict senior code reviewer scoring
ONE dimension of an AI coding agent's output: **MAINTAINABILITY**.

Score only maintainability. Assume the code is correct (that's another
reviewer's job). Focus on whether a human developer who didn't write
this code could read it, modify it, and trust their modifications six
months from now.

## What MAINTAINABILITY measures

- Clarity of naming: variables, functions, classes, files
- Structure: single-responsibility, reasonable file size, no god
  objects, no copy-paste duplication
- Readability: control flow you can follow without re-reading,
  reasonable function lengths, complexity kept in check
- Dead code, commented-out blocks, scaffolding leftovers
- Type hints / type annotations where the language supports them
- Inline comments that explain *why*, not *what* (the latter is noise)
- Code that follows the surrounding file's existing style vs. clashing

## Specific things to look for

- Names like `x`, `data`, `tmp`, `result2`, `process`, `handle` for
  non-trivial things → maintainability penalty.
- Functions over ~50 lines with no clear breakdown → penalty.
- Magic numbers / strings sprinkled inline without explanation →
  penalty.
- Dead imports, unused variables, commented-out code → penalty.
- Inline comments that just restate the code → small penalty (noise).
- TODO / FIXME left in the output → moderate penalty (unfinished
  thinking).
- Multiple near-identical blocks (copy-paste) → penalty.

## What NOT to penalize here

- Failing tests → that's correctness.
- Missing error handling → that's error_handling.
- Non-idiomatic patterns (using a for-loop where a comprehension
  would be Pythonic) → that's best_practices.

{_SHARED_TAIL}""",

    "completeness": f"""You are a strict senior code reviewer scoring
ONE dimension of an AI coding agent's output: **COMPLETENESS**.

Score only completeness — did the agent finish what was asked, or did
it stop / skip / silently drop parts of the task?

## What COMPLETENESS measures

- Coverage of every requirement / acceptance criterion explicitly named
  in the task prompt
- All output files the task implied being created
- All test cases addressed (even if incorrectly — incorrectness is
  scored elsewhere; *missing* is what this dim cares about)
- The agent did NOT mark the task done while leaving stubs, TODOs, or
  "the rest is left as an exercise" comments
- For brownfield tasks: the agent addressed the actual file / function
  the task pointed at, not a tangentially related one
- premature_completion flag in metrics is a strong signal — true means
  the process grader detected the agent declaring victory too early

## Specific things to look for

- Stubs (`pass`, `raise NotImplementedError`, `// TODO`, `return null`
  on a function that needs a real implementation) → severe penalty.
- Task asked for N changes; only M < N landed → score scales with M/N.
- Task said "also update the docs" / "also add a migration" and only
  the code changed → penalty.
- premature_completion=true → strong negative signal.
- Agent stopped mid-implementation and gave up ("I can't proceed
  because...") without exhausting options → severe penalty.

## What NOT to penalize here

- Code that's present but wrong → correctness.
- Code that's present but ugly → maintainability.
- Code that's present but doesn't handle errors → error_handling.

{_SHARED_TAIL}""",

    "best_practices": f"""You are a strict senior code reviewer scoring
ONE dimension of an AI coding agent's output: **BEST PRACTICES**.

Score only best practices — does the code follow language and framework
idioms an experienced practitioner would expect? Assume correctness and
completeness (other reviewers).

## What BEST PRACTICES measures

- Idiomatic use of language features (e.g., Python context managers
  instead of try/finally for resource cleanup; Go's error returns
  instead of panics; TypeScript's narrow union types instead of `any`)
- Framework conventions where the task uses a framework (e.g., React
  hooks naming with `use` prefix; pytest fixtures over setUp/tearDown)
- File / module organization matching the surrounding project's style
- Standard-library use over reinventing helpers
- Avoiding deprecated APIs or known anti-patterns
- Async / concurrency idioms used correctly when the task requires
  them (asyncio.Lock vs threading.Lock; not blocking the event loop)
- Logging via the standard logger rather than print() in production
  code

## Specific things to look for

- `print(...)` for diagnostic output in non-trivial production code →
  penalty (use logging).
- Bare `except:` clauses, except Exception that swallow → penalty.
- Reinventing standard-library functionality → penalty.
- Mutating function defaults (`def foo(x=[])`) → penalty.
- Using `eval` / `exec` on user-influenced strings → severe penalty.
- `time.sleep` in async code → severe penalty.
- Returning sentinel values like `-1` or magic strings instead of
  raising or returning a typed Maybe → penalty.
- Type hints absent in a Python codebase that otherwise uses them →
  moderate penalty.
- Following project-local conventions (look at the surrounding file
  style in output_files) → positive.

## What NOT to penalize here

- Wrong answer → correctness.
- Bad names → maintainability.
- Missing error handling for failures → error_handling (overlap is
  acceptable; focus on the *idiomatic* angle here).

{_SHARED_TAIL}""",

    "error_handling": f"""You are a strict senior code reviewer scoring
ONE dimension of an AI coding agent's output: **ERROR HANDLING**.

Score only error handling — does the code anticipate and handle
failure modes the inputs and runtime can throw at it?

## What ERROR HANDLING measures

- Input validation: does the code check what it depends on before
  using it?
- Network / IO failures: does the code handle timeouts, retries, and
  partial responses? Or does it assume the happy path?
- Missing resources: files not found, env vars unset, optional deps
  missing → does the code degrade gracefully or crash with a useful
  message?
- Type errors: are sentinel-vs-None vs Exception choices coherent?
- Race conditions and concurrency: locks held correctly, no
  read-modify-write hazards in shared state
- Silent failure surface: does the code catch-and-swallow exceptions
  in a way that hides real bugs?
- Error messages: when the code does fail, is the message actionable
  for an operator?

## Specific things to look for

- `try: ... except: pass` with no logging → severe penalty (silent
  failure).
- `except Exception` catching too broadly without re-raising or
  reporting → moderate penalty.
- Reading user input / files / network without checking shape →
  penalty.
- Hard-coded assumptions about env (e.g., assumes a service is at
  localhost without a fallback) → penalty.
- For async code: missing await, fire-and-forget coroutines whose
  exceptions vanish → severe penalty.
- For concurrent code: shared mutable state without locks / atomic
  ops → severe penalty (one of the most common real bugs).
- Validation errors that produce useful messages ("expected X, got Y")
  → positive.
- Idempotency / retry safety where the task domain implies it →
  positive.

## What NOT to penalize here

- Wrong logic in the happy path → correctness.
- Unclear variable names → maintainability.
- Not using a particular library → best_practices.

{_SHARED_TAIL}""",
}


def render_user_prompt(
    *,
    code_grade: dict[str, Any],
    process_grade: dict[str, Any],
    task: dict[str, Any],
    output_files: list[dict[str, Any]],
    transcript_json: bytes,
) -> str:
    """Shared user prompt — identical across the five per-dim calls."""
    files_block = "\n\n".join(
        f"=== {f.get('path', '<unnamed>')} ===\n{_decode_content(f.get('content'))[:4000]}"
        for f in output_files[:10]
    )
    transcript_tail = _decode_content(transcript_json)[-3000:]
    metrics = {
        "test_pass_rate": code_grade.get("test_pass_rate"),
        "lint_score": code_grade.get("lint_score"),
        "type_check_pass": code_grade.get("type_check_pass"),
        "premature_completion": process_grade.get("premature_completion"),
    }
    return (
        f"# Task\n\n{task.get('prompt', '<no prompt>')}\n\n"
        f"# Code metrics\n\n{json.dumps(metrics, indent=2)}\n\n"
        f"# Output files (truncated)\n\n{files_block or '(no files)'}\n\n"
        f"# Transcript tail\n\n{transcript_tail or '(empty)'}\n"
    )


def _decode_content(blob: Any) -> str:
    if blob is None:
        return ""
    if isinstance(blob, bytes):
        try:
            return blob.decode("utf-8", errors="replace")
        except Exception:
            return ""
    return str(blob)
```

The old `SYSTEM_PROMPT` constant is removed.

- [ ] **Step 2: Quick sanity import**

`cd grader && python -c "from grader.llm_judge.prompts import DIMENSION_RUBRICS, render_user_prompt; print(list(DIMENSION_RUBRICS.keys()))"`
Expected: `['correctness', 'maintainability', 'completeness', 'best_practices', 'error_handling']`

- [ ] **Step 3: Commit**

```bash
git add grader/llm_judge/prompts.py
git commit -m "Add detailed per-dimension rubric prompts (DIMENSION_RUBRICS)"
```

---

## Task 3: `grader/llm_judge/grader.py` — async fan-out

**Files:** Modify `grader/llm_judge/grader.py`

- [ ] **Step 1: Replace entire file contents**

Use Write to replace `grader/llm_judge/grader.py` with:

```python
from __future__ import annotations

import asyncio
import logging
from typing import Any

from pydantic import BaseModel, Field

from grader.llm_client import build_client, load_config
from grader.llm_judge.prompts import DIMENSION_RUBRICS, render_user_prompt

logger = logging.getLogger(__name__)


class DimensionVerdict(BaseModel):
    """One per-dimension LLM call's structured output."""
    score: float = Field(ge=0.0, le=10.0)
    rationale: str = Field(max_length=600)


_DIMENSIONS: tuple[str, ...] = (
    "correctness",
    "maintainability",
    "completeness",
    "best_practices",
    "error_handling",
)


def grade(
    code_grade: dict[str, Any],
    process_grade: dict[str, Any],
    task: dict[str, Any] | None = None,
    output_files: list[dict[str, Any]] | None = None,
    transcript_json: bytes | None = None,
    config_override: Any = None,
) -> dict[str, Any]:
    """Score one run on five dimensions via FIVE concurrent LLM calls.

    Public API and return shape are unchanged from the prior single-call
    implementation. raw_responses now contains five entries, each tagged
    'dim=<name>;<payload>' where payload is either a JSON-encoded
    DimensionVerdict or a 'judge_unavailable: <reason>' sentinel.
    """
    try:
        cfg = load_config(config_override)
    except Exception as exc:
        logger.warning("judge config load failed: %s", exc)
        return _all_dims_failed(str(exc))
    return asyncio.run(
        _grade_async(cfg, code_grade, process_grade, task, output_files, transcript_json)
    )


async def _grade_async(
    cfg,
    code_grade: dict[str, Any],
    process_grade: dict[str, Any],
    task: dict[str, Any] | None,
    output_files: list[dict[str, Any]] | None,
    transcript_json: bytes | None,
) -> dict[str, Any]:
    try:
        client = build_client(cfg, async_client=True)
    except Exception as exc:
        logger.warning("judge async client init failed: %s", exc)
        return _all_dims_failed(str(exc))

    user_prompt = render_user_prompt(
        code_grade=code_grade,
        process_grade=process_grade,
        task=task or {},
        output_files=output_files or [],
        transcript_json=transcript_json or b"",
    )

    tasks = [_score_one_dim(client, cfg.model, dim, user_prompt) for dim in _DIMENSIONS]
    # return_exceptions=False because _score_one_dim already catches
    # everything internally and never raises out.
    results = await asyncio.gather(*tasks, return_exceptions=False)

    scores = {dim: results[i][0] for i, dim in enumerate(_DIMENSIONS)}
    raw_responses = [results[i][1] for i in range(len(_DIMENSIONS))]
    return {
        **scores,
        "irr_alpha": 0.0,
        "raw_responses": raw_responses,
    }


async def _score_one_dim(
    client, model: str, dim: str, user_prompt: str,
) -> tuple[float, str]:
    """Run one dim's LLM call. Returns (score, raw_response_or_sentinel).
    Never raises — failures become 0.0 + a judge_unavailable sentinel so
    one failing dim doesn't take down the whole grade.
    """
    try:
        verdict: DimensionVerdict = await client.create(
            model=model,
            response_model=DimensionVerdict,
            max_retries=2,
            max_tokens=512,
            messages=[
                {"role": "system", "content": DIMENSION_RUBRICS[dim]},
                {"role": "user", "content": user_prompt},
            ],
        )
        return verdict.score, _tag_response(dim, verdict.model_dump_json())
    except Exception as exc:
        logger.warning("judge dim=%s call failed: %s", dim, exc)
        return 0.0, _tag_response(dim, f"judge_unavailable: {str(exc)[:300]}")


def _tag_response(dim: str, payload: str) -> str:
    """Prefix each raw_response with its dim so the UI can group them.

    Format: 'dim=<name>;<payload>'. Frontend's extractRationalesByDim
    parses this prefix to map rationales to their dimension.
    """
    return f"dim={dim};{payload}"


def _all_dims_failed(reason: str) -> dict[str, Any]:
    """Sentinel when the whole pipeline fails (config / client init) —
    every dim is zero and every raw_response carries the same reason.
    """
    short = reason[:300]
    return {
        **{dim: 0.0 for dim in _DIMENSIONS},
        "irr_alpha": 0.0,
        "raw_responses": [f"dim={dim};judge_unavailable: {short}" for dim in _DIMENSIONS],
    }
```

The old `JudgeResult` and `_failed_judge_result` are gone. Anything that imported them needs updating — but the only known importer is the test file (next task).

- [ ] **Step 2: Verify imports compile**

`cd grader && python -c "from grader.llm_judge.grader import grade, DimensionVerdict; print('ok')"`
Expected: `ok`.

- [ ] **Step 3: Commit (tests will fail until Task 4 lands)**

```bash
git add grader/llm_judge/grader.py
git commit -m "Rewrite llm_judge as async per-dimension fan-out (5 concurrent calls)"
```

---

## Task 4: Update `grader/llm_judge/tests/test_judge.py`

**Files:** Modify `grader/llm_judge/tests/test_judge.py`

- [ ] **Step 1: Replace entire file contents**

Use Write to replace `grader/llm_judge/tests/test_judge.py` with:

```python
from __future__ import annotations

import json
from types import SimpleNamespace
from unittest.mock import patch

import pytest

from grader.llm_judge.grader import DimensionVerdict, _DIMENSIONS, grade


class _AsyncStub:
    """Async stub that returns a different result per dim.

    Looks at the system prompt's first lines to identify which dim is
    being scored (rubrics start with 'You are a strict senior code
    reviewer scoring ONE dimension of an AI coding agent's output:
    **CORRECTNESS**.' etc.). Maps to the configured result.
    """

    def __init__(self, by_dim: dict[str, DimensionVerdict | Exception]):
        self.by_dim = by_dim
        self.calls: list[str] = []

    async def create(self, *, model, response_model, max_retries, max_tokens, messages):
        sys = messages[0]["content"]
        for dim in _DIMENSIONS:
            tag = f"**{dim.upper()}**"
            if tag in sys:
                self.calls.append(dim)
                result = self.by_dim[dim]
                if isinstance(result, Exception):
                    raise result
                return result
        raise AssertionError(f"no dim tag matched in system prompt: {sys[:120]}")


def _v(score: float, rationale: str = "evidence cited") -> DimensionVerdict:
    return DimensionVerdict(score=score, rationale=rationale)


def test_grade_happy_path_all_five():
    stub = _AsyncStub({
        "correctness": _v(8.0, "fix matches spec exactly"),
        "maintainability": _v(7.5, "names are clear"),
        "completeness": _v(9.0, "all 3 requirements covered"),
        "best_practices": _v(6.0, "uses sync lock in async code"),
        "error_handling": _v(5.5, "happy path only"),
    })
    with patch("grader.llm_judge.grader.build_client", return_value=stub):
        out = grade(
            code_grade={"test_pass_rate": 0.8, "lint_score": 7.0, "type_check_pass": True},
            process_grade={"premature_completion": False},
            task={"prompt": "Fix the race condition"},
            output_files=[{"path": "src/main.py", "content": b"x = 1"}],
            transcript_json=b"{}",
        )

    # Distinct scores — proves we are not anchoring on one number.
    assert out["correctness"] == 8.0
    assert out["maintainability"] == 7.5
    assert out["completeness"] == 9.0
    assert out["best_practices"] == 6.0
    assert out["error_handling"] == 5.5
    assert out["irr_alpha"] == 0.0
    # Five raw_responses, each tagged with its dim.
    assert len(out["raw_responses"]) == 5
    for entry, dim in zip(out["raw_responses"], _DIMENSIONS):
        assert entry.startswith(f"dim={dim};")
        # The non-sentinel payload is JSON-encoded DimensionVerdict.
        payload = entry[len(f"dim={dim};"):]
        parsed = json.loads(payload)
        assert "score" in parsed
        assert "rationale" in parsed
    # All five dims were actually called.
    assert sorted(stub.calls) == sorted(_DIMENSIONS)


def test_grade_one_dim_fails_others_succeed():
    stub = _AsyncStub({
        "correctness": RuntimeError("boom on correctness"),
        "maintainability": _v(7.0),
        "completeness": _v(8.0),
        "best_practices": _v(6.0),
        "error_handling": _v(5.0),
    })
    with patch("grader.llm_judge.grader.build_client", return_value=stub):
        out = grade(
            code_grade={}, process_grade={}, task={}, output_files=[], transcript_json=b"",
        )
    # The failing dim is zero with a sentinel raw_response.
    assert out["correctness"] == 0.0
    correctness_raw = next(r for r in out["raw_responses"] if r.startswith("dim=correctness;"))
    assert "judge_unavailable: boom on correctness" in correctness_raw
    # Other dims still scored normally.
    assert out["maintainability"] == 7.0
    assert out["completeness"] == 8.0
    assert out["best_practices"] == 6.0
    assert out["error_handling"] == 5.0


def test_grade_all_dims_fail_returns_sentinels():
    stub = _AsyncStub({
        dim: RuntimeError(f"boom-{dim}") for dim in _DIMENSIONS
    })
    with patch("grader.llm_judge.grader.build_client", return_value=stub):
        out = grade(
            code_grade={}, process_grade={}, task={}, output_files=[], transcript_json=b"",
        )
    for dim in _DIMENSIONS:
        assert out[dim] == 0.0
    # All five raw_responses are sentinels.
    assert all("judge_unavailable: boom-" in r for r in out["raw_responses"])


def test_grade_client_init_failure_returns_all_dims_failed():
    """When build_client raises, _all_dims_failed is returned without any
    per-dim LLM call attempt. All five raw_responses carry the same reason."""
    with patch("grader.llm_judge.grader.build_client", side_effect=RuntimeError("no key")):
        out = grade(
            code_grade={}, process_grade={}, task={}, output_files=[], transcript_json=b"",
        )
    for dim in _DIMENSIONS:
        assert out[dim] == 0.0
        matching = next(r for r in out["raw_responses"] if r.startswith(f"dim={dim};"))
        assert "judge_unavailable: no key" in matching


def test_grade_config_override_beats_env(monkeypatch):
    """Per-call override (e.g., from the engine's JudgeConfig proto) wins
    over env vars in load_config — verified via the captured cfg."""
    monkeypatch.setenv("FRAMEVAL_LLM_PROVIDER", "openrouter")
    stub = _AsyncStub({dim: _v(5.0) for dim in _DIMENSIONS})
    captured: dict[str, str] = {}

    def fake_build(cfg, *, async_client):
        captured["provider"] = cfg.provider
        captured["model"] = cfg.model
        captured["async_client"] = async_client
        return stub

    with patch("grader.llm_judge.grader.build_client", side_effect=fake_build):
        override = SimpleNamespace(provider="zai", model="glm-4.6", api_key="zk-test")
        grade(
            code_grade={}, process_grade={}, task={}, output_files=[], transcript_json=b"",
            config_override=override,
        )
    assert captured["provider"] == "zai"
    assert captured["model"] == "glm-4.6"
    assert captured["async_client"] is True
```

- [ ] **Step 2: Run the new tests**

`cd grader && uv run pytest llm_judge/tests/test_judge.py -v`
Expected: 5/5 PASS.

- [ ] **Step 3: Full grader suite — no regressions**

`cd grader && uv run pytest 2>&1 | tail -8`
Expected: all PASS (the count will be 5 instead of 4 for judge tests, so total goes from 35 to 36 — confirm).

- [ ] **Step 4: Commit**

```bash
git add grader/llm_judge/tests/test_judge.py
git commit -m "Update judge tests for per-dimension async fan-out"
```

---

## Task 5: Frontend — per-dim rationale rendering

**Files:** Modify `frontend/src/components/grading-inspector/LLMJudgeCard.tsx`

- [ ] **Step 1: Replace `extractRationale` with per-dim parser**

Find the current `extractRationale` helper in `LLMJudgeCard.tsx` and replace with:

```tsx
// Map raw_judge_responses (5 entries, each tagged "dim=<name>;<payload>") to
// {dim: rationale}. Skips sentinels (judge_unavailable: ...) and unparseable
// entries silently — the dim's bar still shows; only the rationale block is
// suppressed for that dim.
function extractRationalesByDim(raw: string[] | undefined): Record<string, string> {
  const out: Record<string, string> = {};
  for (const entry of raw ?? []) {
    const match = /^dim=([^;]+);([\s\S]*)$/.exec(entry);
    if (!match) continue;
    const [, dim, payload] = match;
    try {
      const parsed = JSON.parse(payload);
      if (typeof parsed?.rationale === 'string' && parsed.rationale.length > 0) {
        out[dim] = parsed.rationale;
      }
    } catch {
      /* sentinel or non-JSON; skip */
    }
  }
  return out;
}
```

- [ ] **Step 2: Update the component to render per-dim rationales**

In the same file, change the `rationale` line near the top of the component:

```tsx
const rationale = extractRationale(grade.raw_judge_responses);
```

to:

```tsx
const rationales = extractRationalesByDim(grade.raw_judge_responses);
```

Then find the JSX block that currently renders the single rationale:

```tsx
{rationale && (
  <div className="mt-3 border-t border-border pt-3">
    <div className="mb-1 text-xs uppercase tracking-wider text-fg-muted">Rationale</div>
    <p className="text-sm text-fg">{rationale}</p>
  </div>
)}
```

Replace with:

```tsx
{Object.keys(rationales).length > 0 && (
  <div className="mt-3 space-y-2 border-t border-border pt-3">
    <div className="text-xs uppercase tracking-wider text-fg-muted">Per-dimension rationale</div>
    {Object.entries(rationales).map(([dim, text]) => (
      <div key={dim}>
        <div className="text-xs font-medium capitalize text-fg-muted">{dim.replace(/_/g, ' ')}</div>
        <p className="text-sm text-fg">{text}</p>
      </div>
    ))}
  </div>
)}
```

The "show raw response (debug)" pre block stays as-is.

- [ ] **Step 3: Verify build**

`cd frontend && npm run build 2>&1 | tail -8`
Expected: clean TS + vite build.

- [ ] **Step 4: Commit**

```bash
git add frontend/src/components/grading-inspector/LLMJudgeCard.tsx
git commit -m "Render per-dimension rationales in Grading Inspector"
```

---

## Task 6: Update frontend test fixture

**Files:** Modify `frontend/src/pages/runs/grading.test.tsx`

- [ ] **Step 1: Update the mock `raw_judge_responses` to the tagged shape**

Find the existing mock entry:

```tsx
raw_judge_responses: ['{"correctness":8,"rationale":"solid solution"}'],
```

Replace with five tagged entries:

```tsx
raw_judge_responses: [
  'dim=correctness;{"score":8.0,"rationale":"solid solution on correctness"}',
  'dim=maintainability;{"score":7.0,"rationale":"clean names"}',
  'dim=completeness;{"score":9.0,"rationale":"all requirements covered"}',
  'dim=best_practices;{"score":6.0,"rationale":"sync lock in async code"}',
  'dim=error_handling;{"score":5.0,"rationale":"happy path only"}',
],
```

Update the rationale assertion (if it was looking for `"solid solution"`, that text now lives in the correctness dim only):

```tsx
expect(screen.getByText(/solid solution on correctness/)).toBeInTheDocument();
```

- [ ] **Step 2: Run the test**

`cd frontend && npm test -- grading.test 2>&1 | tail -10`
Expected: 1/1 PASS.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/pages/runs/grading.test.tsx
git commit -m "Update grading test fixture for per-dimension raw_responses shape"
```

---

## Self-review (run before declaring plan done)

**Spec coverage:**
- §4.1 (prompts) → Task 2
- §4.2 (async client) → Task 1
- §4.3 (async fan-out) → Task 3
- §4.4 (tests) → Task 4
- §4.5 (frontend per-dim rationales) → Task 5 (+ Task 6 fixture)
- §5 (testing) → Tasks 4 and 6
- §6 (risks) → addressed in code (sentinels per dim, async-in-sync gRPC handler)
- §7 (rollout) → Tasks 1-6 ordered

All covered.

**Placeholder check:** No TBD/TODO/"fill in later". All code blocks complete.

**Type consistency:**
- `DimensionVerdict(score, rationale)` defined in grader.py, imported in test file. ✓
- `_DIMENSIONS` tuple defined in grader.py, imported in test. ✓
- `extractRationalesByDim` (frontend) parses `dim=<name>;<payload>` exactly matching what `_tag_response` emits in grader.py. ✓
- `build_client(cfg, *, async_client=False)` — sync default preserves failure_classifier compat. ✓

Plan ready.
