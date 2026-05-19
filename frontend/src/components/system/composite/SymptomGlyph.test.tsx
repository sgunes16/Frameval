import { describe, expect, it } from 'vitest';
import { render, screen } from '@testing-library/react';

import { SymptomGlyph } from './SymptomGlyph';

describe('SymptomGlyph', () => {
  it('renders the 2-letter abbreviation for a known failure code', () => {
    render(<SymptomGlyph code="HAL_API" confidence={0.9} />);
    // HAL_API → HA
    expect(screen.getByText('HA')).toBeInTheDocument();
  });

  it('exposes the full code + confidence via title attribute', () => {
    render(<SymptomGlyph code="SCOPE_DRIFT" confidence={0.75} rationale="agent diverged from task" />);
    const el = screen.getByText('SD');
    const title = el.getAttribute('title') || el.parentElement?.getAttribute('title') || '';
    expect(title).toMatch(/SCOPE_DRIFT/);
    expect(title).toMatch(/0\.75/);
  });

  it('renders NONE as a muted neutral glyph (no failure)', () => {
    render(<SymptomGlyph code="NONE" confidence={0} />);
    // Component still renders something so layouts don't shift;
    // visually it must be the neutral variant.
    const el = screen.getByText('—');
    expect(el).toBeInTheDocument();
  });

  it('uses role=img with descriptive aria-label', () => {
    render(<SymptomGlyph code="HAL_API" confidence={0.9} />);
    expect(screen.getByRole('img', { name: /HAL_API/i })).toBeInTheDocument();
  });
});
