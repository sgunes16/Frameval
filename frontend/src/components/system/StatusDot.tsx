import { cn } from '../../lib/utils';

/**
 * StatusDot is the smallest possible status indicator — an 8px circle
 * colored by semantic variant. Used in:
 *
 *   - Run table: each row's status column flips between neutral / running
 *     (pulse) / success / failed.
 *   - Compare V2 grade panel: per-metric pass/fail.
 *   - Future Inspector V2 turn header: live cursor pulses on the
 *     currently-streaming turn.
 *
 * The dot is purely decorative when paired with a text label nearby; the
 * `aria-label` prop is required for standalone usage so screen-reader
 * users get the same information sighted users get from the color.
 */

export type StatusVariant = 'success' | 'warning' | 'danger' | 'info' | 'neutral';

const variantClass: Record<StatusVariant, string> = {
  success: 'bg-success',
  warning: 'bg-warning',
  danger: 'bg-danger',
  info: 'bg-info',
  // neutral has no semantic color of its own; it falls back to the
  // subtle text token so the dot reads as a hairline marker.
  neutral: 'bg-fg-subtle',
};

interface StatusDotProps {
  variant?: StatusVariant;
  pulse?: boolean;
  'aria-label': string;
  className?: string;
}

export function StatusDot({ variant = 'neutral', pulse, className, ...rest }: StatusDotProps) {
  return (
    <span
      role="status"
      aria-label={rest['aria-label']}
      className={cn(
        'inline-block h-2 w-2 rounded-full',
        variantClass[variant],
        pulse && 'animate-pulse',
        className,
      )}
    />
  );
}
