import { useState } from 'react';
import type { Grade } from '../../lib/types';
import { Badge } from '../ui/badge';
import { Card, CardHeader } from '../ui/card';

export function LLMJudgeCard({ grade, isGrading }: { grade: Grade; isGrading?: boolean }) {
  const [showRaw, setShowRaw] = useState(false);
  const [openDims, setOpenDims] = useState<Set<string>>(new Set());
  const scores = grade.judge_scores ?? {};
  const rationales = grade.judge_rationales ?? {};
  const dimEntries = Object.entries(scores);

  const toggle = (dim: string) =>
    setOpenDims((prev) => {
      const next = new Set(prev);
      next.has(dim) ? next.delete(dim) : next.add(dim);
      return next;
    });

  return (
    <Card>
      <CardHeader
        title="LLM-as-judge rubric"
        description="Multi-dimension judgment from the configured judge model."
      />

      {isGrading ? (
        <>
          <div className="space-y-2">
            {[0, 1, 2, 3, 4].map((i) => (
              <div key={i} className="flex items-center gap-3 text-sm">
                <div className="h-3 w-32 animate-pulse rounded bg-bg-elev-2" />
                <div className="h-2 flex-1 animate-pulse rounded bg-bg-elev-2" />
                <div className="h-3 w-12 animate-pulse rounded bg-bg-elev-2" />
              </div>
            ))}
          </div>
          <div className="mt-3 text-xs text-fg-muted">
            Judge in progress… (this can take 30-90s on free-tier models)
          </div>
        </>
      ) : (
        <>
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

          {grade.judge_user_prompt && (
            <details className="mb-3 rounded-lg border border-border bg-bg-elev-1 p-2">
              <summary className="cursor-pointer text-xs font-medium text-fg-muted">
                Prompt sent to judge ({grade.judge_user_prompt.length.toLocaleString()} chars)
              </summary>
              <pre className="mt-2 max-h-96 overflow-auto whitespace-pre-wrap rounded bg-bg-elev-2 p-2 font-mono text-xs text-fg">
                {grade.judge_user_prompt}
              </pre>
            </details>
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
                    open={openDims.has(dim)}
                    onToggle={() => toggle(dim)}
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
        </>
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
  open,
  onToggle,
}: {
  dim: string;
  score: number;
  rationale: string;
  open: boolean;
  onToggle: () => void;
}) {
  const isSentinel = rationale.startsWith('judge_unavailable:');
  return (
    <div className="rounded-lg border border-border bg-bg-elev-1">
      <button
        type="button"
        onClick={onToggle}
        className="flex w-full items-center justify-between gap-2 p-3 text-left"
        aria-expanded={open}
      >
        <div className="flex items-center gap-2">
          <span className="text-fg-muted">{open ? '▾' : '▸'}</span>
          <span className="text-sm font-medium capitalize text-fg">{prettyDim(dim)}</span>
        </div>
        <Badge tone={toneFor(score)}>{score.toFixed(2)} / 10</Badge>
      </button>
      {open && (
        <div className="border-t border-border px-3 pb-3 pt-2">
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
