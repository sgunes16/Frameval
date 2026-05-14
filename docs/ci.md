# CI

This document describes how Frameval's continuous-integration pipeline is structured and how to run it locally before opening a pull request.

## Workflows

Two GitHub Actions workflows live in `.github/workflows/`:

| File | Trigger | Purpose |
|---|---|---|
| `ci.yml` | `push` to `main`, every `pull_request` | The merge gate. Engine + Grader + Frontend jobs in parallel. |
| `nightly.yml` | Daily at 04:00 UTC + `workflow_dispatch` | Expensive jobs that don't belong on every PR: stress tests, Playwright E2E, LLM-fidelity cassette diff. |

### `ci.yml` jobs

A small `changes` job runs first and detects which subdirectories the PR touched. The three service jobs then run **conditionally** based on those paths:

| Job | Runs when | Steps |
|---|---|---|
| **changes** | always | `dorny/paths-filter@v3` — sets `engine`, `grader`, `frontend`, `ci` outputs |
| **Engine (Go)** | push to main, or PR touching `engine/**`, `proto/**`, or CI files | `go vet`, `go build`, `go test -race`, `go test -race -tags=integration ./test/integration/...` |
| **Grader (Python)** | push to main, or PR touching `grader/**`, `proto/**`, or CI files | `uv sync`, `uv run ruff check .`, `uv run pytest` |
| **Frontend (TypeScript)** | push to main, or PR touching `frontend/**` or CI files | `npm ci`, `npm run lint` (`tsc --noEmit`), `npm run build`, `npm test` |

On every push to `main`, all three jobs always run regardless of paths — this is the final guarantee that nothing broken lands. On PRs, the path filter trims out jobs that can't possibly be affected, cutting typical PR runtime by 2–3×. The `ci` filter is the safety net: any change to `.github/workflows/**` or the `Makefile` runs every job so a broken pipeline can't slip through by touching only the workflow file.

`proto/**` belongs to both engine *and* grader filters because the protobuf stubs cross both services.

The default Linux runners are used; no self-hosted runners.

## Running CI locally with `act`

[`act`](https://github.com/nektos/act) executes GitHub Actions workflows on your laptop using Docker. Install it once:

```bash
brew install act           # macOS
# or
gh extension install nektos/gh-act
```

Then use the `Makefile` targets that wrap `act`:

```bash
make ci-local         # run the full pull_request event
make ci-engine        # run only the Engine job
make ci-grader        # run only the Grader job
make ci-frontend      # run only the Frontend job
```

The first invocation downloads a multi-GB Docker image (`catthehacker/ubuntu:act-latest` by default). Subsequent runs reuse the cache.

### When `act` and real CI diverge

`act` runs against the same workflow YAML the real CI runner sees, but it differs in a few ways:

- It runs on your local Docker; macOS / Apple Silicon users need `--container-architecture linux/amd64`. The cleanest way to set this once is a `~/.actrc` file containing a single line `--container-architecture linux/amd64` — then every `make ci-*` target picks it up automatically.
- `secrets.ANTHROPIC_API_KEY` and other secrets are *not* set unless you pass them via `-s ANTHROPIC_API_KEY=...`.
- Scheduled jobs (`nightly.yml`) don't fire under `act push` / `act pull_request`. Use `act schedule` to test them.

If `act` passes but real CI fails (or vice versa), open an issue with the divergence so we can correct it. The contract is: a green `act` run should imply a green CI run.

## Pre-merge checklist

Per the project's PR workflow, every non-trivial PR must:

1. **Run the full local validation** (lint + build + tests across every service, not just files you touched):
   ```bash
   make lint
   make build
   make test
   make test-engine-integration
   ```
   Or, equivalently, `make ci-local`.

2. **Pass `act pull_request`** if you have Docker available.

3. **Pass code-review** from the `feature-dev:code-reviewer` subagent.

4. **Address every reviewer-flagged issue** (fix or explicitly justify).

5. **Push fixup commits** to the same branch.

6. **Merge** with `gh pr merge <number> --squash --delete-branch`.

7. **Sync local main**: `git checkout main && git pull --ff-only`.

Only PRs that complete every step are mergeable.

## Coverage gate

> **Not yet wired** — this section describes the planned mechanism.

A coverage baseline will live at `coverage-baseline.txt` (added incrementally as each service's tests stabilize). Future CI jobs will compare current coverage to the baseline and fail the build on a >1% drop. The baseline is raised manually on green `main` only — never on a feature branch. Tracked alongside the testing foundation epic (#59).

## Currently non-blocking checks

| Check | Status | Why | Becomes blocking when |
|---|---|---|---|
| Playwright E2E | Placeholder no-op | `docker-compose.test.yml` and smoke specs not yet authored (see #83) | Issue #83 lands |
| `mypy` in grader | Not yet wired | Grader code not type-clean yet | Story TBD |
| LLM-fidelity nightly | Placeholder no-op | `respx` cassettes don't exist yet (see #81) | Issue #81 lands |
| Stress tests | Placeholder no-op | No `stress`-tagged tests yet (see #58) | Backend Robustness epic lands |
| Visual regression | Not wired | Design System V2 has not stabilized (see #57) | Issue #74 lands |

### Recently flipped to blocking

- **`ruff check` in grader** — was non-blocking through PR #89; flipped to blocking in #88 once the 6 F401 errors were fixed.

This list is the authoritative source for "checks that *will* be enforced but currently are not". Updating it is part of the PR that flips the corresponding gate to blocking.

## Related

- Issue #87 — this CI scaffolding.
- Issue #88 — ruff cleanup, flips the grader lint gate to blocking.
- Issue #86 — GraderClient persistent connection refactor; lets the goleak ignores in `engine/test/integration/grader_client_test.go` go away.
- Project memory: `feedback_full_validation_before_merge.md`.
