# Grading Inspector — Design

**Status:** Draft
**Date:** 2026-05-25
**Owner:** sgunes16
**Related:** [[2026-05-23-llm-judge-design]], `frontend/src/pages/runs/inspect.tsx`, `frontend/src/components/run-inspector/`, `engine/internal/api/runs.go`, `engine/internal/experiment/grader_client.go`

## 1. Motivation

Real LLM judge scores landed (LLM judge PR, branch `feature/llm-judge`). The grade row now contains a populated `raw_judge_responses_json` field with each judge's rationale (up to 600 chars per dimension), plus per-test code-grader output and failure-classifier verdicts. None of this surfaces in the UI today: the Compare page renders five bar rows for the judge dimensions, but the rationale, the failing test output, the failure-code evidence quotes — all just sit in SQLite unused.

User pain (verbatim): *"grading kısmını da burada bir yere açar mısın grading loglarını görmek istiyorum nasıl değerlendirmiş filan belki yine inspector tarzı bir ekran olabilir ayrıca grading için"* — the existing Inspector V2 is for turn-by-turn execution flow; the user wants a symmetric view for the grading pipeline so they can see *why* the judge scored what it scored.

Secondary motivation: the LLM judge smoke test surfaced a 20s gRPC timeout that cut off legitimate slow free-tier judge calls. A bump to 600s (matching user's directive) plus an env knob for ops removes the cliff. Streaming the judge rationale token-by-token from OpenRouter is a tempting UX win but adds three layers of new plumbing (gRPC server-streaming, WS topology, partial-JSON parsing in React) — deferred to a follow-up spec.

Also, the prior PR left `[judge_debug] ...` `print(..., flush=True)` tracing in `grader/server.py` and `grader/llm_judge/grader.py` to diagnose the silent fallback. With root cause known and judge working, the tracing must come out.

## 2. Goals & non-goals

**Goals.**
- New route `/runs/:id/grading` rendering a "Grading Inspector" — symmetric to `/runs/:id/inspect` (Turn Inspector V2), but for the grading pipeline.
- Five sections in the page, each in its own Card:
  1. **Header** — composite score, source badge (grader / fallback), grader latency, timestamps.
  2. **Code grading** — test-by-test pass/fail list with collapsible output, lint score, type-check pass, file-state-valid.
  3. **Process metrics** — turn count, total tokens, cost, efficiency / utilization / tool-call-accuracy / self-validation bars, backtracks, idle turns, error recoveries, premature-completion flag.
  4. **LLM judge** — five bars (correctness / maintainability / completeness / best_practices / error_handling), the parsed rationale paragraph, IRR α, and a collapsible "Raw response (debug)" pre block dumping `raw_judge_responses_json`.
  5. **Failure classifier** — primary code with description tooltip, secondary codes, confidence, evidence quotes with turn-index links (clicking jumps to `/runs/:id/inspect?focus=N`), rationale.
- **Regrade button** in the header that POSTs to the existing `/api/runs/:id/regrade` endpoint and refreshes on success. Useful when iterating on judge prompts or switching providers.
- **Cross-links**: existing Turn Inspector header gets a "View grading →" button; Compare page LLM-as-Judge section gets a "details" link per cell that deep-links to the per-run grading view.
- **Engine timeout bump** for `GradeRun` gRPC call from current 90s to **600s** default, with `FRAMEVAL_GRADER_TIMEOUT_SECONDS` env override.
- **Remove `[judge_debug]` print tracing** from `grader/server.py` and `grader/llm_judge/grader.py`. The Python `logger.warning` calls stay (those are the proper diagnostic channel).

**Non-goals.**
- Streaming the judge rationale live as it generates. Requires gRPC server-streaming + new WS event types + partial-Pydantic parsing in React. Real value but big scope; deferred.
- Editing the grade from the UI (manual override of judge scores). Out of scope; regrade is the only mutation.
- Multi-run grading comparison view. The Compare page already does that for the score numbers; the Grading Inspector is single-run.
- Showing the prompt that was sent to the judge. The prompt is reconstructable from `task` + `code_grade` + `process_grade` + `output_files` but reconstructing it client-side adds complexity. Out of scope; the `raw_judge_responses_json` is enough debug surface for now.
- New backend tests beyond what changes (handler tests stay green; no new endpoint).

## 3. Approach

Three substitutions:

1. **Backend timeout + env knob** — replace the hardcoded `gradeRunTimeout = 90 * time.Second` constant in `engine/internal/experiment/grader_client.go` with a value read from `FRAMEVAL_GRADER_TIMEOUT_SECONDS` env var (default 600). Read once at engine startup.
2. **Frontend route + page** — register `/runs/:id/grading` in `routes.tsx`, add `GradingInspectorPage` component, build five Card sections consuming the existing `/api/runs/:id/grade` + `/api/runs/:id/diagnostic` endpoints (no backend HTTP changes needed — both endpoints already return all the data the page consumes).
3. **Cleanup** — `git rm` the `[judge_debug]` prints, single small commit.

The Grade type already includes every field we need (test_results, raw_judge_responses, judge_irr_alpha, spec_per_instruction). Failure classification comes via `useDiagnostic()` which is already used in Turn Inspector. No new hooks needed; we reuse `useRun`, `useRunGrade` (already exists? — verify in plan), `useDiagnostic`, plus a new `useRegradeRun` mutation that wraps `POST /api/runs/:id/regrade`.

## 4. Targeted changes

### 4.1 Engine — configurable timeout

`engine/internal/experiment/grader_client.go`:

```go
// gradeRunTimeout caps the end-to-end GradeRun gRPC call. Real LLM judge
// calls on free-tier providers regularly take 30-90s; cumulative grading
// with multiple stages can push past 2 minutes. 600s is generous but
// finite so a hung run cannot pin a worker forever. Override with
// FRAMEVAL_GRADER_TIMEOUT_SECONDS for slower providers or larger prompts.
var gradeRunTimeout = func() time.Duration {
    if raw := os.Getenv("FRAMEVAL_GRADER_TIMEOUT_SECONDS"); raw != "" {
        if n, err := strconv.Atoi(raw); err == nil && n > 0 {
            return time.Duration(n) * time.Second
        }
    }
    return 600 * time.Second
}()
```

(Promoting from `const` to package-level `var` lets it pick up the env var at init time without thread-safety concerns — set once, read many.)

Add `FRAMEVAL_GRADER_TIMEOUT_SECONDS` to the env-var table in `CLAUDE.md`.

### 4.2 Grader — remove diagnostic tracing

`grader/server.py` — remove the four `[judge_debug] ...` print lines added in commit `a0c2def` from `GradeRun`. Leave the unchanged grade-flow logic intact.

`grader/llm_judge/grader.py` — remove the four `[judge_debug] ...` print lines added in the same commit (around `load_config`, `build_client`, `client.create`). Leave the `logger.warning(...)` calls; those are the proper persistent diagnostic channel.

### 4.3 New frontend route — `/runs/:id/grading`

`frontend/src/routes.tsx`:

```tsx
<Route path="/runs/:id/grading" element={<RunGradingPage />} />
```

(Add directly under the existing `<Route path="/runs/:id/inspect" element={<RunInspectPage />} />`.)

### 4.4 New page — `frontend/src/pages/runs/grading.tsx`

Single page component with five Card sections. Pseudocode-level outline (concrete code in the plan):

```tsx
export function RunGradingPage() {
  const { id } = useParams<{ id: string }>();
  const runQuery = useRun(id);
  const gradeQuery = useGrade(id);          // new hook in hooks.ts
  const diagnosticQuery = useDiagnostic(id); // existing
  const regrade = useRegradeRun();           // new mutation hook

  // Loading / error guards mirroring inspect.tsx pattern...

  return (
    <div className="space-y-4">
      <GradingHeader run={...} grade={...} onRegrade={...} regradeBusy={...} />
      <CodeGradingCard grade={...} />
      <ProcessMetricsCard grade={...} />
      <LLMJudgeCard grade={...} />
      <FailureClassifierCard diagnostic={...} runId={id} />
    </div>
  );
}
```

Each `*Card` is a small sibling component in `frontend/src/components/grading-inspector/` (new directory). One file per Card so each stays focused (≤120 lines).

### 4.5 New hooks — `frontend/src/lib/hooks.ts`

**`useGrade` already exists** (hooks.ts:149) using `queryKey: ['grade', runId]`. Reuse as-is.

Add one new mutation hook:

```tsx
export function useRegradeRun() {
  const client = useQueryClient();
  return useMutation({
    mutationFn: (runId: string) => api.post<void>(`/runs/${runId}/regrade`, null),
    onSuccess: (_, runId) => {
      client.invalidateQueries({ queryKey: ['grade', runId] });
      client.invalidateQueries({ queryKey: ['diagnostic', runId] }); // verify exact key
    },
  });
}
```

The `Grade` type already exists in `frontend/src/lib/types.ts` (per LLM judge spec §1) with all needed fields including `raw_judge_responses`, `judge_irr_alpha`, `test_results`, `spec_per_instruction`. No new types.

### 4.6 Cross-link: Turn Inspector → Grading Inspector

`frontend/src/pages/runs/inspect.tsx` header — add next to the existing "Re-parse turns" button. **Button does not support `asChild`** in this codebase, so wrap a Link with the button styling or use `useNavigate`:

```tsx
const navigate = useNavigate();
// ...in the header JSX, after the Re-parse button:
{id && (
  <Button
    size="sm"
    variant="ghost"
    onClick={() => navigate(`/runs/${id}/grading`)}
  >
    View grading →
  </Button>
)}
```

### 4.7 Cross-link: Compare page → Grading Inspector (per run)

`frontend/src/pages/diagnostic/compare.tsx` — in the LLM-as-Judge section header, append a small "details" affordance per run column. Click → navigates to `/runs/<runId>/grading`. Implementation detail: the column headers already hold `runId`; pipe a `runIdForColumn(i)` helper into the section header and render a chevron link.

### 4.8 Failure-code descriptions

The Grading Inspector's Failure Classifier card shows the primary FailureCode (e.g., `HAL_API`) with its human description as a tooltip. The frontend currently mirrors the codes in `frontend/src/lib/types.ts:58-71` (union type) plus color/glyph maps in `failure-breakdown.tsx` and `SymptomGlyph.tsx` — **but no description text exists**. Add a new module `frontend/src/lib/failure-codes.ts` with a `FAILURE_DESCRIPTIONS: Record<FailureCode, string>` map mirroring `grader/failure_classifier/taxonomy.py:FAILURE_DESCRIPTIONS`. Include a header comment listing the Python source so future drift is obvious.

## 5. Testing

**Backend.**
- Unit test the new `gradeRunTimeout` env reading: set env to `120`, assert duration is 120s; set garbage, assert default 600s. Put in `engine/internal/experiment/grader_client_test.go`.

**Frontend.**
- Smoke render test for `RunGradingPage` against a fixture grade with non-zero judge dims. Verify the rationale text appears, the test results render, the regrade button is enabled. Vitest + RTL pattern from existing tests.
- Test that clicking an evidence quote with `turn_index=4` produces a link to `/runs/:id/inspect?focus=4`.

**No new integration / E2E tests.** Manual smoke is the end-to-end gate.

**Manual smoke (`Task 14`-style) gate before merge:**
1. Run an experiment with judge enabled, qwen3-coder:free on OpenRouter (timeout headroom now generous).
2. Open `/runs/<id>/grading` directly. Verify all five Cards render with real data.
3. From the Compare page, click a judge-cell "details" link, verify it lands on the run-specific Grading Inspector.
4. From the Grading Inspector, click "View grading →" inverse link (Inspector → Grading and back).
5. Click an evidence quote on the Failure Classifier card, verify it deep-links into the Turn Inspector with the right `?focus=N`.
6. Click Regrade. Verify the grade refreshes and the source badge stays "grader" (not flipping to "fallback").

## 6. Risks

1. **Timeout 600s + slow free model = orchestrator throughput cap.** With `FRAMEVAL_MAX_CONCURRENT=3` and judge calls averaging 90s, a 5-variant × 5-run experiment serializes 25 judge calls across 3 workers → worst case ~12 minutes. Acceptable for demo; not for production. Document in CLAUDE.md.
2. **Raw JSON debug panel leaks rationale to the UI.** The judge rationale could theoretically include sensitive content from the user's task. Not a real risk for local-first single-user, but flag in a comment.
3. **Regrade button race.** If the user clicks Regrade twice quickly, two grading pipelines may run for the same run. The existing `/regrade` endpoint must already handle this or return 409; verify in the plan. If not, this spec does NOT add an idempotency guard — that's a backend concern out of scope.
4. **Cross-link from Compare requires runId in column header data.** If `compare.tsx`'s header construction doesn't already carry runId per column, the cross-link addition is bigger than 5 lines. Pre-verify in the plan; if non-trivial, scope to defer the Compare cross-link to a follow-up.
5. **Failure classifier might not have run for older runs.** UI must show "no classifier result" gracefully (empty Card or "—") rather than crash on undefined fields.

## 7. Rollout

Single PR, ordered:

1. Engine — env-configurable timeout + tests.
2. Grader — remove `[judge_debug]` prints.
3. Frontend — new hooks (`useGrade`, `useRegradeRun`).
4. Frontend — new page + five Card components + route registration.
5. Frontend — cross-links (Turn Inspector → Grading, Compare → Grading per run).
6. Docs — CLAUDE.md env table update, brief Grading-inspector note in README.

Verify locally with one experiment end-to-end before opening PR. Per project convention, the PR goes through `feature-dev:code-reviewer` before merge.
