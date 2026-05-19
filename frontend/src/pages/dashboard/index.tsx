import { Link } from 'react-router-dom';
import { Badge } from '../../components/ui/badge';
import { Button } from '../../components/ui/button';
import { Card, CardHeader } from '../../components/ui/card';
import { EmptyState } from '../../components/ui/empty-state';
import { useExperiments, useTasks } from '../../lib/hooks';
import { statusTone, statusLabel, formatTimeAgo } from '../../lib/utils';

export function DashboardPage() {
  const { data: experiments = [] } = useExperiments();
  const { data: tasks = [] } = useTasks();

  const running = experiments.filter((experiment) => experiment.status === 'running').length;
  const completed = experiments.filter((experiment) => experiment.status === 'completed').length;
  const recent = [...experiments]
    .sort((left, right) => (right.created_at ?? '').localeCompare(left.created_at ?? ''))
    .slice(0, 5);

  return (
    <div className="space-y-6">
      <Card className="flex flex-col gap-4 bg-fg text-bg sm:flex-row sm:items-center sm:justify-between">
        <div>
          <div className="text-xs uppercase tracking-wider text-bg/60">Context engineering evaluator</div>
          <div className="mt-1 text-xl font-semibold">Benchmark agent context, deterministically.</div>
          <div className="mt-1 text-sm text-bg/70">Spin up sandboxed runs and compare variants with reproducible metrics.</div>
        </div>
        <Link to="/experiments">
          <Button variant="secondary" size="lg" className="bg-bg text-fg hover:bg-bg-elev-2">
            View experiments
          </Button>
        </Link>
      </Card>

      <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
        <StatCard label="Experiments" value={experiments.length} hint={`${running} running · ${completed} completed`} />
        <StatCard label="Task library" value={tasks.length} hint="Greenfield, brownfield, bugfix" />
        <StatCard label="Sandbox runs" value={experiments.reduce((acc, exp) => acc + exp.runs_per_variant * (exp.variants?.length ?? 0), 0)} hint="Total configured" />
      </div>

      <Card>
        <CardHeader
          title="Recent experiments"
          description="Latest configurations you have queued or run."
          action={
            <Link to="/experiments">
              <Button variant="ghost" size="sm">
                View all
              </Button>
            </Link>
          }
        />
        {recent.length === 0 ? (
          <EmptyState
            title="No experiments yet"
            description="Create an experiment to benchmark how context artifacts affect agent behavior."
            action={
              <Link to="/experiments">
                <Button size="sm">View experiments</Button>
              </Link>
            }
          />
        ) : (
          <div className="overflow-hidden rounded-lg border border-border">
            <table className="min-w-full text-sm">
              <thead className="bg-bg-elev-2 text-xs uppercase tracking-wider text-fg-muted">
                <tr>
                  <th className="px-4 py-2 text-left font-medium">Name</th>
                  <th className="px-4 py-2 text-left font-medium">Status</th>
                  <th className="px-4 py-2 text-left font-medium">Agent</th>
                  <th className="px-4 py-2 text-left font-medium">Variants</th>
                  <th className="px-4 py-2 text-left font-medium">Created</th>
                  <th className="px-4 py-2 text-right font-medium">Actions</th>
                </tr>
              </thead>
              <tbody>
                {recent.map((experiment) => (
                  <tr key={experiment.id} className="border-t border-border bg-bg-elev-1 hover:bg-bg-elev-2/60">
                    <td className="px-4 py-2 font-medium text-fg">{experiment.name}</td>
                    <td className="px-4 py-2">
                      <Badge tone={statusTone(experiment.status)}>{statusLabel(experiment.status)}</Badge>
                    </td>
                    <td className="px-4 py-2 text-fg-muted">
                      {experiment.agent_cli} · {experiment.model}
                    </td>
                    <td className="px-4 py-2 text-fg-muted">{experiment.variants?.length ?? 0}</td>
                    <td className="px-4 py-2 text-fg-muted">{formatTimeAgo(experiment.created_at)}</td>
                    <td className="px-4 py-2 text-right">
                      <Link to={`/experiments/${experiment.id}/monitor`}>
                        <Button variant="ghost" size="sm">
                          Open
                        </Button>
                      </Link>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </Card>

      <div className="grid gap-4 lg:grid-cols-2">
        <Card>
          <CardHeader title="Quick start" description="Guided paths for common workflows." />
          <div className="space-y-2">
            <QuickLink to="/experiments" title="See active experiments" description="Monitor in-flight runs and review past comparisons." />
            <QuickLink to="/tasks" title="Browse task library" description="Ready-to-run prompts, tests, and workspace modes." />
            <QuickLink to="/settings" title="Configure models & agents" description="Set API keys and pick your executor." />
          </div>
        </Card>
        <Card>
          <CardHeader title="Coming soon" description="Planned features currently disabled." />
          <div className="space-y-2">
            <ComingSoon title="Baselines" description="Reference runs and regression tracking per task." />
            <ComingSoon title="Cross-model judge" description="Automatic LLM-as-Judge scoring with rubrics." />
            <ComingSoon title="Scheduled sweeps" description="Periodic reruns with drift detection." />
          </div>
        </Card>
      </div>
    </div>
  );
}

function StatCard({ label, value, hint, muted }: { label: string; value: number; hint?: string; muted?: boolean }) {
  return (
    <Card className={muted ? 'opacity-70' : ''}>
      <div className="text-xs font-medium uppercase tracking-wider text-fg-muted">{label}</div>
      <div className="mt-2 text-2xl font-semibold tracking-tight text-fg">{value}</div>
      {hint && <div className="mt-1 text-xs text-fg-muted">{hint}</div>}
    </Card>
  );
}

function QuickLink({ to, title, description }: { to: string; title: string; description: string }) {
  return (
    <Link
      to={to}
      className="flex items-center justify-between rounded-lg border border-border bg-bg-elev-1 px-4 py-3 text-sm transition hover:border-border-strong hover:bg-bg-elev-2"
    >
      <div>
        <div className="font-medium text-fg">{title}</div>
        <div className="text-xs text-fg-muted">{description}</div>
      </div>
      <span className="text-fg-subtle">&rarr;</span>
    </Link>
  );
}

function ComingSoon({ title, description }: { title: string; description: string }) {
  return (
    <div className="flex items-center justify-between rounded-lg border border-dashed border-border bg-bg-elev-2/60 px-4 py-3 text-sm text-fg-muted">
      <div>
        <div className="font-medium text-fg-muted">{title}</div>
        <div className="text-xs text-fg-muted">{description}</div>
      </div>
      <Badge tone="muted">Soon</Badge>
    </div>
  );
}
