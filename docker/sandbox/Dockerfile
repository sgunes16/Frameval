FROM ubuntu:22.04
ENV DEBIAN_FRONTEND=noninteractive
RUN apt-get update && apt-get install -y --no-install-recommends     ca-certificates curl git build-essential pkg-config python3 python3-dev python3-pip python3-venv golang-go npm nodejs sqlite3     && rm -rf /var/lib/apt/lists/*
RUN curl https://cursor.com/install -fsS | bash
# opencode is the recommended executor (structured JSON event
# stream out-of-the-box; aider stays installed for back-compat).
RUN curl -fsSL https://opencode.ai/install | bash
ENV PATH="/root/.local/bin:/root/.cursor/bin:/root/.opencode/bin:${PATH}"
# Project deps pre-baked so each task's test_command (which runs in a
# fresh sandbox container) doesn't have to repeat `pip install`. Tasks
# that need a library not listed here either embed their own
# `pip install` in the test_command or contribute the dep upstream.
RUN pip install --no-cache-dir uv pytest pytest-asyncio pytest-timeout ruff mypy aider-chat \
    aiolimiter sqlalchemy alembic pyyaml openapi-spec-validator
# Frameval copies workspaces in/out of the sandbox via tar; the host
# uid (typically 501 on macOS) survives the round-trip while the
# container process is root (uid 0). git ≥2.35 refuses to operate on
# any directory whose owner uid doesn't match the current uid
# ("dubious ownership"), which breaks test_scope.sh's `git diff
# --name-only baseline`. The sandbox is an ephemeral evaluation env,
# not a multi-tenant prod host, so trusting every directory is safe.
RUN git config --system --add safe.directory '*'
WORKDIR /workspace
CMD ["bash"]
