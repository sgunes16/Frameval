import { describe, expect, it } from 'vitest';

import { filterDiffForTurn } from './per-turn-diff';
import type { FileDiff } from './parse-patch';
import type { ParsedTurn } from './types';

const file = (path: string, added: number, removed: number): FileDiff => ({
  path,
  added,
  removed,
  hunks: `@@ ${path} @@`,
});

const turn = (
  blockKind: ParsedTurn['block_kind'],
  filesTouched: string[],
  parent = 0,
): ParsedTurn => ({
  role: 'assistant',
  content: '',
  block_kind: blockKind,
  parent_turn_index: parent,
  files_touched: filesTouched,
});

describe('filterDiffForTurn', () => {
  it('returns empty array when the patch map is empty', () => {
    expect(filterDiffForTurn(new Map(), [turn('tool_use', ['src/main.go'])])).toEqual([]);
  });

  it('returns empty array when no block in the group has files_touched', () => {
    const patch = new Map([['src/main.go', file('src/main.go', 2, 1)]]);
    const blocks: ParsedTurn[] = [
      { role: 'assistant', content: 'thinking', block_kind: 'thinking' },
      { role: 'assistant', content: 'reply', block_kind: 'text' },
    ];
    expect(filterDiffForTurn(patch, blocks)).toEqual([]);
  });

  it('returns the FileDiff entries matching files_touched across all blocks in the group', () => {
    const patch = new Map([
      ['src/main.go', file('src/main.go', 2, 1)],
      ['README.md', file('README.md', 2, 0)],
      ['other.ts', file('other.ts', 5, 5)],
    ]);
    const blocks = [
      turn('tool_use', ['src/main.go']),
      turn('tool_result', ['README.md']),
    ];
    const result = filterDiffForTurn(patch, blocks);
    expect(result.map((f) => f.path).sort()).toEqual(['README.md', 'src/main.go']);
  });

  it('deduplicates files referenced by multiple blocks', () => {
    const patch = new Map([['src/main.go', file('src/main.go', 2, 1)]]);
    const blocks = [
      turn('tool_use', ['src/main.go']),
      turn('tool_result', ['src/main.go']),
    ];
    const result = filterDiffForTurn(patch, blocks);
    expect(result).toHaveLength(1);
    expect(result[0]?.path).toBe('src/main.go');
  });

  it('silently drops files referenced by a tool but missing from the patch', () => {
    const patch = new Map([['src/main.go', file('src/main.go', 2, 1)]]);
    const blocks = [turn('tool_use', ['src/main.go', 'src/ghost.ts'])];
    const result = filterDiffForTurn(patch, blocks);
    expect(result.map((f) => f.path)).toEqual(['src/main.go']);
  });
});
