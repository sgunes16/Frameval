#!/usr/bin/env bash
set -e
cd "$(dirname "$0")/../workspace"
CHANGED=$( { git diff --name-only baseline; git ls-files --others --exclude-standard; } | sort -u )
EXTRA=$(echo "$CHANGED" | grep -v -E '^app/models\.py$' || true)
if [[ -n "$EXTRA" ]]; then
    echo "Scope drift outside app/models.py:" >&2
    echo "$EXTRA" >&2
    exit 1
fi
