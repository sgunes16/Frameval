import { describe, expect, it } from 'vitest';
import { fireEvent, render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';

import { ReplayTab } from './ReplayTab';

const run = (id: string, keys: string[]) => ({
  run_id: id,
  anchors: keys.map((k, i) => ({ key: k, turn_index: i, parent_turn_index: i })),
});

describe('ReplayTab', () => {
  it('renders the empty state when no rows are produced', () => {
    render(<ReplayTab runs={[]} />);
    expect(screen.getByText(/Select runs with completed transcripts/i)).toBeInTheDocument();
  });

  it('shows the transport bar and exposes play/pause + scrub controls', () => {
    render(<ReplayTab runs={[run('r1', ['a']), run('r2', ['a'])]} />);
    expect(screen.getByRole('toolbar', { name: /replay transport/i })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /play replay/i })).toBeInTheDocument();
    expect(screen.getByRole('slider', { name: /replay position/i })).toBeInTheDocument();
  });

  it('arrow-right key advances the clock; arrow-left steps back', async () => {
    const runs = [
      run('r1', ['a', 'b', 'c']),
      run('r2', ['a', 'b', 'c']),
    ];
    render(<ReplayTab runs={runs} />);
    // Initial reveal: row 0 only (3 anchored rows total). Position
    // counter shows "1/3".
    expect(screen.getByText(/^1\/3$/)).toBeInTheDocument();
    fireEvent.keyDown(window, { key: 'ArrowRight' });
    expect(screen.getByText(/^2\/3$/)).toBeInTheDocument();
    fireEvent.keyDown(window, { key: 'ArrowLeft' });
    expect(screen.getByText(/^1\/3$/)).toBeInTheDocument();
  });

  it('Cmd/Ctrl-K does not hijack the space-key handler when an unrelated input has focus', async () => {
    // Focus a foreign input and press space — the clock should NOT
    // start because the keydown handler bails on input focus.
    const user = userEvent.setup();
    const runs = [run('r1', ['a', 'b']), run('r2', ['a', 'b'])];
    render(
      <>
        <input data-testid="foreign" />
        <ReplayTab runs={runs} />
      </>,
    );
    const foreign = screen.getByTestId('foreign') as HTMLInputElement;
    foreign.focus();
    await user.keyboard(' ');
    // Play button still in the "play" (not "pause") state.
    expect(screen.getByRole('button', { name: /play replay/i })).toBeInTheDocument();
  });
});
