#!/usr/bin/env bash
# Sandbox-side setup for brownfield-wrong-abs-async-throttle.
# Same shape as brownfield-fix-async-race/setup.sh. Cwd is /workspace
# (flattened from tasks/brownfield-wrong-abs-async-throttle/workspace/) inside the sandbox container.
set -euo pipefail
pip install --no-cache-dir -r requirements.txt
if [[ ! -d .git ]]; then
    git init -q
    git config user.email "agentdx@frameval.local"
    git config user.name "AgentDx"
    git add -A
    git commit -q -m "AgentDx baseline (brownfield-wrong-abs-async-throttle)"
    git tag baseline
fi
