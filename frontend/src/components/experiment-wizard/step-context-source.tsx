import { useMemo, useState } from 'react';
import { Badge } from '../ui/badge';
import { Button } from '../ui/button';
import { Input } from '../ui/input';
import type { CatalogExtension } from '../../lib/types';
import { StepCustomArtifacts, type DraftArtifactFile } from './step-custom-artifacts';

export type DraftVariantContext = {
  name: string;
  description: string;
  is_control: boolean;
  catalogExtensionIds: string[];
  customFiles: DraftArtifactFile[];
};

const CATEGORY_FILTERS: Array<{ id: string; label: string; tags?: string[] }> = [
  { id: 'all', label: 'All process extensions' },
  {
    id: 'planning',
    label: 'Planning & specs',
    tags: ['planning', 'specifications', 'product-spec', 'spec-first', 'constitution', 'architecture', 'research'],
  },
  {
    id: 'workflow',
    label: 'Workflow & methodology',
    tags: ['workflow', 'methodology', 'sdlc', 'cdd', 'tdd', 'v-model', 'lifecycle', 'iteration'],
  },
  {
    id: 'implementation',
    label: 'Implementation & agents',
    tags: ['implementation', 'agent', 'agents', 'multi-agent', 'orchestration', 'parallel', 'worktree'],
  },
  {
    id: 'quality',
    label: 'Quality & review',
    tags: ['quality', 'quality-assurance', 'quality-gate', 'testing', 'test-generation', 'review', 'code-review', 'audit', 'coverage'],
  },
  {
    id: 'maintenance',
    label: 'Maintenance & bugfix',
    tags: ['bugfix', 'debugging', 'maintenance', 'refactor', 'remediation', 'tech-debt', 'cleanup', 'fixit'],
  },
  {
    id: 'governance',
    label: 'Governance & compliance',
    tags: ['governance', 'compliance', 'security', 'devsecops', 'owasp', 'enforcement', 'risk-assessment', 'safety-critical'],
  },
  {
    id: 'integration',
    label: 'Integration & tracking',
    tags: ['integration', 'issue-tracking', 'project-management', 'jira', 'linear', 'github', 'github-projects', 'azure-devops', 'trello', 'confluence'],
  },
  {
    id: 'analytics',
    label: 'Analytics & visualization',
    tags: ['metrics', 'analytics', 'diagnostics', 'visualization', 'diagram', 'mermaid', 'progress', 'tracking'],
  },
];

export function StepContextSource({
  variant,
  catalogExtensions,
  contextDisabled = false,
  onChange,
}: {
  variant: DraftVariantContext;
  catalogExtensions: CatalogExtension[];
  contextDisabled?: boolean;
  onChange: (variant: DraftVariantContext) => void;
}) {
  const [query, setQuery] = useState('');
  const [filter, setFilter] = useState<string>('all');

  const filteredExtensions = useMemo(() => {
    const normalizedQuery = query.trim().toLowerCase();
    const activeFilter = CATEGORY_FILTERS.find((item) => item.id === filter);
    const filterTags = new Set((activeFilter?.tags ?? []).map((tag) => tag.toLowerCase()));
    return catalogExtensions.filter((extension) => {
      const extensionTags = (extension.tags ?? []).map((tag) => tag.toLowerCase());
      const matchesFilter = filter === 'all' || extensionTags.some((tag) => filterTags.has(tag));
      const haystack = [extension.name, extension.description, extension.author, ...(extension.tags ?? [])]
        .filter(Boolean)
        .join(' ')
        .toLowerCase();
      const matchesQuery = !normalizedQuery || haystack.includes(normalizedQuery);
      return matchesFilter && matchesQuery;
    });
  }, [catalogExtensions, filter, query]);

  return (
    <div className="space-y-4">
      <div className="grid gap-3 sm:grid-cols-2">
        <Input
          value={variant.name}
          onChange={(event) => onChange({ ...variant, name: event.target.value })}
          placeholder="Variant name"
        />
        <label className="flex items-center gap-2 rounded-lg border border-slate-200 bg-white px-3 text-sm text-slate-700">
          <input
            type="checkbox"
            checked={variant.is_control}
            onChange={(event) =>
              onChange({
                ...variant,
                is_control: event.target.checked,
                catalogExtensionIds: event.target.checked ? [] : variant.catalogExtensionIds,
                customFiles: event.target.checked ? [] : variant.customFiles,
              })
            }
          />
          Mark as control variant
        </label>
      </div>
      <textarea
        className="min-h-20 w-full rounded-lg border border-slate-300 bg-white p-3 text-sm shadow-[0_1px_2px_rgba(15,23,42,0.04)]"
        value={variant.description}
        onChange={(event) => onChange({ ...variant, description: event.target.value })}
        placeholder="What is this variant testing?"
      />

      {contextDisabled ? (
        <div className="rounded-lg border border-dashed border-slate-200 bg-slate-50 p-4 text-xs text-slate-500">
          Control variants intentionally run without catalog extensions or custom context files.
        </div>
      ) : (
        <>
          <div className="space-y-3 rounded-lg border border-slate-200 bg-white p-4">
        <div className="flex flex-wrap items-center justify-between gap-2">
          <div>
            <div className="text-sm font-semibold text-slate-900">Spec-kit catalog</div>
            <div className="text-xs text-slate-500">
              {catalogExtensions.length} community extensions available · filter by process area.
            </div>
          </div>
          <Badge tone="info">{variant.catalogExtensionIds.length} selected</Badge>
        </div>
        <div className="flex flex-wrap gap-2">
          {CATEGORY_FILTERS.map((item) => (
            <button
              key={item.id}
              type="button"
              onClick={() => setFilter(item.id)}
              className={
                filter === item.id
                  ? 'rounded-full bg-slate-900 px-3 py-1 text-[11px] font-medium text-white'
                  : 'rounded-full border border-slate-200 bg-white px-3 py-1 text-[11px] font-medium text-slate-600 hover:border-slate-300'
              }
            >
              {item.label}
            </button>
          ))}
        </div>
        <Input value={query} onChange={(event) => setQuery(event.target.value)} placeholder="Search extension name, tag, or author..." />
        <div className="max-h-[420px] space-y-2 overflow-y-auto pr-1">
          {filteredExtensions.map((extension) => {
            const selected = variant.catalogExtensionIds.includes(extension.id);
            return (
              <button
                key={extension.id}
                type="button"
                onClick={() =>
                  onChange({
                    ...variant,
                    catalogExtensionIds: selected
                      ? variant.catalogExtensionIds.filter((id) => id !== extension.id)
                      : [...variant.catalogExtensionIds, extension.id],
                  })
                }
                className={
                  selected
                    ? 'flex w-full flex-col gap-1 rounded-lg border border-slate-900 bg-slate-900/5 p-3 text-left'
                    : 'flex w-full flex-col gap-1 rounded-lg border border-slate-200 bg-white p-3 text-left transition hover:border-slate-300'
                }
              >
                <div className="flex items-center justify-between gap-2">
                  <div className="text-sm font-medium text-slate-900">{extension.name}</div>
                  <span className="text-[11px] text-slate-500">{extension.id}</span>
                </div>
                <div className="text-xs text-slate-500">{extension.description}</div>
                <div className="mt-1 flex flex-wrap gap-1">
                  {(extension.tags ?? []).slice(0, 6).map((tag) => (
                    <Badge key={tag} tone="neutral">
                      {tag}
                    </Badge>
                  ))}
                </div>
              </button>
            );
          })}
          {filteredExtensions.length === 0 && (
            <div className="rounded-lg border border-dashed border-slate-200 bg-slate-50 p-3 text-xs text-slate-500">
              No extensions match the current filter or search.
            </div>
          )}
        </div>
      </div>

          <div className="rounded-lg border border-slate-200 bg-white p-4">
        <div className="mb-3 flex items-center justify-between gap-2">
          <div>
            <div className="text-sm font-semibold text-slate-900">Custom context files</div>
            <div className="text-xs text-slate-500">Attach AGENTS.md, CLAUDE.md, .cursorrules, or any supporting file.</div>
          </div>
        </div>
        <StepCustomArtifacts files={variant.customFiles} onChange={(customFiles) => onChange({ ...variant, customFiles })} />
      </div>
        </>
      )}

      {!contextDisabled && !variant.catalogExtensionIds.length && !variant.customFiles.length && (
        <div className="rounded-lg border border-dashed border-slate-200 bg-slate-50 p-3 text-xs text-slate-500">
          This variant has no extra context yet. That&apos;s fine for a control variant.
        </div>
      )}
    </div>
  );
}

export function AddComparisonVariantButton({ onClick }: { onClick: () => void }) {
  return (
    <Button type="button" variant="outline" size="sm" onClick={onClick}>
      Add comparison variant
    </Button>
  );
}
