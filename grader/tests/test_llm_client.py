from __future__ import annotations

from types import SimpleNamespace
from unittest.mock import patch, MagicMock

import pytest

from grader.llm_client import LLMClientConfig, build_client_with_cleanup, load_config


def test_load_config_default_is_openrouter(monkeypatch):
    monkeypatch.delenv("FRAMEVAL_LLM_PROVIDER", raising=False)
    monkeypatch.delenv("FRAMEVAL_LLM_MODEL", raising=False)
    monkeypatch.delenv("OPENROUTER_API_KEY", raising=False)
    monkeypatch.delenv("FRAMEVAL_LLM_BASE_URL", raising=False)
    cfg = load_config()
    assert cfg.provider == "openrouter"
    assert cfg.base_url == "https://openrouter.ai/api/v1"
    assert cfg.model == "deepseek/deepseek-chat-v3-0324:free"


def test_load_config_env_overrides_default(monkeypatch):
    monkeypatch.setenv("FRAMEVAL_LLM_PROVIDER", "ollama")
    monkeypatch.setenv("FRAMEVAL_LLM_MODEL", "qwen2.5-coder:7b")
    monkeypatch.delenv("FRAMEVAL_LLM_BASE_URL", raising=False)
    cfg = load_config()
    assert cfg.provider == "ollama"
    assert cfg.base_url == "http://localhost:11434/v1"
    assert cfg.model == "qwen2.5-coder:7b"
    assert cfg.api_key is None


def test_load_config_override_beats_env(monkeypatch):
    monkeypatch.setenv("FRAMEVAL_LLM_PROVIDER", "ollama")
    monkeypatch.setenv("FRAMEVAL_LLM_MODEL", "ignored")
    monkeypatch.delenv("FRAMEVAL_LLM_BASE_URL", raising=False)
    override = SimpleNamespace(provider="zai", model="glm-4.6", api_key="zk-test")
    cfg = load_config(override)
    assert cfg.provider == "zai"
    assert cfg.model == "glm-4.6"
    assert cfg.api_key == "zk-test"
    assert cfg.base_url == "https://api.z.ai/api/coding/paas/v4"


def test_load_config_empty_override_fields_fall_through(monkeypatch):
    monkeypatch.setenv("FRAMEVAL_LLM_PROVIDER", "openrouter")
    monkeypatch.setenv("OPENROUTER_API_KEY", "env-key")
    monkeypatch.delenv("FRAMEVAL_LLM_BASE_URL", raising=False)
    override = SimpleNamespace(provider="", model="", api_key="")
    cfg = load_config(override)
    assert cfg.provider == "openrouter"
    assert cfg.api_key == "env-key"


def test_load_config_unknown_provider_raises():
    override = SimpleNamespace(provider="totally-not-real", model="x", api_key="")
    with pytest.raises(ValueError, match="unknown provider"):
        load_config(override)


def test_build_client_uses_json_mode_on_openai_compatible_path():
    """OpenRouter routes some paid models to providers that 404 on
    `tool_choice`. We force Mode.JSON so the structured-output contract
    works universally across providers OpenRouter fronts."""
    import instructor as instructor_module

    cfg = LLMClientConfig(
        provider="openrouter",
        model="anthropic/claude-sonnet-4",
        api_key="sk-test",
        base_url="https://openrouter.ai/api/v1",
    )

    # Stub instructor.from_openai to capture the mode kwarg without
    # spinning up a real OpenAI client / network connection.
    fake_chained = MagicMock()
    fake_chained.chat.completions = MagicMock()
    with patch("instructor.from_openai", return_value=fake_chained) as patched_from_openai:
        with patch("openai.AsyncOpenAI", return_value=MagicMock()):
            build_client_with_cleanup(cfg, async_client=True)

    patched_from_openai.assert_called_once()
    call_kwargs = patched_from_openai.call_args.kwargs
    assert call_kwargs.get("mode") == instructor_module.Mode.JSON


def test_build_client_anthropic_native_path_does_not_force_json_mode():
    """The Anthropic native path uses instructor.from_anthropic with its
    own (Anthropic-tool-use) default. Mode.JSON would be wrong there;
    only the openai-compatible OpenRouter/Z.ai/Ollama/OpenAI path needs
    the JSON override."""
    cfg = LLMClientConfig(
        provider="anthropic",
        model="claude-sonnet-4-5",
        api_key="sk-ant-test",
        base_url="",
    )
    with patch("instructor.from_anthropic", return_value=MagicMock()) as patched_from_anthropic:
        with patch("anthropic.AsyncAnthropic", return_value=MagicMock()):
            build_client_with_cleanup(cfg, async_client=True)
    patched_from_anthropic.assert_called_once()
    # Just verifying from_anthropic was called without a mode kwarg.
    assert "mode" not in patched_from_anthropic.call_args.kwargs
