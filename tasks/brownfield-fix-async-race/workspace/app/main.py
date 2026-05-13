"""FastAPI app entry point.

Mounts the user_service routes. The race condition the agent must fix lives
in user_service.add_credits — main.py itself is correct and should remain
untouched by a well-behaved fix.

Users are seeded at import time (rather than via on_event("startup")) so the
test harness can stand up an ASGI client without going through a lifespan
manager. The seed is idempotent — db.reset() in tests clears it and the
seed re-runs on module reload.
"""
from __future__ import annotations

import asyncio

from fastapi import FastAPI

from app import db, user_service

app = FastAPI(title="user-credits")


def _seed_users_sync() -> None:
    # Seed via the async db API by running it on a fresh loop. Called at
    # import time so tests using importlib.reload see a populated store.
    asyncio.get_event_loop_policy().get_event_loop() if False else None
    asyncio.run(_async_seed())


async def _async_seed() -> None:
    await db.set_user(1, "Alice", 0)
    await db.set_user(2, "Bob", 0)


_seed_users_sync()


@app.get("/users/{user_id}")
async def get_user(user_id: int) -> dict:
    user = await db.get_user(user_id)
    if user is None:
        return {"error": "not found"}
    return {"id": user["id"], "name": user["name"], "credits": user["credits"]}


@app.post("/users/{user_id}/credits")
async def add_credits_endpoint(user_id: int, payload: dict) -> dict:
    amount = int(payload.get("amount", 0))
    new_total = await user_service.add_credits(user_id, amount)
    return {"id": user_id, "credits": new_total}
