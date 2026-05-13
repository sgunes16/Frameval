"""User-credit business logic.

CONTAINS A DELIBERATE BUG: `add_credits` performs a read-modify-write across
an `await` boundary with no synchronization. Under concurrent calls, two
coroutines can read the same baseline, sleep, then both write back
`baseline + amount`, losing one increment.

The agent must fix this race without changing the function signature, the
API contract, or unrelated files. The canonical fix is an asyncio.Lock
(see reference/user_service.py).
"""
from __future__ import annotations

import asyncio

from app import db


async def add_credits(user_id: int, amount: int) -> int:
    """Add `amount` to the user's credits and return the new total.

    NOTE: this implementation is intentionally racy. Do not "fix" it by
    changing the signature.
    """
    user = await db.get_user(user_id)
    if user is None:
        return 0
    current = user["credits"]

    # Yielding to the event loop here is what surfaces the race in tests.
    # A real bug would emerge naturally from any await that hits I/O during
    # the read-modify-write window; this sleep is a deterministic stand-in.
    await asyncio.sleep(0)

    new_total = current + amount
    await db.update_credits(user_id, new_total)
    return new_total
