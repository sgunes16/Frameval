import type { ReactNode } from 'react';
import { describe, expect, it } from 'vitest';
import { render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';

import { DashboardPage } from '../../src/pages/dashboard';

/**
 * Page-level integration test driver. Renders the DashboardPage with all the
 * providers it needs (router, react-query) and asserts UI state against the
 * default MSW handler set.
 *
 * The default `/api/experiments` handler returns an empty list, which is
 * exactly the empty-state path the DashboardPage renders. This test is the
 * "smoke check" — it proves the entire harness is wired (MSW → fetch →
 * react-query → React → DOM) end-to-end. Richer assertions per scenario
 * live in their own focused tests.
 *
 * Pattern to copy when adding a new page test:
 *   1. Override the relevant MSW endpoint with `server.use(http.get(...))`.
 *   2. Render the page wrapped in `renderWithProviders`.
 *   3. Use `await screen.findBy*` for content that depends on react-query
 *      async resolution.
 */

function renderWithProviders(node: ReactNode) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: 0 } },
  });
  return render(
    <QueryClientProvider client={client}>
      <MemoryRouter>{node}</MemoryRouter>
    </QueryClientProvider>,
  );
}

describe('DashboardPage', () => {
  it('renders the empty state when no experiments exist', async () => {
    renderWithProviders(<DashboardPage />);

    expect(await screen.findByText(/no experiments yet/i)).toBeInTheDocument();
  });
});
