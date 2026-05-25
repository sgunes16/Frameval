# Grading Inspector Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a `/runs/:id/grading` page showing the full grading pipeline (code grader + process metrics + LLM judge rationale + failure classifier) for one run, plus a regrade button. Make the engine's grader timeout env-configurable (default 600s) and clean up the diagnostic `[judge_debug]` prints left from the previous PR.

**Architecture:** New React route + page consuming existing `/api/runs/:id/grade` and `/api/runs/:id/diagnostic` endpoints. Five Card components, each focused. One new mutation hook for regrade. One new module mirroring Python failure-code descriptions. Tiny engine change for env-configurable timeout. No proto/SQLite changes.

**Tech Stack:** Go (engine config), Python (grader cleanup), React + TypeScript + TanStack Query + Tailwind + react-router-dom.

**Spec:** [`docs/superpowers/specs/2026-05-25-grading-inspector-design.md`](../specs/2026-05-25-grading-inspector-design.md)

---

## File Structure

**Modify (Go):**
- `engine/internal/experiment/grader_client.go` — promote `gradeRunTimeout` from const to env-reading var.
- `engine/internal/experiment/grader_client_test.go` — add 2 tests (default + override + garbage).

**Modify (Python):**
- `grader/server.py` — remove 4 `[judge_debug]` print lines.
- `grader/llm_judge/grader.py` — remove 4 `[judge_debug]` print lines.

**Create (Frontend):**
- `frontend/src/lib/failure-codes.ts`
- `frontend/src/pages/runs/grading.tsx`
- `frontend/src/components/grading-inspector/index.ts`
- `frontend/src/components/grading-inspector/GradingHeader.tsx`
- `frontend/src/components/grading-inspector/CodeGradingCard.tsx`
- `frontend/src/components/grading-inspector/ProcessMetricsCard.tsx`
- `frontend/src/components/grading-inspector/LLMJudgeCard.tsx`
- `frontend/src/components/grading-inspector/FailureClassifierCard.tsx`
- `frontend/src/pages/runs/grading.test.tsx` (smoke render test)

**Modify (Frontend):**
- `frontend/src/lib/hooks.ts` — add `useRegradeRun` mutation.
- `frontend/src/routes.tsx` — register `/runs/:id/grading`.
- `frontend/src/pages/runs/inspect.tsx` — add "View grading →" button.
- `frontend/src/pages/diagnostic/compare.tsx` — add per-column "details" link in LLM-as-Judge section.

**Modify (docs):**
- `CLAUDE.md` — add `FRAMEVAL_GRADER_TIMEOUT_SECONDS` env-var row.

---

## Task 1: Env-configurable grader timeout

**Files:**
- Modify: `engine/internal/experiment/grader_client.go`
- Modify: `engine/internal/experiment/grader_client_test.go`

- [ ] **Step 1: Write the failing tests**

Append to `engine/internal/experiment/grader_client_test.go`:

```go
func TestGradeRunTimeout_Default(t *testing.T) {
	t.Setenv("FRAMEVAL_GRADER_TIMEOUT_SECONDS", "")
	got := resolveGradeRunTimeout()
	if got != 600*time.Second {
		t.Errorf("default = %v, want 600s", got)
	}
}

func TestGradeRunTimeout_EnvOverride(t *testing.T) {
	t.Setenv("FRAMEVAL_GRADER_TIMEOUT_SECONDS", "120")
	got := resolveGradeRunTimeout()
	if got != 120*time.Second {
		t.Errorf("override = %v, want 120s", got)
	}
}

func TestGradeRunTimeout_GarbageFallsBackToDefault(t *testing.T) {
	t.Setenv("FRAMEVAL_GRADER_TIMEOUT_SECONDS", "not-a-number")
	got := resolveGradeRunTimeout()
	if got != 600*time.Second {
		t.Errorf("garbage = %v, want default 600s", got)
	}
}

func TestGradeRunTimeout_NegativeFallsBackToDefault(t *testing.T) {
	t.Setenv("FRAMEVAL_GRADER_TIMEOUT_SECONDS", "-5")
	got := resolveGradeRunTimeout()
	if got != 600*time.Second {
		t.Errorf("negative = %v, want default 600s", got)
	}
}
```

- [ ] **Step 2: Run tests — confirm FAIL**

Run: `cd engine && go test ./internal/experiment/ -run TestGradeRunTimeout -v`
Expected: FAIL with "undefined: resolveGradeRunTimeout".

- [ ] **Step 3: Refactor `gradeRunTimeout`**

Edit `engine/internal/experiment/grader_client.go`. Replace the existing constant (currently `const gradeRunTimeout = 90 * time.Second` from the previous PR — verify exact location):

```go
// gradeRunTimeout caps the end-to-end GradeRun gRPC call. Real LLM judge
// calls on free-tier providers regularly take 30-90s; cumulative grading
// with multiple stages can push past 2 minutes. 600s is generous but
// finite so a hung run cannot pin a worker forever. Override with
// FRAMEVAL_GRADER_TIMEOUT_SECONDS for slower providers or larger prompts.
var gradeRunTimeout = resolveGradeRunTimeout()

func resolveGradeRunTimeout() time.Duration {
	if raw := os.Getenv("FRAMEVAL_GRADER_TIMEOUT_SECONDS"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 {
			return time.Duration(n) * time.Second
		}
	}
	return 600 * time.Second
}
```

Add `"os"` and `"strconv"` to the import block at the top of the file if not already present.

- [ ] **Step 4: Run tests — confirm PASS**

Run: `cd engine && go test ./internal/experiment/ -run TestGradeRunTimeout -v`
Expected: all 4 PASS.

- [ ] **Step 5: Run full engine suite — no regressions**

Run: `cd engine && go test ./... 2>&1 | tail -10`
Expected: all packages OK.

- [ ] **Step 6: Commit**

```bash
git add engine/internal/experiment/grader_client.go engine/internal/experiment/grader_client_test.go
git commit -m "Make GradeRun timeout env-configurable (FRAMEVAL_GRADER_TIMEOUT_SECONDS, default 600s)"
```

---

## Task 2: Remove `[judge_debug]` diagnostic prints

**Files:**
- Modify: `grader/server.py`
- Modify: `grader/llm_judge/grader.py`

- [ ] **Step 1: Remove prints from `grader/server.py`**

Find and remove these lines (all four of them, added in commit `a0c2def`):

```python
print(f"[judge_debug] code_grade done; test_pass_rate={code.get('test_pass_rate')}", flush=True)
```
```python
print(f"[judge_debug] process_grade done", flush=True)
```
```python
print(f"[judge_debug] judge_cfg={'<set>' if judge_cfg else None} env_enable={settings.enable_llm_judge} provider={getattr(judge_cfg, 'provider', None)} model={getattr(judge_cfg, 'model', None)} has_key={bool(getattr(judge_cfg, 'api_key', None))}", flush=True)
```
```python
print(f"[judge_debug] calling judge_grade...", flush=True)
```
```python
print(f"[judge_debug] judge_grade returned; raw_responses_head={(judge.get('raw_responses') or ['<empty>'])[0][:120]}", flush=True)
```
```python
print(f"[judge_debug] judge disabled (env=false, no proto config)", flush=True)
```

(There are 6 print lines total in server.py — remove all of them. Keep the surrounding grade-flow code intact.)

- [ ] **Step 2: Remove prints from `grader/llm_judge/grader.py`**

Find and remove:

```python
print(f"[judge_debug] resolved cfg provider={cfg.provider} base_url={cfg.base_url} model={cfg.model} has_key={bool(cfg.api_key)}", flush=True)
```
```python
print(f"[judge_debug] client built", flush=True)
```
```python
print(f"[judge_debug] client init exception: {exc!r}", flush=True)
```
```python
print(f"[judge_debug] calling client.create model={cfg.model} prompt_len={len(prompt)}", flush=True)
```
```python
print(f"[judge_debug] client.create returned ok correctness={verdict.correctness}", flush=True)
```
```python
print(f"[judge_debug] client.create exception: {exc!r}", flush=True)
```

Leave the `logger.warning(...)` calls — those are the persistent diagnostic channel.

- [ ] **Step 3: Verify grader tests still pass**

Run: `cd grader && uv run pytest 2>&1 | tail -8`
Expected: 35/35 PASS (no regressions; the tests don't rely on the print output).

- [ ] **Step 4: Commit**

```bash
git add grader/server.py grader/llm_judge/grader.py
git commit -m "Remove temporary [judge_debug] tracing prints"
```

---

## Task 3: `useRegradeRun` mutation hook + failure-codes module

**Files:**
- Create: `frontend/src/lib/failure-codes.ts`
- Modify: `frontend/src/lib/hooks.ts`

- [ ] **Step 1: Create failure-codes module**

Create `frontend/src/lib/failure-codes.ts`:

```typescript
import type { FailureCode } from './types';

/**
 * Human-readable descriptions for FailureCode enum values.
 *
 * Mirrors grader/failure_classifier/taxonomy.py:FAILURE_DESCRIPTIONS.
 * Keep in sync — when adding/changing a code there, update here too.
 */
export const FAILURE_DESCRIPTIONS: Record<FailureCode, string> = {
  NONE: 'No failure detected.',
  HAL_API: 'Hallucinated API — used a function/method/parameter that does not exist.',
  HAL_FILE: 'Phantom file — referenced a file that was never created or wrong location.',
  DEP_MISS: 'Missing dependency — used package without installing or declaring it.',
  STOP_EARLY: 'Premature completion — declared task done while tests still failing.',
  STOP_GIVEUP: 'Surrender — declared inability to proceed without exhausting options.',
  LOOP_INF: 'Infinite loop / no progress — repeated same action with no state change.',
  WRONG_ABS: "Wrong abstraction — solution structure doesn't match task (sync vs async).",
  MISREAD: 'Spec misread — solution targets wrong requirement (broke contract).',
  ENV_ERR: 'Environment failure — failure caused by sandbox/tool, not the agent.',
  SCOPE_DRIFT: 'Scope drift — modified files outside expected scope for brownfield task.',
  TIMEOUT: 'Wall-clock timeout — run exceeded time budget before completion.',
  SILENT_SKIP: 'Silent failure — agent encountered error and ignored it in subsequent turns.',
};
```

**Important:** Before saving, verify the exact list of FailureCode values in `frontend/src/lib/types.ts:58-71` matches the keys above. If new codes were added since this plan was written, mirror them. Cross-reference with `grader/failure_classifier/taxonomy.py:FAILURE_DESCRIPTIONS` to copy the canonical wording.

- [ ] **Step 2: Add `useRegradeRun` to hooks.ts**

Edit `frontend/src/lib/hooks.ts`. After the existing `useGrade` definition (around line 149), append:

```typescript
export function useRegradeRun() {
  const client = useQueryClient();
  return useMutation({
    mutationFn: (runId: string) => api.post<void>(`/runs/${runId}/regrade`, null),
    onSuccess: (_, runId) => {
      client.invalidateQueries({ queryKey: ['grade', runId] });
      // Diagnostic query key — verify by grepping for the existing useDiagnostic hook;
      // it's likely ['diagnostic', runId]. If different, match what useDiagnostic uses.
      client.invalidateQueries({ queryKey: ['diagnostic', runId] });
    },
  });
}
```

Verify the diagnostic query key by running: `grep -n "queryKey.*diagnostic" frontend/src/lib/hooks.ts` and use whatever key the existing `useDiagnostic` uses.

- [ ] **Step 3: Verify build is clean**

Run: `cd frontend && npm run build 2>&1 | tail -5`
Expected: clean tsc + vite build.

- [ ] **Step 4: Commit**

```bash
git add frontend/src/lib/failure-codes.ts frontend/src/lib/hooks.ts
git commit -m "Add failure-code descriptions and useRegradeRun mutation hook"
```

---

## Task 4: Grading inspector — Card components

**Files:**
- Create: `frontend/src/components/grading-inspector/GradingHeader.tsx`
- Create: `frontend/src/components/grading-inspector/CodeGradingCard.tsx`
- Create: `frontend/src/components/grading-inspector/ProcessMetricsCard.tsx`
- Create: `frontend/src/components/grading-inspector/LLMJudgeCard.tsx`
- Create: `frontend/src/components/grading-inspector/FailureClassifierCard.tsx`
- Create: `frontend/src/components/grading-inspector/index.ts`

Each component is a focused renderer for one Card. Builds bottom-up so the page in Task 5 can compose them.

- [ ] **Step 1: GradingHeader.tsx**

```tsx
import type { Grade, Run } from '../../lib/types';
import { Badge } from '../ui/badge';
import { Button } from '../ui/button';
import { Card, CardHeader } from '../ui/card';

export function GradingHeader({
  run,
  grade,
  onRegrade,
  regradeBusy,
}: {
  run: Run;
  grade: Grade;
  onRegrade: () => void;
  regradeBusy: boolean;
}) {
  const isFallback = grade.source === 'fallback';
  return (
    <Card>
      <CardHeader
        title={`Composite score: ${grade.composite_score?.toFixed(2) ?? '—'}`}
        description={`Run ${run.id} · variant ${run.variant_id} · status ${run.status}`}
      />
      <div className="flex items-center justify-between gap-2">
        <div className="flex items-center gap-2 text-xs">
          {isFallback ? (
            <Badge tone="danger">source: fallback (grader unreachable)</Badge>
          ) : (
            <Badge tone="success">source: grader</Badge>
          )}
          <span className="text-fg-muted">
            graded at {grade.created_at ?? '—'}
          </span>
        </div>
        <Button onClick={onRegrade} disabled={regradeBusy}>
          {regradeBusy ? 'Regrading…' : 'Regrade'}
        </Button>
      </div>
    </Card>
  );
}
```

(Verify `Grade` has fields `source`, `created_at`, `composite_score`. If `source` is missing from the TypeScript type, add it: `source?: 'grader' | 'fallback'`. The Go side writes it per `engine/internal/experiment/grader_client.go:fallbackGrade`.)

- [ ] **Step 2: CodeGradingCard.tsx**

```tsx
import { useState } from 'react';
import type { Grade } from '../../lib/types';
import { Badge } from '../ui/badge';
import { Card, CardHeader } from '../ui/card';

export function CodeGradingCard({ grade }: { grade: Grade }) {
  const tests = grade.test_results ?? [];
  return (
    <Card>
      <CardHeader
        title="Code grading"
        description="Deterministic test runner + lint + type-check."
      />
      <div className="grid gap-2 text-sm">
        <Row label="Test pass rate" value={`${(grade.test_pass_rate * 100).toFixed(0)}%`} />
        <Row label="Tests" value={`${grade.test_pass_count ?? 0} / ${(grade.test_pass_count ?? 0) + (grade.test_fail_count ?? 0)} passed`} />
        <Row label="Lint score" value={`${grade.lint_score?.toFixed(1) ?? '—'} / 10`} />
        <Row label="Type check" value={grade.type_check_pass ? 'pass' : 'fail'} />
        <Row label="File state" value={grade.file_state_valid ? 'ok' : 'broken'} />
      </div>
      {tests.length > 0 && (
        <div className="mt-3 border-t border-border pt-3">
          <div className="mb-2 text-xs uppercase tracking-wider text-fg-muted">Per-test</div>
          <ul className="space-y-1">
            {tests.map((t, i) => (
              <TestRow key={i} test={t} />
            ))}
          </ul>
        </div>
      )}
    </Card>
  );
}

function Row({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex justify-between gap-2 border-b border-border/40 py-1 last:border-0">
      <span className="text-fg-muted">{label}</span>
      <span className="font-mono text-fg">{value}</span>
    </div>
  );
}

function TestRow({ test }: { test: { name: string; passed: boolean; output: string } }) {
  const [open, setOpen] = useState(false);
  return (
    <li className="rounded border border-border bg-bg-elev-1 p-2">
      <div className="flex items-center justify-between gap-2">
        <span className="truncate font-mono text-xs">{test.name}</span>
        <div className="flex items-center gap-2">
          {test.passed ? <Badge tone="success">pass</Badge> : <Badge tone="danger">fail</Badge>}
          {test.output && (
            <button
              className="text-xs text-fg-muted underline"
              onClick={() => setOpen((v) => !v)}
            >
              {open ? 'hide' : 'output'}
            </button>
          )}
        </div>
      </div>
      {open && test.output && (
        <pre className="mt-2 max-h-48 overflow-auto whitespace-pre-wrap rounded bg-bg-elev-2 p-2 text-xs text-fg-muted">
          {test.output}
        </pre>
      )}
    </li>
  );
}
```

- [ ] **Step 3: ProcessMetricsCard.tsx**

```tsx
import type { Grade } from '../../lib/types';
import { Card, CardHeader } from '../ui/card';

export function ProcessMetricsCard({ grade }: { grade: Grade }) {
  return (
    <Card>
      <CardHeader
        title="Process metrics"
        description="Heuristic transcript metrics."
      />
      <div className="grid grid-cols-2 gap-2 text-sm">
        <Row label="Turns" value={`${grade.turn_count ?? '—'}`} />
        <Row label="Tokens" value={grade.total_tokens ? grade.total_tokens.toLocaleString() : '—'} />
        <Row label="Cost (USD)" value={grade.cost_usd != null ? `$${grade.cost_usd.toFixed(4)}` : '—'} />
        <Row label="Backtracks" value={`${grade.backtrack_count ?? 0}`} />
        <Row label="Idle turns" value={`${grade.idle_turns ?? 0}`} />
        <Row label="Error recoveries" value={`${grade.error_recovery_count ?? 0}`} />
        <Row label="Token efficiency" value={fmtBar(grade.token_efficiency)} />
        <Row label="Context utilization" value={fmtBar(grade.context_utilization)} />
        <Row label="Tool call accuracy" value={fmtBar(grade.tool_call_accuracy ?? 0)} />
        <Row label="Self-validation rate" value={fmtBar(grade.self_validation_rate ?? 0)} />
        <Row label="Premature completion" value={grade.premature_completion ? 'yes' : 'no'} />
      </div>
    </Card>
  );
}

function Row({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex justify-between gap-2 border-b border-border/40 py-1">
      <span className="text-fg-muted">{label}</span>
      <span className="font-mono text-fg">{value}</span>
    </div>
  );
}

function fmtBar(v: number): string {
  return `${(v * 100).toFixed(0)}%`;
}
```

- [ ] **Step 4: LLMJudgeCard.tsx**

```tsx
import { useState } from 'react';
import type { Grade } from '../../lib/types';
import { Card, CardHeader } from '../ui/card';

export function LLMJudgeCard({ grade }: { grade: Grade }) {
  const [showRaw, setShowRaw] = useState(false);
  const rationale = extractRationale(grade.raw_judge_responses);
  const dims = [
    { label: 'Correctness', value: grade.judge_correctness },
    { label: 'Maintainability', value: grade.judge_maintainability ?? 0 },
    { label: 'Completeness', value: grade.judge_completeness ?? 0 },
    { label: 'Best practices', value: grade.judge_best_practices ?? 0 },
    { label: 'Error handling', value: grade.judge_error_handling ?? 0 },
  ];
  return (
    <Card>
      <CardHeader
        title="LLM-as-judge rubric"
        description="Five-dimension judgment from the configured judge model."
      />
      <div className="space-y-1">
        {dims.map((d) => (
          <DimRow key={d.label} label={d.label} value={d.value} />
        ))}
      </div>
      {grade.judge_irr_alpha != null && grade.judge_irr_alpha > 0 && (
        <div className="mt-2 text-xs text-fg-muted">
          Inter-rater α: <span className="font-mono">{grade.judge_irr_alpha.toFixed(2)}</span>
        </div>
      )}
      {rationale && (
        <div className="mt-3 border-t border-border pt-3">
          <div className="mb-1 text-xs uppercase tracking-wider text-fg-muted">Rationale</div>
          <p className="text-sm text-fg">{rationale}</p>
        </div>
      )}
      {(grade.raw_judge_responses?.length ?? 0) > 0 && (
        <div className="mt-3 border-t border-border pt-3">
          <button
            className="text-xs text-fg-muted underline"
            onClick={() => setShowRaw((v) => !v)}
          >
            {showRaw ? 'hide raw response' : 'show raw response (debug)'}
          </button>
          {showRaw && (
            <pre className="mt-2 max-h-64 overflow-auto whitespace-pre-wrap rounded bg-bg-elev-2 p-2 text-xs text-fg-muted">
              {JSON.stringify(grade.raw_judge_responses, null, 2)}
            </pre>
          )}
        </div>
      )}
    </Card>
  );
}

function DimRow({ label, value }: { label: string; value: number }) {
  const pct = Math.max(0, Math.min(100, (value / 10) * 100));
  return (
    <div className="flex items-center gap-3 text-sm">
      <div className="w-32 text-fg-muted">{label}</div>
      <div className="flex h-2 flex-1 overflow-hidden rounded bg-bg-elev-2">
        <div className="h-full bg-primary" style={{ width: `${pct}%` }} />
      </div>
      <div className="w-12 text-right font-mono text-fg">{value.toFixed(2)}</div>
    </div>
  );
}

// Pulls the .rationale field out of the first JSON-encoded judge response.
// Returns null when raw_judge_responses is empty, non-JSON (e.g. sentinel),
// or has no .rationale key. Sentinel strings like
// "judge_unavailable: <reason>" surface as null here.
function extractRationale(raw: string[] | undefined): string | null {
  if (!raw || raw.length === 0) return null;
  for (const entry of raw) {
    try {
      const parsed = JSON.parse(entry);
      if (typeof parsed?.rationale === 'string' && parsed.rationale.length > 0) {
        return parsed.rationale;
      }
    } catch {
      // not JSON (probably a sentinel); skip
    }
  }
  return null;
}
```

- [ ] **Step 5: FailureClassifierCard.tsx**

```tsx
import { Link } from 'react-router-dom';
import type { Diagnostic } from '../../lib/types';
import { FAILURE_DESCRIPTIONS } from '../../lib/failure-codes';
import { Badge } from '../ui/badge';
import { Card, CardHeader } from '../ui/card';

export function FailureClassifierCard({
  diagnostic,
  runId,
}: {
  diagnostic: Diagnostic | undefined;
  runId: string;
}) {
  const cls = diagnostic?.classification;
  if (!cls || cls.primary === 'NONE') {
    return (
      <Card>
        <CardHeader
          title="Failure classifier"
          description="LLM-driven failure categorization across 12 codes."
        />
        <div className="text-sm text-fg-muted">No failure classified for this run.</div>
      </Card>
    );
  }
  return (
    <Card>
      <CardHeader
        title="Failure classifier"
        description="LLM-driven failure categorization across 12 codes."
      />
      <div className="space-y-3 text-sm">
        <div className="flex items-center gap-2">
          <Badge tone="danger" title={FAILURE_DESCRIPTIONS[cls.primary]}>{cls.primary}</Badge>
          {(cls.secondary ?? []).map((c) => (
            <Badge key={c} tone="muted" title={FAILURE_DESCRIPTIONS[c]}>{c}</Badge>
          ))}
          {cls.confidence != null && (
            <span className="text-xs text-fg-muted">
              confidence: <span className="font-mono">{cls.confidence.toFixed(2)}</span>
            </span>
          )}
        </div>
        {cls.rationale && <p className="text-fg">{cls.rationale}</p>}
        {(cls.evidence?.length ?? 0) > 0 && (
          <div className="border-t border-border pt-3">
            <div className="mb-1 text-xs uppercase tracking-wider text-fg-muted">Evidence</div>
            <ul className="space-y-1">
              {cls.evidence!.map((e, i) => (
                <li key={i} className="rounded border border-border bg-bg-elev-1 p-2">
                  <div className="mb-1 flex items-center gap-2 text-xs text-fg-muted">
                    <Badge tone="muted">{e.code}</Badge>
                    <Link
                      to={`/runs/${runId}/inspect?focus=${e.turn_index}`}
                      className="underline"
                    >
                      Turn {e.turn_index}
                    </Link>
                  </div>
                  {e.quote && <p className="font-mono text-xs text-fg">{e.quote}</p>}
                </li>
              ))}
            </ul>
          </div>
        )}
      </div>
    </Card>
  );
}
```

- [ ] **Step 6: index.ts barrel export**

```typescript
export { GradingHeader } from './GradingHeader';
export { CodeGradingCard } from './CodeGradingCard';
export { ProcessMetricsCard } from './ProcessMetricsCard';
export { LLMJudgeCard } from './LLMJudgeCard';
export { FailureClassifierCard } from './FailureClassifierCard';
```

- [ ] **Step 7: Verify build**

Run: `cd frontend && npm run build 2>&1 | tail -8`
Expected: clean build. Common errors: missing types on Grade (e.g., `source`, `created_at`). If TS complains, add the missing fields to the Grade type in `frontend/src/lib/types.ts` — but verify the engine actually returns them via the API first (`curl http://localhost:8080/api/runs/<id>/grade | jq`).

- [ ] **Step 8: Commit**

```bash
git add frontend/src/components/grading-inspector/
git commit -m "Add Grading Inspector Card components"
```

---

## Task 5: Grading Inspector page + route

**Files:**
- Create: `frontend/src/pages/runs/grading.tsx`
- Modify: `frontend/src/routes.tsx`

- [ ] **Step 1: Write the page**

Create `frontend/src/pages/runs/grading.tsx`:

```tsx
import { useParams } from 'react-router-dom';
import {
  CodeGradingCard,
  FailureClassifierCard,
  GradingHeader,
  LLMJudgeCard,
  ProcessMetricsCard,
} from '../../components/grading-inspector';
import { ErrorState, LoadingSkeleton } from '../../components/system';
import { useDiagnostic, useGrade, useRegradeRun, useRun } from '../../lib/hooks';

/**
 * Run Grading Inspector — `/runs/:id/grading`.
 *
 * Symmetric to the Turn Inspector (`/runs/:id/inspect`) but for the
 * grading pipeline rather than execution flow. Renders five Cards:
 * header (composite + regrade), code grader, process metrics, LLM
 * judge rationale, and failure classifier evidence.
 */
export function RunGradingPage() {
  const { id } = useParams<{ id: string }>();
  const runQuery = useRun(id);
  const gradeQuery = useGrade(id);
  const diagnosticQuery = useDiagnostic(id);
  const regrade = useRegradeRun();

  if (runQuery.isError || gradeQuery.isError) {
    return (
      <ErrorState
        title="Could not load grading data"
        description="The engine returned an error or the grade doesn't exist."
        onRetry={() => {
          runQuery.refetch();
          gradeQuery.refetch();
        }}
      />
    );
  }

  if (runQuery.isLoading || gradeQuery.isLoading) {
    return (
      <div className="space-y-2">
        <LoadingSkeleton variant="row" count={6} />
      </div>
    );
  }

  const run = runQuery.data;
  const grade = gradeQuery.data;
  if (!run || !grade || !id) {
    return <ErrorState title="Run not found" description="No data to display." />;
  }

  return (
    <div className="space-y-4">
      <GradingHeader
        run={run}
        grade={grade}
        onRegrade={() => regrade.mutate(id)}
        regradeBusy={regrade.isPending}
      />
      <CodeGradingCard grade={grade} />
      <ProcessMetricsCard grade={grade} />
      <LLMJudgeCard grade={grade} />
      <FailureClassifierCard diagnostic={diagnosticQuery.data} runId={id} />
    </div>
  );
}
```

- [ ] **Step 2: Register the route**

Edit `frontend/src/routes.tsx`. Find the existing line:
```tsx
<Route path="/runs/:id/inspect" element={<RunInspectPage />} />
```
Add directly under it:
```tsx
<Route path="/runs/:id/grading" element={<RunGradingPage />} />
```
And add the import at the top:
```tsx
import { RunGradingPage } from './pages/runs/grading';
```

- [ ] **Step 3: Verify build**

Run: `cd frontend && npm run build 2>&1 | tail -8`
Expected: clean build.

- [ ] **Step 4: Manual smoke check (controller, not subagent)**

The controller will manually open `http://localhost:5173/runs/<id>/grading` after the subagent finishes and verify all 5 Cards render. Subagent should not attempt this.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/pages/runs/grading.tsx frontend/src/routes.tsx
git commit -m "Add /runs/:id/grading page composing the five Grading Inspector cards"
```

---

## Task 6: Cross-links from Turn Inspector and Compare

**Files:**
- Modify: `frontend/src/pages/runs/inspect.tsx`
- Modify: `frontend/src/pages/diagnostic/compare.tsx`

- [ ] **Step 1: Turn Inspector → Grading link**

Edit `frontend/src/pages/runs/inspect.tsx`:

a) Add `useNavigate` to the existing `react-router-dom` import:
```tsx
import { useNavigate, useParams, useSearchParams } from 'react-router-dom';
```

b) Inside `RunInspectPage`, just after `const [searchParams, setSearchParams] = useSearchParams();`, add:
```tsx
const navigate = useNavigate();
```

c) In the header JSX, just after the existing `Re-parse turns` button (around line 199 — find the conditional rendering block with `reparse.mutate(id)`), add:
```tsx
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

- [ ] **Step 2: Compare → Grading per-run link**

Edit `frontend/src/pages/diagnostic/compare.tsx`. The page already has `runIds` (array of run ids matching column order, line ~143) and the headers array (`headers`). In the LLM-as-Judge `<SectionHeader>` invocation (around line 653), append a small per-column footer with chevron links.

Implementation approach (subagent decides best fit for the existing JSX structure):
- Add a tiny `JudgeDetailsLinks` row right under the `SectionHeader` for "LLM-as-Judge rubric":
  ```tsx
  <tr>
    <td className="text-xs text-fg-muted pl-3 py-1">details:</td>
    {runIds.map((runId, i) => (
      <td key={i} className="py-1 text-center">
        <Link
          to={`/runs/${runId}/grading`}
          className="text-xs text-fg-muted underline hover:text-fg"
          title="Open grading inspector for this run"
        >
          open →
        </Link>
      </td>
    ))}
  </tr>
  ```
- Import `Link` from `react-router-dom` at the top if not already imported.

This keeps the column alignment intact and gives one link per run column without crowding the existing bar rows.

- [ ] **Step 3: Verify build**

Run: `cd frontend && npm run build 2>&1 | tail -8`

- [ ] **Step 4: Commit**

```bash
git add frontend/src/pages/runs/inspect.tsx frontend/src/pages/diagnostic/compare.tsx
git commit -m "Cross-link Turn Inspector and Compare to Grading Inspector"
```

---

## Task 7: Smoke render test for the new page

**Files:**
- Create: `frontend/src/pages/runs/grading.test.tsx`

- [ ] **Step 1: Write the smoke test**

Create `frontend/src/pages/runs/grading.test.tsx` (Vitest + React Testing Library — match patterns from `frontend/src/components/run-inspector/TurnGroupCard.test.tsx`):

```tsx
import { render, screen } from '@testing-library/react';
import { describe, it, expect, vi } from 'vitest';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter, Route, Routes } from 'react-router-dom';

import { RunGradingPage } from './grading';

// Stub hooks module — only the hooks the page uses.
vi.mock('../../lib/hooks', () => ({
  useRun: () => ({
    data: { id: 'r-1', variant_id: 'v-1', status: 'completed' },
    isLoading: false,
    isError: false,
  }),
  useGrade: () => ({
    data: {
      composite_score: 6.5,
      source: 'grader',
      test_pass_rate: 1.0,
      test_pass_count: 3,
      test_fail_count: 0,
      lint_score: 7,
      type_check_pass: true,
      file_state_valid: true,
      test_results: [{ name: 'test_one', passed: true, output: 'ok' }],
      judge_correctness: 8,
      judge_maintainability: 7,
      judge_completeness: 8,
      judge_best_practices: 6,
      judge_error_handling: 5,
      judge_irr_alpha: 0,
      raw_judge_responses: ['{"correctness":8,"rationale":"solid solution"}'],
      turn_count: 5,
      total_tokens: 1200,
    },
    isLoading: false,
    isError: false,
  }),
  useDiagnostic: () => ({ data: undefined }),
  useRegradeRun: () => ({ mutate: vi.fn(), isPending: false }),
}));

function renderPage() {
  const qc = new QueryClient();
  render(
    <QueryClientProvider client={qc}>
      <MemoryRouter initialEntries={['/runs/r-1/grading']}>
        <Routes>
          <Route path="/runs/:id/grading" element={<RunGradingPage />} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

describe('RunGradingPage', () => {
  it('renders composite, judge scores, and rationale from a populated grade', () => {
    renderPage();
    expect(screen.getByText(/Composite score: 6.50/)).toBeInTheDocument();
    expect(screen.getByText(/Correctness/)).toBeInTheDocument();
    expect(screen.getByText(/solid solution/)).toBeInTheDocument();
    expect(screen.getByText(/Regrade/)).toBeInTheDocument();
  });
});
```

(Adapt mock data shape if the actual Grade type differs from above. Run the test and adjust.)

- [ ] **Step 2: Run the test**

Run: `cd frontend && npm test -- grading.test 2>&1 | tail -15`
Expected: 1 test PASS.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/pages/runs/grading.test.tsx
git commit -m "Add smoke render test for Grading Inspector page"
```

---

## Task 8: CLAUDE.md env-var update

**Files:**
- Modify: `CLAUDE.md`

- [ ] **Step 1: Add the new env-var row**

In `CLAUDE.md`'s "## Environment Variables" section, add this row (insert near the other `FRAMEVAL_*` engine vars):

| Variable | Service | Description |
|---|---|---|
| `FRAMEVAL_GRADER_TIMEOUT_SECONDS` | engine | Caps the engine→grader gRPC `GradeRun` call. Default `600`. Bump higher for slow free-tier judge models. |

- [ ] **Step 2: Commit**

```bash
git add CLAUDE.md
git commit -m "Document FRAMEVAL_GRADER_TIMEOUT_SECONDS env var"
```

---

## Self-review (run before declaring plan done)

**Spec coverage:**
- §4.1 (engine timeout) → Task 1
- §4.2 (remove tracing) → Task 2
- §4.3 (new route) → Task 5
- §4.4 (new page) → Task 5
- §4.5 (hooks) → Task 3
- §4.6 (Turn → Grading cross-link) → Task 6
- §4.7 (Compare → Grading cross-link) → Task 6
- §4.8 (failure-code descriptions) → Task 3
- §5 (testing) → Tasks 1 (engine), 7 (frontend)
- §6 (risks) → addressed by surfacing fallback badge, graceful no-classifier render, 600s timeout

All covered.

**Placeholder check:** No TBD/TODO/"fill in later" in the plan. Every code block is complete or has explicit verify-then-adapt guidance for type drift.

**Type consistency:**
- `useGrade` queryKey is `['grade', runId]` everywhere (matches existing hooks.ts:149).
- `useRegradeRun` invalidates `['grade', runId]` and `['diagnostic', runId]` — verify the diagnostic key matches the existing `useDiagnostic` hook (Task 3 step 2 calls this out explicitly).
- Component prop types: `GradingHeader` takes `run: Run`, `grade: Grade`; `CodeGradingCard`/`ProcessMetricsCard`/`LLMJudgeCard` take `grade: Grade`; `FailureClassifierCard` takes `diagnostic: Diagnostic | undefined` + `runId: string`. All match the page's wiring in Task 5.
- The `source` field on `Grade` may need to be added to the TS type if missing — Task 4 step 7 calls this out.

Plan ready to execute.
