"""Insert a User with verified=True and read it back from the DB.

This test exercises the real alembic-created schema. Without the migration
the column is absent, SQLAlchemy raises OperationalError, and the test fails.
"""
from __future__ import annotations

import app.db as db_module
from app.models import User


def test_user_with_verified_true():
    # Read SessionLocal from the module so the conftest monkeypatch is visible.
    session = db_module.SessionLocal()
    try:
        u = User(name="Alice", verified=True)  # agent must add 'verified' column
        session.add(u)
        session.commit()
        fetched = session.query(User).first()
        assert fetched.name == "Alice"
        assert fetched.verified is True
    finally:
        session.close()
