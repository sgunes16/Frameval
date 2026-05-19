import {
  Bar,
  BarChart,
  CartesianGrid,
  Legend,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from 'recharts';
import type { Diagnostic, FailureCode } from '../../lib/types';

// 13-color palette, one per FailureCode (NONE intentionally muted so it
// reads as "not a failure" in the legend).
const CODE_COLORS: Record<FailureCode, string> = {
  NONE: '#94a3b8',
  HAL_API: '#dc2626',
  HAL_FILE: '#ea580c',
  DEP_MISS: '#ca8a04',
  STOP_EARLY: '#7c3aed',
  STOP_GIVEUP: '#a21caf',
  LOOP_INF: '#0891b2',
  WRONG_ABS: '#0284c7',
  MISREAD: '#16a34a',
  ENV_ERR: '#64748b',
  SCOPE_DRIFT: '#be123c',
  TIMEOUT: '#171717',
  SILENT_SKIP: '#facc15',
};

const ALL_CODES: FailureCode[] = [
  'HAL_API', 'HAL_FILE', 'DEP_MISS',
  'STOP_EARLY', 'STOP_GIVEUP', 'LOOP_INF',
  'WRONG_ABS', 'MISREAD', 'ENV_ERR',
  'SCOPE_DRIFT', 'TIMEOUT', 'SILENT_SKIP',
];

export type FailureBreakdownSeries = {
  label: string;
  diagnostic: Diagnostic;
};

type Props = {
  series: FailureBreakdownSeries[];
};

/**
 * Stacked bar chart counting each FailureCode across the selected runs.
 * One bar per run; segments colored by code. Drives the "what fails how"
 * intuition in a single visual.
 */
export function FailureBreakdown({ series }: Props) {
  if (series.length === 0) {
    return <EmptyState />;
  }
  const data = series.map((s) => {
    const row: Record<string, string | number> = { run: s.label };
    const label = s.diagnostic.failure_label;
    if (label && label.primary !== 'NONE') {
      row[label.primary] = (row[label.primary] as number ?? 0) + 1;
      for (const sec of label.secondary ?? []) {
        row[sec] = (row[sec] as number ?? 0) + 1;
      }
    }
    return row;
  });

  // Only render bars for codes that appear at least once in this series.
  const codesShown = ALL_CODES.filter((code) =>
    data.some((row) => (row[code] as number) > 0),
  );

  // If every selected run still has a null/absent failure_label (the
  // classifier hasn't run yet), codesShown is empty and BarChart would
  // render axes with no bars — show the empty state instead so it's
  // explicit what's missing.
  if (codesShown.length === 0) {
    return (
      <div className="flex h-80 items-center justify-center rounded-lg border border-dashed border-border bg-bg-elev-2/50 text-sm text-fg-muted">
        No failure classifications available yet. The classifier may not have run on these runs.
      </div>
    );
  }

  return (
    <div className="h-80 w-full">
      <ResponsiveContainer>
        <BarChart data={data}>
          <CartesianGrid strokeDasharray="3 3" stroke="#e2e8f0" />
          <XAxis dataKey="run" tick={{ fontSize: 11, fill: '#475569' }} />
          <YAxis allowDecimals={false} tick={{ fontSize: 11, fill: '#475569' }} />
          <Tooltip />
          <Legend wrapperStyle={{ fontSize: 11 }} />
          {codesShown.map((code) => (
            <Bar key={code} dataKey={code} stackId="failures" fill={CODE_COLORS[code]} />
          ))}
        </BarChart>
      </ResponsiveContainer>
    </div>
  );
}

function EmptyState() {
  return (
    <div className="flex h-80 items-center justify-center rounded-lg border border-dashed border-border bg-bg-elev-2/50 text-sm text-fg-muted">
      No failure classifications available yet for the selected runs.
    </div>
  );
}
