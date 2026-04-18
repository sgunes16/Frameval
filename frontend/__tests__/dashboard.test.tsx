import { describe, expect, it } from 'vitest';
import { formatCurrency } from '../src/lib/utils';

describe('formatCurrency', () => {
  it('formats usd values', () => {
    expect(formatCurrency(12.5)).toContain('$');
  });
});
