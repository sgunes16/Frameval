FROM ubuntu:22.04
ENV DEBIAN_FRONTEND=noninteractive
RUN apt-get update && apt-get install -y --no-install-recommends     ca-certificates curl git build-essential pkg-config python3 python3-dev python3-pip python3-venv golang-go npm nodejs sqlite3     && rm -rf /var/lib/apt/lists/*
RUN curl https://cursor.com/install -fsS | bash
ENV PATH="/root/.local/bin:/root/.cursor/bin:${PATH}"
RUN pip install --no-cache-dir uv pytest ruff mypy aider-chat
WORKDIR /workspace
CMD ["bash"]
