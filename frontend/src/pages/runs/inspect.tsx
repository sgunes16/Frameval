import { useState } from 'react';
import { useParams } from 'react-router-dom';

import { TurnList } from '../../components/run-inspector';
import { ErrorState, LoadingSkeleton } from '../../components/system';
import { useRun, useRunTurns } from '../../lib/hooks';

/**
 * Run Inspector V2 — `/runs/:id/inspect`.
 *
 * Three-pane shell:
 *   - top: run header (status, variant, timing)
 *   - left (this PR): virtualized turn list
 *   - right (next stories #63/#64): per-turn diff + symptoms + search
 *
 * The right pane lands incrementally; this PR ships the page route +
 * shell + turn list. The right pane is a placeholder that reflects
 * the focused turn so the wiring is in place when the diff panel
 * actually renders.
 */

export function RunInspectPage() {
  const { id } = useParams<{ id: string }>();
  const runQuery = useRun(id);
  const turnsQuery = useRunTurns(id);
  const [focusedParentIndex, setFocusedParentIndex] = useState<number | null>(null);

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
  const turns = turnsQuery.data ?? [];

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

      <div className="grid min-h-0 flex-1 grid-cols-1 gap-4 md:grid-cols-[1fr_minmax(280px,_360px)]">
        <section
          className="min-h-0 rounded-md border border-border bg-bg-elev-1"
          aria-label="Turn list"
        >
          <TurnList turns={turns} onFocusChange={setFocusedParentIndex} />
        </section>

        <aside
          className="rounded-md border border-border bg-bg-elev-1 p-4"
          aria-label="Turn detail"
        >
          <div className="text-xs uppercase tracking-wider text-fg-muted">Focused turn</div>
          {focusedParentIndex === null ? (
            <div className="mt-2 text-sm text-fg-muted">
              Click any turn on the left to see its diff + symptoms here.
            </div>
          ) : (
            <div className="mt-2 font-mono text-sm text-fg">Turn {focusedParentIndex}</div>
          )}
          <div className="mt-3 text-xs text-fg-subtle">
            Diff panel and symptom links land in #63 / #64.
          </div>
        </aside>
      </div>
    </div>
  );
}
