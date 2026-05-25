from __future__ import annotations

import os
from dataclasses import dataclass
from typing import Any


@dataclass(slots=True)
class LLMClientConfig:
    provider: str
    base_url: str | None
    api_key: str | None
    model: str


# (base_url, env_var_for_key, default_model). Keep in sync with the
# validJudgeProviders set in engine/internal/api/config_handler.go.
_PRESETS: dict[str, tuple[str | None, str | None, str]] = {
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

    `override` is duck-typed (any object with .provider / .model / .api_key
    string attrs -- typically a JudgeConfig proto). Falsy / empty fields
    fall through to env vars, then to the preset default for the provider.
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


async def _noop_aclose() -> None:
    """No-op coroutine returned by build_client for sync clients (which
    don't need explicit teardown). Lets async callers always
    `await aclose()` without branching on the client kind."""


def build_client(cfg: LLMClientConfig, *, async_client: bool = False):
    """Return an instructor-wrapped chat completions surface.

    Back-compat wrapper around build_client_with_cleanup that discards
    the close handle. Sync callers (failure_classifier) keep their
    existing single-return contract. Async callers should prefer
    build_client_with_cleanup so they can release the underlying httpx
    AsyncClient when the per-call event loop tears down.
    """
    surface, _ = build_client_with_cleanup(cfg, async_client=async_client)
    return surface


def build_client_with_cleanup(cfg: LLMClientConfig, *, async_client: bool = False):
    """Return (surface, aclose).

    `surface` is the instructor-wrapped `.create(...)` target. `aclose`
    is an awaitable that releases the underlying httpx AsyncClient pool
    when the caller is done — REQUIRED for async clients to avoid
    accumulating dangling sockets across per-call asyncio.run loops.
    Sync clients get _noop_aclose for API uniformity.
    """
    if cfg.provider == "anthropic":
        import instructor
        if not cfg.api_key:
            raise RuntimeError("anthropic provider needs api_key")
        if async_client:
            from anthropic import AsyncAnthropic
            raw = AsyncAnthropic(api_key=cfg.api_key)
            return instructor.from_anthropic(raw).messages, raw.close
        from anthropic import Anthropic
        raw = Anthropic(api_key=cfg.api_key)
        return instructor.from_anthropic(raw).messages, _noop_aclose

    import instructor
    client_kwargs: dict[str, object] = {}
    if cfg.base_url:
        client_kwargs["base_url"] = cfg.base_url
    # Ollama accepts any non-empty key; OpenRouter / Z.ai / OpenAI require real keys.
    client_kwargs["api_key"] = cfg.api_key or "not-needed"
    if async_client:
        from openai import AsyncOpenAI
        raw = AsyncOpenAI(**client_kwargs)
        return instructor.from_openai(raw).chat.completions, raw.close
    from openai import OpenAI
    raw = OpenAI(**client_kwargs)
    return instructor.from_openai(raw).chat.completions, _noop_aclose
