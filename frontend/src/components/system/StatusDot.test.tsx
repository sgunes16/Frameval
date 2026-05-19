import { describe, expect, it } from 'vitest';
import { render, screen } from '@testing-library/react';

import { StatusDot } from './StatusDot';

describe('StatusDot', () => {
  it('renders with an accessible label', () => {
    render(<StatusDot variant="success" aria-label="Run completed" />);
    expect(screen.getByRole('status', { name: /run completed/i })).toBeInTheDocument();
  });

  it('applies the variant class to drive the token color', () => {
    render(<StatusDot variant="danger" aria-label="failed" />);
    const dot = screen.getByRole('status');
    // Token-driven utility — bg-danger reads --danger from tokens.css.
    expect(dot.className).toMatch(/bg-danger/);
  });

  it('opt-in pulse adds an animation class for live signals', () => {
    render(<StatusDot variant="warning" pulse aria-label="running" />);
    expect(screen.getByRole('status').className).toMatch(/animate-pulse/);
  });

  it('default variant falls back to neutral fg-subtle', () => {
    render(<StatusDot aria-label="idle" />);
    expect(screen.getByRole('status').className).toMatch(/bg-fg-subtle/);
  });
});
