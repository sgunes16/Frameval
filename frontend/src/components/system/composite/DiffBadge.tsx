import { cn } from '../../../lib/utils';

/**
 * DiffBadge renders the `+N / −M` line-count pill used in Inspector V2's
 * per-turn diff panel and Compare V2's artifact diff header.
 *
 * Designed so two badges in different rows visually align: monospace
 * tabular-nums, the same total width regardless of digit count, and a
 * fixed middle gap. Token colors (`--diff-add-text`, `--diff-del-text`)
 * adapt to dark/light without ifs in the component itself.
 *
 * Empty diffs (both counts zero) get a "no changes" affordance instead
 * of two zero pills — there's no useful visual difference between
 * "+0 / −0" and the absence of any diff, and the explicit text gives
 * a clearer signal that nothing happened.
 */

interface DiffBadgeProps {
  added: number;
  removed: number;
  className?: string;
}

export function DiffBadge({ added, removed, className }: DiffBadgeProps) {
  if (added === 0 && removed === 0) {
    return (
      <span
        className={cn(
          'inline-flex items-center rounded-sm border border-border bg-bg-elev-2 px-1.5 py-0.5 font-mono text-xs text-fg-subtle',
          className,
        )}
      >
        no changes
      </span>
    );
  }
  return (
    <span
      className={cn(
        'inline-flex items-center gap-2 rounded-sm border border-border bg-bg-elev-2 px-1.5 py-0.5 font-mono text-xs tabular-nums',
        className,
      )}
    >
      {added > 0 && <span className="text-diff-add-text">+{added}</span>}
      {removed > 0 && <span className="text-diff-del-text">−{removed}</span>}
    </span>
  );
}
