import { useMemo } from 'react';

import { parsePatch, type FileDiff } from './parse-patch';
import { filterDiffForTurn } from './per-turn-diff';
import type { ParsedTurn } from './types';

/**
 * Glue hook: takes the run's raw patch text (from `Transcript.patch`)
 * and one turn group's blocks, returns the FileDiff[] for that step.
 *
 * Why memoise the parse: parsePatch is O(lines) but the transcript
 * patch can be tens of KB. Re-running it on every keystroke-driven
 * re-render of the Inspector right pane would burn cycles for no
 * payoff. The memo invalidates only when the patch text itself
 * changes (i.e. when the run's transcript actually grew).
 */
export function usePerTurnDiff(rawPatch: string | undefined, blocks: ParsedTurn[]): FileDiff[] {
  const patch = useMemo(() => parsePatch(rawPatch ?? ''), [rawPatch]);
  return useMemo(() => filterDiffForTurn(patch, blocks), [patch, blocks]);
}
