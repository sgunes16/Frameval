import { describe, expect, it } from 'vitest';
import { render, screen } from '@testing-library/react';

import { LiveCursor } from './LiveCursor';

describe('LiveCursor', () => {
  it('renders Reconnecting when disconnected', () => {
    render(<LiveCursor isConnected={false} lastEventAt={null} turnCount={null} />);
    expect(screen.getByText(/Reconnecting/i)).toBeInTheDocument();
  });

  it('renders Connected (idle) when connected but no event yet', () => {
    render(<LiveCursor isConnected={true} lastEventAt={null} turnCount={null} />);
    expect(screen.getByText(/Connected/i)).toBeInTheDocument();
  });

  it('renders Live with relative time and turn count when streaming', () => {
    const justNow = Date.now() - 1500;
    render(<LiveCursor isConnected={true} lastEventAt={justNow} turnCount={12} />);
    expect(screen.getByText(/Live/i)).toBeInTheDocument();
    expect(screen.getByText(/12 turns/i)).toBeInTheDocument();
  });

  it('exposes status via role=status for AT live regions', () => {
    render(<LiveCursor isConnected={true} lastEventAt={Date.now()} turnCount={3} />);
    expect(screen.getByRole('status')).toHaveAttribute('aria-live', 'polite');
  });
});
