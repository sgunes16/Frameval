import { useMemo } from 'react';
import type { Task } from '../../lib/types';
import { Badge } from '../ui/badge';

export function StepTask({
  tasks,
  selectedTaskId,
  selectedCategory,
  onCategoryChange,
  onChange,
}: {
  tasks: Task[];
  selectedTaskId: string;
  selectedCategory: string;
  onCategoryChange: (category: string) => void;
  onChange: (taskId: string) => void;
}) {
  const categories = useMemo(() => {
    const unique = new Set<string>();
    tasks.forEach((task) => task.category && unique.add(task.category));
    return ['', ...Array.from(unique).sort()];
  }, [tasks]);

  const filteredTasks = tasks.filter((task) => (selectedCategory ? task.category === selectedCategory : true));

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap gap-2">
        {categories.map((category) => {
          const active = category === selectedCategory;
          return (
            <button
              key={category || 'all'}
              type="button"
              onClick={() => onCategoryChange(category)}
              className={
                active
                  ? 'rounded-full bg-slate-900 px-3 py-1 text-xs font-medium capitalize text-white'
                  : 'rounded-full border border-slate-200 bg-white px-3 py-1 text-xs font-medium capitalize text-slate-600 hover:border-slate-300'
              }
            >
              {category || 'All'}
            </button>
          );
        })}
      </div>
      <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-3">
        {filteredTasks.map((task) => {
          const selected = task.id === selectedTaskId;
          return (
            <button
              key={task.id}
              type="button"
              onClick={() => onChange(task.id)}
              className={
                selected
                  ? 'rounded-lg border border-slate-900 bg-slate-900/5 p-3 text-left transition'
                  : 'rounded-lg border border-slate-200 bg-white p-3 text-left transition hover:border-slate-300'
              }
            >
              <div className="flex items-start justify-between gap-2">
                <div className="text-sm font-semibold text-slate-900">{task.name}</div>
                <Badge tone="neutral">{task.category}</Badge>
              </div>
              <div className="mt-1 text-[11px] uppercase tracking-wider text-slate-500">
                {task.codebase_type} · {task.workspace_mode ?? 'task_codebase'}
              </div>
              <p className="mt-2 line-clamp-3 text-xs leading-5 text-slate-500">
                {task.description || task.task_prompt}
              </p>
              <div className="mt-2 flex items-center justify-between text-[11px] text-slate-500">
                <span>{task.test_cases?.length ?? 0} tests</span>
                <span>Complexity {task.complexity_score}/10</span>
              </div>
            </button>
          );
        })}
      </div>
    </div>
  );
}
