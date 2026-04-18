import { ResponsiveContainer, ScatterChart, Scatter, XAxis, YAxis, Tooltip } from 'recharts';
import type { ExperimentStat } from '../../lib/types';

export function ScatterPlotChart({ stats }: { stats: ExperimentStat[] }) {
  if (stats.length === 0) {
    return <div className="flex h-72 items-center justify-center rounded-md border border-dashed border-slate-300 text-sm text-slate-500">Scatter plot icin karsilastirma verisi bulunamadi.</div>;
  }
  const data = stats.map((stat) => ({ x: stat.mean_a, y: stat.mean_b, name: stat.metric_name }));
  return <div className="h-72"><ResponsiveContainer><ScatterChart><XAxis type="number" dataKey="x" name="Variant A" /><YAxis type="number" dataKey="y" name="Variant B" /><Tooltip cursor={{ strokeDasharray: '3 3' }} /><Scatter data={data} fill="#334155" /></ScatterChart></ResponsiveContainer></div>;
}
