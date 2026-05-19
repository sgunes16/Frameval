import { describe, expect, it, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';

import { MatrixTab } from './MatrixTab';

const run = (id: string, keys: string[]) => ({
  run_id: id,
  anchors: keys.map((k, i) => ({ key: k, turn_index: i, parent_turn_index: i })),
});

describe('MatrixTab', () => {
  it('renders the disabled affordance when fewer than 3 runs are selected', () => {
    render(<MatrixTab runs={[run('r1', ['a']), run('r2', ['a'])]} />);
    expect(screen.getByRole('status')).toHaveTextContent(/at least three runs/i);
  });

  it('renders a column + row header per run and N² cells', () => {
    const runs = [run('r1', ['a']), run('r2', ['a']), run('r3', ['b'])];
    render(<MatrixTab runs={runs} />);
    expect(screen.getByRole('columnheader', { name: 'r1' })).toBeInTheDocument();
    expect(screen.getByRole('rowheader', { name: 'r2' })).toBeInTheDocument();
    // 3 × 3 = 9 cells; diagonal cells are role=gridcell too.
    expect(screen.getAllByRole('gridcell')).toHaveLength(9);
  });

  it('off-diagonal cell click invokes onPairClick with the two run ids', async () => {
    const onPairClick = vi.fn();
    const user = userEvent.setup();
    const runs = [run('r1', ['a']), run('r2', ['a']), run('r3', ['b'])];
    render(<MatrixTab runs={runs} onPairClick={onPairClick} />);
    // Pick the r1 ↔ r2 cell via its accessible name.
    const cell = screen.getByRole('gridcell', { name: /Similarity between r1 and r2/i });
    await user.click(cell);
    expect(onPairClick).toHaveBeenCalledWith('r1', 'r2');
  });

  it('diagonal cells are NOT clickable (no onClick path)', async () => {
    const onPairClick = vi.fn();
    const user = userEvent.setup();
    const runs = [run('r1', ['a']), run('r2', ['a']), run('r3', ['b'])];
    render(<MatrixTab runs={runs} onPairClick={onPairClick} />);
    const diag = screen.getByRole('gridcell', { name: /r2 self-similarity/i });
    await user.click(diag);
    expect(onPairClick).not.toHaveBeenCalled();
  });
});
