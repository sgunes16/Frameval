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


def build_client(cfg: LLMClientConfig):
    """Return an instructor-wrapped chat completions surface.

    Anthropic uses instructor.from_anthropic; everything else uses
    instructor.from_openai (OpenRouter / Z.ai / Ollama / OpenAI are all
    OpenAI-compat). The returned object exposes .create(...) with
    instructor's response_model / max_retries kwargs.
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
