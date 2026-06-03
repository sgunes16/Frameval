import { useState } from 'react';
import {
  Bar,
  BarChart,
  CartesianGrid,
  Cell,
  Legend,
  PolarAngleAxis,
  PolarGrid,
  PolarRadiusAxis,
  Radar,
  RadarChart,
  ResponsiveContainer,
  Scatter,
  ScatterChart,
  Tooltip,
  XAxis,
  YAxis,
  ZAxis,
} from 'recharts';
import { Card } from '../ui/card';
import { CardHeader } from '../ui/card';
import type { Grade } from '../../lib/types';

const SERIES_COLORS = [
  '#2563eb',
  '#16a34a',
  '#dc2626',
  '#ca8a04',
  '#7c3aed',
  '#0891b2',
  '#db2777',
  '#65a30d',
];

type Point = { label: string; grade: Grade };

/**
 * A selectable grade metric. `get` returns the value in display units (e.g.
 * pass rate as a percentage), or null when the grade lacks it. `max` pins the
 * axis domain for bounded metrics so bars/points are comparable across runs.
 */
type Metric = {
  key: string;
  label: string;
  get: (g: Grade) => number | null;
  max?: number;
  format: (v: number) => string;
};

function judgeAvg(g: Grade): number | null {
  const scores = Object.values(g.judge_scores ?? {});
  if (scores.length === 0) return null;
  return scores.reduce((a, b) => a + b, 0) / scores.length;
}

const METRICS: Metric[] = [
  { key: 'composite', label: 'Composite', get: (g) => g.composite_score ?? null, max: 10, format: (v) => v.toFixed(2) },
  { key: 'judge_avg', label: 'Judge avg', get: judgeAvg, max: 10, format: (v) => v.toFixed(2) },
  { key: 'lint', label: 'Lint score', get: (g) => g.lint_score ?? null, max: 10, format: (v) => v.toFixed(2) },
  {
    key: 'test_pass_rate',
    label: 'Test pass rate',
    get: (g) => (g.test_pass_rate != null ? g.test_pass_rate * 100 : null),
    max: 100,
    format: (v) => `${v.toFixed(0)}%`,
  },
  { key: 'harness_adherence', label: 'Harness adherence', get: (g) => g.harness_adherence_score ?? null, max: 1, format: (v) => v.toFixed(2) },
  { key: 'spec_compliance', label: 'Instruction compliance', get: (g) => g.spec_instruction_compliance ?? null, max: 1, format: (v) => v.toFixed(2) },
  { key: 'tokens', label: 'Tokens', get: (g) => g.total_tokens ?? null, format: (v) => v.toLocaleString() },
  { key: 'cost', label: 'Cost (USD)', get: (g) => g.cost_usd ?? null, format: (v) => `$${v.toFixed(4)}` },
  { key: 'turns', label: 'Turns', get: (g) => g.turn_count ?? null, format: (v) => `${v}` },
  { key: 'tool_calls', label: 'Tool calls', get: (g) => g.tool_call_count ?? null, format: (v) => `${v}` },
  {
    key: 'tool_error_rate',
    label: 'Tool error rate',
    get: (g) => (g.tool_error_rate != null ? g.tool_error_rate * 100 : null),
    max: 100,
    format: (v) => `${v.toFixed(0)}%`,
  },
  { key: 'backtracks', label: 'Backtracks', get: (g) => g.backtrack_count ?? null, format: (v) => `${v}` },
];

const prettyDim = (dim: string) => dim.replace(/_/g, ' ').replace(/^\w/, (c) => c.toUpperCase());

/** uniqueLabels suffixes duplicate column labels so recharts dataKeys don't collide. */
function uniqueLabels(points: Point[]): string[] {
  const seen = new Map<string, number>();
  return points.map((p) => {
    const n = (seen.get(p.label) ?? 0) + 1;
    seen.set(p.label, n);
    return n === 1 ? p.label : `${p.label} (${n})`;
  });
}

/**
 * GradeCharts renders the grade comparison visually: a judge-rubric radar and
 * a configurable metric explorer (bar or scatter with switchable axes), driven
 * by the same (headers, grades) the comparison table uses. Always available
 * once runs are graded — independent of the diagnostic stage.
 */
export function GradeCharts({ headers, grades }: { headers: string[]; grades: Array<Grade | null> }) {
  const points: Point[] = headers
    .map((label, i) => ({ label, grade: grades[i] }))
    .filter((p): p is Point => Boolean(p.grade));

  if (points.length === 0) return null;
  const labels = uniqueLabels(points);

  return (
    <div className="grid gap-4 lg:grid-cols-2">
      <Card>
        <CardHeader
          title="Judge rubric radar"
          description="LLM-as-judge dimension scores (0–10), one series per selected variant."
        />
        <RubricRadar points={points} labels={labels} />
      </Card>
      <Card>
        <CardHeader
          title="Metric explorer"
          description="Switch chart type and pick the axes to compare any metrics across variants."
        />
        <MetricExplorer points={points} labels={labels} />
      </Card>
    </div>
  );
}

function RubricRadar({ points, labels }: { points: Point[]; labels: string[] }) {
  const dims = Array.from(new Set(points.flatMap((p) => Object.keys(p.grade.judge_scores ?? {}))));
  if (dims.length === 0) return <ChartEmpty message="No LLM-judge rubric scores for the selected runs yet." />;

  const data = dims.map((dim) => {
    const row: Record<string, string | number> = { dimension: prettyDim(dim) };
    points.forEach((p, i) => {
      row[labels[i]] = Number((p.grade.judge_scores?.[dim] ?? 0).toFixed(2));
    });
    return row;
  });

  return (
    <div className="h-80 w-full">
      <ResponsiveContainer>
        <RadarChart data={data} outerRadius="70%">
          <PolarGrid stroke="#e2e8f0" />
          <PolarAngleAxis dataKey="dimension" tick={{ fontSize: 11, fill: '#475569' }} />
          <PolarRadiusAxis domain={[0, 10]} tick={{ fontSize: 10, fill: '#94a3b8' }} />
          {labels.map((label, i) => (
            <Radar
              key={label}
              name={label}
              dataKey={label}
              stroke={SERIES_COLORS[i % SERIES_COLORS.length]}
              fill={SERIES_COLORS[i % SERIES_COLORS.length]}
              fillOpacity={0.12}
            />
          ))}
          <Legend wrapperStyle={{ fontSize: 11 }} />
          <Tooltip
            content={({ active, payload, label }) =>
              active && payload && payload.length > 0 ? (
                <div className="rounded border border-border bg-bg-elev-1 px-3 py-2 text-xs shadow">
                  <div className="mb-1 font-medium text-fg">{label}</div>
                  {payload.map((entry) => (
                    <div key={String(entry.name)} className="text-fg-muted">
                      <span style={{ color: entry.color }}>●</span> {entry.name}: {entry.value}
                    </div>
                  ))}
                </div>
              ) : null
            }
          />
        </RadarChart>
      </ResponsiveContainer>
    </div>
  );
}

function MetricExplorer({ points, labels }: { points: Point[]; labels: string[] }) {
  const [type, setType] = useState<'bar' | 'scatter'>('bar');
  const [yKey, setYKey] = useState('composite');
  const [xKey, setXKey] = useState('cost');

  const yM = METRICS.find((m) => m.key === yKey) ?? METRICS[0];
  const xM = METRICS.find((m) => m.key === xKey) ?? METRICS[1];

  return (
    <div>
      <div className="mb-3 flex flex-wrap items-center gap-3 text-xs">
        <div className="flex items-center gap-1">
          {(['bar', 'scatter'] as const).map((t) => (
            <button
              key={t}
              type="button"
              onClick={() => setType(t)}
              className={
                type === t
                  ? 'rounded-full bg-fg px-3 py-1 font-medium text-bg'
                  : 'rounded-full border border-border bg-bg-elev-1 px-3 py-1 font-medium text-fg-muted hover:border-border-strong'
              }
            >
              {t === 'bar' ? 'Bar' : 'Scatter'}
            </button>
          ))}
        </div>
        {type === 'scatter' && <AxisSelect label="X" value={xKey} onChange={setXKey} />}
        <AxisSelect label={type === 'bar' ? 'Metric' : 'Y'} value={yKey} onChange={setYKey} />
      </div>

      {type === 'bar' ? (
        <BarMetric points={points} labels={labels} metric={yM} />
      ) : (
        <ScatterMetric points={points} labels={labels} xMetric={xM} yMetric={yM} />
      )}
    </div>
  );
}

function AxisSelect({ label, value, onChange }: { label: string; value: string; onChange: (v: string) => void }) {
  return (
    <label className="flex items-center gap-1.5 text-fg-muted">
      <span>{label}</span>
      <select
        value={value}
        onChange={(e) => onChange(e.target.value)}
        className="rounded-md border border-border bg-bg-elev-1 px-2 py-1 text-xs text-fg focus:border-border-strong focus:outline-none"
      >
        {METRICS.map((m) => (
          <option key={m.key} value={m.key}>
            {m.label}
          </option>
        ))}
      </select>
    </label>
  );
}

function BarMetric({ points, labels, metric }: { points: Point[]; labels: string[]; metric: Metric }) {
  const data = points.map((p, i) => ({ label: labels[i], value: metric.get(p.grade) ?? 0 }));
  return (
    <div className="h-80 w-full">
      <ResponsiveContainer>
        <BarChart data={data} margin={{ top: 8, right: 8, bottom: 8, left: 8 }}>
          <CartesianGrid strokeDasharray="3 3" stroke="#e2e8f0" vertical={false} />
          <XAxis dataKey="label" tick={{ fontSize: 10, fill: '#475569' }} interval={0} angle={-15} textAnchor="end" height={60} />
          <YAxis domain={metric.max ? [0, metric.max] : [0, 'auto']} tick={{ fontSize: 11, fill: '#475569' }} />
          <Tooltip
            cursor={{ fill: 'rgba(148,163,184,0.12)' }}
            content={({ active, payload }) =>
              active && payload && payload.length > 0 ? (
                <div className="rounded border border-border bg-bg-elev-1 px-3 py-2 text-xs shadow">
                  <div className="font-medium text-fg">{(payload[0].payload as { label: string }).label}</div>
                  <div className="text-fg-muted">
                    {metric.label}: {metric.format((payload[0].payload as { value: number }).value)}
                  </div>
                </div>
              ) : null
            }
          />
          <Bar dataKey="value" radius={[3, 3, 0, 0]}>
            {data.map((_, i) => (
              <Cell key={i} fill={SERIES_COLORS[i % SERIES_COLORS.length]} />
            ))}
          </Bar>
        </BarChart>
      </ResponsiveContainer>
    </div>
  );
}

function ScatterMetric({
  points,
  labels,
  xMetric,
  yMetric,
}: {
  points: Point[];
  labels: string[];
  xMetric: Metric;
  yMetric: Metric;
}) {
  const data = points.map((p, i) => ({
    label: labels[i],
    x: xMetric.get(p.grade) ?? 0,
    y: yMetric.get(p.grade) ?? 0,
    color: SERIES_COLORS[i % SERIES_COLORS.length],
  }));
  return (
    <div className="h-80 w-full">
      <ResponsiveContainer>
        <ScatterChart margin={{ top: 8, right: 12, bottom: 28, left: 8 }}>
          <CartesianGrid strokeDasharray="3 3" stroke="#e2e8f0" />
          <XAxis
            dataKey="x"
            type="number"
            name={xMetric.label}
            domain={xMetric.max ? [0, xMetric.max] : ['auto', 'auto']}
            tick={{ fontSize: 11, fill: '#475569' }}
            label={{ value: xMetric.label, position: 'insideBottom', offset: -12, fontSize: 11 }}
          />
          <YAxis
            dataKey="y"
            type="number"
            name={yMetric.label}
            domain={yMetric.max ? [0, yMetric.max] : ['auto', 'auto']}
            tick={{ fontSize: 11, fill: '#475569' }}
            label={{ value: yMetric.label, angle: -90, position: 'insideLeft', fontSize: 11 }}
          />
          <ZAxis range={[140, 140]} />
          <Tooltip
            cursor={{ strokeDasharray: '3 3' }}
            content={({ active, payload }) =>
              active && payload && payload.length > 0 ? (
                <div className="rounded border border-border bg-bg-elev-1 px-3 py-2 text-xs shadow">
                  <div className="font-medium text-fg">{(payload[0].payload as { label: string }).label}</div>
                  <div className="text-fg-muted">
                    {xMetric.label}: {xMetric.format((payload[0].payload as { x: number }).x)}
                  </div>
                  <div className="text-fg-muted">
                    {yMetric.label}: {yMetric.format((payload[0].payload as { y: number }).y)}
                  </div>
                </div>
              ) : null
            }
          />
          <Scatter data={data}>
            {data.map((d, i) => (
              <Cell key={i} fill={d.color} />
            ))}
          </Scatter>
        </ScatterChart>
      </ResponsiveContainer>
    </div>
  );
}

function ChartEmpty({ message }: { message: string }) {
  return (
    <div className="flex h-80 items-center justify-center rounded-lg border border-dashed border-border bg-bg-elev-2/50 text-sm text-fg-muted">
      {message}
    </div>
  );
}
