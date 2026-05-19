import { cn } from '../../lib/utils';

export function Progress({ value, tone = 'neutral', className }: { value: number; tone?: 'neutral' | 'success' | 'danger'; className?: string }) {
  const clamped = Math.max(0, Math.min(100, value));
  const bar = tone === 'success' ? 'bg-emerald-500' : tone === 'danger' ? 'bg-red-500' : 'bg-fg';
  return (
    <div className={cn('h-1.5 w-full overflow-hidden rounded-full bg-bg-elev-2', className)}>
      <div className={cn('h-full rounded-full transition-all', bar)} style={{ width: `${clamped}%` }} />
    </div>
  );
}
