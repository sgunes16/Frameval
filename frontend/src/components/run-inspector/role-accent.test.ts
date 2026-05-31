import { describe, expect, it } from 'vitest';
import { roleAccent } from './role-accent';

describe('roleAccent', () => {
  it('returns a stable fallback for undefined / empty roles', () => {
    expect(roleAccent(undefined)).toBe('border-l-border');
    expect(roleAccent('')).toBe('border-l-border');
  });

  it('returns the same class for the same role name across calls', () => {
    expect(roleAccent('planner')).toBe(roleAccent('planner'));
    expect(roleAccent('coder')).toBe(roleAccent('coder'));
  });

  it('produces at least 3 distinct classes across 5 distinct names (palette coverage)', () => {
    const samples = ['planner', 'coder', 'reviewer', 'critic', 'tester'];
    const distinct = new Set(samples.map(roleAccent));
    expect(distinct.size).toBeGreaterThanOrEqual(3);
  });

  it('every returned class is one of the published palette entries', () => {
    const allowed = new Set([
      'border-l-border',
      'border-l-info-fg/50',
      'border-l-success-fg/50',
      'border-l-warning-fg/50',
      'border-l-fg-subtle/50',
      'border-l-fg-muted/50',
    ]);
    for (const name of ['x', 'y', 'z', 'planner', 'coder', 'reviewer']) {
      expect(allowed.has(roleAccent(name))).toBe(true);
    }
  });
});
