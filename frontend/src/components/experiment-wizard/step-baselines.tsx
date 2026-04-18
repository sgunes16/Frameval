import type { Baseline } from '../../lib/types';

export function StepBaselines({ baselines }: { baselines: Baseline[] }) {
  return <div className="space-y-2">{baselines.map((baseline) => <label key={baseline.id} className="flex items-center gap-2 rounded-md border border-slate-200 p-3 text-sm"><input type="checkbox" />{baseline.name}</label>)}</div>;
}
