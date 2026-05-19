import { useMemo } from 'react';

import { useAnchorAlignment } from '../../lib/use-anchor-alignment';
import type { RunAnchors } from '../../lib/anchor-alignment';
import { TapeRow } from './TapeRow';

/**
 * TapeTab renders the side-by-side anchor-aligned timeline of the
 * runs the user selected to compare. The grid has one gutter column
 * (anchor label or "— drift") followed by one column per run.
 *
 * Empty state covers two causes — no runs selected, and runs that
 * have no anchors yet. Both render the same affordance because
 * neither is distinguishable to a user reading the page; pushing
 * users to /experiments to add runs is the actionable advice in
 * both cases.
 */

interface TapeTabProps {
  /** Per-run anchor lists from `GET /api/experiments/:id/anchors`. */
  runs: RunAnchors[];
  onCellClick?: (runId: string, parentTurnIndex: number) => void;
}

export function TapeTab({ runs, onCellClick }: TapeTabProps) {
  const rows = useAnchorAlignment(runs);
  const runIds = useMemo(() => runs.map((r) => r.run_id), [runs]);

  if (runs.length === 0 || rows.length === 0) {
    return (
      <div className="flex h-40 items-center justify-center rounded-md border border-dashed border-border bg-bg-elev-1 text-sm text-fg-muted">
        Select runs with completed transcripts to see their anchor-aligned timeline here.
      </div>
    );
  }

  return (
    <div
      role="grid"
      aria-label="Anchor-aligned tape"
      className="overflow-x-auto rounded-md border border-border bg-bg-elev-1"
    >
      <div
        role="row"
        className="grid items-stretch border-b border-border-strong bg-bg-elev-2 font-mono text-xs uppercase tracking-wider text-fg-muted"
        style={{ gridTemplateColumns: `220px repeat(${runIds.length}, minmax(140px, 1fr))` }}
      >
        <div role="columnheader" className="border-r border-border px-3 py-2">
          Anchor
        </div>
        {runIds.map((runId) => (
          <div role="columnheader" key={runId} className="border-r border-border px-3 py-2 last:border-r-0">
            {runId}
          </div>
        ))}
      </div>
      {rows.map((row, i) => (
        <TapeRow key={i} row={row} runIds={runIds} onCellClick={onCellClick} />
      ))}
    </div>
  );
}
