import type { Grade } from '../../lib/types';
import { Card, CardHeader } from '../ui/card';

export function ProcessMetricsCard({ grade }: { grade: Grade }) {
  return (
    <Card>
      <CardHeader
        title="Process metrics"
        description="Accurate transcript and token metrics."
      />
      <div className="grid grid-cols-2 gap-2 text-sm">
        <Row label="Turns" value={`${grade.turn_count ?? '—'}`} />
        <Row label="Tokens" value={grade.total_tokens ? grade.total_tokens.toLocaleString() : '—'} />
        <Row label="Cost (USD)" value={grade.cost_usd != null ? `$${grade.cost_usd.toFixed(4)}` : '—'} />
        <Row label="Tool calls" value={grade.tool_call_count != null ? `${grade.tool_call_count}` : '—'} />
        <Row
          label="Tool error rate"
          value={grade.tool_error_rate != null ? `${(grade.tool_error_rate * 100).toFixed(0)}%` : '—'}
        />
        <Row label="Ran validation" value={grade.ran_validation != null ? (grade.ran_validation ? 'yes' : 'no') : '—'} />
        <Row label="Backtracks" value={`${grade.backtrack_count ?? 0}`} />
      </div>
    </Card>
  );
}

function Row({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex justify-between gap-2 border-b border-border/40 py-1">
      <span className="text-fg-muted">{label}</span>
      <span className="font-mono text-fg">{value}</span>
    </div>
  );
}
