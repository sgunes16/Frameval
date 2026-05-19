/**
 * ForkGlyph — the small badge dropped between adjacent drift rows in
 * the Tape tab to mark a fork. Compact (one inline element wide) so
 * it doesn't disturb the column grid.
 *
 * Visually: a diamond-like character + the count of runs that diverged
 * at this fork. Click handler bubbles up so the parent can open the
 * ForkDrawer with the focused fork in view.
 */

interface ForkGlyphProps {
  /** Number of distinct branches this fork represents. */
  count: number;
  onClick?: () => void;
  className?: string;
}

export function ForkGlyph({ count, onClick, className }: ForkGlyphProps) {
  return (
    <button
      type="button"
      onClick={onClick}
      aria-label={`Fork with ${count} branches — open detail`}
      className={
        'inline-flex h-5 min-w-5 items-center justify-center rounded-sm border border-warning/40 bg-warning/15 px-1.5 font-mono text-xs font-semibold text-warning-fg hover:bg-warning/25 ' +
        (className ?? '')
      }
    >
      ⋔{count}
    </button>
  );
}
