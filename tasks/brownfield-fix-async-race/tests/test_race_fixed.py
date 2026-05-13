"""Race-condition regression test.

100 concurrent POST /users/1/credits requests with amount=1 must result in
final credits = 100. On the unfixed code, the read-modify-write race causes
lost updates and the final value is less than 100 (with high probability).
"""
from __future__ import annotations

import asyncio

import httpx
import pytest


@pytest.fixture
def fresh_app():
    """Reload the app and reset its in-memory store before each test."""
    import importlib

    from app import db as db_mod

    # Force-reload so module-level lock state (the agent's fix) is fresh.
    db_mod.reset()
    main_mod = importlib.import_module("app.main")
    importlib.reload(main_mod)

    return main_mod.app


@pytest.mark.asyncio
async def test_concurrent_add_credits_no_lost_update(fresh_app):
    transport = httpx.ASGITransport(app=fresh_app)
    async with httpx.AsyncClient(transport=transport, base_url="http://test") as client:
        # Allow the FastAPI startup event to seed users.
        await client.get("/users/1")

        async def add_one():
            r = await client.post("/users/1/credits", json={"amount": 1})
            assert r.status_code == 200, r.text

        await asyncio.gather(*(add_one() for _ in range(100)))

        final = await client.get("/users/1")
        assert final.status_code == 200, final.text
        body = final.json()
        assert body["credits"] == 100, (
            f"expected 100 credits after 100 concurrent +1 calls, got {body['credits']}. "
            f"Race condition was not fixed."
        )
