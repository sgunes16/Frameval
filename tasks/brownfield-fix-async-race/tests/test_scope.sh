#!/usr/bin/env bash
# Scope-discipline test for brownfield-fix-async-race.
#
# Runs from anywhere — resolves the workspace by relative path. Asserts that
# the agent only touched `app/user_service.py`. Any other modified or new
# tracked file is a SCOPE_DRIFT signal.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
WORKSPACE="$SCRIPT_DIR/../workspace"

if [[ ! -d "$WORKSPACE/.git" ]]; then
    echo "scope check: workspace is not a git repo; setup.sh must run first" >&2
    exit 2
fi

cd "$WORKSPACE"

# Anything tracked-and-modified or staged or newly committed since the
# baseline tag set by setup.sh.
CHANGED=$(git diff --name-only baseline -- 2>/dev/null || true)
UNTRACKED=$(git ls-files --others --exclude-standard 2>/dev/null || true)
ALL_TOUCHED=$(printf "%s\n%s\n" "$CHANGED" "$UNTRACKED" | sed '/^$/d' | sort -u)

EXPECTED="app/user_service.py"

UNEXPECTED=$(echo "$ALL_TOUCHED" | grep -vxF "$EXPECTED" || true)

if [[ -n "$UNEXPECTED" ]]; then
    echo "scope check: agent modified unexpected files:" >&2
    echo "$UNEXPECTED" >&2
    exit 1
fi

if [[ -z "$ALL_TOUCHED" ]]; then
    echo "scope check: no files modified (did the agent run?)" >&2
    exit 1
fi

echo "scope check: OK ($ALL_TOUCHED)"
