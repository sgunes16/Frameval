import { describe, expect, it } from 'vitest';

import { applyTurnFilters, parseFilterTokens, serializeFilters, type TurnFilter } from './turn-filters';
import type { ParsedTurn } from './types';

const t = (
  blockKind: ParsedTurn['block_kind'],
  overrides: Partial<ParsedTurn> = {},
): ParsedTurn => ({
  role: 'assistant',
  content: '',
  block_kind: blockKind,
  turn_index: 0,
  parent_turn_index: 0,
  ...overrides,
});

describe('applyTurnFilters', () => {
  it('returns the input unchanged when filters is empty', () => {
    const turns = [t('text'), t('tool_use')];
    expect(applyTurnFilters(turns, [])).toEqual(turns);
  });

  it('keeps only blocks of the requested block_kind for kind filters', () => {
    const turns = [
      t('text', { turn_index: 0 }),
      t('tool_use', { turn_index: 1 }),
      t('thinking', { turn_index: 2 }),
    ];
    const result = applyTurnFilters(turns, [{ kind: 'block', value: 'tool_use' }]);
    expect(result).toHaveLength(1);
    expect(result[0]?.block_kind).toBe('tool_use');
  });

  it('AND-combines multiple block filters as a union (any matching block_kind passes)', () => {
    // Multiple block kinds are an OR within the kind dimension —
    // "tool_use OR thinking" — but AND across dimensions.
    const turns = [t('text'), t('tool_use'), t('thinking')];
    const result = applyTurnFilters(turns, [
      { kind: 'block', value: 'tool_use' },
      { kind: 'block', value: 'thinking' },
    ]);
    expect(result.map((r) => r.block_kind)).toEqual(['tool_use', 'thinking']);
  });

  it('path filter matches files_touched substring case-insensitively', () => {
    const turns = [
      t('tool_use', { turn_index: 0, files_touched: ['src/main.go'] }),
      t('tool_use', { turn_index: 1, files_touched: ['README.md'] }),
      t('tool_use', { turn_index: 2, files_touched: [] }),
    ];
    const result = applyTurnFilters(turns, [{ kind: 'path', value: 'src/' }]);
    expect(result).toHaveLength(1);
    expect(result[0]?.turn_index).toBe(0);
  });

  it('errors_only keeps tool_result blocks containing "error" plus any stage=error', () => {
    const turns = [
      t('tool_result', { turn_index: 0, content: 'success' }),
      t('tool_result', { turn_index: 1, content: 'Error: rate limit' }),
      t('text', { turn_index: 2, stage: 'error', content: 'bail' }),
    ];
    const result = applyTurnFilters(turns, [{ kind: 'errors_only', value: '' }]);
    expect(result.map((r) => r.turn_index)).toEqual([1, 2]);
  });

  it('AND-combines block-kind and path filters across dimensions', () => {
    const turns = [
      t('tool_use', { turn_index: 0, files_touched: ['src/main.go'] }),
      t('text', { turn_index: 1, files_touched: ['src/main.go'] }),
      t('tool_use', { turn_index: 2, files_touched: ['README.md'] }),
    ];
    const result = applyTurnFilters(turns, [
      { kind: 'block', value: 'tool_use' },
      { kind: 'path', value: 'src/' },
    ]);
    expect(result.map((r) => r.turn_index)).toEqual([0]);
  });
});

describe('parseFilterTokens / serializeFilters', () => {
  it('round-trips simple block filters', () => {
    const filters: TurnFilter[] = [{ kind: 'block', value: 'tool_use' }];
    const tokens = serializeFilters(filters);
    expect(tokens).toEqual(['tool_use']);
    expect(parseFilterTokens(tokens)).toEqual(filters);
  });

  it('round-trips path filters with the path: prefix', () => {
    const filters: TurnFilter[] = [{ kind: 'path', value: 'src/main.go' }];
    expect(serializeFilters(filters)).toEqual(['path:src/main.go']);
    expect(parseFilterTokens(['path:src/main.go'])).toEqual(filters);
  });

  it('round-trips the errors_only synonym', () => {
    expect(parseFilterTokens(['errors_only'])).toEqual([{ kind: 'errors_only', value: '' }]);
    expect(serializeFilters([{ kind: 'errors_only', value: '' }])).toEqual(['errors_only']);
  });

  it('ignores empty and unknown tokens', () => {
    expect(parseFilterTokens(['', 'unknown', 'tool_use'])).toEqual([
      { kind: 'block', value: 'tool_use' },
    ]);
  });
});
