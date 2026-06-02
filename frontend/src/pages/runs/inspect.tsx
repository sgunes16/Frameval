import { useCallback, useEffect, useMemo, useState } from 'react';
import { useNavigate, useParams, useSearchParams } from 'react-router-dom';

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
import { Badge } from '../../components/ui/badge';
import { Button } from '../../components/ui/button';
import { Card } from '../../components/ui/card';
import { useDiagnostic, useReparseRun, useRetryRun, useRun, useRunTurns, useSpecKitCatalog, useTranscript, useVariants } from '../../lib/hooks';
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

const STATUS_TONE: Partial<Record<string, 'success' | 'danger' | 'info' | 'muted' | 'warning' | 'neutral'>> = {
  completed: 'success',
  failed: 'danger',
  running: 'info',
  cancelled: 'muted',
  pending: 'warning',
};

export function RunInspectPage() {
  const { id } = useParams<{ id: string }>();
  const [searchParams, setSearchParams] = useSearchParams();
  const navigate = useNavigate();

  const runQuery = useRun(id);
  const turnsQuery = useRunTurns(id);
  const transcriptQuery = useTranscript(id);
  const diagnosticQuery = useDiagnostic(id);
  const variantsQuery = useVariants(runQuery.data?.experiment_id);
  // Live-streams run.turn events for the focused run and invalidates
  // the turns + transcript queries on each event. Re-connect on
  // socket drop reconciles missed turns via the next REST refetch.
  const stream = useTurnStream(id);
  const reparse = useReparseRun();
  const retry = useRetryRun();

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
  const variant = variantsQuery.data?.find((v) => v.id === run?.variant_id);
  const speckitExtensionId = typeof (variant?.harness_config?.speckit as { extension_id?: unknown } | undefined)?.extension_id === 'string'
    ? (variant!.harness_config!.speckit as { extension_id: string }).extension_id
    : undefined;

  return (
    <div className="flex h-[calc(100vh-8rem)] flex-col gap-4">
      <header className="rounded-md border border-border bg-bg-elev-1 px-4 py-3">
        <div className="flex flex-wrap items-center justify-between gap-x-4 gap-y-3">
          {/* Identity: status + run id + variant name */}
          <div className="min-w-0">
            <div className="flex items-center gap-2">
              <span className="text-xs font-medium uppercase tracking-wider text-fg-muted">Run</span>
              {run && (
                <Badge tone={STATUS_TONE[run.status] ?? 'neutral'} className="capitalize">
                  {run.status}
                </Badge>
              )}
              {/*
                LiveCursor only makes sense while the run is actively
                producing turns. A terminal run still has the WS connected
                and lastEventAt populated from the final turn, but rendering
                "Live" for a finished run would be misleading.
              */}
              {run?.status === 'running' && (
                <LiveCursor
                  isConnected={stream.isConnected}
                  lastEventAt={stream.lastEventAt}
                  turnCount={stream.lastTurnCount}
                />
              )}
            </div>
            <div className="mt-1 flex items-center gap-2 text-sm">
              <span className="truncate font-mono text-fg" title={id}>{id}</span>
              {variant && (
                <>
                  <span className="text-fg-muted">·</span>
                  <span className="truncate font-mono text-xs text-fg-muted" title={variant.name ?? run?.variant_id}>
                    {variant.name ?? run?.variant_id}
                  </span>
                </>
              )}
            </div>
          </div>

          {/* Actions */}
          <div className="flex flex-shrink-0 items-center gap-2">
            <InspectorSearch turns={turns} onFocus={setFocusedParentIndex} />
            {/*
              Retry re-queues a terminal run (backend resets it to pending and
              re-executes); the run/runs queries are invalidated so this view
              flips back to running. Handy for runs that died on a transient —
              a stalled model, a manually-killed sandbox.
            */}
            {id && run && (run.status === 'failed' || run.status === 'completed' || run.status === 'cancelled') && (
              <Button
                size="sm"
                variant="outline"
                className="whitespace-nowrap"
                disabled={retry.isPending}
                onClick={() => {
                  // Retrying a completed run discards its existing grade, so
                  // guard that case; failed/cancelled runs have nothing to lose.
                  if (
                    run.status === 'completed' &&
                    !window.confirm('This run already completed with a grade. Retrying discards it and re-runs the variant. Continue?')
                  ) {
                    return;
                  }
                  retry.mutate(id);
                }}
                title="Reset this run to pending and run it again"
              >
                {retry.isPending ? 'Retrying…' : 'Retry'}
              </Button>
            )}
            {/*
              Re-parse rewires the persisted ParsedTurns from raw_output using
              the latest executor parser — useful when a parser improvement
              landed after this run finished. Only meaningful for terminal runs.
            */}
            {id && run && run.status !== 'running' && (
              <Button
                size="sm"
                variant="ghost"
                className="whitespace-nowrap"
                disabled={reparse.isPending}
                onClick={() => reparse.mutate(id)}
                title="Re-parse the stored raw output with the latest parser"
              >
                {reparse.isPending ? 'Re-parsing…' : 'Re-parse'}
              </Button>
            )}
            {id && (
              <Button
                size="sm"
                variant="ghost"
                className="whitespace-nowrap"
                onClick={() => navigate(`/runs/${id}/grading`)}
              >
                View grading →
              </Button>
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

          {variant?.harness_config?.agent_instructions != null && (
            <div className="mt-3">
              <AgentInstructionsCard
                content={String((variant.harness_config.agent_instructions as { content?: string }).content ?? '')}
              />
            </div>
          )}

          {speckitExtensionId && (
            <div className="mt-3">
              <SpecKitExtensionCard extensionId={speckitExtensionId} />
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

function AgentInstructionsCard({ content }: { content: string }) {
  const [open, setOpen] = useState(true);
  if (!content.trim()) return null;
  return (
    <Card>
      <div className="flex items-center justify-between">
        <div>
          <div className="text-sm font-semibold text-fg">Agent instructions used in this run</div>
          <div className="text-xs text-fg-muted">Laid down as CLAUDE.md in the workspace before the agent ran.</div>
        </div>
        <button
          type="button"
          onClick={() => setOpen((v) => !v)}
          className="text-xs text-fg-muted hover:text-fg"
        >
          {open ? 'Hide' : 'Show'}
        </button>
      </div>
      {open && (
        <pre className="mt-3 max-h-64 overflow-auto rounded-md border border-border bg-bg p-3 font-mono text-xs text-fg">
          {content}
        </pre>
      )}
    </Card>
  );
}

function SpecKitExtensionCard({ extensionId }: { extensionId: string }) {
  const [open, setOpen] = useState(true);
  const query = useSpecKitCatalog();
  const ext = query.data?.find((e) => e.id === extensionId);
  if (!ext) {
    return (
      <Card>
        <div className="text-sm font-semibold text-fg">Spec-kit extension</div>
        <div className="text-xs text-fg-muted">
          Loading catalog or extension <code className="font-mono">{extensionId}</code> not found.
        </div>
      </Card>
    );
  }
  return (
    <Card>
      <div className="flex items-center justify-between">
        <div>
          <div className="flex items-center gap-2 text-sm font-semibold text-fg">
            Spec-kit extension
            {ext.multi_agent && (
              <span className="rounded-full border border-border bg-bg-elev-2 px-1.5 py-0.5 text-xs uppercase tracking-wider text-fg-subtle">
                Multi-agent
              </span>
            )}
          </div>
          <div className="text-xs text-fg-muted">{ext.name} — {ext.description}</div>
        </div>
        <button
          type="button"
          onClick={() => setOpen((v) => !v)}
          className="text-xs text-fg-muted hover:text-fg"
        >
          {open ? 'Hide' : 'Show'}
        </button>
      </div>
      {open && (
        <ol className="mt-3 space-y-1 text-xs text-fg-muted">
          {ext.stages.map((st, i) => (
            <li key={st.name} className="font-mono">
              {i + 1}. {st.name}{' '}
              <span className="text-fg-subtle">({st.slash_command})</span>
              {st.role && <span className="ml-2 text-fg-muted">role: {st.role}</span>}
            </li>
          ))}
        </ol>
      )}
    </Card>
  );
}
