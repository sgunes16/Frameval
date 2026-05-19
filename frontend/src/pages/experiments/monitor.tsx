import { useEffect, useMemo, useRef, useState } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { useParams } from 'react-router-dom';
import { CancelButton } from '../../components/run-monitor/cancel-button';
import { AgentEventViewer, LogViewer, type AgentLogEvent } from '../../components/run-monitor/log-viewer';
import { PatchViewer } from '../../components/run-monitor/patch-viewer';
import { RunProgressBar } from '../../components/run-monitor/progress-bar';
import { RunGrid } from '../../components/run-monitor/run-grid';
import { Badge } from '../../components/ui/badge';
import { Button } from '../../components/ui/button';
import { Card, CardHeader } from '../../components/ui/card';
import { api } from '../../lib/api';
import { useExperiment, useRuns, useTranscripts, useWebSocket } from '../../lib/hooks';
import type { Experiment, Run } from '../../lib/types';
import { statusLabel, statusTone } from '../../lib/utils';

export function ExperimentMonitorPage() {
  const { id } = useParams();
  const queryClient = useQueryClient();
  const { data: experiment } = useExperiment(id);
  const { data: runs = [] } = useRuns(id);
  const { events } = useWebSocket();
  const processedEventID = useRef(-1);
  const runIDs = useMemo(() => new Set(runs.map((run) => run.id)), [runs]);
  const liveLogEvents = useMemo(
    () => {
      return events
        .filter((event) => {
          if (event.type !== 'run.log' || !event.payload || typeof event.payload !== 'object') {
            return false;
          }
          const payload = event.payload as {
            experiment_id?: string;
            run_id?: string;
          };
          return payload.experiment_id === id || (payload.run_id ? runIDs.has(payload.run_id) : false);
        })
        .map((event): AgentLogEvent => {
          const payload = event.payload as {
            timestamp?: string;
            run_number?: number;
            stage?: string;
            line?: string;
          };
          return {
            id: event.id,
            line: payload.line ?? '',
            runId: (event.payload as { run_id?: string }).run_id,
            runNumber: payload.run_number,
            timestamp: payload.timestamp,
            stage: payload.stage,
          };
        });
    },
    [events, id, runIDs],
  );

  const finishedRunIds = useMemo(
    () => runs.filter((r) => r.status === 'completed' || r.status === 'failed').map((r) => r.id),
    [runs],
  );
  const { data: transcripts = [] } = useTranscripts(finishedRunIds);
  const transcriptLogEvents = useMemo(() => {
    const evts: AgentLogEvent[] = [];
    let nextId = -100000;
    for (const transcript of transcripts) {
      const run = runs.find((r) => r.id === transcript.run_id);
      const lines = transcript.raw_output.split('\n').filter((l) => l.trim());
      for (const line of lines) {
        evts.push({
          id: nextId++,
          line,
          runId: transcript.run_id,
          runNumber: run?.run_number,
          timestamp: run?.started_at,
          stage: 'transcript',
        });
      }
    }
    return evts;
  }, [transcripts, runs]);

  const allLogEvents = useMemo(() => {
    const seen = new Set<string>();
    const combined: AgentLogEvent[] = [];
    for (const event of [...transcriptLogEvents, ...liveLogEvents]) {
      const key = `${event.runId ?? ''}:${event.stage ?? ''}:${event.line}`;
      if (seen.has(key)) {
        continue;
      }
      seen.add(key);
      combined.push(event);
    }
    return combined;
  }, [liveLogEvents, transcriptLogEvents]);

  const [selectedRunId, setSelectedRunId] = useState<string | null>(null);

  const selectedRunIds = useMemo(() => {
    if (!selectedRunId) return null;
    return new Set([selectedRunId]);
  }, [selectedRunId]);

  const rawLogEvents = useMemo(
    () => selectedRunIds === null ? allLogEvents : allLogEvents.filter((e) => e.runId && selectedRunIds.has(e.runId)),
    [allLogEvents, selectedRunIds],
  );

  const filteredRuns = useMemo(
    () => selectedRunIds === null ? runs : runs.filter((r) => selectedRunIds.has(r.id)),
    [runs, selectedRunIds],
  );

  const logLines = useMemo(
    () =>
      rawLogEvents.map((event) => {
        const time = event.timestamp ? new Date(event.timestamp).toLocaleTimeString() : '--:--:--';
        const run = event.runId ? runs.find((r) => r.id === event.runId) : undefined;
        const variant = run ? experiment?.variants?.find((v) => v.id === run.variant_id) : undefined;
        const runLabel = variant ? `${variant.name} #${run?.run_number ?? ''}` : event.runNumber ? `run #${event.runNumber}` : 'run';
        const stageLabel = event.stage ?? 'executor';
        return `[${time}] ${runLabel} · ${stageLabel} · ${event.line}`;
      }),
    [rawLogEvents, runs, experiment],
  );

  const variantGroups = useMemo(() => {
    const variants = experiment?.variants ?? [];
    return variants.map((v) => ({
      variant: v,
      runs: runs.filter((r) => r.variant_id === v.id).sort((a, b) => a.run_number - b.run_number),
    }));
  }, [experiment, runs]);

  const selectedRunLabel = useMemo(() => {
    if (!selectedRunId) return 'All runs';
    const run = runs.find((r) => r.id === selectedRunId);
    if (!run) return 'Selected run';
    const variant = experiment?.variants?.find((v) => v.id === run.variant_id);
    return variant ? `${variant.name} · Run #${run.run_number}` : `Run #${run.run_number}`;
  }, [selectedRunId, runs, experiment]);

  const selectedTranscript = useMemo(() => {
    if (!selectedRunId) return undefined;
    return transcripts.find((transcript) => transcript.run_id === selectedRunId);
  }, [selectedRunId, transcripts]);

  const completed = runs.filter((run) => ['completed', 'failed'].includes(run.status)).length;

  useEffect(() => {
    if (!id) {
      return;
    }
    const nextEvents = events.filter((event) => event.id > processedEventID.current);
    if (!nextEvents.length) {
      return;
    }
    processedEventID.current = nextEvents[nextEvents.length - 1].id;

    for (const event of nextEvents) {
      if (!event.payload || typeof event.payload !== 'object') {
        continue;
      }
      const payload = event.payload as {
        experiment_id?: string;
        run_id?: string;
        status?: string;
      };

      if (event.type === 'run.status' && payload.run_id && payload.status) {
        queryClient.setQueryData<Run[]>(['runs', id], (current = []) =>
          current.map((run) => (run.id === payload.run_id ? { ...run, status: payload.status ?? run.status } : run)),
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
        void queryClient.invalidateQueries({ queryKey: ['experiment-stats', id] });
      }
    }
  }, [events, id, queryClient]);

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
        <div className="flex flex-wrap items-center gap-3">
          <button
            onClick={() => setSelectedRunId(null)}
            className={`rounded-lg px-3 py-1.5 text-xs font-medium transition ${selectedRunId === null ? 'bg-fg text-bg' : 'bg-bg-elev-2 text-fg-muted hover:bg-bg-elev-2'}`}
          >
            All runs
          </button>
          {variantGroups.map(({ variant, runs: variantRuns }) => (
            <div key={variant.id} className="flex items-center gap-1">
              <span className="text-xs font-medium uppercase tracking-wide text-fg-subtle">{variant.name}:</span>
              {variantRuns.map((run) => (
                <button
                  key={run.id}
                  onClick={() => setSelectedRunId(run.id)}
                  className={`rounded-lg px-2.5 py-1.5 text-xs font-medium transition ${selectedRunId === run.id ? 'bg-fg text-bg' : 'bg-bg-elev-2 text-fg-muted hover:bg-bg-elev-2'}`}
                >
                  #{run.run_number}
                  <span className={`ml-1 inline-block h-1.5 w-1.5 rounded-full ${run.status === 'completed' ? 'bg-success' : run.status === 'failed' ? 'bg-danger' : run.status === 'running' ? 'animate-pulse bg-warning' : 'bg-fg-subtle'}`} />
                </button>
              ))}
            </div>
          ))}
        </div>
      </Card>
      <Card>
        <CardHeader title="Agent timeline" description={selectedRunLabel} />
        <AgentEventViewer events={rawLogEvents} runs={filteredRuns} />
      </Card>
      <Card>
        <CardHeader title="Patch" description={selectedRunLabel} />
        <PatchViewer transcript={selectedTranscript} runLabel={selectedRunLabel} />
      </Card>
      <Card>
        <CardHeader title="Raw stream" description={selectedRunLabel} />
        <LogViewer lines={logLines} />
      </Card>
    </div>
  );
}
