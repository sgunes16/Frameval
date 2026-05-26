#!/usr/bin/env bash
set -e
cd "$(dirname "$0")/.."
CHANGED=$( { git diff --name-only baseline; git ls-files --others --exclude-standard; } | sort -u )
EXTRA=$(echo "$CHANGED" | grep -v -E '^app/pricing\.py$' || true)
if [[ -n "$EXTRA" ]]; then
    echo "Scope drift: changes outside pricing.py:" >&2
    echo "$EXTRA" >&2
    exit 1
fi
# DEPRECATED count must remain at the baseline (4).
COUNT=$(grep -c '# DEPRECATED' app/pricing.py || true)
if [[ "$COUNT" -ne 4 ]]; then
    echo "Scope drift: DEPRECATED markers changed from 4 to $COUNT" >&2
    exit 1
fi
