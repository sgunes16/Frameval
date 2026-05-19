import type { Diagnostic, EvidenceSpan, FailureCode } from '../../lib/types';

export type TranscriptEvidenceSeries = {
  label: string;
  diagnostic: Diagnostic;
};

type Props = {
  series: TranscriptEvidenceSeries[];
};

/**
 * Renders the failure classifier's per-label evidence spans grouped by
 * FailureCode. Each entry shows the run label, the turn index, and the
 * verbatim quote the classifier latched onto. This is the "why" panel that
 * makes the categorical labels auditable — without it the failure breakdown
 * chart is just unfalsifiable numbers.
 */
export function TranscriptEvidence({ series }: Props) {
  const items: Array<{ runLabel: string; span: EvidenceSpan }> = [];
  for (const s of series) {
    const label = s.diagnostic.failure_label;
    if (!label) continue;
    for (const span of label.evidence ?? []) {
      items.push({ runLabel: s.label, span });
    }
  }

  if (items.length === 0) {
    return <EmptyState />;
  }

  // Group by FailureCode for scannability.
  const groups = new Map<FailureCode, Array<{ runLabel: string; span: EvidenceSpan }>>();
  for (const it of items) {
    const arr = groups.get(it.span.code) ?? [];
    arr.push(it);
    groups.set(it.span.code, arr);
  }

  return (
    <div className="space-y-4 rounded-lg border border-border bg-bg-elev-1 p-4">
      <div className="text-xs font-medium uppercase tracking-wider text-fg-muted">
        Per-failure evidence ({items.length} span{items.length === 1 ? '' : 's'})
      </div>
      {Array.from(groups.entries()).map(([code, spans]) => (
        <div key={code} className="space-y-1">
          <div className="text-xs font-semibold text-fg">{code}</div>
          {spans.map(({ runLabel, span }, i) => (
            <div
              key={`${runLabel}-${span.turn_index}-${i}`}
              className="rounded border border-border bg-bg-elev-2/70 p-2 text-xs leading-snug"
            >
              <div className="mb-1 flex items-center gap-2 text-xs text-fg-muted">
                <span className="font-medium text-fg">{runLabel}</span>
                <span>· turn {span.turn_index}</span>
              </div>
              <div className="font-mono text-fg">{span.quote}</div>
            </div>
          ))}
        </div>
      ))}
    </div>
  );
}

function EmptyState() {
  return (
    <div className="flex h-32 items-center justify-center rounded-lg border border-dashed border-border bg-bg-elev-2/50 text-sm text-fg-muted">
      No failure evidence yet. Evidence appears once the classifier runs on a completed run.
    </div>
  );
}
