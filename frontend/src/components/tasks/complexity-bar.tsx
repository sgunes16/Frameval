import { Progress } from '../ui/progress';

export function ComplexityBar({ value }: { value: number }) {
  return <div className="space-y-1 text-sm"><div>Complexity {value.toFixed(1)}/10</div><Progress value={value * 10} /></div>;
}
