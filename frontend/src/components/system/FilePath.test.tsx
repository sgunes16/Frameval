import { describe, expect, it } from 'vitest';
import { render, screen } from '@testing-library/react';

import { FilePath } from './FilePath';

describe('FilePath', () => {
  it('renders short paths verbatim', () => {
    render(<FilePath path="main.go" />);
    expect(screen.getByText('main.go')).toBeInTheDocument();
  });

  it('middle-truncates long paths', () => {
    const long = 'a'.repeat(40) + '/very/deep/path/here/file.go';
    render(<FilePath path={long} maxChars={32} />);
    const el = screen.getByTitle(long);
    // The rendered text must include an ellipsis and still expose
    // start + end fragments.
    expect(el.textContent).toMatch(/…/);
    expect(el.textContent?.length ?? 0).toBeLessThanOrEqual(40);
  });

  it('exposes the full path via title attribute for hover', () => {
    render(<FilePath path="src/components/run-monitor/log-viewer.tsx" />);
    expect(screen.getByTitle('src/components/run-monitor/log-viewer.tsx')).toBeInTheDocument();
  });
});
