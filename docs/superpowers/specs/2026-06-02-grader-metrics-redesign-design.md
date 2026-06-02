# Grader/metric layer redesign — real process metrics + harness-adherence

**Date:** 2026-06-02
**Status:** Design — pending review

## Problem

An audit of the grading layer found the process-metric layer is almost entirely placeholder, computed by string-grepping the raw opencode JSON (`grader/process_grader/grader.py`):

- `turn_count` = number of JSON lines; `total_tokens` = word count of the JSON; `cost_usd` = words / 100000 — none reflect real values, even though opencode `step_finish` events carry real `part.tokens.{total,input,output,reasoning}` + `part.cost`.
- `tool_call_accuracy` = **hardcoded `0.75`**; `context_utilization` = **hardcoded `0.8`** (`0.4` if few tokens).
- `idle_turns` = count of lines with `<3` words; `token_efficiency` = `1000/tokens`; `backtrack_count`/`self_validation_rate` = naive `"revert"`/`"test"` substring greps; `error_recovery_count` = duplicate of `backtrack_count`.
- `irr_alpha` = **hardcoded `0.0`** (3 sites in `grader/llm_judge/grader.py`).
- Spec-adherence: the `spec_adherence` module does not exist and `grader/server.py` never computes it → `instruction_compliance`/`convention_adherence` are always `0.00`.

Meanwhile the engine already parses opencode events into structured `ParsedTurns` (role, tool calls, files touched, stage) and knows the harness + its stages — none of which the grader uses. And there is no metric for whether the agent actually **followed the harness's methodology** (e.g. spec-kit: two different models both shortcut `/speckit.specify` and implemented directly, never producing `spec.md` — invisible to current metrics).

## Goal

Replace the fake process metrics with **real** ones computed from the structured transcript, **add a harness-adherence (process-fidelity) metric**, implement **spec-adherence** for real, and **remove** the meaningless/hardcoded metrics and IRR.

## Locked decisions

1. **Metric set:** focused real set + harness-adherence; **drop** `idle_turns`, `token_efficiency`, `context_utilization`, `premature_completion`.
2. **Spec-adherence:** implement for real (LLM, grader-side). **IRR:** remove.
3. **Architecture:** hybrid — engine computes process + harness-adherence (deterministic, from `ParsedTurns` + `step_finish` + harness/stage knowledge) and assembles the composite; grader does code-test + LLM judge + spec-adherence.
4. **Harness-adherence is a separate diagnostic** — NOT folded into the composite (composite = output quality; adherence = methodology-fidelity). Harnesses are compared on both.

## Architecture (hybrid)

```
GradeRun(transcript, task, artifact, judge_config)
  → grader (Python): code-test result + LLM judge + spec-adherence(LLM)   [process_grader + IRR removed]
  → engine: + process metrics (from ParsedTurns + step_finish)
            + harness-adherence (from harness id + stage timeline + artifact order)
            + composite (assembled here)
            → persist Grade
```

## Components

### 1. opencode executor — capture real tokens/cost (`engine/internal/executor/opencode.go`)
Today the `case "step_start", "step_finish"` arm returns `nil`, dropping `part.tokens`/`part.cost`. Change it to accumulate, per run:
- `total_tokens` = Σ per-step `part.tokens.output` (generation) + the final step's `part.tokens.input` (context) — summing per-step `total`/`input` double-counts the growing context, so we sum *output* and add the last *input* once.
- `cost_usd` = Σ per-step `part.cost` (already per-step, safe to sum; `0` for free models).
Surface these on `executor.RunResult` (new `TotalTokens int`, `CostUSD float64` fields) and have the orchestrator persist them onto `Transcript.TotalTokens`/`CostUSD` (columns already exist). step_finish still emits no Inspector turn.

### 2. engine `internal/metrics` package (new) — deterministic process + adherence
`func Process(turns []executor.ParsedTurn) ProcessMetrics` returns:
- `TurnCount` — count of real agent turns (assistant + tool-use turns; not JSON lines).
- `ToolCallCount` — number of tool-use turns; `ToolErrorRate` — fraction whose `Stage=="error"` or whose tool output carries an `<error>`/non-zero exit.
- `RanValidation bool` / `ValidationCount` — bash tool calls whose command matches a test/lint runner (`pytest`, `go test`, `ruff`, `mypy`, `npm test`, `bash tests/…`).
- `BacktrackCount` — same file edited, then a later edit reverts/re-edits it (≥2 edits to one path with an intervening read of a failing test), derived from `FilesTouched` + tool sequence.

`func HarnessAdherence(harnessID string, turns []executor.ParsedTurn, artifactOrder ArtifactTimeline) Adherence` returns `{Score float64 /*0..1*/, Checks []Check{Name, Passed}}`:
- **bare** → 1.0 (no methodology).
- **agent_instructions** → CLAUDE.md present and (optionally) read.
- **multiagent** → an architect/planner turn precedes the first coder source edit.
- **ralph** → ≥2 iterations observed and a stop-on-success check ran.
- **speckit/<ext>** → ordered checks: (a) the SDD artifact (`specs/**/spec.md`, or the extension's first artifact) was written **before** the first `app/**` source edit; (b) stages ran in catalog order (from `ParsedTurn.Stage`); (c) the extension's commands drove ≥1 artifact. Score = passed / total.

`ArtifactTimeline` is derived from the transcript's write/edit tool calls (path + turn index), so "spec.md before app/models.py" is decidable without filesystem snapshots.

### 3. grader — spec-adherence in, process + IRR out (`grader/`)
- **Remove** `grader/process_grader/` and its `server.py` call; the engine now owns process metrics.
- **Remove** the hardcoded `irr_alpha` (drop from the judge result + proto consumers; UI stops showing it).
- **Add** `grader/spec_adherence/grader.py`: one LLM call (reusing `llm_judge` client + instructor) that, given the task prompt/constraints + the diff (`filesystem_diff`) + output files, returns `instruction_compliance` (0–1), `convention_adherence` (0–1), and `constraint_violations` (list). `server.py` calls it and populates the existing `SpecAdherenceResult` proto (no proto change needed).

### 4. composite — engine-assembled (`engine` + retire `grader/composite.py`)
The engine assembles the composite after merging grader results with its own process metrics:
`composite = code*0.3 + judge*0.3 + process*0.2 + spec_adherence*0.2` (all terms on a 0–10 scale).
`process_score` (0–10) is the real-metric blend
`process_score = (ranValidation?1:0)*0.4 + (1 - toolErrorRate)*0.4 + tokenEfficiency*0.2`, all ×10, where
`tokenEfficiency = clamp(0,1, targetTokens / max(totalTokens, targetTokens))` with `targetTokens` a per-task constant (default 20000) — a real, monotonic efficiency signal rather than the old arbitrary `1000/tokens`.
`code = testPassRate*10`; `judge` = mean of available judge dims (preserving the `judge_unavailable` exclusion); `spec_adherence = instructionCompliance*10`.
**Harness-adherence is stored but excluded from the composite.** This blend lives in the engine (`recomputeCompositeScore` in `orchestrator.go` + the GradeRun assembly in `grader_client.go`); `grader/composite.py` is retired (the grader no longer assembles a composite).

### 5. models / migration (`engine/internal/models` + a new migration)
- Add Grade fields/columns: `tool_call_count`, `tool_error_rate`, `ran_validation`, `harness_adherence_score`, `harness_adherence_json` (the checks).
- Stop populating `idle_turns`, `token_efficiency`, `context_utilization`, `premature_completion`, `judge_irr_alpha` (leave columns for back-compat; new migration only ADDs the new columns — never edits existing ones).

### 6. frontend (`frontend/src/pages/runs/grading.tsx` + types)
- Process section: show the real metrics; **remove** idle_turns / token_efficiency / context_utilization / premature_completion / inter-rater α from the display.
- Add a **Harness adherence** section: the 0–1 score + the per-check pass/fail list (the diagnostic the thesis cares about).
- Spec-adherence section now shows real values.

## Data flow (after)
```
run finishes → Transcript{ParsedTurns, TotalTokens, CostUSD}
   → engine.GradeRun: grader returns {code, judge, spec_adherence}
   → engine: metrics.Process(turns) + metrics.HarnessAdherence(harnessID, turns, artifactOrder)
   → engine: composite = f(code, judge, process, spec_adherence)   [adherence excluded]
   → persist Grade {real process, harness_adherence(score+checks), spec_adherence, composite}
```

## Testing
- **engine `internal/metrics`:** table-driven unit tests from synthetic `ParsedTurns` — each process metric; harness-adherence per harness (a spec-kit timeline that shortcuts → low score; one that creates spec.md before editing → high score).
- **opencode executor:** parse a fixture stream with `step_finish` events → asserts real `TotalTokens`/`CostUSD`.
- **grader:** `spec_adherence` test with a recorded fixture (no live LLM); assert `process_grader` + `irr_alpha` are gone.
- **engine composite:** unit test of the new blend + adherence-excluded; judge_unavailable exclusion preserved.
- **frontend:** grading page renders the new sections; removed metrics absent.

## Out of scope
- Multi-round judge / real IRR (removed, not implemented).
- New harnesses beyond the existing five + speckit extensions.
- Re-grading historical runs (new metrics apply to new runs; a `/regrade` pass can backfill later).

## Risks
| Risk | Mitigation |
|---|---|
| Token summation double-counts context | Sum per-step `output` + final `input` once (defined above); unit-tested against a real fixture. |
| Harness-adherence false signals on a model that legitimately varies order | Score is a fraction with per-check transparency (the `Checks` list), not a hard pass/fail; surfaced as diagnostic, not gating. |
| Moving composite to the engine diverges from grader/composite.py | Retire grader/composite.py in the same change; one composite implementation (engine). |
| Dropping columns | Migration only ADDs columns; dropped metrics stop being populated, not schema-removed. |
