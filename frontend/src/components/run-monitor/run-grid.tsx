import type { Run } from '../../lib/types';
import { Badge } from '../ui/badge';
import { Card } from '../ui/card';
import { statusLabel, statusTone } from '../../lib/utils';

export function RunGrid({ runs }: { runs: Run[] }) {
  if (!runs.length) {
    return null;
  }
  return (
    <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-3">
      {runs.map((run) => (
        <Card key={run.id}>
          <div className="flex items-start justify-between gap-2">
            <div>
              <div className="text-xs uppercase tracking-wider text-fg-muted">Run</div>
              <div className="mt-0.5 text-sm font-semibold text-fg">#{run.run_number}</div>
            </div>
            <Badge tone={statusTone(run.status)}>{statusLabel(run.status)}</Badge>
          </div>
          <div className="mt-3 text-[11px] text-fg-muted">
            {run.duration_seconds ? `${run.duration_seconds.toFixed(1)}s` : 'Pending duration'}
          </div>
        </Card>
      ))}
    </div>
  );
}
