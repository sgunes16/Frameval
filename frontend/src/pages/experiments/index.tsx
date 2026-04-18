import { useMemo, useState } from 'react';
import { Link } from 'react-router-dom';
import { Badge } from '../../components/ui/badge';
import { Button } from '../../components/ui/button';
import { Card } from '../../components/ui/card';
import { EmptyState } from '../../components/ui/empty-state';
import { Input } from '../../components/ui/input';
import { useExperiments } from '../../lib/hooks';
import { formatCurrency, formatTimeAgo, statusLabel, statusTone } from '../../lib/utils';

const STATUS_FILTERS = ['all', 'draft', 'running', 'completed', 'failed'] as const;

type StatusFilter = (typeof STATUS_FILTERS)[number];

export function ExperimentsPage() {
  const { data: experiments = [] } = useExperiments();
  const [query, setQuery] = useState('');
  const [status, setStatus] = useState<StatusFilter>('all');

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

  return (
    <div className="space-y-4">
      <Card>
        <div className="flex flex-wrap items-center justify-between gap-3">
          <div className="flex flex-wrap items-center gap-2">
            {STATUS_FILTERS.map((value) => (
              <button
                key={value}
                type="button"
                onClick={() => setStatus(value)}
                className={
                  status === value
                    ? 'rounded-full bg-slate-900 px-3 py-1 text-xs font-medium text-white'
                    : 'rounded-full border border-slate-200 bg-white px-3 py-1 text-xs font-medium text-slate-600 hover:border-slate-300'
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
            <Link to="/experiments/new">
              <Button size="sm">New experiment</Button>
            </Link>
          </div>
        </div>
      </Card>

      {filtered.length === 0 ? (
        <EmptyState
          title="No experiments match"
          description="Adjust your filters or create a new experiment."
          action={
            <Link to="/experiments/new">
              <Button size="sm">Create experiment</Button>
            </Link>
          }
        />
      ) : (
        <Card padded={false} className="overflow-hidden">
          <table className="min-w-full text-sm">
            <thead className="bg-slate-50 text-[11px] uppercase tracking-wider text-slate-500">
              <tr>
                <th className="px-4 py-2 text-left font-medium">Experiment</th>
                <th className="px-4 py-2 text-left font-medium">Status</th>
                <th className="px-4 py-2 text-left font-medium">Agent · Model</th>
                <th className="px-4 py-2 text-left font-medium">Variants</th>
                <th className="px-4 py-2 text-left font-medium">Runs / variant</th>
                <th className="px-4 py-2 text-left font-medium">Estimated cost</th>
                <th className="px-4 py-2 text-left font-medium">Created</th>
                <th className="px-4 py-2 text-right font-medium">Actions</th>
              </tr>
            </thead>
            <tbody>
              {filtered.map((experiment) => (
                <tr key={experiment.id} className="border-t border-slate-100 bg-white hover:bg-slate-50/60">
                  <td className="px-4 py-3">
                    <div className="font-medium text-slate-900">{experiment.name}</div>
                    {experiment.description && (
                      <div className="mt-0.5 line-clamp-1 text-xs text-slate-500">{experiment.description}</div>
                    )}
                  </td>
                  <td className="px-4 py-3">
                    <Badge tone={statusTone(experiment.status)}>{statusLabel(experiment.status)}</Badge>
                  </td>
                  <td className="px-4 py-3 text-slate-600">
                    {experiment.agent_cli}
                    <span className="text-slate-400"> · </span>
                    {experiment.model}
                  </td>
                  <td className="px-4 py-3 text-slate-600">{experiment.variants?.length ?? 0}</td>
                  <td className="px-4 py-3 text-slate-600">{experiment.runs_per_variant}</td>
                  <td className="px-4 py-3 text-slate-600">{formatCurrency(experiment.estimated_cost_usd)}</td>
                  <td className="px-4 py-3 text-slate-500">{formatTimeAgo(experiment.created_at)}</td>
                  <td className="px-4 py-3 text-right">
                    <div className="flex justify-end gap-2">
                      <Link to={`/experiments/${experiment.id}/monitor`}>
                        <Button variant="ghost" size="sm">
                          Monitor
                        </Button>
                      </Link>
                      <Link to={`/experiments/${experiment.id}/results`}>
                        <Button variant="outline" size="sm">
                          Results
                        </Button>
                      </Link>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </Card>
      )}
    </div>
  );
}
