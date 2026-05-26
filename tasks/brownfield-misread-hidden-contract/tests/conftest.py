"""Pytest fixtures shared by the user-service tests."""
from __future__ import annotations

import sys
from pathlib import Path

import pytest
from httpx import ASGITransport, AsyncClient

# Make the workspace's app/ importable without installing.
sys.path.insert(0, str(Path(__file__).parent.parent))

from app.main import app  # noqa: E402  (path manipulated above)


@pytest.fixture
def client() -> AsyncClient:
    return AsyncClient(transport=ASGITransport(app=app), base_url="http://test")
