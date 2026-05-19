import { DiffBadge, FilePath } from '../system';
import type { FileDiff } from '../../lib/parse-patch';

/**
 * TurnDiffPanel — right-pane content for one focused turn.
 *
 * Lists every file the focused turn group touched, each row showing
 * the path chip + a DiffBadge (+N / −M) and the raw hunk text below.
 *
 * Empty state covers two cases — no focused turn yet (caller passes
 * an empty `diffs` array) and a focused turn that didn't modify any
 * file (a pure thinking / reply group). The two read the same to the
 * user; we don't try to distinguish them.
 *
 * Why no syntax-highlighting: the raw hunks already carry `+`/`-`
 * line prefixes and a monospace block is enough signal for a thesis
 * demo. Highlighting can come later behind a token-aware highlighter
 * (story #74 follow-up).
 */
export function TurnDiffPanel({ diffs }: { diffs: FileDiff[] }) {
  if (diffs.length === 0) {
    return (
      <div className="text-sm text-fg-muted">
        This turn didn't modify any files.
      </div>
    );
  }
  return (
    <div className="space-y-4">
      {diffs.map((diff) => (
        <article key={diff.path} className="rounded-md border border-border bg-bg-elev-2">
          <header className="flex items-center justify-between gap-2 border-b border-border px-3 py-2">
            <FilePath path={diff.path} />
            <DiffBadge added={diff.added} removed={diff.removed} />
          </header>
          <pre className="overflow-x-auto whitespace-pre px-3 py-2 font-mono text-xs leading-relaxed text-fg">
            {diff.hunks || '(no hunks)'}
          </pre>
        </article>
      ))}
    </div>
  );
}
