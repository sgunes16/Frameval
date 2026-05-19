import type { FileDiff } from './parse-patch';
import type { ParsedTurn } from './types';

/**
 * Given the full parsed patch for a run and the ParsedTurn[] making up
 * one turn group, return only the FileDiff entries the group's blocks
 * touched. The right-hand pane of Inspector V2 uses this to scope the
 * diff view to "what THIS step changed".
 *
 * Deduplicates by path because tool_use and the matching tool_result
 * frequently both stamp the same files_touched field — we only want
 * each file represented once.
 *
 * Files referenced by tools but missing from the patch are silently
 * dropped. That happens when a tool "touches" a file (e.g. Read) that
 * the agent never modified — there's no diff to show.
 */
export function filterDiffForTurn(
  patch: Map<string, FileDiff>,
  blocks: ParsedTurn[],
): FileDiff[] {
  if (patch.size === 0) return [];

  const seen = new Set<string>();
  const out: FileDiff[] = [];
  for (const block of blocks) {
    for (const path of block.files_touched ?? []) {
      if (seen.has(path)) continue;
      const diff = patch.get(path);
      if (!diff) continue;
      seen.add(path);
      out.push(diff);
    }
  }
  return out;
}
