import type { ReactNode } from 'react';
import { describe, expect, it } from 'vitest';
import { fireEvent, render, screen } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { http, HttpResponse } from 'msw';

import { RunInspectPage } from '../../src/pages/runs/inspect';
import { server } from '../msw/server';

function renderInspectAt(runId: string, ui?: ReactNode, search = '') {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  const initialEntry = search ? `/runs/${runId}/inspect?${search}` : `/runs/${runId}/inspect`;
  return render(
    <QueryClientProvider client={client}>
      <MemoryRouter initialEntries={[initialEntry]}>
        <Routes>
          <Route path="/runs/:id/inspect" element={ui ?? <RunInspectPage />} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

describe('RunInspectPage', () => {
  it('renders the empty-turns affordance when the run has no transcript yet', async () => {
    server.use(
      http.get('*/api/runs/run-fresh', () =>
        HttpResponse.json({
          id: 'run-fresh',
          experiment_id: 'exp-1',
          variant_id: 'var-1',
          run_number: 0,
          status: 'running',
        }),
      ),
      http.get('*/api/runs/run-fresh/turns', () => HttpResponse.json([])),
    );

    renderInspectAt('run-fresh');

    expect(await screen.findByText(/no turns yet/i)).toBeInTheDocument();
    expect(await screen.findByText(/run-fresh/)).toBeInTheDocument();
  });

  it('mounts the virtualized turn list when transcript data arrives', async () => {
    // happy-dom doesn't measure layout so react-virtual sees a
    // viewport size of 0 and renders no virtual rows. We can still
    // assert the harness scaffolding mounted (the parent scroll
    // container) and that the empty-state affordance does NOT show —
    // proving the data path through the page is wired correctly.
    server.use(
      http.get('*/api/runs/run-complete', () =>
        HttpResponse.json({
          id: 'run-complete',
          experiment_id: 'exp-1',
          variant_id: 'var-1',
          run_number: 0,
          status: 'completed',
        }),
      ),
      http.get('*/api/runs/run-complete/turns', () =>
        HttpResponse.json([
          { role: 'assistant', content: 'thinking out loud', turn_index: 0, parent_turn_index: 0, block_kind: 'thinking' },
          { role: 'assistant', content: 'edit src/main.go', turn_index: 1, parent_turn_index: 0, block_kind: 'tool_use', tool_name: 'Edit' },
        ]),
      ),
      http.get('*/api/runs/run-complete/transcript', () =>
        HttpResponse.json({
          id: 't1',
          run_id: 'run-complete',
          raw_output: '',
          patch: '',
          total_turns: 2,
          total_tokens: 0,
        }),
      ),
    );

    renderInspectAt('run-complete');

    // The list container mounts as soon as the query resolves.
    expect(await screen.findByTestId('turn-list')).toBeInTheDocument();
    // The empty-state path is NOT taken — proves the turns array
    // was non-empty when the list rendered.
    expect(screen.queryByText(/no turns yet/i)).not.toBeInTheDocument();
  });

  it('shows the tool histogram in the right pane when tool_use blocks exist', async () => {
    server.use(
      http.get('*/api/runs/run-hist', () =>
        HttpResponse.json({
          id: 'run-hist',
          experiment_id: 'exp-1',
          variant_id: 'var-1',
          run_number: 0,
          status: 'completed',
        }),
      ),
      http.get('*/api/runs/run-hist/turns', () =>
        HttpResponse.json([
          { role: 'assistant', content: '', turn_index: 0, parent_turn_index: 0, block_kind: 'tool_use', tool_name: 'Edit' },
          { role: 'assistant', content: '', turn_index: 1, parent_turn_index: 1, block_kind: 'tool_use', tool_name: 'Bash' },
          { role: 'assistant', content: '', turn_index: 2, parent_turn_index: 2, block_kind: 'tool_use', tool_name: 'Edit' },
        ]),
      ),
      http.get('*/api/runs/run-hist/transcript', () =>
        HttpResponse.json({
          id: 't1',
          run_id: 'run-hist',
          raw_output: '',
          patch: '',
          total_turns: 3,
          total_tokens: 0,
        }),
      ),
    );

    renderInspectAt('run-hist');

    // Right pane lists Edit ×2 and Bash ×1, sorted desc by count.
    expect(await screen.findByText('Edit')).toBeInTheDocument();
    expect(screen.getByText('×2')).toBeInTheDocument();
    expect(screen.getByText('Bash')).toBeInTheDocument();
    expect(screen.getByText('×1')).toBeInTheDocument();
  });

  it('focusing a turn renders its file diffs in the right pane', async () => {
    // happy-dom doesn't measure layout, so react-virtual otherwise
    // returns viewport size = 0 and skips rendering rows. We stub
    // offsetHeight/Width and getBoundingClientRect just enough to let
    // the virtualizer render at least one row so the focus → diff
    // path can be exercised end-to-end.
    const restoreHeight = Object.getOwnPropertyDescriptor(
      HTMLElement.prototype,
      'offsetHeight',
    );
    const restoreWidth = Object.getOwnPropertyDescriptor(
      HTMLElement.prototype,
      'offsetWidth',
    );
    Object.defineProperty(HTMLElement.prototype, 'offsetHeight', {
      configurable: true,
      get() {
        return 800;
      },
    });
    Object.defineProperty(HTMLElement.prototype, 'offsetWidth', {
      configurable: true,
      get() {
        return 800;
      },
    });
    const origGetRect = Element.prototype.getBoundingClientRect;
    Element.prototype.getBoundingClientRect = function () {
      return {
        x: 0,
        y: 0,
        top: 0,
        left: 0,
        right: 800,
        bottom: 800,
        width: 800,
        height: 800,
        toJSON: () => ({}),
      } as DOMRect;
    };

    const patch = `diff --git a/src/main.go b/src/main.go
--- a/src/main.go
+++ b/src/main.go
@@ -1,3 +1,4 @@
 package main
+import "log"
`;
    server.use(
      http.get('*/api/runs/run-diff', () =>
        HttpResponse.json({
          id: 'run-diff',
          experiment_id: 'exp-1',
          variant_id: 'var-1',
          run_number: 0,
          status: 'completed',
        }),
      ),
      http.get('*/api/runs/run-diff/turns', () =>
        HttpResponse.json([
          {
            role: 'assistant',
            content: '',
            turn_index: 0,
            parent_turn_index: 0,
            block_kind: 'tool_use',
            tool_name: 'Edit',
            files_touched: ['src/main.go'],
          },
        ]),
      ),
      http.get('*/api/runs/run-diff/transcript', () =>
        HttpResponse.json({
          id: 't1',
          run_id: 'run-diff',
          raw_output: '',
          patch,
          total_turns: 1,
          total_tokens: 0,
        }),
      ),
    );

    renderInspectAt('run-diff');

    // Before focus: the "click any turn" empty-state copy is visible.
    expect(await screen.findByText(/Click any turn/i)).toBeInTheDocument();

    // Tab to the focus target (the role="button" wrapper on the
    // TurnGroupCard) and activate it. fireEvent.click talks directly
    // to the React synthetic event system, which is more reliable
    // than userEvent under happy-dom's pointer-event emulation.
    const focusTarget = await screen.findByRole('button', { name: /focus turn 0/i });
    fireEvent.click(focusTarget);

    // Right pane now reports the focused turn header and the diff.
    // The "Focused turn" label is the unique cue we look for; once
    // it appears the rest of the right-pane content is in the DOM.
    // "src/main.go" appears in both the left-pane tool_use block and
    // the right-pane TurnDiffPanel — assert at least one chip rendered.
    expect(await screen.findByText(/^Focused turn$/i)).toBeInTheDocument();
    const paths = await screen.findAllByText('src/main.go');
    expect(paths.length).toBeGreaterThanOrEqual(1);
    expect(screen.getByText('+1')).toBeInTheDocument();

    if (restoreHeight) {
      Object.defineProperty(HTMLElement.prototype, 'offsetHeight', restoreHeight);
    }
    if (restoreWidth) {
      Object.defineProperty(HTMLElement.prototype, 'offsetWidth', restoreWidth);
    }
    Element.prototype.getBoundingClientRect = origGetRect;
  });

  it('honours filter chips passed via URL search params', async () => {
    server.use(
      http.get('*/api/runs/run-url', () =>
        HttpResponse.json({
          id: 'run-url',
          experiment_id: 'exp-1',
          variant_id: 'var-1',
          run_number: 0,
          status: 'completed',
        }),
      ),
      http.get('*/api/runs/run-url/turns', () =>
        HttpResponse.json([
          { role: 'assistant', content: 'thinking only', turn_index: 0, parent_turn_index: 0, block_kind: 'thinking' },
          { role: 'assistant', content: 'tool call', turn_index: 1, parent_turn_index: 1, block_kind: 'tool_use', tool_name: 'Edit' },
        ]),
      ),
      http.get('*/api/runs/run-url/transcript', () =>
        HttpResponse.json({
          id: 't1',
          run_id: 'run-url',
          raw_output: '',
          patch: '',
          total_turns: 2,
          total_tokens: 0,
        }),
      ),
    );

    // ?filter=tool_use means the thinking block must be filtered out.
    // The Tool use chip should be marked pressed on mount.
    renderInspectAt('run-url', undefined, 'filter=tool_use');

    const toolUseChip = await screen.findByRole('button', { name: /tool use/i });
    expect(toolUseChip).toHaveAttribute('aria-pressed', 'true');
  });
});
