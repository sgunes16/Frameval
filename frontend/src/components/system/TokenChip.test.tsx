import { describe, expect, it } from 'vitest';
import { render, screen } from '@testing-library/react';

import { TokenChip } from './TokenChip';

describe('TokenChip', () => {
  // Text is split across multiple inline spans (in / · / out), so we
  // assert each fragment individually rather than fighting RTL's
  // string-match-across-elements limitation.

  it('formats values below 1k verbatim', () => {
    render(<TokenChip in={500} out={750} />);
    expect(screen.getByText('500 in')).toBeInTheDocument();
    expect(screen.getByText('750 out')).toBeInTheDocument();
  });

  it('formats thousands with a k suffix', () => {
    render(<TokenChip in={12_400} out={3_100} />);
    expect(screen.getByText('12.4k in')).toBeInTheDocument();
    expect(screen.getByText('3.1k out')).toBeInTheDocument();
  });

  it('formats millions with an M suffix', () => {
    render(<TokenChip in={2_500_000} out={0} />);
    expect(screen.getByText('2.5M in')).toBeInTheDocument();
  });

  it('renders monospace styling so digits align', () => {
    render(<TokenChip in={42} out={42} />);
    // The chip wraps the text spans; walk up to the chip span.
    const chip = screen.getByText('42 in').parentElement;
    expect(chip?.className).toMatch(/font-mono/);
  });
});
