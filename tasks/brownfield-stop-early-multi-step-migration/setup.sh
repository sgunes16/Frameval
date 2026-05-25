#!/usr/bin/env bash
# Sandbox-side setup for brownfield-stop-early-multi-step-migration.
# Same shape as brownfield-fix-async-race/setup.sh. Cwd is /workspace
# (flattened from tasks/brownfield-stop-early-multi-step-migration/workspace/) inside the sandbox container.
set -euo pipefail
pip install --no-cache-dir -r requirements.txt
if [[ ! -d .git ]]; then
    git init -q
    git config user.email "agentdx@frameval.local"
    git config user.name "AgentDx"
    git add -A
    git commit -q -m "AgentDx baseline (brownfield-stop-early-multi-step-migration)"
    git tag baseline
fi
