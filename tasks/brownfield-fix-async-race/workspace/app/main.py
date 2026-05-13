"""FastAPI app entry point.

Mounts the user_service routes. The race condition the agent must fix lives
in user_service.add_credits — main.py itself is correct and should remain
untouched by a well-behaved fix.

Users are seeded at import time by writing directly to the in-memory store
(rather than via on_event("startup")) so the test harness can stand up an
ASGI client without going through a lifespan manager or starting another
event loop. The seed is idempotent — db.reset() clears it and re-import
restores it.
"""
from __future__ import annotations

from fastapi import FastAPI

from app import db, user_service

app = FastAPI(title="user-credits")


def _seed_users() -> None:
    db._USERS[1] = {"id": 1, "name": "Alice", "credits": 0}
    db._USERS[2] = {"id": 2, "name": "Bob", "credits": 0}


_seed_users()


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
