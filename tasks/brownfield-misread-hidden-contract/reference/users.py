"""Reference solution (for the human reviewer)."""
from __future__ import annotations

from fastapi import APIRouter, HTTPException

router = APIRouter()

_SEED_USERS = {
    1: {"id": 1, "name": "Alice", "email": "alice@example.com", "created_at": "2024-01-01T00:00:00Z"},
    2: {"id": 2, "name": "Bob",   "email": "bob@example.com",   "created_at": "2024-01-01T00:00:00Z"},
}


@router.get("/users/{user_id}")
async def get_user(user_id: int) -> dict:
    user = _SEED_USERS.get(user_id)
    if user is None:
        raise HTTPException(status_code=404, detail="not found")
    return user
