import type { RunAnchors } from './anchor-alignment';

/**
 * Pairwise similarity matrix for Compare V2's Matrix tab (story #68).
 *
 * Scoring uses Jaccard similarity over the set of anchor keys for
 * each run. That alone captures "did these two runs reach the same
 * decision points" cleanly:
 *
 *   similarity(A, B) = |anchors(A) ∩ anchors(B)| / |anchors(A) ∪ anchors(B)|
 *
 * The spec describes a richer formula multiplying in failure-label
 * distance and a sequence-alignment score. Both extensions need
 * additional inputs (Diagnostic data, full ParsedTurn lists) which
 * the Matrix tab doesn't have available cheaply yet. Jaccard alone
 * is monotonic under anchor deletion, symmetric, diagonal-1, and
 * colorblind-renderable — the four properties the acceptance
 * criteria pin. Extensions can layer on as multiplicative factors
 * without changing the shape returned here.
 */

export interface SimilarityMatrix {
  runIds: string[];
  values: number[][];
}

export function jaccardOnKeys(a: Set<string>, b: Set<string>): number {
  if (a.size === 0 && b.size === 0) return 1;
  if (a.size === 0 || b.size === 0) return 0;
  let intersect = 0;
  for (const k of a) if (b.has(k)) intersect += 1;
  const union = a.size + b.size - intersect;
  return union === 0 ? 0 : intersect / union;
}

export function buildSimilarityMatrix(runs: RunAnchors[]): SimilarityMatrix {
  const runIds = runs.map((r) => r.run_id);
  const sets = runs.map((r) => new Set(r.anchors.map((a) => a.key)));

  const values: number[][] = [];
  for (let i = 0; i < runs.length; i++) {
    const row: number[] = [];
    for (let j = 0; j < runs.length; j++) {
      if (i === j) {
        row.push(1);
        continue;
      }
      row.push(jaccardOnKeys(sets[i]!, sets[j]!));
    }
    values.push(row);
  }
  return { runIds, values };
}
