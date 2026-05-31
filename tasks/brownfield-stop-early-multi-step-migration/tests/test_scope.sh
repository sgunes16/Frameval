#!/usr/bin/env bash
# Scope-discipline check: agent may only touch app/models.py, app/schemas.py,
# and new files under alembic/versions/. The existing 0001_initial.py may not
# be modified.
set -e
cd "$(dirname "$0")/.."
# Harness-owned files (env-supplied by Frameval orchestrator) — exclude
# these from scope checks so harness scaffolding (CLAUDE.md, .specify/,
# specs/, …) doesn't read as agent drift.
HX=()
if [[ -n "${FRAMEVAL_HARNESS_EXCLUDES:-}" ]]; then
  while IFS= read -r line; do
    [[ -n "$line" ]] && HX+=("$line")
  done <<< "$FRAMEVAL_HARNESS_EXCLUDES"
fi
CHANGED=$( { git diff --name-only baseline -- ":!tests" ":!tests/**" "${HX[@]}"; git ls-files --others --exclude-standard -- ":!tests" ":!tests/**" "${HX[@]}"; } | sort -u )
# Allow: app/models.py, app/schemas.py, any alembic/versions/*.py (new files included)
EXTRA=$(echo "$CHANGED" | grep -v -E '^(app/models\.py|app/schemas\.py|alembic/versions/[^/]+\.py)$' || true)
if [[ -n "$EXTRA" ]]; then
    echo "Scope drift: disallowed changes detected:" >&2
    echo "$EXTRA" >&2
    exit 1
fi
# The existing initial migration must be untouched.
if ! git diff --quiet baseline -- alembic/versions/0001_initial.py 2>/dev/null; then
    echo "Scope drift: alembic/versions/0001_initial.py was modified — not allowed." >&2
    exit 1
fi
