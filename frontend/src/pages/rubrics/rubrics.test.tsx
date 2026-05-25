import { render, screen } from '@testing-library/react';
import { describe, it, expect, vi } from 'vitest';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter, Route, Routes } from 'react-router-dom';

import { RubricsPage } from './index';

vi.mock('../../lib/hooks', () => ({
  useRubrics: () => ({
    data: [
      { key: 'correctness', display_name: 'Correctness', prompt: 'p1', sort_order: 1, is_builtin: true },
      { key: 'security',    display_name: 'Security',    prompt: 'p2', sort_order: 99, is_builtin: false },
    ],
    isLoading: false, isError: false, refetch: vi.fn(),
  }),
  useDeleteRubric: () => ({ mutate: vi.fn(), isPending: false }),
}));

describe('RubricsPage', () => {
  it('renders builtin + custom rubrics with appropriate actions', () => {
    const qc = new QueryClient();
    render(
      <QueryClientProvider client={qc}>
        <MemoryRouter initialEntries={['/rubrics']}>
          <Routes>
            <Route path="/rubrics" element={<RubricsPage />} />
          </Routes>
        </MemoryRouter>
      </QueryClientProvider>,
    );
    expect(screen.getByText('Correctness')).toBeInTheDocument();
    expect(screen.getByText('Security')).toBeInTheDocument();
    expect(screen.getAllByText('Edit')).toHaveLength(2);
    expect(screen.getAllByText('Delete')).toHaveLength(1); // only Security
  });
});
