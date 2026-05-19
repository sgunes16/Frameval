import { describe, expect, it } from 'vitest';
import { render, screen } from '@testing-library/react';

import { LoadingSkeleton } from './LoadingSkeleton';

describe('LoadingSkeleton', () => {
  it('renders the requested number of rows', () => {
    render(<LoadingSkeleton variant="row" count={4} />);
    expect(screen.getAllByTestId('skeleton-row')).toHaveLength(4);
  });

  it('block variant exposes its own testid', () => {
    render(<LoadingSkeleton variant="block" />);
    expect(screen.getByTestId('skeleton-block')).toBeInTheDocument();
  });

  it('uses animate-pulse for the loading affordance', () => {
    render(<LoadingSkeleton variant="card" />);
    expect(screen.getByTestId('skeleton-card').className).toMatch(/animate-pulse/);
  });
});
