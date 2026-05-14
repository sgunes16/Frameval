# Backend Robustness Pass — Design

**Status:** Draft
**Date:** 2026-05-14
**Owner:** sgunes16
**Related:** [[2026-05-12-agentdx-design]], [[2026-05-14-testing-foundation-design]], `engine/internal/experiment/orchestrator.go`, `engine/internal/api/ws_hub.go`, `engine/internal/sandbox/manager.go`

## 1. Motivation

The engine functions, but a backend audit (this session) surfaced ten classes of latent failure that will bite us in production demos and at scale. These are not architectural complaints — the architecture is good. They are *implementation* bugs and *operational* gaps. Each section below names the file:line and the specific fix.

User pain (verbatim): *"make the project more robust."*

Concrete failure modes today:

1. Run errors are silently swallowed (`orchestrator.go:56` discards return value of `executeRun`). Runs stuck in `running` forever.
2. WebSocket broadcast is non-blocking per-client but blocking globally — a slow client never receives messages, but if the hub's `broadcast` channel (cap 256) overflows, every broadcast blocks (`ws_hub.go`).
3. Workspace cleanup races with `runTaskVerifications` goroutines (`orchestrator.go:198, 436`).
4. Container log streaming reads into an unbounded in-memory buffer (`sandbox/manager.go:300`).
5. Hub.clients map is read in `broadcast` under `RLock` but written under `Lock` in unregister — safe today, but the broadcast loop deletes clients on send failure without acquiring the write lock.
6. No input validation on `POST /api/experiments/`. Negative `runs_per_variant`, `temperature=-1000`, `timeout_seconds=0` all accepted at the API; only the DB CHECK constraints reject them (and only some).
7. gRPC grader client has 30 s deadline, no retries, no circuit-breaker. Single grader stall → all runs fail.
8. Diagnostic persistence errors are caught and discarded (`orchestrator.go:287`). Runs complete with missing diagnostic data and no log line.
9. Raw error strings from internal layers are returned to HTTP clients as JSON (e.g., raw SQL errors, filesystem paths). Information leak risk + bad UX.
10. No structured logging. `log.Printf` everywhere, no trace ID linking an experiment → run → grader call → WS event.

## 2. Goals & non-goals

**Goals.**
- Eliminate the 10 specific failure modes above.
- Bounded resource usage: capped log buffers, capped channels, capped concurrency.
- Structured logging with trace IDs from API request to grader RPC.
- Strict input validation at the API boundary; opaque error strings outward; rich error logs inward.
- gRPC client resilience: retries with backoff, circuit breaker, deadline propagation.
- A failure does not leave inconsistent state: runs always transition to `failed` if they break.

**Non-goals.**
- Rewriting the orchestrator. The shape is fine; we fix bugs and add wrappers.
- Replacing SQLite. The data model stays.
- Introducing distributed tracing infrastructure (OTLP, Jaeger). We emit OTel-compatible logs but ship without a backend in MVP.
- Authentication / authorization. Separate concern.

## 3. Approach

We attack the issues in three tranches: **bug fixes** (named in §4), **infrastructure** (logging, validation, resilience — §5), **observability** (§6). Each tranche has its own implementation chunk and can ship independently.

## 4. Targeted bug fixes

### 4.1 Silent job failures (orchestrator.go:56)

**Today:**
```go
queue.Enqueue(func(jobCtx context.Context) {
    _ = o.executeRun(jobCtx, run.ID)
})
```

**Fix:** Wrap with a recovery + status-flip helper. If `executeRun` returns a non-nil error, the run is forcibly transitioned to `failed` with the error message recorded. Panics are recovered (`runtime/debug.Stack` captured in `error_message`, logged at `ERROR` with the trace ID).

```go
queue.Enqueue(func(jobCtx context.Context) {
    o.runExecuteRunWithRecovery(jobCtx, run.ID)
})
```

The recovery helper lives next to `executeRun`. Existing happy-path tests are unaffected; new test asserts that a panicking executor produces a `failed` run with non-empty `error_message`.

### 4.2 WebSocket hub bounding (ws_hub.go)

**Today:** `broadcast` channel cap 256 is the only backpressure. If 8 concurrent experiments × 30 log lines/sec saturate the channel, every emitter blocks indefinitely.

**Fix:**
- Make `broadcast` a non-blocking emit from producers. The orchestrator calls `hub.TryBroadcast(msg)`; if the channel is full, the message is dropped and a counter `frameval_ws_dropped_total` is incremented. We never block the orchestrator on WS plumbing.
- Per-client send remains non-blocking (already correct).
- Replace the `RLock`-only iteration in the broadcast loop with a snapshot pattern: build a slice of client channels under `RLock`, then iterate the slice without holding the lock. Removes the iteration-vs-delete hazard.

A new test boots a hub, registers a slow client, fires 10 k messages, and asserts no goroutine is blocked at the end.

### 4.3 Workspace cleanup race (orchestrator.go:198, 436)

**Today:** `defer os.RemoveAll(workspace)` runs immediately after `executeRun` returns, but `runTaskVerifications` spawns goroutines for parallel verifications that may still read from `workspace`.

**Fix:** Use a `sync.WaitGroup` (or `errgroup.Group`) for verification goroutines; the cleanup `defer` waits on it before deleting. Verifications get a derived context with a timeout; if they hang, the workspace is deleted anyway after the timeout and verifications are marked `errored`.

### 4.4 Unbounded container log buffer (sandbox/manager.go:300)

**Today:** Logs streamed into `collector.buf` with no cap.

**Fix:** A ring buffer with configurable cap (default 8 MiB). Once full, oldest bytes are evicted. The full log is still flushed to a transient file on disk (`workspace/.frameval-logs`) so we don't lose it, and the in-memory buffer keeps the most recent tail for WS broadcast. The on-disk log is removed on cleanup.

### 4.5 Hub map iteration safety (ws_hub.go)

Covered by §4.2.

### 4.6 API input validation

**Today:** `decodeJSON(r, &target)` in `render.go:14` then straight to storage. No bounds.

**Fix:** Add a validator using `go-playground/validator/v10` tags on every request struct. New file `engine/internal/api/validate.go` exposes `validate(v any) []FieldError`. Handlers call `validate` after decode and return `400` with a structured field-error list:

```json
{ "error": "validation_failed", "fields": [
  {"field": "runs_per_variant", "rule": "min", "param": "5"}
]}
```

Bounds:

| Field | Rule |
|---|---|
| `runs_per_variant` | `min=5,max=200` |
| `temperature` | `min=0,max=2` |
| `timeout_seconds` | `min=60,max=7200` |
| `max_concurrent` | `min=1,max=16` |
| `model` | `required,oneof=<dynamic from /config/models>` (looked up against the in-memory model registry) |
| `harness_ids` | `min=1,max=8,dive,required` |

We also cap request body size to 1 MiB via `http.MaxBytesReader` in `middleware.go`.

### 4.7 gRPC grader resilience

**Fix:** Wrap the existing `grader_client.go` with:
- Per-RPC deadlines kept (30 s for `GradeRun`, 10 s for `ClassifyFailure`, etc.).
- Retry interceptor for `Unavailable` and `DeadlineExceeded`: up to 3 attempts, exponential backoff with jitter (`100ms, 400ms, 1.6s`), only for idempotent RPCs (all grader RPCs are idempotent — they don't write to the DB; engine persists results).
- Circuit breaker (Sony's `gobreaker`): if 5 consecutive failures occur within a 60 s window, the breaker opens for 30 s; requests during open are short-circuited with `grader_unavailable` and the run is marked `grading_failed` (a new run status) rather than dragging timeouts through.

A new metric `frameval_grader_breaker_state{state=open|halfopen|closed}` records the breaker state for monitoring.

### 4.8 Diagnostic persistence errors (orchestrator.go:287)

**Today:** all errors caught and discarded.

**Fix:** Errors get logged at `WARN` level with the run ID and trace ID. We do not fail the run because diagnostic is best-effort by design — but we surface a `diagnostic_status` field on `Run` (`ok | partial | failed`) that the frontend can display.

### 4.9 Outward error sanitization

**Fix:** A `renderError(w, statusCode, code, publicMessage)` helper. Internal error string is logged with the trace ID; client receives `{error: code, message: publicMessage}`. Codes are stable identifiers (`validation_failed`, `not_found`, `internal_error`, `grader_unavailable`, `forbidden`, `conflict`). Existing handlers migrate to the helper one at a time.

### 4.10 Structured logging + trace IDs

**Fix:** Adopt `log/slog` (Go 1.22 std-lib). Every HTTP request gets a `trace_id` (UUIDv7 or middleware-injected `X-Frameval-Trace` header). The trace ID flows through `context.Context`. Logger derived from `slog.With("trace_id", id, "experiment_id", expID, "run_id", runID)` at every layer boundary.

Output is JSON in production, pretty text in dev (`FRAMEVAL_LOG_FORMAT=pretty`).

Three log levels:
- `ERROR` — unexpected failure that requires investigation.
- `WARN` — handled failure (gRPC retry exhausted, diagnostic partial).
- `INFO` — lifecycle transitions (experiment started/finished, run started/finished).
- `DEBUG` — only with `FRAMEVAL_LOG_LEVEL=debug`.

The grader-side Python service mirrors this with structured logging via `structlog`, forwarding the trace ID from gRPC metadata (`x-frameval-trace`).

## 5. Cross-cutting infrastructure

### 5.1 Context discipline

Every long-running goroutine (queue worker, hub run-loop, log collector) accepts a context and watches `ctx.Done()`. No `context.Background()` deep inside handlers. The `cleanup` path that needs to outlive the request context uses a *bounded* detached context (`context.WithTimeout(context.Background(), 30*time.Second)`), not unbounded `context.Background()`.

### 5.2 Storage transaction discipline

Multi-step storage updates (e.g., save transcript + save grade + persist diagnostic + flip run status) currently happen as separate calls. We introduce `storage.Tx(ctx, fn)` that runs all updates in one SQLite transaction. The orchestrator's per-run finalization (`finalizeRun`) becomes one transaction; either everything commits or nothing does, and the run never lands in a half-completed state.

### 5.3 Health endpoint expansion

`GET /api/health` returns `{ok: true}` regardless of state. We extend it (without breaking the legacy shape) to include sub-component health: `{ok, components: { db: ok, docker: ok|degraded|unavailable, grader: ok|circuit_open|unavailable, queue: {depth, active} }}`. The frontend dashboard can display a system-health strip.

## 6. Observability

- **Logs:** structured JSON via slog; trace_id, experiment_id, run_id, component fields.
- **Metrics:** `expvar` or a tiny Prometheus-format `/metrics` endpoint (no `prometheus/client_golang` dependency unless we want full counters/histograms; for MVP, `expvar` is enough). Metrics: queue depth, active workers, runs in each status, grader breaker state, WS dropped count, hub clients count.
- **No tracing in MVP.** We emit OTel-compatible trace IDs in logs so we can adopt OTLP later without re-instrumenting.

## 7. Error handling & edge cases

- **DB locked.** SQLite `BUSY` retries via the existing driver's `_busy_timeout` parameter raised to 5 s. We continue to use one connection for writes; reads are concurrent.
- **Sandbox `Pull` fails.** Surfaced as `sandbox_unavailable` and the experiment can still launch in local fallback mode (existing logic), with a banner on the dashboard.
- **Migration applied partially.** Migrations remain idempotent and additive (already enforced). A new `migrations_status` table records applied migration filenames + checksums so we can detect drift.

## 8. Testing strategy

See [[2026-05-14-testing-foundation-design]] for the harness. For this work specifically:

**Unit (Go):**
- `validate.go`: every validator rule has at least one passing and one failing case.
- `renderError`: maps known codes to expected status codes; unknown error becomes `internal_error`.
- Hub: 1k concurrent broadcasts with slow consumer; no goroutine leaks (verified via `goleak`).
- gRPC client: `Unavailable` triggers retry; 5 consecutive failures open the breaker; half-open returns to closed after a success.

**Integration (Go + testcontainers-go):**
- Full run lifecycle test (`engine/test/integration/orchestrator_test.go`): start a fake executor that panics; run must end in `failed` with stack trace in `error_message`, not stuck in `running`.
- Workspace cleanup vs verification: launch a run whose verifications sleep 10 s; cleanup must not delete workspace early.
- Grader unavailable: shut the grader container; assert breaker opens after 5 failures and runs are marked `grading_failed`.

**Stress test (manual, not CI):**
- 32 concurrent experiments × 10 runs each, asserting (a) no goroutine leak, (b) memory stays below 1 GiB, (c) all runs reach terminal status.

## 9. Migration plan

Order matters because some changes are foundational:

1. **Slog + trace IDs.** Cosmetic for consumers; required by everything else.
2. **Validation + renderError.** Tightens the API boundary; tests can rely on stable error codes.
3. **Hub bounding + workspace cleanup race.** Two highest-impact concurrency fixes.
4. **gRPC resilience + diagnostic status field.**
5. **Storage transactions + ring-buffer log capture.**
6. **Health endpoint expansion.**

Each step is a separate PR with its own integration tests.

## 10. Acceptance criteria

- A panicking executor results in a `failed` run with a non-empty `error_message`. Verified by integration test.
- A slow WS client never blocks the orchestrator. Verified by stress test and `frameval_ws_dropped_total` metric exposed.
- Workspace directory is not deleted while verification goroutines are reading it. Verified by integration test.
- Container logs do not consume more than 8 MiB per run in memory. Verified by manual memory profiling.
- `POST /api/experiments/` rejects negative `runs_per_variant`, out-of-range temperature, and zero timeout with `400 validation_failed`.
- Grader becoming unavailable does not stall the orchestrator beyond 30 s; breaker opens, subsequent runs short-circuit.
- Every HTTP request and every spawned goroutine logs with a consistent `trace_id`.
- `GET /api/health` returns sub-component status; the frontend dashboard surfaces it.

## 11. Out of scope / future

- Distributed tracing backend (OTLP, Jaeger).
- Authentication / authorization (`who can launch experiments`).
- Multi-node engine deployment. Today we assume a single binary; horizontal scaling is post-MVP.
- Replacing SQLite. Future: pluggable storage interface to support Postgres.
