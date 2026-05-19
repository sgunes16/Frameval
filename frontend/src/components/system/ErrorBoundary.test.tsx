import { afterAll, describe, expect, it, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';

import { ErrorBoundary } from './ErrorBoundary';

function Boom({ fail }: { fail: boolean }) {
  if (fail) throw new Error('boom');
  return <div>ok</div>;
}

describe('ErrorBoundary', () => {
  // React logs caught errors via console.error; silence them in tests
  // so the suite output stays clean.
  const consoleErrorSpy = vi.spyOn(console, 'error').mockImplementation(() => {});
  afterAll(() => consoleErrorSpy.mockRestore());

  it('renders children when no error is thrown', () => {
    render(
      <ErrorBoundary>
        <Boom fail={false} />
      </ErrorBoundary>,
    );
    expect(screen.getByText('ok')).toBeInTheDocument();
  });

  it('renders the ErrorState fallback when a child throws', () => {
    render(
      <ErrorBoundary>
        <Boom fail={true} />
      </ErrorBoundary>,
    );
    expect(screen.getByRole('alert')).toBeInTheDocument();
    expect(screen.getByText(/Something went wrong/i)).toBeInTheDocument();
    expect(screen.getByText(/boom/i)).toBeInTheDocument();
  });

  it('exposes a Try again button that clears the caught error', async () => {
    const onReset = vi.fn();
    const user = userEvent.setup();
    render(
      <ErrorBoundary onReset={onReset}>
        <Boom fail={true} />
      </ErrorBoundary>,
    );
    await user.click(screen.getByRole('button', { name: /try again/i }));
    expect(onReset).toHaveBeenCalled();
  });
});
