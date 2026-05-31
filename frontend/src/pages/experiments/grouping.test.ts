import { describe, expect, it } from 'vitest';
import { groupByBatch, type GroupedExperiment } from './grouping';
import type { Experiment } from '../../lib/types';

function exp(id: string, batchId: string | undefined, createdAt: string, status = 'completed'): Experiment {
  return {
    id,
    name: id,
    status,
    task_id: 't',
    model: 'm',
    agent_cli: 'a',
    execution_mode: 'cli',
    runs_per_variant: 1,
    temperature: 0,
    timeout_seconds: 600,
    max_concurrent: 1,
    created_at: createdAt,
    batch_id: batchId,
  };
}

describe('groupByBatch', () => {
  it('groups two experiments that share a batch_id', () => {
    const items = [exp('a', 'b1', '2026-05-31T10:00:00Z'), exp('b', 'b1', '2026-05-31T10:01:00Z')];
    const out = groupByBatch(items);
    expect(out).toHaveLength(1);
    expect(out[0].kind).toBe('group');
    if (out[0].kind === 'group') {
      expect(out[0].experiments.map((e) => e.id).sort()).toEqual(['a', 'b']);
    }
  });

  it('renders a singleton batch as solo (avoid 1-item groups)', () => {
    const items = [exp('a', 'b1', '2026-05-31T10:00:00Z')];
    const out = groupByBatch(items);
    expect(out).toHaveLength(1);
    expect(out[0].kind).toBe('solo');
  });

  it('renders experiments without a batch_id as solo', () => {
    const items = [exp('a', undefined, '2026-05-31T10:00:00Z')];
    const out = groupByBatch(items);
    expect(out).toHaveLength(1);
    expect(out[0].kind).toBe('solo');
  });

  it('mixed input: 3 batched + 1 solo-batched + 1 unbatched → 1 group + 2 solos, sorted by recency', () => {
    const items = [
      exp('older', 'A', '2026-05-31T09:00:00Z'),
      exp('newer', 'A', '2026-05-31T11:00:00Z'),
      exp('mid', 'A', '2026-05-31T10:00:00Z'),
      exp('lonelyBatch', 'B', '2026-05-31T12:00:00Z'),
      exp('noBatch', undefined, '2026-05-31T13:00:00Z'),
    ];
    const out = groupByBatch(items);
    // ordering: noBatch (13:00) → lonelyBatch (12:00) → group A (max=11:00)
    expect(out.map((g: GroupedExperiment) => (g.kind === 'group' ? `group:${g.batchId}` : `solo:${g.experiment.id}`))).toEqual([
      'solo:noBatch',
      'solo:lonelyBatch',
      'group:A',
    ]);
    const groupA = out.find((g) => g.kind === 'group' && g.batchId === 'A');
    expect(groupA).toBeDefined();
    if (groupA && groupA.kind === 'group') {
      // children sorted newest-first within the group
      expect(groupA.experiments.map((e) => e.id)).toEqual(['newer', 'mid', 'older']);
    }
  });
});
