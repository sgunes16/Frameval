#!/usr/bin/env bash
# Sandbox-side setup for brownfield-fix-async-race.
#
# Installs deps, initializes the workspace as a git repo, and tags the
# baseline commit so test_scope.sh can diff against it later. The agent
# never sees this script; it runs before the agent's invocation.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$SCRIPT_DIR/workspace"

pip install --no-cache-dir -r requirements.txt

if [[ ! -d .git ]]; then
    git init -q
    git config user.email "agentdx@frameval.local"
    git config user.name "AgentDx"
    git add -A
    git commit -q -m "AgentDx baseline (brownfield-fix-async-race)"
    git tag baseline
fi
