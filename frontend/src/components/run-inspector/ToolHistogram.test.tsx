import { describe, expect, it } from 'vitest';
import { render, screen } from '@testing-library/react';

import { ToolHistogram } from './ToolHistogram';

describe('ToolHistogram', () => {
  it('renders an empty state when no tool calls happened', () => {
    render(<ToolHistogram rows={[]} />);
    expect(screen.getByText(/no tool calls/i)).toBeInTheDocument();
  });

  it('renders one row per tool with name and count', () => {
    render(
      <ToolHistogram
        rows={[
          { tool: 'Edit', count: 3 },
          { tool: 'Bash', count: 1 },
        ]}
      />,
    );
    expect(screen.getByText('Edit')).toBeInTheDocument();
    expect(screen.getByText('×3')).toBeInTheDocument();
    expect(screen.getByText('Bash')).toBeInTheDocument();
    expect(screen.getByText('×1')).toBeInTheDocument();
  });

  it('labels the list for accessibility', () => {
    render(<ToolHistogram rows={[{ tool: 'Read', count: 2 }]} />);
    expect(screen.getByRole('list', { name: /tool usage histogram/i })).toBeInTheDocument();
  });
});
