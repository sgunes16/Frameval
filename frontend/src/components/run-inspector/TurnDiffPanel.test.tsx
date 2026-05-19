import { describe, expect, it } from 'vitest';
import { render, screen } from '@testing-library/react';

import { TurnDiffPanel } from './TurnDiffPanel';
import type { FileDiff } from '../../lib/parse-patch';

const fd = (path: string, added: number, removed: number, hunks = '@@ stub @@'): FileDiff => ({
  path,
  added,
  removed,
  hunks,
});

describe('TurnDiffPanel', () => {
  it('renders an empty-state when no diffs apply', () => {
    render(<TurnDiffPanel diffs={[]} />);
    expect(screen.getByText(/didn't modify any files/i)).toBeInTheDocument();
  });

  it('renders one card per file with path, +N/−M and the hunk text', () => {
    render(
      <TurnDiffPanel
        diffs={[
          fd('src/main.go', 2, 1, '@@ -1,3 +1,4 @@\n hello\n+world'),
          fd('README.md', 5, 0, '@@ -0,0 +1,5 @@\n+hi'),
        ]}
      />,
    );
    expect(screen.getByText('src/main.go')).toBeInTheDocument();
    expect(screen.getByText('README.md')).toBeInTheDocument();
    expect(screen.getByText('+2')).toBeInTheDocument();
    expect(screen.getByText('−1')).toBeInTheDocument();
    expect(screen.getByText('+5')).toBeInTheDocument();
    expect(screen.getByText(/\+world/)).toBeInTheDocument();
  });
});
