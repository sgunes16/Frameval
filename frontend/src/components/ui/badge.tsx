import { PropsWithChildren } from 'react';
import { cn } from '../../lib/utils';

type Tone = 'neutral' | 'success' | 'warning' | 'danger' | 'info' | 'muted';

const tones: Record<Tone, string> = {
  neutral: 'bg-bg-elev-2 text-fg border-border',
  success: 'bg-emerald-50 text-emerald-700 border-emerald-200',
  warning: 'bg-amber-50 text-amber-700 border-amber-200',
  danger: 'bg-red-50 text-red-700 border-red-200',
  info: 'bg-indigo-50 text-indigo-700 border-indigo-200',
  muted: 'bg-bg-elev-2 text-fg-muted border-border',
};

export function Badge({ children, tone = 'neutral', className }: PropsWithChildren<{ tone?: Tone; className?: string }>) {
  return (
    <span
      className={cn(
        'inline-flex items-center gap-1 rounded-full border px-2 py-0.5 text-[11px] font-medium leading-4',
        tones[tone],
        className,
      )}
    >
      {children}
    </span>
  );
}
