import { useMemo, useState } from 'react';

import { useArtifacts } from '../../lib/hooks';
import type { ArtifactVersion } from '../../lib/types';
import { ArtifactDiff } from './ArtifactDiff';

/**
 * ArtifactsTab — Compare V2's per-variant artifact diff with cross-
 * highlighting hooks back into the Tape tab.
 *
 * Inputs (from the Compare page shell):
 *   - `variantIds`: the variant_id of each selected run (typically
 *     2-5; only first 2 distinct variants are diffed here, since the
 *     diff renderer is pairwise).
 *   - `onParagraphHover`: bubbles the hovered paragraph text up to
 *     the page shell, which then forwards it into
 *     `useArtifactTapeLink` to compute the highlight set for the
 *     Tape tab.
 *
 * Stacking for 3+ variants: when the user selects runs across more
 * than two distinct variants, the tab renders one pairwise diff per
 * adjacent variant pair (v1↔v2, v2↔v3, ...). This is the same
 * pattern the spec calls out under "Works correctly with 3 selected
 * runs across 2 variants".
 */

interface ArtifactsTabProps {
  variantIds: string[];
  onParagraphHover?: (paragraph: string | null) => void;
}

export function ArtifactsTab({ variantIds, onParagraphHover }: ArtifactsTabProps) {
  const distinct = useMemo(() => Array.from(new Set(variantIds)), [variantIds]);
  const pairs = useMemo(() => {
    const out: Array<[string, string]> = [];
    for (let i = 0; i < distinct.length - 1; i++) {
      out.push([distinct[i]!, distinct[i + 1]!]);
    }
    return out;
  }, [distinct]);

  if (distinct.length < 2) {
    return (
      <div
        role="status"
        className="flex h-40 items-center justify-center rounded-md border border-dashed border-border bg-bg-elev-1 text-sm text-fg-muted"
      >
        Select runs from at least two variants to diff their artifacts.
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-4">
      {pairs.map(([leftId, rightId]) => (
        <ArtifactPairDiff
          key={`${leftId}-${rightId}`}
          leftVariantId={leftId}
          rightVariantId={rightId}
          onParagraphHover={onParagraphHover}
        />
      ))}
    </div>
  );
}

function ArtifactPairDiff({
  leftVariantId,
  rightVariantId,
  onParagraphHover,
}: {
  leftVariantId: string;
  rightVariantId: string;
  onParagraphHover?: (paragraph: string | null) => void;
}) {
  const left = useArtifacts(leftVariantId);
  const right = useArtifacts(rightVariantId);

  if (left.isLoading || right.isLoading) {
    return <div className="text-sm text-fg-muted">Loading artifacts…</div>;
  }
  if (left.isError || right.isError) {
    return (
      <div role="alert" className="rounded-md border border-danger/30 bg-danger/10 p-3 text-sm text-danger-fg">
        Failed to load artifacts for one or both variants.
      </div>
    );
  }

  const leftData = left.data ?? [];
  const rightData = right.data ?? [];

  // Group artifacts by type so we only diff like-with-like (CLAUDE.md
  // on the left only ever diffs against CLAUDE.md on the right).
  const types = Array.from(
    new Set([
      ...leftData.map((a) => a.artifact_type),
      ...rightData.map((a) => a.artifact_type),
    ]),
  );

  if (types.length === 0) {
    return (
      <div className="rounded-md border border-dashed border-border bg-bg-elev-1 p-3 text-sm text-fg-muted">
        No artifacts on either variant.
      </div>
    );
  }

  return (
    <section className="flex flex-col gap-3">
      <header className="text-xs font-mono text-fg-muted">
        {leftVariantId} ↔ {rightVariantId}
      </header>
      {types.map((type) => {
        const l = leftData.find((a) => a.artifact_type === type);
        const r = rightData.find((a) => a.artifact_type === type);
        if (!l || !r) {
          return (
            <div key={type} className="rounded-md border border-warning/30 bg-warning/10 p-3 text-sm text-warning-fg">
              <strong>{type}</strong>: present on only one variant.
            </div>
          );
        }
        return (
          <ArtifactDiff
            key={type}
            left={l satisfies ArtifactVersion}
            right={r satisfies ArtifactVersion}
            onParagraphHover={onParagraphHover}
          />
        );
      })}
    </section>
  );
}
