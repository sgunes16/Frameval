import { useMemo } from 'react';

import type { ArtifactVersion } from '../../lib/types';

/**
 * ArtifactDiff — minimal side-by-side renderer for two variants of
 * the same artifact_type (e.g. CLAUDE.md from variant A vs variant B).
 *
 * Splits content by `\n\n` paragraphs so the trigram cross-link can
 * highlight at paragraph granularity. Each paragraph carries an
 * onMouseEnter handler that surfaces its text to the parent — the
 * Artifacts tab forwards that into useArtifactTapeLink.
 *
 * The diff itself is intentionally simple: paragraph-level
 * equality, no character-level diff highlighting. Future work can
 * layer LCS-based diff coloring on top.
 */

interface ArtifactDiffProps {
  left: ArtifactVersion;
  right: ArtifactVersion;
  onParagraphHover?: (paragraph: string | null) => void;
  /** Paragraphs (by indexOf) that the parent has marked as active. */
  activeLeftParagraph?: number | null;
  activeRightParagraph?: number | null;
}

function splitParagraphs(text: string): string[] {
  return text.split(/\n{2,}/).map((p) => p.trim()).filter((p) => p.length > 0);
}

export function ArtifactDiff({
  left,
  right,
  onParagraphHover,
  activeLeftParagraph,
  activeRightParagraph,
}: ArtifactDiffProps) {
  const leftParas = useMemo(() => splitParagraphs(left.content), [left.content]);
  const rightParas = useMemo(() => splitParagraphs(right.content), [right.content]);
  return (
    <div className="grid gap-3 md:grid-cols-2">
      <ArtifactPane
        artifact={left}
        paragraphs={leftParas}
        active={activeLeftParagraph}
        side="left"
        onParagraphHover={onParagraphHover}
      />
      <ArtifactPane
        artifact={right}
        paragraphs={rightParas}
        active={activeRightParagraph}
        side="right"
        onParagraphHover={onParagraphHover}
      />
    </div>
  );
}

function ArtifactPane({
  artifact,
  paragraphs,
  active,
  side,
  onParagraphHover,
}: {
  artifact: ArtifactVersion;
  paragraphs: string[];
  active?: number | null;
  side: 'left' | 'right';
  onParagraphHover?: (paragraph: string | null) => void;
}) {
  return (
    <article
      className="rounded-md border border-border bg-bg-elev-1"
      aria-label={`${side === 'left' ? 'Left' : 'Right'} artifact ${artifact.artifact_type}`}
    >
      <header className="flex items-center justify-between border-b border-border bg-bg-elev-2 px-3 py-2 text-xs">
        <span className="font-mono font-medium text-fg">{artifact.artifact_type}</span>
        <span className="font-mono text-fg-muted">{artifact.file_path}</span>
      </header>
      <div className="space-y-2 p-3 text-sm text-fg">
        {paragraphs.map((p, i) => (
          // Paragraphs are focusable so keyboard / AT users get the
          // same cross-highlight as mouse users. Focus mirrors hover,
          // blur mirrors mouse-leave. The visible bg change still
          // signals which paragraph drives the Tape highlight.
          <p
            key={i}
            data-paragraph-index={i}
            tabIndex={0}
            onMouseEnter={() => onParagraphHover?.(p)}
            onMouseLeave={() => onParagraphHover?.(null)}
            onFocus={() => onParagraphHover?.(p)}
            onBlur={() => onParagraphHover?.(null)}
            className={
              'rounded-sm px-2 py-1 transition focus:outline-none focus-visible:ring-2 focus-visible:ring-accent ' +
              (active === i ? 'bg-accent/15 border-l-2 border-accent' : 'hover:bg-bg-elev-2')
            }
          >
            {p}
          </p>
        ))}
      </div>
    </article>
  );
}
