FROM ubuntu:22.04
ENV DEBIAN_FRONTEND=noninteractive
RUN apt-get update && apt-get install -y --no-install-recommends     ca-certificates curl git build-essential pkg-config python3 python3-dev python3-pip python3-venv golang-go npm nodejs sqlite3     && rm -rf /var/lib/apt/lists/*
RUN curl https://cursor.com/install -fsS | bash
# opencode is the recommended executor (structured JSON event
# stream out-of-the-box; aider stays installed for back-compat).
RUN curl -fsSL https://opencode.ai/install | bash
ENV PATH="/root/.local/bin:/root/.cursor/bin:/root/.opencode/bin:${PATH}"
RUN pip install --no-cache-dir uv pytest ruff mypy aider-chat
WORKDIR /workspace
CMD ["bash"]
