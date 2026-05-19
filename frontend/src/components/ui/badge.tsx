import { PropsWithChildren } from 'react';
import { cn } from '../../lib/utils';

type Tone = 'neutral' | 'success' | 'warning' | 'danger' | 'info' | 'muted';

// Tone styling uses token alphas (e.g. `bg-success/10`) so the badges
// adapt to light + dark themes without per-mode overrides. The
// `/15` fill paired with `/40` border keeps WCAG contrast in dark
// mode while staying readable on the light surface.
const tones: Record<Tone, string> = {
  neutral: 'bg-bg-elev-2 text-fg border-border',
  success: 'bg-success/10 text-success border-success/30',
  warning: 'bg-warning/15 text-warning border-warning/40',
  danger: 'bg-danger/10 text-danger border-danger/30',
  info: 'bg-info/10 text-info border-info/30',
  muted: 'bg-bg-elev-2 text-fg-muted border-border',
};

export function Badge({ children, tone = 'neutral', className }: PropsWithChildren<{ tone?: Tone; className?: string }>) {
  return (
    <span
      className={cn(
        'inline-flex items-center gap-1 rounded-full border px-2 py-0.5 text-xs font-medium leading-4',
        tones[tone],
        className,
      )}
    >
      {children}
    </span>
  );
}
