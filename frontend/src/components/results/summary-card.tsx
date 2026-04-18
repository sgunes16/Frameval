import { Card } from '../ui/card';

export function SummaryCard({ title, value }: { title: string; value: string | number }) {
  return <Card><div className="text-sm text-slate-500">{title}</div><div className="mt-2 text-2xl font-semibold">{value}</div></Card>;
}
