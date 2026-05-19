import { describe, expect, it, vi } from 'vitest';
import { fireEvent, render, screen } from '@testing-library/react';

import { ArtifactDiff } from './ArtifactDiff';
import type { ArtifactVersion } from '../../lib/types';

const artifact = (id: string, content: string): ArtifactVersion => ({
  id,
  variant_id: `var-${id}`,
  artifact_type: 'CLAUDE.md',
  file_path: 'CLAUDE.md',
  content,
  content_hash: 'h',
  created_at: '',
});

describe('ArtifactDiff', () => {
  it('splits content by double-newline paragraphs and renders each pane', () => {
    const left = artifact('l', 'First paragraph.\n\nSecond paragraph.');
    const right = artifact('r', 'Different first.\n\nDifferent second.');
    render(<ArtifactDiff left={left} right={right} />);
    expect(screen.getByText('First paragraph.')).toBeInTheDocument();
    expect(screen.getByText('Second paragraph.')).toBeInTheDocument();
    expect(screen.getByText('Different first.')).toBeInTheDocument();
    expect(screen.getByText('Different second.')).toBeInTheDocument();
  });

  it('fires onParagraphHover with the paragraph text on mouse enter and null on leave', () => {
    const onParagraphHover = vi.fn();
    const left = artifact('l', 'Paragraph one.\n\nParagraph two.');
    const right = artifact('r', 'Other side.');
    render(<ArtifactDiff left={left} right={right} onParagraphHover={onParagraphHover} />);
    const para = screen.getByText('Paragraph two.');
    fireEvent.mouseEnter(para);
    expect(onParagraphHover).toHaveBeenLastCalledWith('Paragraph two.');
    fireEvent.mouseLeave(para);
    expect(onParagraphHover).toHaveBeenLastCalledWith(null);
  });

  it('renders activeLeftParagraph with the accent border style', () => {
    const left = artifact('l', 'A.\n\nB.');
    const right = artifact('r', 'X.');
    const { container } = render(
      <ArtifactDiff left={left} right={right} activeLeftParagraph={1} />,
    );
    const second = container.querySelector('[data-paragraph-index="1"]');
    expect(second?.className).toMatch(/border-accent/);
  });
});
