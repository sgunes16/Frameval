import { useEffect, useMemo, useState } from 'react';
import { Link, useSearchParams } from 'react-router-dom';
import { Badge } from '../../components/ui/badge';
import { Button } from '../../components/ui/button';
import { Card } from '../../components/ui/card';
import { EmptyState } from '../../components/ui/empty-state';
import { Input } from '../../components/ui/input';
import { useExperiments } from '../../lib/hooks';
import type { Experiment } from '../../lib/types';
import { formatCurrency, formatTimeAgo, statusLabel, statusTone } from '../../lib/utils';
import { groupByBatch, type GroupedExperiment } from './grouping';

const STATUS_FILTERS = ['all', 'draft', 'running', 'completed', 'failed'] as const;

type StatusFilter = (typeof STATUS_FILTERS)[number];

export function ExperimentsPage() {
  const { data: experiments = [] } = useExperiments();
  const [query, setQuery] = useState('');
  const [status, setStatus] = useState<StatusFilter>('all');
  const [searchParams] = useSearchParams();
  const focusBatch = searchParams.get('batch') ?? '';
  const [expandedBatches, setExpandedBatches] = useState<Record<string, boolean>>({});

  const filtered = useMemo(() => {
    const normalized = query.trim().toLowerCase();
    return experiments
      .filter((experiment) => (status === 'all' ? true : experiment.status === status))
      .filter((experiment) =>
        !normalized
          ? true
          : [experiment.name, experiment.description, experiment.model, experiment.agent_cli]
              .join(' ')
              .toLowerCase()
              .includes(normalized),
      )
      .sort((left, right) => (right.created_at ?? '').localeCompare(left.created_at ?? ''));
  }, [experiments, query, status]);

  const grouped = useMemo(() => groupByBatch(filtered), [filtered]);

  useEffect(() => {
    setExpandedBatches((prev) => {
      const next = { ...prev };
      for (const unit of grouped) {
        if (unit.kind !== 'group') continue;
        if (next[unit.batchId] !== undefined) continue;
        // Default: expanded for small groups, collapsed for large ones.
        // The URL-focused batch always defaults to expanded.
        next[unit.batchId] = unit.experiments.length <= 3 || unit.batchId === focusBatch;
      }
      return next;
    });
  }, [grouped, focusBatch]);

  useEffect(() => {
    if (!focusBatch) return;
    const el = document.querySelector(`[data-batch-id="${focusBatch}"]`);
    if (el && 'scrollIntoView' in el) {
      el.scrollIntoView({ behavior: 'smooth', block: 'start' });
    }
  }, [focusBatch]);

  return (
    <div className="space-y-4">
      <SummaryChips experiments={experiments} />
      <Card className="sticky top-0 z-10">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <div className="flex flex-wrap items-center gap-2">
            {STATUS_FILTERS.map((value) => (
              <button
                key={value}
                type="button"
                onClick={() => setStatus(value)}
                className={
                  status === value
                    ? 'rounded-full bg-fg px-3 py-1 text-xs font-medium text-bg'
                    : 'rounded-full border border-border bg-bg-elev-1 px-3 py-1 text-xs font-medium text-fg-muted hover:border-border-strong'
                }
              >
                {value === 'all' ? 'All' : statusLabel(value)}
              </button>
            ))}
          </div>
          <div className="flex items-center gap-2">
            <Input
              value={query}
              onChange={(event) => setQuery(event.target.value)}
              placeholder="Search by name, model, agent..."
              className="w-64"
            />
          </div>
        </div>
      </Card>

      {filtered.length === 0 ? (
        <EmptyState
          title={experiments.length === 0 ? 'No experiments yet' : 'No experiments match'}
          description={
            experiments.length === 0
              ? 'Start a diagnostic run to compare harnesses on a task.'
              : 'Adjust your filters or clear the search to see all experiments.'
          }
          action={
            <Link to="/diagnostic/launch">
              <Button size="sm">New diagnostic run</Button>
            </Link>
          }
        />
      ) : (
        <Card padded={false} className="overflow-hidden">
          <table className="min-w-full text-sm">
            <thead className="bg-bg-elev-2 text-xs uppercase tracking-wider text-fg-muted">
              <tr>
                <th className="px-4 py-2 text-left font-medium">Experiment</th>
                <th className="px-4 py-2 text-left font-medium">Status</th>
                <th className="px-4 py-2 text-left font-medium">Agent · Model</th>
                <th className="px-4 py-2 text-left font-medium">Runs</th>
                <th className="px-4 py-2 text-right font-medium">Created</th>
              </tr>
            </thead>
            {grouped.map((unit) => {
              if (unit.kind === 'solo') {
                return (
                  <tbody key={unit.experiment.id}>
                    <ExperimentRow experiment={unit.experiment} />
                  </tbody>
                );
              }
              const expanded = expandedBatches[unit.batchId] ?? false;
              const counts = countByStatus(unit.experiments);
              return (
                <GroupBlock
                  key={unit.batchId}
                  unit={unit}
                  expanded={expanded}
                  counts={counts}
                  onToggle={() =>
                    setExpandedBatches((prev) => ({ ...prev, [unit.batchId]: !expanded }))
                  }
                />
              );
            })}
          </table>
        </Card>
      )}
    </div>
  );
}

function SummaryChips({ experiments }: { experiments: ReturnType<typeof useExperiments>['data'] }) {
  const list = experiments ?? [];
  const total = list.length;
  const running = list.filter((e) => e.status === 'running').length;
  const queued = list.filter((e) => e.status === 'draft' || e.status === 'queued').length;
  const completed = list.filter((e) => e.status === 'completed').length;

  return (
    <div className="flex flex-wrap items-center gap-2 px-1 text-xs text-fg-muted">
      <span className="font-medium text-fg">{total} experiments</span>
      <span className="text-fg-subtle">·</span>
      <span>{running} running</span>
      {queued > 0 && (
        <>
          <span className="text-fg-subtle">·</span>
          <span>{queued} queued</span>
        </>
      )}
      <span className="text-fg-subtle">·</span>
      <span>{completed} completed</span>
    </div>
  );
}

function runsLabel(experiment: Experiment): string {
  const variantCount = experiment.variants?.length ?? 0;
  if (variantCount === 0) {
    return `${experiment.runs_per_variant} run${experiment.runs_per_variant === 1 ? '' : 's'}`;
  }
  const harnessNames = (experiment.variants ?? []).map((v) => v.harness_id ?? v.name).join(', ');
  return `${variantCount}v × ${experiment.runs_per_variant}r${harnessNames ? ` · ${harnessNames}` : ''}`;
}

function countByStatus(experiments: Experiment[]): Record<string, number> {
  const out: Record<string, number> = {};
  for (const e of experiments) {
    out[e.status] = (out[e.status] ?? 0) + 1;
  }
  return out;
}

function statusSummary(counts: Record<string, number>, total: number): string {
  const completed = counts['completed'] ?? 0;
  if (completed === total) return `${total}/${total} completed`;
  const parts: string[] = [];
  if (counts['running']) parts.push(`${counts['running']} running`);
  if ((counts['draft'] ?? 0) + (counts['queued'] ?? 0)) {
    parts.push(`${(counts['draft'] ?? 0) + (counts['queued'] ?? 0)} queued`);
  }
  if (completed) parts.push(`${completed} completed`);
  if (counts['failed']) parts.push(`${counts['failed']} failed`);
  return parts.length ? parts.join(' · ') : `${total} experiments`;
}

function ExperimentRow({ experiment, nested = false }: { experiment: Experiment; nested?: boolean }) {
  const cost = experiment.estimated_cost_usd ?? 0;
  return (
    <tr className={`border-t border-border ${nested ? 'bg-bg-elev-1/60' : 'bg-bg-elev-1'} hover:bg-bg-elev-2/60`}>
      <td
        className={
          `px-4 py-3 ${
            nested
              ? 'border-l-4 border-l-fg-muted/40 pl-6'
              : ''
          }`
        }
      >
        <div className="font-medium text-fg">{experiment.name}</div>
        {experiment.description && (
          <div className="mt-0.5 line-clamp-1 text-xs text-fg-muted">{experiment.description}</div>
        )}
      </td>
      <td className="px-4 py-3">
        <Badge tone={statusTone(experiment.status)}>{statusLabel(experiment.status)}</Badge>
      </td>
      <td className="px-4 py-3 text-fg-muted">
        {experiment.agent_cli}
        <span className="text-fg-subtle"> · </span>
        {experiment.model}
      </td>
      <td className="px-4 py-3">
        <div className="text-fg">{runsLabel(experiment)}</div>
        {cost > 0 && <div className="text-xs text-fg-subtle">~{formatCurrency(cost)}</div>}
      </td>
      <td className="px-4 py-3 text-right text-fg-muted">
        <div className="flex items-center justify-end gap-3">
          <span>{formatTimeAgo(experiment.created_at)}</span>
          <Link to={`/experiments/${experiment.id}/monitor`} className="text-fg-muted hover:text-fg">
            Open →
          </Link>
        </div>
      </td>
    </tr>
  );
}

function GroupBlock({
  unit,
  expanded,
  counts,
  onToggle,
}: {
  unit: Extract<GroupedExperiment, { kind: 'group' }>;
  expanded: boolean;
  counts: Record<string, number>;
  onToggle: () => void;
}) {
  const summary = statusSummary(counts, unit.experiments.length);
  const newest = unit.experiments[0];
  // One <tbody> per group so the browser renders natural vertical
  // separation between adjacent units. Combined with the strong top
  // border on the group header + a left accent on each child row, a
  // solo experiment that happens to sit between two groups can no
  // longer read as part of either of them.
  return (
    <tbody className="border-t-2 border-border-strong">
      <tr
        className="bg-bg-elev-2 hover:bg-bg-elev-2/80"
        data-batch-id={unit.batchId}
      >
        <td colSpan={5} className="px-4 py-2">
          <button
            type="button"
            onClick={onToggle}
            className="flex w-full items-center justify-between text-left"
          >
            <div className="flex items-center gap-2">
              <span className={`text-fg-muted transition ${expanded ? 'rotate-90' : ''}`}>▶</span>
              <span className="font-medium text-fg">{unit.batchLabel}</span>
              <span className="text-xs text-fg-subtle">·</span>
              <span className="text-xs text-fg-muted">{unit.experiments.length} experiments</span>
              <span className="text-xs text-fg-subtle">·</span>
              <span className="text-xs text-fg-muted">{summary}</span>
            </div>
            <span className="text-xs text-fg-subtle">{formatTimeAgo(newest.created_at)}</span>
          </button>
        </td>
      </tr>
      {expanded &&
        unit.experiments.map((experiment) => (
          <ExperimentRow key={experiment.id} experiment={experiment} nested />
        ))}
    </tbody>
  );
}
