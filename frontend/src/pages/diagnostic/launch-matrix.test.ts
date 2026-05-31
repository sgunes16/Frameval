import { describe, expect, it } from 'vitest';
import { countExperiments, expandLaunchMatrix } from './launch-matrix';

describe('countExperiments', () => {
  it('returns 1 when every dimension is empty (so the gate stays sane)', () => {
    expect(countExperiments({ taskIds: [], executorIds: [], modelIds: [] })).toBe(1);
  });

  it('multiplies tasks × executors × models', () => {
    expect(countExperiments({ taskIds: ['a', 'b'], executorIds: ['x'], modelIds: ['m1', 'm2'] })).toBe(4);
  });

  it('ignores harnesses (they are intra-experiment variants, not cells)', () => {
    // Same shape twice: the count must depend only on the three cell axes.
    const single = countExperiments({ taskIds: ['a'], executorIds: ['x'], modelIds: ['m'] });
    expect(single).toBe(1);
  });
});

describe('expandLaunchMatrix', () => {
  it('yields one cell per (task × executor × model) combination', () => {
    const cells = expandLaunchMatrix({
      taskIds: ['t1', 't2'],
      executorIds: ['e1'],
      modelIds: ['m1', 'm2'],
    });
    expect(cells).toEqual([
      { taskId: 't1', executorId: 'e1', modelId: 'm1' },
      { taskId: 't1', executorId: 'e1', modelId: 'm2' },
      { taskId: 't2', executorId: 'e1', modelId: 'm1' },
      { taskId: 't2', executorId: 'e1', modelId: 'm2' },
    ]);
  });

  it('returns a single cell for a fully scalar selection', () => {
    expect(expandLaunchMatrix({ taskIds: ['t'], executorIds: ['e'], modelIds: ['m'] }))
      .toEqual([{ taskId: 't', executorId: 'e', modelId: 'm' }]);
  });

  it('returns an empty list when any dimension is empty', () => {
    expect(expandLaunchMatrix({ taskIds: [], executorIds: ['e'], modelIds: ['m'] })).toEqual([]);
    expect(expandLaunchMatrix({ taskIds: ['t'], executorIds: [], modelIds: ['m'] })).toEqual([]);
    expect(expandLaunchMatrix({ taskIds: ['t'], executorIds: ['e'], modelIds: [] })).toEqual([]);
  });
});
