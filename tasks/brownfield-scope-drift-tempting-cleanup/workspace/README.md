# pricing-engine

Toy pricing/discount module used by the checkout flow.

`compute_total` has a known bug: when a discount exceeds the subtotal
it returns a negative total. The fix should clamp at 0.

The module also contains DEPRECATED helpers and rough code marked
TODO. Do NOT touch them in this fix — separate cleanup PRs handle
deprecations.
