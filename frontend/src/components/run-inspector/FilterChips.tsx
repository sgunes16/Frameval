import { useState } from 'react';

import type { TurnFilter } from '../../lib/turn-filters';
import type { BlockKind } from '../../lib/types';

/**
 * FilterChips — chip toggles above the Inspector V2 turn list.
 *
 * Owns no state of its own beyond a transient `pathDraft` for the
 * path input. The active filter set lives in the parent (so it can
 * sync with URL state) and is passed back as the canonical
 * TurnFilter[] shape. The parent decides whether to apply via
 * applyTurnFilters or to forward to URL search params.
 *
 * Chip set: thinking / tool_use / tool_result / errors only / path.
 * Path filter is a free-text substring match; users add via a small
 * inline input. Each active filter renders as a removable chip on
 * the right side of the row so the active set is visible.
 */

interface FilterChipsProps {
  filters: TurnFilter[];
  onChange: (next: TurnFilter[]) => void;
}

const BLOCK_TOGGLES: Array<{ value: BlockKind; label: string }> = [
  { value: 'thinking', label: 'Thinking' },
  { value: 'tool_use', label: 'Tool use' },
  { value: 'tool_result', label: 'Tool result' },
];

function hasBlock(filters: TurnFilter[], value: BlockKind): boolean {
  return filters.some((f) => f.kind === 'block' && f.value === value);
}

function hasErrorsOnly(filters: TurnFilter[]): boolean {
  return filters.some((f) => f.kind === 'errors_only');
}

export function FilterChips({ filters, onChange }: FilterChipsProps) {
  const [pathDraft, setPathDraft] = useState('');

  const toggleBlock = (value: BlockKind) => {
    if (hasBlock(filters, value)) {
      onChange(filters.filter((f) => !(f.kind === 'block' && f.value === value)));
    } else {
      onChange([...filters, { kind: 'block', value }]);
    }
  };

  const toggleErrorsOnly = () => {
    if (hasErrorsOnly(filters)) {
      onChange(filters.filter((f) => f.kind !== 'errors_only'));
    } else {
      onChange([...filters, { kind: 'errors_only', value: '' }]);
    }
  };

  const addPath = (value: string) => {
    const trimmed = value.trim();
    if (!trimmed) return;
    if (filters.some((f) => f.kind === 'path' && f.value === trimmed)) return;
    onChange([...filters, { kind: 'path', value: trimmed }]);
    setPathDraft('');
  };

  const removeFilter = (target: TurnFilter) => {
    onChange(
      filters.filter(
        (f) => !(f.kind === target.kind && (f.kind !== 'path' || f.value === target.value)),
      ),
    );
  };

  return (
    <div className="flex flex-wrap items-center gap-2 px-3 py-2" role="toolbar" aria-label="Turn filters">
      {BLOCK_TOGGLES.map(({ value, label }) => (
        <button
          key={value}
          type="button"
          aria-pressed={hasBlock(filters, value)}
          onClick={() => toggleBlock(value)}
          className={`rounded-full border px-2.5 py-0.5 text-xs transition ${
            hasBlock(filters, value)
              ? 'border-accent bg-accent text-bg'
              : 'border-border bg-bg-elev-2 text-fg-muted hover:bg-bg-elev-1'
          }`}
        >
          {label}
        </button>
      ))}
      <button
        type="button"
        aria-pressed={hasErrorsOnly(filters)}
        onClick={toggleErrorsOnly}
        className={`rounded-full border px-2.5 py-0.5 text-xs transition ${
          hasErrorsOnly(filters)
            ? 'border-danger bg-danger text-bg'
            : 'border-border bg-bg-elev-2 text-fg-muted hover:bg-bg-elev-1'
        }`}
      >
        Errors only
      </button>

      <form
        onSubmit={(e) => {
          e.preventDefault();
          addPath(pathDraft);
        }}
        className="flex items-center gap-1"
      >
        <label className="sr-only" htmlFor="path-filter-input">
          Filter by file path
        </label>
        <input
          id="path-filter-input"
          type="text"
          value={pathDraft}
          onChange={(e) => setPathDraft(e.target.value)}
          placeholder="path:src/"
          className="w-32 rounded-md border border-border bg-bg-elev-2 px-2 py-0.5 text-xs text-fg placeholder:text-fg-subtle focus:outline-none focus:ring-1 focus:ring-accent"
        />
      </form>

      {filters
        .filter((f) => f.kind === 'path')
        .map((f) => (
          <span
            key={`path-${f.value}`}
            className="inline-flex items-center gap-1 rounded-full border border-border bg-bg-elev-2 px-2 py-0.5 font-mono text-xs text-fg"
          >
            path:{f.value}
            <button
              type="button"
              aria-label={`Remove ${f.value} filter`}
              onClick={() => removeFilter(f)}
              className="text-fg-subtle hover:text-fg"
            >
              ×
            </button>
          </span>
        ))}
    </div>
  );
}
