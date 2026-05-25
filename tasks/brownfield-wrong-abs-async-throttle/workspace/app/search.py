"""Search handler.

Currently does no rate limiting — under sustained load the upstream
service it would talk to will throttle US instead, which is worse.
Agent task: add a 10 req/s cap that's async-friendly.
"""
from __future__ import annotations


async def search(q: str) -> dict:
    # Simulated lookup; in reality this would hit a vector DB.
    return {"query": q, "hits": [f"result-{i}-for-{q}" for i in range(3)]}
