import { describe, expect, it } from 'vitest';
import { buildLaunchVariants } from './build-variants';

describe('buildLaunchVariants', () => {
  it('emits one variant per non-speckit harness with its own config block', () => {
    const out = buildLaunchVariants(
      ['bare', 'agent_instructions', 'multiagent', 'ralph'],
      {
        agent_instructions: { content: 'rules' },
        multiagent: { roles: [{ name: 'planner', prompt: 'p' }] },
      },
      [],
    );
    expect(out.map((v) => v.name)).toEqual(['bare', 'agent_instructions', 'multiagent', 'ralph']);
    expect(out.every((v) => v.harness_id === v.name)).toBe(true);
    // bare / ralph carry no config; the configured ones carry only their own block.
    expect(out[0].harness_config).toBeUndefined();
    expect(out[1].harness_config).toEqual({ agent_instructions: { content: 'rules' } });
    expect(out[2].harness_config).toEqual({ multiagent: { roles: [{ name: 'planner', prompt: 'p' }] } });
    expect(out[3].harness_config).toBeUndefined();
  });

  it('emits one speckit variant per extension, named speckit/<ext>', () => {
    const out = buildLaunchVariants(['speckit'], {}, ['canonical', 'lite', 'dual-role']);
    expect(out).toEqual([
      { harness_id: 'speckit', name: 'speckit/canonical', harness_config: { speckit: { extension_id: 'canonical' } } },
      { harness_id: 'speckit', name: 'speckit/lite', harness_config: { speckit: { extension_id: 'lite' } } },
      { harness_id: 'speckit', name: 'speckit/dual-role', harness_config: { speckit: { extension_id: 'dual-role' } } },
    ]);
  });

  it('mixes non-speckit harnesses and speckit extensions in selection order', () => {
    const out = buildLaunchVariants(
      ['bare', 'speckit', 'ralph'],
      {},
      ['canonical', 'lite'],
    );
    expect(out.map((v) => v.name)).toEqual([
      'bare', 'speckit/canonical', 'speckit/lite', 'ralph',
    ]);
  });

  it('emits no speckit variants when speckit is selected but no extensions chosen', () => {
    const out = buildLaunchVariants(['bare', 'speckit'], {}, []);
    expect(out.map((v) => v.name)).toEqual(['bare']);
  });
});
