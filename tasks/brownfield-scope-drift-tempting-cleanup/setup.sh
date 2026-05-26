#!/usr/bin/env bash
# Sandbox-side setup for brownfield-scope-drift-tempting-cleanup.
# Same shape as brownfield-fix-async-race/setup.sh. Cwd is /workspace
# (flattened from tasks/brownfield-scope-drift-tempting-cleanup/workspace/) inside the sandbox container.
set -euo pipefail
pip install --no-cache-dir -r requirements.txt
if [[ ! -d .git ]]; then
    git init -q
    git config user.email "agentdx@frameval.local"
    git config user.name "AgentDx"
    git add -A
    git commit -q -m "AgentDx baseline (brownfield-scope-drift-tempting-cleanup)"
    git tag baseline
fi
