import { describe, expect, it, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';

import { ErrorState } from './ErrorState';

describe('ErrorState', () => {
  it('shows the title and description', () => {
    render(<ErrorState title="Something broke" description="Try again in a moment." />);
    expect(screen.getByText(/something broke/i)).toBeInTheDocument();
    expect(screen.getByText(/try again in a moment/i)).toBeInTheDocument();
  });

  it('fires onRetry when the retry button is clicked', async () => {
    const onRetry = vi.fn();
    render(<ErrorState title="Failure" onRetry={onRetry} />);
    await userEvent.click(screen.getByRole('button', { name: /try again/i }));
    expect(onRetry).toHaveBeenCalledOnce();
  });

  it('omits the retry button when onRetry is not provided', () => {
    render(<ErrorState title="Failure" />);
    expect(screen.queryByRole('button', { name: /try again/i })).not.toBeInTheDocument();
  });
});
