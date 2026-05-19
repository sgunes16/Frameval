import type { ParsedTurn } from './types';

/**
 * One row of the Inspector V2 tool histogram sidebar — "Edit ×7", etc.
 *
 * Counts are computed across the entire run: each tool_use block adds 1
 * to its tool's bucket. tool_result blocks are intentionally ignored
 * (one user-facing tool call = one tool_use; counting tool_result would
 * double everything and break the intuition behind the bar height).
 */
export interface ToolHistogramRow {
  tool: string;
  count: number;
}

export function buildToolHistogram(turns: ParsedTurn[]): ToolHistogramRow[] {
  const counts = new Map<string, number>();
  for (const turn of turns) {
    if (turn.block_kind !== 'tool_use') continue;
    const name = turn.tool_name?.trim();
    if (!name) continue;
    counts.set(name, (counts.get(name) ?? 0) + 1);
  }

  return Array.from(counts.entries())
    .map(([tool, count]) => ({ tool, count }))
    .sort((a, b) => {
      if (b.count !== a.count) return b.count - a.count;
      return a.tool.localeCompare(b.tool);
    });
}
