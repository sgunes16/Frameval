import { useMemo } from 'react';

import { buildSimilarityMatrix } from '../../lib/similarity-matrix';
import type { RunAnchors } from '../../lib/anchor-alignment';
import { MatrixCell } from './MatrixCell';

/**
 * MatrixTab renders the N×N similarity heatmap. Disabled when fewer
 * than 3 runs are selected (a 2×2 matrix would be just two cells and
 * one is the diagonal — not useful). Off-diagonal clicks bubble out
 * via `onPairClick` so the parent can navigate to the Tape tab
 * scoped to that pair via URL state.
 *
 * Symmetry means we render the full grid but the upper and lower
 * triangles carry the same information; the visual redundancy is
 * deliberate — it makes the row-by-row scan easier for the user.
 */

interface MatrixTabProps {
  runs: RunAnchors[];
  onPairClick?: (runIdA: string, runIdB: string) => void;
}

export function MatrixTab({ runs, onPairClick }: MatrixTabProps) {
  const matrix = useMemo(() => buildSimilarityMatrix(runs), [runs]);

  if (runs.length < 3) {
    return (
      <div
        role="status"
        className="flex h-40 items-center justify-center rounded-md border border-dashed border-border bg-bg-elev-1 text-sm text-fg-muted"
      >
        Select at least three runs to populate the similarity matrix.
      </div>
    );
  }

  const cols = `200px repeat(${matrix.runIds.length}, minmax(80px, 1fr))`;
  return (
    <div role="grid" aria-label="Run similarity matrix" className="overflow-x-auto rounded-md border border-border bg-bg-elev-1">
      <div
        role="row"
        className="grid items-stretch border-b border-border-strong bg-bg-elev-2 text-xs uppercase tracking-wider text-fg-muted"
        style={{ gridTemplateColumns: cols }}
      >
        <div role="columnheader" className="border-r border-border px-3 py-2 font-mono">
          Run
        </div>
        {matrix.runIds.map((id) => (
          <div key={id} role="columnheader" className="border-r border-border px-3 py-2 font-mono text-center last:border-r-0">
            {id}
          </div>
        ))}
      </div>
      {matrix.runIds.map((rowId, i) => (
        <div
          key={rowId}
          role="row"
          className="grid items-stretch"
          style={{ gridTemplateColumns: cols }}
        >
          <div role="rowheader" className="border-r border-b border-border bg-bg-elev-2 px-3 py-2 font-mono text-xs text-fg-muted">
            {rowId}
          </div>
          {matrix.runIds.map((colId, j) => (
            <MatrixCell
              key={colId}
              value={matrix.values[i]![j]!}
              isDiagonal={i === j}
              rowRunId={rowId}
              colRunId={colId}
              onClick={i === j ? undefined : () => onPairClick?.(rowId, colId)}
            />
          ))}
        </div>
      ))}
    </div>
  );
}
