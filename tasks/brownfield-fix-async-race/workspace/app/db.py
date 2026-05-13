"""In-memory user store.

Intentionally unprotected — the agent's job is to add locking in
user_service.add_credits, not to refactor this module. db.get/set are
async to play nicely with FastAPI's event loop but they hold no locks.
"""
from __future__ import annotations

from typing import Any

_USERS: dict[int, dict[str, Any]] = {}


async def get_user(user_id: int) -> dict[str, Any] | None:
    return _USERS.get(user_id)


async def set_user(user_id: int, name: str, credits: int) -> None:
    _USERS[user_id] = {"id": user_id, "name": name, "credits": credits}


async def update_credits(user_id: int, credits: int) -> None:
    if user_id in _USERS:
        _USERS[user_id]["credits"] = credits


def reset() -> None:
    """Test helper — clear the store between cases."""
    _USERS.clear()
