import { useCallback, useEffect, useMemo, useState } from 'react';
import { useParams, useSearchParams } from 'react-router-dom';

import {
  FilterChips,
  groupTurns,
  InspectorSearch,
  LiveCursor,
  TurnList,
} from '../../components/run-inspector';
import { ToolHistogram } from '../../components/run-inspector/ToolHistogram';
import { TurnDiffPanel } from '../../components/run-inspector/TurnDiffPanel';
import { ErrorState, LoadingSkeleton } from '../../components/system';
import { useDiagnostic, useRun, useRunTurns, useTranscript } from '../../lib/hooks';
import { buildEvidenceByTurn } from '../../lib/symptom-evidence';
import { buildToolHistogram } from '../../lib/tool-histogram';
import {
  applyTurnFilters,
  parseFilterTokens,
  serializeFilters,
  type TurnFilter,
} from '../../lib/turn-filters';
import { usePerTurnDiff } from '../../lib/use-per-turn-diff';
import { useTurnStream } from '../../lib/use-turn-stream';

/**
 * Run Inspector V2 — `/runs/:id/inspect`.
 *
 * Three-pane shell:
 *   - top: run header (status, variant, search, filters)
 *   - left: virtualized turn list (#62)
 *   - right: per-turn diff + tool histogram sidebar (#63)
 *
 * The right pane updates when the user focuses a turn in the list.
 * Filter chips and the focused-turn index are mirrored to URL search
 * params (`?filter=tool_use&filter=path:src/&focus=4`) so reloads,
 * back/forward, and copy-link share the exact same view.
 */

export function RunInspectPage() {
  const { id } = useParams<{ id: string }>();
  const [searchParams, setSearchParams] = useSearchParams();

  const runQuery = useRun(id);
  const turnsQuery = useRunTurns(id);
  const transcriptQuery = useTranscript(id);
  const diagnosticQuery = useDiagnostic(id);
  // Live-streams run.turn events for the focused run and invalidates
  // the turns + transcript queries on each event. Re-connect on
  // socket drop reconciles missed turns via the next REST refetch.
  const stream = useTurnStream(id);

  const filters = useMemo<TurnFilter[]>(
    () => parseFilterTokens(searchParams.getAll('filter')),
    [searchParams],
  );

  const focusedParentIndex = useMemo<number | null>(() => {
    const raw = searchParams.get('focus');
    if (raw === null) return null;
    const parsed = Number.parseInt(raw, 10);
    return Number.isFinite(parsed) ? parsed : null;
  }, [searchParams]);

  const setFocusedParentIndex = useCallback(
    (next: number | null) => {
      setSearchParams(
        (prev) => {
          const params = new URLSearchParams(prev);
          if (next === null) {
            params.delete('focus');
          } else {
            params.set('focus', String(next));
          }
          return params;
        },
        { replace: true },
      );
    },
    [setSearchParams],
  );

  const setFilters = useCallback(
    (next: TurnFilter[]) => {
      setSearchParams(
        (prev) => {
          const params = new URLSearchParams(prev);
          params.delete('filter');
          for (const token of serializeFilters(next)) {
            params.append('filter', token);
          }
          return params;
        },
        { replace: true },
      );
    },
    [setSearchParams],
  );

  const turns = turnsQuery.data ?? [];
  const filteredTurns = useMemo(() => applyTurnFilters(turns, filters), [turns, filters]);
  const groups = useMemo(() => groupTurns(filteredTurns), [filteredTurns]);
  const histogramRows = useMemo(() => buildToolHistogram(turns), [turns]);
  const evidenceByTurn = useMemo(
    () => buildEvidenceByTurn(diagnosticQuery.data),
    [diagnosticQuery.data],
  );

  const focusedGroup = useMemo(
    () =>
      focusedParentIndex === null
        ? null
        : groups.find((g) => g.parentTurnIndex === focusedParentIndex) ?? null,
    [groups, focusedParentIndex],
  );
  const diffs = usePerTurnDiff(transcriptQuery.data?.patch, focusedGroup?.blocks);

  // If the URL's ?focus=N points at a turn that the current filter
  // set hides, drop the focus rather than render the right pane
  // pointing at a turn that isn't in the list. Skip while data is
  // still loading/refetching — `groups.length === 0` during a
  // background refresh would otherwise wipe a valid deep-link.
  useEffect(() => {
    if (focusedParentIndex === null) return;
    if (turnsQuery.isLoading || turnsQuery.isFetching) return;
    if (groups.length === 0) return;
    const stillVisible = groups.some((g) => g.parentTurnIndex === focusedParentIndex);
    if (!stillVisible) setFocusedParentIndex(null);
  }, [focusedParentIndex, groups, setFocusedParentIndex, turnsQuery.isLoading, turnsQuery.isFetching]);

  if (runQuery.isError || turnsQuery.isError) {
    return (
      <ErrorState
        title="Could not load run"
        description="The engine returned an error or the run doesn't exist."
        onRetry={() => {
          runQuery.refetch();
          turnsQuery.refetch();
        }}
      />
    );
  }

  if (runQuery.isLoading || turnsQuery.isLoading) {
    return (
      <div className="space-y-2">
        <LoadingSkeleton variant="row" count={6} />
      </div>
    );
  }

  const run = runQuery.data;

  return (
    <div className="flex h-[calc(100vh-8rem)] flex-col gap-4">
      <header className="rounded-md border border-border bg-bg-elev-1 px-4 py-3">
        <div className="flex items-baseline justify-between">
          <div>
            <div className="text-xs uppercase tracking-wider text-fg-muted">Run</div>
            <div className="font-mono text-sm text-fg">{id}</div>
          </div>
          <div className="flex items-center gap-3">
            <InspectorSearch turns={turns} onFocus={setFocusedParentIndex} />
            <LiveCursor
              isConnected={stream.isConnected}
              lastEventAt={stream.lastEventAt}
              turnCount={stream.lastTurnCount}
            />
            {run && (
              <div className="text-xs text-fg-muted">
                status: <span className="font-mono text-fg">{run.status}</span>
                {' · '}
                variant: <span className="font-mono text-fg">{run.variant_id}</span>
              </div>
            )}
          </div>
        </div>
      </header>

      <div className="grid min-h-0 flex-1 grid-cols-1 gap-4 md:grid-cols-[1fr_minmax(320px,_420px)]">
        <section
          className="flex min-h-0 flex-col rounded-md border border-border bg-bg-elev-1"
          aria-label="Turn list"
        >
          <FilterChips filters={filters} onChange={setFilters} />
          <div className="min-h-0 flex-1 border-t border-border">
            <TurnList
              turns={filteredTurns}
              evidenceByTurn={evidenceByTurn}
              onFocusChange={setFocusedParentIndex}
            />
          </div>
        </section>

        <aside
          className="flex min-h-0 flex-col gap-3 overflow-y-auto rounded-md border border-border bg-bg-elev-1 p-4"
          aria-label="Turn detail"
        >
          {focusedGroup ? (
            <>
              <div>
                <div className="text-xs uppercase tracking-wider text-fg-muted">
                  Focused turn
                </div>
                <div className="mt-1 font-mono text-sm text-fg">
                  Turn {focusedGroup.parentTurnIndex}
                  {focusedGroup.toolName ? ` · ${focusedGroup.toolName}` : ''}
                </div>
              </div>
              <TurnDiffPanel diffs={diffs} />
            </>
          ) : (
            <div className="text-sm text-fg-muted">
              Click any turn on the left to see its diff here.
            </div>
          )}

          <div className="border-t border-border pt-3">
            <div className="mb-2 text-xs uppercase tracking-wider text-fg-muted">
              Tool usage
            </div>
            <ToolHistogram rows={histogramRows} />
          </div>
        </aside>
      </div>
    </div>
  );
}
