import { useMemo, useState } from 'react';
import { Link } from 'react-router-dom';
import { TaskCard } from '../../components/tasks/task-card';
import { Button } from '../../components/ui/button';
import { Card } from '../../components/ui/card';
import { EmptyState } from '../../components/ui/empty-state';
import { Input } from '../../components/ui/input';
import { useTasks } from '../../lib/hooks';

export function TasksPage() {
  const { data: tasks = [] } = useTasks();
  const [category, setCategory] = useState('all');
  const [query, setQuery] = useState('');

  const categories = useMemo(() => {
    const unique = new Set<string>();
    tasks.forEach((task) => task.category && unique.add(task.category));
    return ['all', ...Array.from(unique).sort()];
  }, [tasks]);

  const filteredTasks = useMemo(() => {
    const normalized = query.trim().toLowerCase();
    return tasks
      .filter((task) => (category === 'all' ? true : task.category === category))
      .filter((task) =>
        !normalized
          ? true
          : [task.name, task.description, task.task_prompt, task.codebase_type]
              .join(' ')
              .toLowerCase()
              .includes(normalized),
      );
  }, [category, query, tasks]);

  return (
    <div className="space-y-4">
      <Card>
        <div className="flex flex-wrap items-center justify-between gap-3">
          <div className="flex flex-wrap items-center gap-2">
            {categories.map((value) => (
              <button
                key={value}
                type="button"
                onClick={() => setCategory(value)}
                className={
                  category === value
                    ? 'rounded-full bg-fg px-3 py-1 text-xs font-medium capitalize text-bg'
                    : 'rounded-full border border-border bg-bg-elev-1 px-3 py-1 text-xs font-medium capitalize text-fg-muted hover:border-border-strong'
                }
              >
                {value}
              </button>
            ))}
          </div>
          <div className="flex items-center gap-2">
            <Input
              value={query}
              onChange={(event) => setQuery(event.target.value)}
              placeholder="Search tasks..."
              className="w-56"
            />
            <Link to="/tasks/new">
              <Button size="sm" variant="outline">
                New task
              </Button>
            </Link>
          </div>
        </div>
      </Card>
      {filteredTasks.length === 0 ? (
        <EmptyState
          title="No tasks match"
          description="Try a different category or search query, or add a new task."
        />
      ) : (
        <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-3">
          {filteredTasks.map((task) => (
            <TaskCard key={task.id} task={task} />
          ))}
        </div>
      )}
    </div>
  );
}
