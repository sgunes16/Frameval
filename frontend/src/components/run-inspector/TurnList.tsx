import { useEffect, useLayoutEffect, useRef, useState } from 'react';
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

  // Stick-to-bottom while the run is live. Pattern follows Slack /
  // terminal emulators: as long as the user is "near" the bottom we
  // auto-follow new turns; once they manually scroll up, we stop
  // chasing — the operator wants to read what they're reading.
  // `stickToBottom` flips to false when the user scrolls more than
  // 80 px above the bottom, and back to true when they scroll within
  // 16 px of it.
  const [stickToBottom, setStickToBottom] = useState(true);
  useEffect(() => {
    const el = parentRef.current;
    if (!el) return;
    const onScroll = () => {
      const distance = el.scrollHeight - el.scrollTop - el.clientHeight;
      if (distance < 16) setStickToBottom(true);
      else if (distance > 80) setStickToBottom(false);
    };
    el.addEventListener('scroll', onScroll, { passive: true });
    return () => el.removeEventListener('scroll', onScroll);
  }, []);

  // When new turns arrive and the user is anchored at the bottom,
  // scroll the virtualized list to the last group. useLayoutEffect
  // (not useEffect) so the scroll happens before the browser paints
  // the new row — otherwise the user sees a one-frame flash of the
  // old bottom before the auto-scroll catches up.
  const lastGroupIndex = groups.length - 1;
  useLayoutEffect(() => {
    if (!stickToBottom || lastGroupIndex < 0) return;
    virtualizer.scrollToIndex(lastGroupIndex, { align: 'end' });
  }, [lastGroupIndex, stickToBottom, virtualizer]);

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
