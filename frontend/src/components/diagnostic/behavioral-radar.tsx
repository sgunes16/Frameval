import {
  PolarAngleAxis,
  PolarGrid,
  PolarRadiusAxis,
  Radar,
  RadarChart,
  ResponsiveContainer,
  Legend,
  Tooltip,
} from 'recharts';
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

// recovery_latency is on a turn-count scale, not [0,1]. Display it elsewhere
// (in RecoveryTimeline) — the radar is for the 9 normalized dimensions so
// every series uses the same axis.

const SERIES_COLORS = ['#2563eb', '#16a34a', '#dc2626', '#ca8a04', '#7c3aed'];

export type BehavioralRadarSeries = {
  label: string;
  diagnostic: Diagnostic;
};

type Props = {
  series: BehavioralRadarSeries[];
};

/**
 * Overlaid radar showing the 9 normalized fingerprint dimensions for up to
 * 5 runs side-by-side. Recovery latency is excluded from this view because
 * its scale is unbounded (turn count) and would distort the [0,1] axis.
 */
export function BehavioralRadar({ series }: Props) {
  if (series.length === 0) {
    return <EmptyState />;
  }
  const data = DIMENSIONS.map(({ key, label }) => {
    const row: Record<string, string | number> = { dimension: label };
    series.forEach((s) => {
      row[s.label] = Number((s.diagnostic.fingerprint[key] ?? 0).toFixed(3));
    });
    return row;
  });

  return (
    <div className="h-80 w-full">
      <ResponsiveContainer>
        <RadarChart data={data} outerRadius="75%">
          <PolarGrid stroke="#e2e8f0" />
          <PolarAngleAxis dataKey="dimension" tick={{ fontSize: 11, fill: '#475569' }} />
          <PolarRadiusAxis angle={90} domain={[0, 1]} tick={{ fontSize: 10, fill: '#94a3b8' }} />
          {series.map((s, i) => (
            <Radar
              key={s.label}
              name={s.label}
              dataKey={s.label}
              stroke={SERIES_COLORS[i % SERIES_COLORS.length]}
              fill={SERIES_COLORS[i % SERIES_COLORS.length]}
              fillOpacity={0.15}
            />
          ))}
          <Tooltip />
          <Legend />
        </RadarChart>
      </ResponsiveContainer>
    </div>
  );
}

function EmptyState() {
  return (
    <div className="flex h-80 items-center justify-center rounded-lg border border-dashed border-slate-200 bg-slate-50/50 text-sm text-slate-500">
      Select 2+ runs to overlay their behavioral fingerprints.
    </div>
  );
}
