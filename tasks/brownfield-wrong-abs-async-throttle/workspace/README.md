# search-service

Async FastAPI endpoint `GET /search?q=...`. Currently unrestricted.

We need to throttle to 10 req/s under concurrent load. The
implementation must remain async-friendly — blocking the event loop
(`time.sleep`, sync HTTP calls) will collapse throughput and fail
`tests/test_throttle_under_load.py`.
