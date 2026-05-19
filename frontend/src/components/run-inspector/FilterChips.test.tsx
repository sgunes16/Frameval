import { describe, expect, it, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';

import { FilterChips } from './FilterChips';
import type { TurnFilter } from '../../lib/turn-filters';

describe('FilterChips', () => {
  it('renders four block + errors-only toggles', () => {
    render(<FilterChips filters={[]} onChange={() => {}} />);
    expect(screen.getByRole('button', { name: /thinking/i })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /tool use/i })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /tool result/i })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /errors only/i })).toBeInTheDocument();
  });

  it('marks active block toggles via aria-pressed', () => {
    render(
      <FilterChips
        filters={[{ kind: 'block', value: 'tool_use' }]}
        onChange={() => {}}
      />,
    );
    expect(screen.getByRole('button', { name: /tool use/i })).toHaveAttribute(
      'aria-pressed',
      'true',
    );
    expect(screen.getByRole('button', { name: /^thinking$/i })).toHaveAttribute(
      'aria-pressed',
      'false',
    );
  });

  it('toggling a block chip calls onChange with the added or removed filter', async () => {
    const onChange = vi.fn();
    const user = userEvent.setup();
    const { rerender } = render(<FilterChips filters={[]} onChange={onChange} />);
    await user.click(screen.getByRole('button', { name: /tool use/i }));
    expect(onChange).toHaveBeenLastCalledWith([{ kind: 'block', value: 'tool_use' }]);

    rerender(
      <FilterChips
        filters={[{ kind: 'block', value: 'tool_use' }]}
        onChange={onChange}
      />,
    );
    await user.click(screen.getByRole('button', { name: /tool use/i }));
    expect(onChange).toHaveBeenLastCalledWith([]);
  });

  it('submitting the path input adds a path filter', async () => {
    const onChange = vi.fn();
    const user = userEvent.setup();
    render(<FilterChips filters={[]} onChange={onChange} />);
    const input = screen.getByLabelText(/file path/i);
    await user.type(input, 'src/main.go{enter}');
    expect(onChange).toHaveBeenLastCalledWith([{ kind: 'path', value: 'src/main.go' }]);
  });

  it('renders active path filters as removable chips', async () => {
    const onChange = vi.fn();
    const user = userEvent.setup();
    const filters: TurnFilter[] = [{ kind: 'path', value: 'src/main.go' }];
    render(<FilterChips filters={filters} onChange={onChange} />);
    expect(screen.getByText('path:src/main.go')).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: /remove src\/main\.go filter/i }));
    expect(onChange).toHaveBeenLastCalledWith([]);
  });
});
