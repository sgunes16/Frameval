import { Fragment, useEffect, useMemo, useState } from 'react';

import { cn } from '../../lib/utils';
import { Link, useSearchParams } from 'react-router-dom';

import { BehavioralRadar } from '../../components/diagnostic/behavioral-radar';
import { CostQualityScatter } from '../../components/diagnostic/cost-quality-scatter';
import { FailureBreakdown } from '../../components/diagnostic/failure-breakdown';
import { RecoveryTimeline } from '../../components/diagnostic/recovery-timeline';
import { TranscriptEvidence } from '../../components/diagnostic/transcript-evidence';
import { ErrorBoundary } from '../../components/system';
import { Badge } from '../../components/ui/badge';
import { Button } from '../../components/ui/button';
import { Card, CardHeader } from '../../components/ui/card';
import { EmptyState } from '../../components/ui/empty-state';
import { ScoreBar } from '../../components/system/composite/ScoreBar';
import { useCompareDiagnostics, useCompareGrades, useExperiments, useRuns } from '../../lib/hooks';
import type { Diagnostic, Grade } from '../../lib/types';
import { formatTimeAgo, statusLabel, statusTone } from '../../lib/utils';

/**
 * Diagnostic Compare page.
 *
 * The user flow is:
 *   1. Pick an experiment from the dropdown.
 *   2. Tick 2–5 runs from that experiment's run list (checkboxes,
 *      pre-checked for completed runs by default).
 *   3. The five comparison charts render below.
 *
 * The selection is mirrored to URL search params (`?experiment=…&runs=…`)
 * so a share-link button copies a deep link. Pasting a manual list of run
 * IDs is still possible via the URL but no longer the primary UI — the
 * earlier "paste a CSV" textarea was a terrible first-run experience.
 */
export function DiagnosticComparePage() {
  const [searchParams, setSearchParams] = useSearchParams();
  const experimentID = searchParams.get('experiment') ?? '';

  // URL → selection. selectedRuns drives both the checkboxes and the
  // compare-diagnostics fetch. Empty when no experiment is picked.
  const initialRunIds = useMemo(() => {
    const raw = searchParams.get('runs') ?? '';
    return raw.split(',').map((id) => id.trim()).filter(Boolean);
  }, [searchParams]);
  const [selected, setSelected] = useState<Set<string>>(() => new Set(initialRunIds));

  // Re-sync local state when the URL changes from outside (back/forward,
  // share-link paste). We deliberately avoid pushing local state changes
  // back into the URL on every checkbox click — that would re-trigger
  // this effect in a loop. URL state is only pushed via the explicit
  // Share-link / Clear buttons or when the experiment is picked.
  useEffect(() => {
    setSelected(new Set(initialRunIds));
  }, [initialRunIds]);

  const { data: experiments = [], isLoading: experimentsLoading } = useExperiments();
  const { data: experimentRuns = [], isLoading: runsLoading } = useRuns(experimentID || undefined);

  const setExperiment = (next: string) => {
    // Switching experiments clears the run selection — the old ids would
    // never be valid against the new experiment's runs anyway.
    setSelected(new Set());
    setSearchParams(next ? { experiment: next } : {});
  };

  const toggleRun = (runId: string) => {
    setSelected((prev) => {
      const out = new Set(prev);
      if (out.has(runId)) out.delete(runId);
      else out.add(runId);
      return out;
    });
  };

  const selectAllCompleted = () => {
    setSelected(new Set(experimentRuns.filter((r) => r.status === 'completed').map((r) => r.id)));
  };
  const clearSelection = () => setSelected(new Set());

  const runIds = useMemo(() => Array.from(selected), [selected]);
  const overLimit = runIds.length > 5;

  const { data: diagnostics, isLoading: diagLoading, isError, error } = useCompareDiagnostics(
    overLimit ? [] : runIds,
  );
  const { data: grades } = useCompareGrades(overLimit ? [] : runIds);

  // Drop any diagnostic that lacks the fingerprint surface the charts
  // read from. A missing fingerprint indicates the diagnostic row was
  // written before the AgentDx schema landed; rendering would throw at
  // `s.diagnostic.fingerprint[key]` and the user would see a blank
  // page (no surrounding ErrorBoundary on a per-chart granularity).
  // Skipping these here keeps the surrounding charts working.
  const series = useMemo(() => {
    if (!diagnostics) return [];
    return diagnostics
      .map((diag, i) => ({
        label: shortLabel(experimentRuns, runIds[i] ?? `run-${i + 1}`),
        diagnostic: diag,
      }))
      .filter((entry): entry is { label: string; diagnostic: Diagnostic } => isValidDiagnostic(entry.diagnostic));
  }, [diagnostics, runIds, experimentRuns]);

  const droppedDiagnostics =
    diagnostics !== undefined && diagnostics.length > series.length
      ? diagnostics.length - series.length
      : 0;

  const shareLink = () => {
    const params: Record<string, string> = {};
    if (experimentID) params.experiment = experimentID;
    if (runIds.length > 0) params.runs = runIds.join(',');
    setSearchParams(params);
  };

  return (
    <div className="space-y-6">
      <Card>
        <div className="flex items-start justify-between gap-3">
          <CardHeader
            title="Diagnostic Compare"
            description="Pick an experiment, then choose 2–5 runs to compare their AgentDx profiles side-by-side."
          />
          <Link to="/diagnostic/launch">
            <Button size="sm">New diagnostic run</Button>
          </Link>
        </div>

        <div className="mt-3 grid gap-3 md:grid-cols-[280px_1fr]">
          <label className="flex flex-col gap-1 text-xs text-fg-muted">
            Experiment
            <select
              value={experimentID}
              onChange={(e) => setExperiment(e.target.value)}
              className="rounded-md border border-border bg-bg-elev-1 px-2 py-1.5 text-sm text-fg"
              disabled={experimentsLoading}
            >
              <option value="">{experimentsLoading ? 'Loading…' : '— Select experiment —'}</option>
              {experiments.map((exp) => (
                <option key={exp.id} value={exp.id}>
                  {exp.name} · {exp.agent_cli} ({exp.runs_per_variant * (exp.variants?.length ?? 0)} runs)
                </option>
              ))}
            </select>
          </label>

          <div className="flex flex-col gap-1 text-xs text-fg-muted">
            <div className="flex items-baseline justify-between">
              <span>Runs ({runIds.length} selected{overLimit ? ' — too many' : ''})</span>
              {experimentID && experimentRuns.length > 0 && (
                <span className="flex gap-2">
                  <button
                    type="button"
                    onClick={selectAllCompleted}
                    className="rounded-sm border border-border bg-bg-elev-2 px-2 py-0.5 text-xs text-fg-muted hover:bg-bg-elev-1"
                  >
                    Select all completed
                  </button>
                  <button
                    type="button"
                    onClick={clearSelection}
                    className="rounded-sm border border-border bg-bg-elev-2 px-2 py-0.5 text-xs text-fg-muted hover:bg-bg-elev-1"
                  >
                    Clear
                  </button>
                </span>
              )}
            </div>
            <RunPicker
              experimentId={experimentID}
              runs={experimentRuns}
              isLoading={runsLoading}
              selected={selected}
              onToggle={toggleRun}
            />
          </div>
        </div>

        <div className="mt-3 flex items-center justify-between gap-2">
          <span className="text-xs text-fg-muted">
            {overLimit
              ? 'Pick at most 5 runs. The five charts get noisy beyond that.'
              : runIds.length < 2 && runIds.length > 0
              ? 'Pick at least one more run to compare.'
              : ''}
          </span>
          <Button size="sm" variant="secondary" onClick={shareLink} disabled={runIds.length === 0}>
            Share link
          </Button>
        </div>
      </Card>

      {!experimentID ? (
        <Card>
          <EmptyState
            title="Pick an experiment to start"
            description="Compare runs side-by-side: deterministic grader metrics + behavioral diagnostic profile. Choose an experiment above, then tick at least two of its runs."
          />
        </Card>
      ) : runIds.length === 0 ? (
        <Card>
          <EmptyState
            title="Pick runs to compare"
            description="Tick at least two completed runs from the list above. Use 'Select all completed' for the common case."
          />
        </Card>
      ) : overLimit ? (
        <Card>
          <div className="text-sm text-warning-fg">Too many runs selected. The compare view tops out at 5.</div>
        </Card>
      ) : (
        <>
          {/* Grade comparison renders independently of the diagnostic
              profile — grader writes its row as soon as the run
              finishes, while the diagnostic stage is a separate
              downstream step that can lag or fail. Showing grades
              even when diagnostic is missing is the difference
              between "useful for thesis writing" and "blank page". */}
          <Card>
            <CardHeader
              title="Grade comparison"
              description="Deterministic scores from the grader sidecar — code, judge, spec, and process metrics side-by-side."
            />
            <GradeComparisonTable runIds={runIds} runs={experimentRuns} grades={grades ?? []} />
          </Card>

          <Card>
            <CardHeader
              title="Test results"
              description="Verification cases the grader ran inside the sandbox. Click a failing row to expand its output."
            />
            <TestResultsTable runIds={runIds} runs={experimentRuns} grades={grades ?? []} />
          </Card>

          {/* Behavioral diagnostic charts. These need fingerprint /
              symptoms / recovery rows in the diagnostic table,
              which the diagnostic-extraction step writes after
              grading. If they're missing we render a small notice
              instead of swallowing the whole page. */}
          {diagLoading ? (
            <Card>
              <div className="flex h-32 items-center justify-center text-sm text-fg-muted">
                Loading behavioral diagnostics…
              </div>
            </Card>
          ) : isError ? (
            <Card>
              <div className="text-sm text-danger-fg">
                Failed to load diagnostic profiles.
                {error instanceof Error ? ` (${error.message})` : ''}
              </div>
            </Card>
          ) : series.length === 0 ? (
            <Card>
              <div className="text-sm text-fg-muted">
                Behavioral diagnostic profile not yet available for the selected runs — the
                diagnostic stage runs after grading and may still be in progress.
                Grade metrics above are independent and ready now.
              </div>
            </Card>
          ) : (
            <ErrorBoundary
              title="A chart failed to render"
              description="One of the diagnostic charts threw while drawing this selection. Try a different set of runs or refresh."
            >
              {droppedDiagnostics > 0 && (
                <Card>
                  <div className="text-xs text-warning-fg">
                    Skipped {droppedDiagnostics} run{droppedDiagnostics === 1 ? '' : 's'} with incomplete diagnostic data.
                  </div>
                </Card>
              )}
              <div className="grid gap-4 lg:grid-cols-2">
                <Card>
                  <CardHeader
                    title="Behavioral fingerprint"
                    description="9 of the 10 fingerprint dimensions overlaid; recovery latency is unbounded (turn count) so it appears in the recovery timeline below instead of on this normalized radar."
                  />
                  <BehavioralRadar series={series} />
                </Card>
                <Card>
                  <CardHeader
                    title="Failure breakdown"
                    description="Stacked counts of primary + secondary failure labels per run."
                  />
                  <FailureBreakdown series={series} />
                </Card>
              </div>
              <Card>
                <CardHeader
                  title="Recovery timeline"
                  description="Error events along the run's turn axis; one row per run."
                />
                <RecoveryTimeline series={series} />
              </Card>
              <div className="grid gap-4 lg:grid-cols-2">
                <Card>
                  <CardHeader
                    title="Pass rate vs wall clock"
                    description="Higher / faster is better. Eyeball the Pareto frontier."
                  />
                  <CostQualityScatter series={series} />
                </Card>
                <Card>
                  <CardHeader
                    title="Transcript evidence"
                    description="Verbatim quotes the classifier latched onto, grouped by failure code."
                  />
                  <TranscriptEvidence series={series} />
                </Card>
              </div>
            </ErrorBoundary>
          )}
        </>
      )}
    </div>
  );
}

interface RunPickerProps {
  experimentId: string;
  runs: Array<{ id: string; run_number: number; status: string; variant_id: string; created_at?: string }>;
  isLoading: boolean;
  selected: Set<string>;
  onToggle: (id: string) => void;
}

function RunPicker({ experimentId, runs, isLoading, selected, onToggle }: RunPickerProps) {
  if (!experimentId) {
    return (
      <div className="rounded-md border border-dashed border-border bg-bg-elev-1 px-3 py-4 text-center text-xs text-fg-subtle">
        Pick an experiment to list its runs.
      </div>
    );
  }
  if (isLoading) {
    return (
      <div className="rounded-md border border-border bg-bg-elev-1 px-3 py-4 text-center text-xs text-fg-muted">
        Loading runs…
      </div>
    );
  }
  if (runs.length === 0) {
    return (
      <div className="rounded-md border border-dashed border-border bg-bg-elev-1 px-3 py-4 text-center text-xs text-fg-muted">
        This experiment has no runs yet.
      </div>
    );
  }
  return (
    <ul
      role="listbox"
      aria-label="Runs to compare"
      aria-multiselectable="true"
      className="max-h-64 overflow-y-auto rounded-md border border-border bg-bg-elev-1"
    >
      {runs.map((run) => {
        const isSelected = selected.has(run.id);
        return (
          <li
            key={run.id}
            role="option"
            aria-selected={isSelected}
            className="border-b border-border last:border-b-0"
          >
            <label className="flex cursor-pointer items-center gap-3 px-3 py-2 text-sm text-fg hover:bg-bg-elev-2">
              <input
                type="checkbox"
                checked={isSelected}
                onChange={() => onToggle(run.id)}
                className="h-4 w-4 accent-accent"
                aria-label={`Compare run ${run.run_number}`}
              />
              <span className="flex flex-1 items-center gap-2">
                <span className="font-mono text-fg-muted">#{run.run_number}</span>
                <Badge tone={statusTone(run.status)}>{statusLabel(run.status)}</Badge>
                <span className="font-mono text-xs text-fg-subtle">{run.id.slice(0, 8)}…</span>
              </span>
              {run.created_at && (
                <span className="text-xs text-fg-subtle">{formatTimeAgo(run.created_at)}</span>
              )}
            </label>
          </li>
        );
      })}
    </ul>
  );
}

interface GradeComparisonTableProps {
  runIds: string[];
  runs: Array<{ id: string; run_number: number; status: string }>;
  grades: Array<Grade | null>;
}

/**
 * Side-by-side grade table for 2-5 selected runs. Each metric is one
 * row; each run is one column. Rows are grouped into four sections
 * matching the grader pipeline's structure (engine/grader/proto):
 *
 *   1. Code grading      — deterministic test/lint/type signals
 *   2. Process metrics   — transcript-derived behavior counters
 *   3. LLM-as-Judge      — cross-model rubric scores
 *   4. Spec adherence    — instruction-compliance checks
 *
 * Numeric metrics on a 0..1 scale render with a ScoreBar so deltas
 * read at a glance. composite_score is on 0..10 and gets a divide-by-
 * ten before being fed to the bar. Counts, booleans, and currency
 * render as plain mono-font cells. Missing data renders "—" rather
 * than a 0 so the user can tell "not graded yet" from "scored zero".
 */
// Plain-English definition for every metric row, surfaced as a
// title= tooltip on hover. Keeps the table itself tight while still
// making each axis self-explanatory; thesis readers no longer have
// to leave the page to figure out what "self-validation rate" means.
const METRIC_HELP: Record<string, string> = {
  Composite:
    'Weighted blend of code / judge / spec / process scores. 0..10. Weights come from the experiment\'s composite_weights (defaults 0.3 / 0.3 / 0.2 / 0.2).',
  'Test pass rate':
    'Fraction of the task\'s deterministic test_cases that exited 0. 0..1.',
  'Tests passed': 'Pass count / total count from the grader\'s deterministic test_cases.',
  'Lint score':
    'Static-analysis score from the grader (ruff/flake8 for Python, golangci-lint for Go, etc.). 0..10, higher is cleaner.',
  'Type check':
    'Did the workspace pass the language-native type checker (mypy, tsc, go vet)? Boolean.',
  'File state valid':
    'Did the agent leave the workspace in a state the grader could ingest — at least one output file produced, no broken imports.',
  Turns: 'Total ParsedTurn count emitted by the agent: thinking + tool_use + tool_result + text.',
  Tokens: 'Total tokens the model consumed across all turns (input + output combined).',
  'Cost (USD)':
    'Sum of priced tokens × the model_config rate. Local-model runs report 0.00.',
  'Token efficiency':
    'Code-quality outcome per 1k tokens. Penalises runs that burned tokens without progress. 0..1.',
  'Context utilization':
    'How much of the model\'s context window was actually filled. 0..1; very low values can indicate the harness under-grounded the agent.',
  'Tool call accuracy':
    'Fraction of tool_use events whose tool_result reported success (no error string, expected schema). 0..1.',
  'Self-validation rate':
    'How often the agent ran tests / lint / type checks itself before declaring done. 0..1.',
  Backtracks:
    'Number of times the agent reverted its own edit (write→re-write of the same file with overlapping ranges).',
  'Idle turns':
    'Turns that produced neither a tool call nor net progress (pure restating, navel-gazing). High counts → SLOP signal.',
  'Error recoveries':
    'Times the agent hit an error and produced a working follow-up. Higher means more resilient.',
  'Premature completion':
    'Did the agent declare done while at least one deterministic test was still failing? Boolean — true is bad.',
  Correctness: 'LLM-as-Judge: does the change correctly solve the stated problem? 0..1.',
  Maintainability: 'LLM-as-Judge: clarity, naming, structure. 0..1.',
  Completeness: 'LLM-as-Judge: did it cover the stated scope without leaving TODOs? 0..1.',
  'Best practices': 'LLM-as-Judge: idiomatic patterns for the language/framework. 0..1.',
  'Error handling': 'LLM-as-Judge: defensiveness, edge case coverage. 0..1.',
  'Inter-rater α':
    'Krippendorff\'s alpha across the judge\'s replicates. >0.67 = reliable; <0.5 = judge is wobbly, treat rubric numbers with caution.',
  'Instruction compliance':
    'Fraction of the task\'s explicit instructions the patch honored (e.g. "do not change signature"). 0..1.',
  'Convention adherence':
    'Code-style match against the repo\'s prevailing patterns (auto-detected from the codebase\'s anchor files).',
  'Constraint violations':
    'Number of explicit "don\'t" constraints the agent broke (touching forbidden files, changing public schema, etc.).',
};

function GradeComparisonTable({ runIds, runs, grades }: GradeComparisonTableProps) {
  if (runIds.length === 0) {
    return <div className="text-sm text-fg-muted">No runs selected.</div>;
  }
  const headers = runIds.map((id) => shortLabel(runs, id));

  return (
    <div className="overflow-x-auto">
      <table className="w-full text-sm">
        <thead>
          <tr className="border-b border-border text-left text-xs text-fg-muted">
            <th className="py-2 pr-3 font-medium">Metric</th>
            {headers.map((label, i) => (
              <th key={i} className="py-2 pr-3 font-medium">{label}</th>
            ))}
          </tr>
        </thead>
        <tbody>
          <HeadlineRow label="Composite" headers={headers} grades={grades} pick={(g) => g.composite_score} scale={10} />

          <SectionHeader colSpan={headers.length + 1} label="Code grading (deterministic)" />
          <BarRow label="Test pass rate" headers={headers} grades={grades} pick={(g) => g.test_pass_rate} />
          <NumericRow label="Tests passed" headers={headers} grades={grades} format={(g) => `${g.test_pass_count ?? 0} / ${(g.test_pass_count ?? 0) + (g.test_fail_count ?? 0)}`} />
          <BarRow label="Lint score" headers={headers} grades={grades} pick={(g) => g.lint_score} scale={10} />
          <BoolRow label="Type check" headers={headers} grades={grades} pick={(g) => g.type_check_pass} positive="pass" negative="fail" />
          <BoolRow label="File state valid" headers={headers} grades={grades} pick={(g) => g.file_state_valid} positive="ok" negative="broken" />

          <SectionHeader colSpan={headers.length + 1} label="Process metrics" />
          <NumericRow label="Turns" headers={headers} grades={grades} format={(g) => `${g.turn_count ?? '—'}`} />
          <NumericRow label="Tokens" headers={headers} grades={grades} format={(g) => g.total_tokens ? g.total_tokens.toLocaleString() : '—'} />
          <NumericRow label="Cost (USD)" headers={headers} grades={grades} format={(g) => g.cost_usd != null ? `$${g.cost_usd.toFixed(4)}` : '—'} />
          <BarRow label="Token efficiency" headers={headers} grades={grades} pick={(g) => g.token_efficiency} />
          <BarRow label="Context utilization" headers={headers} grades={grades} pick={(g) => g.context_utilization} />
          <BarRow label="Tool call accuracy" headers={headers} grades={grades} pick={(g) => g.tool_call_accuracy ?? 0} />
          <BarRow label="Self-validation rate" headers={headers} grades={grades} pick={(g) => g.self_validation_rate ?? 0} />
          <NumericRow label="Backtracks" headers={headers} grades={grades} format={(g) => `${g.backtrack_count ?? 0}`} />
          <NumericRow label="Idle turns" headers={headers} grades={grades} format={(g) => `${g.idle_turns ?? 0}`} />
          <NumericRow label="Error recoveries" headers={headers} grades={grades} format={(g) => `${g.error_recovery_count ?? 0}`} />
          <BoolRow label="Premature completion" headers={headers} grades={grades} pick={(g) => g.premature_completion} positive="yes" negative="no" tone="warn-positive" />

          <SectionHeader colSpan={headers.length + 1} label="LLM-as-Judge rubric" />
          <BarRow label="Correctness" headers={headers} grades={grades} pick={(g) => g.judge_correctness} />
          <BarRow label="Maintainability" headers={headers} grades={grades} pick={(g) => g.judge_maintainability ?? 0} />
          <BarRow label="Completeness" headers={headers} grades={grades} pick={(g) => g.judge_completeness ?? 0} />
          <BarRow label="Best practices" headers={headers} grades={grades} pick={(g) => g.judge_best_practices ?? 0} />
          <BarRow label="Error handling" headers={headers} grades={grades} pick={(g) => g.judge_error_handling ?? 0} />
          <NumericRow label="Inter-rater α" headers={headers} grades={grades} format={(g) => g.judge_irr_alpha != null ? g.judge_irr_alpha.toFixed(2) : '—'} />

          <SectionHeader colSpan={headers.length + 1} label="Spec adherence" />
          <BarRow label="Instruction compliance" headers={headers} grades={grades} pick={(g) => g.spec_instruction_compliance} />
          <BarRow label="Convention adherence" headers={headers} grades={grades} pick={(g) => g.spec_convention_adherence ?? 0} />
          <NumericRow label="Constraint violations" headers={headers} grades={grades} format={(g) => `${g.spec_constraint_violations ?? 0}`} />
        </tbody>
      </table>
    </div>
  );
}

// --- Row primitives ---------------------------------------------------

function SectionHeader({ label, colSpan }: { label: string; colSpan: number }) {
  return (
    <tr>
      <td
        colSpan={colSpan}
        className="border-b border-border bg-bg-elev-2/40 py-1.5 px-3 text-[10px] font-semibold uppercase tracking-[0.12em] text-fg-muted"
      >
        {label}
      </td>
    </tr>
  );
}

function HeadlineRow({
  label,
  headers,
  grades,
  pick,
  scale,
}: {
  label: string;
  headers: string[];
  grades: Array<Grade | null>;
  pick: (g: Grade) => number;
  scale?: number;
}) {
  return (
    <tr className="border-b border-border">
      <td
        className="cursor-help py-2 pr-3 font-medium text-fg decoration-dotted underline-offset-2 hover:underline"
        title={METRIC_HELP[label]}
      >
        {label}
      </td>
      {grades.map((g, i) => {
        if (!g) return <td key={i} className="py-2 pr-3 text-xs text-fg-subtle">—</td>;
        const raw = pick(g);
        const normalized = scale ? raw / scale : raw;
        return (
          <td key={i} className="py-2 pr-3">
            <div className="flex items-center gap-2">
              <ScoreBar value={normalized} label={`${label} ${headers[i]}`} />
              <span className="font-mono text-sm font-medium text-fg">{raw.toFixed(2)}</span>
            </div>
          </td>
        );
      })}
    </tr>
  );
}

function BarRow({
  label,
  headers,
  grades,
  pick,
  scale,
}: {
  label: string;
  headers: string[];
  grades: Array<Grade | null>;
  pick: (g: Grade) => number;
  scale?: number;
}) {
  return (
    <tr className="border-b border-border/60 last:border-b-0">
      <td
        className="cursor-help py-1.5 pr-3 text-fg-muted decoration-dotted underline-offset-2 hover:underline"
        title={METRIC_HELP[label]}
      >
        {label}
      </td>
      {grades.map((g, i) => {
        if (!g) return <td key={i} className="py-1.5 pr-3 text-xs text-fg-subtle">—</td>;
        const raw = pick(g);
        const normalized = scale ? raw / scale : raw;
        return (
          <td key={i} className="py-1.5 pr-3">
            <div className="flex items-center gap-2">
              <ScoreBar value={normalized} label={`${label} ${headers[i]}`} />
              <span className="font-mono text-xs text-fg">{raw.toFixed(2)}</span>
            </div>
          </td>
        );
      })}
    </tr>
  );
}

function NumericRow({
  label,
  headers: _,
  grades,
  format,
}: {
  label: string;
  headers: string[];
  grades: Array<Grade | null>;
  format: (g: Grade) => string;
}) {
  return (
    <tr className="border-b border-border/60 last:border-b-0">
      <td
        className="cursor-help py-1.5 pr-3 text-fg-muted decoration-dotted underline-offset-2 hover:underline"
        title={METRIC_HELP[label]}
      >
        {label}
      </td>
      {grades.map((g, i) => (
        <td key={i} className="py-1.5 pr-3 font-mono text-xs text-fg">
          {g ? format(g) : '—'}
        </td>
      ))}
    </tr>
  );
}

function BoolRow({
  label,
  grades,
  pick,
  positive,
  negative,
  tone = 'good-positive',
}: {
  label: string;
  headers: string[];
  grades: Array<Grade | null>;
  pick: (g: Grade) => boolean | undefined;
  positive: string;
  negative: string;
  // good-positive: true is the desirable outcome (type check passes → green)
  // warn-positive: true is the bad outcome (premature_completion → amber)
  tone?: 'good-positive' | 'warn-positive';
}) {
  return (
    <tr className="border-b border-border/60 last:border-b-0">
      <td
        className="cursor-help py-1.5 pr-3 text-fg-muted decoration-dotted underline-offset-2 hover:underline"
        title={METRIC_HELP[label]}
      >
        {label}
      </td>
      {grades.map((g, i) => {
        if (!g) return <td key={i} className="py-1.5 pr-3 font-mono text-xs text-fg-subtle">—</td>;
        const v = pick(g);
        if (v == null) return <td key={i} className="py-1.5 pr-3 font-mono text-xs text-fg-subtle">—</td>;
        const goodSide = tone === 'good-positive' ? v : !v;
        return (
          <td
            key={i}
            className={
              'py-1.5 pr-3 font-mono text-xs ' +
              (goodSide ? 'text-success-fg' : 'text-warning-fg')
            }
          >
            {v ? positive : negative}
          </td>
        );
      })}
    </tr>
  );
}

function shortLabel(runs: Array<{ id: string; run_number: number }>, runId: string): string {
  // Prefer the human-readable run_number ("Run 3") when we can find it
  // in the loaded experiment's run list. Falls back to the UUID prefix
  // for share-link round-trips where the run list hasn't loaded yet.
  const found = runs.find((r) => r.id === runId);
  if (found) return `Run ${found.run_number}`;
  return runId.length > 12 ? runId.slice(0, 8) + '…' : runId;
}

/**
 * The diagnostic charts read off `fingerprint`, `symptoms`, and
 * `recovery`. A diagnostic written before the AgentDx schema landed
 * can have those fields missing; treating such a row as "no
 * diagnostic" is friendlier than crashing the chart render.
 */
function isValidDiagnostic(diag: Diagnostic | null | undefined): diag is Diagnostic {
  if (!diag) return false;
  if (typeof diag.fingerprint !== 'object' || diag.fingerprint === null) return false;
  if (typeof diag.symptoms !== 'object' || diag.symptoms === null) return false;
  if (typeof diag.recovery !== 'object' || diag.recovery === null) return false;
  return true;
}

// --- Test results table ---------------------------------------------

/**
 * Side-by-side test verdicts. Rows are the unique test names across
 * the selected runs (union, not intersection — a run that never had a
 * test name renders "—" for that row); columns are the runs in pick
 * order. Pass/fail badge clicks toggle an inline output panel so a
 * failing case's traceback is one click away without leaving Compare.
 */
function TestResultsTable({ runIds, runs, grades }: {
  runIds: string[];
  runs: Array<{ id: string; run_number: number; status: string }>;
  grades: Array<Grade | null>;
}) {
  const [expandedKey, setExpandedKey] = useState<string | null>(null);
  if (runIds.length === 0) return <div className="text-sm text-fg-muted">No runs selected.</div>;

  const headers = runIds.map((id) => shortLabel(runs, id));
  // Build the union of test names while keeping a stable order: first
  // seen wins. Tests rarely change between runs but a re-grade can
  // introduce new ones; the union avoids losing them.
  const names: string[] = [];
  const seen = new Set<string>();
  for (const g of grades) {
    if (!g?.test_results) continue;
    for (const t of g.test_results) {
      if (!seen.has(t.name)) {
        seen.add(t.name);
        names.push(t.name);
      }
    }
  }
  if (names.length === 0) {
    return (
      <div className="text-sm text-fg-muted">
        No test cases recorded yet. The grader runs each task's verification
        scripts inside the sandbox; results land here once a run finishes
        the grading stage.
      </div>
    );
  }

  return (
    <div className="overflow-x-auto">
      <table className="w-full text-sm">
        <thead>
          <tr className="border-b border-border text-left text-xs text-fg-muted">
            <th className="py-2 pr-3 font-medium">Test case</th>
            {headers.map((label, i) => (
              <th key={i} className="py-2 pr-3 font-medium">{label}</th>
            ))}
          </tr>
        </thead>
        <tbody>
          {names.map((name) => {
            const cellEntries = grades.map((g) => g?.test_results?.find((t) => t.name === name) ?? null);
            const expanded = expandedKey === name;
            const anyFailedOutput = cellEntries.some((t) => t && !t.passed && t.output);
            return (
              <Fragment key={name}>
                <tr
                  className={cn(
                    'border-b border-border/60 last:border-b-0',
                    anyFailedOutput && 'cursor-pointer hover:bg-bg-elev-1/40',
                  )}
                  onClick={() => anyFailedOutput && setExpandedKey(expanded ? null : name)}
                >
                  <td className="py-1.5 pr-3 font-mono text-xs text-fg">
                    {anyFailedOutput && (
                      <span className="mr-1.5 select-none text-fg-subtle">
                        {expanded ? '▾' : '▸'}
                      </span>
                    )}
                    {name}
                  </td>
                  {cellEntries.map((t, i) => (
                    <td key={i} className="py-1.5 pr-3 font-mono text-xs">
                      {t == null ? (
                        <span className="text-fg-subtle">—</span>
                      ) : t.passed ? (
                        <span className="text-success-fg">✓ pass</span>
                      ) : (
                        <span className="text-danger-fg">✗ fail</span>
                      )}
                    </td>
                  ))}
                </tr>
                {expanded && (
                  <tr>
                    <td colSpan={headers.length + 1} className="bg-bg-elev-2/40 px-3 py-2">
                      <div className="grid gap-3" style={{ gridTemplateColumns: `repeat(${headers.length}, minmax(0, 1fr))` }}>
                        {cellEntries.map((t, i) => (
                          <div key={i} className="min-w-0">
                            <div className="mb-1 text-[10px] uppercase tracking-wider text-fg-muted">
                              {headers[i]}
                            </div>
                            {t == null ? (
                              <div className="text-xs text-fg-subtle">no result</div>
                            ) : t.passed ? (
                              <div className="text-xs text-success-fg">passed — no output</div>
                            ) : (
                              <pre className="max-h-48 overflow-auto whitespace-pre-wrap bg-code-bg/60 px-2 py-1.5 font-mono text-[11px] leading-[1.5] text-fg-muted">
                                {t.output || '(no output captured)'}
                              </pre>
                            )}
                          </div>
                        ))}
                      </div>
                    </td>
                  </tr>
                )}
              </Fragment>
            );
          })}
        </tbody>
      </table>
    </div>
  );
}
