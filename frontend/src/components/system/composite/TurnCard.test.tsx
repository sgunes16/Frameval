import { describe, expect, it } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';

import { TurnCard } from './TurnCard';

describe('TurnCard', () => {
  it('renders turn index in the header', () => {
    render(
      <TurnCard turnIndex={7} blockKind="tool_use">
        body content
      </TurnCard>,
    );
    expect(screen.getByText(/turn 7/i)).toBeInTheDocument();
  });

  it('renders children as the card body', () => {
    render(
      <TurnCard turnIndex={1} blockKind="thinking">
        let me read the file first
      </TurnCard>,
    );
    expect(screen.getByText(/let me read the file first/i)).toBeInTheDocument();
  });

  it('left-bar color reflects the block kind', () => {
    const { container } = render(
      <TurnCard turnIndex={1} blockKind="tool_use">
        x
      </TurnCard>,
    );
    // The left bar is a span with data-testid="turn-bar"; its className
    // contains the variant color (chart-1 for tool_use).
    const bar = container.querySelector('[data-testid="turn-bar"]');
    expect(bar?.className).toMatch(/bg-chart/);
  });

  it('collapses to header-only when collapsed=true', async () => {
    const user = userEvent.setup();
    render(
      <TurnCard turnIndex={1} blockKind="text" defaultCollapsed>
        secret body that should be hidden
      </TurnCard>,
    );
    expect(screen.queryByText(/secret body/i)).not.toBeInTheDocument();
    // Clicking the disclosure expands.
    await user.click(screen.getByRole('button', { name: /expand turn 1/i }));
    expect(screen.getByText(/secret body/i)).toBeInTheDocument();
  });

  it('renders the toolName when supplied', () => {
    render(
      <TurnCard turnIndex={1} blockKind="tool_use" toolName="Edit">
        x
      </TurnCard>,
    );
    expect(screen.getByText('Edit')).toBeInTheDocument();
  });
});
