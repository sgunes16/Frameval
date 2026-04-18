# Frameval

Frameval is a local-first evaluation harness for context engineering artifacts such as `CLAUDE.md`, `AGENTS.md`, `.cursorrules`, and prompt presets. It runs controlled experiments across artifact variants, captures transcripts and file diffs, grades outcomes, and visualizes statistically comparable results.

## Architecture

- `frontend/`: Vite + React UI for artifact editing, experiment setup, monitoring, and results.
- `engine/`: Go orchestration service with REST API, WebSocket hub, SQLite storage, task seeding, and execution queue.
- `grader/`: Python gRPC sidecar for code/process/judge/spec grading and pairwise statistics.
- `tasks/`: Built-in task library used to seed the experiment workspace.
- `baselines/`: Baseline seed data.

## Quickstart

```bash
cd frontend && npm install
cd ../engine && go test ./...
cd .. && python3 -m pytest grader/tests

docker compose up --build
```

## Local Dev

```bash
# Engine hot reload
./scripts/dev-engine.sh

# Frontend hot reload
./scripts/dev-frontend.sh

# Engine + frontend together
./scripts/dev-local.sh
```

Once the stack is running:

1. Open `http://localhost:5173`
2. Create an experiment by choosing a repo source, a task template, and per-variant context sources
3. Attach catalog-backed spec-kit extensions or your own files such as `AGENTS.md`, `CLAUDE.md`, and `.cursorrules`
4. Start the experiment
5. Follow progress in the monitor view and inspect results in the dashboard

## Notes

- The supported executors are `cursor`, `gemini`, and `api`.
- Cursor CLI uses the `agent` binary in headless mode and can be overridden with `FRAMEVAL_CURSOR_COMMAND`.
- Gemini CLI can be overridden with `FRAMEVAL_GEMINI_COMMAND`.
- Spec-kit catalog data is fetched from the upstream community catalog and selected extensions are imported into variant artifacts automatically.
- LLM judge and spec-adherence stages remain in the pipeline, but are disabled by default via `FRAMEVAL_ENABLE_LLM_JUDGE=false` and `FRAMEVAL_ENABLE_SPEC_ADHERENCE=false`.
- Local path workspace sources require the engine process to have filesystem access to the selected path.
- SQLite is the system of record; the default path is `./frameval.db`.
