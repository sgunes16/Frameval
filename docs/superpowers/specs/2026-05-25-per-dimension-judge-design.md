# Per-Dimension LLM Judge — Design

**Status:** Draft
**Date:** 2026-05-25
**Owner:** sgunes16
**Related:** [[2026-05-23-llm-judge-design]], [[2026-05-25-grading-inspector-design]], `grader/llm_judge/grader.py`, `grader/llm_judge/prompts.py`

## 1. Motivation

The current judge makes **one** LLM call asking for all five dimensions at once. Real-world output on `qwen/qwen3-coder:free` for a typical fix:

```
Correctness        9.00
Maintainability    9.00
Completeness       9.00
Best practices     9.00
Error handling     7.00
Rationale: "The fix correctly uses an asyncio.Lock... However, error
handling is not improved beyond the existing behavior..."
```

Four dimensions identical, rationale only explains the outlier. This is classic anchoring: when the model produces five scores in a single forward pass, it tends to copy-paste the first one into the others. The output looks confident but carries almost no per-dimension signal. The SYSTEM_PROMPT's "Most real-world outputs are 4-7" calibration text is overridden by the join-distribution prior.

User pain (verbatim): *"her boyut için ayrı run atsın knk bu bana çok ezik geldi açıkçası sallıyo bence"* — separate LLM call per dimension; the current one-shot is weak / making things up.

Beyond UX, this matters for the thesis: the judge needs to actually distinguish dimensions for any experimental comparison to mean something. If "context X improves maintainability" can never be detected because the judge always copies correctness into maintainability, the whole evaluation framework is unreliable.

## 2. Goals & non-goals

**Goals.**
- Five independent LLM calls per `judge_grade()`, one per dimension (correctness, maintainability, completeness, best_practices, error_handling).
- Each call uses a focused, dimension-specific system prompt + a shared task-context user prompt.
- Each call returns a `DimensionVerdict(score: float, rationale: str)` Pydantic model — structured, validated, retried via `instructor` as before.
- **Calls run concurrently** via `asyncio.gather` so total latency stays comparable to the current single-call path (one slow free-tier call dominates). Sequential fallback for environments where async isn't viable is NOT in scope.
- Existing return shape preserved: dict with five float fields + `irr_alpha=0.0` + `raw_responses=[5 JSON strings]`. Grading Inspector's `extractRationale` already iterates the list and pulls the first parseable rationale; we'll extend it to surface all five (one per dim).
- Same fallback semantics: any per-dim call failing produces `0.0` for that dim with a `judge_unavailable: <reason>` entry in `raw_responses`. Other dims that succeeded still report real scores.
- Tests updated: judge tests cover happy path with 5 fake calls, one-dim-fails-others-succeed, all-dims-fail.

**Non-goals.**
- Streaming. Already deferred in the Grading Inspector spec; still out of scope.
- Cross-model judging or IRR. Still single-model; `irr_alpha=0.0` stays.
- Reworking the failure_classifier. That's a different problem (multi-label classification, not per-dimension scoring) — stays as one call.
- Caching judge results per (run, dim) pair. Each grade always runs all 5.
- Configurable dimensions. The 5 dimensions stay hard-coded; rubric override is a future spec.
- Sequential mode. We commit to async/parallel — `asyncio` is in the stdlib, no new deps.

## 3. Approach

Three substitutions:

1. **`grader/llm_judge/prompts.py`** — replace `SYSTEM_PROMPT` (single all-dim) and `render_user_prompt` with:
   - `DIMENSION_RUBRICS: dict[str, str]` — one per-dim system prompt focused on a single scoring axis.
   - `render_user_prompt(...)` — unchanged shape (task + metrics + files + transcript tail); same renderer used for every dim call.
2. **`grader/llm_judge/grader.py`** — replace one `client.create` call with an `asyncio.gather` over 5 calls. Use `instructor.from_openai(AsyncOpenAI(...))` for the OpenAI-compatible providers (OpenRouter / Z.ai / Ollama / OpenAI). Use `instructor.from_anthropic(AsyncAnthropic(...))` for the anthropic branch — both `instructor` factories support async clients.
3. **`grader/llm_client.py`** — `build_client` gains an `async_client: bool = False` parameter. When True, returns the async-wrapped `instructor` surface. Default (False) preserves the old sync behavior so the failure_classifier — which still uses one call — keeps working without changes.

The gRPC handler in `grader/server.py` stays synchronous. The judge's async work runs via `asyncio.run(_grade_async(...))` inside the handler's worker thread (each gRPC call gets its own thread per `ThreadPoolExecutor(max_workers=8)`; creating a new event loop per call is safe and idiomatic for sync-on-the-outside, async-on-the-inside).

## 4. Targeted changes

### 4.1 `grader/llm_judge/prompts.py` — per-dim rubrics

Replace `SYSTEM_PROMPT` with `DIMENSION_RUBRICS`. Each rubric instructs the LLM to score ONE dimension and explain its reasoning in 200-400 chars. Keeping the user prompt renderer unchanged means the task/code-metrics/files/transcript context is identical across the 5 calls — only the framing changes.

```python
# grader/llm_judge/prompts.py
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
    """Unchanged from the prior single-call shape. Shared across all dim calls."""
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

The old `SYSTEM_PROMPT` constant is deleted. Any test that imports it must move to `DIMENSION_RUBRICS["correctness"]` (or whatever dim it was exercising).

### 4.2 `grader/llm_client.py` — async client option

Add an `async_client` flag. The OpenAI-compatible branch uses `AsyncOpenAI`; the Anthropic branch uses `AsyncAnthropic`. Both work with `instructor.from_openai` / `instructor.from_anthropic` — `instructor` auto-detects the client kind.

```python
def build_client(cfg: LLMClientConfig, *, async_client: bool = False):
    """Return an instructor-wrapped chat completions surface.

    When async_client=True, the returned surface exposes `.create(...)` as
    an awaitable — required for per-dimension judge calls that we gather
    concurrently. The default (sync) surface is preserved so failure_
    classifier and other single-call callers don't need changes.
    """
    if cfg.provider == "anthropic":
        import instructor
        if async_client:
            from anthropic import AsyncAnthropic
            if not cfg.api_key:
                raise RuntimeError("anthropic provider needs api_key")
            return instructor.from_anthropic(AsyncAnthropic(api_key=cfg.api_key)).messages
        from anthropic import Anthropic
        if not cfg.api_key:
            raise RuntimeError("anthropic provider needs api_key")
        return instructor.from_anthropic(Anthropic(api_key=cfg.api_key)).messages

    import instructor
    client_kwargs: dict[str, object] = {}
    if cfg.base_url:
        client_kwargs["base_url"] = cfg.base_url
    client_kwargs["api_key"] = cfg.api_key or "not-needed"
    if async_client:
        from openai import AsyncOpenAI
        return instructor.from_openai(AsyncOpenAI(**client_kwargs)).chat.completions
    from openai import OpenAI
    return instructor.from_openai(OpenAI(**client_kwargs)).chat.completions
```

### 4.3 `grader/llm_judge/grader.py` — async fan-out

Rewrite to fan out 5 dim calls via `asyncio.gather`. Sync `grade()` entry point preserved so `grader/server.py` doesn't change.

```python
from __future__ import annotations

import asyncio
import logging
from typing import Any

from pydantic import BaseModel, Field

from grader.llm_client import build_client, load_config
from grader.llm_judge.prompts import DIMENSION_RUBRICS, render_user_prompt

logger = logging.getLogger(__name__)


# A single-dim verdict — what each per-dim LLM call returns.
class DimensionVerdict(BaseModel):
    score: float = Field(ge=0.0, le=10.0)
    rationale: str = Field(max_length=600)


_DIMENSIONS = ("correctness", "maintainability", "completeness", "best_practices", "error_handling")


def grade(
    code_grade: dict[str, Any],
    process_grade: dict[str, Any],
    task: dict[str, Any] | None = None,
    output_files: list[dict[str, Any]] | None = None,
    transcript_json: bytes | None = None,
    config_override: Any = None,
) -> dict[str, Any]:
    """Score one run on five dimensions via FIVE concurrent LLM calls.

    Public API unchanged from the prior single-call implementation. The
    return dict has the same keys and shape; raw_responses now contains
    five entries (one JSON-encoded DimensionVerdict per dim, or a
    judge_unavailable sentinel for any dim that failed).
    """
    try:
        cfg = load_config(config_override)
    except Exception as exc:
        logger.warning("judge config load failed: %s", exc)
        return _all_dims_failed(str(exc))
    return asyncio.run(_grade_async(cfg, code_grade, process_grade, task, output_files, transcript_json))


async def _grade_async(cfg, code_grade, process_grade, task, output_files, transcript_json) -> dict[str, Any]:
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
    results = await asyncio.gather(*tasks, return_exceptions=False)
    # results is a list of (score: float, raw_json_or_sentinel: str), one per dim, in _DIMENSIONS order.

    scores = {dim: results[i][0] for i, dim in enumerate(_DIMENSIONS)}
    raw_responses = [results[i][1] for i in range(len(_DIMENSIONS))]
    return {
        **scores,
        "irr_alpha": 0.0,
        "raw_responses": raw_responses,
    }


async def _score_one_dim(client, model: str, dim: str, user_prompt: str) -> tuple[float, str]:
    """Run one dim's LLM call. Returns (score, raw_response_or_sentinel).
    Never raises — failures become 0.0 + a judge_unavailable sentinel.
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

    Frontend's extractRationale parses each entry as JSON; the dim tag is
    stripped before parsing. Format: 'dim=<name>;<payload>'.
    """
    return f"dim={dim};{payload}"


def _all_dims_failed(reason: str) -> dict[str, Any]:
    """Sentinel when even the config / client init fails — every dim is
    zero and every raw_response carries the same reason. Preserves the
    contract that the orchestrator always gets a complete dict back."""
    short = reason[:300]
    return {
        **{dim: 0.0 for dim in _DIMENSIONS},
        "irr_alpha": 0.0,
        "raw_responses": [f"dim={dim};judge_unavailable: {short}" for dim in _DIMENSIONS],
    }
```

The `_tag_response` wrapper lets the frontend group multiple rationales by dimension. See §4.5 for the frontend update.

### 4.4 Tests — `grader/llm_judge/tests/test_judge.py`

The existing 4 tests mock `build_client`; they need to mock the *async* surface now. Replace the `_fake_client_returning` helper with an async-compatible stub:

```python
class _AsyncStub:
    def __init__(self, by_dim):
        self.by_dim = by_dim
    async def create(self, *, model, response_model, max_retries, max_tokens, messages):
        # System prompt's first line contains the dim name; pick the result by dim.
        sys = messages[0]["content"]
        for dim, result in self.by_dim.items():
            if dim.upper() in sys.split("\n", 1)[0]:
                if isinstance(result, Exception):
                    raise result
                return result
        raise AssertionError(f"no result mocked for system prompt: {sys[:80]}")
```

New / updated test cases:
1. **`test_grade_happy_path_all_five`** — stub returns a different `DimensionVerdict` per dim, asserts the dict has the right per-dim scores (not all the same value), and raw_responses has 5 entries each starting with `dim=<name>;`.
2. **`test_grade_one_dim_fails_others_succeed`** — stub raises on `correctness` but returns valid `DimensionVerdict` for the other 4. Asserts `correctness == 0.0`, others non-zero, `raw_responses[0]` starts with `dim=correctness;judge_unavailable:`.
3. **`test_grade_all_dims_fail_returns_sentinel`** — stub raises on every dim. Asserts all 5 scores are 0.0, all 5 raw_responses are sentinels.
4. **`test_grade_config_override_beats_env`** — preserved; capture cfg.provider/model on first dim call.

Run `cd grader && uv run pytest llm_judge/tests/ -v` after the rewrite — expect 4 PASS.

### 4.5 Frontend — `LLMJudgeCard.tsx` per-dim rationales

Currently `extractRationale` returns the first parseable rationale from `raw_judge_responses`. With per-dim responses, surface all five — one per dimension, labeled.

Update `frontend/src/components/grading-inspector/LLMJudgeCard.tsx`:

```tsx
// Map raw_judge_responses (5 entries, each tagged "dim=<name>;<payload>") to
// {dim: rationale}. Skips sentinels (judge_unavailable: ...) and unparseable
// entries silently — the dim's bar still shows; only the rationale block is
// suppressed for that dim.
function extractRationalesByDim(raw: string[] | undefined): Record<string, string> {
  const out: Record<string, string> = {};
  for (const entry of raw ?? []) {
    const match = /^dim=([^;]+);(.*)$/s.exec(entry);
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

Then render rationales section as a list per dimension, not a single paragraph. Existing layout (5 dim bars stacked) stays — just below the bars, replace the single rationale `<p>` with a list of per-dim rationales:

```tsx
{Object.keys(rationales).length > 0 && (
  <div className="mt-3 space-y-2 border-t border-border pt-3">
    <div className="text-xs uppercase tracking-wider text-fg-muted">Per-dimension rationale</div>
    {Object.entries(rationales).map(([dim, text]) => (
      <div key={dim}>
        <div className="text-xs font-medium text-fg-muted capitalize">{dim.replace('_', ' ')}</div>
        <p className="text-sm text-fg">{text}</p>
      </div>
    ))}
  </div>
)}
```

The raw debug `<pre>` block stays as-is (already shows full `raw_judge_responses` JSON).

## 5. Testing

**Python:** 4 judge tests updated + run green. Full grader suite stays at 35 passed (we replace tests, not add).

**Frontend:** existing `grading.test.tsx` continues to pass — update mock `raw_judge_responses` to the new tagged shape (`dim=correctness;{"score":8,"rationale":"..."}`). The single rationale assertion changes to one of the per-dim rationale strings.

**Manual smoke:**
1. Re-run the same experiment that produced the 9/9/9/9/7 anchored output.
2. Open `/runs/<id>/grading`.
3. Expect five distinct scores (not all identical) AND five separate rationales below the bars, one per dimension.
4. If the new scores are still all 9, the issue is the model — switch to a stronger judge model (e.g. via OpenRouter free tier: `deepseek/deepseek-chat:free`, `meta-llama/llama-3.3-70b-instruct:free`) and re-test.

## 6. Risks

1. **5x cost when judge runs on paid providers.** Free tiers swallow it; paid providers (OpenAI, Anthropic, Z.ai paid plans) get 5x billing. Document in CLAUDE.md. Acceptable for a thesis-grade local-first tool — users on paid plans can disable the judge.
2. **Free-tier rate limits.** OpenRouter free models throttle around 20 req/min. Five concurrent calls per grade × `FRAMEVAL_MAX_CONCURRENT=3` = 15 req/min worst case — fits comfortably under the cap. If a future free model is stricter we add a semaphore in `grader/llm_judge/grader.py`.
3. **Async-in-sync gRPC handler.** `asyncio.run` inside a thread per RPC is a known pattern. Each call gets its own event loop; no global loop conflict. Worst case if it misbehaves: revert to a sequential loop (still per-dim, just no parallelism).
4. **Anchoring may move into the *user prompt* instead.** All 5 calls see the same user prompt (task + code + transcript). The model could anchor on its prior beliefs about that code regardless of which dim it's asked. Mitigation: the per-dim system prompts are forceful ("Ignore correctness — focus only on...") which empirically helps. If anchoring persists, future spec can try dim-specific prompt redaction (e.g., for maintainability, strip test results from the context).
5. **Rationale length pressure.** With 5x calls, total prompt budget rises. Each call carries a ~3-7KB user prompt (capped via the existing file/transcript truncation). 5 calls in parallel = same per-call budget; total tokens billed roughly 5x. No spec change needed; just budget awareness.

## 7. Rollout

Single PR on the existing `feature/llm-judge` branch, ordered:

1. **`grader/llm_judge/prompts.py`** — replace SYSTEM_PROMPT with DIMENSION_RUBRICS. Keep render_user_prompt.
2. **`grader/llm_client.py`** — add `async_client=False` kwarg to `build_client`.
3. **`grader/llm_judge/grader.py`** — rewrite to async fan-out.
4. **`grader/llm_judge/tests/test_judge.py`** — update mocks + assertions.
5. **`frontend/src/components/grading-inspector/LLMJudgeCard.tsx`** — per-dim rationale rendering.
6. **`frontend/src/pages/runs/grading.test.tsx`** — update mock to tagged-response shape.

Verify with the user's actual setup (qwen/qwen3-coder:free via OpenRouter) before declaring done.

Per project convention this work joins the existing `feature/llm-judge` branch — same PR cycle when the user opens it.
