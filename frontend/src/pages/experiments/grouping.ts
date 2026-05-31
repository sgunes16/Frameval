import type { Experiment } from '../../lib/types';

export type GroupedExperiment =
  | { kind: 'solo'; experiment: Experiment }
  | { kind: 'group'; batchId: string; batchLabel: string; experiments: Experiment[] };

/**
 * Bucket experiments by their batch_id.
 *
 *   - Two or more experiments sharing a batch_id form a `group`.
 *   - A batch_id with only one experiment in the current view, or an
 *     experiment without a batch_id at all, renders as `solo`.
 *
 * Within a group, members sort newest-first. Across groups + solos,
 * the position of each unit is the most-recent created_at among its
 * members (a group's anchor is its newest child).
 */
export function groupByBatch(experiments: Experiment[]): GroupedExperiment[] {
  const byBatch = new Map<string, Experiment[]>();
  const solos: Experiment[] = [];

  for (const exp of experiments) {
    if (exp.batch_id) {
      const bucket = byBatch.get(exp.batch_id);
      if (bucket) bucket.push(exp);
      else byBatch.set(exp.batch_id, [exp]);
    } else {
      solos.push(exp);
    }
  }

  const units: GroupedExperiment[] = solos.map((experiment) => ({ kind: 'solo' as const, experiment }));

  for (const [batchId, members] of byBatch.entries()) {
    if (members.length < 2) {
      // Singleton "batch" — render flat to avoid visual noise.
      units.push({ kind: 'solo', experiment: members[0] });
      continue;
    }
    // Newest child first within the group.
    members.sort((a, b) => (b.created_at ?? '').localeCompare(a.created_at ?? ''));
    const batchLabel = members[0].batch_label ?? batchId;
    units.push({ kind: 'group', batchId, batchLabel, experiments: members });
  }

  // Anchor each unit by its newest member, then sort descending.
  const anchor = (g: GroupedExperiment): string =>
    g.kind === 'solo'
      ? g.experiment.created_at ?? ''
      : g.experiments[0]?.created_at ?? '';

  units.sort((a, b) => anchor(b).localeCompare(anchor(a)));
  return units;
}
