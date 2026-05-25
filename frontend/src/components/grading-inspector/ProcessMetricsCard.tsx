import type { Grade } from '../../lib/types';
import { Card, CardHeader } from '../ui/card';

export function ProcessMetricsCard({ grade }: { grade: Grade }) {
  return (
    <Card>
      <CardHeader
        title="Process metrics"
        description="Heuristic transcript metrics."
      />
      <div className="grid grid-cols-2 gap-2 text-sm">
        <Row label="Turns" value={`${grade.turn_count ?? '—'}`} />
        <Row label="Tokens" value={grade.total_tokens ? grade.total_tokens.toLocaleString() : '—'} />
        <Row label="Cost (USD)" value={grade.cost_usd != null ? `$${grade.cost_usd.toFixed(4)}` : '—'} />
        <Row label="Backtracks" value={`${grade.backtrack_count ?? 0}`} />
        <Row label="Idle turns" value={`${grade.idle_turns ?? 0}`} />
        <Row label="Error recoveries" value={`${grade.error_recovery_count ?? 0}`} />
        <Row label="Token efficiency" value={fmtBar(grade.token_efficiency)} />
        <Row label="Context utilization" value={fmtBar(grade.context_utilization)} />
        <Row label="Tool call accuracy" value={fmtBar(grade.tool_call_accuracy ?? 0)} />
        <Row label="Self-validation rate" value={fmtBar(grade.self_validation_rate ?? 0)} />
        <Row label="Premature completion" value={grade.premature_completion ? 'yes' : 'no'} />
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

function fmtBar(v: number): string {
  return `${(v * 100).toFixed(0)}%`;
}
