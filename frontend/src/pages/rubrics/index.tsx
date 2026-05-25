import { useState } from 'react';
import { useDeleteRubric, useRubrics } from '../../lib/hooks';
import { Button } from '../../components/ui/button';
import { Card, CardHeader } from '../../components/ui/card';
import { ErrorState, LoadingSkeleton } from '../../components/system';
import { AddRubricDialog } from './AddRubricDialog';
import { RubricEditor } from './RubricEditor';

export function RubricsPage() {
  const { data: rubrics, isLoading, isError, refetch } = useRubrics();
  const del = useDeleteRubric();
  const [editingKey, setEditingKey] = useState<string | null>(null);
  const [showAdd, setShowAdd] = useState(false);

  if (isLoading) return <LoadingSkeleton variant="row" count={5} />;
  if (isError) return <ErrorState title="Could not load rubrics" onRetry={() => refetch()} />;
  const list = rubrics ?? [];
  const editing = editingKey ? list.find((r) => r.key === editingKey) ?? null : null;

  return (
    <div className="space-y-4">
      <Card>
        <CardHeader
          title="Rubrics"
          description="Per-dimension system prompts the LLM judge uses. Changes take effect on the next experiment."
          action={<Button onClick={() => setShowAdd(true)}>+ Add dimension</Button>}
        />
        <ul className="space-y-1">
          {list.map((r) => (
            <li
              key={r.key}
              className="flex items-center justify-between rounded-lg border border-border bg-bg-elev-1 px-3 py-2 text-sm"
            >
              <div className="flex-1">
                <div className="font-medium text-fg">{r.display_name}</div>
                <div className="font-mono text-xs text-fg-muted">
                  {r.key}
                  {r.is_builtin && ' (builtin)'}
                </div>
              </div>
              <div className="flex gap-1">
                <Button variant="ghost" onClick={() => setEditingKey(r.key)}>
                  Edit
                </Button>
                {!r.is_builtin && (
                  <Button
                    variant="ghost"
                    onClick={() => {
                      if (confirm(`Delete "${r.display_name}"?`)) del.mutate(r.key);
                    }}
                    disabled={del.isPending}
                  >
                    Delete
                  </Button>
                )}
              </div>
            </li>
          ))}
        </ul>
      </Card>

      {editing && <RubricEditor rubric={editing} onClose={() => setEditingKey(null)} />}
      {showAdd && <AddRubricDialog onClose={() => setShowAdd(false)} />}
    </div>
  );
}
