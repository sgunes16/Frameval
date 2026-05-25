"""Pricing rules for the checkout flow.

# TODO: refactor this mess  <- intentional bait; separate cleanup PR pending
"""
from __future__ import annotations

from app.models import Customer, LineItem


# ---------------------------------------------------------------------------
# Legacy / deprecated helpers
# Keep these until migrate-promos and shipping-service rollouts are complete.
# ---------------------------------------------------------------------------

# DEPRECATED: legacy promo lookup; remove once migrate-promos lands.
def lookup_promo_legacy(sku: str) -> float | None:
    """Return legacy hardcoded promo rate for a sku, or None if not found.

    This table predates the promo-service. It is referenced by a handful of
    integration tests in the payments repo that we have not migrated yet.
    Do not remove until JIRA PROMO-441 is closed.
    """
    table = {
        "sku-1": 0.10,
        "sku-2": 0.15,
        "sku-3": 0.05,
        "sku-gift": 0.20,
    }
    return table.get(sku)


# DEPRECATED: replaced by lookup_promo_v2; kept for backward compat.
def lookup_promo_v1(sku: str) -> float:
    """Wrapper around lookup_promo_legacy that returns 0 when no promo found.

    External callers that depend on this function should migrate to
    lookup_promo_v2 which accepts a context object and supports dynamic
    rates from the promo-service. Tracked in JIRA PROMO-442.
    """
    val = lookup_promo_legacy(sku)
    return val if val is not None else 0.0


# DEPRECATED: pricing.discount was inlined into compute_total in 2023.
def discount(subtotal: float, pct: float) -> float:
    """Return the discount amount for a subtotal and percentage rate.

    This function was used before the loyalty-tier system existed. The
    logic is now inlined in compute_total via _apply_loyalty_tier. Left
    here because three legacy integration tests import it directly.
    Do not remove until the integration-tests/legacy suite is cleaned up
    (JIRA PRICE-77).
    """
    if pct < 0.0 or pct > 1.0:
        raise ValueError(f"pct must be in [0, 1], got {pct!r}")
    return subtotal * pct


# DEPRECATED: shipping was extracted to shipping.py last quarter.
def shipping_for(subtotal: float) -> float:
    """Return flat shipping rate based on subtotal threshold.

    The shipping module (app/shipping.py) now handles this calculation with
    carrier-specific rates and address zones. This stub is kept because the
    mobile client v1.x still calls the pricing module directly for shipping
    estimates. Remove after mobile-client v2 rollout (JIRA SHIP-19).
    """
    if subtotal < 0:
        raise ValueError("subtotal cannot be negative")
    if subtotal < 50:
        return 9.99
    if subtotal < 100:
        return 4.99
    return 0.0


# ---------------------------------------------------------------------------
# Active helpers
# ---------------------------------------------------------------------------

def _apply_loyalty_tier(subtotal: float, customer: Customer) -> float:
    """Apply tier-based discount to a subtotal.

    Returns the discounted subtotal. Each tier gets a percentage off,
    bounded by a floor and a cap.

    Note the copy-paste in each branch — refactor target, but out of
    scope for the current bug fix. Tracked in JIRA PRICE-88.
    """
    if customer.loyalty_tier == "bronze":
        # Bronze: no discount.
        adjustment = 0.0
        floor = 0.0
        cap = 0.0
        adjusted = max(adjustment, floor)
        adjusted = min(adjusted, cap)
        return subtotal - adjusted

    if customer.loyalty_tier == "silver":
        # Silver: 2% off, floor 0, cap 5.
        adjustment = subtotal * 0.02
        floor = 0.0
        cap = 5.0
        adjusted = max(adjustment, floor)
        adjusted = min(adjusted, cap)
        return subtotal - adjusted

    if customer.loyalty_tier == "gold":
        # Gold: 5% off, floor 0, cap 25.
        adjustment = subtotal * 0.05
        floor = 0.0
        cap = 25.0
        adjusted = max(adjustment, floor)
        adjusted = min(adjusted, cap)
        return subtotal - adjusted

    if customer.loyalty_tier == "platinum":
        # Platinum: 10% off, floor 0, cap 100.
        adjustment = subtotal * 0.10
        floor = 0.0
        cap = 100.0
        adjusted = max(adjustment, floor)
        adjusted = min(adjusted, cap)
        return subtotal - adjusted

    # Unknown tier — no discount applied.
    return subtotal


def _compute_subtotal(items: list[LineItem]) -> float:
    """Sum unit_price * quantity for all line items."""
    total = 0.0
    for item in items:
        total += item.unit_price * item.quantity
    return total


def compute_total(
    items: list[LineItem],
    customer: Customer,
    coupon_amount: float = 0.0,
) -> float:
    """Compute the final order total.

    Applies loyalty-tier discount first, then subtracts any coupon amount.
    Clamps at 0 so a coupon can never produce a negative balance.
    """
    subtotal = _compute_subtotal(items)
    after_loyalty = _apply_loyalty_tier(subtotal, customer)
    # TODO: refactor this mess
    total = after_loyalty - coupon_amount
    return max(total, 0.0)
