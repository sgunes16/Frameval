# AGENTS.md — Frameval

## What is Frameval?

Frameval is an open-source, local-first tool for empirically evaluating context engineering artifacts (CLAUDE.md, AGENTS.md, skills, spec-kit templates, MCP configs). It measures how these artifacts affect AI agent performance through controlled experiments with statistical rigor.

## Architecture at a Glance

```
┌──────────────┐    REST/WS     ┌──────────────┐    gRPC      ┌──────────────┐
│   Frontend   │ ◄────────────► │  Go Engine   │ ◄──────────► │ Python Grader│
│ (Vite+React) │                │  (Chi, SQLite)│              │ (LLM Judge,  │
│  Port 5173   │                │  Port 8080   │              │  Stats, Code)│
└──────────────┘                └──────┬───────┘              │  Port 50051  │
                                       │ Docker SDK           └──────────────┘
                                       ▼
                                ┌──────────────┐
                                │  Sandboxes   │
                                │ (ephemeral   │
                                │  containers) │
                                └──────────────┘
```

**Go Engine** owns: HTTP API, WebSocket hub, experiment orchestration, job queue, Docker sandbox lifecycle, SQLite database.

**Python Grader** owns: code grading (test execution, lint), LLM-as-Judge (structured rubric evaluation), process metrics (transcript analysis), spec adherence scoring, statistical analysis.

**Frontend** owns: UI rendering, client-side state, API calls via TanStack Query, real-time updates via WebSocket.

Communication boundary: Go calls Python exclusively over gRPC. Frontend calls Go exclusively over REST/WebSocket. No direct frontend-to-grader communication.

## Repository Layout

```
engine/                     # Go — core engine
├── cmd/server/main.go      # Entry point
├── internal/
│   ├── api/                # Chi router, handlers, middleware, WebSocket hub
│   ├── experiment/         # Orchestrator, job queue (goroutine pool)
│   ├── sandbox/            # Docker container manager
│   ├── executor/           # AgentExecutor interface + per-agent adapters
│   │   ├── executor.go     # Interface definition
│   │   ├── claude.go       # Claude Code adapter
│   │   ├── codex.go        # Codex CLI adapter
│   │   ├── gemini.go       # Gemini CLI adapter
│   │   ├── api_mode.go     # Direct LLM API adapter
│   │   ├── manual.go       # Manual transcript upload adapter
│   │   └── registry.go     # Adapter registration
│   ├── storage/            # SQLite repos + migrations
│   └── models/             # Domain types
└── proto/                  # Generated gRPC client stubs

grader/                     # Python — grader sidecar
├── server.py               # gRPC server entry
├── code_grader/            # Runs tests, lint, type check in sandbox artifacts
├── llm_judge/              # Calls cross-model LLM with rubric, uses instructor
├── process_grader/         # Parses transcript, extracts process metrics
├── spec_adherence/         # Evaluates instruction-by-instruction compliance
├── stats/                  # Mann-Whitney U, Cohen's d, bootstrap CI, power analysis
└── proto/                  # Generated gRPC server stubs

frontend/                   # TypeScript — Vite + React SPA
├── src/
│   ├── pages/              # Route page components
│   ├── components/         # UI components (shadcn/ui base + custom)
│   ├── lib/                # API client, hooks, types, utilities
│   ├── routes.tsx          # React Router route definitions
│   ├── App.tsx             # Root layout + router outlet
│   └── main.tsx            # Entry point + providers
└── index.html              # Vite entry HTML

proto/                      # Shared protobuf definitions
└── grader.proto            # Single source of truth for gRPC interface

tasks/                      # Built-in evaluation tasks
├── jwt-auth-express/       # Each task: README.md, codebase/, tests/, setup.sh
├── rate-limiter-custom/
└── ...

baselines/                  # Pre-evaluated baseline data
├── seed.sql                # SQLite seed for baseline grades
└── transcripts/            # Archived transcripts for baseline runs

docker/
├── engine/Dockerfile
├── grader/Dockerfile
└── sandbox/Dockerfile      # Base image for agent execution sandboxes
```

## Development Setup

### Prerequisites

- Docker Engine 24+ and Docker Compose v2
- Go 1.22+
- Python 3.11+ with uv
- Node.js 20+ with npm
- buf (for protobuf generation)

### Running Locally

```bash
# Full stack via Docker Compose
docker compose up --build

# Or run services individually for development:

# Terminal 1: Engine
cd engine && go run cmd/server/main.go

# Terminal 2: Grader
cd grader && uv run python server.py

# Terminal 3: Frontend
cd frontend && npm install && npm run dev
```

### Running Tests

```bash
cd engine && go test ./...          # Go unit + integration tests
cd grader && uv run pytest          # Python grader tests
cd frontend && npm test             # Frontend tests
```

### Regenerating gRPC Stubs

After editing `proto/grader.proto`:

```bash
cd proto && buf generate
```

This generates Go stubs in `engine/proto/` and Python stubs in `grader/proto/`.

## Contributing Guidelines

### Adding a New Agent Adapter

1. Create `engine/internal/executor/<agent>.go`
2. Implement the `AgentExecutor` interface:
   ```go
   type AgentExecutor interface {
       Name() string
       SupportedModes() []ExecutionMode
       Execute(ctx context.Context, cfg RunConfig) (*RunResult, error)
       ParseTranscript(raw []byte) (*Transcript, error)
   }
   ```
3. Register the adapter in `engine/internal/executor/registry.go`
4. Add the agent to the sandbox Dockerfile if it needs a CLI tool installed
5. Add tests in `engine/internal/executor/<agent>_test.go`

### Adding a New Grader Dimension

1. Add the field to the relevant protobuf message in `proto/grader.proto`
2. Run `buf generate` to update stubs
3. Implement the extraction logic in the appropriate grader module (`grader/code_grader/`, etc.)
4. Add the field to the SQLite `grades` table via a new migration in `engine/internal/storage/migrations/`
5. Expose in the REST API response and update the frontend results view

### Adding a New Task

1. Create a directory in `tasks/<task-id>/`
2. Include:
   - `README.md` — task description, prompt, technical details
   - `codebase/` — starter codebase (if brownfield)
   - `tests/` — test suite files
   - `setup.sh` — dependency installation and environment setup
   - `task.json` — metadata (name, category, complexity_score, codebase_type)
3. The task auto-registers on engine startup by scanning the `tasks/` directory

### Database Migrations

- Migrations live in `engine/internal/storage/migrations/`
- Files are named `NNN_description.sql` (e.g., `001_initial_schema.sql`)
- Never modify an existing migration — create a new one
- Migrations run automatically on engine startup

### PR Conventions

- Branch naming: `feat/<description>`, `fix/<description>`, `refactor/<description>`
- Commit messages: imperative mood, concise ("add codex executor adapter", "fix race in sandbox cleanup")
- PRs must include: description of what changed, testing done, screenshots for UI changes
- Go code must pass `go vet` and `golangci-lint`
- Python code must pass `ruff check` and `mypy`
- Frontend code must pass `eslint` and `tsc --noEmit`

## Key Design Decisions

1. **Go + Python hybrid**: Go handles concurrency-heavy orchestration; Python handles LLM/ML ecosystem work. They communicate via gRPC with shared protobuf schemas.

2. **SQLite over Postgres**: Local-first simplicity. Single file backup. No configuration. Sufficient for expected scale (thousands of runs, not millions).

3. **Three execution modes**: CLI Agent (Docker sandbox), API (direct LLM call), Manual (user uploads transcript). This covers the full spectrum from fully-automated to IDE-based agents.

4. **Cross-model judging**: When the agent uses Claude, the judge uses GPT-4o (and vice versa). This is enforced by default to prevent self-preference bias.

5. **Minimum n=5**: Statistical validity requires sufficient sample size. The system enforces this minimum and shows power analysis to encourage appropriate run counts.

6. **Environment fingerprinting**: Every run records Docker image SHA, CLI version, model ID, temperature, seed. This enables reproducibility auditing.
