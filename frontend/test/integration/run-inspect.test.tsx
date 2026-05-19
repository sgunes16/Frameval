import type { ReactNode } from 'react';
import { describe, expect, it } from 'vitest';
import { render, screen } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { http, HttpResponse } from 'msw';

import { RunInspectPage } from '../../src/pages/runs/inspect';
import { server } from '../msw/server';

function renderInspectAt(runId: string, ui?: ReactNode) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={client}>
      <MemoryRouter initialEntries={[`/runs/${runId}/inspect`]}>
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
});
