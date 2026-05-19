import { describe, expect, it } from 'vitest';

import { buildTrigramIndex, matchByTrigrams, trigramsOf } from './trigram-index';

describe('trigramsOf', () => {
  it('produces character trigrams from a normalized string', () => {
    expect(trigramsOf('hello')).toEqual(new Set(['hel', 'ell', 'llo']));
  });

  it('lowercases input and collapses whitespace before extracting', () => {
    expect(trigramsOf('  Hello  WORLD  ')).toEqual(
      trigramsOf('hello world'),
    );
  });

  it('returns an empty set for strings shorter than 3 chars', () => {
    expect(trigramsOf('ab')).toEqual(new Set());
    expect(trigramsOf('').size).toBe(0);
  });

  it('non-ascii content is included as-is (no transliteration)', () => {
    // Frameval is bilingual TR/EN; the index must not silently drop
    // Turkish characters.
    expect(trigramsOf('çığ')).toContain('çığ');
  });
});

describe('buildTrigramIndex / matchByTrigrams', () => {
  it('matches a paragraph against entries that share enough trigrams', () => {
    const index = buildTrigramIndex([
      { id: 'turn-1', text: 'agent considered the rate limit policy carefully' },
      { id: 'turn-2', text: 'unrelated discussion about user accounts' },
    ]);
    const matches = matchByTrigrams(index, 'rate limit policy', 0.2);
    const ids = matches.map((m) => m.id);
    expect(ids).toContain('turn-1');
    expect(ids).not.toContain('turn-2');
  });

  it('threshold filters out weak matches', () => {
    const index = buildTrigramIndex([
      { id: 'turn-1', text: 'unrelated text that shares one tiny word the' },
    ]);
    // Threshold 0.5 is too high for "the" to match anything.
    expect(matchByTrigrams(index, 'the', 0.5)).toEqual([]);
  });

  it('returns matches sorted descending by Jaccard score', () => {
    const index = buildTrigramIndex([
      { id: 'low', text: 'rate limiter installed today' },
      { id: 'high', text: 'rate limit policy enforced strictly' },
    ]);
    const matches = matchByTrigrams(index, 'rate limit policy', 0.05);
    expect(matches[0]?.id).toBe('high');
    // Scores are descending.
    for (let i = 1; i < matches.length; i++) {
      expect(matches[i - 1]!.score).toBeGreaterThanOrEqual(matches[i]!.score);
    }
  });

  it('empty index produces no matches', () => {
    expect(matchByTrigrams(buildTrigramIndex([]), 'anything', 0)).toEqual([]);
  });

  it('paragraph shorter than 3 chars produces no matches (no trigrams to query)', () => {
    const index = buildTrigramIndex([{ id: 't1', text: 'whatever long thing' }]);
    expect(matchByTrigrams(index, 'ab', 0.1)).toEqual([]);
  });
});
