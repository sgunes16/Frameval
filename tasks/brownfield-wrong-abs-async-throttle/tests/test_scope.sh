#!/usr/bin/env bash
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
EXTRA=$(echo "$CHANGED" | grep -v -E '^app/search\.py$' || true)
if [[ -n "$EXTRA" ]]; then
    echo "Scope drift outside app/search.py:" >&2
    echo "$EXTRA" >&2
    exit 1
fi
