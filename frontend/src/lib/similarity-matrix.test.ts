import { describe, expect, it } from 'vitest';

import { buildSimilarityMatrix, jaccardOnKeys } from './similarity-matrix';
import type { RunAnchors } from './anchor-alignment';

const r = (id: string, keys: string[]): RunAnchors => ({
  run_id: id,
  anchors: keys.map((k, i) => ({ key: k, turn_index: i, parent_turn_index: i })),
});

describe('jaccardOnKeys', () => {
  it('identical key sets score 1.0', () => {
    const a = new Set(['Edit|a', 'Read|b']);
    const b = new Set(['Edit|a', 'Read|b']);
    expect(jaccardOnKeys(a, b)).toBe(1);
  });

  it('disjoint sets score 0.0', () => {
    expect(jaccardOnKeys(new Set(['x']), new Set(['y']))).toBe(0);
  });

  it('half-overlap scores 1/3 (intersection 1, union 3)', () => {
    expect(jaccardOnKeys(new Set(['a', 'b']), new Set(['a', 'c']))).toBeCloseTo(1 / 3, 6);
  });

  it('two empty sets score 1.0 (vacuously identical)', () => {
    expect(jaccardOnKeys(new Set(), new Set())).toBe(1);
  });

  it('one empty, one non-empty scores 0.0', () => {
    expect(jaccardOnKeys(new Set(), new Set(['a']))).toBe(0);
  });
});

describe('buildSimilarityMatrix', () => {
  it('returns an N×N symmetric matrix with 1.0 on the diagonal', () => {
    const runs = [
      r('r1', ['Edit|a', 'Read|b']),
      r('r2', ['Edit|a']),
      r('r3', ['Bash|x']),
    ];
    const m = buildSimilarityMatrix(runs);
    expect(m.runIds).toEqual(['r1', 'r2', 'r3']);
    expect(m.values).toHaveLength(3);
    expect(m.values[0]).toHaveLength(3);
    // Diagonal == 1
    for (let i = 0; i < 3; i++) expect(m.values[i]![i]).toBe(1);
    // Symmetric
    for (let i = 0; i < 3; i++) {
      for (let j = i + 1; j < 3; j++) {
        expect(m.values[i]![j]).toBe(m.values[j]![i]);
      }
    }
  });

  it('disjoint runs produce 0 off-diagonal', () => {
    const runs = [r('r1', ['Edit|a']), r('r2', ['Bash|z'])];
    const m = buildSimilarityMatrix(runs);
    expect(m.values[0]![1]).toBe(0);
    expect(m.values[1]![0]).toBe(0);
  });

  it('partially overlapping runs produce Jaccard scores', () => {
    const runs = [
      r('r1', ['a', 'b']),
      r('r2', ['a', 'c']),
    ];
    const m = buildSimilarityMatrix(runs);
    expect(m.values[0]![1]).toBeCloseTo(1 / 3, 6);
  });

  it('monotonic under turn deletion: removing a shared anchor from one run lowers its similarity to the other', () => {
    // Spec acceptance criterion: similarity must be monotonic under
    // anchor deletion — removing a shared anchor never raises the
    // pair's score.
    const before = buildSimilarityMatrix([
      r('r1', ['a', 'b', 'c']),
      r('r2', ['a', 'b']),
    ]);
    const after = buildSimilarityMatrix([
      r('r1', ['a', 'b', 'c']),
      r('r2', ['a']), // dropped 'b'
    ]);
    expect(after.values[0]![1]).toBeLessThanOrEqual(before.values[0]![1]!);
  });

  it('empty input produces an empty matrix', () => {
    const m = buildSimilarityMatrix([]);
    expect(m.runIds).toEqual([]);
    expect(m.values).toEqual([]);
  });

  it('single run produces a 1x1 matrix with 1.0', () => {
    const m = buildSimilarityMatrix([r('r1', ['a'])]);
    expect(m.values).toEqual([[1]]);
  });
});
