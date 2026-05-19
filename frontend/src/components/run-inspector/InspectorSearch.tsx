import { useEffect, useMemo, useRef, useState } from 'react';

import { Kbd } from '../system';
import type { ParsedTurn } from '../../lib/types';
import { searchTurns, type TurnSearchResult } from '../../lib/turn-search';

/**
 * InspectorSearch — Cmd-K palette overlay for the Run Inspector V2.
 *
 * Modal lifecycle:
 *   - Cmd-K / Ctrl-K toggles open. Esc closes.
 *   - ↑/↓ cycle through results, Enter focuses the highlighted turn.
 *
 * Results are computed via `searchTurns` (pure function, see
 * lib/turn-search.ts) so this component stays a thin shell over the
 * data path.
 *
 * Why the palette is a modal rather than an inline filter: the user's
 * working set during a long run is the visible turn list. A modal
 * gives the keyboard search workflow its own scoped surface without
 * stealing the list's scroll position.
 */

interface InspectorSearchProps {
  turns: ParsedTurn[];
  onFocus: (parentTurnIndex: number) => void;
}

export function InspectorSearch({ turns, onFocus }: InspectorSearchProps) {
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState('');
  const [selected, setSelected] = useState(0);
  const inputRef = useRef<HTMLInputElement | null>(null);

  // Global Cmd-K / Ctrl-K listener. Bound once at mount; cleanup on
  // unmount keeps test renders from leaking listeners across runs.
  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 'k') {
        e.preventDefault();
        setOpen((prev) => !prev);
      }
      if (e.key === 'Escape') setOpen(false);
    };
    window.addEventListener('keydown', handler);
    return () => window.removeEventListener('keydown', handler);
  }, []);

  useEffect(() => {
    if (open) {
      setSelected(0);
      // Defer one tick — the input mounts inside the same effect
      // cycle, so focus() before paint would no-op.
      queueMicrotask(() => inputRef.current?.focus());
    }
  }, [open]);

  const results = useMemo<TurnSearchResult[]>(
    () => searchTurns(turns, query),
    [turns, query],
  );

  if (!open) {
    return (
      <button
        type="button"
        onClick={() => setOpen(true)}
        className="inline-flex items-center gap-2 rounded-md border border-border bg-bg-elev-2 px-2 py-1 text-xs text-fg-muted hover:bg-bg-elev-1"
      >
        <span>Search turns</span>
        <Kbd>⌘K</Kbd>
      </button>
    );
  }

  const choose = (idx: number) => {
    const row = results[idx];
    if (!row) return;
    const parent = row.turn.parent_turn_index ?? row.turn.turn_index ?? 0;
    onFocus(parent);
    setOpen(false);
    setQuery('');
  };

  const onKeyDown = (e: React.KeyboardEvent<HTMLInputElement>) => {
    if (e.key === 'ArrowDown') {
      e.preventDefault();
      setSelected((s) => Math.min(results.length - 1, s + 1));
    } else if (e.key === 'ArrowUp') {
      e.preventDefault();
      setSelected((s) => Math.max(0, s - 1));
    } else if (e.key === 'Enter') {
      e.preventDefault();
      choose(selected);
    }
  };

  return (
    <div
      role="dialog"
      aria-label="Search turns"
      className="fixed inset-0 z-50 flex items-start justify-center bg-bg/60 p-4 pt-24"
      onClick={() => setOpen(false)}
    >
      <div
        className="w-full max-w-xl rounded-md border border-border bg-bg-elev-1 shadow-lg"
        onClick={(e) => e.stopPropagation()}
      >
        <input
          ref={inputRef}
          type="text"
          value={query}
          onChange={(e) => {
            setQuery(e.target.value);
            setSelected(0);
          }}
          onKeyDown={onKeyDown}
          placeholder="Search turn content, tools, file paths…"
          className="w-full rounded-t-md border-b border-border bg-transparent px-4 py-3 text-sm text-fg placeholder:text-fg-subtle focus:outline-none"
          aria-label="Search query"
        />
        <ul
          role="listbox"
          aria-label="Search results"
          className="max-h-80 overflow-y-auto"
        >
          {results.length === 0 && query.trim().length > 0 && (
            <li className="px-4 py-3 text-sm text-fg-muted">No matching turns.</li>
          )}
          {results.map((row, i) => {
            const active = i === selected;
            const parent = row.turn.parent_turn_index ?? row.turn.turn_index ?? 0;
            return (
              <li
                key={`${parent}-${i}`}
                role="option"
                aria-selected={active}
                onMouseEnter={() => setSelected(i)}
                onClick={() => choose(i)}
                className={`cursor-pointer border-b border-border px-4 py-2 text-sm last:border-b-0 ${
                  active ? 'bg-bg-elev-2' : ''
                }`}
              >
                <div className="flex items-baseline justify-between gap-2">
                  <span className="font-mono text-xs text-fg-muted">Turn {parent}</span>
                  {row.turn.tool_name && (
                    <span className="font-mono text-xs text-fg-subtle">{row.turn.tool_name}</span>
                  )}
                </div>
                <div className="mt-0.5 text-fg">{row.snippet || '(no preview)'}</div>
              </li>
            );
          })}
        </ul>
      </div>
    </div>
  );
}
