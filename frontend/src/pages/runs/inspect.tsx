import { useMemo, useState } from 'react';
import { useParams } from 'react-router-dom';

import { groupTurns, TurnList } from '../../components/run-inspector';
import { ToolHistogram } from '../../components/run-inspector/ToolHistogram';
import { TurnDiffPanel } from '../../components/run-inspector/TurnDiffPanel';
import { ErrorState, LoadingSkeleton } from '../../components/system';
import { useRun, useRunTurns, useTranscript } from '../../lib/hooks';
import { buildToolHistogram } from '../../lib/tool-histogram';
import { usePerTurnDiff } from '../../lib/use-per-turn-diff';

/**
 * Run Inspector V2 — `/runs/:id/inspect`.
 *
 * Three-pane shell:
 *   - top: run header (status, variant, timing)
 *   - left: virtualized turn list (#62)
 *   - right: per-turn diff + tool histogram sidebar (this story, #63)
 *
 * The right pane updates when the user focuses a turn in the list.
 * "No focus" shows the tool histogram as the at-a-glance summary;
 * once a turn is selected, the diff for that step takes the top
 * slot and the histogram drops below it.
 */

export function RunInspectPage() {
  const { id } = useParams<{ id: string }>();
  const runQuery = useRun(id);
  const turnsQuery = useRunTurns(id);
  // The full transcript fetch is what gives us `patch`. It's a heavier
  // payload than `useRunTurns`, but only this route needs it — and
  // TanStack Query dedupes per key so a second consumer is free.
  const transcriptQuery = useTranscript(id);
  const [focusedParentIndex, setFocusedParentIndex] = useState<number | null>(null);

  const turns = turnsQuery.data ?? [];
  const groups = useMemo(() => groupTurns(turns), [turns]);
  const histogramRows = useMemo(() => buildToolHistogram(turns), [turns]);

  const focusedGroup = useMemo(
    () =>
      focusedParentIndex === null
        ? null
        : groups.find((g) => g.parentTurnIndex === focusedParentIndex) ?? null,
    [groups, focusedParentIndex],
  );
  const diffs = usePerTurnDiff(transcriptQuery.data?.patch, focusedGroup?.blocks);

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
          {run && (
            <div className="text-xs text-fg-muted">
              status: <span className="font-mono text-fg">{run.status}</span>
              {' · '}
              variant: <span className="font-mono text-fg">{run.variant_id}</span>
            </div>
          )}
        </div>
      </header>

      <div className="grid min-h-0 flex-1 grid-cols-1 gap-4 md:grid-cols-[1fr_minmax(320px,_420px)]">
        <section
          className="min-h-0 rounded-md border border-border bg-bg-elev-1"
          aria-label="Turn list"
        >
          <TurnList turns={turns} onFocusChange={setFocusedParentIndex} />
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
