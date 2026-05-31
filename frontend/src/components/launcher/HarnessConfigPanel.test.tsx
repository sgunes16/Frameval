import { describe, expect, it, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { HarnessConfigPanel } from './HarnessConfigPanel';

describe('HarnessConfigPanel', () => {
  it('renders nothing for an unknown harness id', () => {
    const { container } = render(
      <HarnessConfigPanel harnessId="unknown" value={undefined} onChange={() => {}} />,
    );
    expect(container.firstChild).toBeNull();
  });

  it('renders a textarea for agent_instructions and reports edits', () => {
    const onChange = vi.fn();
    render(
      <HarnessConfigPanel
        harnessId="agent_instructions"
        value={{ content: 'hello' }}
        onChange={onChange}
      />,
    );
    const ta = screen.getByLabelText(/agent instructions/i) as HTMLTextAreaElement;
    expect(ta.value).toBe('hello');
    fireEvent.change(ta, { target: { value: 'updated' } });
    expect(onChange).toHaveBeenCalledWith({ content: 'updated' });
  });

  it('renders empty textarea when value has no content', () => {
    render(
      <HarnessConfigPanel harnessId="agent_instructions" value={undefined} onChange={() => {}} />,
    );
    const ta = screen.getByLabelText(/agent instructions/i) as HTMLTextAreaElement;
    expect(ta.value).toBe('');
  });
});
