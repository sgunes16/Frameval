# LLM Judge — Design

**Status:** Draft
**Date:** 2026-05-23
**Owner:** sgunes16
**Related:** [[2026-05-12-agentdx-design]], `grader/llm_judge/grader.py`, `grader/failure_classifier/grader.py`, `grader/composite.py`, `grader/server.py`, `proto/grader.proto`, `frontend/src/pages/settings/index.tsx`, `engine/internal/api/config_handler.go`, `engine/internal/storage/config_repo.go`

## 1. Motivation

The `JudgeGradeResult` boundary in `proto/grader.proto:109` and the `grades.judge_*` columns in `engine/internal/storage/migrations/001_initial_schema.sql:76-112` were laid down assuming an LLM-as-Judge stage that grades each agent run on five dimensions (correctness, maintainability, completeness, best_practices, error_handling). The SQLite schema, the gRPC contract, the Go orchestrator wiring, and the frontend `Grade` type (`frontend/src/lib/types.ts:191`) all already reserve space for these scores.

The current implementation in `grader/llm_judge/grader.py:6-22` is **not an LLM judge** — it is a deterministic heuristic that re-derives the same five fields from `code_grade` and `process_grade` outputs and stamps `irr_alpha = 0.72` as a hardcoded constant. When `FRAMEVAL_ENABLE_LLM_JUDGE=true` is set, the pipeline flips on and produces these heuristic values, *labeling them as judge scores*. This is worse than the disabled state: it doubles-counts existing signals while presenting them as an independent judgment, and it pollutes the composite score weight rebalance in `composite.py:27` with non-information.

Separately, the only real LLM stage in the grader today — the failure classifier at `grader/failure_classifier/grader.py:84-93` — is hard-bound to `anthropic.Anthropic` and `ANTHROPIC_API_KEY`. For thesis demos we want a setup that runs without any Anthropic billing exposure, ideally with a free model provider.

Finally, all judge configuration today is environment-only — change provider or rotate a key and the engine must restart. The frontend's `SettingsPage` (`frontend/src/pages/settings/index.tsx`) already has an `ApiKeysPanel` rendering redacted keys read-only, the SQLite `api_keys` table already encrypts keys via AES-GCM (`config_repo.go:172-186`), the `/config/api-keys` endpoint accepts upserts (`config_handler.go:31-45`), and `JudgeConfig` is already defined as field 7 of `GradeRunRequest` in `proto/grader.proto:22-30`. The pieces are laid down; nothing is wired end-to-end.

User pain (verbatim): *"anthropic key kullanmak istemiyorum tek model olsun çapraz işi sıkıntı ya"* and *"frontend'de de bu ayarlar yapıalbilsin ayarlar kısmından ve ayrıca openrouter api key koycak yer de olsun veya zai filan"* — single judge model, no cross-model IRR, no Anthropic dependency, and provider + API key editable from the Settings page.

## 2. Goals & non-goals

**Goals.**
- Replace the heuristic placeholder in `llm_judge/grader.py` with a real LLM call.
- Switch both the LLM judge and the failure classifier to an **OpenAI-compatible** client surface, so the same code path works against any OpenAI-compatible endpoint (OpenRouter, Z.ai, Ollama, vLLM, etc.).
- Default provider: **OpenRouter** with a free model (e.g., `deepseek/deepseek-chat-v3-0324:free` or equivalent currently-free model).
- Keep structured-output guarantees via `instructor` (the failure classifier already uses this pattern; the judge gains it).
- Make ANTHROPIC_API_KEY truly optional — present only as a fallback provider, not the default.
- **SQLite is the source of truth for judge configuration**; env vars become fallback defaults for headless / dev mode. The engine reads current settings on each `GradeRun` and passes them via the existing `JudgeConfig` proto field — no proto change, no proto migration.
- **Frontend Settings page** lets the user pick provider, model, and enter API keys without restarting the engine. The existing `ApiKeysPanel` becomes editable; a new `JudgeProviderPanel` exposes provider + model + enable toggle.
- One SQLite migration only: a small `app_settings` key-value table for non-secret settings (active provider, active model, enable flag). API keys continue to use the existing encrypted `api_keys` table — no schema change there.

**Non-goals.**
- Cross-model judging or inter-rater reliability. Schema field `judge_irr_alpha` is retained for forward compatibility but always written as `0.0` (single-model runs have undefined IRR). A future spec can re-introduce cross-model.
- Per-task customizable rubrics. The five fixed dimensions stay. (The `JudgeConfig.rubric` proto field exists but stays empty.)
- Adding a new sandbox layer around the grader. The grader's existing container isolation is sufficient; per-judgment ephemeral sandboxes are out of scope.
- Refactoring `composite.py` weighting beyond the values already documented (0.3 code / 0.3 judge / 0.2 process / 0.2 spec when judge enabled).
- Multi-user / per-tenant settings. The `app_settings` table is a single global row set; this is a local-first single-user tool.
- API key rotation flows, audit logs, or RBAC on the settings endpoints. Local-first scope.
- Reworking the other Settings panels (ModelsPanel, AgentsPanel, DefaultsPanel). Only `ApiKeysPanel` is touched, plus the new `JudgeProviderPanel`.

## 3. Approach

Five layers of change, top-down:

1. **Storage** — new `app_settings` table (key-value) for non-secret judge settings. `api_keys` table reused as-is.
2. **Backend HTTP** — new `GET/PUT /api/config/llm-settings` endpoints in `engine/internal/api/config_handler.go`. Existing `POST /api/config/api-keys` already accepts provider + key; reused unchanged.
3. **Engine grading path** — `grader_client.go` reads `app_settings` + relevant `api_keys` row on each `GradeRun`, populates the existing `JudgeConfig` proto field. No proto change. No reading of env vars on the engine side anymore — engine is now the authority over what to grade with.
4. **Grader** — new shared OpenAI-compatible client factory (`grader/llm_client.py`). Both `llm_judge` and `failure_classifier` consume it. `load_config()` priority: `JudgeConfig` from request > env vars > preset default. Env vars remain a fallback path for headless setups.
5. **Frontend** — `ApiKeysPanel` becomes editable (input + save per provider). New `JudgeProviderPanel` for active-provider / model / enable selection. Both panels use the existing TanStack Query mutation pattern (`useCreateExperiment` is the reference).

The composite scoring formula in `composite.py:27` is **already** correct for an enabled judge — no change needed. The hardcoded `irr_alpha = 0.72` in the old heuristic is dropped; new code writes `0.0` (single-model, no IRR).

## 4. Targeted changes

### 4.1 SQLite — new `app_settings` table

A new migration `engine/internal/storage/migrations/00X_app_settings.sql` (next free number):

```sql
CREATE TABLE app_settings (
  key        TEXT PRIMARY KEY,
  value      TEXT NOT NULL,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

INSERT INTO app_settings (key, value) VALUES
  ('judge.provider', 'openrouter'),
  ('judge.model', 'deepseek/deepseek-chat-v3-0324:free'),
  ('judge.enabled', 'true');
```

A flat key-value table is intentional: future settings (temperature, max_tokens, classifier model override) become rows, not columns. No risk of column-explosion. Three rows seed reasonable defaults at install.

**Settings repository** — new `engine/internal/storage/settings_repo.go`:

```go
type SettingsRepo interface {
    Get(ctx context.Context, key string) (string, error)
    Set(ctx context.Context, key, value string) error
    GetAll(ctx context.Context, prefix string) (map[string]string, error)
}
```

Returns sql.ErrNoRows when a key is missing; the caller decides whether to fall back to env-var defaults or fail loudly.

### 4.2 Backend HTTP — `/api/config/llm-settings`

Two new handlers in `engine/internal/api/config_handler.go`:

```go
// GET /api/config/llm-settings
// → {"provider": "openrouter", "model": "deepseek/...:free", "enabled": true,
//    "api_key_present": true}
//
// PUT /api/config/llm-settings
// body: {"provider": "...", "model": "...", "enabled": bool}
// → 200 with the new state
```

Router additions (`engine/internal/api/router.go:90-97`):

```go
r.Get("/config/llm-settings", service.GetLLMSettings)
r.Put("/config/llm-settings", service.PutLLMSettings)
```

The existing `POST /api/config/api-keys` and `DELETE /api/config/api-keys/{provider}` endpoints are reused unchanged for key entry — the frontend just calls them when the user types a key into the (now editable) `ApiKeysPanel`.

Validation: `provider` must be one of the five known presets (openrouter / zai / ollama / openai / anthropic). `model` is a free-form string (the user may run any model the provider exposes). 400 on invalid provider.

### 4.3 Engine call site — populate `JudgeConfig` per `GradeRun`

`engine/internal/experiment/grader_client.go` currently calls `GradeRun` with `judge_config` empty. Pre-call hook:

```go
func (c *GraderClient) buildJudgeConfig(ctx context.Context) (*graderpb.JudgeConfig, error) {
    settings, err := c.settingsRepo.GetAll(ctx, "judge.")
    if err != nil { return nil, err }
    if settings["judge.enabled"] != "true" {
        return nil, nil // grader-side default kicks in (disabled_judge_result)
    }
    provider := settings["judge.provider"]
    apiKey, _ := c.apiKeysRepo.GetDecrypted(ctx, provider) // empty for ollama
    return &graderpb.JudgeConfig{
        Provider: provider,
        Model:    settings["judge.model"],
        ApiKey:   apiKey,
    }, nil
}
```

Called once per `GradeRun` to construct the request. No caching for now — settings rarely change and the SQL lookup is sub-millisecond.

**Security note:** the API key now flows over the gRPC wire grader↔engine (localhost in default deployment, in-container in compose). This is acceptable for local-first single-user scope but warrants gRPC TLS if deployment ever crosses a network boundary. Documented as a known limitation in §6.

### 4.4 Grader — request-time config priority

`grader/server.py:GradeRun` already receives `request.judge_config` via the proto. Updated `judge_grade()` call:

```python
judge_cfg_proto = request.judge_config if request.HasField("judge_config") else None
judge = judge_grade(
    code_grade=code,
    process_grade=process,
    task=task,
    output_files=output_files,
    transcript_json=request.transcript_json,
    config_override=judge_cfg_proto,  # None falls through to env defaults
)
```

The `failure_classifier` does **not** receive per-request config in this PR — it continues to read from `load_config()` env defaults. The classifier is engine-side-triggered via `ClassifyFailure` RPC which doesn't carry a JudgeConfig field today. Adding it is a one-line proto change but out of scope here; failure classifier follows whatever the env / preset says, and that's fine because in practice the user will set `OPENROUTER_API_KEY` once at startup and forget it.

### 4.5 New shared client factory — `grader/llm_client.py`

A single module owns provider configuration and `instructor` wrapping. Both the judge and the failure classifier consume it.

```python
# grader/llm_client.py
from __future__ import annotations

import os
from dataclasses import dataclass
from typing import Any


@dataclass(slots=True)
class LLMClientConfig:
    provider: str         # "openrouter" | "zai" | "ollama" | "openai" | "anthropic"
    base_url: str | None  # None means SDK default
    api_key: str | None   # None for Ollama / local
    model: str            # provider-namespaced model id


_PRESETS = {
    "openrouter": ("https://openrouter.ai/api/v1", "OPENROUTER_API_KEY",
                   "deepseek/deepseek-chat-v3-0324:free"),
    "zai":        ("https://api.z.ai/api/coding/paas/v4", "ZAI_API_KEY",
                   "glm-4.6"),
    "ollama":     ("http://localhost:11434/v1", None,
                   "qwen2.5-coder:32b"),
    "openai":     (None, "OPENAI_API_KEY", "gpt-4o-mini"),
    "anthropic":  (None, "ANTHROPIC_API_KEY", "claude-haiku-4-5-20251001"),
}


def load_config(override: Any = None) -> LLMClientConfig:
    """Resolve config with priority: per-call override > env vars > preset default.

    `override` is a proto JudgeConfig (or any object with .provider / .model
    / .api_key string attrs). Falsy / empty values fall through to env / preset.
    """
    provider = (
        getattr(override, "provider", "") or
        os.getenv("FRAMEVAL_LLM_PROVIDER", "openrouter")
    ).lower()
    if provider not in _PRESETS:
        raise ValueError(f"unknown provider={provider!r}; valid: {list(_PRESETS)}")
    base_url, key_env, default_model = _PRESETS[provider]

    api_key = (
        getattr(override, "api_key", "") or
        (os.getenv(key_env) if key_env else None)
    )
    model = (
        getattr(override, "model", "") or
        os.getenv("FRAMEVAL_LLM_MODEL", default_model)
    )
    return LLMClientConfig(
        provider=provider,
        base_url=os.getenv("FRAMEVAL_LLM_BASE_URL", base_url),
        api_key=api_key,
        model=model,
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
            raise RuntimeError("anthropic provider needs api_key")
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

**Why one module:** today each grader stage would otherwise re-implement provider switching, retry caps, and key-env lookups. Centralizing means swapping providers is a one-call-or-one-env-var change everywhere.

**Why dual `instructor` paths:** the OpenAI and Anthropic SDKs expose different completion surfaces (`.chat.completions.create` vs `.messages.create`). `instructor` already abstracts the validation/retry layer over both. Both branches return a `.create(...)` callable that takes the same kwargs the caller already uses.

**Why the `override` is duck-typed instead of importing the proto:** keeps `llm_client.py` free of proto imports so it stays trivially unit-testable from a plain dict / `SimpleNamespace`. Tests in §5 use this.

### 4.6 LLM judge — `grader/llm_judge/grader.py` (rewrite)

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
          transcript_json: bytes | None = None,
          config_override: Any = None) -> dict[str, Any]:
    """Score one run on five dimensions via a single LLM call.

    Returns the dict shape that grader_pb2.JudgeGradeResult expects.
    On hard failure (network, validation, no key), returns the disabled
    sentinel so the orchestrator never crashes — same fallback contract
    as the failure classifier.

    `config_override` is the request-time JudgeConfig proto passed by the
    engine; falsy fields fall through to env-var defaults in load_config.
    """
    try:
        cfg = load_config(config_override)
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

### 4.7 Failure classifier — `grader/failure_classifier/grader.py` (refactor)

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

### 4.8 Composite scoring — no change

`grader/composite.py:27` already implements the correct 0.3/0.3/0.2/0.2 split when judge is enabled. The only adjacent concern: with judge now producing *real* scores (not heuristic re-derivations of code+process), composite ranges shift — likely toward lower averages because the judge is stricter than the heuristic. This is desirable signal, not a bug, but means existing baseline scores will not be directly comparable to post-merge scores. Document this in the changelog.

### 4.9 Server wiring — `grader/server.py`

Two edits at `server.py:38`:

1. Pass `task`, `output_files`, `transcript_json`, and `request.judge_config` to `judge_grade()` (function signature widened above).
2. `grader/config.py:11` — `enable_llm_judge` env-var fallback default flips from `false` to `true`. SQLite `app_settings['judge.enabled']` is the new authority; the env var is only consulted when the engine passes no `JudgeConfig` (headless / dev path).

### 4.10 Frontend Settings page

Two changes in `frontend/src/pages/settings/`:

**A. `ApiKeysPanel` becomes editable.** Current implementation displays redacted keys read-only. New behavior:

- Each provider row gets an inline edit toggle: clicking "Edit" reveals an `<Input type="password">` and a "Save" / "Cancel" button pair.
- Save calls `POST /api/config/api-keys` with `{provider, api_key}` — the existing endpoint, no backend change.
- Delete button calls `DELETE /api/config/api-keys/{provider}`.
- New providers (OpenRouter, Z.ai) appear in the panel alongside the existing ones. The provider list is defined client-side (`PROVIDERS` const) and matches the grader's `_PRESETS`.
- TanStack Query mutation hook `useUpsertAPIKey()` follows the `useCreateExperiment` pattern (`frontend/src/lib/hooks.ts:244`), invalidating `['config', 'api-keys']` on success.

**B. New `JudgeProviderPanel`.** Renders below `ApiKeysPanel`. Three controls:

- **Provider** — `<select>` of the five presets, default = current `judge.provider` setting.
- **Model** — text `<Input>` with a placeholder showing the preset default for the selected provider. Editing the provider does not auto-clear the model (user may have intentionally customized it).
- **Enable LLM judge** — toggle button (uses existing `<Button variant="ghost">` patterns). Maps to `judge.enabled`.
- **Save** button calls `PUT /api/config/llm-settings`.
- A small "Status" line below the form reads from `GET /api/config/llm-settings` and shows: `Active: <provider>/<model>` plus a colored badge — `green` if the corresponding API key is present, `red` ("missing API key") if not. The endpoint returns `api_key_present` boolean exactly for this UI.

**Hooks** — added to `frontend/src/lib/hooks.ts`:

```typescript
export function useLLMSettings() {
  return useQuery({
    queryKey: ['config', 'llm-settings'],
    queryFn: () => api.get<LLMSettings>('/config/llm-settings'),
  });
}

export function useSaveLLMSettings() {
  const client = useQueryClient();
  return useMutation({
    mutationFn: (payload: LLMSettings) =>
      api.put<LLMSettings>('/config/llm-settings', payload),
    onSuccess: () => client.invalidateQueries({ queryKey: ['config', 'llm-settings'] }),
  });
}

export function useUpsertAPIKey() {
  const client = useQueryClient();
  return useMutation({
    mutationFn: (payload: {provider: string; api_key: string}) =>
      api.post<void>('/config/api-keys', payload),
    onSuccess: () => client.invalidateQueries({ queryKey: ['config', 'api-keys'] }),
  });
}
```

Types live in `frontend/src/lib/types.ts`:

```typescript
export type LLMSettings = {
  provider: 'openrouter' | 'zai' | 'ollama' | 'openai' | 'anthropic';
  model: string;
  enabled: boolean;
  api_key_present: boolean;
};
```

**No new shadcn primitives needed.** `Card`, `Input`, `Button`, `Badge` already in `frontend/src/components/ui/`.

### 4.11 Dependencies — `grader/pyproject.toml`

- `anthropic>=0.52.0` stays (still supported as a provider, just not default).
- `openai>=1.76.0` stays.
- `instructor>=1.7.0` stays.
- No new deps.

### 4.12 Environment variables

Env vars become **fallback defaults** for headless mode. SQLite is authoritative when the engine populates `JudgeConfig`.

| Variable | Default | Purpose |
|---|---|---|
| `FRAMEVAL_LLM_PROVIDER` | `openrouter` | Fallback only — overridden by SQLite `app_settings['judge.provider']` |
| `FRAMEVAL_LLM_BASE_URL` | provider preset | Override endpoint (e.g., for self-hosted Ollama) |
| `FRAMEVAL_LLM_MODEL` | provider preset | Fallback only — overridden by SQLite `app_settings['judge.model']` |
| `OPENROUTER_API_KEY` | — | Fallback only — overridden by `api_keys` table entry for provider=openrouter |
| `ZAI_API_KEY` | — | Same pattern as OPENROUTER_API_KEY |
| `OPENAI_API_KEY` | — | Same pattern |
| `ANTHROPIC_API_KEY` | — | Same pattern; no longer required at startup |
| `FRAMEVAL_ENABLE_LLM_JUDGE` | `true` (was `false`) | Fallback only — overridden by SQLite `app_settings['judge.enabled']` |

`CLAUDE.md` env table at the project root is updated to reflect these and to call out "SQLite is the source of truth, env vars are fallbacks."

## 5. Testing

**Grader (Python).** The existing test surface is well-shaped:

- `grader/failure_classifier/tests/test_classifier.py` already injects a fake `_ClassifierClient`. After the refactor the same fixture works because `build_client(...)` is bypassed when a `client=` kwarg is passed to `FailureClassifier(...)`. No test change needed.
- `grader/tests/integration/test_grpc_server.py` drives `GradeRun` through the real gRPC handler. Update fixture to set `FRAMEVAL_LLM_PROVIDER=ollama` and stub the OpenAI HTTP layer with `respx` (already in `pyproject.toml`) so the test never hits the network. Add a second case that supplies a `JudgeConfig` in the request and asserts it overrides the env defaults.
- **New: `grader/llm_judge/tests/test_judge.py`** — four cases:
  1. Happy path with a mocked client returning a valid `JudgeResult`.
  2. Hard failure (client raises) returns `_failed_judge_result` with sentinel `raw_responses`.
  3. Pydantic validation failure on out-of-range score retries then returns sentinel.
  4. `config_override` proto with provider+model+key wins over env vars.
- **New: `grader/tests/test_llm_client.py`** — config loader tests:
  - each provider preset resolves to expected base_url / key env / default model;
  - bad provider name raises;
  - override duck-type object beats env vars;
  - empty override fields fall through to env / preset.

**Engine (Go).**
- **New: `engine/internal/storage/settings_repo_test.go`** — Get/Set roundtrip; GetAll prefix filter; ErrNoRows on missing key.
- **New: `engine/internal/api/config_handler_llm_test.go`** — `GetLLMSettings` returns defaults when DB is empty; `PutLLMSettings` rejects unknown provider with 400; `PutLLMSettings` upserts existing row; `api_key_present` reflects `api_keys` table state.
- **New: `engine/internal/experiment/grader_client_judge_config_test.go`** — `buildJudgeConfig` returns nil when `judge.enabled=false`; returns populated proto when enabled; pulls api_key from `api_keys` table for the active provider; tolerates missing api_key (proto field empty, grader-side falls back to env).

**Frontend.**
- The repo has limited frontend test infrastructure today (per `docs/superpowers/specs/2026-05-14-testing-foundation-design.md` — testing is a parallel workstream). For this PR: a single Vitest component test for `JudgeProviderPanel` validating the happy-path render + save click. Skip full integration tests; manual smoke test covers end-to-end.

**End-to-end smoke test.** From a clean `docker compose up`:
1. Open Settings → enter OpenRouter key, select provider=openrouter, model=`deepseek/...`, enable=on, save.
2. Launch an experiment with two variants, three runs each.
3. Verify `grades.judge_correctness > 0` in SQLite, scores render in the Compare view, no engine restart was required.
4. Disable judge from Settings, run another experiment, verify all `judge_*` columns are zeroed and composite uses the 0.6/0.4 fallback.

## 6. Risks

1. **Free OpenRouter models rate-limit aggressively.** Free tiers commonly cap at ~20 req/min. With `FRAMEVAL_MAX_CONCURRENT=3` and per-run grading, a 5-variant × 5-run experiment is 25 judge calls — well above per-minute caps. Mitigation: judge calls have a built-in retry-with-backoff layer (instructor handles transient HTTP 429 via its retry param); if this is insufficient, add a simple sliding-window throttle in `llm_client.py`.
2. **Free model quality is uneven.** DeepSeek V3 free is strong for code reasoning but smaller free models (Llama 3.1 8B Instruct Free) will produce noisier scores. Mitigation: document the recommended free model in `README.md` and pin it in `_PRESETS`. Allow override via Settings page (or env var as fallback).
3. **Prompt sensitivity in failure classifier.** The current prompt was tuned for Haiku. Swapping to a generic OSS model may drop classification accuracy. Mitigation: scoped to a follow-up calibration run (Story #28 already on the board). This spec accepts the risk for the demo path.
4. **`raw_responses` field is now a JSON-dumped Pydantic model** rather than the heuristic placeholder string. Downstream consumers (any frontend code that displays `raw_judge_responses_json`) need to handle this. Quick grep shows the field is only persisted, not currently rendered — no UI change needed today.
5. **Composite score backwards-compat.** Existing baseline grades in the DB were produced with the heuristic. New runs will score lower on average. Comparing pre/post grades in Compare V2 will mislead. Mitigation: add a `grader_version` field consideration to the changelog; for the demo, recommend re-running baselines.
6. **API key leaves the engine over the grader gRPC channel.** In default `docker-compose` deployment grader and engine are on the same Docker network, no real network egress. Acceptable for local-first scope. If anyone ever deploys grader on a separate host, gRPC TLS becomes mandatory. Documented in `CLAUDE.md` "Important Constraints" section.
7. **AES-GCM cipher key is hardcoded to `"frameval-local-dev-key"`** (`config_repo.go:172-186`). This is a pre-existing weakness, not introduced here, but the new Settings page makes it more user-facing — users will assume "saved encrypted" means real protection. Out of scope to fix in this PR but called out so it's not forgotten; a follow-up should accept a `FRAMEVAL_DB_CIPHER_KEY` env var (random per install).
8. **Settings page misuse — wrong key for wrong provider.** A user could enter a Z.ai key but leave provider=openrouter selected; the judge would call OpenRouter with the wrong key and fail. Mitigation: `api_key_present` status badge in `JudgeProviderPanel` reads the key for the *currently selected* provider — turns red if missing, giving immediate feedback.

## 7. Rollout

Single PR, ordered so each step compiles + tests pass on its own:

1. **Storage** — new migration `00X_app_settings.sql`, `settings_repo.go` + tests.
2. **Backend handlers** — `GetLLMSettings` / `PutLLMSettings` in `config_handler.go`, router wiring, handler tests.
3. **Engine grading path** — `buildJudgeConfig` in `grader_client.go`, populate `JudgeConfig` per call, tests.
4. **Grader shared client** — `grader/llm_client.py` + tests.
5. **Failure classifier refactor** — swap `_client_lazy()` body to use shared client. CI run — existing fake-client tests must still pass.
6. **LLM judge rewrite** — `grader/llm_judge/grader.py` + `prompts.py` + tests.
7. **Grader server wiring** — pass request-time `judge_config` through; flip default env fallback.
8. **Frontend hooks + types** — `useLLMSettings`, `useSaveLLMSettings`, `useUpsertAPIKey`; `LLMSettings` type.
9. **Frontend panels** — edit `ApiKeysPanel` to be editable; add `JudgeProviderPanel`; render below in `SettingsPage`.
10. **Docs** — `CLAUDE.md` env table, `README.md` provider docs, brief Settings-page screenshot in `docs/demo/`.

Each step is a focused commit; the PR is the union. Verify locally with a fresh OpenRouter free account before opening PR — run the §5 end-to-end smoke test (Settings → save → experiment → verify scores). Per project convention (`feedback_github_workflow`), the PR goes through `feature-dev:code-reviewer` before merge.
