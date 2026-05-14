# Testing Foundation — Design

**Status:** Draft
**Date:** 2026-05-14
**Owner:** sgunes16
**Related:** [[2026-05-14-run-inspector-v2-design]], [[2026-05-14-compare-v2-design]], [[2026-05-14-design-system-v2-design]], [[2026-05-14-backend-robustness-design]]

## 1. Motivation

Current state of testing across the project:

- **Go engine.** 11 test files (~2k LOC) covering `diagnostic/`, `executor/`, `builtin/harness/`, and `storage/diagnostic_repo`. **Zero tests** for `api/` handlers (8 files), `experiment/orchestrator.go` (598 LOC), `sandbox/manager.go` (857 LOC), `experiment/queue.go`, or the other 11 storage repos.
- **Python grader.** A handful of tests for `failure_classifier/` and `tests/test_code_grader.py`, `tests/test_stats.py`. **Zero tests** for `server.py` (gRPC handlers), `llm_judge/`, `process_grader/`, or `composite.py`.
- **Frontend.** Two test files total. `__tests__/dashboard.test.tsx` (8 lines, 1 assertion) and `components/run-monitor/log-viewer.test.ts`. No component tests, no hook tests, no E2E.
- **CI.** Tests run as part of the existing pipeline, but the actual coverage gates are toothless given how little is tested.

User pain (verbatim): *"implement tdd (integration, e2e, unit test to do all frontend backend) make the project more robust."*

A bare minimum is insufficient. We need a **pyramid** — fast unit tests, focused integration tests, and a small high-value E2E suite — that the team can rely on before every merge.

## 2. Goals & non-goals

**Goals.**
- A consistent test harness per language: Go (`go test` + `testify` + `testcontainers-go` + `goleak`), Python (`pytest` + `pytest-asyncio` + `grpcio-testing`), TypeScript (`vitest` + `@testing-library/react` + `msw`), E2E (`playwright`).
- One canonical command per layer: `make test-engine`, `make test-grader`, `make test-frontend`, `make test-e2e`.
- Fixtures and golden files live next to the code they test, not in a top-level fixtures bin.
- CI matrix: unit + integration on every push; E2E + visual regression on pull requests; nightly cron for the heavy run.
- Coverage gates with realistic targets: 75% statements on new code, 60% on legacy paths, never below the existing baseline.
- A `Makefile` target `make test` runs everything locally that CI runs.

**Non-goals.**
- Migrating away from existing test frameworks. Keep `testify`, keep `vitest`, keep `pytest`.
- Mutation testing or property-based testing as a hard requirement. They are nice to have; we add property-based tests where they pay off (the anchor-alignment algorithm in Compare V2 is a candidate).
- Hitting 100% coverage. Optimizing for assurance, not numbers.

## 3. Test taxonomy

We use four categories. Each has a clear contract and runtime budget.

| Layer | Tool | Runtime budget | What it tests |
|---|---|---|---|
| **Unit** | `go test`, `pytest`, `vitest` | < 30 s total | Pure functions, single struct/class behavior, parsers, validators, reducers. No I/O, no network. |
| **Integration** | `testcontainers-go`, `pytest` w/ real grpc, `vitest` w/ `msw` | < 5 min total | Cross-component slices: orchestrator with a fake executor, gRPC round-trip, frontend hook against a mocked API. |
| **E2E** | `playwright` | < 8 min total | Whole-stack flows: launch a diagnostic from the UI, wait for a run to complete, inspect it. Run against the docker-compose stack. |
| **Visual regression** | `playwright` snapshots | < 4 min total | Every page in both themes, fixed fixture data. Diffs block merges. |

Slow stuff (LLM judge calls, long stress runs) goes in a nightly cron, not on every PR.

## 4. Go engine harness

### 4.1 Layout

```
engine/
├── internal/<pkg>/
│   ├── <code>.go
│   └── <code>_test.go               (unit tests, table-driven)
├── test/
│   ├── integration/
│   │   ├── orchestrator_test.go     (full run lifecycle w/ testcontainers)
│   │   ├── hub_test.go              (concurrency, goleak-protected)
│   │   ├── api_test.go              (HTTP through chi router → in-memory store)
│   │   └── fixtures/                (canned transcripts, task definitions)
│   └── stress/
│       └── orchestrator_stress_test.go   (32×10 runs, gated by build tag)
```

Integration tests build the binary into a testcontainer (or spin up the engine in-process with a temp DB and a fake grader served on a `bufconn` listener).

### 4.2 Dependencies (`engine/go.mod` additions)

- `github.com/stretchr/testify/v2` — assertions, mocks.
- `github.com/testcontainers/testcontainers-go` — for spinning up the grader and a sqlite-backed container in integration.
- `go.uber.org/goleak` — assert no goroutine leaks after each test.
- `google.golang.org/grpc/test/bufconn` — in-memory gRPC for testing the grader client without a real socket.

### 4.3 Standard fixtures

- **`fakeExecutor`** — implements `AgentExecutor`, returns canned `RunResult` from a fixture file. Supports: success, panic, slow output, partial output then stop.
- **`fakeGrader`** — implements `GraderServiceServer`, returns canned `GradeRunResponse`. Supports: success, slow (deadline), unavailable, partial result.
- **`tmpStore`** — opens a fresh in-memory SQLite, runs migrations, returns a `*storage.Store`. Each test gets its own.

### 4.4 Coverage gate

`go test -coverprofile -coverpkg ./engine/internal/...` with a CI step that compares the new coverage to `coverage-baseline.txt` checked into the repo. Drops > 1% fail the build; raises update the baseline on green main.

## 5. Python grader harness

### 5.1 Layout

```
grader/
├── <module>/
│   ├── <code>.py
│   └── tests/test_<code>.py
├── tests/
│   ├── integration/
│   │   ├── test_grade_run.py        (full RPC, fake LLM)
│   │   ├── test_classify_failure.py
│   │   └── conftest.py              (shared fixtures)
│   └── fixtures/
│       ├── transcripts/             (JSON files, parsed turn arrays)
│       ├── tasks/                   (task YAML)
│       └── llm_responses/           (recorded API responses)
```

### 5.2 Dependencies (`grader/pyproject.toml`)

- `pytest`, `pytest-asyncio`, `pytest-cov` (already in).
- `grpcio-testing` for gRPC server in-process testing.
- `respx` or VCR-style recording for replaying LLM responses without hitting the live API.
- `hypothesis` (optional) for property-based tests on `composite.py` (the score combiner has obvious invariants).

### 5.3 LLM mocking strategy

Real LLM calls in tests are forbidden. Three options:

| Option | When |
|---|---|
| **`respx`** | Record once, replay forever. Used for `llm_judge` and `failure_classifier`. |
| **In-test stub via `instructor.from_anthropic(client_stub)`** | Used for the unit tests where we want to assert prompt construction. |
| **VCR cassette** | When response fidelity matters and we want it stored as YAML for human review. |

Recording is gated behind `FRAMEVAL_RECORD=1` env var; never recorded in CI.

### 5.4 Coverage gate

`pytest --cov=grader --cov-fail-under=70` on the grader/ tree. Per-module budgets in `pyproject.toml`.

## 6. Frontend harness

### 6.1 Layout

```
frontend/
├── src/
│   ├── components/...
│   │   └── *.test.tsx        (collocated unit tests)
│   ├── hooks/
│   │   └── *.test.ts
│   ├── lib/
│   │   └── *.test.ts
│   └── pages/
│       └── *.test.tsx        (integration: page + hooks + mocked API)
├── test/
│   ├── e2e/
│   │   ├── launch-diagnostic.spec.ts
│   │   ├── inspect-run.spec.ts
│   │   ├── compare-runs.spec.ts
│   │   └── fixtures/
│   ├── visual/
│   │   ├── design-system.spec.ts
│   │   └── snapshots/
│   └── msw/
│       ├── handlers.ts        (mock API)
│       └── server.ts
└── playwright.config.ts
```

### 6.2 Dependencies

Already in: `vitest`, `@testing-library/react`, `@testing-library/user-event`, `jsdom`, `@vitest/coverage-v8`.

Add:
- `msw` (Mock Service Worker) — intercept fetches in unit/integration; also supports Playwright.
- `playwright/test` — E2E.
- `@axe-core/playwright` — accessibility checks in CI.

### 6.3 Test conventions

- **Unit tests** are collocated. Pattern: `Component.tsx` ↔ `Component.test.tsx`.
- **Hooks tests** mock with `msw` instead of stubbing fetch, so the same handlers can serve E2E.
- **Page-level integration tests** render the page with `MemoryRouter` and a `QueryClientProvider`, with `msw` handlers set per test.
- **Snapshots** are limited to visual regression. We do not use Jest-style serialized DOM snapshots — they're fragile.

### 6.4 Coverage gate

Vitest's c8 coverage with a per-file threshold of 75% statements on `components/system/*` and `components/run-inspector/*` (the high-value, well-defined components), 60% global.

## 7. E2E harness

### 7.1 Playwright setup

`playwright.config.ts` runs against `http://localhost:5173` (Vite dev) with the engine on `http://localhost:8080` and the grader on `localhost:50051`. A `docker-compose.test.yml` adds a deterministic grader (LLM responses canned via `respx`) so the same test produces the same result run after run.

The `before-all` step boots the stack via `docker compose -f docker-compose.test.yml up -d --wait`, seeds three known tasks and one canned diagnostic baseline, then hands off to Playwright.

### 7.2 Test list (MVP)

| Spec | What it covers |
|---|---|
| `launch-diagnostic.spec.ts` | `/diagnostic/launch` form → backend creates experiment → poll until at least one run finishes. |
| `inspect-run.spec.ts` | Open Run Inspector V2 on a completed run; expand a turn; search for a phrase; assert results filter. |
| `compare-runs.spec.ts` | Select 3 runs in Compare V2; switch to Tape; assert ≥ 1 fork glyph; click; assert drawer. |
| `dark-mode.spec.ts` | Toggle theme; navigate three pages; assert no color contrast violations via axe. |
| `streaming.spec.ts` | Start a run; open Inspector while still running; assert turns stream in. |
| `error-states.spec.ts` | Kill the grader container mid-run; assert breaker opens; assert UI surfaces "grading unavailable" rather than spinning forever. |

Total target runtime under 8 minutes on a 4-core CI runner.

### 7.3 Visual regression

A single `design-system.spec.ts` renders every system component (StatusDot, TurnCard, DiffBadge, ScoreBar, SymptomGlyph, EmptyState, LoadingSkeleton, ErrorState, Toast) plus every top-level page on a canned fixture, in both themes. Snapshots stored under `test/visual/snapshots/`. Diff threshold 0.1% pixel difference.

## 8. CI wiring

A single GitHub Actions workflow (`.github/workflows/ci.yml`) with three jobs:

```yaml
jobs:
  engine:
    steps:
      - go test -race ./engine/...
      - go test -tags=integration ./engine/test/integration/...

  grader:
    steps:
      - uv run pytest grader/

  frontend:
    steps:
      - npm test -- --coverage
      - npx playwright install --with-deps
      - npm run test:e2e
      - npm run test:visual
```

PRs require all three to pass. The nightly stress test runs the `stress` build tag job and the `FRAMEVAL_RECORD=1` LLM-fidelity job.

A separate workflow uploads coverage reports as artifacts; no SaaS dependency.

## 9. Migration plan

Hard to retrofit tests for 2500 LOC of untested code in one PR. Phased:

1. **Land the harnesses.** Make targets, deps, CI wiring, fixture skeletons. No tests yet.
2. **Smoke E2E.** Three Playwright tests (`launch-diagnostic`, `inspect-run`, `compare-runs`) using *current* UI to lock in basic behavior. These will need updates when V2 ships — that's fine.
3. **Backfill orchestrator tests.** Highest-value unit + integration tests for the most failure-prone code path.
4. **Backfill API handler tests.** One test per endpoint covering happy path + validation failure.
5. **Backfill grader RPC tests.** One test per RPC.
6. **Add frontend hook tests** as Run Inspector V2 lands (each hook gets tests in its own PR).
7. **Visual regression baseline** captured after Design System V2 has stabilized.

Each step is a separate PR. By step 5 we have a meaningful pyramid.

## 10. Acceptance criteria

- `make test` runs all unit + integration locally in under 5 minutes on a developer laptop.
- `make test-e2e` runs E2E locally against the dev stack in under 10 minutes.
- A push to a feature branch triggers engine + grader + frontend test jobs; a PR adds E2E + visual.
- A new contributor can add a feature with tests by following the patterns in any existing component's `*.test.tsx` and `*_test.go`.
- Coverage of the engine `experiment/` package rises to ≥ 70% statements (currently 0%).
- Coverage of the grader is ≥ 70%.
- Coverage of frontend `components/run-inspector/` and `components/system/` is ≥ 80% statements.
- Zero `t.Skip` / `it.skip` / `pytest.skip` in main branch outside of platform-specific gates.

## 11. Open questions

- **Where do recorded LLM responses live?** Option A: in the repo (tiny, deterministic). Option B: in an artifact bucket (avoids repo bloat). Recommend A for MVP — responses gzipped, < 1 MiB total.
- **Cursor cloud executor in E2E?** It requires a real `CURSOR_API_KEY` and hits a paid service. Recommend: only nightly cron, behind a label-gated workflow trigger. PR E2E uses Aider against local Ollama.
- **Snapshot policy on Playwright visual tests across OS?** Pixel-diff on Linux runner only; Mac local runs use a tolerant threshold and don't fail merges.

## 12. Out of scope / future

- Mutation testing.
- Fuzzing the engine HTTP API (could add `go-fuzz` later).
- Load testing (could add `k6` later).
- Cross-browser E2E (Chromium-only for MVP).
