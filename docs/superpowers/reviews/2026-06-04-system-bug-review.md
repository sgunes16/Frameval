# Frameval / AgentDx — System Bug Review

**Date:** 2026-06-04
**Scope:** Functionality bugs (correctness, logic, wiring, data integrity, metrics fidelity, dead/contradictory code). Security explicitly out of scope.
**Method:** 8-dimension adversarial review + completeness-critic pass, every finding re-verified against the real code, the live `engine/frameval.db`, and (where relevant) reproduced. Each finding cites `file:line`.

---

## 1. Executive Summary

The system is architecturally sound and the happy path (judge-on, OpenRouter provider, multi-variant comparison) largely produces correct results — the live DB confirms composites in the expected 0–10 range with working judge scores. But the **2026 grader/metrics redesign that moved process-metrics, harness-adherence, and composite computation into the Go engine introduced a cluster of correctness regressions** that are concentrated in exactly the place a thesis depends on: the metrics and diagnostics that distinguish one harness from another.

The recurring root cause is a **structural impedance mismatch between the metric extractors and the canonical `opencode` executor's turn shape.** The diagnostic/metrics code was written against a free-text / `Role:"tool"` transcript model, but opencode emits structured turns (`Role:"assistant"`, `BlockKind:"tool_use"`, command JSON in `Content`, real output in `ToolOutput`, status in its own field). Six separate metrics silently read ~zero/empty for the executor users are told to use.

### Findings by severity

| Severity | Count |
|---|---|
| Critical | 1 |
| High | 9 |
| Medium | 12 |
| Low | 13 |
| **Total** | **35** |

### The 5 things that most need fixing

1. **Foreign-key cascades are inert at runtime (CRITICAL).** The DB is opened without `?_foreign_keys=on`, so after the first boot `ON DELETE CASCADE` never fires. The live DB already has **338 orphaned variants and 590 orphaned runs**; deletes silently corrupt referential integrity. (`db.go:28`)
2. **Tool-error and recovery metrics are structurally blind to opencode failures (HIGH ×3).** `ToolErrorRate` is `0.0` across **all 295 grades**; recovery profiles, tool-failure symptoms, and the failure classifier's structured signals are empty for the canonical executor. Harnesses cannot be discriminated on the very behaviors AgentDx exists to measure.
3. **API/model-failure runs are stored as `completed` and graded (HIGH ×2).** opencode prints a `type:error` event and exits 0; the engine grades it as a real attempt. The error event is also dropped entirely because `Error` is typed `string` but opencode sends an object. Phantom runs (composite ~2.0–2.6) pollute the dataset; the user can't tell the agent never ran.
4. **Composite halves when the judge is off (HIGH).** With judge disabled, the engine still applies the full `judge*0.3 + spec*0.2` weight against phantom zeros, so a flawless run maxes at 5.0/10. The Python grader handles this correctly, but its result is discarded.
5. **Compare V2 timeline alignment is wrong for every multi-stage harness (HIGH).** speckit/ralph/multiagent merge per-stage transcripts without re-indexing `TurnIndex`, so anchors collide on duplicate keys — the Replay/Tape/Matrix views misalign for the multi-stage harnesses AgentDx is built to diagnose.

A secondary theme: **stored/configured values that are silently ignored** — per-experiment `composite_weights` (no-op), `actual_cost_usd` (never written), `backtrack_count` (computed then dropped), task-root `pyproject.toml` (never copied), and the empty `baselines/` dir (which breaks `docker compose up --build`).

---

## 2. Findings by Area

### 2.1 Storage / API

#### F1 — `ON DELETE CASCADE` is inert at runtime; deletes orphan all child rows **[CRITICAL]**
- **Location:** `engine/internal/storage/db.go:28` (DSN), `migrations/001_initial_schema.sql:1` (PRAGMA), `experiment_repo.go:140` (`DeleteExperiment`), `variant_repo.go:120` (`DeleteVariant`)
- **What's wrong:** SQLite enforces foreign keys only when `PRAGMA foreign_keys = ON` is set *per connection*. The engine opens with a bare DSN `sql.Open("sqlite3", dbPath)` — confirmed at `db.go:28`. The only place the PRAGMA runs is inside migration 001, which `runMigrations` skips on every boot after the first (it's already in `schema_migrations`). `DeleteExperiment`/`DeleteVariant` run only `DELETE FROM experiments/variants WHERE id = ?` and rely entirely on cascade; there is no application-level child cleanup.
- **Evidence:** `sqlite3 frameval.db "PRAGMA foreign_keys"` on the live, already-migrated DB returns `0`. Of 408 variants, **338 are orphaned** (reference non-existent experiment_ids); of 668 runs, **590 are orphaned** (251 completed, 80 failed). 253/295 grades and 315/383 transcripts JOIN to runs whose experiment no longer exists.
- **Impact:** Every experiment/variant deletion after the first restart silently orphans all dependent runs/grades/transcripts/diagnostics. The single-source-of-truth DB bloats permanently and any cross-experiment aggregation is polluted by ghost rows.
- **Fix:** Open with `sql.Open("sqlite3", dbPath+"?_foreign_keys=on")` (or a `ConnectHook` running `PRAGMA foreign_keys=ON` on every new connection), independent of migrations. Add a regression test that opens → closes → re-opens the same file before asserting cascade. Consider a one-time cleanup of existing orphans.

#### F2 — `ListTasks` always returns empty task metadata **[LOW]**
- **Location:** `task_repo.go:390` vs `task_repo.go:413`, `scanTask` at `task_repo.go:469`
- **What's wrong:** `ListTasks` hard-codes the literal `'{}'` in the column position where `GetTask` selects the real `metadata_json`; both feed `scanTask` which unmarshals that position into `task.Metadata`. So `GET /api/tasks` always reports empty metadata while `GET /api/tasks/{id}` returns the real value.
- **Evidence:** Live DB has real non-sensitive metadata (e.g. `{"primary_failure_mode":"STOP_EARLY"}`, `MISREAD`, `SCOPE_DRIFT`, `WRONG_ABS`) on 8 tasks, dropped from the list endpoint. Orchestrator uses `GetTask`, so hidden-file materialization is unaffected.
- **Impact:** List-view fidelity only; no current frontend consumer, no data corruption.
- **Fix:** Replace `'{}'` with `metadata_json` in the `ListTasks` query (the sanitizer already strips `hidden_files`/`workspace_files`).

#### F3 — `experiments.actual_cost_usd` is read everywhere but never written **[LOW]**
- **Location:** `experiment_repo.go:67,108,251` (read/scan), `models/experiment.go:23` (field); no writer anywhere
- **What's wrong:** The column is selected/scanned into `Experiment.ActualCostUSD`, but no code path sets it — `SetExperimentEstimate` writes only `estimated_cost_usd`. Per-run cost *is* tracked (`transcripts.cost_usd`, `grade.CostUSD`) but never rolled up.
- **Evidence:** `COUNT(actual_cost_usd)` over 19 experiments (16 completed) = 0 non-NULL.
- **Impact:** Any "actual cost" surface is always blank (`omitempty` hides it). Reporting gap only.
- **Fix:** Populate on experiment finalize from the sum of per-run costs, or remove the field.

#### F4 — Queue-health Details uses keys that differ from the `QueueStatus` wire shape **[LOW]**
- **Location:** `system_handler.go:118-122` vs `models/config.go:31-33` and `system_handler.go:142`
- **What's wrong:** `/api/health` emits `depth`/`active`/`max`; `/system/queue` (the canonical endpoint the frontend reads) uses `depth`/`active_workers`/`max_workers`.
- **Evidence:** Frontend `types.ts:360-364` + `hooks.ts:368` read `/system/queue` with `active_workers`/`max_workers`. No consumer of the `/api/health` queue Details exists.
- **Impact:** Cosmetic; `/api/health` Details is a non-contractual diagnostic blob.
- **Fix:** Use `active_workers`/`max_workers` in `queueHealth` Details, or embed the snapshot directly.

#### F5 — Several read handlers map any store error to 404, masking real DB failures **[LOW]**
- **Location:** `experiment_handler.go:43-46` (GetExperiment), `run_handler.go:23-26,32-35,68-71` (GetRun/GetTranscript/GetRunGrade)
- **What's wrong:** These four handlers collapse *every* non-nil store error to HTTP 404. They never call `errors.Is(err, sql.ErrNoRows)`, so a transient scan/connection error surfaces as "not found." (`%w` wrapping preserves the sentinel, so the fix is trivial — the cited "can't distinguish" reasoning is inaccurate, but the bug is real.) `diagnostic_handler.go:19` and the rubrics handlers already branch correctly.
- **Impact:** Misleading status codes / degraded observability; no data corruption.
- **Fix:** Branch on `errors.Is(err, sql.ErrNoRows)` → 404, else 500.

#### F6 — Table-rebuild migrations are not transactional / not crash-safe **[LOW]**
- **Location:** `db.go:75-82` (per-statement Exec, no tx); migrations 002/007/016 use `CREATE … / DROP / RENAME` rebuilds
- **What's wrong:** `runMigrations` Execs each statement separately with no enclosing transaction and records the migration only after all statements succeed. A crash mid-rebuild leaves a partially-applied, unrecorded migration that re-runs on next boot. The genuinely destructive window is a crash **after `DROP TABLE experiments` but before `RENAME`** (migration 002:43-44): on re-run, the unconditional `DELETE FROM experiments_new` (002:28) wipes the only surviving data copy. (The reviewer's specific worked example was wrong — the intervening `INSERT…SELECT` makes their window idempotent — but a destructive window does exist.)
- **Impact:** Potential data loss only on a crash mid-migration; one-time risk on fresh/old DBs.
- **Fix:** Wrap each migration file's statements in a single transaction, or drop `IF NOT EXISTS` on scratch tables so a re-run fails loudly.

---

### 2.2 Engine — Orchestrator & Composite Scoring

#### F7 — Composite caps at ~5.0/10 when the LLM judge is disabled **[HIGH]**
- **Location:** `composite.go:21-36`; `grader/server.py:74-82,210-225`; `orchestrator.go:580`
- **What's wrong:** When the judge is off, the grader returns empty `judge.scores` and `adherence.instruction_compliance=0.0`. The engine discards the grader's own composite (`grader_client.go:369-372`: "Do NOT copy response.CompositeScore") and recomputes via `computeComposite`, which *unconditionally* applies `code*0.3 + judge*0.3 + process*0.2 + spec*0.2` (`composite.go:34`). With judge and spec forced to 0, a flawless run maxes at `3.0 + 2.0 = 5.0/10`. The Python `compute_composite` correctly reweights to `code*0.6 + process*0.4` (= 10.0) for the same inputs — but that result is thrown away.
- **Evidence:** `composite.go:34` has no judge-disabled branch; `TestComputeComposite_NoJudgeScores` (`composite_test.go:64-85`) bakes in the depressed 5.0 behavior. This contradicts the code's own stated philosophy (`composite.go:48-51`) of excluding non-gradeable dimensions so they don't "drag the composite down with a phantom zero judge term." The live DB has judge on, so it's latent there — but judge-off is a documented, UI-toggleable, default-fallback mode (`judgeEnabled` defaults false).
- **Impact:** In any judge-off comparison, the headline composite shown on `/diagnostic/compare` is roughly halved for every run regardless of actual performance, silently mis-ranking harnesses.
- **Fix:** Mirror the grader: when there is no usable judge AND no adherence signal, grade on `code*0.6 + process*0.4`. Distinguish "judge disabled" from "judge ran but every dim unavailable."

#### F8 — Per-experiment `composite_weights` are stored, shown in UI, but ignored **[MEDIUM]**
- **Location:** `composite.go:21-36`; `models/experiment.go:24,76`; `experiment_repo.go:253`; `compare.tsx:539`
- **What's wrong:** `Experiment.CompositeWeights` is persisted (`composite_weights_json`, default `{code:0.3,judge:0.3,process:0.2,adherence:0.2}`) and read back per-experiment, and the compare tooltip says *"Weights come from the experiment's composite_weights."* But `computeComposite(grade models.Grade)` takes no weights param and applies hardcoded constants at all three call sites (`orchestrator.go:182,225,580`). A repo-wide grep shows `CompositeWeights` is referenced only in the model, storage marshal/unmarshal, and test fixtures.
- **Impact:** Customizing weights is a silent no-op contradicting the UI. The default `adherence` key doesn't even match the formula's `spec` term, confirming the map is decorative.
- **Fix:** Thread the weights into `computeComposite(grade, weights)`; map `adherence` → spec consistently; or remove the field + tooltip.

#### F9 — In-process completion check omits cancelled/timeout runs; experiment can hang in `running` **[MEDIUM]**
- **Location:** `orchestrator.go:884-897` (`refreshExperimentState`); `run_repo.go:72` (`ListRunnableRuns`); `experiment_repo.go:167` (`ReconcileCompletedExperiments`); `main.go:69`
- **What's wrong:** `refreshExperimentState` (the only *runtime* auto-completion path) counts a run terminal only if status is `completed`/`failed` and gates on `completed == len(runs)`. It never counts `cancelled`/`timeout`. The startup reconciler treats all four as terminal but runs once at boot. After `CancelExperiment` marks runs `cancelled`, a restart (`StartExperiment` has no status guard, `INSERT OR IGNORE` doesn't reset cancelled rows) enqueues only `pending`/`failed` — cancelled runs are neither re-run nor counted, so `completed != len(runs)` stays true forever and the experiment is stuck until the next engine restart.
- **Impact:** A partially-cancelled-then-restarted experiment hangs in `running`; progress/active counters are also inconsistent (omit `pending`/`cancelled`). Self-heals only on restart; no data loss. (Note: `timeout` status is never actually written anywhere, so that branch is theoretical; the `cancelled` branch is concrete.)
- **Fix:** Make `refreshExperimentState`'s terminal set identical to the reconciler's (`completed,failed,cancelled,timeout`). Optionally have `StartExperiment` reset `cancelled` → `pending`.

#### F10 — Regrade paths never refresh the Diagnostic Profile **[MEDIUM]**
- **Location:** `orchestrator.go:121-186` (`RegradeRun`) vs `592,596` (`executeRun`)
- **What's wrong:** `executeRun` computes the grade *and* calls `persistDiagnostic` (fingerprint + symptoms + recovery + failure classifier) + `refreshExperimentAnchors`. `RegradeRun` recomputes the grade/composite and `SaveGrade`s, but never calls `persistDiagnostic` or refreshes anchors. After a regrade, `composite_score`/judge scores are fresh while the diagnostic row (`failure_label`, `classifier_model`, anchors) is stale.
- **Evidence:** `SaveDiagnostic`'s own comment says re-grading should be idempotent (`diagnostic_repo.go:28-29`), confirming this is an oversight. The deterministic fingerprint/symptoms/recovery derive from unchanged `ParsedTurns`, so the genuinely-drifting field is the **LLM failure label** (re-enabling the judge / fixing the API key, then regrading, leaves `failure_label` stale/NULL). `RegradeRunPayload` (188-233) has zero callers — dead code.
- **Impact:** The Compare view mixes a fresh composite with a stale failure label, no indication.
- **Fix:** Call `persistDiagnostic` + `refreshExperimentAnchors` after `SaveGrade` in `RegradeRun`.

#### F11 — `SaveDiagnostic` does DELETE-then-INSERT without a transaction **[LOW]**
- **Location:** `diagnostic_repo.go:34,52`
- **What's wrong:** Two independent `ExecContext` calls (DELETE then INSERT), no surrounding tx — unlike `SaveGrade`, which was explicitly fixed to wrap its DELETE+INSERT in a tx (`grade_repo.go:19-33`) to avoid handing out an empty window. `GetDiagnosticByRun` can momentarily 404 between the two statements, and a crash mid-save loses the diagnostic permanently.
- **Impact:** Narrow: only on a *re-diagnosis* of an already-diagnosed run (first runs DELETE 0 rows). Compounds F25 (frontend fails the whole batch on any 404).
- **Fix:** Wrap DELETE+INSERT in one transaction, mirroring `SaveGrade`.

#### F12 — Engine-computed `BacktrackCount` never persisted (always 0) **[LOW]**
- **Location:** `orchestrator.go:562-566`; `metrics/process.go:155-159`
- **What's wrong:** The grader sends an empty `ProcessGradeResult()`, so `BacktrackCount`/`SelfValidationRate` start at 0. The engine recomputes via `metrics.Process` (which *does* compute `BacktrackCount`) but only copies `ToolCallCount`/`ToolErrorRate`/`RanValidation`/`TurnCount` onto the grade — `pm.BacktrackCount` is never assigned. (`SelfValidationRate` is never computed for the grade at all; the radar's self-validation reads the independently-correct fingerprint, so the only misleading value is `BacktrackCount`.)
- **Evidence:** Over the 59 grades since the redesign (`graded_at >= 2026-06-03`), `backtrack_count` is 0 in all of them; the only nonzero values are legacy pre-redesign rows. Surfaced in `ProcessMetricsCard.tsx:21`, `grade-charts.tsx:81`, `compare.tsx:648` — all show 0 even when the transcript shows repeated edits.
- **Impact:** A per-run rework diagnostic always reads 0. Not in the composite, so scoring unaffected.
- **Fix:** Add `grade.BacktrackCount = pm.BacktrackCount` in `executeRun` (and regrade), or drop the field from display.

---

### 2.3 Engine — Metrics / Harness-Diagnostics (the opencode shape mismatch)

> **Common root cause for F13–F18 + F22–F24:** the canonical `opencode` executor (`registry.go:14` — "the canonical local-Ollama executor") emits tool calls as `Role:"assistant"` turns with `BlockKind:"tool_use"`, the command JSON in `Content`, the real output (stderr/test text) in a separate `ToolOutput` field, the tool name in `ToolName`, and paths in `FilesTouched`. The metrics/diagnostic extractors were written against a `Role:"tool"` + free-text model and never read the structured fields. `anchors.go` reads them correctly — proving the data is available — but the metrics path ignores it.

#### F13 — Recovery analysis + tool-failure symptoms are blind to opencode tool errors **[HIGH]**
- **Location:** `diagnostic/recovery.go:134` (`classifyError`); `diagnostic/symptoms.go:91` (`collectToolFailures`), `121-129` (`lastErrorMessage`)
- **What's wrong:** Error detection requires `Role=="tool"` (`recovery.go:134`, `symptoms.go:91`) and only ever reads `Content`, never `ToolOutput`. opencode never emits `Role:"tool"` turns and puts failure text in `ToolOutput`. So `RecoveryProfile` (`ErrorEvents`, `ErrorAcknowledgmentRate`, `CorrectionLatencyMean`, `CorrectionSuccessRate`, `SilentSkipCount`) and `Symptoms.ToolFailures`/`LastErrorMessage` are systematically zero/empty for opencode runs even when real failures occurred.
- **Evidence:** The Role-agnostic `compileErrorRE`/`testFailureRE` match against `Content`, which holds only the command-input JSON, so they don't compensate. The `recovery_test.go` fixtures (24-153) are built entirely from `Role:"tool"` + inline `tool: write_file` turns opencode never produces — passing tests give false confidence. (The LLM classifier is *partially* mitigated because it also gets the raw transcript tail, but the deterministic profile and structured symptoms have no fallback.)
- **Impact:** A core AgentDx deliverable (deterministic recovery/error metrics, feeding Compare's RecoveryTimeline and the classifier's Symptoms packet) is blank for the recommended executor; cross-harness recovery comparisons are meaningless.
- **Fix:** Treat `BlockKind==tool_use` as a tool event and inspect `ToolOutput` (+ `Stage=="error"`) for failure signatures. Add a fixture in opencode's real shape.

#### F14 — `ToolErrorRate` under-reports: opencode failing tool calls are not detected **[HIGH]**
- **Location:** `metrics/process.go:70` (`isToolError`); `executor/opencode.go:300,387-428`
- **What's wrong:** `isToolError` flags a turn only if `Stage=="error"` OR `ToolOutput` contains the literal `<error>`. opencode sets `Stage="error"` *only* when the tool name is literally `"invalid"` (a hallucinated tool); it never reads `state.status`/`state.error` for valid-but-failed tools, and never emits an `<error>` tag (that's an Anthropic convention). A `pytest`/`bash` that exits non-zero is a routine `tool_use` with plain stderr in `ToolOutput`.
- **Evidence:** `grep "<error>"` finds it only in `process.go` + `process_test.go` — no executor emits it. `ToolErrorRate` feeds the composite's process term (`composite.go:69`: `toolReliability := 1 - grade.ToolErrorRate`, weight 0.4).
- **Impact:** `ToolErrorRate` is near-zero regardless of how many commands failed (see F22 for the all-grades data), inflating the process term and making harnesses indistinguishable on tool reliability.
- **Fix:** Read opencode's `state.status`/`state.error`, stamp `Stage="error"` (or a `ToolError` bool) on non-zero exits; match real failure signatures in `ToolOutput` instead of a tag no executor produces.

#### F15 — `BacktrackCount` counts file READS as rework **[MEDIUM]**
- **Location:** `metrics/process.go:139-146`; `executor/opencode.go:410-415`
- **What's wrong:** The doc says backtrack = "distinct file paths in two or more separate edit/write tool turns," but the loop increments `fileEditCount` for *every* tool turn with `FilesTouched`, with no `ToolName` filter. opencode populates `FilesTouched` for any tool with a `path`/`file`/`filePath` key — including `Read`/`View`. So the normal Read-then-Edit pattern yields `BacktrackCount=1` with a single real edit; re-reading also counts. (opencode `Glob`/`Grep` use a `pattern` key, so they don't trigger — `Read` alone is sufficient.)
- **Impact:** Over-reports rework; distorts the process panel and backtrack-based comparisons. (Currently masked by F12, which zeroes it on persist — but the logic is still wrong and would mislead once F12 is fixed.)
- **Fix:** Restrict counting to mutating tools (`Edit`/`Write`/`str_replace`/`create_file`/`MultiEdit`/`Patch`), or split a separate `ReadCount`.

#### F16 — Harness-adherence "first app edit" / "spec before impl" treats reads as edits **[MEDIUM]**
- **Location:** `metrics/adherence.go:45-64` (`firstEditTurnIndex`), `141-167` (`checkMultiagent`), `222-237` (`checkSpeckit`)
- **What's wrong:** Same root cause as F15. `firstEditTurnIndex` returns the earliest `tool_use` turn whose `FilesTouched` matches, with no edit/write filter. So `checkSpeckit`'s `spec_before_impl` flips to FAIL if the agent Reads an `app/` file during `/speckit.specify`; `checkMultiagent`'s `plan_before_code` FAILs if a planner reads source before emitting its plan.
- **Impact:** `HarnessAdherenceScore` for multiagent and speckit can be a false negative — undermining the metric meant to verify the harness's prescribed plan-before-code / spec-before-impl process.
- **Fix:** Add a mutating-tool filter to `firstEditTurnIndex`.

#### F17 — Behavioral fingerprint dimensions are mismatched to opencode's structured turns **[MEDIUM]**
- **Location:** `diagnostic/fingerprint.go:73,76,77,79,306-311,313-324` (and 159-311)
- **What's wrong:** Most of the 10 fingerprint dimensions regex-scan `Content` for free text (`toolCallRE` needs a `tool:`/`call:` prefix; `fileWriteRE`=`write_file|edit_file|str_replace`; `fileReadRE`=`read_file|cat|view_file`). For opencode, `Content` is command-input JSON and the output is in `ToolOutput`. Reproduced: for Edit/Read/Write/Glob/Bash input JSON, `hasToolCall=false`; only `pytest` matches incidentally because it's a substring. So real tool turns are miscounted as idle thinking → `idleThinkingRatio`/`planningDepth` inflated, `turnEfficiency`/`toolCallDiversity`/`selfValidationRate` deflated; `recoveryLatency`/`looksLikeError` miss errors in `ToolOutput`.
- **Evidence:** `anchors.go:82-88` uses `BlockKind==tool_use` + `ToolName` correctly; the fingerprint ignores those fields. Computed for every run (`orchestrator.go:624`); opencode is the documented default.
- **Impact:** The headline 10-dim fingerprint (shown in Compare, fed to humans) is driven by incidental JSON substrings rather than real behavior for the recommended executor.
- **Fix:** Reimplement dimensions on the structured fields (`BlockKind==tool_use`, `ToolName` classification, `ToolOutput` for error/validation), keeping regexes as a fallback for unstructured (aider) transcripts.

#### F18 — Spec-kit canonical description says "4-stage" but the catalog defines 6 **[LOW]**
- **Location:** `builtin/speckit/catalog.go:33-42`; `builtin/harness/speckit.go:51`
- **What's wrong:** `Name: "Canonical (4-stage)"` and the description list `specify → plan → tasks → implement`, but `Stages` actually has six: specify, clarify, plan, tasks, analyze, implement. `Invoke` walks all six → 6 agent invocations, not 4.
- **Impact:** Misleading label and underestimated run cost (6 vs 4 agent calls). `stages_ran` (`>=2 distinct`) is unaffected. Purely descriptive.
- **Fix:** Update Name/Description/comment to 6 stages, or trim `Stages` to four.

---

### 2.4 Engine — Sandbox / Executor

#### F19 — Multi-stage harness merge leaves colliding `TurnIndex`/`ParentTurnIndex`, corrupting Compare V2 alignment **[HIGH]**
- **Location:** `builtin/harness/speckit.go:223-226`, `ralph.go:145-148`, `multiagent.go:117-121`; consumed by `diagnostic/anchors.go:94-98`
- **What's wrong:** Each executor invocation stamps `TurnIndex` starting at 0 (`pkg/executor/grouping.go:35`, `opencode.go:163`). The merge functions append per-stage turns without re-running `AssignTurnGrouping`, so an N-stage run has multiple turns sharing index 0,1,2…. The merged slice is persisted verbatim (`orchestrator.go:470`). `anchors.go:98` sorts anchors by `TurnIndex` (stable), so stage-2/3 index-0 turns interleave ahead of stage-1 index-5 turns, destroying chronological order. The frontend alignment (`anchor-alignment.ts:96,127-128`) keys columns/tie-breaks on the corrupted indices.
- **Evidence:** `speckit_test.go:297-309` only asserts token/cost summing — re-indexing was never designed in. `symptoms.go`/`recovery.go` use the loop index `i` (`TurnIndex: i`), so they re-index internally and are NOT affected — only the anchor path trusts the stamped field. Single-Execute harnesses (bare, agent_instructions) are unaffected.
- **Impact:** Compare V2 (Replay/Tape/Matrix) timeline alignment is wrong for any speckit/ralph/multiagent run — the multi-stage harnesses AgentDx exists to diagnose. Grading/metrics stay correct; this is a visualization-correctness bug.
- **Fix:** Re-run `AssignTurnGrouping` over the merged `ParsedTurns` at the end of each merge so `TurnIndex` is a single monotonic sequence. Namespace per-stage `ToolUseID`/`messageID` (e.g. prefix with stage name) before regrouping so identical IDs across stages don't merge.

#### F20 — Docker run does not copy workspace back on timeout/cancel **[LOW]**
- **Location:** `sandbox/manager.go:369-388`
- **What's wrong:** `copyWorkspaceFromContainer` is reached only on the clean-exit (`statusCh`) branch. On cancel/timeout, `ContainerWait` returns via `errCh` and the function returns at `wait for sandbox container` before the copy; the deferred `ContainerRemove(Force:true)` then destroys partial agent edits. The timeout-path transcript (`orchestrator.go:455`) saves only `RawOutput`/`ParsedTurns`/tokens — no `OutputFiles`/`Patch`.
- **Impact:** A timed-out run looks like the agent did nothing (no partial patch/output for diagnosis). Free-tier timeouts are common, so this affects how those runs are read. The run is failed and not graded, so no scoring impact. (Fix is non-trivial — the copy needs a fresh background context since the run ctx is cancelled.)
- **Fix:** On the `errCh` branch, attempt a best-effort copy with `context.Background()` + short deadline before returning.

#### F21 — Infrastructure-skipped runs (missing/unconfigured agent binary) persisted as `completed` + graded **[LOW]**
- **Location:** `executor/cursor.go:49-55`, `aider.go:81-83`; `orchestrator.go:597-606`
- **What's wrong:** The general pattern is real and *live via cursor*: when `CURSOR_API_KEY` is unset, `cursor.go` returns a non-nil `RunResult` with nil error, so `executeRun` grades it and marks it `completed` with a degenerate composite (~1.2 from the process term's `toolReliability=1`/`tokenEfficiency=1`). **However the reviewer's aider evidence is stale** — `aider.go:83` now returns `(RunResult, err)` (the "binary not found; execution skipped" path was deleted in commit f20e137 on 2026-05-14); the 5 cited DB rows all predate that fix.
- **Impact:** Misconfigured-cursor non-runs enter the dataset as completed+graded, dragging variant means. Narrow trigger now.
- **Fix:** Detect infra-failure sentinels (not-configured) and mark the run `failed` (or `env_error`); don't emit a composite for runs with no agent execution.

---

### 2.5 Empirical Data (live-DB spot-checks)

#### F22 — `tool_error_rate` is structurally always 0.0 across every run **[MEDIUM]**
- **Location:** `metrics/process.go:66-74`; `executor/opencode.go:299-311` (same root cause as F14)
- **Evidence:** Across **all 295 grades**, `tool_error_rate=0` for 295, nonzero for 0, MAX=0. Across 1625 tool turns in recent completed runs, 0 contain `<error>` and 0 have `stage=='error'`. Run `ed162464` has 118 tool calls and `tool_error_rate=0.0`, yet one tool output is verbatim `ERROR: Not on a feature branch … SCRIPT_EXIT_CODE: 1` — a command that demonstrably exited 1. aider/cursor never set `ToolOutput` or an error `Stage` either, so the conclusion holds for all three executors.
- **Impact:** The composite's tool-reliability term `(1 - ToolErrorRate)*0.4` is a constant 1.0 for every real run — never discriminates clean vs error-laden runs. This is the empirical confirmation of F14.
- **Fix:** Same as F14.

#### F23 — API/model-failure runs are marked `completed` and graded **[HIGH]**
- **Location:** `executor/opencode.go:97-106`; `orchestrator.go:443-460,597-603`
- **What's wrong:** On an API error mid-stream (model not found, free-tier promotion ended), opencode prints a `{"type":"error",…}` event and **exits 0**. `Execute` returns `err==nil`, so the orchestrator (which only fails on `execErr != nil`) grades it and stores it `completed`. Pre-existing repo tests partially pass with no agent work (`test_pass_rate` up to 0.333) and the process term still awards points.
- **Evidence:** All 4 completed runs whose `raw_output` contains a `type:error` event are API failures: `9a81c592`/`f7fec014` ("Free promotion has ended") composite=2.08; `f3704968` ("Model … is not found") composite=2.38; `c49064d2` (`model 'qwen2.5-coder:1.5b' not found`, 404) composite=2.59, `test_pass_rate=0.333`, `parsed_turns_json='[]'`. SQL grouping: `completed|4, failed|10`.
- **Impact:** Infrastructure/config failures pollute the dataset as legitimate scored attempts, skewing per-harness composite distributions; the user can't tell the agent never ran. This is the tool's primary output (Compare).
- **Fix:** In `opencode.go Execute`, scan parsed events for a top-level `type:error` and return a non-nil error; and/or in the orchestrator fail any run with zero meaningful agent turns.

#### F24 — opencode error events are silently dropped (`Error` typed `string`, opencode sends an object) **[HIGH]**
- **Location:** `executor/opencode.go:189` (`Error string`), `323-327` (`case "error"`)
- **What's wrong:** `opencodeEvent.Error` is `string`, but real error events carry `{"error":{"name":"APIError","data":{…}}}`. `json.Unmarshal` of such a line into the struct **fails entirely**, and `ParseTranscript`'s err branch (`132-134`) `continue`s — discarding the whole event including its `type`, so the `case "error"` branch is never reached. The error never becomes a turn, never sets `Stage="error"`.
- **Evidence:** Reproduced with a standalone Go program on the real error line from `c49064d2`: `cannot unmarshal object into Go struct field … of type string`, `Type=""`. That run's `parsed_turns_json='[]'` while `raw_output` holds the full APIError JSON. The test fixture (`opencode_test.go:70-83`) uses `"error":"…"` (a *string*), so it passes while the real object shape fails. The doc at `opencode.go:30` describes behavior that doesn't work.
- **Impact:** Error diagnostics are invisible: the Inspector "Errors only" filter never shows API failures, the classifier gets an empty symptom set, and (with F23) the run is graded as a success.
- **Fix:** Type `Error` as `json.RawMessage` (or `{Name,Data}`), or unmarshal each line into a map first to read `type`. Add a fixture using opencode's real object-shaped error.

#### F25 — Missing/failed spec-adherence result treated as 0% compliance, docking ~2.0 composite points **[MEDIUM]**
- **Location:** `grader_client.go:361-368`; `composite.go:32`
- **What's wrong:** The real trigger is the Python grader's `_SENTINEL` (`spec_adherence/grader.py:12-17,88-128`): on *any* LLM/config failure it returns `instruction_compliance=0.0` + empty `per_instruction`. The engine faithfully copies that 0.0, and `composite.go:32` does `spec = 0*10 = 0` with **no "spec unavailable" sentinel** (unlike the judge term, which excludes `judge_unavailable` dims). A grader hiccup is indistinguishable from violating every instruction.
- **Evidence:** Among all `test_pass_rate=1.0` runs, **48 have spec=0.0 and 48 have spec>0** — half of passing runs scored 0% spec, all with a fully working judge. Every one of the 238 grades with empty `per_instruction` (`'null'`) has spec=0.0 (the `_SENTINEL` signature). Cited examples: `3dd3671e` (tpr=1.0, spec=0.0, complete working rate-limiter, composite=6.546) vs comparable `5a093eee` (spec=1.0, composite=8.498).
- **Impact:** ~half of passing runs are docked ~2 composite points for a grader hiccup, not non-compliance — distorting the harness rankings that are AgentDx's core output.
- **Fix:** When Adherence is missing/empty/sentinel, mark spec unavailable and renormalize the remaining composite weights, rather than counting a hard 0.

#### F26 — `metrics.Process` computes `BacktrackCount` but the orchestrator discards it (empirical) **[LOW]**
- **Location:** `orchestrator.go:562-566`
- This is the live-DB confirmation of F12: among the 59 grades since the redesign, all have `backtrack_count=0`; the 5 nonzero rows are pre-redesign. `ValidationCount` is by-design collapsed into `RanValidation` (which *is* copied), so only `BacktrackCount` is the bug. See F12 for the fix.

---

### 2.6 Grader (Python)

#### F27 — Failure classifier sends wrong-provider default model when `judge.model` is empty **[HIGH]**
- **Location:** `failure_classifier/grader.py:119-122`; reachable via `grader_client.go:419` + `config_handler.go:159-176`
- **What's wrong:** `_client_lazy()` builds the client with the correct provider override, but the model string sent to the API is recomputed at lines 120-122: `effective_model = self.model; if not effective_model: effective_model = load_config().model` — the fallback `load_config()` has **no override**, so it resolves provider from `FRAMEVAL_LLM_PROVIDER` (default `openrouter`) and returns OpenRouter's preset `deepseek/deepseek-chat-v3-0324:free`. So with provider=`anthropic` + empty model, the client is Anthropic but the model id is a deepseek id → API rejects → `classify()` returns `unclassified()` (confidence=0) → orchestrator drops the verdict (`orchestrator.go:664`).
- **Evidence:** Reproduced: `load_config(anthropic-override).model` = `claude-haiku-4-5…`, `load_config().model` = `deepseek/…`, MISMATCH. Reachable in practice: `PutLLMSettings` stores `judge.model` without requiring non-empty, and the Settings UI defaults the model field to `''` (`judge-provider.tsx:21,101`) with the provider default shown only as a placeholder. The LLM judge does it correctly (`llm_judge/grader.py:49,100` — single override-aware `load_config`), proving this is a classifier-specific defect.
- **Impact:** For any non-OpenRouter judge provider with a blank model, every failure classification silently fails — the entire AgentDx failure-diagnosis feature goes dark with no error surfaced.
- **Fix:** Resolve `effective_model` from the same override-aware config used to build the client (store `cfg.model` during `_client_lazy`, or call `load_config(override).model`).

#### F28 — Judge always told `PREMATURE COMPLETION: NO` because `process_grade` is hardcoded empty **[MEDIUM]**
- **Location:** `grader/server.py:52`; `llm_judge/prompts.py:287,297,42-43`
- **What's wrong:** `process: dict = {}` is created and never populated (process metrics moved Go-side; `GradeRunRequest` has no process/premature field). `render_user_prompt` computes `premature = bool(process_grade.get('premature_completion', False))` → always False, so the CRITICAL FACTS block always prints `PREMATURE COMPLETION: NO`, and the system prompt forbids the judge from claiming the agent stopped early.
- **Evidence:** `proto/grader.proto` GradeRunRequest has no process field; the engine computes `PrematureCompletion` deterministically Go-side but never sends it back. (Partial mitigation: the judge also gets a 3000-char transcript tail and the completeness rubric tells it to detect stubs/TODOs directly, so genuine abandonment visible in the tail can still be penalized.)
- **Impact:** The judge is authoritatively misinformed that no run stopped early, biasing completeness/correctness upward on every composite.
- **Fix:** Add a `premature_completion` field to `GradeRunRequest` and populate it from the engine value, or remove the PREMATURE COMPLETION line (and the rubric's reliance on it).

#### F29 — `type_check_pass` uses substring `"any"` match — false FAIL fed to judge as authoritative **[MEDIUM]**
- **Location:** `code_grader/grader.py:50`
- **What's wrong:** `if "any" in text and file["path"].endswith((".ts",".tsx")): type_check_pass = False` — a plain substring test that fires on `many`, `company`, `anything`. The false value is stored (`grade.TypeCheckPass`) and injected into the judge as a CRITICAL FACT (`prompts.py:295`) the judge is told it MUST treat as authoritative (`prompts.py:42-44`).
- **Evidence:** Reproduced: a clean `.ts` file with `manyItems()`/`companyData` → `type_check_pass: False`. The `verified_test_results` override (`server.py:42-51`) does NOT touch `type_check_pass`.
- **Impact:** TS/TSX outputs are mis-reported as failing type checks in stored grades and the judge's facts, biasing scores down. (Current bundled task library is Python-only, so impact today is limited to user-supplied TS tasks.)
- **Fix:** Drop the substring heuristic (it isn't a real type check); use a real `tsc` invocation, or omit `type_check_pass` when no real checker ran.

#### F30 — Code-grader subprocess `TimeoutExpired` is uncaught and propagates out of `GradeRun` **[LOW]**
- **Location:** `code_grader/grader.py:25-32`, called unguarded at `server.py:35`
- **What's wrong:** `grade()` runs each visible test via `subprocess.run(..., shell=True, timeout=120)` with no `try/except`. A >120s hang raises `TimeoutExpired`, which propagates through `GradeRun`; the engine then catches the gRPC error and returns a synthetic `fallbackGrade` (losing real judge/adherence/test results). Note `check=True` is not set, so non-zero exits don't raise — only a true hang triggers this; narrow because the grader tempdir lacks test files so commands usually fail fast.
- **Impact:** A single hanging visible test → synthetic fallback grade for that run.
- **Fix:** Wrap `subprocess.run` in `try/except (TimeoutExpired, OSError)` per case; and/or skip `grade_code`'s own test run when `verified_test_results` is present (see F33).

#### F31 — Python `compute_composite` has a 0-1 vs 0-10 scale mismatch for adherence (dead code) **[LOW]**
- **Location:** `composite.py:44-45`
- **What's wrong:** Blends code/judge/process (0-10) with `adherence_score = instruction_compliance` (0-1) using equal weights, so the adherence term contributes ~1/10th of intent. The Go formula correctly does `*10`. The grader's value is overwritten by the engine.
- **Impact:** None on stored results (discarded); latent maintenance trap. No test exercises the adherence branch.
- **Fix:** Delete the dead `compute_composite` (and stop returning `composite_score`), or fix the scale.

#### F32 — `ComputeStats` would crash: `PairwiseStat(**stat)` includes a non-existent `experiment_id` field (dead RPC) **[LOW]**
- **Location:** `stats/engine.py:37`; `server.py:123`
- **What's wrong:** `compute_stats` builds each stat with an `experiment_id` key; `PairwiseStat(**stat)` would raise `ValueError` (proto has no such field). Dead — engine retired the `ComputeStats` plumbing (no caller). The stats math is also non-meaningful (`cohens_d` = raw mean diff, `observed_power` hardcoded 0.5/0.8, CI = min/max of raw values).
- **Impact:** None today; would crash immediately if re-enabled.
- **Fix:** Remove the dead RPC + stats engine, or drop `experiment_id` and implement real statistics.

---

### 2.7 Frontend

#### F33 *(numbering note: see Cross-cutting)* — see F37 for the grader-side redundancy; frontend items below.

#### F34 — `useCompareDiagnostics` fails the whole query if any one run lacks a diagnostic **[HIGH]**
- **Location:** `lib/hooks.ts:177-178`; consumed in `pages/diagnostic/compare.tsx:146-148,317-323`
- **What's wrong:** `Promise.all(runIds.map((id) => api.get(`/runs/${id}/diagnostic`)))` with no per-item `.catch`, unlike its sibling `useCompareGrades` which uses `.catch(() => null)`. The diagnostic endpoint 404s when a run has no diagnostic row yet (`diagnostic_handler.go:19-21`), which `api.ts:8-11` throws on. A single 404 rejects the entire `Promise.all` → `isError` → "Failed to load diagnostic profiles."
- **Evidence:** The handler's own comment (`diagnostic_handler.go:14-15`) confirms a missing row is a normal state. The downstream partial-tolerance machinery (`isValidDiagnostic`, `droppedDiagnostics`) was purpose-built to skip missing diagnostics but never runs because the query rejects first.
- **Impact:** Selecting any run set where even one lacks a diagnostic (the normal state right after a run completes, or when the classifier failed/timed out) blanks the entire behavioral-analysis half of Compare (radar, failure breakdown, recovery, scatter, evidence) behind a generic error.
- **Fix:** Mirror `useCompareGrades`: `.catch(() => null)` per id, return `Array<Diagnostic | null>` (consumers already handle null).

#### F35 — Compare grade table presents fallback (synthetic) grades identically to real grades **[MEDIUM]**
- **Location:** `compare.tsx:582-690` (GradeComparisonTable); `GradingHeader.tsx:17`
- **What's wrong:** On grader failure the engine synthesizes a placeholder grade (`Source='fallback'`, `LintScore=5`, `TypeCheckPass=false`, `TestPassRate=0`) and persists it on the normal path (`grader_client.go:295-321`, `orchestrator.go:528-533,584`). The Compare table never references `grade.source`. **Worse than the original claim: `Source` is dropped at persistence** — the `grades` table has no `source` column (verified across all migrations + live `PRAGMA table_info`), so `GradingHeader`'s `grade.source === 'fallback'` check is also effectively dead for DB-loaded grades.
- **Impact:** Synthetic placeholder scores from a failed grader call are indistinguishable from real measurements everywhere in the UI — exactly what `Source` was added to flag. Manifests whenever the grader is unavailable (a known-frequent free-tier condition).
- **Fix:** Add a `source` column to the grades schema, persist + serialize it, and badge/dim fallback columns in `GradeComparisonTable` (and ideally `GradeCharts`).

#### F36 — LLM-judge card shows perpetual "Judge in progress…" when the judge is disabled **[MEDIUM]**
- **Location:** `pages/runs/grading.tsx:58`; `components/grading-inspector/LLMJudgeCard.tsx:27-41`
- **What's wrong:** `isGrading = !grade || !grade.judge_scores || Object.keys(grade.judge_scores).length === 0`. With the judge disabled, `judge_scores` is `{}` forever (`disabled_judge_result()`), so `isGrading` is permanently true and the card shows the animated "Judge in progress… (30-90s on free-tier models)" skeleton. `useGrade` correctly stops polling via the `llm_judge_disabled` sentinel, but the card never inspects it.
- **Impact:** With the judge disabled (a supported config), every run's inspector shows an infinite misleading loading state.
- **Fix:** Treat the disabled sentinel (`grade.raw_judge_responses?.[0]?.startsWith('llm_judge_disabled')`) as not-grading and render an explicit "LLM judge disabled" state.

#### F37 — Behavioral charts collide on duplicate run labels, dropping series **[MEDIUM]**
- **Location:** `components/diagnostic/behavioral-radar.tsx:49-73`; `recovery-timeline.tsx:43-49`; `failure-breakdown.tsx:56-90`; labels from `compare.tsx:1032-1048`/`493-508`
- **What's wrong:** Radar builds `row[s.label]` and renders one `<Radar dataKey={s.label}>`; duplicate labels overwrite and recharts renders one series for both. `shortLabel`/`variantSignature` returns the same `harness/agent/model` string for every run of a single variant (the experiment name's tail has no `/`, so the fallback fires). `shortLabel` even ignores its own documented "Run N" disambiguation path because `expIndex` is always populated. `GradeCharts` avoids this via `uniqueLabels()`, but the diagnostic charts don't.
- **Evidence:** Currently *latent* in the live DB — no non-orphaned variant has 2+ completed runs (free-tier timeouts), so present comparisons use distinct multi-variant labels. But `runs_per_variant` defaults to 5 and the "measure variance across N runs of one harness" workflow is supported and would trigger it. (recovery-timeline only produces a React key warning, not a row collapse — slightly overstated originally.)
- **Impact:** When comparing 2+ runs of the same variant, the radar shows one fingerprint instead of all; failure bars merge — fewer series than runs, no warning.
- **Fix:** Run series labels through `uniqueLabels` (lift the helper from `grade-charts.tsx`) before passing to the diagnostic charts, or include the run number when labels would collide.

#### F38 — Compare over-limit warning says "tops out at 5" but the cap is 10 **[LOW]**
- **Location:** `compare.tsx:144,270`
- `overLimit = runIds.length > 10` and all hints say "2–10 runs," but the over-limit card says "tops out at 5" (a stale number matching the 5-color palette, which wraps modulo anyway). Gating logic is correct; pure messaging. **Fix:** change to "tops out at 10."

#### F39 — RunPicker "created" timestamp never renders **[LOW]**
- **Location:** `compare.tsx:391-400,477-478`
- RunPicker renders `{run.created_at && …}`, but the Run model/JSON has no `created_at` (the DB column exists but `run_repo.go` SELECTs omit it; `types.ts` omits it). Always undefined → never shows. **Fix:** use `run.started_at` (which the API sends), or add `created_at` to the Run model.

---

### 2.8 Cross-cutting / Diagnostics Pipeline

#### F40 — Symptom packet never carries `WallClockSeconds`/`TimeoutHit`/`UnexpectedFilesModified` → classifier blind to TIMEOUT & SCOPE_DRIFT; `timeout` status is dead **[HIGH]**
- **Location:** `orchestrator.go:636-643` (RunOutcome); `pkg/diagnostic/symptoms.go:24-43`; `run_repo.go:112-114`
- **What's wrong:** `persistDiagnostic` builds `RunOutcome` with only tests + `CompileFailed` + `FilesTouched`. `WallClockSeconds`, `TimeoutHit`, and `ExpectedFilesModified` are never set anywhere (no DB column for expected files even exists). The symptom packet is serialized verbatim into the classifier prompt, so `timeout_hit` is always false, `wall_clock_seconds` always 0, `unexpected_files_modified` always empty. Compounding this, a timed-out run is recorded as `failed` (`orchestrator.go:457`) and returns *before* `persistDiagnostic`, so the `timeout` CASE branches in `UpdateRunStatus` are dead and `TimeoutHit` can never be true.
- **Evidence:** All 293 diagnostic rows have `timeout_hit:false`, `wall_clock_seconds:0`, no `unexpected_files_modified`. No run has `timeout` status; 30 runs with timeout error_messages are all `failed`. TIMEOUT primary label: **0** across all runs. (SCOPE_DRIFT appeared once as a *secondary* LLM label from transcript text alone, never as a primary — its structured signal is dead.)
- **Impact:** Two of the taxonomy's twelve failure modes (TIMEOUT structurally unreachable, SCOPE_DRIFT signal dead) are under-served despite being advertised. Free-tier timeouts (frequent) are mislabeled as generic test failures, and the calibration study (`score_validation.py` macro-F1 over 12 codes) is biased.
- **Fix:** Populate `WallClockSeconds` from measured duration, set `TimeoutHit` when the run hits the wall-clock deadline (the orchestrator already computes `timedOut` in `invokeWithTimeout` but discards it), pass `ExpectedFilesModified` for brownfield tasks (needs a task field/column). Persist a real `timeout` status or remove the dead branches.

#### F41 — Failure-classifier `transcript_tail` is raw bytes, violating the documented `[idx][role] content` contract **[MEDIUM]**
- **Location:** `orchestrator.go:657-660`; `proto/grader.proto:213`; `failure_classifier/prompts.py:67-68`; `taxonomy.py:70`
- **What's wrong:** Both the proto comment and the prompt docstring specify a per-turn formatted tail (`[<turn_index>][<role>] <content>`). The orchestrator sends `tail := transcript.RawOutput` truncated to the last 4000 *bytes* — for opencode a slab of raw NDJSON with no role/turn markers, often cut mid-object. The classifier is asked to return `EvidenceSpan.turn_index` (`Field(ge=0)`) keyed to those turns; with raw JSON it can't reliably map quotes to indices.
- **Impact:** Degraded classification quality and ungrounded turn-index evidence, worst on the canonical opencode path. (The original "Pydantic validation failures force `unclassified()`" mechanism is speculative — an LLM following the schema emits non-negative ints regardless; the real harm is poor input + ungrounded evidence, matching medium severity.)
- **Fix:** Format the last N `ParsedTurns` as `[<turn_index>][<role>] <content>` (they carry `TurnIndex`/`Role`) and truncate by turn, not bytes.

#### F42 — Grader re-runs all visible test commands on every `GradeRun` even when `verified_test_results` override them **[LOW]**
- **Location:** `grader/server.py:35-51`; `code_grader/grader.py:24-38`; `grader_client.go:238-243`
- **What's wrong:** `code = grade_code(task, output_files)` runs unconditionally (executing every non-hidden test command in a tempdir) *before* the `verified_test_results` override at `server.py:42-51`. On the normal path the engine always sends both verified results AND the test_cases, so the grader's subprocess test results are always computed then discarded. All 28 seeded test_cases are public, so all are re-run.
- **Impact:** Pure overhead (the suite runs twice per graded run), and the discarded run is the one that can raise the F30 `TimeoutExpired`. Correctness preserved by the override; the brownfield commands fail fast (missing test files), so the hang is a latent edge case.
- **Fix:** Skip `grade_code`'s test execution when `verified_test_results` is present (compute lint/type_check/file_state from files only), or stop sending non-hidden test_cases when verified results are attached.

---

## 3. Task Library Quality

The task library is the empirical backbone of the thesis, so trap fidelity and answer-leakage matter as much as code correctness. Three of the seven bundled tasks have material discrimination/leakage problems, one has a broken doc/test contract, and two infrastructure issues (empty `baselines/`, uncopied `pyproject.toml`) affect the library's surroundings.

### Task-quality findings

- **F43 (MEDIUM) — wordfreq doesn't discriminate 3 of its 4 stated failure modes.** `tasks/greenfield-cli-wordfreq/`: all 5 hidden tests are black-box stdout/exit checks. An argparse-only solution (explicitly forbidden) passes all 5; `setup.sh:5` pre-installs `click` before the agent, defeating DEP_MISS. Only STOP_EARLY (the `-c` flag) is genuinely tested. HAL_API/DEP_MISS/WRONG_ABS signals from this task are noise. **Fix:** AST-check `import click` present / `argparse` absent + require `click` in requirements.txt + stop pre-installing click; or drop those modes from the description.

- **F44 (MEDIUM) — hal-api leaks its primary STOP_EARLY trap; the trap fired 0/13 runs.** `tasks/brownfield-hal-api-pydantic-version/`: `technical_details:34` spells out the exact guard ("reject non-empty email strings that lack an @"), and the trap is further pre-disclosed in `workspace/README.md` and the model docstring. `technical_details` is injected into agent prompts (speckit `{{TECHNICAL_DETAILS}}`). DB: across 13 graded runs the legacy-blank regression failed 0 times; the secondary v2-API (HAL_API) check failed 3 times — so the task actually discriminates on HAL_API, not its declared STOP_EARLY. **Fix:** remove the leaking phrasing, leave the blank-email constraint to be discovered in the repo, and re-label `primary_failure_mode` to HAL_API (or strengthen the trap).

- **F45 (MEDIUM) — scope-discipline tests false-positive on agent-created `pytest.ini`.** `tests/test_scope.sh` (all 6 brownfield tasks) flags any tracked-modified or untracked file outside the allowed path as SCOPE_DRIFT. `FRAMEVAL_HARNESS_EXCLUDES` covers opencode.json/CLAUDE.md/.specify etc. but not `pytest.ini`/`conftest.py`/`setup.py`. DB: a ralph run on `brownfield-wrong-abs-async-throttle` (2026-06-04, after the opencode.json fix) PASSED both functional tests but FAILED "Only app/search.py modified" solely on `pytest.ini`. Process-loop harnesses that run tests are systematically under-scored vs bare agents that never run tests — inverting the intended reward. **Fix:** add common test/build artifacts to the exclude set, or diff only `app/**`.

- **F46 (LOW) — throttle task prompt describes a test that doesn't exist.** `tasks/brownfield-wrong-abs-async-throttle/task.yaml:25-26` says "30 concurrent requests … must not collapse below 8 req/s," but the actual test uses `n_requests=50` and asserts a *minimum*-elapsed (rate-limit-enforced) + an event-loop liveness canary — there is no 8 req/s floor and the count is 50. Task still discriminates correctly; doc-level only. **Fix:** sync the prompt to n=50 and the real success criterion.

- **F47 (HIGH) — `baselines/` is empty AND breaks `docker compose up --build`.** `find baselines` returns nothing; CLAUDE.md still documents `baselines/seed.sql`. The AgentDx pivot (commit b5e07e0) deleted the seed + handlers + dropped the `baselines` table (migration 007:78-79) but **left an orphaned `COPY baselines ./baselines` at `docker/engine/Dockerfile:22`.** Docker `COPY` fails when the source matches nothing, so the documented primary startup command fails to build the engine image. CI has no docker build step, so it isn't caught. **Fix:** remove the orphaned `COPY` line (and the stale `baselines/**` path filter in `ci.yml:45`); update CLAUDE.md. *(This was originally filed as task-quality but is really a broken-build bug — see prioritized list.)*

- **F48 (LOW) — task-root `pyproject.toml` is not copied into the sandbox.** `PrepareWorkspace` copies only `task.CodebasePath` (= `taskDir/workspace`); `async-race`/`rate-limiter` place `pyproject.toml` at the task root, so its `asyncio_mode`/`testpaths` settings never reach the sandbox. Currently harmless (tests use explicit paths; pytest-asyncio≥0.23 defaults to strict; tests carry explicit `@pytest.mark.asyncio`), but a latent trap. **Fix:** move needed config into `workspace/`, or drop the ineffective root files.

### Per-task verdict table

| Task | Solvable? | Trap good? | Discriminating? | Leaks? |
|---|---|---|---|---|
| greenfield-cli-wordfreq | Yes | Partial — only STOP_EARLY (`-c`) bites | **No for HAL_API/DEP_MISS/WRONG_ABS** (F43) | No, but trap pre-defeated by `setup.sh` pre-install |
| brownfield-hal-api-pydantic-version | Yes | **No** — primary STOP_EARLY trap fired 0/13 (F44); HAL_API check is the real signal | Yes, but on HAL_API not the declared mode | **Yes** — trap spelled out in `technical_details` + README + docstring |
| brownfield-wrong-abs-async-throttle | Yes | Yes (canary distinguishes sync vs async sleep) | Yes (functional) but **scope test false-positives** (F45) | No (but prompt mis-describes the test, F46) |
| brownfield-fix-async-race | Yes | Yes | Yes | No (root `pyproject.toml` ineffective but harmless, F48) |
| brownfield-misread / scope-drift / stop-early (other brownfield) | Yes | Yes (allowlist scope tests) | Yes, **subject to the same `pytest.ini` false-positive** (F45) | No |
| greenfield-rate-limiter-fastapi | Yes | Yes | Yes | No (root `pyproject.toml` ineffective, F48) |

---

## 4. Empirical Validation (do grades/metrics reflect reality?)

Spot-checks against the live `engine/frameval.db` (≈295 grades, ≈668 runs) reveal that **several headline metrics are constant or systematically wrong on real data**, not just in theory:

| Metric | What the data shows | Reflects reality? |
|---|---|---|
| `tool_error_rate` | 0.0 for **all 295 grades** (MAX=0). Run `ed162464` has 118 tool calls + a verbatim `SCRIPT_EXIT_CODE: 1` output, still 0.0. | **No** — constant (F14/F22) |
| `backtrack_count` | 0 for all 59 post-redesign grades; nonzero only on legacy rows | **No** — computed then discarded (F12/F26) |
| `spec_instruction_compliance` | 48 of 96 `test_pass_rate=1.0` runs scored 0.0, all with a working judge; every empty-`per_instruction` row = 0.0 | **No** — grader hiccups counted as 0% (F25) |
| Composite (judge-off) | Latent (live DB has judge on), but flawless runs would cap at 5.0/10 | **No when judge off** (F7) |
| Run status vs reality | 4 API-failure runs (`type:error`, agent never ran) stored `completed` with composite 2.08–2.59; cursor non-runs ~1.2 | **No** — phantom runs in the dataset (F23/F21) |
| `failure_label` TIMEOUT | 0 of 293 diagnostics; 30 timeout runs all labeled generically | **No** — TIMEOUT unreachable (F40) |
| Referential integrity | 338 orphaned variants, 590 orphaned runs from inert cascades | **No** — DB corrupted (F1) |
| Composite range / judge scores (judge-on path) | 0–9.574, real per-dimension judge scores, no `judge_unavailable` on the passing set | **Yes** — the happy path works |

**Takeaway for the thesis:** the judge-on, OpenRouter, multi-variant comparison configuration in current use produces *believable composite rankings*, but the **process/diagnostic half of every comparison is degraded** (tool errors, backtracks, recovery, fingerprint, TIMEOUT/SCOPE_DRIFT all under-report for the canonical executor), and the dataset is contaminated by phantom infra-failure runs and orphaned rows. Any thesis claim that rests on harness differences in *tool reliability, error recovery, or behavioral fingerprint* is currently unsupported by the data, even though the composite-ranking claim survives.

---

## 5. Cross-cutting / Architectural Notes

1. **The opencode-shape mismatch is the single highest-leverage architectural issue.** Eight findings (F13, F14, F15, F16, F17, F22, and the inputs to F23/F24/F41) all stem from metrics/diagnostic code assuming a free-text or `Role:"tool"` transcript while the canonical executor emits structured turns. `anchors.go` already consumes the structured fields correctly. **Recommendation:** define a small canonical helper set (`isToolUse(turn)`, `toolKind(turn) → read|write|exec|test`, `toolFailed(turn)`, `toolErrorText(turn)`) over `ParsedTurn`'s structured fields, and route *all* metrics/diagnostic extractors through it. This fixes most of the metrics findings at once and prevents regression. A single shared opencode error-event fixture would have caught F13/F14/F22/F24.

2. **"Compute Go-side, return empty from Python" left dangling zeros.** The redesign that moved process/composite into the engine left the Python grader returning empty `ProcessGradeResult()`/`process={}`, which then surfaces as authoritative zeros in three places: the composite's spec/judge terms when disabled (F7), the judge's PREMATURE COMPLETION fact (F28), and `BacktrackCount` (F12). Each "disabled/empty" path needs an explicit sentinel distinct from a real zero.

3. **`Source`/availability sentinels exist in the model but don't survive persistence or the formula.** `grade.Source` has no DB column (F35); there's no "spec unavailable" sentinel (F25); the judge has one but the composite doesn't reuse the pattern. A consistent "this dimension is unavailable → exclude and renormalize" policy across composite, spec, and the UI would resolve F7, F25, and F35 coherently.

4. **Two completion-detection paths disagree (F9), and regrade is not a full re-derivation (F10).** The runtime vs startup terminal-state mismatch and the regrade-doesn't-refresh-diagnostic gap both stem from completion/finalization logic living in more than one place. Consolidate "finalize a run" (grade + diagnostic + anchors + state) into one function used by both `executeRun` and `RegradeRun`.

5. **Transaction discipline is inconsistent** — `SaveGrade` wraps DELETE+INSERT in a tx (F11 says `SaveDiagnostic` doesn't), and migrations aren't transactional (F6). A house rule ("any DELETE-then-INSERT or multi-statement schema change runs in a tx") plus the `_foreign_keys=on` DSN (F1) would close the data-integrity gaps.

6. **Dead/contradictory artifacts to prune:** `RegradeRunPayload` (no callers), `ComputeStats`/`stats/engine.py` (retired RPC that would crash), Python `compute_composite` (discarded), the `timeout` status CASE branches (never written), the `baselines/` Dockerfile COPY (broken), `actual_cost_usd` (never written), `composite_weights` (ignored), `created_at` in RunPicker (never sent). Each is a maintenance trap or a misleading UI/build affordance.

---

## 6. Prioritized Fix List

Ordered by impact (correctness/data-integrity of the thesis dataset first, then UI/diagnostic fidelity, then polish).

1. **F1 — Set `_foreign_keys=on` in the DSN** (`db.go:28`) and clean up existing orphans. *Critical data-integrity; one-line fix.*
2. **F47 — Remove the orphaned `COPY baselines` from `docker/engine/Dockerfile:22`** (+ update CLAUDE.md). *Restores the documented `docker compose up --build`.*
3. **F23 + F24 — Fail runs on opencode `type:error`** (return non-nil error / fail on zero meaningful turns) and **fix `Error` parsing** (`json.RawMessage`). *Stops phantom runs from polluting the dataset and makes API failures visible.*
4. **F13 + F14 + F22 — Make tool-error/recovery/symptom extractors read structured fields** (`BlockKind==tool_use`, `ToolName`, `ToolOutput`, opencode `state.status`). *Restores the process/diagnostic half of every comparison; biggest single-fix leverage (introduce the shared helper in §5.1).*
5. **F25 — Treat missing/sentinel spec-adherence as "unavailable" and renormalize composite weights** (not a hard 0). *Half of passing runs are mis-scored ~2 points today.*
6. **F7 — Add a judge-off branch to `computeComposite`** (`code*0.6 + process*0.4` when no judge/spec signal). *Headline metric halves in the default-fallback config.*
7. **F40 — Populate `WallClockSeconds`/`TimeoutHit`/`ExpectedFilesModified`** in `RunOutcome` (the orchestrator already computes `timedOut`); persist a real `timeout` status. *Unblocks TIMEOUT/SCOPE_DRIFT classification + de-biases the calibration study.*
8. **F19 — Re-run `AssignTurnGrouping` on merged transcripts** (speckit/ralph/multiagent), namespacing per-stage IDs. *Fixes Compare V2 alignment for all multi-stage harnesses.*
9. **F34 — Add `.catch(() => null)` to `useCompareDiagnostics`.** *Stops one missing diagnostic from blanking the whole behavioral half of Compare; trivial fix.*
10. **F27 — Resolve the failure-classifier model from the override-aware config.** *Restores failure diagnosis for non-OpenRouter providers with a blank model.*
11. **F17 — Reimplement fingerprint dimensions on structured fields** (folds into #4's helper). *Headline 10-dim fingerprint currently meaningless for the default executor.*
12. **F35 — Persist `grade.source` (add column) and badge fallback grades in the UI.** *Synthetic grades currently indistinguishable from real ones.*
13. **F45 — Exclude `pytest.ini`/`conftest.py`/`setup.py` from brownfield scope checks** (or diff only `app/**`). *Stops penalizing test-running harnesses; un-inverts the comparison.*
14. **F44 + F43 — Fix task answer-leakage / discrimination** (hal-api `technical_details`+README; wordfreq AST checks + stop pre-installing click). *Restores trap fidelity for the thesis tasks.*
15. **F8 — Thread per-experiment `composite_weights` into `computeComposite`** (or remove the field + tooltip). *Configurable evaluation is currently a no-op.*
16. **F28 — Send/populate `premature_completion` to the judge** (or remove the fabricated fact). *De-biases the judge component.*
17. **F29 — Drop the substring `"any"` type-check heuristic.** *Avoids feeding the judge a fabricated FAIL on TS tasks.*
18. **F9 + F10 — Unify run completion/finalization** (terminal set incl. cancelled/timeout; regrade refreshes diagnostic + anchors). *Fixes stuck experiments and stale-after-regrade profiles.*
19. **F12/F26 — Persist `pm.BacktrackCount`; then F15/F16 — add a mutating-tool filter** so backtrack/adherence stop counting reads. *Make the rework + harness-adherence metrics correct.*
20. **F36 — Render an explicit "LLM judge disabled" state** instead of an infinite skeleton.
21. **F41 — Format `transcript_tail` as `[idx][role] content`** per the documented contract.
22. **F42 + F30 — Skip the redundant grader test run when verified results are present; wrap `subprocess.run` in `try/except`.**
23. **F11 + F6 — Wrap `SaveDiagnostic` (and table-rebuild migrations) in transactions.**
24. **F21 — Mark unconfigured-cursor non-runs as `failed`/`env_error`.**
25. **Polish/cleanup:** F18 (speckit 6-stage label), F38 (compare "tops out at 10"), F39 (RunPicker `started_at`), F2 (ListTasks metadata), F3 (`actual_cost_usd`), F4 (queue-health keys), F5 (404-vs-500 in read handlers), F31/F32 (delete dead grader composite + ComputeStats), F46/F48 (throttle prompt + uncopied pyproject), F20 (best-effort workspace copy on timeout).
