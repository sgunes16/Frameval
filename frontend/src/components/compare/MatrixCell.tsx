/**
 * MatrixCell — one cell of the Compare V2 Matrix tab. Colour scales
 * from `bg-bg-elev-2` (0.0) to `bg-accent` (1.0) via opacity, so a
 * single token drives the gradient without burning more chart tokens
 * on a discrete heatmap palette.
 *
 * The accent at 0% alpha is just the page background — invisible —
 * and at 100% is the full focus colour. Linear interpolation matches
 * the human sense of "how similar"; perceptually-uniform isn't worth
 * the extra calculation for a 5×5 max grid.
 *
 * Diagonal cells render with reduced contrast (muted text) so the
 * user reads off-diagonal as the actionable data.
 */

interface MatrixCellProps {
  value: number;
  isDiagonal?: boolean;
  rowRunId: string;
  colRunId: string;
  onClick?: () => void;
}

export function MatrixCell({ value, isDiagonal, rowRunId, colRunId, onClick }: MatrixCellProps) {
  const clamped = Math.max(0, Math.min(1, value));
  const label = isDiagonal
    ? `${rowRunId} self-similarity (1.00)`
    : `Similarity between ${rowRunId} and ${colRunId}: ${clamped.toFixed(2)}`;
  const body = (
    <span
      className={
        'font-mono ' + (isDiagonal ? 'text-fg-subtle' : 'text-fg')
      }
    >
      {clamped.toFixed(2)}
    </span>
  );
  // Diagonal is informational only — no click target.
  if (isDiagonal || !onClick) {
    return (
      <div
        role="gridcell"
        aria-label={label}
        className="flex h-12 items-center justify-center border-r border-b border-border text-xs"
        style={{ backgroundColor: `hsl(var(--accent) / ${clamped * 0.9})` }}
      >
        {body}
      </div>
    );
  }
  return (
    <button
      type="button"
      role="gridcell"
      aria-label={`${label} — open Tape`}
      onClick={onClick}
      className="flex h-12 items-center justify-center border-r border-b border-border text-xs hover:ring-2 hover:ring-accent/40"
      style={{ backgroundColor: `hsl(var(--accent) / ${clamped * 0.9})` }}
    >
      {body}
    </button>
  );
}
