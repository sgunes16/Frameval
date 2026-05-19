import { cn } from '../../lib/utils';

/**
 * TokenChip renders an LLM token-count pill (`12.4k in · 3.1k out`).
 * Used in:
 *
 *   - Run cards on the dashboard: at-a-glance per-run cost.
 *   - Inspector V2 turn header: per-turn token spend.
 *   - Compare V2 summary panel: aggregate.
 *
 * Monospace digits so column alignment works in tables and dense lists.
 */

interface TokenChipProps {
  in: number;
  out: number;
  className?: string;
}

export function TokenChip({ in: tokensIn, out: tokensOut, className }: TokenChipProps) {
  return (
    <span
      className={cn(
        'inline-flex items-center gap-1 rounded-sm border border-border bg-bg-elev-2 px-1.5 py-0.5 font-mono text-xs tabular-nums text-fg-muted',
        className,
      )}
    >
      <span>{formatCount(tokensIn)} in</span>
      <span aria-hidden="true">·</span>
      <span>{formatCount(tokensOut)} out</span>
    </span>
  );
}

/**
 * formatCount turns 12_400 into "12.4k" and 2_500_000 into "2.5M".
 * Below 1000 it returns the raw number; values that would round to an
 * extra trailing zero are trimmed for readability ("1.0k" → "1k").
 */
function formatCount(n: number): string {
  if (n < 1000) return String(n);
  if (n < 1_000_000) {
    const value = n / 1000;
    return trimTrailingZero(value.toFixed(1)) + 'k';
  }
  const value = n / 1_000_000;
  return trimTrailingZero(value.toFixed(1)) + 'M';
}

function trimTrailingZero(s: string): string {
  return s.endsWith('.0') ? s.slice(0, -2) : s;
}
