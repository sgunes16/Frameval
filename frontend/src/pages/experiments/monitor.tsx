import { useMemo } from 'react';
import { useParams } from 'react-router-dom';
import { CancelButton } from '../../components/run-monitor/cancel-button';
import { LogViewer } from '../../components/run-monitor/log-viewer';
import { RunProgressBar } from '../../components/run-monitor/progress-bar';
import { RunGrid } from '../../components/run-monitor/run-grid';
import { Badge } from '../../components/ui/badge';
import { Button } from '../../components/ui/button';
import { Card, CardHeader } from '../../components/ui/card';
import { api } from '../../lib/api';
import { useExperiment, useRuns, useWebSocket } from '../../lib/hooks';
import { statusLabel, statusTone } from '../../lib/utils';

export function ExperimentMonitorPage() {
  const { id } = useParams();
  const { data: experiment } = useExperiment(id);
  const { data: runs = [] } = useRuns(id);
  const { events } = useWebSocket();
  const runIDs = useMemo(() => new Set(runs.map((run) => run.id)), [runs]);
  const logLines = useMemo(
    () =>
      events
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
        .map((event) => {
          const payload = event.payload as {
            timestamp?: string;
            run_number?: number;
            stage?: string;
            line?: string;
          };
          const time = payload.timestamp ? new Date(payload.timestamp).toLocaleTimeString() : '--:--:--';
          const runLabel = payload.run_number ? `run #${payload.run_number}` : 'run';
          const stageLabel = payload.stage ?? 'executor';
          return `[${time}] ${runLabel} · ${stageLabel} · ${payload.line ?? ''}`;
        }),
    [events, id, runIDs],
  );

  const completed = runs.filter((run) => ['completed', 'failed'].includes(run.status)).length;

  return (
    <div className="space-y-4">
      <Card>
        <div className="flex flex-wrap items-center justify-between gap-3">
          <div>
            <div className="text-lg font-semibold text-slate-900">{experiment?.name ?? 'Experiment monitor'}</div>
            <div className="mt-1 flex items-center gap-2 text-xs text-slate-500">
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
        <CardHeader title="Live logs" description="Streaming over WebSocket from the engine." />
        <LogViewer lines={logLines} />
      </Card>
    </div>
  );
}
