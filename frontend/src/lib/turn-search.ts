import type { ParsedTurn } from './types';

/**
 * One row of the Cmd-K search palette: the matching turn plus the
 * raw match count (for ranking) and a short text snippet to render
 * in the result row.
 *
 * `snippet` is centred on the first match with `…` ellipses when the
 * surrounding text is wider than `SNIPPET_RADIUS_CHARS`. Highlighting
 * the match span itself is the palette's concern, not this hook's —
 * keeping the data plain string makes downstream highlighters easy
 * to write.
 */
export interface TurnSearchResult {
  turn: ParsedTurn;
  matches: number;
  snippet: string;
}

const SNIPPET_RADIUS_CHARS = 40;

/**
 * Build a single searchable haystack string from a turn: content,
 * tool_name, and files_touched. Everything is lowercased once so the
 * caller can match a pre-lowercased query without re-allocating per
 * turn. Search is substring-based; treating the query as a phrase
 * (rather than tokenising) keeps `rate limit` ranked over the noise
 * case `rate is fine but limit is not`.
 */
function haystackFor(turn: ParsedTurn): string {
  const parts = [
    turn.content ?? '',
    turn.tool_name ?? '',
    ...(turn.files_touched ?? []),
  ];
  return parts.join(' ').toLowerCase();
}

function countOccurrences(haystack: string, needle: string): number {
  if (!needle) return 0;
  let count = 0;
  let idx = haystack.indexOf(needle);
  while (idx !== -1) {
    count += 1;
    idx = haystack.indexOf(needle, idx + needle.length);
  }
  return count;
}

function makeSnippet(content: string, needleLower: string): string {
  if (!content) return '';
  const idx = content.toLowerCase().indexOf(needleLower);
  if (idx === -1) return content.slice(0, SNIPPET_RADIUS_CHARS * 2);

  const start = Math.max(0, idx - SNIPPET_RADIUS_CHARS);
  const end = Math.min(content.length, idx + needleLower.length + SNIPPET_RADIUS_CHARS);
  const prefix = start > 0 ? '…' : '';
  const suffix = end < content.length ? '…' : '';
  return `${prefix}${content.slice(start, end)}${suffix}`;
}

export function searchTurns(turns: ParsedTurn[], query: string): TurnSearchResult[] {
  const needle = query.trim().toLowerCase();
  if (!needle) return [];

  const results: TurnSearchResult[] = [];
  for (const turn of turns) {
    const matches = countOccurrences(haystackFor(turn), needle);
    if (matches === 0) continue;
    results.push({ turn, matches, snippet: makeSnippet(turn.content ?? '', needle) });
  }

  results.sort((a, b) => {
    if (b.matches !== a.matches) return b.matches - a.matches;
    return (a.turn.turn_index ?? 0) - (b.turn.turn_index ?? 0);
  });
  return results;
}
