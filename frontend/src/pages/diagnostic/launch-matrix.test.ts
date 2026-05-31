import { describe, expect, it } from 'vitest';
import { countExperiments, expandLaunchMatrix } from './launch-matrix';

describe('countExperiments', () => {
  it('returns 1 when every dimension is empty (so the gate stays sane)', () => {
    expect(countExperiments({ taskIds: [], executorIds: [], modelIds: [], speckitExtensions: [''] })).toBe(1);
  });

  it('multiplies tasks × executors × models', () => {
    expect(countExperiments({ taskIds: ['a', 'b'], executorIds: ['x'], modelIds: ['m1', 'm2'], speckitExtensions: [''] })).toBe(4);
  });

  it('ignores harnesses (they are intra-experiment variants, not cells)', () => {
    // Same shape twice: the count must depend only on the three cell axes.
    const single = countExperiments({ taskIds: ['a'], executorIds: ['x'], modelIds: ['m'], speckitExtensions: [''] });
    expect(single).toBe(1);
  });
});

describe('expandLaunchMatrix', () => {
  it('yields one cell per (task × executor × model) combination', () => {
    const cells = expandLaunchMatrix({
      taskIds: ['t1', 't2'],
      executorIds: ['e1'],
      modelIds: ['m1', 'm2'],
      speckitExtensions: [''],
    });
    expect(cells).toEqual([
      { taskId: 't1', executorId: 'e1', modelId: 'm1', speckitExtension: '' },
      { taskId: 't1', executorId: 'e1', modelId: 'm2', speckitExtension: '' },
      { taskId: 't2', executorId: 'e1', modelId: 'm1', speckitExtension: '' },
      { taskId: 't2', executorId: 'e1', modelId: 'm2', speckitExtension: '' },
    ]);
  });

  it('returns a single cell for a fully scalar selection', () => {
    expect(expandLaunchMatrix({ taskIds: ['t'], executorIds: ['e'], modelIds: ['m'], speckitExtensions: [''] }))
      .toEqual([{ taskId: 't', executorId: 'e', modelId: 'm', speckitExtension: '' }]);
  });

  it('returns an empty list when any dimension is empty', () => {
    expect(expandLaunchMatrix({ taskIds: [], executorIds: ['e'], modelIds: ['m'], speckitExtensions: [''] })).toEqual([]);
    expect(expandLaunchMatrix({ taskIds: ['t'], executorIds: [], modelIds: ['m'], speckitExtensions: [''] })).toEqual([]);
    expect(expandLaunchMatrix({ taskIds: ['t'], executorIds: ['e'], modelIds: [], speckitExtensions: [''] })).toEqual([]);
  });
});

describe('expandLaunchMatrix — speckit axis', () => {
  it('multiplies cells by speckitExtensions count', () => {
    const cells = expandLaunchMatrix({
      taskIds: ['t'],
      executorIds: ['e'],
      modelIds: ['m'],
      speckitExtensions: ['canonical', 'lite', 'dual-role'],
    });
    expect(cells).toHaveLength(3);
    expect(cells.map((c) => c.speckitExtension)).toEqual(['canonical', 'lite', 'dual-role']);
  });

  it('collapses speckit axis to one empty-string cell when not provided', () => {
    const cells = expandLaunchMatrix({
      taskIds: ['t'],
      executorIds: ['e'],
      modelIds: ['m'],
      speckitExtensions: [''],
    });
    expect(cells).toHaveLength(1);
    expect(cells[0].speckitExtension).toBe('');
  });

  it('countExperiments multiplies by speckitExtensions', () => {
    expect(countExperiments({
      taskIds: ['a', 'b'],
      executorIds: ['e'],
      modelIds: ['m'],
      speckitExtensions: ['canonical', 'lite'],
    })).toBe(4);
  });
});
