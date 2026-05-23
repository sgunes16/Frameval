from __future__ import annotations

import os
from types import SimpleNamespace

import pytest

from grader.llm_client import LLMClientConfig, load_config


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
