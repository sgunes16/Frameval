"""FastAPI entry point — mounts the users router."""
from __future__ import annotations

from fastapi import FastAPI

from app import users

app = FastAPI(title="user-service")
app.include_router(users.router)
