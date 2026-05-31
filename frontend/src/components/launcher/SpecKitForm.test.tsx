import { describe, expect, it, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import type { SpecKitExtensionPublic } from '../../lib/types';
import { SpecKitForm } from './SpecKitForm';

const CATALOG: SpecKitExtensionPublic[] = [
  { id: 'canonical', name: 'Canonical (4-stage)', description: 'baseline', stages: [], multi_agent: false },
  { id: 'lite', name: 'Lite (2-stage)', description: 'minimal', stages: [], multi_agent: false },
  { id: 'dual-role', name: 'Dual-role (multi-agent)', description: 'role-tagged', stages: [], multi_agent: true },
];

function setupQuery(initialData: SpecKitExtensionPublic[]) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  client.setQueryData(['speckit', 'catalog'], initialData);
  return client;
}

function Wrap({ children, client }: { children: React.ReactNode; client: QueryClient }) {
  return <QueryClientProvider client={client}>{children}</QueryClientProvider>;
}

describe('SpecKitForm', () => {
  it('seeds canonical on mount when value is undefined', () => {
    const onChange = vi.fn();
    const client = setupQuery(CATALOG);
    render(
      <Wrap client={client}>
        <SpecKitForm value={undefined} onChange={onChange} />
      </Wrap>,
    );
    expect(onChange).toHaveBeenCalledTimes(1);
    expect(onChange).toHaveBeenCalledWith({ extension_ids: ['canonical'] });
  });

  it('renders one chip per catalog entry', () => {
    const onChange = vi.fn();
    const client = setupQuery(CATALOG);
    render(
      <Wrap client={client}>
        <SpecKitForm value={{ extension_ids: ['canonical'] }} onChange={onChange} />
      </Wrap>,
    );
    expect(screen.getByRole('button', { name: /Canonical/i })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /Lite/i })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /Dual-role/i })).toBeInTheDocument();
  });

  it('shows the Multi-agent badge on the dual-role chip', () => {
    const onChange = vi.fn();
    const client = setupQuery(CATALOG);
    render(
      <Wrap client={client}>
        <SpecKitForm value={{ extension_ids: ['canonical'] }} onChange={onChange} />
      </Wrap>,
    );
    const dualBtn = screen.getByRole('button', { name: /Dual-role/i });
    expect(dualBtn.textContent).toMatch(/Multi-agent/i);
  });

  it('toggles selection on chip click', () => {
    const onChange = vi.fn();
    const client = setupQuery(CATALOG);
    render(
      <Wrap client={client}>
        <SpecKitForm value={{ extension_ids: ['canonical'] }} onChange={onChange} />
      </Wrap>,
    );
    fireEvent.click(screen.getByRole('button', { name: /Lite/i }));
    expect(onChange).toHaveBeenLastCalledWith({ extension_ids: ['canonical', 'lite'] });
  });

  it('shows the empty-catalog fallback message', () => {
    const onChange = vi.fn();
    const client = setupQuery([]);
    render(
      <Wrap client={client}>
        <SpecKitForm value={{ extension_ids: [] }} onChange={onChange} />
      </Wrap>,
    );
    expect(screen.getByText(/could not load/i)).toBeInTheDocument();
  });
});
