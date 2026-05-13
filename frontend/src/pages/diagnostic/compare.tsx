import { useMemo, useState } from 'react';
import { useSearchParams } from 'react-router-dom';
import { BehavioralRadar } from '../../components/diagnostic/behavioral-radar';
import { FailureBreakdown } from '../../components/diagnostic/failure-breakdown';
import { RecoveryTimeline } from '../../components/diagnostic/recovery-timeline';
import { CostQualityScatter } from '../../components/diagnostic/cost-quality-scatter';
import { TranscriptEvidence } from '../../components/diagnostic/transcript-evidence';
import { Card, CardHeader } from '../../components/ui/card';
import { EmptyState } from '../../components/ui/empty-state';
import { useCompareDiagnostics } from '../../lib/hooks';

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
  const initialRunIds = useMemo(() => {
    const raw = searchParams.get('runs') ?? '';
    return raw.split(',').map((id) => id.trim()).filter(Boolean);
  }, [searchParams]);
  const [draftInput, setDraftInput] = useState(initialRunIds.join('\n'));

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
        <CardHeader
          title="Diagnostic Compare"
          description="Side-by-side AgentDx profile across 2–5 runs. Paste run IDs (one per line or comma-separated) to overlay them."
        />
        <textarea
          value={draftInput}
          onChange={(event) => setDraftInput(event.target.value)}
          className="mt-3 w-full rounded-lg border border-slate-200 px-3 py-2 font-mono text-sm"
          rows={3}
          placeholder="run-id-1, run-id-2, run-id-3"
        />
        <div className="mt-2 flex justify-end gap-2">
          <button
            type="button"
            className="rounded-md border border-slate-200 px-3 py-1 text-xs text-slate-600 hover:bg-slate-50"
            onClick={() => {
              setDraftInput('');
              setSearchParams({});
            }}
          >
            Clear
          </button>
          <button
            type="button"
            className="rounded-md bg-slate-900 px-3 py-1 text-xs font-medium text-white hover:bg-slate-800"
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
          <div className="flex h-32 items-center justify-center text-sm text-slate-500">Loading diagnostics…</div>
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
                description="9 normalized dimensions overlaid across the selected runs."
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
