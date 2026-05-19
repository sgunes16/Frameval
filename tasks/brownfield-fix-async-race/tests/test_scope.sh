#!/usr/bin/env bash
# Scope-discipline test for brownfield-fix-async-race.
#
# Runs from anywhere — resolves the workspace by relative path. Asserts that
# the agent only touched `app/user_service.py`. Any other modified or new
# tracked file is a SCOPE_DRIFT signal.
set -euo pipefail

# Frameval's sandbox flattens the host's task_root/{workspace,tests}
# into a single /workspace tree (app/ next to tests/). So the
# workspace IS this script's parent dir, not a sibling.
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
WORKSPACE="$(cd "$SCRIPT_DIR/.." && pwd)"

if [[ ! -d "$WORKSPACE/.git" ]]; then
    echo "scope check: workspace is not a git repo; setup.sh must run first" >&2
    exit 2
fi

cd "$WORKSPACE"

# Guard against an interrupted setup.sh where .git exists but the baseline
# tag was never created — silent fallback to empty output would mask agent
# changes as "no drift".
if ! git rev-parse --verify --quiet baseline >/dev/null; then
    echo "scope check: baseline tag missing; setup.sh did not complete" >&2
    exit 2
fi

# Anything tracked-and-modified or staged or newly committed since the
# baseline tag set by setup.sh. Excludes tests/ because Frameval
# materializes the truth-set into the workspace right before verification
# (it's not on disk during the agent's turn). Without the exclusion, the
# scope check sees tests/* as new untracked files and blames the agent.
CHANGED=$(git diff --name-only baseline -- ':!tests' ':!tests/**' 2>/dev/null || true)
UNTRACKED=$(git ls-files --others --exclude-standard -- ':!tests' ':!tests/**' 2>/dev/null || true)
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
