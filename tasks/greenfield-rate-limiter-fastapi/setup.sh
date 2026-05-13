#!/usr/bin/env bash
# Sandbox-side setup for greenfield-rate-limiter-fastapi.
set -euo pipefail
pip install --no-cache-dir fastapi starlette httpx pytest freezegun
