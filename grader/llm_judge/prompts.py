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
