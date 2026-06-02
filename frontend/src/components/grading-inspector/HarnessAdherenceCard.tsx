import type { Grade } from '../../lib/types';
import { Card, CardHeader } from '../ui/card';

interface AdherenceCheck {
  name: string;
  passed: boolean;
}

function parseChecks(json: string | undefined): AdherenceCheck[] | null {
  if (!json) return null;
  try {
    const parsed = JSON.parse(json);
    if (Array.isArray(parsed)) return parsed as AdherenceCheck[];
    return null;
  } catch {
    return null;
  }
}

export function HarnessAdherenceCard({ grade }: { grade: Grade }) {
  const score = grade.harness_adherence_score;
  const checks = parseChecks(grade.harness_adherence_json);
  const pct = score != null ? Math.max(0, Math.min(100, score * 100)) : null;

  const barColor =
    score == null
      ? 'bg-fg-muted'
      : score >= 0.7
        ? 'bg-success'
        : score >= 0.5
          ? 'bg-info'
          : score >= 0.3
            ? 'bg-warning'
            : 'bg-danger';

  return (
    <Card>
      <CardHeader
        title="Harness adherence"
        description="How well the agent followed the harness configuration and constraints."
      />

      {score != null ? (
        <div className="mb-3 flex items-center gap-3 text-sm">
          <div className="flex h-2 flex-1 overflow-hidden rounded bg-bg-elev-2">
            <div className={`h-full ${barColor}`} style={{ width: `${pct}%` }} />
          </div>
          <span className="w-16 text-right font-mono text-fg">
            {score.toFixed(2)} / 1.0
          </span>
        </div>
      ) : (
        <p className="mb-3 text-sm text-fg-muted">No score available.</p>
      )}

      {checks && checks.length > 0 && (
        <div className="space-y-1">
          <div className="text-xs uppercase tracking-wider text-fg-muted">Per-check results</div>
          <ul className="mt-1 space-y-0.5">
            {checks.map((check, i) => (
              <li key={i} className="flex items-center gap-2 text-sm">
                <span className={check.passed ? 'text-success' : 'text-danger'}>
                  {check.passed ? '✓' : '✗'}
                </span>
                <span className={check.passed ? 'text-fg' : 'text-fg-muted'}>{check.name}</span>
              </li>
            ))}
          </ul>
        </div>
      )}
    </Card>
  );
}
