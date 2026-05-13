"""API contract test.

The agent must NOT change response schemas or status codes. This test
exercises GET /users/{id} and POST /users/{id}/credits to verify both
contracts are preserved.
"""
from __future__ import annotations

import httpx
import pytest


@pytest.fixture
def fresh_app():
    import importlib

    from app import db as db_mod

    db_mod.reset()
    main_mod = importlib.import_module("app.main")
    importlib.reload(main_mod)
    return main_mod.app


@pytest.mark.asyncio
async def test_get_user_schema_unchanged(fresh_app):
    transport = httpx.ASGITransport(app=fresh_app)
    async with httpx.AsyncClient(transport=transport, base_url="http://test") as client:
        r = await client.get("/users/1")
        assert r.status_code == 200, r.text
        body = r.json()
        assert set(body.keys()) == {"id", "name", "credits"}, f"unexpected keys: {sorted(body.keys())}"
        assert isinstance(body["id"], int)
        assert isinstance(body["name"], str)
        assert isinstance(body["credits"], int)


@pytest.mark.asyncio
async def test_post_credits_schema_unchanged(fresh_app):
    transport = httpx.ASGITransport(app=fresh_app)
    async with httpx.AsyncClient(transport=transport, base_url="http://test") as client:
        await client.get("/users/1")  # ensure startup ran
        r = await client.post("/users/1/credits", json={"amount": 5})
        assert r.status_code == 200, r.text
        body = r.json()
        assert set(body.keys()) == {"id", "credits"}, f"unexpected keys: {sorted(body.keys())}"
        assert isinstance(body["id"], int)
        assert isinstance(body["credits"], int)
