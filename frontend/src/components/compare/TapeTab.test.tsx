import { describe, expect, it } from 'vitest';
import { render, screen } from '@testing-library/react';

import { TapeTab } from './TapeTab';

const anchor = (key: string, turnIndex: number) => ({
  key,
  turn_index: turnIndex,
  parent_turn_index: turnIndex,
});

describe('TapeTab', () => {
  it('renders the empty state when no runs are passed', () => {
    render(<TapeTab runs={[]} />);
    expect(screen.getByText(/Select runs with completed transcripts/i)).toBeInTheDocument();
  });

  it('renders one column header per run plus the anchor gutter', () => {
    const runs = [
      { run_id: 'run-a', anchors: [anchor('Edit|src/main.go', 0)] },
      { run_id: 'run-b', anchors: [anchor('Edit|src/main.go', 0)] },
    ];
    render(<TapeTab runs={runs} />);
    expect(screen.getByRole('columnheader', { name: /anchor/i })).toBeInTheDocument();
    expect(screen.getByRole('columnheader', { name: 'run-a' })).toBeInTheDocument();
    expect(screen.getByRole('columnheader', { name: 'run-b' })).toBeInTheDocument();
  });

  it('renders an anchored row when the two runs share the anchor', () => {
    const runs = [
      { run_id: 'run-a', anchors: [anchor('Edit|src/main.go', 0)] },
      { run_id: 'run-b', anchors: [anchor('Edit|src/main.go', 0)] },
    ];
    render(<TapeTab runs={runs} />);
    expect(screen.getByRole('rowheader', { name: 'Edit|src/main.go' })).toBeInTheDocument();
  });

  it('renders drift rows when the runs diverge', () => {
    const runs = [
      { run_id: 'run-a', anchors: [anchor('Edit|a', 0), anchor('Edit|c', 5)] },
      { run_id: 'run-b', anchors: [anchor('Edit|b', 0), anchor('Edit|c', 5)] },
    ];
    render(<TapeTab runs={runs} />);
    // Three rows total: two drift (one per run's unique mid step) + one anchored.
    const driftHeaders = screen.getAllByRole('rowheader', { name: /drift/i });
    expect(driftHeaders.length).toBeGreaterThanOrEqual(1);
    expect(screen.getByRole('rowheader', { name: 'Edit|c' })).toBeInTheDocument();
  });
});
