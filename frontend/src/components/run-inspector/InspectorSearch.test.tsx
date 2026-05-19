import { describe, expect, it, vi } from 'vitest';
import { fireEvent, render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';

import { InspectorSearch } from './InspectorSearch';
import type { ParsedTurn } from '../../lib/types';

const turns: ParsedTurn[] = [
  { role: 'assistant', content: 'hello world', block_kind: 'text', turn_index: 0, parent_turn_index: 0 },
  { role: 'assistant', content: 'rate limit exceeded', block_kind: 'text', turn_index: 1, parent_turn_index: 1 },
  { role: 'assistant', content: '', block_kind: 'tool_use', tool_name: 'Bash', turn_index: 2, parent_turn_index: 2 },
];

describe('InspectorSearch', () => {
  it('shows the trigger button when closed', () => {
    render(<InspectorSearch turns={turns} onFocus={() => {}} />);
    expect(screen.getByRole('button', { name: /search turns/i })).toBeInTheDocument();
  });

  it('opens the dialog on Cmd-K', async () => {
    render(<InspectorSearch turns={turns} onFocus={() => {}} />);
    fireEvent.keyDown(window, { key: 'k', metaKey: true });
    expect(await screen.findByRole('dialog', { name: /search turns/i })).toBeInTheDocument();
  });

  it('filters results live as the query changes', async () => {
    render(<InspectorSearch turns={turns} onFocus={() => {}} />);
    fireEvent.keyDown(window, { key: 'k', metaKey: true });
    const input = await screen.findByLabelText(/search query/i);
    const user = userEvent.setup();
    await user.type(input, 'rate limit');
    expect(await screen.findByText(/rate limit exceeded/i)).toBeInTheDocument();
    expect(screen.queryByText(/hello world/i)).not.toBeInTheDocument();
  });

  it('Enter chooses the highlighted result and calls onFocus with the parent index', async () => {
    const onFocus = vi.fn();
    render(<InspectorSearch turns={turns} onFocus={onFocus} />);
    fireEvent.keyDown(window, { key: 'k', metaKey: true });
    const input = await screen.findByLabelText(/search query/i);
    const user = userEvent.setup();
    await user.type(input, 'rate limit{enter}');
    expect(onFocus).toHaveBeenCalledWith(1);
  });

  it('Escape closes the dialog', async () => {
    render(<InspectorSearch turns={turns} onFocus={() => {}} />);
    fireEvent.keyDown(window, { key: 'k', metaKey: true });
    expect(await screen.findByRole('dialog')).toBeInTheDocument();
    fireEvent.keyDown(window, { key: 'Escape' });
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
  });

  it('marks the dialog as aria-modal', async () => {
    render(<InspectorSearch turns={turns} onFocus={() => {}} />);
    fireEvent.keyDown(window, { key: 'k', metaKey: true });
    const dialog = await screen.findByRole('dialog');
    expect(dialog).toHaveAttribute('aria-modal', 'true');
  });

  it('exposes the active option via aria-activedescendant for AT', async () => {
    render(<InspectorSearch turns={turns} onFocus={() => {}} />);
    fireEvent.keyDown(window, { key: 'k', metaKey: true });
    const input = await screen.findByLabelText(/search query/i);
    const user = userEvent.setup();
    await user.type(input, 'rate');
    expect(input).toHaveAttribute('aria-activedescendant');
    const activeId = input.getAttribute('aria-activedescendant');
    expect(activeId).toBeTruthy();
    expect(document.getElementById(activeId!)).toHaveAttribute('aria-selected', 'true');
  });

  it('ignores Cmd-K when focus is in a foreign input', () => {
    // Render the palette alongside an unrelated input. Focusing the
    // foreign input and pressing Cmd-K must NOT open the palette,
    // because the user is mid-typing somewhere else on the page.
    render(
      <>
        <input data-testid="foreign" />
        <InspectorSearch turns={turns} onFocus={() => {}} />
      </>,
    );
    const foreign = screen.getByTestId('foreign') as HTMLInputElement;
    foreign.focus();
    fireEvent.keyDown(window, { key: 'k', metaKey: true });
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
  });
});
