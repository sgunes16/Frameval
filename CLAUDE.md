# CLAUDE.md — Frameval

## Project Overview

Frameval is an open-source, local-first context engineering evaluation tool. It lets users empirically test how CLAUDE.md files, AGENTS.md configurations, skills, spec-kit templates, and other context artifacts affect AI agent behavior and output quality.

## Architecture

Three-service architecture communicating over well-defined boundaries:

```
frontend/   → Vite + React (TypeScript, shadcn/ui, Recharts, TanStack Query, React Router)
engine/     → Go core engine (Chi router, Docker SDK, SQLite, gRPC client)
grader/     → Python sidecar (gRPC server, LLM SDKs, scipy, instructor)
proto/      → Shared protobuf definitions
```

**IPC**: Go engine calls Python grader over gRPC (protobuf). Frontend talks to engine via REST + WebSocket.

**Storage**: Single SQLite file (`frameval.db`) owned by the Go engine. Python grader has read-only access for re-grading scenarios.

**Sandboxes**: Each experiment run spawns an ephemeral Docker container managed by the Go engine via the Docker SDK.

## Directory Structure

```
frameval/
├── frontend/                # Vite + React SPA
│   ├── src/
│   │   ├── pages/           # Route page components
│   │   ├── components/      # shadcn/ui + custom components
│   │   │   ├── ui/          # shadcn primitives (don't edit directly)
│   │   │   └── ...          # Custom composite components
│   │   ├── lib/             # API client, hooks, types, utils
│   │   ├── routes.tsx       # React Router route definitions
│   │   ├── App.tsx          # Root layout + router outlet
│   │   └── main.tsx         # Entry point + providers
│   └── index.html           # Vite entry HTML
├── engine/                  # Go core engine
│   ├── cmd/server/          # main.go entry point
│   ├── internal/
│   │   ├── api/             # HTTP handlers, router setup, WebSocket hub
│   │   ├── experiment/      # Experiment orchestrator, job queue
│   │   ├── sandbox/         # Docker container lifecycle manager
│   │   ├── executor/        # AgentExecutor interface + adapters (claude, codex, gemini, api, manual)
│   │   ├── storage/         # SQLite repository layer (sqlc or raw queries)
│   │   └── models/          # Domain types (experiment, run, grade, etc.)
│   └── proto/               # Generated gRPC client stubs (from proto/)
├── grader/                  # Python grader sidecar
│   ├── server.py            # gRPC server entry point
│   ├── code_grader/         # Test runner, lint, type check
│   ├── llm_judge/           # LLM-as-Judge with instructor for structured output
│   ├── process_grader/      # Transcript parser, process metric extraction
│   ├── spec_adherence/      # Instruction compliance checker
│   ├── stats/               # Statistical analysis (scipy, numpy)
│   └── proto/               # Generated gRPC server stubs (from proto/)
├── proto/                   # Shared protobuf definitions
│   └── grader.proto         # Source of truth for gRPC interface
├── tasks/                   # Built-in task library
├── baselines/               # Pre-evaluated baseline data (seed.sql)
├── docker/                  # Dockerfiles for engine, grader, sandbox
└── docker-compose.yml       # Three services: frontend, engine, grader
```

## Coding Conventions

### Go (engine/)

- Go 1.22+ with modules
- Use `internal/` for all non-exported packages
- Error handling: wrap errors with `fmt.Errorf("operation: %w", err)`, never ignore errors
- Concurrency: use `context.Context` for cancellation, `errgroup` for parallel operations
- HTTP: Chi router, middleware for logging and recovery, JSON responses via a `render` helper
- Database: raw SQL with `database/sql` + go-sqlite3 driver. Use prepared statements. No ORM.
- Testing: table-driven tests, `testify/assert` for assertions, `testcontainers-go` for integration tests
- Naming: follow standard Go conventions (unexported by default, interfaces named by behavior)
- Docker SDK: `github.com/docker/docker/client` for container lifecycle
- gRPC: generated stubs from `proto/grader.proto`, client lives in `engine/proto/`

### Python (grader/)

- Python 3.11+ with type hints everywhere
- Package manager: uv
- gRPC server: grpcio + grpcio-tools
- LLM calls: `anthropic` and `openai` SDKs with `instructor` for structured output
- Statistics: scipy for Mann-Whitney U, numpy for basic math
- Code analysis: subprocess calls to linters/test runners (not in-process)
- Testing: pytest, no mocking of LLM calls in unit tests (use recorded fixtures)
- Structure: each grader is a module with a `grade()` function taking typed inputs

### TypeScript (frontend/)

- Vite + React SPA with React Router for client-side routing
- All components are client-side (no SSR/server components)
- shadcn/ui for all base components — never build primitives from scratch
- TanStack Query for all API calls (queries + mutations)
- Recharts for charts; Monaco Editor for code editing and diff
- Tailwind CSS for styling, no CSS modules
- Types: shared API types in `src/lib/types.ts`, generated from Go API responses
- Naming: PascalCase for components, camelCase for hooks/utils, kebab-case for files

## Key Interfaces

The most important abstraction in the system:

```go
// engine/internal/executor/executor.go
type AgentExecutor interface {
    Name() string
    SupportedModes() []ExecutionMode
    Execute(ctx context.Context, cfg RunConfig) (*RunResult, error)
    ParseTranscript(raw []byte) (*Transcript, error)
}
```

Every agent adapter (Claude, Codex, Gemini, API mode, Manual mode) implements this interface. To add a new agent, create a new file in `executor/` and register it in the adapter registry.

## Development Commands

```bash
# Start all services
docker compose up --build

# Engine only (development)
cd engine && go run cmd/server/main.go

# Grader only (development)
cd grader && uv run python server.py

# Frontend only (development)
cd frontend && npm run dev

# Regenerate gRPC stubs (after editing proto/grader.proto)
cd proto && buf generate

# Run Go tests
cd engine && go test ./...

# Run Python tests
cd grader && uv run pytest

# Run frontend tests
cd frontend && npm test
```

## Database Migrations

SQLite schema lives in `engine/internal/storage/migrations/`. Migrations are numbered SQL files applied in order on startup. Never modify an existing migration file — create a new one.

## Environment Variables

| Variable | Service | Description |
|---|---|---|
| `FRAMEVAL_DB_PATH` | engine | SQLite file path (default: `./frameval.db`) |
| `FRAMEVAL_GRADER_ADDR` | engine | Grader gRPC address (default: `grader:50051`) |
| `FRAMEVAL_MAX_CONCURRENT` | engine | Max parallel sandboxes (default: `3`) |
| `FRAMEVAL_PORT` | engine | HTTP server port (default: `8080`) |
| `GRADER_PORT` | grader | gRPC server port (default: `50051`) |
| `ANTHROPIC_API_KEY` | grader, sandbox | Anthropic API key |
| `OPENAI_API_KEY` | grader, sandbox | OpenAI API key |
| `GOOGLE_API_KEY` | grader, sandbox | Google API key |

## Important Constraints

- API keys are passed as environment variables to Docker containers. They must never be written to disk inside a sandbox or logged in transcripts.
- The SQLite database is the single source of truth. The grader sidecar only reads from it for re-grading; all writes go through the Go engine.
- Sandbox containers have outbound-only network access (for LLM API calls). No inbound connections.
- Temperature defaults to 0.0 for reproducibility. This is intentional.
- Cross-model judging is enforced by default: if the agent is Claude, the judge must be GPT-4o (or user-overridden).
- Minimum runs per variant is 5. This is enforced in both the API and the UI.
- All gRPC changes start in `proto/grader.proto`. Run `buf generate` to update both Go and Python stubs.
