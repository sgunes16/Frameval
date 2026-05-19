import { describe, expect, it } from 'vitest';
import { render, screen } from '@testing-library/react';

import { Kbd } from './Kbd';

describe('Kbd', () => {
  it('renders the supplied key label', () => {
    render(<Kbd>Esc</Kbd>);
    expect(screen.getByText('Esc')).toBeInTheDocument();
  });

  it('renders as a <kbd> element semantically', () => {
    const { container } = render(<Kbd>K</Kbd>);
    expect(container.querySelector('kbd')).not.toBeNull();
  });
});
