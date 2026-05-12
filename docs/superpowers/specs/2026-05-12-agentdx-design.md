# AgentDx Design Specification

**Project:** Frameval → AgentDx pivot
**Date:** 2026-05-12
**Status:** Draft, awaiting user review
**Owner:** Mustafa Selman Günes
**Target deadline:** 4 weeks (demo-ready framework)

---

## 0. TL;DR (Türkçe özet)

AgentDx, agentic coding harness'larını (spec-kit, Ralph loop, plain CLI, skill bundle, multi-agent setupları) ortak bir arayüzde plug-able hale getiren ve her birine **diagnostic profile** üreten bir framework. Mevcut Frameval kod tabanından ~%60'ı silinerek 4 hafta içinde demo-ready hale getirilecek; tez bulguları framework'ün kendi çıktılarıyla yazılacak.

Üç katkı:
1. **Metodoloji:** Kategorik failure taxonomy + structured classification (free-form LLM-as-judge'a alternatif)
2. **Sistem:** Harness × Executor × Task plug-able adapter layer
3. **Ampirik:** Popüler harness'ların ilk diagnostic karakterizasyonu (demo sonrası, framework kullanılarak)

Hedef kullanıcı kitleleri: (a) kendi harness'ını diagnose etmek isteyen practitioner; (b) yeni orchestration pattern öneren araştırmacı; (c) kendi agent CLI'sini karşılaştırmak isteyen tool builder.

---

## 1. Problem and Goal

### 1.1 Problem statement

The agentic software development community is converging on a small set of recurring tooling patterns — **harnesses** — that wrap coding LLMs to make them productive:

- Plain CLI agents (Aider, Cursor, Claude Code, Gemini)
- Instruction files (CLAUDE.md, AGENTS.md, .cursorrules)
- Workflow scaffolds (spec-kit, custom skill bundles, MCP servers)
- Orchestration patterns (Ralph loop, planner+coder, debate, Reflexion)

Practitioners argue about which harnesses work, often without rigorous evidence. Existing benchmarks (SWE-bench, Terminal-Bench, HumanEval) report a **single scalar score** — pass rate — which collapses meaningful differences:

- Two harnesses with equal pass rate may differ wildly in cost, behavior, and failure modes
- Pass-rate alone cannot answer *why* a harness fails or what failure mode it is vulnerable to
- LLM-as-Judge approaches that grade on subjective scales (1–10 maintainability) are uncalibrated and drift across runs

There is no widely-adopted **diagnostic framework** for agentic coding harnesses.

### 1.2 Goal

Build AgentDx: a framework that, given an agent run, produces a **multi-dimensional diagnostic profile** consisting of:

1. **Behavioral fingerprint** — 10 deterministic features extracted from the transcript (planning depth, tool diversity, self-validation rate, etc.)
2. **Failure mode classification** — categorical, multi-label, evidence-grounded labels from a fixed taxonomy
3. **Recovery analysis** — error event timeline and recovery latency/success metrics
4. **Comparative views** — side-by-side comparison of N runs on the same task

The framework must support:
- 5 built-in harnesses (`bare`, `claudemd`, `speckit`, `ralph`, `planner_coder`)
- 2 executors (`aider` for local Ollama; `cursor` for cloud agent mode)
- 3 stress-designed tasks (one greenfield-CLI, one brownfield-fix, one greenfield-API)
- Plug-in extension: users can add their own harness/task/executor without forking core code
- A demo-able comparison UI

### 1.3 Non-goals (explicit out-of-scope)

To stay within 4-week scope:

- **Not a benchmark suite.** Pass rate is one signal among many; AgentDx does not maintain a leaderboard.
- **Not a full statistical analysis package.** Mann-Whitney, Cohen's d, Krippendorff α, bootstrap CI are removed from prior Frameval; only mean ± std reporting in MVP.
- **Not multi-model in MVP.** Single agent model (Qwen2.5-Coder-7B for local, Cursor's default for cloud).
- **Not real-time multi-user.** Single-user local-first; no auth, no multi-tenancy.
- **Not a baseline library.** Earlier Frameval shipped pre-evaluated baselines for community-known configurations; this is removed. Users produce their own runs.
- **Not LLM-judge for quality scoring.** Quality dimensions (1–10 correctness, maintainability) are deprecated; only categorical failure classification remains.
- **Not spec-adherence grading.** Overlaps with LLM judge; removed.

---

## 2. Conceptual Model

### 2.1 The three pluggable axes

```
                    ┌─────────────────────────────┐
                    │  Task                       │  ← what to build
                    │  - prompt, workspace, tests │
                    └──────────────┬──────────────┘
                                   │
                    ┌──────────────▼──────────────┐
                    │  Harness                    │  ← how to scaffold the agent
                    │  - bare, claudemd, speckit, │
                    │    ralph, planner_coder     │
                    └──────────────┬──────────────┘
                                   │
                    ┌──────────────▼──────────────┐
                    │  Executor                   │  ← which agent CLI
                    │  - aider (local Ollama)     │
                    │  - cursor (cloud agent)     │
                    └──────────────┬──────────────┘
                                   │ produces
                    ┌──────────────▼──────────────┐
                    │  Transcript                 │  ← evidence
                    └──────────────┬──────────────┘
                                   │
                    ┌──────────────▼──────────────┐
                    │  AgentDx                    │  ← measurement
                    │  - Fingerprint              │
                    │  - Symptoms                 │
                    │  - Recovery                 │
                    │  - Failure classification   │
                    └──────────────┬──────────────┘
                                   │
                    ┌──────────────▼──────────────┐
                    │  Diagnostic Profile         │  ← output
                    └─────────────────────────────┘
```

A **run** is a tuple `(Task, Harness, Executor)` plus a replica index. A run produces a transcript; AgentDx maps transcripts → diagnostic profiles. The frontend compares profiles across runs.

### 2.2 Why three axes (not one)

Without this separation, you cannot answer questions like:
- "Does Ralph help equally for Aider+Qwen and Cursor+Claude?" — requires Executor axis
- "Does spec-kit help on greenfield tasks but hurt on brownfield?" — requires Task axis
- "Is ralph-over-bare better than planner-coder-over-bare?" — requires Harness axis

The three-axis design is the conceptual core. Everything else is implementation.

---

## 3. Architecture

### 3.1 Service layout (unchanged from Frameval)

```
frontend/   (Vite + React)  ── REST/WS ──► engine (Go) ── gRPC ──► grader (Python)
                                                │
                                                └─ Docker SDK ──► ephemeral sandboxes
```

Engine writes to SQLite (`frameval.db`). Grader reads only.

### 3.2 Public vs internal Go packages

To support framework adoption, the harness/executor/task/diagnostic interfaces move to public `pkg/`:

```
engine/
├── pkg/                         ← PUBLIC, importable by third parties
│   ├── harness/                 ← Harness interface
│   ├── executor/                ← AgentExecutor interface
│   ├── task/                    ← Task spec, loader
│   └── diagnostic/              ← Fingerprint, Symptoms, Recovery, FailureLabel types
└── internal/                    ← Implementation, not importable externally
    ├── api/                     ← HTTP handlers, WebSocket hub
    ├── experiment/              ← Orchestrator, queue
    ├── sandbox/                 ← Docker container manager
    ├── storage/                 ← SQLite repos
    └── builtin/
        ├── harness/             ← bare, claudemd, speckit, ralph, planner_coder
        ├── executor/            ← aider, cursor
        └── diagnostic/          ← fingerprint extractor, classifier client
```

Third-party harness authors import `engine/pkg/harness` only; they never touch `internal/`. This is the framework contract.

### 3.3 Run lifecycle

```
1. User submits run config: (task_id, harness_id, executor_id, replicas)
2. Orchestrator creates N replica run records, enqueues each
3. Worker picks up a run:
   a. Sandbox.Create(image) → container
   b. Mount workspace (writable), tests (read-only)
   c. Harness.Setup(workspace, task) → harness-specific files laid down
   d. Harness.Invoke(executor) → executor runs agent, transcript captured
   e. Harness.Teardown(workspace)
   f. Run eval.sh in sandbox → test results
   g. Sandbox.Destroy()
4. Orchestrator triggers AgentDx pipeline:
   a. Fingerprint.Extract(transcript) → 10-dim vector
   b. Symptoms.Extract(transcript, test_results, fs_diff) → symptom packet
   c. Recovery.Analyze(transcript) → recovery profile
   d. FailureClassifier.Classify(symptoms, task, transcript_tail) → labels via LLM
5. DiagnosticProfile stored, WebSocket event broadcast
6. Frontend updates Compare view
```

---

## 4. Components

### 4.1 Harness interface

```go
// engine/pkg/harness/harness.go
package harness

type Harness interface {
    Name() string
    Description() string

    // Setup prepares the workspace before agent invocation.
    // Returns a HarnessRun handle that carries harness-specific state.
    Setup(ctx context.Context, ws Workspace, task task.Task) (HarnessRun, error)

    // Invoke runs the agent. The harness controls how many times the executor
    // is called, with what prompts, and how transcripts are merged.
    Invoke(ctx context.Context, run HarnessRun, exec executor.AgentExecutor) (*Transcript, error)

    // Teardown cleans up workspace artifacts. Tests directory is never touched.
    Teardown(ctx context.Context, run HarnessRun) error
}

type Workspace struct {
    Path     string  // mounted into sandbox
    TestsDir string  // mounted read-only, not visible to agent
    GitRef   string  // for brownfield-git mode
}

type HarnessRun struct {
    HarnessName string
    Budget      Budget
    Metadata    map[string]any
}

type Budget struct {
    MaxIterations  int     // for Ralph
    MaxTokens      int
    MaxWallSeconds int
    StopOnSuccess  bool    // halt when tests pass
}
```

### 4.2 Built-in harness implementations

#### 4.2.1 `bare`

The baseline. No setup, single agent invocation with the raw task prompt.

```go
// engine/internal/builtin/harness/bare.go
func (h *Bare) Setup(...) (HarnessRun, error)    { return ..., nil }
func (h *Bare) Invoke(ctx, run, exec) (*Transcript, error) {
    return exec.Execute(ctx, ExecuteConfig{Prompt: run.Task.Prompt})
}
func (h *Bare) Teardown(...) error               { return nil }
```

Implementation time: ~1 day.

#### 4.2.2 `claudemd`

Lays down `CLAUDE.md` from the task's `harness_context/claudemd.md` (or user-provided), then runs bare invocation. Isolates the "wording" signal from other harness layers.

```go
func (h *ClaudeMd) Setup(ctx, ws, task) (HarnessRun, error) {
    src := filepath.Join(task.HarnessContextDir, "claudemd.md")
    dst := filepath.Join(ws.Path, "CLAUDE.md")
    return ..., os.WriteFile(dst, content, 0644)
}
```

Implementation: ~1 day.

#### 4.2.3 `speckit`

Revives the spec-kit workflow integration that exists in current Frameval (`.specify/` dir + catalog code). Pipeline:

1. `specify init . --ai cursor` (or `--ai claude` etc.)
2. Lay down constitution from `harness_context/constitution.md`
3. Sequentially invoke: `/speckit.specify` → `/speckit.plan` → `/speckit.tasks` → `/speckit.implement`
4. Each step transcript is captured, concatenated with stage markers

```go
func (h *SpecKit) Invoke(ctx, run, exec) (*Transcript, error) {
    stages := []string{"specify", "plan", "tasks", "implement"}
    transcripts := []SubTranscript{}
    for _, stage := range stages {
        t, err := exec.Execute(ctx, ExecuteConfig{
            Prompt: speckitPrompt(stage, run.Task),
            Stage:  stage,
        })
        if err != nil { /* partial-run handling */ }
        transcripts = append(transcripts, t)
    }
    return mergeWithStageMarkers(transcripts), nil
}
```

Implementation: ~3-4 days (revive + adapt).

#### 4.2.4 `ralph`

Loops bare invocation until budget exhausted or stop condition met. State (workspace files) persists between iterations.

```go
func (h *Ralph) Invoke(ctx, run, exec) (*Transcript, error) {
    iterations := []SubTranscript{}
    for i := 0; i < run.Budget.MaxIterations; i++ {
        t, _ := exec.Execute(ctx, ExecuteConfig{
            Prompt: ralphPrompt(run.Task, i, prevResult),
        })
        iterations = append(iterations, t)
        if run.Budget.StopOnSuccess && testsPassed(ws) {
            break
        }
        if !meaningfulProgress(iterations) { // no-progress detector
            break
        }
    }
    return mergeWithIterationMarkers(iterations), nil
}
```

Implementation: ~2 days. Key complexity: in-sandbox state persistence between iterations (sandbox manager must support "keep container alive").

#### 4.2.5 `planner_coder`

Two-role multi-agent pattern. First invocation produces a plan; second uses the plan plus the workspace to implement.

```go
func (h *PlannerCoder) Invoke(ctx, run, exec) (*Transcript, error) {
    planT, _ := exec.Execute(ctx, ExecuteConfig{
        Prompt: plannerPrompt(run.Task),
        Role:   "planner",
    })
    plan := extractPlanFromTranscript(planT)

    coderT, _ := exec.Execute(ctx, ExecuteConfig{
        Prompt: coderPrompt(run.Task, plan),
        Role:   "coder",
    })
    return mergeWithRoleMarkers(planT, coderT), nil
}
```

Implementation: ~2 days.

### 4.3 Executor interface

```go
// engine/pkg/executor/executor.go
package executor

type AgentExecutor interface {
    Name() string
    Execute(ctx context.Context, cfg ExecuteConfig) (*Transcript, error)
    ParseTranscript(raw []byte) (*Transcript, error)
}

type ExecuteConfig struct {
    Prompt    string
    Workspace string
    Stage     string  // optional: speckit stage name
    Role      string  // optional: multi-agent role
    Model     string
    Timeout   time.Duration
}
```

### 4.4 Built-in executors

#### 4.4.1 `aider`

Talks to local Ollama via Aider's OpenAI-compatible mode:

```go
cmd := exec.CommandContext(ctx, "aider",
    "--model", "openai/qwen2.5-coder:7b",
    "--openai-api-base", "http://host.docker.internal:11434/v1",
    "--openai-api-key", "ollama",
    "--no-stream",
    "--yes-always",
    "--message", cfg.Prompt,
)
```

Aider transcripts have a known JSON-line format; ParseTranscript handles it.

Implementation: ~1.5 days.

#### 4.4.2 `cursor`

Existing executor in current Frameval; refactor to use `pkg/executor` interface and Cursor's `agent` mode (auto). Cursor CLI uses cloud backend, so this executor requires `CURSOR_API_KEY`.

Implementation: ~1 day (refactor existing code).

### 4.5 Task format

```
tasks/
└── greenfield-cli-wordfreq/
    ├── task.yaml              ← prompt, category, complexity, harness_context refs
    ├── workspace/             ← initial files (empty for greenfield)
    │   └── .gitkeep
    ├── tests/                 ← hidden from agent — mounted read-only outside workspace
    │   ├── test_cli.py
    │   └── conftest.py
    ├── harness_context/       ← optional: per-harness context bundles
    │   ├── claudemd.md        ← used by claudemd harness
    │   └── constitution.md    ← used by speckit harness
    ├── setup.sh               ← runs in sandbox before agent: installs deps
    └── eval.sh                ← runs after agent: invokes test runner, exits with code
```

`task.yaml` schema:

```yaml
id: greenfield-cli-wordfreq
name: Build a wordfreq CLI tool
description: Scaffold a Python CLI that prints top-K most common words
category: greenfield
complexity_score: 4.5
codebase_type: python
workspace:
  mode: empty                 # or: git, local, tarball
  # git_url, git_ref, local_path as appropriate
prompt: |
  Build a Python CLI tool named `wordfreq` ...
technical_details: |
  Allowed dependencies: click. ...
expected_files_modified:        # for SCOPE_DRIFT detection in brownfield
  - "*"                          # greenfield: anything allowed
test_cases:                      # informational; eval.sh is the real grader
  - name: Basic top-K
    test_command: pytest tests/test_cli.py::test_basic
    expected_result: exit 0
```

### 4.6 The three MVP tasks

#### 4.6.1 `greenfield-cli-wordfreq`

**Workspace:** empty.

**Prompt summary:** Build a Python CLI `wordfreq` taking a file path, printing top-K most common words. Support `-k N` (default 10) and `-c` (case-sensitive). Use `click`. Output format: `WORD: COUNT` per line.

**Tests:**
- `test_basic_top_k` — counts and orders correctly
- `test_k_flag` — `-k 3` returns 3 entries
- `test_case_sensitive` — `-c` distinguishes case
- `test_missing_file` — exit 1 with error
- `test_output_format` — regex match on output lines

**Stresses:** HAL_API, DEP_MISS, STOP_EARLY, WRONG_ABS (argparse vs click).

#### 4.6.2 `brownfield-fix-async-race`

**Workspace:** small FastAPI app provided. `app/user_service.py` contains `async def add_credits(user_id, amount)` with a deliberate read-modify-write race condition.

**Prompt summary:** Fix the race condition in `add_credits` without changing the function signature, API contract, or modifying other files unless strictly required.

**Tests:**
- `test_race_fixed.py` — 100 concurrent `+1` requests result in final value 100
- `test_api_unchanged.py` — response schema and status code preserved
- `test_scope.sh` — `git diff --name-only HEAD` returns only `app/user_service.py`

**Stresses:** SCOPE_DRIFT, MISREAD (signature change), WRONG_ABS (sync lock instead of async).

#### 4.6.3 `greenfield-rate-limiter-fastapi`

**Workspace:** empty.

**Prompt summary:** Build a FastAPI service with `GET /api/data` returning `{"data": "ok"}`. Rate-limit to 10 req/min per client IP. Exceeded → 429 with `{"error": "rate limit exceeded"}`. After 60-sec window, allow again.

**Tests:**
- 10 sequential requests → 200
- 11th request → 429 + body match
- Wait 60s → 200 again
- Different IPs (`X-Forwarded-For`) have independent budgets

**Stresses:** HAL_API (wrong rate-limit lib), MISREAD (global vs per-IP), STOP_EARLY (skipped window logic).

### 4.7 AgentDx pipeline

#### 4.7.1 Fingerprint extractor (deterministic, no LLM)

```go
// engine/pkg/diagnostic/fingerprint.go
type Fingerprint struct {
    PlanningDepth         float64  // pre-action intent turns / total turns
    ToolCallDiversity     float64  // Shannon entropy of tool distribution
    SelfValidationRate    float64  // test/build/read-back actions / code-producing turns
    BacktrackRate         float64  // file reverts + "let me try again" patterns
    FileFocus             float64  // unique files / total file ops
    RecoveryLatency       float64  // mean turns from error → corrective action
    PrematureCompletion   float64  // declared-done while tests failing rate
    TurnEfficiency        float64  // state-changing tool calls / total turns
    ContextReferenceRate  float64  // turns citing CLAUDE.md / instructions
    IdleThinkingRatio     float64  // text-only turns / total turns
}
```

Each metric: pure transcript parsing + regex + counters. Golden-file tests with handcrafted transcripts verify each metric in isolation.

#### 4.7.2 Symptom extractor (deterministic)

Compresses transcript + test results + fs diff into a ~500-1000 token packet for the LLM classifier:

```go
type Symptoms struct {
    TestsPassed, TestsFailed, TestsTotal int
    CompileFailed                         bool
    LintErrors                            []string
    ToolFailures                          []ToolFailure
    LastErrorMessage                      string
    FilesTouched, FilesCreated, FilesDeleted []string
    LastAssistantClaim                    string  // last 500 chars
    DeclaredCompletion                    bool
    AcknowledgedFailure                   bool
    TimeToFirstError                      int     // turn index
    TimeoutHit                            bool
    WallClockSeconds                      float64
    UnexpectedFilesModified               []string  // brownfield SCOPE_DRIFT signal
}
```

#### 4.7.3 Recovery analyzer (deterministic)

```go
type RecoveryProfile struct {
    ErrorEvents              []ErrorEvent  // (turn_index, error_type, tool_name)
    ErrorAcknowledgmentRate  float64
    CorrectionLatencyMean    float64
    CorrectionSuccessRate    float64
    SilentSkipCount          int
}
```

Walks transcript, finds error events, traces what happens in subsequent turns.

#### 4.7.4 Failure classifier (LLM via Anthropic Haiku + `instructor`)

```python
# grader/failure_classifier/grader.py
from enum import Enum
from pydantic import BaseModel, Field
import instructor
from anthropic import Anthropic

class FailureCode(str, Enum):
    NONE = "NONE"
    HAL_API = "HAL_API"           # Hallucinated API
    HAL_FILE = "HAL_FILE"         # Phantom file
    DEP_MISS = "DEP_MISS"         # Missing dependency
    STOP_EARLY = "STOP_EARLY"     # Premature completion
    STOP_GIVEUP = "STOP_GIVEUP"   # Surrender
    LOOP_INF = "LOOP_INF"         # Infinite loop / no progress
    WRONG_ABS = "WRONG_ABS"       # Wrong abstraction
    MISREAD = "MISREAD"           # Spec misread
    ENV_ERR = "ENV_ERR"           # Environment failure
    SCOPE_DRIFT = "SCOPE_DRIFT"   # Modified unrelated files
    TIMEOUT = "TIMEOUT"           # Wall-clock hit
    SILENT_SKIP = "SILENT_SKIP"   # Ignored error

class EvidenceSpan(BaseModel):
    code: FailureCode
    quote: str = Field(description="Verbatim text from transcript")
    turn_index: int

class FailureClassification(BaseModel):
    primary: FailureCode
    secondary: list[FailureCode] = Field(default_factory=list, max_items=3)
    evidence: list[EvidenceSpan]
    confidence: float = Field(ge=0, le=1)
    rationale: str = Field(max_length=400)

class FailureClassifier:
    def __init__(self, model="claude-haiku-4-5-20251001"):
        self.client = instructor.from_anthropic(Anthropic())
        self.model = model

    def classify(self, symptoms, task_spec, transcript_tail) -> FailureClassification:
        return self.client.messages.create(
            model=self.model,
            max_tokens=1024,
            response_model=FailureClassification,
            max_retries=2,
            messages=[
                {"role": "system", "content": TAXONOMY_PROMPT},
                {"role": "user", "content": render(symptoms, task_spec, transcript_tail)},
            ],
        )
```

**Why Haiku for MVP:** reliable structured output, cheap (~$0.005/run), fast (~3 sec). Local Qwen-7B has higher JSON validation failure rate; deferred to post-demo comparison study.

#### 4.7.5 Calibration

100-run validation set:
- 5 harnesses × 3 tasks × ~7 replicas ≈ 105 runs (sampled down to a balanced 100 for labeling)
- User hand-labels each run with primary + secondary FailureCode (target: 2 hours, ~1.2 min/run)
- Per-category classifier accuracy reported: confusion matrix + macro-F1
- Stored in `docs/calibration/2026-06-validation.md` (date set at run-time) for thesis citation

**Pragmatic limitations acknowledged in thesis:**
- Single rater (no inter-rater Cohen's κ in MVP)
- Pilot-grade rigor; thesis writeup phase may extend
- Calibration data versioned and reproducible

### 4.8 Frontend: Diagnostic Compare view

Single canonical page replacing prior Frameval experiment/results views.

**Layout:**
```
┌─────────────────────────────────────────────────────────────────┐
│  [Task: greenfield-cli-wordfreq]   [Executor: aider]   [Run]    │
├─────────────────────────────────────────────────────────────────┤
│ Selected harnesses: ☑ bare  ☑ claudemd  ☑ speckit  ☑ ralph  ☑ planner_coder │
├─────────────────────────────────────────────────────────────────┤
│  ┌───────────────────┐  ┌───────────────────┐                   │
│  │ Behavioral Radar  │  │ Failure Breakdown │                   │
│  │ (5 overlaid)      │  │ (stacked bar)     │                   │
│  └───────────────────┘  └───────────────────┘                   │
│  ┌───────────────────────────────────────────┐                  │
│  │ Recovery Timeline (5 lanes, error events) │                  │
│  └───────────────────────────────────────────┘                  │
│  ┌───────────────────────────────────────────┐                  │
│  │ Pass rate × Cost scatter                  │                  │
│  └───────────────────────────────────────────┘                  │
│  ┌───────────────────────────────────────────┐                  │
│  │ Transcript Evidence (per-failure quotes)  │                  │
│  └───────────────────────────────────────────┘                  │
└─────────────────────────────────────────────────────────────────┘
```

**Components (Recharts where applicable):**

- `<BehavioralRadar />` — multi-series radar, hover shows per-dim values
- `<FailureBreakdown />` — stacked bar by category, hover shows evidence
- `<RecoveryTimeline />` — gantt-style; rows = harnesses, x-axis = turn, red/green markers
- `<CostQualityScatter />` — pass rate (y) vs tokens or wall-time (x), one point per harness
- `<TranscriptEvidence />` — per-failure-label quote with link to transcript line

Implementation: ~3-4 days.

### 4.9 CLI

Built post-demo (per user scope decision), but interface designed now for consistency:

```
agentdx run --config experiment.yaml
agentdx run --task ./tasks/X --harness bare,ralph --executor aider --replicas 5
agentdx report --experiment exp-123 --format html
agentdx list-harnesses
agentdx list-executors
agentdx new-task --kind brownfield-local --from-local ~/proj --name fix-X
agentdx validate-task ./tasks/X
```

`experiment.yaml`:
```yaml
task: ./tasks/greenfield-cli-wordfreq
executor: aider
harnesses: [bare, claudemd, speckit, ralph, planner_coder]
replicas: 5
budget:
  max_wall_seconds: 600
  max_tokens: 50000
output: ./reports/exp-001
```

### 4.10 Task authoring (pre-demo: docs only; CLI deferred)

**Pre-demo deliverable:** `docs/task-authoring.md` covering:

- File structure of a task
- 4 test recipes: "detect the fix", "no regression", "scope discipline", "behavior battery"
- 4 anti-patterns
- One worked brownfield example (`examples/02-evaluate-your-own-claudemd/`)

**Post-demo deliverable** (not in 4-week scope):
- `agentdx new-task` scaffolding CLI
- `agentdx validate-task` validation tool
- Brownfield import helper

---

## 5. Data Model

### 5.1 SQLite schema changes

Migrations applied on top of current Frameval schema:

```sql
-- 006_agentdx.sql

-- Diagnostic profile per run
CREATE TABLE diagnostic (
    id              TEXT PRIMARY KEY,
    run_id          TEXT NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
    fingerprint     TEXT NOT NULL,  -- JSON: Fingerprint
    symptoms        TEXT NOT NULL,  -- JSON: Symptoms
    recovery        TEXT NOT NULL,  -- JSON: RecoveryProfile
    failure_label   TEXT,            -- JSON: FailureClassification (nullable if classifier skipped)
    classifier_model TEXT,
    classifier_latency_ms INTEGER,
    created_at      TEXT NOT NULL
);

-- Harness reference (built-in or user-registered)
CREATE TABLE harness_registry (
    id              TEXT PRIMARY KEY,
    name            TEXT NOT NULL UNIQUE,
    kind            TEXT NOT NULL,    -- 'builtin' | 'external'
    description     TEXT,
    config_json     TEXT,
    created_at      TEXT NOT NULL
);

-- Calibration / validation labels (user-provided ground truth)
CREATE TABLE validation_label (
    id              TEXT PRIMARY KEY,
    run_id          TEXT NOT NULL REFERENCES runs(id),
    primary_label   TEXT NOT NULL,
    secondary_labels TEXT,           -- JSON array
    labeler         TEXT,             -- e.g., 'mustafa' or 'claude'
    notes           TEXT,
    labeled_at      TEXT NOT NULL
);
```

### 5.2 Migrations to drop

```sql
-- 007_drop_legacy.sql
DROP TABLE IF EXISTS baselines;
DROP TABLE IF EXISTS experiment_stats;
DROP TABLE IF EXISTS artifact_versions; -- keeping artifact for harness_context
-- Plus prior dimension/judge/spec_adherence columns on grade table
```

---

## 6. Code to delete from current Frameval

To make room for AgentDx and keep the codebase shrunk:

```
benchmarks/                                      (whole tree)
baselines/                                       (whole tree, except keep one .gitkeep)
engine/internal/benchmark/                       (whole package — uncommitted)
engine/internal/catalog/                         (whole package, parts moved to speckit harness)
engine/internal/storage/migrations/005_*.sql     (uncommitted; revert before next migration)
engine/internal/executor/gemini.go               (replaced by aider)
engine/internal/executor/api_mode.go             (out of scope)
grader/spec_adherence/                           (whole package)
grader/stats/engine.py                           (kept minimal: just mean ± std)
frontend/src/pages/baselines/                    (whole tree)
frontend/src/components/baselines/               (whole tree)
frontend/src/pages/experiments/results.tsx       (replaced by diagnostic compare)
frontend/src/components/experiment-wizard/       (simplified to harness selector)
frontend/src/components/results/heatmap.tsx
frontend/src/components/results/stats-table.tsx
frontend/src/components/results/comparison-table.tsx  (replaced by diagnostic components)
frontend/src/components/results/transcript-diff.tsx   (replaced by transcript-evidence)
frontend/src/components/artifacts/dimension-tags.tsx
frontend/src/components/artifacts/version-timeline.tsx
```

Total estimated removal: ~3000 LoC. Net codebase shrink after AgentDx additions: ~30-40%.

---

## 7. Implementation Plan (4 weeks)

Assumes full-time work. Adjust week boundaries if part-time.

### Week 1 — Slim down + foundation

**Days 1-2:**
- Delete everything in §6 above
- Drop unused migrations, run `006_agentdx.sql` migration
- Update `.env.example` to AgentDx-relevant vars only
- Move executor/harness/task/diagnostic types to `engine/pkg/`

**Days 3-4:**
- Implement `Harness` interface + `bare` + `claudemd` harnesses
- Refactor existing `cursor.go` to satisfy new `executor.AgentExecutor` interface
- Implement `aider.go` executor + Ollama integration test

**Day 5:**
- Build `greenfield-cli-wordfreq` task (workspace, tests, setup.sh, eval.sh)
- End-to-end smoke test: aider + bare + greenfield-cli-wordfreq produces a transcript

### Week 2 — Harnesses + tasks

**Days 1-2:**
- Implement `speckit` harness by reviving relevant catalog code
- Test 5-stage workflow end-to-end with aider

**Day 3:**
- Implement `ralph` harness (loop + budget + no-progress detector)
- Extend sandbox manager to support "keep container alive between iterations"

**Day 4:**
- Implement `planner_coder` harness (2-call sequential with role markers)

**Day 5:**
- Build `brownfield-fix-async-race` task
- Verify scope-discipline test works

### Week 3 — AgentDx core + 3rd task

**Days 1-2:**
- Implement `Fingerprint.Extract`, `Symptoms.Extract`, `RecoveryProfile.Analyze` (deterministic)
- Golden-file tests for each
- Wire into orchestrator: after each run, extract diagnostic

**Days 3-4:**
- Implement `FailureClassifier` in grader (Python + instructor + Haiku)
- Define gRPC `ClassifyFailure` RPC in `proto/grader.proto`
- Wire orchestrator to call classifier post-run

**Day 5:**
- Build `greenfield-rate-limiter-fastapi` task
- Run initial 30-50 trial runs to debug failures

### Week 4 — Frontend + calibration + framework polish

**Days 1-2:**
- Implement frontend Diagnostic Compare page + components
- WebSocket wiring for live updates

**Day 3:**
- Run ~100 calibration runs (5 harness × 3 task × ~7 replicas)
- User hand-labels (2 hours)
- Compute per-category accuracy
- Document in `docs/calibration/2026-XX-validation.md`

**Day 4:**
- Move public types to `pkg/`, write `docs/task-authoring.md`
- Write `examples/01-bare-vs-ralph/`, `examples/02-evaluate-your-own-claudemd/`
- Write top-level `README.md` with 5-minute start

**Day 5:**
- Demo dry-run: full flow on a clean machine
- Screen recording of demo
- Buffer for fixes

### Post-demo (out of 4-week scope)

- `agentdx new-task` + `agentdx validate-task` CLI scaffolding
- Local Qwen-7B classifier comparison with Haiku (thesis methodology study)
- Extended calibration (additional rater, Cohen's κ)
- Broader empirical study (5×3×N grid) for thesis Chapter 6
- Additional harness adapters (superpowers, Reflexion, etc.) as community contributions

---

## 8. Risks and Mitigations

| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| Aider transcript parsing breaks for some prompts | Medium | High | Golden-file tests; fallback raw-text parser |
| Ralph loop hangs / runs over budget | Medium | High | Hard timeout per iteration + total budget; no-progress detector |
| Spec-kit revival blocked by missing context | Low | High | Existing `.specify/` integration tested before sprint; deferred to Week 2 buffer |
| 100 hand-labels in 2 hours is unrealistic | High | Medium | Acknowledge as pilot rigor; thesis section flags this | 
| Qwen-7B too weak → all harnesses fail | Medium | Medium | Use as strength signal; framework still demonstrates differences |
| Sandbox state persistence (Ralph) hard on Docker | Medium | Medium | Already partially supported in current sandbox manager |
| Haiku classifier produces brittle structured output | Low | High | `instructor` retries + Pydantic validation; fallback to "UNCLASSIFIED" |
| Demo machine differs from dev machine | Medium | High | Docker compose ensures parity; pre-record demo as backup |
| User time-budget overrun (part-time work) | High | High | Aggressive cuts: drop planner_coder first, then claudemd, then one task |

---

## 9. Thesis Contribution Map

### 9.1 Three contributions

**C1 — Methodology:** Categorical failure taxonomy + structured-classification protocol replacing subjective LLM-as-judge scoring. Validated on N=100 hand-labeled runs.

**C2 — System:** Open-source framework with pluggable Harness × Executor × Task interfaces; 5 reference harness implementations covering current practice (bare, claudemd, speckit, ralph, planner_coder); 2 executors (local + cloud).

**C3 — Empirical:** First diagnostic characterization of popular harnesses on a stress-task suite. Findings produced *post-demo* using the framework itself; supports thesis writeup chapter.

### 9.2 Chapter outline

1. Introduction — agentic coding, harness ecosystem, evaluation gap
2. Related Work — SWE-bench-style benchmarks, LLM-as-judge, prompt optimization
3. Methodology (AgentDx) — fingerprint dimensions, taxonomy, classifier
4. System Architecture (Frameval/AgentDx) — three-axis design, plug-in API
5. Calibration Study — classifier validation, taxonomy κ (pilot)
6. Empirical Study — harness characterization, findings
7. Application & Adoption — framework as artifact, CLI, examples
8. Limitations & Future Work
9. Conclusion

### 9.3 Defensive structure

Each contribution stands alone:
- If C1 is questioned → demonstrate C2 (system value) + C3 (findings)
- If C2 is questioned → demonstrate C1 (methodology) + C3
- If C3 is questioned → demonstrate C1 + C2 + framework reusability

---

## 10. Adoption Story (framework users)

### 10.1 Three target user journeys

**Practitioner ("Does my CLAUDE.md actually help?"):**

```
$ git clone https://github.com/mustafaselman/agentdx
$ cd agentdx && docker compose up -d
$ cp my-claudemd-from-work.md examples/02-evaluate-your-own-claudemd/my-harness/CLAUDE.md
$ agentdx run --config examples/02-evaluate-your-own-claudemd/experiment.yaml
[output: my-claudemd vs bare vs ralph diagnostic comparison]
```

**Researcher ("My new harness vs baselines"):**

```
$ agentdx new-harness ./my-reflexion-mod   # post-demo
$ # write 100-200 lines implementing Harness interface
$ agentdx run --task ./tasks/X --harness ./my-reflexion-mod,bare,ralph,speckit
[output: comparable diagnostic profile, citable]
```

**Tool builder ("My CLI as executor"):**

```
$ agentdx new-executor ./my-openhands-exec   # post-demo
$ agentdx run --executor ./my-openhands-exec --harness speckit,ralph
[output: same-task same-harness, different executor; capability profile]
```

### 10.2 README pitch (sketch)

```markdown
# AgentDx

Diagnose your agentic coding harness. Stop guessing whether your
CLAUDE.md, your skill bundle, or your orchestration pattern is working —
get a diagnostic profile and compare against canonical baselines.

Built-in: bare, claudemd, speckit, ralph, planner_coder.
Plug your own. Open source. Local-first.

→ [5-minute quickstart]   → [Why diagnostic > scalar]   → [Bring your own harness]
(links populated once docs land in Week 4)
```

---

## 11. Open Questions

These do not block implementation but should be revisited during execution:

1. **Cursor agent mode integration:** Current `cursor.go` uses Cursor CLI's headless mode. "Auto" / "agent" mode requires investigation — may need updated CLI flags. Owner: Week 1 day 3.

2. **Aider transcript stability:** Aider's transcript JSON format has changed across versions. Pin a specific version in setup, document in README.

3. **Sandbox state persistence for Ralph:** Current `engine/internal/sandbox/manager.go` creates a container per run. Ralph needs container reuse across iterations within a single run. Confirm Docker SDK supports a clean "exec into existing container" pattern at the manager level.

4. **`instructor` Python version compatibility:** Newer Pydantic V2 may require pinning instructor version. Verify in Week 3 day 3.

5. **Test hiding mechanism:** Confirm sandbox bind-mount setup hides `tests/` from agent. Spot-check by inspecting workspace from inside container.

6. **Calibration labeling tool:** A minimal in-browser labeling UI may help reach 100 labels in 2 hours. If skipped, user labels via spreadsheet.

---

## 12. Definition of Done (4-week demo readiness)

The 4-week milestone is met when **all** of the following are true:

- [ ] Frameval slim-down complete; codebase passes `go build ./...` + `npm run build` + `pytest grader/tests`
- [ ] 5 harnesses each pass an integration smoke test with aider executor
- [ ] 3 tasks each have valid `eval.sh` and `setup.sh`; pristine workspace fails tests as expected
- [ ] AgentDx pipeline emits Fingerprint + Symptoms + Recovery + FailureClassification for any complete run
- [ ] Failure classifier achieves ≥60% per-category macro-F1 on the 100-run calibration set
- [ ] Frontend Diagnostic Compare view renders all 4 sub-views for any selection of 2-5 runs
- [ ] `examples/01-bare-vs-ralph/` runs end-to-end on a clean machine with `docker compose up`
- [ ] `docs/task-authoring.md` exists with 4 recipes and 4 anti-patterns
- [ ] README.md at repo root pitches AgentDx to a stranger
- [ ] Demo screen recording captured and ≤10 minutes long

Each checkbox is binary, verifiable, and reviewed before declaring done.

---

## 13. Out-of-scope future work (post-demo, thesis-time)

These are explicitly **not** done in 4 weeks but are pre-planned for thesis writeup:

- Extended calibration (additional rater, full Cohen's κ analysis)
- Local Qwen-7B classifier vs Haiku — methodological comparison study
- Broader empirical study (5×3×N grid, statistical reporting)
- `agentdx new-task` + `agentdx validate-task` CLI tools
- Brownfield import helper (`--from-git`, `--from-local`)
- Additional reference harnesses (Reflexion, debate, superpowers)
- SWE-bench Lite hand-picked external-validity slice (5-10 tasks)
- Continuous-integration support (regression detection for agentic systems)
- Multi-rater labeling UI

---

## 14. Appendix A — Failure Taxonomy (full definitions)

| Code | Category | Definition | Stress-task association | Typical evidence pattern |
|---|---|---|---|---|
| `NONE` | Success | Tests pass, no significant issues observed | — | `tests_passed == tests_total` |
| `HAL_API` | Hallucinated API | Used a function, method, parameter, or import that does not exist in the named library | wordfreq, ratelimiter | `AttributeError`, `ImportError`, `is not a function` |
| `HAL_FILE` | Phantom File | Referenced a file that wasn't created, or expected a file in a wrong location | greenfield tasks | `FileNotFoundError`, `no such file or directory` |
| `DEP_MISS` | Missing Dependency | Used a package without installing/importing it; missing `requirements.txt` entry | wordfreq, ratelimiter | `ModuleNotFoundError`, `pip install` not in setup |
| `STOP_EARLY` | Premature Completion | Declared task complete while tests are failing or build is broken | all tasks | "I've completed", "done" in last turn while tests fail |
| `STOP_GIVEUP` | Surrender | Declared inability to proceed without exhausting reasonable options | any | "I cannot", "unable to figure out" |
| `LOOP_INF` | Infinite Loop / No Progress | Same action repeated across iterations with no state change | ralph harness specifically | Same diff applied ≥3 times; same error recurring |
| `WRONG_ABS` | Wrong Abstraction | Solution structure does not match task type (e.g., function when class needed, sync when async required) | async-race, wordfreq | Sync lock for async; argparse instead of click |
| `MISREAD` | Spec Misread | Solution targets wrong requirement (changed wrong function, misinterpreted constraint) | async-race, ratelimiter | Modified function signature when "do not change signature" given |
| `ENV_ERR` | Environment Failure | Failure caused by sandbox or tool infrastructure, not agent | any | Docker timeout, network unavailable, disk full |
| `SCOPE_DRIFT` | Scope Drift | Agent modified files outside the expected scope | brownfield tasks specifically | `git diff` includes files outside `expected_files_modified` |
| `TIMEOUT` | Wall-Clock Timeout | Run exceeded `MaxWallSeconds` before completion | ralph + complex tasks | `run.duration_seconds == budget.max_wall_seconds` |
| `SILENT_SKIP` | Silent Failure | Encountered a tool error but ignored it in subsequent turns | any | Error in turn N's tool_result; turn N+1 makes no reference |

**Total: 12 failure categories** plus the `NONE` sentinel for successful runs (13 codes overall in the enum). A run can carry **multiple labels** (primary + up to 3 secondary). `NONE` is mutually exclusive with all others — enforced at the Pydantic validator level: if `primary == NONE`, `secondary` must be empty, and vice versa.

---

## 15. Appendix B — Behavioral Fingerprint (full definitions)

| Dimension | Range | Computation |
|---|---|---|
| `planning_depth` | [0, 1] | (turns containing intent/plan keywords with no tool calls) / total turns |
| `tool_call_diversity` | [0, log(N)] | Shannon entropy of tool call type distribution |
| `self_validation_rate` | [0, 1] | (turns running tests, builds, file re-reads) / (turns producing code changes) |
| `backtrack_rate` | [0, 1] | (file reverts + "let me try" patterns + undo operations) / total tool calls |
| `file_focus` | [0, 1] | 1 - (unique files touched / total file operations); higher = more focused |
| `recovery_latency` | turns | mean of (corrective_action_turn - error_turn) across error events |
| `premature_completion` | [0, 1] | binary if claimed done while tests failing, else 0 |
| `turn_efficiency` | [0, 1] | (state-changing tool calls) / total turns |
| `context_reference_rate` | [0, 1] | (turns explicitly referencing CLAUDE.md or instruction file content) / total turns |
| `idle_thinking_ratio` | [0, 1] | (turns producing text without tool calls) / total turns |

Each dimension is a Float64 stored in JSON. Aggregate fingerprint is the per-replica mean; std reported separately.

---

*End of specification. Ready for user review.*
