import { describe, expect, it } from 'vitest';
import { validateMultiAgentConfig } from './multiagent-validate';

describe('validateMultiAgentConfig', () => {
  it('rejects undefined or empty configs', () => {
    expect(validateMultiAgentConfig(undefined)).toBe(false);
    expect(validateMultiAgentConfig({ roles: [] })).toBe(false);
  });

  it('rejects more than 5 roles', () => {
    const six = Array.from({ length: 6 }, (_, i) => ({ name: `r${i}`, prompt: 'x' }));
    expect(validateMultiAgentConfig({ roles: six })).toBe(false);
  });

  it('rejects invalid role names', () => {
    expect(validateMultiAgentConfig({ roles: [{ name: 'Planner', prompt: 'x' }] })).toBe(false); // uppercase
    expect(validateMultiAgentConfig({ roles: [{ name: '1bad', prompt: 'x' }] })).toBe(false); // leading digit
    expect(validateMultiAgentConfig({ roles: [{ name: '', prompt: 'x' }] })).toBe(false);
    expect(validateMultiAgentConfig({ roles: [{ name: 'has space', prompt: 'x' }] })).toBe(false);
  });

  it('rejects empty / whitespace-only prompts', () => {
    expect(validateMultiAgentConfig({ roles: [{ name: 'a', prompt: '' }] })).toBe(false);
    expect(validateMultiAgentConfig({ roles: [{ name: 'a', prompt: '   \n\t  ' }] })).toBe(false);
  });

  it('rejects duplicate role names', () => {
    expect(validateMultiAgentConfig({ roles: [
      { name: 'a', prompt: 'x' },
      { name: 'a', prompt: 'y' },
    ] })).toBe(false);
  });

  it('accepts a clean 1-5 role config', () => {
    expect(validateMultiAgentConfig({ roles: [{ name: 'planner', prompt: 'x' }] })).toBe(true);
    expect(validateMultiAgentConfig({ roles: [
      { name: 'planner', prompt: 'plan {{TASK}}' },
      { name: 'coder', prompt: 'code from {{PREV_OUTPUT}}' },
    ] })).toBe(true);
  });
});
