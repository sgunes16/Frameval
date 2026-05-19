import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { Button } from '../../components/ui/button';
import { Card, CardHeader } from '../../components/ui/card';
import { Input } from '../../components/ui/input';
import { api } from '../../lib/api';

export function NewTaskPage() {
  const navigate = useNavigate();
  const [name, setName] = useState('');
  const [description, setDescription] = useState('');
  const [prompt, setPrompt] = useState('');
  const [category, setCategory] = useState('brownfield');
  const [codebaseType, setCodebaseType] = useState('typescript');
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState('');

  async function handleSave() {
    setSaving(true);
    setError('');
    try {
      const created = await api.post<{ id: string }>('/tasks', {
        name,
        description,
        task_prompt: prompt,
        category,
        codebase_type: codebaseType,
        complexity_score: 5,
        test_cases: [],
      });
      navigate(`/tasks/${created.id}`);
    } catch (submitError) {
      setError(submitError instanceof Error ? submitError.message : 'Could not create task.');
    } finally {
      setSaving(false);
    }
  }

  return (
    <Card className="space-y-4">
      <CardHeader title="Create custom task" description="Define a new evaluation task with your own prompt and tests." />
      <div className="grid gap-3 sm:grid-cols-2">
        <Input value={name} onChange={(event) => setName(event.target.value)} placeholder="Task name" />
        <Input value={codebaseType} onChange={(event) => setCodebaseType(event.target.value)} placeholder="typescript, go, python..." />
      </div>
      <div className="grid gap-3 sm:grid-cols-2">
        <Input value={category} onChange={(event) => setCategory(event.target.value)} placeholder="brownfield, greenfield, bugfix..." />
      </div>
      <textarea
        className="min-h-24 w-full rounded-lg border border-border-strong bg-bg-elev-1 p-3 text-sm shadow-[0_1px_2px_rgba(15,23,42,0.04)]"
        value={description}
        onChange={(event) => setDescription(event.target.value)}
        placeholder="Short description shown on task cards"
      />
      <textarea
        className="min-h-40 w-full rounded-lg border border-border-strong bg-bg-elev-1 p-3 text-sm shadow-[0_1px_2px_rgba(15,23,42,0.04)] font-mono"
        value={prompt}
        onChange={(event) => setPrompt(event.target.value)}
        placeholder="Full instruction prompt handed to the agent"
      />
      {error && <div className="rounded-lg border border-danger/30 bg-danger/10 p-3 text-sm text-danger">{error}</div>}
      <div className="flex justify-end">
        <Button onClick={handleSave} disabled={saving || !name || !prompt}>
          {saving ? 'Saving...' : 'Save task'}
        </Button>
      </div>
    </Card>
  );
}
