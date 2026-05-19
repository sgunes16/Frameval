import { describe, expect, it } from 'vitest';
import { render, screen } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { http, HttpResponse } from 'msw';

import { ArtifactsTab } from './ArtifactsTab';
import { server } from '../../../test/msw/server';

function renderTab(variantIds: string[]) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <ArtifactsTab variantIds={variantIds} />
    </QueryClientProvider>,
  );
}

describe('ArtifactsTab', () => {
  it('renders the disabled affordance when fewer than two distinct variants are passed', () => {
    renderTab(['var-1', 'var-1']);
    expect(screen.getByRole('status')).toHaveTextContent(/at least two variants/i);
  });

  it('renders the diff for two variants that share an artifact_type', async () => {
    server.use(
      http.get('*/api/variants/var-a/artifacts', () =>
        HttpResponse.json([
          {
            id: 'art-a',
            variant_id: 'var-a',
            artifact_type: 'CLAUDE.md',
            file_path: 'CLAUDE.md',
            content: 'Variant A paragraph.\n\nAnother A paragraph.',
            content_hash: 'h-a',
            created_at: '',
          },
        ]),
      ),
      http.get('*/api/variants/var-b/artifacts', () =>
        HttpResponse.json([
          {
            id: 'art-b',
            variant_id: 'var-b',
            artifact_type: 'CLAUDE.md',
            file_path: 'CLAUDE.md',
            content: 'Variant B paragraph.\n\nAnother B paragraph.',
            content_hash: 'h-b',
            created_at: '',
          },
        ]),
      ),
    );

    renderTab(['var-a', 'var-b']);

    // Each paragraph from each variant renders.
    expect(await screen.findByText('Variant A paragraph.')).toBeInTheDocument();
    expect(await screen.findByText('Variant B paragraph.')).toBeInTheDocument();
  });

  it('flags artifacts that exist on only one side of a pair', async () => {
    server.use(
      http.get('*/api/variants/var-c/artifacts', () =>
        HttpResponse.json([
          {
            id: 'art-c',
            variant_id: 'var-c',
            artifact_type: 'spec.md',
            file_path: 'spec.md',
            content: 'spec content',
            content_hash: 'h-c',
            created_at: '',
          },
        ]),
      ),
      http.get('*/api/variants/var-d/artifacts', () => HttpResponse.json([])),
    );

    renderTab(['var-c', 'var-d']);
    expect(await screen.findByText(/present on only one variant/i)).toBeInTheDocument();
  });
});
