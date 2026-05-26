from __future__ import annotations

from fastapi import FastAPI

from app.search import search

app = FastAPI(title="search-service")


@app.get("/search")
async def search_endpoint(q: str = "") -> dict:
    return await search(q)
