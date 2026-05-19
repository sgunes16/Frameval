import { Link } from 'react-router-dom';
import type { Task } from '../../lib/types';
import { Badge } from '../ui/badge';
import { Card } from '../ui/card';

export function TaskCard({ task }: { task: Task }) {
  const complexity = Math.round((task.complexity_score ?? 0) * 10) / 10;
  return (
    <Link to={`/tasks/${task.id}`} className="block">
      <Card hoverable className="h-full space-y-3">
        <div className="flex items-start justify-between gap-2">
          <div>
            <div className="text-sm font-semibold text-fg">{task.name}</div>
            <div className="mt-0.5 text-xs uppercase tracking-wider text-fg-muted">
              {task.category} · {task.codebase_type}
            </div>
          </div>
          <Badge tone={task.workspace_mode === 'empty' ? 'info' : task.workspace_mode === 'git' ? 'success' : 'neutral'}>
            {task.workspace_mode ?? 'empty'}
          </Badge>
        </div>
        <p className="line-clamp-3 text-xs leading-5 text-fg-muted">{task.description || task.task_prompt}</p>
        {task.workspace_git_url && (
          <div className="truncate rounded-md bg-bg-elev-2 px-2 py-1 font-mono text-xs text-fg-muted">
            git: {task.workspace_git_url}
            {task.workspace_git_ref ? `@${task.workspace_git_ref}` : ''}
          </div>
        )}
        <div className="flex items-center justify-between border-t border-border pt-3 text-xs text-fg-muted">
          <span>{task.test_cases?.length ?? 0} checks</span>
          <span>Complexity {complexity}/10</span>
        </div>
      </Card>
    </Link>
  );
}
