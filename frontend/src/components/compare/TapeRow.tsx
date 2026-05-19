import type { AlignmentRow, AnchoredRow, DriftRow } from '../../lib/anchor-alignment';

/**
 * TapeRow renders one row of the Tape tab. The row's `kind` decides
 * how the gutter label and cells render:
 *   - anchored: gutter shows the anchor key (tool|files); every
 *     column carries the parent_turn_index of that anchor in the
 *     corresponding run.
 *   - drift: gutter shows a dash; only the run(s) that contributed
 *     to this drift row carry a cell, the rest are empty.
 *
 * Column order is dictated by the parent — the Tape tab passes the
 * canonical run list and we render in that order so the visual
 * positions stay stable across all rows.
 */

interface TapeRowProps {
  row: AlignmentRow;
  runIds: string[];
  /** Click handler when a populated cell is clicked. */
  onCellClick?: (runId: string, parentTurnIndex: number) => void;
}

export function TapeRow({ row, runIds, onCellClick }: TapeRowProps) {
  if (row.kind === 'anchored') return <AnchoredRowView row={row} runIds={runIds} onCellClick={onCellClick} />;
  return <DriftRowView row={row} runIds={runIds} onCellClick={onCellClick} />;
}

function AnchoredRowView({
  row,
  runIds,
  onCellClick,
}: {
  row: AnchoredRow;
  runIds: string[];
  onCellClick?: (runId: string, parentTurnIndex: number) => void;
}) {
  return (
    <div
      role="row"
      className="grid items-stretch border-b border-border bg-bg-elev-1"
      style={{ gridTemplateColumns: `220px repeat(${runIds.length}, minmax(140px, 1fr))` }}
    >
      <div className="border-r border-border bg-bg-elev-2 px-3 py-2 font-mono text-xs text-fg-muted" role="rowheader">
        {row.anchor.key}
      </div>
      {runIds.map((runId) => {
        const parent = row.columns.get(runId);
        // Only render as <button> when there's actually a click handler.
        // An unconditional <button> appears in the tab order and AT
        // announces it as a no-op control even when the parent didn't
        // pass onCellClick. Mirrors the conditional DriftRowView does.
        if (parent === undefined || !onCellClick) {
          return (
            <div
              role="gridcell"
              key={runId}
              className="border-r border-border px-3 py-2 text-xs text-fg last:border-r-0"
            >
              <span className="font-mono text-fg-muted">Turn </span>
              <span className="font-mono text-fg">{parent ?? ''}</span>
            </div>
          );
        }
        return (
          <button
            type="button"
            role="gridcell"
            key={runId}
            onClick={() => onCellClick(runId, parent)}
            className="border-r border-border px-3 py-2 text-left text-xs text-fg hover:bg-bg-elev-2 last:border-r-0"
          >
            <span className="font-mono text-fg-muted">Turn </span>
            <span className="font-mono text-fg">{parent}</span>
          </button>
        );
      })}
    </div>
  );
}

function DriftRowView({
  row,
  runIds,
  onCellClick,
}: {
  row: DriftRow;
  runIds: string[];
  onCellClick?: (runId: string, parentTurnIndex: number) => void;
}) {
  return (
    <div
      role="row"
      className="grid items-stretch border-b border-border"
      style={{ gridTemplateColumns: `220px repeat(${runIds.length}, minmax(140px, 1fr))` }}
    >
      <div className="border-r border-border bg-bg-elev-2 px-3 py-2 font-mono text-xs text-fg-subtle" role="rowheader">
        — drift
      </div>
      {runIds.map((runId) => {
        const parent = row.columns.get(runId);
        if (parent === null || parent === undefined) {
          return (
            <div
              key={runId}
              role="gridcell"
              className="border-r border-border bg-bg-elev-2/30 px-3 py-2 text-xs text-fg-subtle last:border-r-0"
              aria-label={`No turn for run ${runId} on this row`}
            />
          );
        }
        if (!onCellClick) {
          return (
            <div
              key={runId}
              role="gridcell"
              className="border-r border-border bg-warning/5 px-3 py-2 text-xs text-fg last:border-r-0"
            >
              <span className="font-mono text-fg-muted">Turn </span>
              <span className="font-mono text-fg">{parent}</span>
            </div>
          );
        }
        return (
          <button
            type="button"
            role="gridcell"
            key={runId}
            onClick={() => onCellClick(runId, parent)}
            className="border-r border-border bg-warning/5 px-3 py-2 text-left text-xs text-fg hover:bg-warning/10 last:border-r-0"
          >
            <span className="font-mono text-fg-muted">Turn </span>
            <span className="font-mono text-fg">{parent}</span>
          </button>
        );
      })}
    </div>
  );
}
