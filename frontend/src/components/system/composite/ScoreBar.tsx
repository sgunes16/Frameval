import { cn } from '../../../lib/utils';

/**
 * ScoreBar renders a 0..1 dimension as a row of 10 discrete segments,
 * the fraction filled corresponding to the value. Used in:
 *
 *   - Fingerprint dimensions (planning_depth, tool_call_diversity, …)
 *   - Judge rubric scores (correctness, maintainability, …)
 *   - Calibration accuracy in the diagnostic report
 *
 * Discrete segments (vs. a continuous gradient) make the value
 * legible at a glance — operators can count the filled cells without
 * mousing over to read the number. The exact value is exposed via the
 * `role="meter"` ARIA contract so screen readers and automated audits
 * see the same number sighted users read off the bar.
 */

interface ScoreBarProps {
  value: number;
  label: string;
  segments?: number;
  className?: string;
}

export function ScoreBar({ value, label, segments = 10, className }: ScoreBarProps) {
  // Clamp to [0,1] so an out-of-range value (e.g., a buggy upstream
  // calculation) never overflows the bar; the meter's aria-valuenow
  // still reports the raw value so the bug is debuggable downstream.
  const clamped = Math.max(0, Math.min(1, value));
  const filledCount = Math.round(clamped * segments);

  return (
    <div
      role="meter"
      aria-label={label}
      aria-valuemin={0}
      aria-valuemax={1}
      // Report the raw value (not the clamped one) so an upstream
      // bug producing an out-of-range score surfaces in automated
      // a11y audits and screen-reader output. The visual fill still
      // clamps to [0,1] so the bar doesn't overflow.
      aria-valuenow={value}
      title={label}
      className={cn('flex items-center gap-0.5', className)}
    >
      {Array.from({ length: segments }, (_, i) => {
        const filled = i < filledCount;
        return (
          <span
            key={i}
            data-testid="score-segment"
            data-filled={filled ? 'true' : 'false'}
            className={cn(
              'h-1.5 w-2 rounded-sm transition-colors',
              filled ? 'bg-accent' : 'bg-bg-elev-2',
            )}
          />
        );
      })}
    </div>
  );
}
