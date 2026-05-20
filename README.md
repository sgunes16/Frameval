# Frameval

**Diagnose your agentic coding harness.** Stop guessing whether your
CLAUDE.md, your spec-kit setup, or your orchestration pattern is working —
get a multi-dimensional diagnostic profile and compare against the canonical
harness baselines that ship with this repo.

Frameval is a local-first framework for running agentic coding harnesses in
sandboxed environments and producing structured diagnostic profiles
(behavioral fingerprint + failure-mode classification + recovery timeline)
that benchmark scalar scores cannot. Built originally as a master's thesis
project; designed for open-source community extension.

> Status: 4-week MVP. Core framework + 5 built-in harnesses + 3 stress tasks
> are in. Orchestrator-side Frameval wiring (auto-extract + persist on each
> completed run) and the calibration study are the active follow-ups.

---

## What it does

Frameval separates three things benchmarks usually conflate:

| Axis | Meaning | Built-ins |
|---|---|---|
| **Task** | What to build | 3 stress tasks under `tasks/` |
| **Harness** | How to scaffold the agent | `bare`, `claudemd`, `speckit`, `ralph`, `planner_coder` |
| **Executor** | Which agent CLI | `aider` (local Ollama), `cursor` (Cursor cloud) |

A *run* is a `(task, harness, executor)` triple plus a replica index. After
the agent finishes, Frameval computes a **Diagnostic Profile**:

1. **Behavioral fingerprint** — 10 deterministic features (planning depth,
   tool diversity, self-validation, backtrack rate, file focus, recovery
   latency, premature completion, turn efficiency, context-reference rate,
   idle thinking).
2. **Symptoms** — compact deterministic packet (test counts, tool failures,
   timing, file deltas, last-claim quote).
3. **Recovery profile** — error event timeline + acknowledgment / correction
   / silent-skip metrics.
4. **Failure classification** — LLM-tagged (Anthropic Haiku 4.5 via
   `instructor`) multi-label categorical verdict from a 12-category
   taxonomy with evidence quotes.

The first three stages are deterministic; only the last calls an LLM, and
that one returns a typed sentinel on hard failure rather than crashing the
pipeline.

## Prerequisites

You will need:

| Tool | Minimum | Why |
|---|---|---|
| **Docker Desktop** | running | The sandbox image (`frameval-sandbox:local`) runs every agent step inside a throwaway container. |
| **Go** | 1.22+ | Engine (`engine/`). `brew install go` or [go.dev/dl](https://go.dev/dl/). |
| **Node** | 20+ | Frontend (`frontend/`). `brew install node` or [nodejs.org](https://nodejs.org/). |
| **Python** | 3.11+ | Grader sidecar (`grader/`). `brew install python@3.11`. |
| **uv** | latest | Python package manager. `curl -LsSf https://astral.sh/uv/install.sh \| sh`. |
| **Ollama** | recent | Only if you want to run **local** models. `brew install ollama && ollama serve`. |

> **macOS note:** If you're behind a VPN or have "Use system proxy" enabled in Docker Desktop, `make sandbox` will fail with `ports.ubuntu.com` timeouts. The Makefile passes `--build-arg http_proxy=` to bypass it, but if you still hit issues, turn the Docker proxy off (Settings → Resources → Proxies → "No proxy") and rebuild.

The default executor (**opencode**) and its bundled cloud models work **without any API keys**. Local Ollama models also work without keys. Cloud agents (Claude, OpenAI, Cursor) need their respective keys — set them in `.env` at the repo root.

## 5-minute quickstart

```bash
git clone https://github.com/sgunes16/Frameval.git
cd Frameval

# 1. Build the sandbox image once.
#    Ships opencode + cursor-agent + aider CLIs and the Python
#    test runners (pytest, pytest-asyncio, ruff, mypy). Re-run
#    whenever docker/sandbox/Dockerfile changes — the dev scripts
#    do NOT auto-rebuild it.
make sandbox

# 2. (Optional) Pull at least one Ollama model if you want a
#    local run, otherwise the opencode free cloud models are fine.
ollama pull qwen2.5-coder:7b

# 3. Start everything: grader (Python) → engine (Go) → frontend (Vite).
#    Tails all three logs into the same terminal; Ctrl-C stops the lot.
make dev-full

# 4. Open the UI.
open http://localhost:5173
```

Then click **"New diagnostic run"**, pick a task + at least one harness
+ a model, and hit Launch. The launcher creates one experiment per
`(harness × executor × model)` cell and redirects you straight to the
Compare page where the metrics show up side-by-side as runs complete.

### Day-to-day commands

```bash
make sandbox             # rebuild the sandbox image (after Dockerfile edits)
make dev-full            # grader + engine + frontend, all in one shell
make stop                # kill anything still bound to :5173 / :8080 / :50051
make dev-grader          # run only the Python grader on :50051
make dev-engine          # run only the Go engine (no Air)
make dev-frontend        # run only the Vite dev server

make test                # unit tests across engine + grader + frontend
make test-engine-integration   # Go integration tests (FakeGrader, no Docker)
make lint                # `go vet` + `ruff check` + `npm run lint`
make build               # build all three services (mirror of CI build steps)
```

### When something breaks

- **`make sandbox` times out on `ports.ubuntu.com`** → Docker Desktop proxy is on. See the macOS note above.
- **Run fails with `sh: opencode: not found`** → the sandbox image is stale. Re-run `make sandbox`.
- **Grader times out / nothing in `:50051` logs** → grader sidecar didn't start. Check `make dev-grader` separately.
- **Test results show `tests/test_*.py not found`** → the task's `tests/` folder isn't being materialized. `make dev-full` restart usually clears it.

## What's included

```
tasks/                                  Stress-task suite
  greenfield-cli-wordfreq/              CLI scaffold (stresses HAL_API, DEP_MISS, STOP_EARLY)
  brownfield-fix-async-race/            FastAPI race fix (stresses SCOPE_DRIFT, MISREAD, WRONG_ABS)
  greenfield-rate-limiter-fastapi/      Per-IP rate limiter (stresses HAL_API, MISREAD, STOP_EARLY)

engine/pkg/                             Public Go API (3rd parties plug in here)
  harness/      Frameval Harness interface + Workspace, HarnessRun, Budget
  executor/     AgentExecutor interface + RunConfig, RunResult, ParsedTurn, Stage, Role
  task/         Task + TestCase types (matches task.yaml)
  diagnostic/   Fingerprint, Symptoms, RecoveryProfile, FailureCode, FailureClassification

engine/internal/builtin/harness/        5 reference Harness implementations
grader/failure_classifier/              Python LLM classifier (Pydantic + instructor + Haiku)
frontend/src/pages/diagnostic/          Diagnostic Compare UI

examples/                               Runnable starter configurations
  01-bare-vs-ralph/                     A/B Ralph loop vs single-shot
  02-evaluate-your-own-claudemd/        Drop your CLAUDE.md in, see how it performs
```

## Three user journeys

### Practitioner — "Does my CLAUDE.md actually help?"

You're a working engineer with your own CLAUDE.md. You want to know if it
moves the needle vs `bare`.

```bash
cp ~/your-project/CLAUDE.md examples/02-evaluate-your-own-claudemd/my-claudemd/
# Edit examples/02-evaluate-your-own-claudemd/experiment.yaml to point at a
# task in tasks/ (or one you author following docs/task-authoring.md).
# Run it through the UI or via the REST API; see Diagnostic Compare.
```

See [examples/02-evaluate-your-own-claudemd/README.md](examples/02-evaluate-your-own-claudemd/README.md).

### Researcher — "My new harness vs the baselines"

You've designed a new orchestration pattern (Reflexion variant, debate,
custom skill bundle). You want to characterize its diagnostic profile vs the
5 built-ins.

```go
// my_harness/harness.go in your own module
package myharness

import (
    "github.com/mustafaselman/frameval/engine/pkg/executor"
    "github.com/mustafaselman/frameval/engine/pkg/harness"
    "github.com/mustafaselman/frameval/engine/pkg/task"
)

type MyHarness struct{}

func (h *MyHarness) Name() string        { return "my-harness" }
func (h *MyHarness) Description() string { return "Your description" }
func (h *MyHarness) Setup(ctx, ws, t, b) (harness.HarnessRun, error) { /* ... */ }
func (h *MyHarness) Invoke(ctx, run, exec) (*executor.RunResult, error) { /* ... */ }
func (h *MyHarness) Teardown(ctx, run) error { return nil }
```

See `examples/test-external-harness/main.go` for a working no-op
implementation that compiles against the public `pkg/`.

### Tool builder — "My agent CLI as an executor"

You ship an agent CLI (Aider fork, Cursor competitor, your custom orchestrator)
and want users to benchmark with it. Implement `pkg/executor.AgentExecutor`
and register it.

## Methodology

The diagnostic methodology, fingerprint formulas, and failure taxonomy are
documented in the spec at
[docs/superpowers/specs/2026-05-12-agentdx-design.md](docs/superpowers/specs/2026-05-12-agentdx-design.md).
Calibration study (classifier accuracy on hand-labeled validation set) lands
in `docs/calibration/` once the validation run completes.

## Authoring your own task

See [docs/task-authoring.md](docs/task-authoring.md) — 4 test recipes,
4 anti-patterns, brownfield vs greenfield conventions.

## Citation

This project began as a master's thesis on agentic coding harness diagnostics.
If you use Frameval in research, please cite:

```bibtex
@mastersthesis{gunes2026agentdx,
  author = {Günes, Mustafa Selman},
  title  = {Frameval: A Diagnostic Framework for Agentic Coding Harnesses},
  school = {(institution TBD)},
  year   = {2026}
}
```

## License

MIT. See [LICENSE](LICENSE).
