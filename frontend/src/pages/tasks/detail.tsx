import { Link, useParams } from 'react-router-dom';
import { Badge } from '../../components/ui/badge';
import { Button } from '../../components/ui/button';
import { Card, CardHeader } from '../../components/ui/card';
import { useTask } from '../../lib/hooks';

export function TaskDetailPage() {
  const { id } = useParams();
  const { data: task } = useTask(id);
  if (!task) {
    return <div className="text-sm text-slate-500">Loading...</div>;
  }
  return (
    <div className="space-y-4">
      <Card>
        <div className="flex items-start justify-between gap-3">
          <div>
            <div className="text-xl font-semibold text-slate-900">{task.name}</div>
            <div className="mt-1 flex items-center gap-2 text-[11px] uppercase tracking-wider text-slate-500">
              <Badge tone="neutral">{task.category}</Badge>
              {task.external_source && <Badge tone="info">{task.external_source}</Badge>}
              <span>{task.codebase_type}</span>
              <span>·</span>
              <span>{task.workspace_mode ?? 'empty'}</span>
              <span>·</span>
              <span>Complexity {task.complexity_score}/10</span>
            </div>
          </div>
          <Link to="/experiments/new">
            <Button size="sm">Use in experiment</Button>
          </Link>
        </div>
        <p className="mt-4 whitespace-pre-wrap text-sm text-slate-600">{task.description}</p>
        {task.workspace_git_url && (
          <div className="mt-3 rounded-lg border border-slate-200 bg-slate-50 px-3 py-2 text-xs text-slate-600">
            <div className="font-medium text-slate-700">Workspace is cloned from</div>
            <code className="font-mono text-[11px]">
              {task.workspace_git_url}
              {task.workspace_git_ref ? `@${task.workspace_git_ref}` : ''}
            </code>
          </div>
        )}
      </Card>
      <Card>
        <CardHeader title="Prompt" description="This is handed to the agent as the main instruction." />
        <pre className="whitespace-pre-wrap rounded-lg bg-slate-50 p-4 text-sm leading-6 text-slate-800">{task.task_prompt}</pre>
      </Card>
      <Card>
        <CardHeader title={`Verification (${task.test_cases?.length ?? 0})`} description="Deterministic compile/syntax checks executed after the agent run." />
        <div className="space-y-2">
          {(task.test_cases ?? []).map((test) => (
            <div key={test.name} className="rounded-lg border border-slate-200 p-3 text-sm">
              <div className="flex items-center gap-2">
                <div className="font-medium text-slate-900">{test.name}</div>
                {test.visibility === 'hidden' && <Badge tone="warning">hidden</Badge>}
              </div>
              {test.test_command ? (
                <code className="mt-1 block rounded bg-slate-50 px-2 py-1 text-xs text-slate-700">{test.test_command}</code>
              ) : (
                <div className="mt-1 rounded bg-slate-50 px-2 py-1 text-xs text-slate-500">Hidden verification command is not exposed in the UI.</div>
              )}
            </div>
          ))}
          {(!task.test_cases || task.test_cases.length === 0) && (
            <div className="rounded-lg border border-dashed border-slate-200 p-4 text-xs text-slate-500">
              No explicit verification — grading will rely on process metrics and filesystem diff.
            </div>
          )}
        </div>
      </Card>
    </div>
  );
}
