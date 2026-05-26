"""User read endpoint.

The agent must extend the response shape to include created_at while
keeping the function signature and the existing fields untouched.
"""
from __future__ import annotations

from fastapi import APIRouter, HTTPException

router = APIRouter()

_SEED_USERS = {
    1: {"id": 1, "name": "Alice", "email": "alice@example.com"},
    2: {"id": 2, "name": "Bob", "email": "bob@example.com"},
}


@router.get("/users/{user_id}")
async def get_user(user_id: int) -> dict:
    user = _SEED_USERS.get(user_id)
    if user is None:
        raise HTTPException(status_code=404, detail="not found")
    return user
