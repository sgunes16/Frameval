# Progressive Grading View — Design

**Status:** Draft
**Date:** 2026-05-25
**Owner:** sgunes16
**Related:** [[2026-05-25-grading-inspector-design]], [[2026-05-25-rubric-editor-design]], `grader/llm_judge/prompts.py`, `engine/internal/experiment/orchestrator.go`

## 1. Motivation

Three rough edges on the new Grading Inspector that the user surfaced after live use:

1. **Per-dim rationale section is a wall of text.** Currently a 2-col grid renders all 5 rationale paragraphs simultaneously. Hard to scan. Want each one collapsible with score visible at all times, expand on click.
2. **No way to see what was actually sent to the judge.** Debugging prompts is core to the thesis ("wordings of context engineering"), but the rendered user prompt evaporates after the gRPC call. Want it surfaced as a "View prompt" detail at the top of the LLM judge section.
3. **Grading page is a black box during grading.** Today `useGrade()` 404s until the entire pipeline (code grader → process metrics → LLM judge × N dims) finishes. With a 600s timeout and free-tier latency, the page sits blank for 30-90s. Want the page to load immediately, show whatever the engine has already computed (verifications + process), and only show a loading skeleton for the LLM judge section.

User pain (verbatim): *"bunu akordiyon şeklinde yap ve ayrıca user prompt'u da görebilelim en başta ek olarak, grading sayfası açılsın bunların sonuçlar dönene kadar sadece llm as judg ekısmı loadingte kalsın knkk ama oradan info dönünce grading sayfası da güncellensin"*.

## 2. Goals & non-goals

**Goals.**
- **Accordion rationales.** `LLMJudgeCard` renders per-dim cards in collapsed state by default (dim name + score badge visible). Click expands to show the rationale; click again collapses. Multiple cards can be open at once. State is local to the page (no persistence; refresh resets to collapsed).
- **Show the user prompt.** Capture the rendered user prompt at judge time, persist it on the grade row, expose it via the existing `/api/runs/:id/grade` endpoint. Frontend renders a `<details>` block (or button-toggled `<pre>`) above the per-dim cards labelled "Prompt sent to judge".
- **Progressive page render.** Engine writes a partial grade row to SQLite as soon as `runTaskVerifications` + `process_grade` produce values (BEFORE calling the LLM judge). The judge call then runs asynchronously; when it completes, the engine UPDATEs the same row with judge fields. Frontend polls `/api/runs/:id/grade` and renders sections progressively — code grading + process metrics + failure classifier appear immediately, LLM judge section shows a loading skeleton until `grade.judge_scores` is populated.
- **No new gRPC RPC.** The existing `GradeRun` call still does both pieces; we split persistence (insert early, update late), not the gRPC boundary.
- **Backward compatible.** `judge_user_prompt` field on the grade is optional. Old grades render with the section hidden. No proto re-numbering risk.

**Non-goals.**
- Streaming the judge's text token-by-token. Still deferred.
- WebSocket push for grade updates. Polling with TanStack Query's `refetchInterval` is enough; WS push is a future optimization.
- Persisting the per-dim system prompts (those are already retrievable via `/api/config/rubrics`).
- Re-architecting GradeRun into separate `GradeCode` + `JudgeRun` RPCs. The split here is persistence-only, not transport.

## 3. Approach

Four substitutions:

1. **Storage** — migration 018 adds `judge_user_prompt TEXT` to grades. New `Store.UpdateGradeJudge(ctx, runID, scores, rationales, irrAlpha, rawResponses, userPrompt)` for the late update.
2. **Proto** — `GradeRunResponse` gains `string judge_user_prompt = 6;` (append-only).
3. **Grader** — `judge_grade()` returns the rendered prompt in its dict (`"user_prompt": prompt`). `server.py` puts it on the proto.
4. **Engine** — `executeRun`:
   - After `runTaskVerifications` + transcript persist, build a Grade with only the deterministic fields and call a NEW `Store.SaveGradePartial(...)` (or reuse `SaveGrade` with empty judge maps).
   - Then call `o.grader.GradeRun(...)` (the LLM-heavy step).
   - When it returns, call `Store.UpdateGradeJudge(...)` to fill the judge maps + irr + raw responses + user prompt.
5. **Frontend** — `useGrade` gains a `refetchInterval` that auto-polls every 2s while the LLM judge fields are still empty AND the run is in a non-terminal state. `LLMJudgeCard` reads `grade.judge_user_prompt` and renders the new prompt details + the new accordion `RationaleCard`. `RunGradingPage` no longer 404s — partial grade exists immediately.

## 4. Targeted changes

### 4.1 SQLite migration `018_grade_judge_user_prompt.sql`

```sql
ALTER TABLE grades ADD COLUMN judge_user_prompt TEXT;
```

That's it. New column nullable. Existing rows untouched.

### 4.2 Store changes (`engine/internal/storage/grade_repo.go`)

- `SaveGrade` already supports nil JudgeScores / nil JudgeRationales (writes NULL JSON). Add `JudgeUserPrompt` to the INSERT.
- New `UpdateGradeJudge(ctx, runID, scores, rationales, irrAlpha, rawResponses, userPrompt) error` — single UPDATE statement that sets the 5 judge-related columns by `run_id`. Useful for the late update path.

### 4.3 Models — `engine/internal/models/grade.go`

Add `JudgeUserPrompt string \`json:"judge_user_prompt,omitempty"\``.

### 4.4 Proto — `proto/grader.proto`

```protobuf
message GradeRunResponse {
  CodeGradeResult code = 1;
  ProcessGradeResult process = 2;
  JudgeGradeResult judge = 3;
  SpecAdherenceResult adherence = 4;
  float composite_score = 5;
  string judge_user_prompt = 6;  // NEW
}
```

`buf generate` regenerates Go + Python stubs. Field 6 is the next free tag.

### 4.5 Grader — `grader/llm_judge/grader.py` + `grader/server.py`

- `_grade_async()` and `grade()` return shape gains `"user_prompt": user_prompt` field. (Already computed inside `_grade_async`; just return it.)
- `_all_dims_failed(reason, rubrics)` returns `"user_prompt": ""` for the sentinel path.
- `server.py:GradeRun` passes `judge["user_prompt"]` to the proto: `judge_user_prompt=judge.get("user_prompt", "")`.

### 4.6 Engine — `orchestrator.go` 2-phase persistence

Find the existing single-shot SaveGrade in `executeRun`. Restructure as:

```go
// Phase 1: persist deterministic grade fields IMMEDIATELY after verifications,
// so the frontend can render the grading page without waiting for the judge.
partial := models.Grade{
    ID:                 uuid.NewString(),
    RunID:              run.ID,
    TestResults:        testResults,
    TestPassCount:      passed,
    TestFailCount:      failed,
    TestPassRate:       float64(passed) / float64(total), // guarded
    FileStateValid:     len(outputFiles) > 0,
    LintScore:          10.0,  // placeholder; updated by Phase 2 from grade_code
    TypeCheckPass:      true,
    Source:             models.GradeSourceGrader,
    GradedAt:           time.Now().UTC().Format(time.RFC3339),
}
_ = o.store.SaveGrade(ctx, partial)

// Phase 2: LLM judge happens here (may take 30-90s on free tier).
grade, gradeErr := o.grader.GradeRun(ctx, *task, artifact, transcript, testResults)
if gradeErr != nil { /* ... existing error handling ... */ }
// Merge judge fields into the row.
_ = o.store.UpdateGradeJudge(ctx, run.ID,
    grade.JudgeScores, grade.JudgeRationales,
    grade.JudgeIRRAlpha, grade.RawJudgeResponses, grade.JudgeUserPrompt)
// Also update process metrics + the recomputed composite.
// Cleanest: a single UpdateGradeFinal(...) that takes the whole Grade
// and overwrites everything except the row id. Keeps the partial row's
// id stable.
```

**Subtlety:** the partial grade has TestPassRate from verifications (good). LintScore + TypeCheckPass come from grader's grade_code, which we DON'T have at Phase 1 — placeholder values get overwritten at Phase 2. Document this in the partial-row comment.

The frontend uses `grade.judge_scores` being empty as the "judge still in flight" signal. If grade exists with empty judge_scores AND the run is in a non-terminal state, poll.

### 4.7 Engine HTTP — no change

`/api/runs/:id/grade` already returns the Grade row. After Phase 1 it returns the partial; after Phase 2 it returns the complete. Frontend handles both.

### 4.8 Frontend — `useGrade` polling

`frontend/src/lib/hooks.ts`:

```typescript
export function useGrade(runId?: string, runStatus?: string) {
  return useQuery({
    queryKey: ['grade', runId],
    enabled: Boolean(runId),
    queryFn: () => api.get<Grade>(`/runs/${runId}/grade`),
    refetchInterval: (q) => {
      // Stop polling once judge_scores has any entries, OR run finished.
      const grade = q.state.data as Grade | undefined;
      if (grade && grade.judge_scores && Object.keys(grade.judge_scores).length > 0) return false;
      if (runStatus === 'completed' || runStatus === 'failed') return false;
      return 2000; // poll every 2s while grading is in flight
    },
  });
}
```

Backward compat: `useGrade(id)` still works (no runStatus → polls until judge appears). Callers that have run.status pass it in.

### 4.9 Frontend — accordion RationaleCard

`frontend/src/components/grading-inspector/LLMJudgeCard.tsx`:

- New `useState<Set<string>>` for open dim keys.
- Each `RationaleCard` becomes a `<button>` wrapper for the header row (dim + score badge). Click toggles. Rationale text only renders when key is in the open set.
- Add a chevron icon (or `▸` / `▾` text) to signal collapse state.
- When the judge data is still loading (empty `judge_scores`), render a skeleton inside the LLM judge Card section instead of the bars + accordion.

### 4.10 Frontend — show user prompt details

Above the per-dim accordion, add:

```tsx
{grade.judge_user_prompt && (
  <details className="mb-3 rounded-lg border border-border bg-bg-elev-1 p-2">
    <summary className="cursor-pointer text-xs font-medium text-fg-muted">
      Prompt sent to judge ({grade.judge_user_prompt.length.toLocaleString()} chars)
    </summary>
    <pre className="mt-2 max-h-96 overflow-auto whitespace-pre-wrap rounded bg-bg-elev-2 p-2 font-mono text-xs text-fg">
      {grade.judge_user_prompt}
    </pre>
  </details>
)}
```

Frontend Grade type gains `judge_user_prompt?: string`.

### 4.11 Frontend — loading skeleton for the LLM judge Card

When `Object.keys(grade.judge_scores ?? {}).length === 0` AND we're still polling (no judge result yet), the LLM judge Card body shows a `LoadingSkeleton` placeholder + a text "Judge in progress…" instead of zeros. Five empty rows are misleading.

The other Cards (CodeGradingCard, ProcessMetricsCard, FailureClassifierCard) render normally with available data.

## 5. Testing

**Backend.**
- `grade_repo_test.go` gains a test for `UpdateGradeJudge` (insert partial, then update, then GetGradeByRun returns the merged shape).
- `orchestrator_test.go` (if it exists) — verify the two-SaveGrade-call sequence. Lower priority; happy-path manual smoke is enough.

**Grader.**
- `test_judge.py` cases already return `"user_prompt"` now — assert it's present and non-empty in the happy-path test.

**Frontend.**
- `grading.test.tsx` — assert the accordion renders collapsed by default, click expands. Existing mock + test pattern.

**Manual smoke.**
1. Trigger a run, immediately open `/runs/:id/grading`.
2. Expect: page loads instantly, Code grading + Process metrics render with real numbers, LLM judge card shows a loading skeleton.
3. After ~60-90s, judge result lands → card refreshes, bars + collapsed accordion appear.
4. Click a dim card → rationale expands.
5. Click "Prompt sent to judge" details → see the full rendered prompt with CRITICAL FACTS, Task, Output files, Transcript tail.

## 6. Risks

1. **Partial grade row before judge means the Compare view briefly shows judge_scores={} for in-flight runs.** Acceptable; the BarRow loop gracefully renders nothing for empty maps. Document.
2. **Two-phase persistence introduces a transient invalid composite_score** (Phase 1 row's composite was computed without judge). Recompute and overwrite in Phase 2 — already happens in the existing recomputeCompositeScore call.
3. **Polling at 2s × N concurrent runs hits the engine.** Acceptable for local-first. If needed, switch to WS push later.
4. **User prompt can be very large (~10KB+).** Storing it in SQLite is fine; rendering as a `<pre>` is bounded by `max-h-96` + `overflow-auto`. No frontend perf issue.

## 7. Rollout

Single PR on `feature/llm-judge`, ordered:

1. Migration 018 + `UpdateGradeJudge` repo method + tests.
2. Proto edit + regen stubs.
3. Grader `grade()` returns `user_prompt`; `server.py` puts it on the proto.
4. Engine `executeRun` two-phase persistence; orchestrator regrade paths gain `UpdateGradeJudge` for symmetry (or stay one-shot since regrade is async-blocking already — pick whichever matches the existing pattern).
5. Frontend `Grade` type field + `useGrade` polling + accordion + user-prompt details + loading skeleton.
6. Smoke test gate.
