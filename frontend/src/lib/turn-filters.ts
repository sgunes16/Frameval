import type { BlockKind, ParsedTurn } from './types';

/**
 * Inspector V2 filter language.
 *
 * Filters AND across `kind` dimensions and OR within the same kind —
 * so `[block:tool_use, block:thinking, path:src/]` matches "blocks
 * that are tool_use OR thinking AND whose files_touched include
 * src/".
 *
 * URL encoding is a flat `?filter=<token>` list. Block kinds use
 * their bare names; path filters use the `path:<substring>` prefix;
 * the errors-only toggle uses the bare token `errors_only`. Tokens
 * the parser doesn't recognise are dropped silently so users editing
 * the URL by hand can't crash the route.
 */

export type TurnFilter =
  | { kind: 'block'; value: BlockKind }
  | { kind: 'path'; value: string }
  | { kind: 'errors_only'; value: '' };

const BLOCK_KINDS: BlockKind[] = ['thinking', 'tool_use', 'tool_result', 'text', 'system'];

function isErrorTurn(turn: ParsedTurn): boolean {
  if (turn.stage === 'error') return true;
  if (turn.block_kind !== 'tool_result') return false;
  return (turn.content ?? '').toLowerCase().includes('error');
}

export function applyTurnFilters(turns: ParsedTurn[], filters: TurnFilter[]): ParsedTurn[] {
  if (filters.length === 0) return turns;

  const blockValues = filters.filter((f) => f.kind === 'block').map((f) => f.value);
  const pathValues = filters
    .filter((f): f is Extract<TurnFilter, { kind: 'path' }> => f.kind === 'path')
    .map((f) => f.value.toLowerCase());
  const errorsOnly = filters.some((f) => f.kind === 'errors_only');

  return turns.filter((turn) => {
    if (blockValues.length > 0 && !blockValues.includes(turn.block_kind ?? ('' as BlockKind))) {
      return false;
    }
    if (pathValues.length > 0) {
      const haystack = (turn.files_touched ?? []).join(' ').toLowerCase();
      if (!pathValues.some((needle) => haystack.includes(needle))) return false;
    }
    if (errorsOnly && !isErrorTurn(turn)) return false;
    return true;
  });
}

export function parseFilterTokens(tokens: string[]): TurnFilter[] {
  const out: TurnFilter[] = [];
  for (const raw of tokens) {
    const token = raw.trim();
    if (!token) continue;
    if (token === 'errors_only') {
      out.push({ kind: 'errors_only', value: '' });
      continue;
    }
    if (token.startsWith('path:')) {
      const value = token.slice('path:'.length);
      if (value) out.push({ kind: 'path', value });
      continue;
    }
    if ((BLOCK_KINDS as string[]).includes(token)) {
      out.push({ kind: 'block', value: token as BlockKind });
      continue;
    }
    // unknown token — silently dropped
  }
  return out;
}

export function serializeFilters(filters: TurnFilter[]): string[] {
  return filters.map((f) => {
    if (f.kind === 'block') return f.value;
    if (f.kind === 'path') return `path:${f.value}`;
    return 'errors_only';
  });
}
