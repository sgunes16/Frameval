import { useEffect, useMemo, useRef, useState } from 'react';
import { Link, useSearchParams } from 'react-router-dom';
import { BehavioralRadar } from '../../components/diagnostic/behavioral-radar';
import { FailureBreakdown } from '../../components/diagnostic/failure-breakdown';
import { RecoveryTimeline } from '../../components/diagnostic/recovery-timeline';
import { CostQualityScatter } from '../../components/diagnostic/cost-quality-scatter';
import { TranscriptEvidence } from '../../components/diagnostic/transcript-evidence';
import { Button } from '../../components/ui/button';
import { Card, CardHeader } from '../../components/ui/card';
import { EmptyState } from '../../components/ui/empty-state';
import { useCompareDiagnostics, useRuns } from '../../lib/hooks';

/**
 * Diagnostic Compare page — the centerpiece of the AgentDx demo.
 *
 * Accepts a comma-separated list of run IDs via the `runs` query param,
 * fetches each run's Diagnostic Profile, and renders the 5 comparison
 * sub-views side-by-side. Run IDs are sourced from the Monitor page's
 * "Compare diagnostics" link (wired in a follow-up); for now operators
 * can paste run IDs directly into the input below.
 */
export function DiagnosticComparePage() {
  const [searchParams, setSearchParams] = useSearchParams();
  const experimentID = searchParams.get('experiment') ?? '';
  const initialRunIds = useMemo(() => {
    const raw = searchParams.get('runs') ?? '';
    return raw.split(',').map((id) => id.trim()).filter(Boolean);
  }, [searchParams]);
  const [draftInput, setDraftInput] = useState(initialRunIds.join('\n'));

  // When the URL carries ?experiment=<id>, auto-populate the run list from
  // that experiment's runs — ONCE. Subsequent poll ticks add newly-completed
  // runs to the list but never clobber edits the user has typed into the
  // textarea. Tracking via a ref so re-renders don't re-fire the effect.
  const { data: experimentRuns = [] } = useRuns(experimentID || undefined);
  const seenRunIds = useRef<Set<string>>(new Set());
  useEffect(() => {
    if (!experimentID || experimentRuns.length === 0) return;
    const fresh: string[] = [];
    for (const r of experimentRuns) {
      if (!seenRunIds.current.has(r.id)) {
        seenRunIds.current.add(r.id);
        fresh.push(r.id);
      }
    }
    if (fresh.length === 0) return;
    setDraftInput((prev) => {
      const lines = prev.split(/\r?\n/).map((s) => s.trim()).filter(Boolean);
      const merged = [...lines];
      for (const id of fresh) {
        if (!lines.includes(id)) merged.push(id);
      }
      return merged.join('\n');
    });
  }, [experimentID, experimentRuns]);

  const runIds = useMemo(
    () => draftInput.split(/[\n,]+/).map((s) => s.trim()).filter(Boolean),
    [draftInput],
  );

  const { data: diagnostics, isLoading, isError, error } = useCompareDiagnostics(runIds);

  const series = useMemo(() => {
    if (!diagnostics) return [];
    return diagnostics.map((diag, i) => ({
      label: shortLabel(runIds[i] ?? `run-${i + 1}`),
      diagnostic: diag,
    }));
  }, [diagnostics, runIds]);

  return (
    <div className="space-y-6">
      <Card>
        <div className="flex items-start justify-between gap-3">
          <CardHeader
            title="Diagnostic Compare"
            description="Side-by-side AgentDx profile across 2–5 runs. Pass ?experiment=<id> to auto-load every run from an experiment, or paste run IDs manually."
          />
          <Link to="/diagnostic/launch">
            <Button size="sm">New diagnostic run</Button>
          </Link>
        </div>
        {experimentID && (
          <div className="mb-2 rounded-md bg-bg-elev-2 px-3 py-2 text-xs text-fg-muted">
            Auto-loading runs from experiment{' '}
            <code className="font-mono">{experimentID.slice(0, 8)}…</code> ({experimentRuns.length}{' '}
            so far). The list updates as queued runs finish.
          </div>
        )}
        <textarea
          value={draftInput}
          onChange={(event) => setDraftInput(event.target.value)}
          className="mt-3 w-full rounded-lg border border-border px-3 py-2 font-mono text-sm"
          rows={3}
          placeholder="run-id-1, run-id-2, run-id-3"
        />
        <div className="mt-2 flex justify-end gap-2">
          <button
            type="button"
            className="rounded-md border border-border px-3 py-1 text-xs text-fg-muted hover:bg-bg-elev-2"
            onClick={() => {
              setDraftInput('');
              setSearchParams({});
            }}
          >
            Clear
          </button>
          <button
            type="button"
            className="rounded-md bg-fg px-3 py-1 text-xs font-medium text-white hover:bg-fg"
            onClick={() => setSearchParams({ runs: runIds.join(',') })}
          >
            Share link
          </button>
        </div>
      </Card>

      {runIds.length === 0 ? (
        <Card>
          <EmptyState
            title="Select runs to compare"
            description="Diagnostic profiles appear once you paste 2 or more run IDs above. The future Run Monitor page will provide a direct link here."
          />
        </Card>
      ) : isLoading ? (
        <Card>
          <div className="flex h-32 items-center justify-center text-sm text-fg-muted">Loading diagnostics…</div>
        </Card>
      ) : isError ? (
        <Card>
          <div className="text-sm text-rose-600">
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

function shortLabel(runID: string): string {
  // Run IDs are UUIDs — take a stable prefix so the chart legend stays
  // readable. 8 chars is enough to disambiguate across the 2–5-run set.
  return runID.length > 12 ? runID.slice(0, 8) + '…' : runID;
}
