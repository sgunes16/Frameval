import { useState } from 'react';
import type { Grade } from '../../lib/types';
import { Badge } from '../ui/badge';
import { Card, CardHeader } from '../ui/card';

export function LLMJudgeCard({ grade }: { grade: Grade }) {
  const [showRaw, setShowRaw] = useState(false);
  const scores = grade.judge_scores ?? {};
  const rationales = grade.judge_rationales ?? {};
  const dimEntries = Object.entries(scores);

  return (
    <Card>
      <CardHeader
        title="LLM-as-judge rubric"
        description="Multi-dimension judgment from the configured judge model."
      />

      <div className="space-y-1">
        {dimEntries.map(([key, value]) => (
          <DimRow key={key} label={prettyDim(key)} value={value} />
        ))}
      </div>

      {grade.judge_irr_alpha != null && grade.judge_irr_alpha > 0 && (
        <div className="mt-2 text-xs text-fg-muted">
          Inter-rater α: <span className="font-mono">{grade.judge_irr_alpha.toFixed(2)}</span>
        </div>
      )}

      {dimEntries.length > 0 && (
        <div className="mt-4 space-y-3 border-t border-border pt-4">
          <div className="text-xs uppercase tracking-wider text-fg-muted">
            Per-dimension rationale
          </div>
          <div className="grid gap-3 md:grid-cols-2">
            {dimEntries.map(([dim, score]) => (
              <RationaleCard
                key={dim}
                dim={dim}
                score={score}
                rationale={rationales[dim] ?? ''}
              />
            ))}
          </div>
        </div>
      )}

      {(grade.raw_judge_responses?.length ?? 0) > 0 && (
        <div className="mt-3 border-t border-border pt-3">
          <button
            className="text-xs text-fg-muted underline"
            onClick={() => setShowRaw((v) => !v)}
          >
            {showRaw ? 'hide raw response' : 'show raw response (debug)'}
          </button>
          {showRaw && (
            <pre className="mt-2 max-h-64 overflow-auto whitespace-pre-wrap rounded bg-bg-elev-2 p-2 text-xs text-fg-muted">
              {JSON.stringify(grade.raw_judge_responses, null, 2)}
            </pre>
          )}
        </div>
      )}
    </Card>
  );
}

function DimRow({ label, value }: { label: string; value: number }) {
  const pct = Math.max(0, Math.min(100, (value / 10) * 100));
  return (
    <div className="flex items-center gap-3 text-sm">
      <div className="w-32 text-fg-muted">{label}</div>
      <div className="flex h-2 flex-1 overflow-hidden rounded bg-bg-elev-2">
        <div className={`h-full ${barColorFor(value)}`} style={{ width: `${pct}%` }} />
      </div>
      <div className="w-12 text-right font-mono text-fg">{value.toFixed(2)}</div>
    </div>
  );
}

function RationaleCard({
  dim,
  score,
  rationale,
}: {
  dim: string;
  score: number;
  rationale: string;
}) {
  const isSentinel = rationale.startsWith('judge_unavailable:');
  return (
    <div className="flex flex-col gap-2 rounded-lg border border-border bg-bg-elev-1 p-3">
      <div className="flex items-center justify-between gap-2">
        <div className="text-sm font-medium capitalize text-fg">{prettyDim(dim)}</div>
        <Badge tone={toneFor(score)}>{score.toFixed(2)} / 10</Badge>
      </div>
      {rationale ? (
        isSentinel ? (
          <p className="font-mono text-xs text-fg-muted">{rationale}</p>
        ) : (
          <p className="text-sm leading-relaxed text-fg">{rationale}</p>
        )
      ) : (
        <p className="text-xs italic text-fg-muted">No rationale returned by the judge.</p>
      )}
    </div>
  );
}

function prettyDim(key: string): string {
  return key.replace(/_/g, ' ').replace(/^\w/, (c) => c.toUpperCase());
}

// Map a 0-10 score to a semantic tone for the score badge.
function toneFor(score: number): 'danger' | 'warning' | 'info' | 'success' {
  if (score < 3) return 'danger';
  if (score < 5) return 'warning';
  if (score < 7) return 'info';
  return 'success';
}

// Map a 0-10 score to a bar fill color class. Tied to the same thresholds
// as toneFor() so the bar and badge agree.
function barColorFor(score: number): string {
  if (score < 3) return 'bg-danger';
  if (score < 5) return 'bg-warning';
  if (score < 7) return 'bg-info';
  return 'bg-success';
}
