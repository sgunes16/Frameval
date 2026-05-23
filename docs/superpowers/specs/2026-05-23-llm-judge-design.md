# LLM Judge — Design

**Status:** Draft
**Date:** 2026-05-23
**Owner:** sgunes16
**Related:** [[2026-05-12-agentdx-design]], `grader/llm_judge/grader.py`, `grader/failure_classifier/grader.py`, `grader/composite.py`, `grader/server.py`, `proto/grader.proto`

## 1. Motivation

The `JudgeGradeResult` boundary in `proto/grader.proto:109` and the `grades.judge_*` columns in `engine/internal/storage/migrations/001_initial_schema.sql:76-112` were laid down assuming an LLM-as-Judge stage that grades each agent run on five dimensions (correctness, maintainability, completeness, best_practices, error_handling). The SQLite schema, the gRPC contract, the Go orchestrator wiring, and the frontend `Grade` type (`frontend/src/lib/types.ts:191`) all already reserve space for these scores.

The current implementation in `grader/llm_judge/grader.py:6-22` is **not an LLM judge** — it is a deterministic heuristic that re-derives the same five fields from `code_grade` and `process_grade` outputs and stamps `irr_alpha = 0.72` as a hardcoded constant. When `FRAMEVAL_ENABLE_LLM_JUDGE=true` is set, the pipeline flips on and produces these heuristic values, *labeling them as judge scores*. This is worse than the disabled state: it doubles-counts existing signals while presenting them as an independent judgment, and it pollutes the composite score weight rebalance in `composite.py:27` with non-information.

Separately, the only real LLM stage in the grader today — the failure classifier at `grader/failure_classifier/grader.py:84-93` — is hard-bound to `anthropic.Anthropic` and `ANTHROPIC_API_KEY`. For thesis demos we want a setup that runs without any Anthropic billing exposure, ideally with a free model provider.

User pain (verbatim): *"anthropic key kullanmak istemiyorum tek model olsun çapraz işi sıkıntı ya"* — single judge model, no cross-model IRR, no Anthropic dependency.

## 2. Goals & non-goals

**Goals.**
- Replace the heuristic placeholder in `llm_judge/grader.py` with a real LLM call.
- Switch both the LLM judge and the failure classifier to an **OpenAI-compatible** client surface, so the same code path works against any OpenAI-compatible endpoint (OpenRouter, Z.ai, Ollama, vLLM, etc.).
- Default provider: **OpenRouter** with a free model (e.g., `deepseek/deepseek-chat-v3-0324:free` or equivalent currently-free model).
- Keep structured-output guarantees via `instructor` (the failure classifier already uses this pattern; the judge gains it).
- Make ANTHROPIC_API_KEY truly optional — present only as a fallback provider, not the default.
- Preserve all existing gRPC and storage contracts. No proto change, no migration.

**Non-goals.**
- Cross-model judging or inter-rater reliability. Schema field `judge_irr_alpha` is retained for forward compatibility but always written as `0.0` (single-model runs have undefined IRR). A future spec can re-introduce cross-model.
- Per-task customizable rubrics. The five fixed dimensions stay.
- Adding a new sandbox layer around the grader. The grader's existing container isolation is sufficient; per-judgment ephemeral sandboxes are out of scope.
- Refactoring `composite.py` weighting beyond the values already documented (0.3 code / 0.3 judge / 0.2 process / 0.2 spec when judge enabled).
- Frontend changes. The judge fields already render in `Grade`-consuming components.

## 3. Approach

Two parallel substitutions, one shared client factory:

1. **Shared OpenAI-compatible client factory** (`grader/llm_client.py`, new module) — produces an `instructor`-wrapped `openai.OpenAI` client configured from environment variables. One construction site, one set of provider knobs.
2. **LLM judge** (`grader/llm_judge/grader.py`, rewrite) — calls the shared client with a judge prompt, returns a Pydantic `JudgeResult` (five floats + raw response). Replaces the heuristic.
3. **Failure classifier** (`grader/failure_classifier/grader.py`, refactor) — swap `instructor.from_anthropic(Anthropic(...))` for the shared client factory. The `FailureClassifier` class and `classify()` entry point keep their signatures so the gRPC handler and tests do not change.

The composite scoring formula in `composite.py:27` is **already** correct for an enabled judge — no change needed. The hardcoded `irr_alpha = 0.72` in the old heuristic is dropped; new code writes `0.0` (single-model, no IRR).

## 4. Targeted changes

### 4.1 New shared client factory — `grader/llm_client.py`

A single module owns provider configuration and `instructor` wrapping. Both the judge and the failure classifier consume it.

```python
# grader/llm_client.py
from __future__ import annotations

import os
from dataclasses import dataclass


@dataclass(slots=True)
class LLMClientConfig:
    provider: str         # "openrouter" | "zai" | "ollama" | "openai" | "anthropic"
    base_url: str | None  # None means SDK default
    api_key: str | None   # None for Ollama / local
    model: str            # provider-namespaced model id


def load_config() -> LLMClientConfig:
    provider = os.getenv("FRAMEVAL_LLM_PROVIDER", "openrouter").lower()
    presets = {
        "openrouter": ("https://openrouter.ai/api/v1", "OPENROUTER_API_KEY",
                       "deepseek/deepseek-chat-v3-0324:free"),
        "zai":        ("https://api.z.ai/api/coding/paas/v4", "ZAI_API_KEY",
                       "glm-4.6"),
        "ollama":     ("http://localhost:11434/v1", None,
                       "qwen2.5-coder:32b"),
        "openai":     (None, "OPENAI_API_KEY", "gpt-4o-mini"),
        "anthropic":  (None, "ANTHROPIC_API_KEY", "claude-haiku-4-5-20251001"),
    }
    if provider not in presets:
        raise ValueError(f"unknown FRAMEVAL_LLM_PROVIDER={provider}; valid: {list(presets)}")
    base_url, key_env, default_model = presets[provider]
    return LLMClientConfig(
        provider=provider,
        base_url=os.getenv("FRAMEVAL_LLM_BASE_URL", base_url),
        api_key=os.getenv(key_env) if key_env else None,
        model=os.getenv("FRAMEVAL_LLM_MODEL", default_model),
    )


def build_client(cfg: LLMClientConfig):
    """Returns an instructor-wrapped chat completions surface.

    Both OpenAI and Anthropic providers go through `instructor.from_openai`
    or `instructor.from_anthropic`. All non-Anthropic providers use the
    OpenAI SDK with a custom base_url — this is the OpenAI-compat contract.
    """
    if cfg.provider == "anthropic":
        import instructor
        from anthropic import Anthropic
        if not cfg.api_key:
            raise RuntimeError("anthropic provider needs ANTHROPIC_API_KEY")
        return instructor.from_anthropic(Anthropic(api_key=cfg.api_key)).messages

    import instructor
    from openai import OpenAI
    client_kwargs: dict[str, object] = {}
    if cfg.base_url:
        client_kwargs["base_url"] = cfg.base_url
    # Ollama accepts any non-empty key; OpenRouter / Z.ai / OpenAI require real keys.
    client_kwargs["api_key"] = cfg.api_key or "not-needed"
    return instructor.from_openai(OpenAI(**client_kwargs)).chat.completions
```

**Why one module:** today each grader stage would otherwise re-implement provider switching, retry caps, and key-env lookups. Centralizing means swapping providers is a one-env-var change everywhere.

**Why dual `instructor` paths:** the OpenAI and Anthropic SDKs expose different completion surfaces (`.chat.completions.create` vs `.messages.create`). `instructor` already abstracts the validation/retry layer over both. Both branches return a `.create(...)` callable that takes the same kwargs the caller already uses.

### 4.2 LLM judge — `grader/llm_judge/grader.py` (rewrite)

Delete the 22-line heuristic. Replace with:

```python
# grader/llm_judge/grader.py
from __future__ import annotations

import json
import logging
from typing import Any

from pydantic import BaseModel, Field

from grader.llm_client import build_client, load_config
from grader.llm_judge.prompts import SYSTEM_PROMPT, render_user_prompt

logger = logging.getLogger(__name__)


class JudgeResult(BaseModel):
    correctness: float = Field(ge=0.0, le=10.0)
    maintainability: float = Field(ge=0.0, le=10.0)
    completeness: float = Field(ge=0.0, le=10.0)
    best_practices: float = Field(ge=0.0, le=10.0)
    error_handling: float = Field(ge=0.0, le=10.0)
    rationale: str = Field(max_length=600)


def grade(code_grade: dict[str, Any], process_grade: dict[str, Any],
          task: dict[str, Any] | None = None,
          output_files: list[dict[str, Any]] | None = None,
          transcript_json: bytes | None = None) -> dict[str, Any]:
    """Score one run on five dimensions via a single LLM call.

    Returns the dict shape that grader_pb2.JudgeGradeResult expects.
    On hard failure (network, validation, no key), returns the disabled
    sentinel so the orchestrator never crashes — same fallback contract
    as the failure classifier.
    """
    try:
        cfg = load_config()
        client = build_client(cfg)
    except Exception as exc:
        logger.warning("judge client init failed: %s", exc)
        return _failed_judge_result(str(exc))

    prompt = render_user_prompt(
        code_grade=code_grade,
        process_grade=process_grade,
        task=task or {},
        output_files=output_files or [],
        transcript_json=transcript_json or b"",
    )
    try:
        verdict: JudgeResult = client.create(
            model=cfg.model,
            response_model=JudgeResult,
            max_retries=2,
            max_tokens=1024,
            messages=[
                {"role": "system", "content": SYSTEM_PROMPT},
                {"role": "user", "content": prompt},
            ],
        )
    except Exception as exc:
        logger.warning("judge call failed: %s", exc)
        return _failed_judge_result(str(exc))

    return {
        "correctness": verdict.correctness,
        "maintainability": verdict.maintainability,
        "completeness": verdict.completeness,
        "best_practices": verdict.best_practices,
        "error_handling": verdict.error_handling,
        "irr_alpha": 0.0,  # single-model run — no IRR by definition
        "raw_responses": [verdict.model_dump_json()],
    }


def _failed_judge_result(reason: str) -> dict[str, Any]:
    return {
        "correctness": 0.0, "maintainability": 0.0, "completeness": 0.0,
        "best_practices": 0.0, "error_handling": 0.0,
        "irr_alpha": 0.0,
        "raw_responses": [f"judge_unavailable: {reason[:300]}"],
    }
```

A new `grader/llm_judge/prompts.py` holds the system prompt and the user-prompt renderer. The system prompt is a rubric for the five dimensions; the user prompt embeds the task description, the code grade summary, a transcript tail, and the modified files. Prompt content is the focus of an iteration cycle once the wiring lands — initial draft uses the same compact-context pattern as `failure_classifier/prompts.py`.

**Important:** the function signature widens — the heuristic only saw `code_grade` and `process_grade`, but a real LLM judge needs the task, the output files, and a transcript tail. Update the call site at `grader/server.py:38` accordingly. This is internal to the grader process; no proto change.

### 4.3 Failure classifier — `grader/failure_classifier/grader.py` (refactor)

The shape of `FailureClassifier` stays. The only change is in `_client_lazy()`:

**Today:**
```python
def _client_lazy(self) -> _ClassifierClient:
    if self._client is not None:
        return self._client
    import instructor
    from anthropic import Anthropic
    api_key = os.getenv("ANTHROPIC_API_KEY")
    if not api_key:
        raise RuntimeError("ANTHROPIC_API_KEY is not set ...")
    self._client = instructor.from_anthropic(Anthropic(api_key=api_key)).messages
    return self._client
```

**After:**
```python
def _client_lazy(self) -> _ClassifierClient:
    if self._client is not None:
        return self._client
    from grader.llm_client import build_client, load_config
    cfg = load_config()
    # FailureClassifier may have been constructed with model=<override>; keep it.
    self._client = build_client(cfg)
    return self._client
```

The `DEFAULT_MODEL = "claude-haiku-4-5-20251001"` constant at line 26 is removed. The classifier now follows whatever `FRAMEVAL_LLM_MODEL` (or the provider default) resolves to. Calibration ablation runs that pass `classifier_model` override via `ClassifyFailure` RPC (`server.py:103`) still work because the override is applied at the per-call boundary, not at client construction.

**The system prompt at `failure_classifier/prompts.py:SYSTEM_PROMPT`** was written against Haiku's tone and brevity. Swapping models will surface prompt-fragility. Plan: keep the prompt as-is in this PR (priority is the wiring), then iterate prompts after calibration in a follow-up.

### 4.4 Composite scoring — no change

`grader/composite.py:27` already implements the correct 0.3/0.3/0.2/0.2 split when judge is enabled. The only adjacent concern: with judge now producing *real* scores (not heuristic re-derivations of code+process), composite ranges shift — likely toward lower averages because the judge is stricter than the heuristic. This is desirable signal, not a bug, but means existing baseline scores will not be directly comparable to post-merge scores. Document this in the changelog.

### 4.5 Server wiring — `grader/server.py`

Two edits at `server.py:38`:

1. Pass `task`, `output_files`, `transcript_json` to `judge_grade()` (function signature widened above).
2. Default `enable_llm_judge` flips from `false` to `true` in `grader/config.py:11` — judge is now production code, not gated. Users who don't want it can set `FRAMEVAL_ENABLE_LLM_JUDGE=false` explicitly.

### 4.6 Dependencies — `grader/pyproject.toml`

- `anthropic>=0.52.0` stays (still supported as a provider, just not default).
- `openai>=1.76.0` stays.
- `instructor>=1.7.0` stays.
- No new deps.

### 4.7 Environment variables

| Variable | Default | Purpose |
|---|---|---|
| `FRAMEVAL_LLM_PROVIDER` | `openrouter` | `openrouter` / `zai` / `ollama` / `openai` / `anthropic` |
| `FRAMEVAL_LLM_BASE_URL` | provider preset | Override endpoint (e.g., for self-hosted Ollama) |
| `FRAMEVAL_LLM_MODEL` | provider preset | Override default model id |
| `OPENROUTER_API_KEY` | — | Key for OpenRouter (default provider) |
| `ZAI_API_KEY` | — | Key for Z.ai |
| `OPENAI_API_KEY` | — | Key for OpenAI |
| `ANTHROPIC_API_KEY` | — | Key for Anthropic fallback (no longer required) |
| `FRAMEVAL_ENABLE_LLM_JUDGE` | `true` (was `false`) | Master kill-switch; on by default once judge is real |

`CLAUDE.md` env table at the project root is updated to reflect these.

## 5. Testing

The existing test surface is well-shaped for this change:

- **`grader/failure_classifier/tests/test_classifier.py`** already injects a fake `_ClassifierClient`. After the refactor the same fixture works because `build_client(...)` is bypassed when a `client=` kwarg is passed to `FailureClassifier(...)`. No test change needed for this file.
- **`grader/tests/integration/test_grpc_server.py`** drives `GradeRun` through the real gRPC handler. Update fixture to set `FRAMEVAL_LLM_PROVIDER=ollama` and stub the OpenAI HTTP layer with `respx` (already in `pyproject.toml`) so the test never hits the network.
- **New: `grader/llm_judge/tests/test_judge.py`** — three cases:
  1. Happy path with a mocked client returning a valid `JudgeResult`.
  2. Hard failure (client raises) returns `_failed_judge_result` with sentinel `raw_responses`.
  3. Pydantic validation failure on out-of-range score retries then returns sentinel.
- **New: `grader/tests/test_llm_client.py`** — config loader tests for each provider preset + bad provider name raises.

End-to-end: a manual smoke test from `engine/cmd/server` against a live OpenRouter key, one experiment with two variants, three runs each. Verify `grades.judge_correctness > 0` in SQLite and the score appears in the frontend Compare view.

## 6. Risks

1. **Free OpenRouter models rate-limit aggressively.** Free tiers commonly cap at ~20 req/min. With `FRAMEVAL_MAX_CONCURRENT=3` and per-run grading, a 5-variant × 5-run experiment is 25 judge calls — well above per-minute caps. Mitigation: judge calls have a built-in retry-with-backoff layer (instructor handles transient HTTP 429 via its retry param); if this is insufficient, add a simple sliding-window throttle in `llm_client.py`.
2. **Free model quality is uneven.** DeepSeek V3 free is strong for code reasoning but smaller free models (Llama 3.1 8B Instruct Free) will produce noisier scores. Mitigation: document the recommended free model in `README.md` and pin it in `load_config()`. Allow override via `FRAMEVAL_LLM_MODEL`.
3. **Prompt sensitivity in failure classifier.** The current prompt was tuned for Haiku. Swapping to a generic OSS model may drop classification accuracy. Mitigation: scoped to a follow-up calibration run (Story #28 already on the board). This spec accepts the risk for the demo path.
4. **`raw_responses` field is now a JSON-dumped Pydantic model** rather than the heuristic placeholder string. Downstream consumers (any frontend code that displays `raw_judge_responses_json`) need to handle this. Quick grep shows the field is only persisted, not currently rendered — no UI change needed today.
5. **Composite score backwards-compat.** Existing baseline grades in the DB were produced with the heuristic. New runs will score lower on average. Comparing pre/post grades in Compare V2 will mislead. Mitigation: add a `grader_version` field consideration to the changelog; for the demo, recommend re-running baselines.

## 7. Rollout

Single PR. Order of changes inside the PR:

1. New `grader/llm_client.py` + tests.
2. Refactor `failure_classifier/grader.py` to use shared client. CI run — failure classifier integration test must still pass with a stubbed OpenAI client.
3. Rewrite `llm_judge/grader.py` + new `llm_judge/prompts.py` + tests.
4. Update `server.py` call site + `config.py` default flip.
5. Update `pyproject.toml` (no new deps but version bumps if needed).
6. Update `CLAUDE.md` env-var table + `README.md` provider docs.

Verify locally with `OPENROUTER_API_KEY` and a fresh OpenRouter free account before opening PR. Per project convention (`feedback_github_workflow`), the PR goes through `feature-dev:code-reviewer` before merge.
