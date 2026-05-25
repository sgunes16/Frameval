"""Reference solution — SQLAlchemy ORM model with verified column."""
from __future__ import annotations

from sqlalchemy import Boolean, Column, Integer, String

from app.db import Base


class User(Base):
    __tablename__ = "users"
    id = Column(Integer, primary_key=True)
    name = Column(String(80), nullable=False)
    verified = Column(Boolean, nullable=False, default=False)
