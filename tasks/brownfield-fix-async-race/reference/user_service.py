"""Reference solution for brownfield-fix-async-race.

The canonical fix is an asyncio.Lock that serializes the read-modify-write
window. Per-user locks would be more concurrent but a single global lock
suffices for the test's single-user workload and matches the task's
"asyncio-friendly lock" hint.

Compared to the buggy version: same signature, same return type, same
exception behavior. Only diff: wrap the RMW block in `async with _lock:`.
"""
from __future__ import annotations

import asyncio

from app import db

_lock = asyncio.Lock()


async def add_credits(user_id: int, amount: int) -> int:
    async with _lock:
        user = await db.get_user(user_id)
        if user is None:
            return 0
        current = user["credits"]
        await asyncio.sleep(0)
        new_total = current + amount
        await db.update_credits(user_id, new_total)
        return new_total
