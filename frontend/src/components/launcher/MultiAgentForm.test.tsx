import { describe, expect, it, vi } from 'vitest';
import { render, screen, fireEvent, within } from '@testing-library/react';
import { MultiAgentForm } from './MultiAgentForm';

describe('MultiAgentForm', () => {
  it('seeds planner + coder roles when value is undefined', () => {
    const onChange = vi.fn();
    render(<MultiAgentForm value={undefined} onChange={onChange} />);
    // The first render should call onChange once with the seed value
    // so the parent picks up the default.
    expect(onChange).toHaveBeenCalledTimes(1);
    const seed = onChange.mock.calls[0][0];
    expect(seed.roles).toHaveLength(2);
    expect(seed.roles[0].name).toBe('planner');
    expect(seed.roles[1].name).toBe('coder');
  });

  it('renders one row per role and edits name / prompt round-trip', () => {
    const onChange = vi.fn();
    render(<MultiAgentForm value={{ roles: [{ name: 'planner', prompt: 'p1' }] }} onChange={onChange} />);
    const nameInput = screen.getByLabelText(/role 1 name/i) as HTMLInputElement;
    expect(nameInput.value).toBe('planner');
    fireEvent.change(nameInput, { target: { value: 'plotter' } });
    expect(onChange).toHaveBeenCalledWith({ roles: [{ name: 'plotter', prompt: 'p1' }] });

    const promptArea = screen.getByLabelText(/role 1 prompt/i) as HTMLTextAreaElement;
    fireEvent.change(promptArea, { target: { value: 'new prompt' } });
    expect(onChange).toHaveBeenLastCalledWith({ roles: [{ name: 'planner', prompt: 'new prompt' }] });
  });

  it('adds a new empty role up to the 5-role cap', () => {
    const onChange = vi.fn();
    const four = Array.from({ length: 4 }, (_, i) => ({ name: `r${i}`, prompt: 'x' }));
    render(<MultiAgentForm value={{ roles: four }} onChange={onChange} />);
    fireEvent.click(screen.getByRole('button', { name: /add role/i }));
    expect(onChange).toHaveBeenCalled();
    const last = onChange.mock.calls[onChange.mock.calls.length - 1][0];
    expect(last.roles).toHaveLength(5);

    // Re-render with the new five-role value; Add button should now be disabled.
    onChange.mockClear();
    const { container } = render(<MultiAgentForm value={last} onChange={onChange} />);
    expect(within(container).getByRole('button', { name: /add role/i })).toBeDisabled();
  });

  it('removes a role; Remove is disabled when only one role remains', () => {
    const onChange = vi.fn();
    const two = { roles: [{ name: 'a', prompt: 'x' }, { name: 'b', prompt: 'y' }] };
    const { rerender } = render(<MultiAgentForm value={two} onChange={onChange} />);
    const removeButtons = screen.getAllByRole('button', { name: /remove role/i });
    fireEvent.click(removeButtons[0]);
    const after = onChange.mock.calls[onChange.mock.calls.length - 1][0];
    expect(after.roles).toEqual([{ name: 'b', prompt: 'y' }]);

    rerender(<MultiAgentForm value={after} onChange={onChange} />);
    expect(screen.getByRole('button', { name: /remove role/i })).toBeDisabled();
  });

  it('reorders roles with up / down arrows', () => {
    const onChange = vi.fn();
    const two = { roles: [{ name: 'a', prompt: 'x' }, { name: 'b', prompt: 'y' }] };
    render(<MultiAgentForm value={two} onChange={onChange} />);
    fireEvent.click(screen.getByRole('button', { name: /move role 1 down/i }));
    expect(onChange).toHaveBeenLastCalledWith({ roles: [{ name: 'b', prompt: 'y' }, { name: 'a', prompt: 'x' }] });
  });
});
