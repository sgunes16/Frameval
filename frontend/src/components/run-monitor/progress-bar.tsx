import { Progress } from '../../components/ui/progress';

export function RunProgressBar({ completed, total }: { completed: number; total: number }) {
  const percent = total ? (completed / total) * 100 : 0;
  return (
    <div className="space-y-2">
      <div className="flex items-center justify-between text-xs text-slate-500">
        <span>{completed}/{total} runs finished</span>
        <span>{percent.toFixed(0)}%</span>
      </div>
      <Progress value={percent} tone={percent === 100 ? 'success' : 'neutral'} />
    </div>
  );
}
