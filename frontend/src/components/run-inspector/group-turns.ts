import type { BlockKind, ParsedTurn } from '../../lib/types';

/**
 * A TurnGroup is one "decision" in the Inspector V2 view — the unit
 * the user thinks about as a single agent step. It collects every
 * ParsedTurn that the backend's AssignTurnGrouping helper stamped with
 * the same parent_turn_index (thinking + tool_use + tool_result + any
 * trailing prose).
 */
export interface TurnGroup {
  parentTurnIndex: number;
  blocks: ParsedTurn[];
  /**
   * The block kind that drives the visual identity of the group (left
   * bar color, glyph). Picked from the most "decisive" member — a
   * tool_use beats a thinking block, etc. See `pickRepresentativeKind`.
   */
  representativeKind: BlockKind;
  /**
   * If any block in the group is a tool_use, surface its tool name so
   * the card header can show it ("Turn 7 · Tool · Edit"). Undefined
   * when no tool_use block is present.
   */
  toolName?: string;
}

// Priority list: higher index = more decisive. A group's representative
// kind is the highest-priority block kind that appears in it.
const KIND_PRIORITY: BlockKind[] = ['system', 'text', 'thinking', 'tool_result', 'tool_use'];

function pickRepresentativeKind(blocks: ParsedTurn[]): BlockKind {
  let best: BlockKind = '';
  let bestRank = -1;
  for (const block of blocks) {
    const kind = (block.block_kind ?? '') as BlockKind;
    const rank = KIND_PRIORITY.indexOf(kind);
    if (rank > bestRank) {
      best = kind;
      bestRank = rank;
    }
  }
  return best;
}

/**
 * groupTurns folds a flat ParsedTurn[] into TurnGroup[] keyed by the
 * parent_turn_index stamp the backend's grouping helper produced.
 *
 * Legacy escape hatch: pre-Inspector-V2 transcripts have every block
 * stamped with parent_turn_index = 0. Collapsing them all into one
 * group would render a 200-turn legacy run as a single unreadable
 * blob. The helper detects "all blocks have empty BlockKind" (the
 * defining symptom of unstamped data) and falls back to one group
 * per block, indexed by array position.
 *
 * Output is ordered by the first appearance of each group in the
 * input — important when transcripts arrive out-of-order from
 * background re-grading.
 */
export function groupTurns(turns: ParsedTurn[]): TurnGroup[] {
  if (turns.length === 0) return [];

  const allUnstamped = turns.every((t) => !t.block_kind);
  if (allUnstamped) {
    return turns.map((block, i) => ({
      parentTurnIndex: i,
      blocks: [block],
      representativeKind: '' as BlockKind,
      toolName: block.tool_name,
    }));
  }

  // Use a Map so groups stay in insertion order — easy to reason about,
  // matches the order the helper saw the blocks, and we sort by first
  // turn_index at the end to recover from out-of-order input.
  const grouped = new Map<number, ParsedTurn[]>();
  for (const turn of turns) {
    const key = turn.parent_turn_index ?? 0;
    const bucket = grouped.get(key);
    if (bucket) {
      bucket.push(turn);
    } else {
      grouped.set(key, [turn]);
    }
  }

  const groups: TurnGroup[] = Array.from(grouped.entries()).map(([parent, blocks]) => {
    const toolBlock = blocks.find((b) => b.block_kind === 'tool_use');
    return {
      parentTurnIndex: parent,
      blocks,
      representativeKind: pickRepresentativeKind(blocks),
      toolName: toolBlock?.tool_name,
    };
  });

  // Sort by the smallest turn_index in each group — the first block's
  // position in the transcript. Stable across re-renders because both
  // the grouping pass and the sort are deterministic.
  groups.sort((a, b) => {
    const aFirst = Math.min(...a.blocks.map((blk) => blk.turn_index ?? 0));
    const bFirst = Math.min(...b.blocks.map((blk) => blk.turn_index ?? 0));
    return aFirst - bFirst;
  });

  return groups;
}
