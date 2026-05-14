import type { ReactNode } from 'react';
import { describe, expect, it } from 'vitest';
import { renderHook, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { http, HttpResponse } from 'msw';

import { useExperiments } from '../../src/lib/hooks';
import { server } from '../msw/server';
import { makeExperiment } from '../fixtures/experiments';

/**
 * Hook-level integration test.
 *
 * History (#98): the original attempt under happy-dom v15 hung with
 * `Invalid state: ReadableStream is locked` — a known incompatibility
 * between happy-dom v15's fetch and msw v2's response stream. Upgrading
 * happy-dom to v18 fixes it without any other test-environment tricks.
 *
 * Pattern to copy for new hook tests:
 *   1. Fresh QueryClient per test — no shared cache between cases.
 *   2. retry: false so a single network error fails fast.
 *   3. Use waitFor for any react-query state assertion — the synchronous
 *      result.current snapshot taken right after renderHook is pre-fetch.
 */

function wrapper({ children }: { children: ReactNode }) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return <QueryClientProvider client={client}>{children}</QueryClientProvider>;
}

describe('useExperiments hook + MSW', () => {
  it('returns the default empty list', async () => {
    const { result } = renderHook(() => useExperiments(), { wrapper });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(result.current.data).toEqual([]);
  });

  it('returns whatever the per-test override responds with', async () => {
    server.use(
      http.get('*/api/experiments', () =>
        HttpResponse.json([makeExperiment({ id: 'override-1', name: 'Override test' })]),
      ),
    );

    const { result } = renderHook(() => useExperiments(), { wrapper });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(result.current.data).toHaveLength(1);
    expect(result.current.data?.[0]?.name).toBe('Override test');
  });
});
