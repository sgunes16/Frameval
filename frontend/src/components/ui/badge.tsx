import { PropsWithChildren } from 'react';
import { cn } from '../../lib/utils';

type Tone = 'neutral' | 'success' | 'warning' | 'danger' | 'info' | 'muted';

// Tone styling pairs a tinted background (`bg-{tone}/10`) with the
// matching `*-fg` text token — the body-copy variants calibrated to
// hit WCAG AA against a 10-15% wash in BOTH themes (see tokens.css).
// Using `text-{tone}` directly here drops to ~3:1 in light mode.
const tones: Record<Tone, string> = {
  neutral: 'bg-bg-elev-2 text-fg border-border',
  success: 'bg-success/10 text-success-fg border-success/30',
  warning: 'bg-warning/15 text-warning-fg border-warning/40',
  danger: 'bg-danger/10 text-danger-fg border-danger/30',
  info: 'bg-info/10 text-info-fg border-info/30',
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
