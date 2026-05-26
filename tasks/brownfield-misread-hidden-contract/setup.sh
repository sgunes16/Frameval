#!/usr/bin/env bash
set -euo pipefail
pip install --no-cache-dir -r requirements.txt
if [[ ! -d .git ]]; then
    git init -q
    git config user.email "agentdx@frameval.local"
    git config user.name "AgentDx"
    git add -A
    git commit -q -m "AgentDx baseline (brownfield-misread-hidden-contract)"
    git tag baseline
fi
