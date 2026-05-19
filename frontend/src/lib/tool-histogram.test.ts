import { describe, expect, it } from 'vitest';

import { buildToolHistogram } from './tool-histogram';
import type { ParsedTurn } from './types';

const tu = (toolName: string, parent: number): ParsedTurn => ({
  role: 'assistant',
  content: '',
  block_kind: 'tool_use',
  tool_name: toolName,
  parent_turn_index: parent,
});

describe('buildToolHistogram', () => {
  it('returns an empty array on empty input', () => {
    expect(buildToolHistogram([])).toEqual([]);
  });

  it('only counts tool_use blocks (ignores thinking / text / tool_result)', () => {
    const turns: ParsedTurn[] = [
      tu('Edit', 0),
      { role: 'assistant', content: 'reasoning', block_kind: 'thinking' },
      { role: 'tool', content: 'ok', block_kind: 'tool_result', tool_name: 'Edit' },
      { role: 'assistant', content: 'reply', block_kind: 'text' },
    ];
    expect(buildToolHistogram(turns)).toEqual([{ tool: 'Edit', count: 1 }]);
  });

  it('aggregates by tool_name, sorted descending by count then alpha by name', () => {
    const turns: ParsedTurn[] = [
      tu('Read', 0),
      tu('Edit', 1),
      tu('Edit', 2),
      tu('Bash', 3),
      tu('Edit', 4),
    ];
    expect(buildToolHistogram(turns)).toEqual([
      { tool: 'Edit', count: 3 },
      { tool: 'Bash', count: 1 },
      { tool: 'Read', count: 1 },
    ]);
  });

  it('skips tool_use blocks with missing tool_name (defensive)', () => {
    const turns: ParsedTurn[] = [
      { role: 'assistant', content: '', block_kind: 'tool_use' },
      tu('Read', 1),
    ];
    expect(buildToolHistogram(turns)).toEqual([{ tool: 'Read', count: 1 }]);
  });
});
