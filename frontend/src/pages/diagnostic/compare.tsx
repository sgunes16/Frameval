import { useEffect, useMemo, useState } from 'react';
import { Link, useSearchParams } from 'react-router-dom';

import { BehavioralRadar } from '../../components/diagnostic/behavioral-radar';
import { CostQualityScatter } from '../../components/diagnostic/cost-quality-scatter';
import { FailureBreakdown } from '../../components/diagnostic/failure-breakdown';
import { RecoveryTimeline } from '../../components/diagnostic/recovery-timeline';
import { TranscriptEvidence } from '../../components/diagnostic/transcript-evidence';
import { Badge } from '../../components/ui/badge';
import { Button } from '../../components/ui/button';
import { Card, CardHeader } from '../../components/ui/card';
import { EmptyState } from '../../components/ui/empty-state';
import { useCompareDiagnostics, useExperiments, useRuns } from '../../lib/hooks';
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

  const series = useMemo(() => {
    if (!diagnostics) return [];
    return diagnostics.map((diag, i) => ({
      label: shortLabel(experimentRuns, runIds[i] ?? `run-${i + 1}`),
      diagnostic: diag,
    }));
  }, [diagnostics, runIds, experimentRuns]);

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
            description="Diagnostic profiles compare runs against each other. Choose an experiment above, then tick at least two of its runs."
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
      ) : diagLoading ? (
        <Card>
          <div className="flex h-32 items-center justify-center text-sm text-fg-muted">Loading diagnostics…</div>
        </Card>
      ) : isError ? (
        <Card>
          <div className="text-sm text-danger-fg">
            Failed to load diagnostics. One or more runs may not have a diagnostic profile yet.
            {error instanceof Error ? ` (${error.message})` : ''}
          </div>
        </Card>
      ) : (
        <>
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

function shortLabel(runs: Array<{ id: string; run_number: number }>, runId: string): string {
  // Prefer the human-readable run_number ("Run 3") when we can find it
  // in the loaded experiment's run list. Falls back to the UUID prefix
  // for share-link round-trips where the run list hasn't loaded yet.
  const found = runs.find((r) => r.id === runId);
  if (found) return `Run ${found.run_number}`;
  return runId.length > 12 ? runId.slice(0, 8) + '…' : runId;
}
