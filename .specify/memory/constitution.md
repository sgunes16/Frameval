# Frameval — Project Constitution

## Mission

Build a tool that brings empirical rigor to context engineering. Every design decision must serve this mission: enabling users to measure, compare, and improve the instructions they give AI agents.

## Governing Principles

### 1. Measurement Over Opinion

No claim about spec quality is valid without data. The system must produce statistically sound comparisons. Point estimates without confidence intervals are insufficient. Minimum sample sizes are enforced, not suggested.

### 2. Local-First, Zero Trust

All user data stays on their machine. No telemetry, no cloud sync, no analytics. API keys are transient (environment variables) and never persisted in logs or databases. The tool works fully offline except for LLM API calls during experiment runs.

### 3. Reproducibility as a Feature

Every experiment must be reproducible. This means: environment fingerprinting on every run, raw transcript archival, deterministic defaults (temperature=0), and the ability to re-grade without re-running. If a result cannot be reproduced, the system should detect and flag this.

### 4. Statistical Honesty

Never present a result as significant when it is not. Show p-values, effect sizes, and confidence intervals. Color-code significance levels. Warn when sample size is insufficient. The system must resist the temptation to show flashy results at the expense of accuracy.

### 5. Extensibility Without Complexity

New agents, graders, tasks, and artifact types must be addable without modifying core code. Use interfaces (Go) and contracts (protobuf) to define extension points. The adapter pattern for agent executors and the plugin-like grader architecture serve this principle.

### 6. Cost Transparency

Users pay for every LLM API call. The system must estimate costs before execution, track actual costs during execution, and display total costs prominently in results. Never start an experiment without user confirmation of the estimated cost.

### 7. Separation of Concerns

Go handles orchestration and I/O. Python handles intelligence (LLM calls, statistical analysis). Frontend handles presentation. These boundaries are enforced by the gRPC interface and REST API. No service should reach into another's domain.

## Quality Standards

### Code Quality

- All code must pass linting (golangci-lint for Go, ruff for Python, eslint for TypeScript)
- All code must pass type checking (go vet, mypy, tsc --noEmit)
- Public interfaces must have tests
- Integration tests must cover the happy path of every API endpoint
- No dead code, no commented-out code, no TODO comments without linked issues

### API Design

- REST endpoints follow resource-oriented design
- All responses include appropriate HTTP status codes
- Error responses include: error code, human-readable message, and suggested fix
- Breaking API changes require a new version prefix

### Database

- All schema changes via numbered migrations, never ad-hoc ALTER TABLE
- Foreign keys enforced
- Indexes on all foreign keys and frequently queried columns
- No data deletion without explicit user action (soft delete where appropriate)

### Security

- API keys encrypted at rest (AES-256-GCM)
- Container network isolation (outbound-only, LLM API allowlist)
- Input sanitization on all user-provided values before SQL or Docker commands
- No shell injection vectors (use SDK APIs, not shell commands with string interpolation)

### Performance

- Dashboard loads in under 2 seconds for experiments with up to 100 runs
- API responses under 200ms for read operations
- WebSocket events delivered within 500ms of occurrence
- Docker container startup under 10 seconds with pre-built images

## Decision Framework

When facing a technical decision, apply these priorities in order:

1. **Correctness**: Does it produce accurate, statistically valid results?
2. **Reliability**: Does it handle failures gracefully without losing data?
3. **Simplicity**: Is this the simplest approach that meets the requirement?
4. **Performance**: Is it fast enough for the expected use case?
5. **Extensibility**: Can it be extended without modifying existing code?

If two approaches are equally correct and reliable, choose the simpler one.
