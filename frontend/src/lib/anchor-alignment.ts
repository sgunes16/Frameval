/**
 * Greedy anchor alignment for Compare V2's Tape tab (story #67).
 *
 * Given the per-run anchor lists produced by the engine's
 * `BuildAnchors` helper (story #66), produce a row-by-row alignment:
 *   - Each row is either `anchored` (every run hits the same anchor
 *     at the current pointer position) or `drift` (at least one run
 *     is on a different anchor key).
 *   - Drift rows are populated for the run(s) whose current anchor
 *     has the smallest `turn_index`, advancing those pointers; the
 *     other runs' cells stay null on this row and are picked up on
 *     a future row.
 *
 * The algorithm is O(total_anchors) — each anchor consumes exactly
 * one row of work. We accept a less precise alignment than full LCS
 * because the Tape UI is exploratory: a few extra drift rows in a
 * mid-fork-rejoin scenario are visually fine and far cheaper than
 * the quadratic LCS at 5 runs × 200 anchors.
 *
 * Why not a memoised hook helper here: this file owns the data
 * shape only. The React hook in use-anchor-alignment.ts wraps this
 * with `useMemo` so callers stay declarative.
 */

export interface Anchor {
  key: string;
  turn_index: number;
  parent_turn_index: number;
}

export interface RunAnchors {
  run_id: string;
  anchors: Anchor[];
}

/**
 * AnchoredRow: every run's current pointer agreed on the same key.
 * `columns` maps run_id → that anchor's parent_turn_index, which
 * the Tape UI uses to scroll the Inspector turn group into view
 * when the row is focused.
 */
export interface AnchoredRow {
  kind: 'anchored';
  anchor: Anchor;
  columns: Map<string, number>;
}

/**
 * DriftRow: at least one run is on a different key than the others.
 * `columns` is run_id → parent_turn_index of the cell this row
 * represents, or `null` for runs that don't contribute to this row.
 * The right-hand fork drawer pulls the contributed turns by index.
 */
export interface DriftRow {
  kind: 'drift';
  columns: Map<string, number | null>;
}

export type AlignmentRow = AnchoredRow | DriftRow;

export function alignAnchors(runs: RunAnchors[]): AlignmentRow[] {
  if (runs.length === 0) return [];

  const pointers = new Map<string, number>();
  for (const r of runs) pointers.set(r.run_id, 0);

  const rows: AlignmentRow[] = [];

  const current = (r: RunAnchors): Anchor | undefined => {
    const p = pointers.get(r.run_id) ?? 0;
    return r.anchors[p];
  };

  /** Does `key` appear at-or-after `run`'s current pointer? */
  const futureContains = (run: RunAnchors, key: string): boolean => {
    const p = pointers.get(run.run_id) ?? 0;
    for (let i = p; i < run.anchors.length; i++) {
      if (run.anchors[i]!.key === key) return true;
    }
    return false;
  };

  while (true) {
    const heads = runs.map((r) => ({ run: r, anchor: current(r) }));
    const active = heads.filter((h) => h.anchor !== undefined);
    if (active.length === 0) break;

    const firstKey = active[0]!.anchor!.key;
    const allShare =
      active.length === runs.length &&
      active.every((h) => h.anchor!.key === firstKey);

    if (allShare) {
      const cols = new Map<string, number>();
      for (const h of active) {
        cols.set(h.run.run_id, h.anchor!.parent_turn_index);
        pointers.set(h.run.run_id, (pointers.get(h.run.run_id) ?? 0) + 1);
      }
      rows.push({ kind: 'anchored', anchor: active[0]!.anchor!, columns: cols });
      continue;
    }

    // Drift step. The lookahead rule decides which runs advance:
    // a run is an "advancer" iff its current key does NOT appear
    // later in any OTHER run's remaining anchors. Otherwise it's a
    // "waiter" — we hold its pointer so the others catch up to the
    // shared key.
    //
    // Each advancer gets its own drift row so the Tape UI doesn't
    // visually align two runs' different decisions in the same row.
    // If lookahead produces no advancers (true deadlock — every run
    // is waiting on every other), fall back to advancing the run
    // with the smallest current turn_index, breaking the cycle.
    const advancers = active.filter((h) => {
      for (const other of active) {
        if (other.run.run_id === h.run.run_id) continue;
        if (futureContains(other.run, h.anchor!.key)) return false;
      }
      return true;
    });

    let toEmit = advancers;
    if (toEmit.length === 0) {
      let min = Infinity;
      let pick: { run: RunAnchors; anchor: Anchor } | null = null;
      for (const h of active) {
        if (h.anchor!.turn_index < min) {
          min = h.anchor!.turn_index;
          pick = { run: h.run, anchor: h.anchor! };
        }
      }
      if (pick) toEmit = [pick];
    }

    for (const adv of toEmit) {
      const cols = new Map<string, number | null>();
      for (const r of runs) cols.set(r.run_id, null);
      cols.set(adv.run.run_id, adv.anchor!.parent_turn_index);
      rows.push({ kind: 'drift', columns: cols });
      pointers.set(adv.run.run_id, (pointers.get(adv.run.run_id) ?? 0) + 1);
    }
  }

  return rows;
}
