import { useMemo } from 'react';

import { parsePatch, type FileDiff } from './parse-patch';
import { filterDiffForTurn } from './per-turn-diff';
import type { ParsedTurn } from './types';

const EMPTY_BLOCKS: ParsedTurn[] = [];

/**
 * Glue hook: takes the run's raw patch text (from `Transcript.patch`)
 * and one turn group's blocks, returns the FileDiff[] for that step.
 *
 * Why memoise the parse: parsePatch is O(lines) but the transcript
 * patch can be tens of KB. Re-running it on every keystroke-driven
 * re-render of the Inspector right pane would burn cycles for no
 * payoff. The memo invalidates only when the patch text itself
 * changes (i.e. when the run's transcript actually grew).
 *
 * The `blocks ?? EMPTY_BLOCKS` sentinel keeps the second memo's
 * dependency identity stable when the caller passes a fresh `[]`
 * literal on every render (which happens when no turn is focused).
 */
export function usePerTurnDiff(
  rawPatch: string | undefined,
  blocks: ParsedTurn[] | undefined,
): FileDiff[] {
  const patch = useMemo(() => parsePatch(rawPatch ?? ''), [rawPatch]);
  const safeBlocks = blocks ?? EMPTY_BLOCKS;
  return useMemo(() => filterDiffForTurn(patch, safeBlocks), [patch, safeBlocks]);
}
