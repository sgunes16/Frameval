import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';

import { ThemeToggle } from './theme-toggle';

beforeEach(() => {
  localStorage.clear();
  document.documentElement.classList.remove('dark');
});

afterEach(() => {
  localStorage.clear();
  document.documentElement.classList.remove('dark');
});

describe('ThemeToggle', () => {
  it('renders three labelled segments', () => {
    render(<ThemeToggle />);
    expect(screen.getByRole('button', { name: /system theme/i })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /light theme/i })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /dark theme/i })).toBeInTheDocument();
  });

  it('marks the active mode via aria-pressed', () => {
    render(<ThemeToggle />);
    // Default mode is 'system'.
    expect(screen.getByRole('button', { name: /system theme/i })).toHaveAttribute(
      'aria-pressed',
      'true',
    );
  });

  it('toggles the html .dark class when dark is chosen', async () => {
    const user = userEvent.setup();
    render(<ThemeToggle />);
    await user.click(screen.getByRole('button', { name: /dark theme/i }));
    expect(document.documentElement.classList.contains('dark')).toBe(true);
  });
});
