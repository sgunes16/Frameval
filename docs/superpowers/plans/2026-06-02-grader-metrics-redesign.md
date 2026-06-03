# Grader/metric redesign — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax.

**Goal:** Replace fake process metrics with real ones computed in the engine from the structured transcript, add a harness-adherence diagnostic, implement spec-adherence (grader, LLM), fix the silently-disabled failure classifier, remove IRR, and assemble the composite in the engine.

**Architecture:** Hybrid — engine owns process metrics + harness-adherence + composite; grader owns code-test + LLM judge + spec-adherence.

**Spec:** `docs/superpowers/specs/2026-06-02-grader-metrics-redesign-design.md`

**Tech:** Go (engine), Python (grader), React/TS (frontend), SQLite.

---

## Task 1: opencode executor captures real tokens/cost

**Files:** `engine/internal/executor/opencode.go`; `engine/pkg/executor/executor.go` (RunResult); `engine/internal/experiment/orchestrator.go` (persist onto Transcript). Test: `engine/internal/executor/opencode_test.go`.

- [ ] **Step 1 (test first):** add a parser fixture with two `step_finish` events (`part.tokens.output`=101 and 50; final `part.tokens.input`=8140; `part.cost`=0.001 and 0.002) and assert the executor surfaces `TotalTokens == 8140 + 101 + 50` and `CostUSD == 0.003`.
- [ ] **Step 2:** add `TotalTokens int` + `CostUSD float64` to `executor.RunResult`.
- [ ] **Step 3:** in `opencode.go`, accumulate while parsing the stream: per `step_finish`, add `part.tokens.output` to a running `genTokens`, track the last `part.tokens.input` as `lastInput`, add `part.cost` to `costUSD`. Set `RunResult.TotalTokens = genTokens + lastInput`, `RunResult.CostUSD = costUSD`. Keep `step_finish` emitting no Inspector turn.
- [ ] **Step 4:** in `orchestrator.go` where the Transcript is built from the merged RunResult, set `Transcript.TotalTokens`/`CostUSD` from the result (replace the word-count-derived values).
- [ ] **Step 5:** run `go test ./internal/executor/`; commit `executor: capture real tokens/cost from opencode step_finish`.

---

## Task 2: engine `internal/metrics` — real process metrics

**Files:** Create `engine/internal/metrics/process.go` + `process_test.go`.

- [ ] **Step 1 (tests first):** table-driven cases over synthetic `[]executor.ParsedTurn`:
  - 3 tool-use turns, 1 with `Stage=="error"` → `ToolCallCount==3`, `ToolErrorRate≈0.33`.
  - a bash turn with command `pytest -q` → `RanValidation==true`.
  - two edits to `app/models.py` with an intervening read → `BacktrackCount==1`.
  - `TurnCount` counts assistant/tool turns, not empty/system.
- [ ] **Step 2:** implement `type ProcessMetrics struct { TurnCount, ToolCallCount int; ToolErrorRate float64; RanValidation bool; ValidationCount, BacktrackCount int }` and `func Process(turns []executor.ParsedTurn) ProcessMetrics`. Tool turns: those with a tool name/`ToolOutput`; error: `Stage=="error"` or output contains `<error>`/`exit 1..` ; validation commands: regex `pytest|go test|ruff|mypy|npm (run )?test|bash tests/`; backtrack: ≥2 edits to the same path in `FilesTouched`.
- [ ] **Step 3:** `go test ./internal/metrics/`; commit `metrics: real process metrics from parsed turns`.

---

## Task 3: engine `internal/metrics` — harness-adherence

**Files:** `engine/internal/metrics/adherence.go` + `adherence_test.go`.

- [ ] **Step 1 (tests first):**
  - `bare` → Score 1.0.
  - `speckit/canonical` timeline where `app/models.py` is edited BEFORE any `specs/**/spec.md` write → low score, failing check `spec_before_impl`.
  - `speckit/canonical` timeline where `specs/spec.md` written before the source edit AND stages in order → Score 1.0.
  - `multiagent` where an `architect`-role turn precedes the first coder edit → check passes.
  - `ralph` with ≥2 iteration markers → check passes.
- [ ] **Step 2:** implement `type Check struct { Name string; Passed bool }`, `type Adherence struct { Score float64; Checks []Check }`, and `func HarnessAdherence(harnessID string, turns []executor.ParsedTurn) Adherence`. Derive an artifact timeline from write/edit tool calls (path + TurnIndex). Per-harness checks per the spec; `Score = passed/len(checks)` (bare → 1.0, no checks). speckit id prefix match (`speckit` / variant name).
- [ ] **Step 3:** `go test ./internal/metrics/`; commit `metrics: harness-adherence (process-fidelity) scoring`.

---

## Task 4: grader — spec-adherence in, process + IRR out

**Files:** Create `grader/spec_adherence/__init__.py` + `grader.py` + test; modify `grader/server.py`; delete `grader/process_grader/`; remove IRR from `grader/llm_judge/grader.py`.

- [ ] **Step 1 (test first):** `grader/tests/test_spec_adherence.py` — with a recorded fixture (no live LLM, monkeypatch the client), `grade(task, diff)` returns `instruction_compliance` (0–1), `convention_adherence` (0–1), `constraint_violations` (list).
- [ ] **Step 2:** implement `spec_adherence/grader.py`: one instructor call reusing `llm_judge`'s client builder, prompt = task prompt + constraints + the diff/output, structured output model with the three fields.
- [ ] **Step 3:** in `server.py`: call spec_adherence, populate the existing `SpecAdherenceResult` proto; **remove** the `process_grade` import + call (engine owns process now — `ProcessGradeResult` left empty/zero); **remove** `irr_alpha` population.
- [ ] **Step 4:** delete `grader/process_grader/` + its references; remove the 3 hardcoded `irr_alpha` lines.
- [ ] **Step 5:** `cd grader && uv run pytest`; commit `grader: real spec-adherence; remove placeholder process grader + IRR`.

---

## Task 5: models + migration — new Grade fields

**Files:** `engine/internal/models/grade.go` (or wherever Grade is); new `engine/internal/storage/migrations/022_grade_real_metrics.sql`; `engine/internal/storage/grade_repo.go` (insert/scan).

- [ ] **Step 1:** migration ADDs columns: `tool_call_count INTEGER DEFAULT 0`, `tool_error_rate REAL DEFAULT 0`, `ran_validation INTEGER DEFAULT 0`, `harness_adherence_score REAL DEFAULT 0`, `harness_adherence_json TEXT`.
- [ ] **Step 2:** add the matching fields to the `Grade` model; update the grade insert + scan in `grade_repo.go` to persist/read them. Leave the dropped columns (`idle_turns`, `token_efficiency`, `context_utilization`, `premature_completion`, `judge_irr_alpha`) in the schema but stop writing them (write 0/NULL).
- [ ] **Step 3:** `go test ./internal/storage/`; commit `storage: Grade columns for real process + harness-adherence`.

---

## Task 6: engine wiring — assemble process + adherence + composite

**Files:** `engine/internal/experiment/grader_client.go` (GradeRun mapping), `engine/internal/experiment/orchestrator.go` (recomputeCompositeScore + where Grade is built); retire `grader/composite.py` usage.

- [ ] **Step 1 (test first):** unit-test the new composite blend (`code*0.3+judge*0.3+process*0.2+spec*0.2`, process_score from the spec's formula, harness-adherence excluded, judge_unavailable dims excluded from the judge mean).
- [ ] **Step 2:** in `GradeRun`, after receiving the grader response (code/judge/spec-adherence), call `metrics.Process(transcript.ParsedTurns)` + `metrics.HarnessAdherence(harnessID, turns)`, populate the Grade's process + harness-adherence fields, and compute composite via the new blend (move the formula into a single Go helper used by both GradeRun and `recomputeCompositeScore`).
- [ ] **Step 3:** update `recomputeCompositeScore` to the same helper; ensure spec-adherence (`instruction_compliance`) feeds the composite's adherence term.
- [ ] **Step 4:** `go test ./internal/experiment/`; commit `engine: assemble real process + harness-adherence + composite (hybrid)`.

---

## Task 7: failure classifier — fix gate + use configured judge

**Files:** `engine/internal/experiment/orchestrator.go` (`persistDiagnostic`); maybe `grader_client.go` (ClassifyFailure signature to accept judge config).

- [ ] **Step 1 (test first):** a unit test that `persistDiagnostic`'s classifier-enable decision follows the resolved `judge.enabled` (app_settings → env fallback), not the raw env only. (Extract the enable-decision into a testable helper `classifierEnabled(ctx, store) bool`.)
- [ ] **Step 2:** replace `os.Getenv("FRAMEVAL_ENABLE_LLM_JUDGE")=="true"` with the resolved judge-enabled check (reuse the same precedence as `buildJudgeConfig`: `GetSetting("judge.enabled")` → env). Drive `ClassifyFailure` with the configured judge provider/model/api-key (pass the assembled `JudgeConfig`) instead of the hardcoded `"claude-haiku-4-5"`.
- [ ] **Step 3:** `go test ./internal/experiment/`; commit `diagnostic: enable failure classifier via judge.enabled + configured model`.

---

## Task 8: frontend — grading page

**Files:** `frontend/src/pages/runs/grading.tsx`; `frontend/src/lib/types.ts` (Grade type).

- [ ] **Step 1:** extend the Grade type with the new fields; remove the dropped ones from display.
- [ ] **Step 2:** Process section shows: turns, tokens, cost, tool calls + error rate, ran-validation, backtracks. Remove idle_turns / token_efficiency / context_utilization / premature_completion / inter-rater α.
- [ ] **Step 3:** add a **Harness adherence** section: the 0–1 score + the per-check pass/fail list (from `harness_adherence_json`). Spec-adherence section now shows real values.
- [ ] **Step 4:** `cd frontend && npm run lint && npm run build && npm test -- --run`; commit `frontend: grading page — real metrics + harness-adherence section`.

---

## Task 9: end-to-end verification
- [ ] Restart engine + grader; run one experiment (a brownfield task, judge enabled). Confirm: real tokens/cost on the grade; process metrics populated (no 0.75/0.8 constants); harness-adherence score + checks present; spec-adherence non-zero; failure_label populated (classifier ran); IRR + dropped metrics gone from the UI.

## Self-review
- Composite formula is concrete (spec §5); harness-adherence excluded — consistent across Task 6.
- Token summation defined (Task 1 Step 3) to avoid context double-count.
- Migration only ADDs columns (Task 5) — never edits existing.
- Classifier fix (Task 7) reuses the judge config — no new key plumbing.
