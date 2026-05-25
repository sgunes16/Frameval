"""Concurrent test that detects event-loop blocking.

Two assertions:

1. RATE CAP: 30 concurrent requests must take at least (n-1)/10 * 0.7 seconds
   (i.e. the rate limit is actually enforced, not bypassed entirely).

2. EVENT-LOOP LIVENESS: a canary coroutine runs alongside the load. It
   wakes every 50 ms via asyncio.sleep(0.05). If the implementation uses
   time.sleep (blocking), the event loop is monopolized during each 100 ms
   window and the canary gets zero scheduled turns. If asyncio.sleep is
   used correctly, the canary wakes at least a few times per second.

   A time.sleep-based "fix" passes the rate-cap check (it accidentally
   serialises correctly in a single-threaded loop), but the canary
   receives 0 pings → assertion fails.
"""
from __future__ import annotations

import asyncio
import time

import pytest


@pytest.mark.asyncio
@pytest.mark.timeout(20)
async def test_throughput_holds_under_concurrent_load(client):
    n_requests = 30
    canary_pings: list[float] = []

    async def canary() -> None:
        """Lightweight heartbeat — only fires if the loop is not blocked."""
        while True:
            t0 = time.monotonic()
            await asyncio.sleep(0.05)
            canary_pings.append(time.monotonic() - t0)

    canary_task = asyncio.create_task(canary())
    start = time.monotonic()
    try:
        responses = await asyncio.gather(
            *(client.get(f"/search?q=q{i}") for i in range(n_requests))
        )
    finally:
        canary_task.cancel()
    elapsed = time.monotonic() - start

    # All complete.
    assert all(r.status_code == 200 for r in responses), \
        f"Some requests failed: {[r.status_code for r in responses]}"

    # Rate cap: must take at least ~ (n_requests-1)/10 seconds (30% slack).
    expected_min_elapsed = (n_requests - 1) / 10 * 0.7  # ~2.03 s
    assert elapsed >= expected_min_elapsed, \
        (f"finished in {elapsed:.2f}s; rate limit not enforced "
         f"(expected >= {expected_min_elapsed:.2f}s)")

    # Event-loop liveness: canary must have fired at least once per second
    # on average. With time.sleep the canary gets 0 pings.
    min_expected_pings = int(elapsed / 0.05 * 0.3)  # 30% of theoretical max
    assert len(canary_pings) >= max(min_expected_pings, 3), \
        (f"event loop appears blocked: canary fired only {len(canary_pings)} "
         f"times in {elapsed:.2f}s (expected >= {max(min_expected_pings, 3)}). "
         f"Did you use time.sleep instead of asyncio.sleep?")
