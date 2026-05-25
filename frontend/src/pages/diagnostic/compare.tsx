import { Fragment, useEffect, useMemo, useRef, useState, type MutableRefObject } from 'react';

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
import {
  useCompareDiagnostics,
  useCompareGrades,
  useExperiments,
  useExperimentsForIds,
  useRuns,
  useRunsForExperiments,
} from '../../lib/hooks';
import type { Diagnostic, Experiment, Grade, Run } from '../../lib/types';
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

  // Two URL shapes: legacy `?experiment=X` (single) and new
  // `?experiments=X,Y,Z` (matrix from the launcher). Multi wins when
  // both are present so a copy-paste from a matrix launch doesn't
  // accidentally fall back to the first-only behaviour.
  const experimentIDs = useMemo<string[]>(() => {
    const multi = searchParams.get('experiments');
    if (multi) return multi.split(',').map((s) => s.trim()).filter(Boolean);
    const single = searchParams.get('experiment');
    return single ? [single] : [];
  }, [searchParams]);
  const isMatrix = experimentIDs.length > 1;

  const initialRunIds = useMemo(() => {
    const raw = searchParams.get('runs') ?? '';
    return raw.split(',').map((id) => id.trim()).filter(Boolean);
  }, [searchParams]);
  const [selected, setSelected] = useState<Set<string>>(() => new Set(initialRunIds));

  useEffect(() => {
    setSelected(new Set(initialRunIds));
  }, [initialRunIds]);

  const { data: experiments = [], isLoading: experimentsLoading } = useExperiments();
  // Single-experiment fast path uses the existing cached endpoint;
  // matrix mode fans out across the whole id list. Both produce the
  // same `experimentRuns` array shape downstream so the picker and
  // diagnostic queries don't have to care.
  const singleID = isMatrix ? undefined : experimentIDs[0];
  const { data: singleRuns = [], isLoading: singleRunsLoading } = useRuns(singleID);
  const { data: matrixBundles, isLoading: matrixLoading } = useRunsForExperiments(
    isMatrix ? experimentIDs : [],
  );
  const { data: matrixExperiments } = useExperimentsForIds(isMatrix ? experimentIDs : []);

  // Index from experimentId → Experiment so RunPicker can label rows
  // with the experiment's variant signature (harness/agent/model).
  const expIndex = useMemo<Map<string, Experiment>>(() => {
    const m = new Map<string, Experiment>();
    if (isMatrix && matrixExperiments) {
      for (const e of matrixExperiments) {
        if (e) m.set(e.id, e);
      }
    } else if (!isMatrix && experimentIDs[0]) {
      const e = experiments.find((x) => x.id === experimentIDs[0]);
      if (e) m.set(e.id, e);
    }
    return m;
  }, [isMatrix, matrixExperiments, experiments, experimentIDs]);

  // Flatten runs across all selected experiments; each row still
  // carries its `experiment_id` (it's already on the Run schema)
  // so the picker can show which variant a run belongs to.
  const experimentRuns: Run[] = useMemo(() => {
    if (isMatrix && matrixBundles) {
      return matrixBundles.flatMap((b) => b.runs);
    }
    return singleRuns;
  }, [isMatrix, matrixBundles, singleRuns]);
  const runsLoading = isMatrix ? matrixLoading : singleRunsLoading;

  // Auto-select all completed runs the first time a matrix lands —
  // the user usually launched N variants specifically to compare
  // them, so default to "all of them". Only fires once per URL,
  // gated on the user not having a pre-existing run selection.
  const autoSelectedRef = useUrlAutoSelect();
  useEffect(() => {
    if (!isMatrix) return;
    if (autoSelectedRef.current) return;
    if (initialRunIds.length > 0) return;
    if (experimentRuns.length === 0) return;
    const completed = experimentRuns.filter((r) => r.status === 'completed').map((r) => r.id);
    if (completed.length === 0) return;
    autoSelectedRef.current = true;
    // Cap at 5 to respect the chart-sanity ceiling.
    setSelected(new Set(completed.slice(0, 5)));
  }, [isMatrix, experimentRuns, initialRunIds.length, autoSelectedRef]);

  const setExperiment = (next: string) => {
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
    setSelected(
      new Set(experimentRuns.filter((r) => r.status === 'completed').map((r) => r.id).slice(0, 5)),
    );
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
        label: shortLabel(experimentRuns, runIds[i] ?? `run-${i + 1}`, expIndex),
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
    if (experimentIDs.length > 1) params.experiments = experimentIDs.join(',');
    else if (experimentIDs.length === 1) params.experiment = experimentIDs[0];
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
          <ExperimentPicker
            experiments={experiments}
            isLoading={experimentsLoading}
            selectedIDs={experimentIDs}
            onChange={(ids) => {
              setSelected(new Set());
              if (ids.length === 0) setSearchParams({});
              else if (ids.length === 1) setSearchParams({ experiment: ids[0] });
              else setSearchParams({ experiments: ids.join(',') });
            }}
          />

          <div className="flex flex-col gap-1 text-xs text-fg-muted">
            <div className="flex items-baseline justify-between">
              <span>Runs ({runIds.length} selected{overLimit ? ' — too many' : ''})</span>
              {experimentIDs.length > 0 && experimentRuns.length > 0 && (
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
              experimentId={experimentIDs[0] ?? ''}
              runs={experimentRuns}
              isLoading={runsLoading}
              selected={selected}
              onToggle={toggleRun}
              expIndex={expIndex}
              isMatrix={isMatrix}
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

      {experimentIDs.length === 0 ? (
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
            <GradeComparisonTable
              runIds={runIds}
              runs={experimentRuns}
              grades={grades ?? []}
              expIndex={expIndex}
            />
          </Card>

          <Card>
            <CardHeader
              title="Test results"
              description="Verification cases the grader ran inside the sandbox. Click a failing row to expand its output."
            />
            <TestResultsTable
              runIds={runIds}
              runs={experimentRuns}
              grades={grades ?? []}
              expIndex={expIndex}
            />
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
  runs: Array<{
    id: string;
    run_number: number;
    status: string;
    variant_id: string;
    experiment_id?: string;
    created_at?: string;
  }>;
  isLoading: boolean;
  selected: Set<string>;
  onToggle: (id: string) => void;
  expIndex?: Map<string, Experiment>;
  isMatrix?: boolean;
}

function RunPicker({
  experimentId,
  runs,
  isLoading,
  selected,
  onToggle,
  expIndex,
  isMatrix,
}: RunPickerProps) {
  if (!isMatrix && !experimentId) {
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
        {isMatrix ? 'No runs across the selected experiments yet.' : 'This experiment has no runs yet.'}
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
        const exp = run.experiment_id ? expIndex?.get(run.experiment_id) : undefined;
        const variantSig = exp ? variantSignature(exp) : null;
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
              <span className="flex flex-1 items-center gap-2 min-w-0">
                <span className="font-mono text-fg-muted">#{run.run_number}</span>
                <Badge tone={statusTone(run.status)}>{statusLabel(run.status)}</Badge>
                {variantSig && (
                  <span
                    className="truncate font-mono text-xs text-fg"
                    title={`harness/executor/model · ${variantSig}`}
                  >
                    {variantSig}
                  </span>
                )}
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

// Build a short `harness/executor/model` label out of an experiment.
// The launcher already encodes the matrix coordinates into the
// experiment's name (`"<prefix> · <harness>/<executor>/<model>"`),
// so we just pull the trailing `… · X` segment off; falling back to
// agent_cli + model when the name doesn't carry the convention.
function variantSignature(exp: Experiment): string {
  const dot = ' · ';
  const idx = exp.name.lastIndexOf(dot);
  if (idx >= 0) {
    const tail = exp.name.slice(idx + dot.length).trim();
    if (tail.includes('/')) return tail;
  }
  const harness = exp.variants?.[0]?.harness_id ?? exp.variants?.[0]?.name ?? '?';
  return `${harness}/${exp.agent_cli}/${exp.model}`;
}

interface GradeComparisonTableProps {
  runIds: string[];
  runs: Array<{ id: string; run_number: number; status: string; experiment_id?: string }>;
  grades: Array<Grade | null>;
  expIndex?: Map<string, Experiment>;
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

type Dim = 'harness' | 'agent' | 'model';
const DIM_LABEL: Record<Dim, string> = {
  harness: 'Harness',
  agent: 'Agent',
  model: 'Model',
};

function GradeComparisonTable({ runIds, runs, grades, expIndex }: GradeComparisonTableProps) {
  // The dimension order controls which axis groups outermost in the
  // multi-index header. Drag-to-reorder so the user can pivot the
  // comparison: "show me models grouped by harness" vs "show me
  // harnesses grouped by agent" without losing the underlying data.
  const [dimOrder, setDimOrder] = useState<Dim[]>(['harness', 'agent', 'model']);

  if (runIds.length === 0) {
    return <div className="text-sm text-fg-muted">No runs selected.</div>;
  }

  // Per-run coordinate in the (harness, agent, model) lattice. We
  // sort columns lexicographically by the user-chosen dim order so
  // the multi-index header's colspans stay contiguous; otherwise a
  // 4-variant matrix renders columns in insertion order and the
  // grouped headers look ragged.
  const rawHeaders = runIds.map((id) => shortLabel(runs, id, expIndex));
  const rawCoords = runIds.map((id) => {
    const run = runs.find((r) => r.id === id);
    const exp = run?.experiment_id ? expIndex?.get(run.experiment_id) : undefined;
    return {
      harness: exp?.variants?.[0]?.harness_id ?? exp?.variants?.[0]?.name ?? '—',
      agent: exp?.agent_cli ?? '—',
      model: exp?.model ?? '—',
    } satisfies Record<Dim, string>;
  });
  const colOrder = runIds
    .map((_, i) => i)
    .sort((a, b) => {
      for (const dim of dimOrder) {
        const cmp = rawCoords[a][dim].localeCompare(rawCoords[b][dim]);
        if (cmp !== 0) return cmp;
      }
      return 0;
    });
  const headers = colOrder.map((i) => rawHeaders[i]);
  const orderedGrades = colOrder.map((i) => grades[i]);
  const orderedRunIds = colOrder.map((i) => runIds[i]);
  const coords = colOrder.map((i) => rawCoords[i]);
  grades = orderedGrades;

  return (
    <div className="overflow-x-auto">
      <DimOrderBar order={dimOrder} onChange={setDimOrder} />
      <table className="w-full text-sm">
        <PivotHead dimOrder={dimOrder} coords={coords} headers={headers} />
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
          <tr>
            <td className="py-1 pl-3 text-xs text-fg-muted">details:</td>
            {orderedRunIds.map((runId, i) => (
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

// --- Multi-index header primitives -----------------------------------

/**
 * DimOrderBar renders the three pivot dimensions (Harness · Agent ·
 * Model) as drag-reorderable pills. The leftmost pill becomes the
 * outermost header row in the grade table, so reordering inverts the
 * grouping ("models grouped by harness" → "harnesses grouped by
 * model"). Native HTML5 drag-and-drop; no extra dependency.
 */
function DimOrderBar({
  order,
  onChange,
}: {
  order: Dim[];
  onChange: (next: Dim[]) => void;
}) {
  const [dragging, setDragging] = useState<number | null>(null);
  const move = (from: number, to: number) => {
    if (from === to || to < 0 || to >= order.length) return;
    const next = [...order];
    const [item] = next.splice(from, 1);
    next.splice(to, 0, item);
    onChange(next);
  };
  return (
    <div className="mb-2 flex items-center gap-2 text-xs text-fg-muted">
      <span className="uppercase tracking-wider">Group by</span>
      <ol className="flex flex-wrap items-center gap-1">
        {order.map((dim, i) => (
          <li key={dim} className="flex items-center gap-1">
            <button
              type="button"
              draggable
              onDragStart={() => setDragging(i)}
              onDragOver={(e) => e.preventDefault()}
              onDrop={() => {
                if (dragging !== null) move(dragging, i);
                setDragging(null);
              }}
              onDragEnd={() => setDragging(null)}
              className={cn(
                'inline-flex cursor-grab items-center gap-1 rounded-md border px-2 py-0.5 font-mono text-xs active:cursor-grabbing',
                dragging === i
                  ? 'border-accent bg-accent/10 text-accent'
                  : 'border-border bg-bg-elev-1 text-fg hover:border-border-strong',
              )}
              title="Drag to reorder, or use arrows to move"
            >
              <span aria-hidden className="text-fg-subtle">⋮⋮</span>
              <span>{DIM_LABEL[dim]}</span>
              <span className="ml-1 inline-flex gap-0.5">
                <button
                  type="button"
                  onClick={(e) => {
                    e.stopPropagation();
                    move(i, i - 1);
                  }}
                  disabled={i === 0}
                  aria-label={`Move ${DIM_LABEL[dim]} left`}
                  className="rounded-sm px-1 text-fg-subtle hover:text-fg disabled:opacity-30"
                >
                  ◀
                </button>
                <button
                  type="button"
                  onClick={(e) => {
                    e.stopPropagation();
                    move(i, i + 1);
                  }}
                  disabled={i === order.length - 1}
                  aria-label={`Move ${DIM_LABEL[dim]} right`}
                  className="rounded-sm px-1 text-fg-subtle hover:text-fg disabled:opacity-30"
                >
                  ▶
                </button>
              </span>
            </button>
            {i < order.length - 1 && <span className="text-fg-subtle">›</span>}
          </li>
        ))}
      </ol>
      <span className="ml-auto text-fg-subtle">
        outer ←→ inner · drag to pivot
      </span>
    </div>
  );
}

/**
 * PivotHead renders the multi-index `<thead>`: one row per dimension
 * in `dimOrder`, with consecutive same-value cells merged via
 * colspan. Last row is the innermost dim (one cell per column).
 * The "Metric" corner cell sits in the first column with rowSpan
 * matching the depth so it visually grounds the row labels below.
 */
function PivotHead({
  dimOrder,
  coords,
  headers,
}: {
  dimOrder: Dim[];
  coords: Array<Record<Dim, string>>;
  headers: string[];
}) {
  // Pre-compute, per row, the colspan groups: [{value, span, startCol}].
  const rows = dimOrder.map((dim) => {
    const groups: Array<{ value: string; span: number; startCol: number }> = [];
    coords.forEach((c, col) => {
      const last = groups[groups.length - 1];
      if (last && last.value === c[dim]) {
        last.span += 1;
      } else {
        groups.push({ value: c[dim], span: 1, startCol: col });
      }
    });
    return { dim, groups };
  });

  return (
    <thead>
      {rows.map((row, rowIdx) => (
        <tr key={row.dim} className="text-left text-xs text-fg-muted">
          {rowIdx === 0 ? (
            <th
              rowSpan={dimOrder.length}
              className="border-b border-border py-2 pr-3 align-bottom text-xs font-semibold uppercase tracking-wider text-fg-muted"
            >
              Metric
            </th>
          ) : null}
          {row.groups.map((g) => (
            // Every cell shows its dim/value. Adjacent same-values
            // were already collapsed into one colspan during the
            // grouping pass above, so we never render the same value
            // twice horizontally — a span=1 cell on the innermost
            // row genuinely has a distinct value worth showing.
            <th
              key={`${row.dim}-${g.startCol}`}
              colSpan={g.span}
              className={cn(
                'border-b border-border py-1.5 pr-3 font-medium',
                rowIdx === dimOrder.length - 1
                  ? 'border-b-2 border-b-border-strong text-fg'
                  : 'text-fg-muted',
                g.startCol > 0 && 'border-l border-border/60',
              )}
              title={`${DIM_LABEL[row.dim]}: ${g.value}`}
            >
              <span className="flex flex-col gap-0.5">
                <span className="text-xs uppercase tracking-wider text-fg-subtle">
                  {DIM_LABEL[row.dim]}
                </span>
                <span className="truncate font-mono text-xs text-fg">{g.value}</span>
              </span>
            </th>
          ))}
        </tr>
      ))}
      {/* eslint-disable-next-line @typescript-eslint/no-unused-vars */}
      {(() => {
        // headers prop is retained for callers that want a flat
        // fallback row label; we don't render it in MultiIndex mode
        // because the bottom row already names every column.
        return null;
      })()}
      <tr aria-hidden className="sr-only">
        {headers.map((h, i) => (
          <th key={i}>{h}</th>
        ))}
      </tr>
    </thead>
  );
}

// --- Row primitives ---------------------------------------------------

function SectionHeader({ label, colSpan }: { label: string; colSpan: number }) {
  return (
    <tr>
      <td
        colSpan={colSpan}
        className="border-b border-border bg-bg-elev-2/40 py-1.5 px-3 text-xs font-semibold uppercase tracking-[0.12em] text-fg-muted"
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

function shortLabel(
  runs: Array<{ id: string; run_number: number; experiment_id?: string }>,
  runId: string,
  expIndex?: Map<string, Experiment>,
): string {
  // Matrix mode: prefer the variant signature ("bare/opencode/Big
  // Pickle") so charts can be distinguished by what was actually
  // compared. Single-experiment mode: keep the cheap "Run N" label
  // since every variant shares the same harness/exec/model anyway.
  const found = runs.find((r) => r.id === runId);
  if (found && expIndex && found.experiment_id) {
    const exp = expIndex.get(found.experiment_id);
    if (exp) return variantSignature(exp);
  }
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
function TestResultsTable({ runIds, runs, grades, expIndex }: {
  runIds: string[];
  runs: Array<{ id: string; run_number: number; status: string; experiment_id?: string }>;
  grades: Array<Grade | null>;
  expIndex?: Map<string, Experiment>;
}) {
  const [expandedKey, setExpandedKey] = useState<string | null>(null);
  if (runIds.length === 0) return <div className="text-sm text-fg-muted">No runs selected.</div>;

  const headers = runIds.map((id) => shortLabel(runs, id, expIndex));
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
                            <div className="mb-1 text-xs uppercase tracking-wider text-fg-muted">
                              {headers[i]}
                            </div>
                            {t == null ? (
                              <div className="text-xs text-fg-subtle">no result</div>
                            ) : t.passed ? (
                              <div className="text-xs text-success-fg">passed — no output</div>
                            ) : (
                              <pre className="max-h-48 overflow-auto whitespace-pre-wrap bg-code-bg/60 px-2 py-1.5 font-mono text-xs leading-[1.5] text-fg-muted">
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

// useUrlAutoSelect tracks whether the "auto-select-all-completed"
// effect has already fired for the current URL. Without this gate
// the effect would race against the user's clicks and re-tick rows
// they intentionally cleared. The ref resets on remount, which is
// the right granularity — a back/forward to the same URL keeps the
// selection, a fresh navigation re-arms it.
function useUrlAutoSelect(): MutableRefObject<boolean> {
  return useRef(false);
}

interface ExperimentPickerProps {
  experiments: Experiment[];
  isLoading: boolean;
  selectedIDs: string[];
  onChange: (ids: string[]) => void;
}

/**
 * ExperimentPicker is a search-filtered checkbox list of experiments
 * that produces a multi-select URL. It replaces the old single-pick
 * dropdown so users can compare runs across the matrix that the
 * launcher just fanned out into N "1-variant" experiments — without
 * needing to know about the `?experiments=…` URL contract.
 */
function ExperimentPicker({ experiments, isLoading, selectedIDs, onChange }: ExperimentPickerProps) {
  const [query, setQuery] = useState('');
  const selected = useMemo(() => new Set(selectedIDs), [selectedIDs]);
  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase();
    if (!q) return experiments;
    return experiments.filter(
      (e) =>
        e.name.toLowerCase().includes(q) ||
        e.agent_cli.toLowerCase().includes(q) ||
        e.model.toLowerCase().includes(q),
    );
  }, [experiments, query]);

  const toggle = (id: string) => {
    const next = new Set(selected);
    if (next.has(id)) next.delete(id);
    else next.add(id);
    onChange(Array.from(next));
  };
  const allFilteredSelected =
    filtered.length > 0 && filtered.every((e) => selected.has(e.id));

  return (
    <div className="flex flex-col gap-1.5 text-xs text-fg-muted">
      <div className="flex items-baseline justify-between gap-2">
        <span>Experiments ({selectedIDs.length} selected)</span>
        {selectedIDs.length > 0 && (
          <button
            type="button"
            onClick={() => onChange([])}
            className="rounded-sm border border-border bg-bg-elev-2 px-2 py-0.5 text-xs text-fg-muted hover:bg-bg-elev-1"
          >
            Clear
          </button>
        )}
      </div>
      <input
        type="search"
        value={query}
        onChange={(e) => setQuery(e.target.value)}
        placeholder={isLoading ? 'Loading…' : 'Search by name, agent, model…'}
        className="rounded-md border border-border bg-bg-elev-1 px-2 py-1.5 text-sm text-fg placeholder:text-fg-subtle"
      />
      <div className="max-h-56 overflow-y-auto rounded-md border border-border bg-bg-elev-1">
        {filtered.length === 0 ? (
          <div className="px-3 py-3 text-center text-xs text-fg-subtle">
            {experiments.length === 0 ? 'No experiments yet.' : 'No matches.'}
          </div>
        ) : (
          <>
            {filtered.length > 1 && (
              <button
                type="button"
                onClick={() => {
                  if (allFilteredSelected) {
                    onChange(selectedIDs.filter((id) => !filtered.some((e) => e.id === id)));
                  } else {
                    const next = new Set(selectedIDs);
                    for (const e of filtered) next.add(e.id);
                    onChange(Array.from(next));
                  }
                }}
                className="block w-full border-b border-border px-3 py-1.5 text-left text-xs text-fg-subtle hover:bg-bg-elev-2 hover:text-fg"
              >
                {allFilteredSelected ? 'Untick all visible' : `Tick all ${filtered.length} visible`}
              </button>
            )}
            <ul role="listbox" aria-multiselectable="true">
              {filtered.map((exp) => {
                const isSelected = selected.has(exp.id);
                const totalRuns = exp.runs_per_variant * (exp.variants?.length ?? 0);
                return (
                  <li key={exp.id} role="option" aria-selected={isSelected} className="border-b border-border last:border-b-0">
                    <label className="flex cursor-pointer items-center gap-3 px-3 py-2 text-sm text-fg hover:bg-bg-elev-2">
                      <input
                        type="checkbox"
                        checked={isSelected}
                        onChange={() => toggle(exp.id)}
                        className="h-4 w-4 accent-accent"
                        aria-label={`Compare ${exp.name}`}
                      />
                      <span className="flex min-w-0 flex-1 flex-col gap-0.5">
                        <span className="truncate">{exp.name}</span>
                        <span className="truncate font-mono text-xs text-fg-subtle">
                          {exp.agent_cli} · {exp.model} · {totalRuns} run{totalRuns === 1 ? '' : 's'}
                        </span>
                      </span>
                    </label>
                  </li>
                );
              })}
            </ul>
          </>
        )}
      </div>
    </div>
  );
}
