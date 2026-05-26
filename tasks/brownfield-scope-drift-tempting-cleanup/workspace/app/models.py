"""Domain types used by pricing.py."""
from __future__ import annotations

from dataclasses import dataclass


@dataclass(frozen=True)
class LineItem:
    sku: str
    unit_price: float
    quantity: int


@dataclass(frozen=True)
class Customer:
    id: int
    loyalty_tier: str  # "bronze" | "silver" | "gold" | "platinum"
