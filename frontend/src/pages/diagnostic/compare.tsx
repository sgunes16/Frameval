import { useEffect, useMemo, useState } from 'react';
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
 * row; each run is one column. ScoreBars on 0..1-scale metrics make
 * the deltas legible at a glance — operators can spot which variant
 * dominated which axis without reading numbers off a tooltip.
 *
 * composite_score is the only metric on a 0..10 scale (see backend
 * `composite()`); we divide by 10 before feeding ScoreBar.
 */
function GradeComparisonTable({ runIds, runs, grades }: GradeComparisonTableProps) {
  if (runIds.length === 0) {
    return <div className="text-sm text-fg-muted">No runs selected.</div>;
  }
  const headers = runIds.map((id) => shortLabel(runs, id));
  const metrics: Array<{ label: string; pick: (g: Grade) => number; scale?: number }> = [
    { label: 'Composite', pick: (g) => g.composite_score, scale: 10 },
    { label: 'Test pass rate', pick: (g) => g.test_pass_rate },
    { label: 'Judge correctness', pick: (g) => g.judge_correctness },
    { label: 'Spec compliance', pick: (g) => g.spec_instruction_compliance },
    { label: 'Tool call accuracy', pick: (g) => g.tool_call_accuracy ?? 0 },
    { label: 'Token efficiency', pick: (g) => g.token_efficiency },
  ];

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
          {metrics.map((m) => (
            <tr key={m.label} className="border-b border-border last:border-b-0">
              <td className="py-2 pr-3 text-fg-muted">{m.label}</td>
              {grades.map((g, i) => {
                if (!g) {
                  return <td key={i} className="py-2 pr-3 text-xs text-fg-subtle">—</td>;
                }
                const raw = m.pick(g);
                const normalized = m.scale ? raw / m.scale : raw;
                return (
                  <td key={i} className="py-2 pr-3">
                    <div className="flex items-center gap-2">
                      <ScoreBar value={normalized} label={`${m.label} ${headers[i]}`} />
                      <span className="font-mono text-xs text-fg">{raw.toFixed(2)}</span>
                    </div>
                  </td>
                );
              })}
            </tr>
          ))}
          <tr className="border-b border-border last:border-b-0">
            <td className="py-2 pr-3 text-fg-muted">Premature completion</td>
            {grades.map((g, i) => (
              <td key={i} className="py-2 pr-3 font-mono text-xs">
                {!g ? '—' : g.premature_completion ? (
                  <span className="text-warning-fg">yes</span>
                ) : (
                  <span className="text-success-fg">no</span>
                )}
              </td>
            ))}
          </tr>
          <tr>
            <td className="py-2 pr-3 text-fg-muted">Tests passed</td>
            {grades.map((g, i) => (
              <td key={i} className="py-2 pr-3 font-mono text-xs text-fg">
                {!g ? '—' : `${g.test_pass_count ?? 0} / ${(g.test_pass_count ?? 0) + (g.test_fail_count ?? 0)}`}
              </td>
            ))}
          </tr>
        </tbody>
      </table>
    </div>
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
