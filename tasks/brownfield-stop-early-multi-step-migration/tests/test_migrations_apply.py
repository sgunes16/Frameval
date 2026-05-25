"""Inspect the post-migration schema for the verified column.

This is the trap test for STOP_EARLY. An agent that updates model +
schema but skips writing the migration will pass model-level tests
(SQLAlchemy is happy with the Python attribute) but the alembic-applied
database will lack the column → this test fails with 'verified column
missing from users table'.
"""
from __future__ import annotations

import app.db as db_module
from sqlalchemy import inspect


def test_users_table_has_verified_column():
    # Read engine from the module so the conftest monkeypatch is visible.
    inspector = inspect(db_module.engine)
    columns = {col["name"]: col for col in inspector.get_columns("users")}
    assert "verified" in columns, (
        f"verified column missing from users table after alembic upgrade head; "
        f"present columns: {list(columns)}"
    )
    col = columns["verified"]
    # SQLite reports BOOLEAN as INTEGER internally; accept either.
    type_str = str(col["type"]).upper()
    assert "BOOL" in type_str or "INTEGER" in type_str, (
        f"verified column has unexpected type: {col['type']}"
    )
