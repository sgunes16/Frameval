import { describe, expect, it } from 'vitest';

import type { ParsedTurn } from '../../lib/types';
import { groupTurns } from './group-turns';

// Default block_kind to 'text' so test fixtures count as "stamped"
// data — the legacy escape hatch in groupTurns only fires when every
// block has an empty block_kind. Override per-test where the kind matters.
const t = (overrides: Partial<ParsedTurn>): ParsedTurn => ({
  role: 'assistant',
  content: '',
  block_kind: 'text',
  ...overrides,
});

describe('groupTurns', () => {
  it('returns one group per unique parent_turn_index', () => {
    const turns: ParsedTurn[] = [
      t({ turn_index: 0, parent_turn_index: 0, block_kind: 'thinking' }),
      t({ turn_index: 1, parent_turn_index: 0, block_kind: 'tool_use' }),
      t({ turn_index: 2, parent_turn_index: 0, block_kind: 'tool_result' }),
      t({ turn_index: 3, parent_turn_index: 3, block_kind: 'thinking' }),
      t({ turn_index: 4, parent_turn_index: 3, block_kind: 'tool_use' }),
    ];

    const groups = groupTurns(turns);

    expect(groups).toHaveLength(2);
    expect(groups[0]?.blocks).toHaveLength(3);
    expect(groups[1]?.blocks).toHaveLength(2);
  });

  it('preserves insertion order within each group', () => {
    const turns: ParsedTurn[] = [
      t({ turn_index: 0, parent_turn_index: 0, content: 'first' }),
      t({ turn_index: 1, parent_turn_index: 0, content: 'second' }),
      t({ turn_index: 2, parent_turn_index: 0, content: 'third' }),
    ];

    const groups = groupTurns(turns);

    expect(groups[0]?.blocks.map((b) => b.content)).toEqual(['first', 'second', 'third']);
  });

  it('sorts groups by their first block`s turn_index', () => {
    // Out-of-order input (legacy backfill, race conditions) shouldn't
    // produce shuffled groups — sort by first appearance.
    const turns: ParsedTurn[] = [
      t({ turn_index: 5, parent_turn_index: 5 }),
      t({ turn_index: 0, parent_turn_index: 0 }),
      t({ turn_index: 6, parent_turn_index: 5 }),
      t({ turn_index: 1, parent_turn_index: 0 }),
    ];

    const groups = groupTurns(turns);

    expect(groups.map((g) => g.parentTurnIndex)).toEqual([0, 5]);
  });

  it('picks a representative block kind from the most informative member', () => {
    // Priority: tool_use > tool_result > thinking > text > system.
    // A group with thinking + tool_use renders as tool_use overall so
    // the left bar reflects the decision, not the thought.
    const turns: ParsedTurn[] = [
      t({ turn_index: 0, parent_turn_index: 0, block_kind: 'thinking' }),
      t({ turn_index: 1, parent_turn_index: 0, block_kind: 'tool_use' }),
      t({ turn_index: 2, parent_turn_index: 0, block_kind: 'tool_result' }),
    ];

    const groups = groupTurns(turns);

    expect(groups[0]?.representativeKind).toBe('tool_use');
  });

  it('handles legacy turns with no grouping stamps (all turn_index=0)', () => {
    // Pre-Inspector-V2 transcripts have parent_turn_index=0 and
    // turn_index=0 on every block. groupTurns must NOT collapse them
    // all into one group — fall back to treating each as its own group
    // keyed by array position.
    const turns: ParsedTurn[] = [
      t({ block_kind: '', content: 'a' }),
      t({ block_kind: '', content: 'b' }),
      t({ block_kind: '', content: 'c' }),
    ];

    const groups = groupTurns(turns);
    expect(groups).toHaveLength(3);
  });

  it('extracts the tool name when a tool_use block is present', () => {
    const turns: ParsedTurn[] = [
      t({ turn_index: 0, parent_turn_index: 0, block_kind: 'thinking' }),
      t({ turn_index: 1, parent_turn_index: 0, block_kind: 'tool_use', tool_name: 'Edit' }),
    ];

    const groups = groupTurns(turns);
    expect(groups[0]?.toolName).toBe('Edit');
  });

  it('returns an empty array for empty input', () => {
    expect(groupTurns([])).toEqual([]);
  });
});
