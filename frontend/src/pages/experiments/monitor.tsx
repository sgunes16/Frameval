import { useEffect, useMemo, useRef } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { Link, useParams } from 'react-router-dom';

import { CancelButton } from '../../components/run-monitor/cancel-button';
import { RunProgressBar } from '../../components/run-monitor/progress-bar';
import { RunGrid } from '../../components/run-monitor/run-grid';
import { Badge } from '../../components/ui/badge';
import { Button } from '../../components/ui/button';
import { Card, CardHeader } from '../../components/ui/card';
import { EmptyState } from '../../components/ui/empty-state';
import { api } from '../../lib/api';
import { useExperiment, useRuns, useWebSocket } from '../../lib/hooks';
import type { Experiment, Run } from '../../lib/types';
import { formatTimeAgo, statusLabel, statusTone } from '../../lib/utils';

/**
 * Experiment Monitor — the at-a-glance dashboard for a single
 * experiment. Shows progress, the variant×run grid, and a list of
 * runs each linking out to Inspector V2 (`/runs/:id/inspect`) for
 * the detailed turn-by-turn view.
 *
 * Earlier versions of this page inlined an agent-event timeline and
 * a raw log stream side-by-side. Both surfaces became redundant
 * once Inspector V2 shipped — the Inspector renders the same data
 * with proper turn grouping, diffs, and search. Linking out keeps
 * this page a 30-second overview rather than a noisy console dump.
 *
 * Live WS updates still flow into the run grid via cache
 * invalidation (the WS event handlers below); the user sees runs
 * transition to "completed" in real time without leaving this view.
 */
export function ExperimentMonitorPage() {
  const { id } = useParams();
  const queryClient = useQueryClient();
  const { data: experiment } = useExperiment(id);
  const { data: runs = [] } = useRuns(id);
  const { events } = useWebSocket();
  const processedEventID = useRef(-1);

  // Wire WS events into the React Query cache so the page reflects
  // live state without polling. Each handler updates the minimum
  // slice the UI needs; the heavier invalidate-all paths only fire
  // on terminal status transitions.
  useEffect(() => {
    if (!id) return;
    const nextEvents = events.filter((event) => event.id > processedEventID.current);
    if (!nextEvents.length) return;
    processedEventID.current = nextEvents[nextEvents.length - 1]!.id;

    for (const event of nextEvents) {
      if (!event.payload || typeof event.payload !== 'object') continue;
      const payload = event.payload as {
        experiment_id?: string;
        run_id?: string;
        status?: string;
      };

      if (event.type === 'run.status' && payload.run_id && payload.status) {
        queryClient.setQueryData<Run[]>(['runs', id], (current = []) =>
          current.map((run) =>
            run.id === payload.run_id ? { ...run, status: payload.status ?? run.status } : run,
          ),
        );
        if (payload.status === 'completed' || payload.status === 'failed') {
          void queryClient.invalidateQueries({ queryKey: ['runs', id] });
        }
      }

      if (event.type === 'run.progress' && payload.experiment_id === id) {
        void queryClient.invalidateQueries({ queryKey: ['runs', id] });
        void queryClient.invalidateQueries({ queryKey: ['experiment', id] });
      }

      if (event.type === 'experiment.status' && payload.experiment_id === id && payload.status) {
        queryClient.setQueryData<Experiment>(['experiment', id], (current) =>
          current ? { ...current, status: payload.status ?? current.status } : current,
        );
      }

      if (event.type === 'experiment.complete' && payload.experiment_id === id) {
        queryClient.setQueryData<Experiment>(['experiment', id], (current) =>
          current ? { ...current, status: 'completed' } : current,
        );
        void queryClient.invalidateQueries({ queryKey: ['runs', id] });
        void queryClient.invalidateQueries({ queryKey: ['experiment', id] });
      }
    }
  }, [events, id, queryClient]);

  const completed = runs.filter((run) => ['completed', 'failed'].includes(run.status)).length;

  // Group runs by variant for the per-variant run list section so
  // the user can scan "variant A: 5 runs, all completed" at a glance.
  const variantGroups = useMemo(() => {
    const variants = experiment?.variants ?? [];
    return variants.map((v) => ({
      variant: v,
      runs: runs.filter((r) => r.variant_id === v.id).sort((a, b) => a.run_number - b.run_number),
    }));
  }, [experiment, runs]);

  return (
    <div className="space-y-4">
      <Card>
        <div className="flex flex-wrap items-center justify-between gap-3">
          <div>
            <div className="text-lg font-semibold text-fg">{experiment?.name ?? 'Experiment monitor'}</div>
            <div className="mt-1 flex items-center gap-2 text-xs text-fg-muted">
              <Badge tone={statusTone(experiment?.status)}>{statusLabel(experiment?.status)}</Badge>
              <span>
                {experiment?.agent_cli} · {experiment?.model}
              </span>
            </div>
          </div>
          <div className="flex gap-2">
            {experiment && id && (
              <Link to={`/diagnostic/compare?experiment=${id}`}>
                <Button size="sm" variant="secondary">
                  Compare runs
                </Button>
              </Link>
            )}
            <Button size="sm" onClick={() => id && api.post(`/experiments/${id}/start`)}>
              Start
            </Button>
            <CancelButton onClick={() => id && api.post(`/experiments/${id}/cancel`)} />
          </div>
        </div>
      </Card>

      <Card>
        <CardHeader title="Progress" description={`${completed} / ${runs.length} runs finished.`} />
        <RunProgressBar completed={completed} total={runs.length} />
      </Card>

      <RunGrid runs={runs} />

      <Card>
        <CardHeader
          title="Runs"
          description="Click any run to open it in the Inspector for a turn-by-turn view."
        />
        {variantGroups.length === 0 ? (
          <EmptyState
            title="No runs yet"
            description="Once you start the experiment, every run will appear here with a link to its Inspector view."
          />
        ) : (
          <div className="space-y-4">
            {variantGroups.map(({ variant, runs: variantRuns }) => (
              <section key={variant.id}>
                <div className="mb-2 flex items-baseline justify-between">
                  <h3 className="font-mono text-sm font-medium text-fg">{variant.name}</h3>
                  <span className="text-xs text-fg-muted">
                    {variantRuns.filter((r) => r.status === 'completed').length} / {variantRuns.length} completed
                  </span>
                </div>
                {variantRuns.length === 0 ? (
                  <div className="rounded-md border border-dashed border-border bg-bg-elev-1 px-3 py-2 text-xs text-fg-muted">
                    No runs queued for this variant.
                  </div>
                ) : (
                  <ul className="overflow-hidden rounded-md border border-border bg-bg-elev-1">
                    {variantRuns.map((run) => (
                      <li key={run.id} className="border-b border-border last:border-b-0">
                        <Link
                          to={`/runs/${run.id}/inspect`}
                          className="flex items-center gap-3 px-3 py-2 text-sm text-fg transition hover:bg-bg-elev-2"
                        >
                          <span className="font-mono text-fg-muted">#{run.run_number}</span>
                          <Badge tone={statusTone(run.status)}>{statusLabel(run.status)}</Badge>
                          <span className="font-mono text-xs text-fg-subtle">{run.id.slice(0, 8)}…</span>
                          {run.started_at && (
                            <span className="text-xs text-fg-subtle">
                              started {formatTimeAgo(run.started_at)}
                            </span>
                          )}
                          <span className="ml-auto text-xs text-fg-muted">Open in Inspector →</span>
                        </Link>
                      </li>
                    ))}
                  </ul>
                )}
              </section>
            ))}
          </div>
        )}
      </Card>
    </div>
  );
}
