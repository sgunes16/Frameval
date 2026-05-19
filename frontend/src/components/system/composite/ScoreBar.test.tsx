import { describe, expect, it } from 'vitest';
import { render, screen } from '@testing-library/react';

import { ScoreBar } from './ScoreBar';

describe('ScoreBar', () => {
  it('renders 10 segments by default', () => {
    render(<ScoreBar value={0.5} label="planning depth" />);
    expect(screen.getAllByTestId('score-segment')).toHaveLength(10);
  });

  it('fills the proportion of segments corresponding to value', () => {
    render(<ScoreBar value={0.3} label="metric" />);
    const segments = screen.getAllByTestId('score-segment');
    const filled = segments.filter((s) => s.dataset.filled === 'true');
    // 0.3 of 10 segments = 3 filled.
    expect(filled).toHaveLength(3);
  });

  it('clamps values above 1 to fully filled and below 0 to empty', () => {
    const { rerender } = render(<ScoreBar value={1.5} label="x" />);
    let filled = screen.getAllByTestId('score-segment').filter((s) => s.dataset.filled === 'true');
    expect(filled).toHaveLength(10);

    rerender(<ScoreBar value={-0.5} label="x" />);
    filled = screen.getAllByTestId('score-segment').filter((s) => s.dataset.filled === 'true');
    expect(filled).toHaveLength(0);
  });

  it('exposes the value via aria-valuenow for screen readers', () => {
    render(<ScoreBar value={0.73} label="judge correctness" />);
    const meter = screen.getByRole('meter', { name: /judge correctness/i });
    expect(meter.getAttribute('aria-valuenow')).toBe('0.73');
  });

  it('reports the raw (unclamped) value via aria-valuenow so out-of-range bugs surface', () => {
    // The visual fill clamps but the meter contract MUST surface the
    // upstream value as-is — otherwise a calculator that produces 1.8
    // looks identical in audits to one that produces 1.0.
    const { rerender } = render(<ScoreBar value={1.8} label="x" />);
    expect(screen.getByRole('meter').getAttribute('aria-valuenow')).toBe('1.8');

    rerender(<ScoreBar value={-0.2} label="x" />);
    expect(screen.getByRole('meter').getAttribute('aria-valuenow')).toBe('-0.2');
  });
});
