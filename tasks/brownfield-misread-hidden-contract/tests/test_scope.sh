#!/usr/bin/env bash
set -e
cd "$(dirname "$0")/.."
CHANGED=$( { git diff --name-only baseline -- ":!tests" ":!tests/**"; git ls-files --others --exclude-standard -- ":!tests" ":!tests/**"; } | sort -u )
EXTRA=$(echo "$CHANGED" | grep -v -E '^(app/users\.py|docs/api/contract\.v1\.yaml)$' || true)
if [[ -n "$EXTRA" ]]; then
    echo "Scope drift detected. Disallowed changes:" >&2
    echo "$EXTRA" >&2
    exit 1
fi
