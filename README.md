# AgentDx (formerly Frameval)

**Diagnose your agentic coding harness.** Stop guessing whether your
CLAUDE.md, your spec-kit setup, or your orchestration pattern is working —
get a multi-dimensional diagnostic profile and compare against the canonical
harness baselines that ship with this repo.

AgentDx is a local-first framework for running agentic coding harnesses in
sandboxed environments and producing structured diagnostic profiles
(behavioral fingerprint + failure-mode classification + recovery timeline)
that benchmark scalar scores cannot. Built originally as a master's thesis
project; designed for open-source community extension.

> Status: 4-week MVP. Core framework + 5 built-in harnesses + 3 stress tasks
> are in. Orchestrator-side AgentDx wiring (auto-extract + persist on each
> completed run) and the calibration study are the active follow-ups.

---

## What it does

AgentDx separates three things benchmarks usually conflate:

| Axis | Meaning | Built-ins |
|---|---|---|
| **Task** | What to build | 3 stress tasks under `tasks/` |
| **Harness** | How to scaffold the agent | `bare`, `claudemd`, `speckit`, `ralph`, `planner_coder` |
| **Executor** | Which agent CLI | `aider` (local Ollama), `cursor` (Cursor cloud) |

A *run* is a `(task, harness, executor)` triple plus a replica index. After
the agent finishes, AgentDx computes a **Diagnostic Profile**:

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

## 5-minute quickstart

```bash
git clone https://github.com/sgunes16/Frameval.git
cd Frameval

# 1. Build the sandbox image once
docker build -t frameval-sandbox:local -f docker/sandbox/Dockerfile .

# 2. Bring up engine + grader + frontend
docker compose up --build

# 3. Open the UI
open http://localhost:5173

# 4. Compare diagnostics across runs (after at least one run completes)
open "http://localhost:5173/diagnostic/compare?runs=<run-id-1>,<run-id-2>"
```

If you want to drive everything locally (no Docker compose):

```bash
./scripts/dev-engine.sh        # Go engine + air hot reload
./scripts/dev-frontend.sh      # Vite + React
./scripts/dev-local.sh         # Convenience wrapper
```

## What's included

```
tasks/                                  Stress-task suite
  greenfield-cli-wordfreq/              CLI scaffold (stresses HAL_API, DEP_MISS, STOP_EARLY)
  brownfield-fix-async-race/            FastAPI race fix (stresses SCOPE_DRIFT, MISREAD, WRONG_ABS)
  greenfield-rate-limiter-fastapi/      Per-IP rate limiter (stresses HAL_API, MISREAD, STOP_EARLY)

engine/pkg/                             Public Go API (3rd parties plug in here)
  harness/      AgentDx Harness interface + Workspace, HarnessRun, Budget
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
If you use AgentDx in research, please cite:

```bibtex
@mastersthesis{gunes2026agentdx,
  author = {Günes, Mustafa Selman},
  title  = {AgentDx: A Diagnostic Framework for Agentic Coding Harnesses},
  school = {(institution TBD)},
  year   = {2026}
}
```

## License

MIT. See [LICENSE](LICENSE).
