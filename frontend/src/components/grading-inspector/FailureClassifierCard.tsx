import { Link } from 'react-router-dom';
import type { Diagnostic } from '../../lib/types';
import { FAILURE_DESCRIPTIONS } from '../../lib/failure-codes';
import { Badge } from '../ui/badge';
import { Card, CardHeader } from '../ui/card';

export function FailureClassifierCard({
  diagnostic,
  runId,
}: {
  diagnostic: Diagnostic | undefined;
  runId: string;
}) {
  const cls = diagnostic?.failure_label;
  if (!cls || cls.primary === 'NONE') {
    return (
      <Card>
        <CardHeader
          title="Failure classifier"
          description="LLM-driven failure categorization across 12 codes."
        />
        <div className="text-sm text-fg-muted">No failure classified for this run.</div>
      </Card>
    );
  }
  return (
    <Card>
      <CardHeader
        title="Failure classifier"
        description="LLM-driven failure categorization across 12 codes."
      />
      <div className="space-y-3 text-sm">
        <div className="flex items-center gap-2">
          <span title={FAILURE_DESCRIPTIONS[cls.primary]}>
            <Badge tone="danger">{cls.primary}</Badge>
          </span>
          {(cls.secondary ?? []).map((c) => (
            <span key={c} title={FAILURE_DESCRIPTIONS[c]}>
              <Badge tone="muted">{c}</Badge>
            </span>
          ))}
          {cls.confidence != null && (
            <span className="text-xs text-fg-muted">
              confidence: <span className="font-mono">{cls.confidence.toFixed(2)}</span>
            </span>
          )}
        </div>
        {cls.rationale && <p className="text-fg">{cls.rationale}</p>}
        {(cls.evidence?.length ?? 0) > 0 && (
          <div className="border-t border-border pt-3">
            <div className="mb-1 text-xs uppercase tracking-wider text-fg-muted">Evidence</div>
            <ul className="space-y-1">
              {cls.evidence!.map((e, i) => (
                <li key={i} className="rounded border border-border bg-bg-elev-1 p-2">
                  <div className="mb-1 flex items-center gap-2 text-xs text-fg-muted">
                    <Badge tone="muted">{e.code}</Badge>
                    <Link
                      to={`/runs/${runId}/inspect?focus=${e.turn_index}`}
                      className="underline"
                    >
                      Turn {e.turn_index}
                    </Link>
                  </div>
                  {e.quote && <p className="font-mono text-xs text-fg">{e.quote}</p>}
                </li>
              ))}
            </ul>
          </div>
        )}
      </div>
    </Card>
  );
}
