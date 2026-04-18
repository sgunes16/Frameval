import type { ExperimentStat } from '../../lib/types';

export function ComparisonTable({ stats }: { stats: ExperimentStat[] }) {
  if (stats.length === 0) {
    return <div className="rounded-md border border-dashed border-slate-300 px-4 py-8 text-sm text-slate-500">Henuz karsilastirma istatistigi olusmadi. Sayfa run ortalamalarini altta gostermeye devam ediyor.</div>;
  }

  return <div className="overflow-auto rounded-md border border-slate-200"><table className="min-w-full text-sm"><thead className="bg-slate-100"><tr><th className="px-3 py-2 text-left">Metric</th><th className="px-3 py-2 text-left">Variant A</th><th className="px-3 py-2 text-left">Variant B</th><th className="px-3 py-2 text-left">p-value</th></tr></thead><tbody>{stats.map((stat) => <tr key={`${stat.metric_name}-${stat.variant_a_id}-${stat.variant_b_id}`} className="border-t border-slate-200"><td className="px-3 py-2">{stat.metric_name}</td><td className="px-3 py-2">{stat.mean_a.toFixed(2)}</td><td className="px-3 py-2">{stat.mean_b.toFixed(2)}</td><td className="px-3 py-2">{Number.isFinite(stat.p_value) ? stat.p_value.toFixed(3) : '-'}</td></tr>)}</tbody></table></div>;
}
