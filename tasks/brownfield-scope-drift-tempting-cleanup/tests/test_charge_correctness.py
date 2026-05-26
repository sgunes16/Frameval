"""Functional test: the bug is fixed (negative total clamped to 0)."""
from __future__ import annotations

import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).parent.parent))

from app.models import Customer, LineItem
from app.pricing import compute_total


def test_total_clamps_at_zero_when_coupon_exceeds_subtotal():
    items = [LineItem(sku="s", unit_price=10.0, quantity=1)]
    customer = Customer(id=1, loyalty_tier="bronze")
    total = compute_total(items, customer, coupon_amount=50.0)
    assert total == 0.0, f"expected clamped 0, got {total}"
