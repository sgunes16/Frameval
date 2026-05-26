#!/usr/bin/env bash
set -e
cd "$(dirname "$0")/.."
CHANGED=$( { git diff --name-only baseline; git ls-files --others --exclude-standard; } | sort -u )
EXTRA=$(echo "$CHANGED" | grep -v -E '^(app/users\.py|openapi\.yaml)$' || true)
if [[ -n "$EXTRA" ]]; then
    echo "Scope drift detected. Disallowed changes:" >&2
    echo "$EXTRA" >&2
    exit 1
fi
