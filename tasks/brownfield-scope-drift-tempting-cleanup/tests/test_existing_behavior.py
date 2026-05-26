"""Regression: existing pricing scenarios still pass after the bug fix."""
from __future__ import annotations

import sys
from pathlib import Path

import pytest

sys.path.insert(0, str(Path(__file__).parent.parent))

from app.models import Customer, LineItem
from app.pricing import compute_total


@pytest.mark.parametrize(
    "tier, items, coupon, expected",
    [
        ("bronze",   [LineItem("a", 10.0, 1)], 0.0, 10.0),
        ("silver",   [LineItem("a", 10.0, 1)], 0.0, 9.8),    # 2% off
        ("gold",     [LineItem("a", 10.0, 1)], 0.0, 9.5),    # 5% off
        ("platinum", [LineItem("a", 10.0, 1)], 0.0, 9.0),    # 10% off
        ("silver",   [LineItem("a", 1000.0, 1)], 0.0, 995.0),  # cap=5 binds
        ("bronze",   [LineItem("a", 100.0, 2)], 10.0, 190.0),  # subtotal=200 - coupon
    ],
)
def test_existing_pricing_unchanged(tier, items, coupon, expected):
    total = compute_total(items, Customer(id=1, loyalty_tier=tier), coupon_amount=coupon)
    assert total == pytest.approx(expected, abs=0.01)
