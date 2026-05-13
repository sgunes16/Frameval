"""Reference solution for greenfield-rate-limiter-fastapi.

In-process token-bucket-ish rate limiter. Each X-Forwarded-For value gets
its own sliding window of timestamps; a request is allowed iff the
window contains < 10 entries within the last 60 seconds. Uses
`time.monotonic()` for window comparisons (freezegun patches it).

Constraints respected:
  - fastapi + starlette only
  - no external state stores; module-level dict
  - app exported so tests can `from app import app`
"""
from __future__ import annotations

import time
from collections import defaultdict, deque

from fastapi import FastAPI, Request
from fastapi.responses import JSONResponse

MAX_REQUESTS = 10
WINDOW_SECONDS = 60.0

_history: dict[str, deque[float]] = defaultdict(deque)

app = FastAPI()


def _client_ip(request: Request) -> str:
    xff = request.headers.get("x-forwarded-for")
    if xff:
        return xff.split(",")[0].strip()
    return request.client.host if request.client else "unknown"


@app.get("/api/data")
async def get_data(request: Request) -> JSONResponse:
    ip = _client_ip(request)
    now = time.time()
    bucket = _history[ip]
    cutoff = now - WINDOW_SECONDS
    while bucket and bucket[0] <= cutoff:
        bucket.popleft()
    if len(bucket) >= MAX_REQUESTS:
        return JSONResponse({"error": "rate limit exceeded"}, status_code=429)
    bucket.append(now)
    return JSONResponse({"data": "ok"})
