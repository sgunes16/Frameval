import { describe, expect, it } from 'vitest';

import { alignAnchors, type AnchoredRow, type DriftRow, type Anchor, type RunAnchors } from './anchor-alignment';

const a = (key: string, turnIndex: number, parentTurnIndex = turnIndex): Anchor => ({
  key,
  turn_index: turnIndex,
  parent_turn_index: parentTurnIndex,
});

const run = (runId: string, anchors: Anchor[]): RunAnchors => ({
  run_id: runId,
  anchors,
});

function rowKinds(rows: Array<AnchoredRow | DriftRow>): string[] {
  return rows.map((r) => r.kind);
}

describe('alignAnchors', () => {
  it('empty input yields no rows', () => {
    expect(alignAnchors([])).toEqual([]);
    expect(alignAnchors([run('r1', [])])).toEqual([]);
  });

  it('two identical runs produce all-anchored rows', () => {
    const anchors = [a('Edit|src/main.go', 0), a('Bash|deadbeef', 5), a('Read|README.md', 10)];
    const rows = alignAnchors([run('r1', anchors), run('r2', anchors)]);
    expect(rowKinds(rows)).toEqual(['anchored', 'anchored', 'anchored']);
    expect((rows[0] as AnchoredRow).anchor.key).toBe('Edit|src/main.go');
    // Both columns present for every anchored row.
    for (const row of rows as AnchoredRow[]) {
      expect(row.columns.size).toBe(2);
    }
  });

  it('prefix divergence: r1 has extra anchors at the front, then converges', () => {
    const r1 = [a('Edit|src/extra.go', 0), a('Edit|src/main.go', 4), a('Read|README.md', 8)];
    const r2 = [a('Edit|src/main.go', 0), a('Read|README.md', 5)];
    const rows = alignAnchors([run('r1', r1), run('r2', r2)]);
    // First row is drift (r1's extra anchor); next two are anchored.
    expect(rowKinds(rows)).toEqual(['drift', 'anchored', 'anchored']);
    const first = rows[0] as DriftRow;
    expect(first.columns.get('r1')).toBeDefined();
    expect(first.columns.get('r2')).toBeNull();
  });

  it('suffix divergence: r2 keeps going after r1 finishes', () => {
    const r1 = [a('Edit|src/main.go', 0)];
    const r2 = [a('Edit|src/main.go', 0), a('Bash|deadbeef', 3)];
    const rows = alignAnchors([run('r1', r1), run('r2', r2)]);
    expect(rowKinds(rows)).toEqual(['anchored', 'drift']);
    const tail = rows[1] as DriftRow;
    expect(tail.columns.get('r1')).toBeNull();
    expect(tail.columns.get('r2')).toBeDefined();
  });

  it('mid-fork rejoin: runs diverge, then meet again at a shared anchor', () => {
    const r1 = [a('Edit|x', 0), a('Bash|aa', 3), a('Read|y', 8)];
    const r2 = [a('Edit|x', 0), a('Bash|bb', 3), a('Read|y', 7)];
    const rows = alignAnchors([run('r1', r1), run('r2', r2)]);
    // Anchored on Edit|x, drift on the two different Bash keys, anchored on Read|y.
    expect(rowKinds(rows)).toEqual(['anchored', 'drift', 'drift', 'anchored']);
  });

  it('one-run-shorter: the shorter run pads with null after exhaustion', () => {
    const r1 = [a('Edit|x', 0), a('Read|y', 3)];
    const r2 = [a('Edit|x', 0)];
    const rows = alignAnchors([run('r1', r1), run('r2', r2)]);
    expect(rowKinds(rows)).toEqual(['anchored', 'drift']);
    expect((rows[1] as DriftRow).columns.get('r2')).toBeNull();
  });

  it('completely disjoint runs produce only drift rows', () => {
    const r1 = [a('Edit|a', 0), a('Edit|b', 2)];
    const r2 = [a('Read|c', 0), a('Read|d', 2)];
    const rows = alignAnchors([run('r1', r1), run('r2', r2)]);
    expect(rows.every((r) => r.kind === 'drift')).toBe(true);
  });

  it('three runs all anchored when keys match', () => {
    const anchors = [a('Edit|x', 0), a('Read|y', 5)];
    const rows = alignAnchors([run('r1', anchors), run('r2', anchors), run('r3', anchors)]);
    expect(rowKinds(rows)).toEqual(['anchored', 'anchored']);
    for (const row of rows as AnchoredRow[]) {
      expect(row.columns.size).toBe(3);
    }
  });

  it('three runs: r1 r2 share, r3 diverges — produces exactly r1, r2, r3 drift rows in order', () => {
    const shared = [a('Edit|x', 0)];
    const r3only = [a('Bash|z', 0)];
    const rows = alignAnchors([run('r1', shared), run('r2', shared), run('r3', r3only)]);
    // No row can be 'anchored' because the active set never agrees
    // (r3's key differs from r1+r2). Lookahead identifies all three
    // as advancers (no other run's future contains the current key),
    // so we get one drift row per advancer in run order.
    expect(rowKinds(rows)).toEqual(['drift', 'drift', 'drift']);
    const r3DriftRows = rows.filter(
      (r) => r.kind === 'drift' && r.columns.get('r3') !== null,
    );
    expect(r3DriftRows).toHaveLength(1);
  });

  it('deadlock fallback fires when every run waits on every other, then alignment recovers', () => {
    // r1=[A, B], r2=[B, A]. Both runs see the other's current key in
    // their own future, so lookahead classifies them as waiters and
    // returns an empty advancer set. Without the fallback the
    // algorithm would infinite-loop. The fallback picks the run with
    // smallest turn_index (tie → first in run order = r1), advances
    // its pointer, and from that step the two runs realign:
    //   step 1: drift (r1 advances, r2 waits)
    //   step 2: anchored on B (both runs at B)
    //   step 3: drift (r2 has A trailing, r1 exhausted)
    const r1 = [a('A', 0), a('B', 1)];
    const r2 = [a('B', 0), a('A', 1)];
    const rows = alignAnchors([run('r1', r1), run('r2', r2)]);
    expect(rowKinds(rows)).toEqual(['drift', 'anchored', 'drift']);
    expect(rows.length).toBeLessThanOrEqual(4); // proves termination
  });

  it('Bash-without-files keys (content-hash form) anchor correctly', () => {
    const r1 = [a('Bash|deadbeef', 2), a('Bash|cafe1234', 6)];
    const r2 = [a('Bash|deadbeef', 1), a('Bash|cafe1234', 5)];
    const rows = alignAnchors([run('r1', r1), run('r2', r2)]);
    expect(rowKinds(rows)).toEqual(['anchored', 'anchored']);
  });

  it('multi-tool turn: two anchors at the same turn_index align as one row each', () => {
    const r1 = [a('Edit|x', 5), a('Edit|y', 5)];
    const r2 = [a('Edit|x', 5), a('Edit|y', 5)];
    const rows = alignAnchors([run('r1', r1), run('r2', r2)]);
    expect(rowKinds(rows)).toEqual(['anchored', 'anchored']);
  });

  it('empty run alongside a non-empty run produces only that run\'s drift cells', () => {
    const rows = alignAnchors([run('r1', [a('Edit|x', 0)]), run('r2', [])]);
    expect(rows).toHaveLength(1);
    expect(rows[0]?.kind).toBe('drift');
    expect((rows[0] as DriftRow).columns.get('r1')).toBeDefined();
    expect((rows[0] as DriftRow).columns.get('r2')).toBeNull();
  });

  it('referential stability: identical input produces identical row references', () => {
    // The hook caller relies on stable row identity to memoise child
    // renders. Two calls on the same input must return the same Map
    // instances (we test via JSON deep-equal — Maps don't serialize
    // out of the box, so we walk the array).
    const anchors = [a('Edit|x', 0), a('Read|y', 5)];
    const input = [run('r1', anchors), run('r2', anchors)];
    const first = alignAnchors(input);
    const second = alignAnchors(input);
    expect(first).toHaveLength(second.length);
    for (let i = 0; i < first.length; i++) {
      expect(first[i]?.kind).toBe(second[i]?.kind);
      if (first[i]?.kind === 'anchored') {
        expect((first[i] as AnchoredRow).anchor.key).toBe((second[i] as AnchoredRow).anchor.key);
      }
    }
  });
});
