import type { Grade, Run } from '../../lib/types';
import { Badge } from '../ui/badge';
import { Button } from '../ui/button';
import { Card, CardHeader } from '../ui/card';

export function GradingHeader({
  run,
  grade,
  onRegrade,
  regradeBusy,
}: {
  run: Run;
  grade: Grade;
  onRegrade: () => void;
  regradeBusy: boolean;
}) {
  const isFallback = grade.source === 'fallback';
  return (
    <Card>
      <CardHeader
        title={`Composite score: ${grade.composite_score?.toFixed(2) ?? '—'}`}
        description={`Run ${run.id} · variant ${run.variant_id} · status ${run.status}`}
      />
      <div className="flex items-center justify-between gap-2">
        <div className="flex items-center gap-2 text-xs">
          {isFallback ? (
            <Badge tone="danger">source: fallback (grader unreachable)</Badge>
          ) : (
            <Badge tone="success">source: grader</Badge>
          )}
          <span className="text-fg-muted">
            graded at {grade.created_at ?? '—'}
          </span>
        </div>
        <Button onClick={onRegrade} disabled={regradeBusy}>
          {regradeBusy ? 'Regrading…' : 'Regrade'}
        </Button>
      </div>
    </Card>
  );
}
