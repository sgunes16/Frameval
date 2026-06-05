import type { Diagnostic } from '../../lib/types';

const DIMENSIONS: Array<{ key: keyof Diagnostic['fingerprint']; label: string }> = [
  { key: 'planning_depth', label: 'Planning' },
  { key: 'tool_call_diversity', label: 'Tool diversity' },
  { key: 'self_validation_rate', label: 'Self-validation' },
  { key: 'backtrack_rate', label: 'Backtrack' },
  { key: 'file_focus', label: 'File focus' },
  { key: 'premature_completion', label: 'Premature stop' },
  { key: 'turn_efficiency', label: 'Turn efficiency' },
  { key: 'context_reference_rate', label: 'Context refs' },
  { key: 'idle_thinking_ratio', label: 'Idle thinking' },
];

const SERIES_COLORS = ['#2563eb', '#16a34a', '#dc2626', '#ca8a04', '#7c3aed', '#0891b2', '#be185d'];

export type BehavioralRadarSeries = {
  label: string;
  diagnostic: Diagnostic;
};

type Props = {
  series: BehavioralRadarSeries[];
};

/**
 * Compact grid of mini bar-charts. Each cell = one dimension, each colored
 * segment = one variant. Fits in ~200px height regardless of series count.
 */
export function BehavioralRadar({ series }: Props) {
  if (series.length === 0) {
    return <EmptyState />;
  }

  return (
    <div className="space-y-3">
      {/* Legend — single row */}
      <div className="flex flex-wrap gap-x-4 gap-y-1 text-xs">
        {series.map((s, i) => (
          <div key={s.label} className="flex items-center gap-1.5">
            <div
              className="h-2 w-2 rounded-sm shrink-0"
              style={{ backgroundColor: SERIES_COLORS[i % SERIES_COLORS.length] }}
            />
            <span className="truncate max-w-36 text-fg-muted" title={s.label}>
              {s.label}
            </span>
          </div>
        ))}
      </div>

      {/* 3-column grid of dimension rows */}
      <div className="grid grid-cols-3 gap-x-6 gap-y-2">
        {DIMENSIONS.map(({ key, label }) => (
          <div key={key} className="space-y-1">
            <div className="text-xs font-medium text-fg">{label}</div>
            <div className="flex items-center gap-1 h-5">
              {series.map((s, i) => {
                const value = Number((s.diagnostic.fingerprint[key] ?? 0).toFixed(3));
                const pct = Math.round(value * 100);
                return (
                  <div
                    key={s.label}
                    className="h-3 rounded-sm transition-all duration-300"
                    style={{
                      width: `${Math.max(pct, 2)}%`,
                      backgroundColor: SERIES_COLORS[i % SERIES_COLORS.length],
                      opacity: 0.85,
                    }}
                    title={`${s.label}: ${pct}%`}
                  />
                );
              })}
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}

function EmptyState() {
  return (
    <div className="flex h-20 items-center justify-center rounded-lg border border-dashed border-border bg-bg-elev-2/50 text-xs text-fg-muted">
      Select 2+ runs to compare their behavioral fingerprints.
    </div>
  );
}
