import { useMemo } from 'react';

import { buildTrigramIndex, matchByTrigrams } from './trigram-index';
import type { ParsedTurn } from './types';

/**
 * useArtifactTapeLink — cross-highlighting between an artifact
 * paragraph and the thinking blocks in a run's Tape.
 *
 * Given the currently-hovered paragraph in the Artifacts tab and
 * the run's ParsedTurns, return the set of parent_turn_index values
 * whose thinking content shares enough trigrams with the paragraph
 * to count as a reference. The Tape tab consumes this set and
 * applies a border-left style to those rows.
 *
 * The threshold defaults to 0.3 (the spec value). Lower values let
 * weak topical similarity match; higher values demand near-verbatim
 * quotes.
 *
 * Memoisation: the trigram index is rebuilt only when `turns`
 * identity changes (typical TanStack-Query caching makes that
 * rare), and the match query reruns when the paragraph changes.
 */

interface UseArtifactTapeLinkArgs {
  paragraph: string | null;
  turns: ParsedTurn[];
  threshold?: number;
}

interface UseArtifactTapeLinkResult {
  highlights: Set<number>;
}

export function useArtifactTapeLink({
  paragraph,
  turns,
  threshold = 0.3,
}: UseArtifactTapeLinkArgs): UseArtifactTapeLinkResult {
  const index = useMemo(() => {
    const thinkingTurns = turns.filter((t) => t.block_kind === 'thinking');
    return buildTrigramIndex(
      thinkingTurns.map((t) => ({
        id: String(t.parent_turn_index ?? t.turn_index ?? -1),
        text: t.content,
      })),
    );
  }, [turns]);

  const highlights = useMemo(() => {
    if (!paragraph) return new Set<number>();
    const matches = matchByTrigrams(index, paragraph, threshold);
    return new Set(matches.map((m) => Number(m.id)));
  }, [index, paragraph, threshold]);

  return { highlights };
}
