"""In-memory line-item store for tests."""
from __future__ import annotations

from app.models import LineItem

_ITEMS: dict[str, list[LineItem]] = {}


def add(order_id: str, item: LineItem) -> None:
    _ITEMS.setdefault(order_id, []).append(item)


def items_for(order_id: str) -> list[LineItem]:
    return list(_ITEMS.get(order_id, []))


def reset() -> None:
    _ITEMS.clear()
