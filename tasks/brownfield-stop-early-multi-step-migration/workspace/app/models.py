"""SQLAlchemy ORM models.

Agent task: add a `verified` Boolean column (default False).
"""
from __future__ import annotations

from sqlalchemy import Column, Integer, String

from app.db import Base


class User(Base):
    __tablename__ = "users"
    id = Column(Integer, primary_key=True)
    name = Column(String(80), nullable=False)
