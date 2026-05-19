import { useRef, useState } from 'react';
import { useVirtualizer } from '@tanstack/react-virtual';

import type { ParsedTurn } from '../../lib/types';
import type { EvidenceForTurn } from '../../lib/symptom-evidence';
import { groupTurns } from './group-turns';
import { TurnGroupCard } from './TurnGroupCard';

/**
 * TurnList — the Inspector V2 main pane. Groups incoming ParsedTurns
 * by parent_turn_index (see groupTurns) and renders each group as a
 * collapsible card, virtualized so 500+ turn runs stay responsive.
 *
 * The list also owns the "focused turn" state — clicking a card sets
 * it as focused; the parent route uses that to drive the right-side
 * detail pane (per-turn diff, symptoms). Focus state is local to this
 * component for now; lifting it up is a Story #63 / #64 concern when
 * the diff panel and symptom glyph wiring land.
 */

interface TurnListProps {
  turns: ParsedTurn[];
  onFocusChange?: (parentTurnIndex: number | null) => void;
  evidenceByTurn?: Map<number, EvidenceForTurn>;
}

// Conservative estimated row height in pixels. react-virtual uses this
// for initial layout; actual height is re-measured on render so the
// estimate only matters for the first paint. Set high enough that
// short cards don't cause layout shift on initial scroll.
const ROW_ESTIMATE_PX = 140;

export function TurnList({ turns, onFocusChange, evidenceByTurn }: TurnListProps) {
  const parentRef = useRef<HTMLDivElement | null>(null);
  const groups = groupTurns(turns);
  const [focused, setFocused] = useState<number | null>(null);

  const virtualizer = useVirtualizer({
    count: groups.length,
    getScrollElement: () => parentRef.current,
    estimateSize: () => ROW_ESTIMATE_PX,
    overscan: 4,
  });

  if (groups.length === 0) {
    return (
      <div className="flex h-full items-center justify-center text-sm text-fg-muted">
        No turns yet.
      </div>
    );
  }

  return (
    <div
      ref={parentRef}
      className="h-full overflow-auto"
      data-testid="turn-list"
    >
      <div
        style={{ height: virtualizer.getTotalSize(), position: 'relative', width: '100%' }}
      >
        {virtualizer.getVirtualItems().map((virtualRow) => {
          const group = groups[virtualRow.index];
          if (!group) return null;
          const isFocused = focused === group.parentTurnIndex;
          return (
            <div
              key={group.parentTurnIndex}
              data-index={virtualRow.index}
              ref={virtualizer.measureElement}
              style={{
                position: 'absolute',
                top: 0,
                left: 0,
                width: '100%',
                transform: `translateY(${virtualRow.start}px)`,
              }}
              className={isFocused ? 'ring-2 ring-accent ring-offset-2 ring-offset-bg' : ''}
            >
              <div>
                <TurnGroupCard
                  group={group}
                  evidenceByTurn={evidenceByTurn}
                  onFocus={(idx) => {
                    setFocused(idx);
                    onFocusChange?.(idx);
                  }}
                />
              </div>
            </div>
          );
        })}
      </div>
    </div>
  );
}
