import { ResponsiveContainer, BarChart, Bar, XAxis, YAxis, Tooltip } from 'recharts';
import type { ExperimentStat } from '../../lib/types';

export function HeatmapChart({ stats }: { stats: ExperimentStat[] }) {
  if (stats.length === 0) {
    return <div className="flex h-72 items-center justify-center rounded-md border border-dashed border-slate-300 text-sm text-slate-500">Grafik icin yeterli karsilastirma verisi yok.</div>;
  }
  const data = stats.map((stat) => ({ name: stat.metric_name, value: stat.mean_b - stat.mean_a }));
  return <div className="h-72"><ResponsiveContainer><BarChart data={data}><XAxis dataKey="name" /><YAxis /><Tooltip /><Bar dataKey="value" fill="#0f172a" /></BarChart></ResponsiveContainer></div>;
}
