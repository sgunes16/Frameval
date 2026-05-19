import { TurnCard } from '../system';
import type { BlockKind } from '../../lib/types';
import type { TurnGroup } from './group-turns';
import { renderBlock } from './blocks';

/**
 * TurnGroupCard renders one "decision" — the parent_turn_index group
 * that AssignTurnGrouping (engine/pkg/executor/grouping.go) produced.
 * It composes the design-system primitive TurnCard with one block
 * renderer per child member of the group.
 *
 * The Inspector V2 list renders one of these per group; clicking the
 * card focuses it in the right-side detail pane.
 */

interface TurnGroupCardProps {
  group: TurnGroup;
  defaultCollapsed?: boolean;
  onFocus?: (parentTurnIndex: number) => void;
}

export function TurnGroupCard({ group, defaultCollapsed, onFocus }: TurnGroupCardProps) {
  const kind: BlockKind = (group.representativeKind || 'text') as BlockKind;
  return (
    <button
      type="button"
      onClick={() => onFocus?.(group.parentTurnIndex)}
      className="block w-full text-left"
      aria-label={`Focus turn ${group.parentTurnIndex}`}
    >
      <TurnCard
        turnIndex={group.parentTurnIndex}
        blockKind={(kind === '' ? 'text' : kind) as Exclude<BlockKind, ''>}
        toolName={group.toolName}
        defaultCollapsed={defaultCollapsed}
      >
        <div className="space-y-2">
          {group.blocks.map((block, idx) => (
            <div key={block.turn_index ?? idx}>{renderBlock(block)}</div>
          ))}
        </div>
      </TurnCard>
    </button>
  );
}
