import { useState, type ReactNode } from 'react';
import { cn } from '../../../lib/utils';

/**
 * TurnCard is the canonical "one decision" container in Inspector V2.
 * Each card renders a parent-turn group: header (turn index, role
 * glyph, optional tool name, optional symptom glyph), then the body
 * (thinking text, tool input/output, prose). Compare V2 reuses it in
 * a dense variant inside the Tape tab.
 *
 * Block kind controls the left bar color so a scanner can see at a
 * glance which row is thinking, which is a tool call, which is a
 * tool result, etc. The mapping uses chart tokens so the colors track
 * the rest of the design language (chart-1..N) rather than carrying
 * their own bespoke palette.
 *
 * Collapse is opt-in via `defaultCollapsed`; long thinking blocks and
 * verbose tool outputs benefit from starting collapsed so the cinema-
 * strip stays readable. The disclosure button is real (role=button) so
 * keyboard users can toggle it without a mouse.
 */

/**
 * BlockKind constrained to the visual-identity values TurnCard accepts.
 * The wider, transcript-level type lives in lib/types.ts and includes
 * '' (empty) for legacy data; consumers reach TurnCard only after
 * picking a concrete representative kind via groupTurns.
 */
export type BlockKind = 'thinking' | 'text' | 'tool_use' | 'tool_result' | 'system';

const barColor: Record<BlockKind, string> = {
  thinking: 'bg-chart-5',
  text: 'bg-fg-subtle',
  tool_use: 'bg-chart-1',
  tool_result: 'bg-chart-2',
  system: 'bg-chart-3',
};

const roleLabel: Record<BlockKind, string> = {
  thinking: 'Thinking',
  text: 'Assistant',
  tool_use: 'Tool',
  tool_result: 'Result',
  system: 'System',
};

interface TurnCardProps {
  turnIndex: number;
  blockKind: BlockKind;
  toolName?: string;
  defaultCollapsed?: boolean;
  symptomGlyph?: ReactNode;
  children: ReactNode;
  className?: string;
}

export function TurnCard({
  turnIndex,
  blockKind,
  toolName,
  defaultCollapsed = false,
  symptomGlyph,
  children,
  className,
}: TurnCardProps) {
  const [expanded, setExpanded] = useState(!defaultCollapsed);
  return (
    <div
      className={cn(
        'flex overflow-hidden rounded-md border border-border bg-bg-elev-1',
        className,
      )}
    >
      <span
        data-testid="turn-bar"
        aria-hidden="true"
        className={cn('w-1 flex-shrink-0', barColor[blockKind])}
      />
      <div className="flex min-w-0 flex-1 flex-col">
        <header className="flex items-center gap-2 px-3 py-2 text-xs text-fg-muted">
          <button
            type="button"
            aria-label={`${expanded ? 'Collapse' : 'Expand'} turn ${turnIndex}`}
            onClick={(e) => {
              // Stop the click from bubbling to any outer click handler
              // (TurnGroupCard wraps the card in a role=button div that
              // fires onFocus on click — without this, expanding would
              // also focus the turn, which is unexpected UX).
              e.stopPropagation();
              setExpanded((v) => !v);
            }}
            className="inline-flex h-5 w-5 items-center justify-center rounded-sm border border-border bg-bg-elev-2 text-fg-muted transition hover:bg-bg-elev-1"
          >
            {expanded ? '−' : '+'}
          </button>
          <span className="font-mono font-medium text-fg">Turn {turnIndex}</span>
          <span aria-hidden="true">·</span>
          <span>{roleLabel[blockKind]}</span>
          {toolName && (
            <>
              <span aria-hidden="true">·</span>
              <span className="font-mono text-fg">{toolName}</span>
            </>
          )}
          {symptomGlyph && <span className="ml-auto">{symptomGlyph}</span>}
        </header>
        {expanded && (
          <div className="border-t border-border bg-bg-elev-2/40 px-3 py-2 text-sm text-fg">
            {children}
          </div>
        )}
      </div>
    </div>
  );
}
