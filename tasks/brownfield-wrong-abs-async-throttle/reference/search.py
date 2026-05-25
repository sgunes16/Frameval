from __future__ import annotations

import asyncio
import time

_RATE = 10  # requests per second
_INTERVAL = 1.0 / _RATE
_lock = asyncio.Lock()
_last_emit = 0.0


async def search(q: str) -> dict:
    """Rate-limited search. Token-bucket on a single async lock."""
    global _last_emit
    async with _lock:
        now = time.monotonic()
        wait_for = _last_emit + _INTERVAL - now
        if wait_for > 0:
            await asyncio.sleep(wait_for)
        _last_emit = time.monotonic()
    return {"query": q, "hits": [f"result-{i}-for-{q}" for i in range(3)]}
