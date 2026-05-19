import type { Diagnostic, ErrorEvent } from '../../lib/types';

const KIND_COLORS: Record<ErrorEvent['type'], string> = {
  tool_failure: '#dc2626',
  test_failure: '#7c3aed',
  stderr: '#ea580c',
  compile_error: '#0891b2',
};

export type RecoveryTimelineSeries = {
  label: string;
  diagnostic: Diagnostic;
};

type Props = {
  series: RecoveryTimelineSeries[];
};

/**
 * Gantt-style horizontal timeline showing error events per run.
 * Each row is one run; markers along the x-axis are error events colored
 * by ErrorKind. Hover over a marker to see the truncated error message.
 *
 * Width is normalized to the max turn count across the selected runs so
 * comparing position across rows reflects relative timing within a run.
 */
export function RecoveryTimeline({ series }: Props) {
  if (series.length === 0) {
    return <EmptyState />;
  }
  const maxTurn = Math.max(
    ...series.flatMap((s) =>
      (s.diagnostic.recovery.error_events ?? []).map((e) => e.turn_index),
    ),
    20, // floor so a near-empty timeline still has a sensible axis
  );

  return (
    <div className="space-y-2 rounded-lg border border-border bg-bg-elev-1 p-4">
      <div className="text-xs font-medium uppercase tracking-wider text-fg-muted">
        Error events by run · width normalized to turn index (max {maxTurn})
      </div>
      {series.map((s) => {
        const events = s.diagnostic.recovery.error_events ?? [];
        return (
          <div key={s.label} className="flex items-center gap-3">
            <div className="w-28 truncate text-xs font-medium text-fg" title={s.label}>
              {s.label}
            </div>
            <div className="relative h-6 flex-1 rounded bg-bg-elev-2">
              {events.length === 0 ? (
                <div className="absolute inset-0 flex items-center justify-center text-xs text-fg-subtle">
                  no errors
                </div>
              ) : (
                events.map((e, i) => {
                  const leftPct = Math.min(100, (e.turn_index / maxTurn) * 100);
                  return (
                    <div
                      key={`${e.turn_index}-${i}`}
                      title={`turn ${e.turn_index} · ${e.type}${e.tool_name ? ` (${e.tool_name})` : ''}: ${e.message}`}
                      className="absolute top-1 h-4 w-1.5 rounded-sm"
                      style={{
                        left: `${leftPct}%`,
                        backgroundColor: KIND_COLORS[e.type] ?? '#475569',
                      }}
                    />
                  );
                })
              )}
            </div>
            <div className="w-32 shrink-0 text-right text-xs text-fg-muted">
              {events.length} err · skip {s.diagnostic.recovery.silent_skip_count}
            </div>
          </div>
        );
      })}
      <RecoveryLegend />
    </div>
  );
}

function RecoveryLegend() {
  const entries: Array<[ErrorEvent['type'], string]> = [
    ['tool_failure', 'Tool failure'],
    ['test_failure', 'Test failure'],
    ['stderr', 'Stderr / traceback'],
    ['compile_error', 'Compile error'],
  ];
  return (
    <div className="mt-3 flex flex-wrap gap-3 border-t border-border pt-2 text-xs text-fg-muted">
      {entries.map(([kind, label]) => (
        <span key={kind} className="inline-flex items-center gap-1">
          <span className="inline-block h-2 w-2 rounded-sm" style={{ backgroundColor: KIND_COLORS[kind] }} />
          {label}
        </span>
      ))}
    </div>
  );
}

function EmptyState() {
  return (
    <div className="flex h-32 items-center justify-center rounded-lg border border-dashed border-border bg-bg-elev-2/50 text-sm text-fg-muted">
      No recovery data yet for the selected runs.
    </div>
  );
}
