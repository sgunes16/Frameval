import {
  CartesianGrid,
  ResponsiveContainer,
  Scatter,
  ScatterChart,
  Tooltip,
  XAxis,
  YAxis,
  ZAxis,
} from 'recharts';
import type { Diagnostic } from '../../lib/types';

export type CostQualitySeries = {
  label: string;
  diagnostic: Diagnostic;
};

type Props = {
  series: CostQualitySeries[];
};

/**
 * Pass-rate (y) vs wall-clock seconds (x) scatter. One point per selected
 * run, labeled with the harness/variant name. Lets viewers eyeball the
 * Pareto frontier at a glance — high-y/low-x is best.
 */
export function CostQualityScatter({ series }: Props) {
  if (series.length === 0) {
    return <EmptyState />;
  }
  const data = series.map((s) => {
    const { tests_passed, tests_total, wall_clock_seconds } = s.diagnostic.symptoms;
    const passRate = tests_total > 0 ? tests_passed / tests_total : 0;
    return {
      label: s.label,
      x: wall_clock_seconds,
      y: Number((passRate * 100).toFixed(1)),
    };
  });

  return (
    <div className="h-80 w-full">
      <ResponsiveContainer>
        <ScatterChart>
          <CartesianGrid strokeDasharray="3 3" stroke="#e2e8f0" />
          <XAxis
            dataKey="x"
            name="Wall clock (s)"
            type="number"
            tick={{ fontSize: 11, fill: '#475569' }}
            label={{ value: 'Wall clock (seconds)', position: 'insideBottom', offset: -4, fontSize: 11 }}
          />
          <YAxis
            dataKey="y"
            name="Pass rate %"
            domain={[0, 100]}
            tick={{ fontSize: 11, fill: '#475569' }}
            label={{ value: 'Pass rate (%)', angle: -90, position: 'insideLeft', fontSize: 11 }}
          />
          <ZAxis range={[120, 120]} />
          <Tooltip
            cursor={{ strokeDasharray: '3 3' }}
            content={({ active, payload }) =>
              active && payload && payload.length > 0 ? (
                <div className="rounded border border-border bg-bg-elev-1 px-3 py-2 text-xs shadow">
                  <div className="font-medium text-fg">{(payload[0].payload as { label: string }).label}</div>
                  <div className="text-fg-muted">pass: {(payload[0].payload as { y: number }).y}%</div>
                  <div className="text-fg-muted">wall: {(payload[0].payload as { x: number }).x}s</div>
                </div>
              ) : null
            }
          />
          <Scatter data={data} fill="#2563eb" />
        </ScatterChart>
      </ResponsiveContainer>
    </div>
  );
}

function EmptyState() {
  return (
    <div className="flex h-80 items-center justify-center rounded-lg border border-dashed border-border bg-bg-elev-2/50 text-sm text-fg-muted">
      No timing / pass-rate data yet for the selected runs.
    </div>
  );
}
