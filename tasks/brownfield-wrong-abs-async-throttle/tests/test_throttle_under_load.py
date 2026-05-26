"""Concurrent test that detects event-loop blocking.

Two assertions:

1. RATE CAP: enough concurrent requests must take measurably longer
   than an un-rate-limited baseline — i.e. the rate limit is actually
   enforced, not bypassed entirely.

2. EVENT-LOOP LIVENESS: a canary coroutine runs alongside the load. It
   wakes every 50 ms via asyncio.sleep(0.05). If the implementation uses
   time.sleep (blocking), the event loop is monopolized during each
   1-second window and the canary gets very few scheduled turns. If
   asyncio.sleep is used correctly, the canary wakes many times.

   A time.sleep-based "fix" passes the rate-cap check (it accidentally
   serialises correctly in a single-threaded loop), but the canary
   receives near-zero pings → assertion fails.

The rate-cap threshold accounts for aiolimiter's initial bucket: with
max_rate=R and time_period=1 the first R requests fire immediately
(bucket starts full), so n requests take approximately (n-R)/R seconds,
not (n-1)/R. We use n=50, R=10 so the canonical solution takes ~4 s,
well above the 2.4 s lower bound — and a no-throttle implementation
finishes in <0.5 s.
"""
from __future__ import annotations

import asyncio
import time

import pytest


@pytest.mark.asyncio
@pytest.mark.timeout(30)
async def test_throughput_holds_under_concurrent_load(client):
    n_requests = 50
    rate_per_sec = 10  # must match AsyncLimiter(max_rate=10, time_period=1)
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

    assert all(r.status_code == 200 for r in responses), \
        f"Some requests failed: {[r.status_code for r in responses]}"

    # Rate cap: must take at least ~ (n - rate) / rate seconds (60% slack
    # to absorb container/CI jitter). The full canonical pacing is
    # 4 seconds for n=50, R=10 — well above this floor.
    expected_min_elapsed = (n_requests - rate_per_sec) / rate_per_sec * 0.6
    assert elapsed >= expected_min_elapsed, \
        (f"finished in {elapsed:.2f}s; rate limit not enforced "
         f"(expected >= {expected_min_elapsed:.2f}s for n={n_requests}, "
         f"rate={rate_per_sec}/s)")

    # Event-loop liveness: canary fires at most elapsed/0.05 times. With
    # asyncio.sleep we expect close to that ceiling; with time.sleep the
    # canary gets near-zero turns. The 10% slack catches the latter
    # without flaking on the former.
    max_pings = int(elapsed / 0.05)
    min_expected_pings = max(int(max_pings * 0.1), 5)
    assert len(canary_pings) >= min_expected_pings, \
        (f"event loop appears blocked: canary fired only {len(canary_pings)} "
         f"times in {elapsed:.2f}s (expected >= {min_expected_pings}). "
         f"Did you use time.sleep instead of asyncio.sleep?")
