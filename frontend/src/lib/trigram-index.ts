/**
 * Trigram (3-character n-gram) similarity index for Compare V2's
 * Artifacts ↔ Tape cross-highlighting (story #69).
 *
 * The Artifacts tab needs to find which Tape rows (specifically,
 * their thinking blocks) reference a hovered CLAUDE.md / spec.md
 * paragraph. A full embedding lookup would be overkill — a tiny
 * trigram Jaccard score reliably surfaces "this turn used a phrase
 * from this paragraph" without any model dependency.
 *
 * Why trigrams and not bigrams: bigrams blow up to too many shared
 * matches on noise like "of the", "in the". Trigrams strike the
 * balance the spec prescribes (≥ 3 trigram overlap is the
 * acceptance threshold).
 *
 * Why lowercased + whitespace-collapsed: artifacts have
 * unpredictable casing and indentation; the index must match
 * regardless.
 */

export interface TrigramEntry {
  id: string;
  text: string;
}

export interface TrigramIndex {
  entries: Array<{ id: string; trigrams: Set<string> }>;
}

export interface TrigramMatch {
  id: string;
  score: number;
}

const WHITESPACE = /\s+/g;

/**
 * Normalises the input (lowercase, collapse whitespace) and emits the
 * set of 3-character substrings. Strings shorter than 3 chars produce
 * an empty set — they have nothing to compare on.
 */
export function trigramsOf(text: string): Set<string> {
  const normalised = text.toLowerCase().replace(WHITESPACE, ' ').trim();
  const out = new Set<string>();
  if (normalised.length < 3) return out;
  for (let i = 0; i <= normalised.length - 3; i++) {
    out.add(normalised.slice(i, i + 3));
  }
  return out;
}

export function buildTrigramIndex(entries: TrigramEntry[]): TrigramIndex {
  return {
    entries: entries.map((e) => ({ id: e.id, trigrams: trigramsOf(e.text) })),
  };
}

export function matchByTrigrams(
  index: TrigramIndex,
  query: string,
  threshold: number,
): TrigramMatch[] {
  const q = trigramsOf(query);
  if (q.size === 0) return [];
  const matches: TrigramMatch[] = [];
  for (const entry of index.entries) {
    if (entry.trigrams.size === 0) continue;
    let intersect = 0;
    for (const t of q) if (entry.trigrams.has(t)) intersect += 1;
    const union = q.size + entry.trigrams.size - intersect;
    const score = union === 0 ? 0 : intersect / union;
    if (score >= threshold) matches.push({ id: entry.id, score });
  }
  matches.sort((a, b) => b.score - a.score);
  return matches;
}
