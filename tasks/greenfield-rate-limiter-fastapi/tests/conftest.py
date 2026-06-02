"""Pytest fixtures for the rate-limiter task.

Adds the workspace root to sys.path so `from app import app` resolves to
the app.py the agent wrote there (the tests/ dir is materialized under that
same root at run time). Each test fixture reloads the module so per-test
rate-limit state is fresh.
"""
from __future__ import annotations

import importlib
import sys
from pathlib import Path

import pytest

WORKSPACE = Path(__file__).resolve().parent.parent
if str(WORKSPACE) not in sys.path:
    sys.path.insert(0, str(WORKSPACE))


@pytest.fixture
def fresh_app():
    """Reload the agent's app module so each test gets a clean limiter state."""
    if not (WORKSPACE / "app.py").exists():
        pytest.fail(f"app.py not found at {WORKSPACE / 'app.py'}")
    mod = importlib.import_module("app")
    importlib.reload(mod)
    return mod.app
