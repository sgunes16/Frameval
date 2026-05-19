import { describe, expect, it } from 'vitest';

import { searchTurns } from './turn-search';
import type { ParsedTurn } from './types';

const tu = (
  blockKind: ParsedTurn['block_kind'],
  content: string,
  overrides: Partial<ParsedTurn> = {},
): ParsedTurn => ({
  role: 'assistant',
  content,
  block_kind: blockKind,
  turn_index: 0,
  parent_turn_index: 0,
  ...overrides,
});

describe('searchTurns', () => {
  it('returns empty array for empty query', () => {
    const turns = [tu('text', 'hello world')];
    expect(searchTurns(turns, '')).toEqual([]);
    expect(searchTurns(turns, '   ')).toEqual([]);
  });

  it('returns empty array when no turn matches', () => {
    const turns = [tu('text', 'hello world')];
    expect(searchTurns(turns, 'nope')).toEqual([]);
  });

  it('matches case-insensitively against content', () => {
    const turns = [
      tu('text', 'Rate Limit exceeded', { turn_index: 0, parent_turn_index: 0 }),
      tu('thinking', 'no match here', { turn_index: 1, parent_turn_index: 1 }),
    ];
    const results = searchTurns(turns, 'rate limit');
    expect(results).toHaveLength(1);
    expect(results[0]?.turn.parent_turn_index).toBe(0);
  });

  it('matches against tool_name and files_touched', () => {
    const turns = [
      tu('tool_use', '', {
        turn_index: 0,
        parent_turn_index: 0,
        tool_name: 'Bash',
        files_touched: ['src/main.go'],
      }),
    ];
    expect(searchTurns(turns, 'bash')).toHaveLength(1);
    expect(searchTurns(turns, 'src/main')).toHaveLength(1);
  });

  it('ranks results by match count desc, then by turn_index asc', () => {
    const turns = [
      tu('text', 'foo bar', { turn_index: 0, parent_turn_index: 0 }),
      tu('text', 'foo foo foo', { turn_index: 1, parent_turn_index: 1 }),
      tu('text', 'foo', { turn_index: 2, parent_turn_index: 2 }),
    ];
    const results = searchTurns(turns, 'foo');
    expect(results.map((r) => r.turn.parent_turn_index)).toEqual([1, 0, 2]);
  });

  it('returns a snippet centered on the first match with ellipses for context overflow', () => {
    const long = 'a'.repeat(60) + ' MATCH ' + 'b'.repeat(60);
    const turns = [tu('text', long)];
    const [result] = searchTurns(turns, 'MATCH');
    expect(result?.snippet).toContain('MATCH');
    expect(result?.snippet.startsWith('…')).toBe(true);
    expect(result?.snippet.endsWith('…')).toBe(true);
  });

  it('does not double-count multi-token queries (treats as a single substring)', () => {
    // "rate limit" should not count separately as "rate" + "limit"
    // for ranking — we want phrase matches to rank higher than
    // accidental co-occurrence.
    const turns = [
      tu('text', 'rate limit exceeded', { turn_index: 0, parent_turn_index: 0 }),
      tu('text', 'rate is fine but limit is not', { turn_index: 1, parent_turn_index: 1 }),
    ];
    const results = searchTurns(turns, 'rate limit');
    expect(results).toHaveLength(1);
    expect(results[0]?.turn.parent_turn_index).toBe(0);
  });
});
