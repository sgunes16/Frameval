import { describe, expect, it } from 'vitest';
import { render, screen } from '@testing-library/react';

import { DiffBadge } from './DiffBadge';

describe('DiffBadge', () => {
  it('renders +N / -M pill with token-driven colors', () => {
    render(<DiffBadge added={12} removed={3} />);
    expect(screen.getByText('+12')).toBeInTheDocument();
    expect(screen.getByText('−3')).toBeInTheDocument();
  });

  it('omits the +N span when there are no additions', () => {
    render(<DiffBadge added={0} removed={5} />);
    expect(screen.queryByText(/^\+/)).not.toBeInTheDocument();
    expect(screen.getByText('−5')).toBeInTheDocument();
  });

  it('omits the −M span when there are no deletions', () => {
    render(<DiffBadge added={5} removed={0} />);
    expect(screen.getByText('+5')).toBeInTheDocument();
    expect(screen.queryByText(/^−/)).not.toBeInTheDocument();
  });

  it('renders a placeholder when both counts are zero', () => {
    render(<DiffBadge added={0} removed={0} />);
    expect(screen.getByText(/no changes/i)).toBeInTheDocument();
  });

  it('uses tabular-nums so digit columns align across rows', () => {
    render(<DiffBadge added={1} removed={1} />);
    const badge = screen.getByText('+1').parentElement;
    expect(badge?.className).toMatch(/tabular-nums/);
  });
});
