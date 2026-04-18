import type { Baseline } from '../../lib/types';
import { Card } from '../ui/card';

export function BaselineCompare({ baselines }: { baselines: Baseline[] }) {
  return <Card><div className="font-semibold">Baseline Compare</div><div className="mt-3 space-y-2">{baselines.map((baseline) => <div key={baseline.id} className="rounded-md border border-slate-200 p-3 text-sm">{baseline.name} · {baseline.model}</div>)}</div></Card>;
}
