import { useMemo, useState, useEffect } from 'react';
import { Link, useNavigate, useSearchParams } from 'react-router-dom';
import { Badge } from '../../components/ui/badge';
import { Button } from '../../components/ui/button';
import { Card, CardHeader } from '../../components/ui/card';
import { Input } from '../../components/ui/input';
import {
  useExecutors,
  useHarnesses,
  useLaunchDiagnostic,
  useModels,
  useTasks,
} from '../../lib/hooks';

const DEFAULT_RUNS_PER_VARIANT = 5;
const MIN_RUNS_PER_VARIANT = 5;

export function DiagnosticLaunchPage() {
  const [searchParams] = useSearchParams();
  const navigate = useNavigate();
  const { data: tasks = [] } = useTasks();
  const { data: harnesses = [] } = useHarnesses();
  const { data: executors = [] } = useExecutors();
  const { data: models = [] } = useModels();
  const launch = useLaunchDiagnostic();

  const initialTask = searchParams.get('task') ?? '';
  const [taskID, setTaskID] = useState(initialTask);
  const [selectedHarnesses, setSelectedHarnesses] = useState<string[]>(['bare']);
  const [executorID, setExecutorID] = useState('');
  const [modelID, setModelID] = useState('');
  const [runsPerVariant, setRunsPerVariant] = useState(DEFAULT_RUNS_PER_VARIANT);
  const [name, setName] = useState('');

  useEffect(() => {
    if (!executorID && executors.length > 0) setExecutorID(executors[0].id);
  }, [executors, executorID]);
  useEffect(() => {
    if (!taskID && tasks.length > 0 && !initialTask) setTaskID(tasks[0].id);
  }, [tasks, taskID, initialTask]);

  // Filter the model dropdown to the providers each executor can actually
  // route to. Cursor only talks to its own cloud; Aider supports Ollama
  // locally AND OpenAI/Anthropic/Google when API keys are configured.
  // Unknown executors get the full list as a safety fallback.
  const allowedProviders = useMemo<string[]>(() => {
    switch (executorID) {
      case 'cursor':
        return ['cursor'];
      case 'aider':
        return ['ollama', 'openai', 'anthropic', 'google'];
      default:
        return [];
    }
  }, [executorID]);
  const filteredModels = useMemo(() => {
    if (models.length === 0 || allowedProviders.length === 0) return models;
    const preferred = models.filter((m) => allowedProviders.includes(m.provider));
    return preferred.length > 0 ? preferred : models;
  }, [models, allowedProviders]);
  useEffect(() => {
    if (filteredModels.length === 0) return;
    const stillValid = filteredModels.some((m) => m.model_id === modelID);
    if (!stillValid) setModelID(filteredModels[0].model_id);
  }, [filteredModels, modelID]);

  const toggleHarness = (id: string) => {
    setSelectedHarnesses((prev) =>
      prev.includes(id) ? prev.filter((h) => h !== id) : [...prev, id],
    );
  };

  const selectedTask = useMemo(() => tasks.find((t) => t.id === taskID), [tasks, taskID]);
  const canSubmit =
    Boolean(taskID) && Boolean(executorID) && selectedHarnesses.length > 0 && !launch.isPending;

  const handleLaunch = async () => {
    if (!canSubmit) return;
    try {
      const response = await launch.mutateAsync({
        task_id: taskID,
        executor_id: executorID,
        harness_ids: selectedHarnesses,
        model: modelID || undefined,
        runs_per_variant: runsPerVariant,
        name: name.trim() || undefined,
      });
      navigate(`/diagnostic/compare?experiment=${response.experiment_id}`);
    } catch {
      // mutation error renders inline below
    }
  };

  return (
    <div className="space-y-4">
      <Card>
        <CardHeader
          title="Launch diagnostic run"
          description="Pick a task, one or more harnesses, and an executor. AgentDx creates one variant per harness and starts the runs immediately."
        />
      </Card>

      <Card>
        <CardHeader title="1. Task" description="The agent receives this task's prompt." />
        {tasks.length === 0 ? (
          <div className="text-sm text-fg-muted">
            No tasks available. Add a task directory under <code className="font-mono">tasks/</code>
            then restart the engine.
          </div>
        ) : (
          <div className="grid gap-2">
            {tasks.map((task) => (
              <button
                key={task.id}
                type="button"
                onClick={() => setTaskID(task.id)}
                className={
                  taskID === task.id
                    ? 'rounded-lg border-2 border-fg bg-bg-elev-2 p-3 text-left'
                    : 'rounded-lg border border-border p-3 text-left hover:border-border-strong'
                }
              >
                <div className="flex items-start justify-between gap-2">
                  <div>
                    <div className="font-medium text-fg">{task.name}</div>
                    <div className="mt-0.5 line-clamp-1 text-xs text-fg-muted">{task.description}</div>
                  </div>
                  <div className="flex shrink-0 items-center gap-2 text-xs uppercase tracking-wider text-fg-muted">
                    <Badge tone="neutral">{task.category}</Badge>
                    <span>{task.codebase_type}</span>
                  </div>
                </div>
              </button>
            ))}
          </div>
        )}
      </Card>

      <Card>
        <CardHeader
          title="2. Harnesses"
          description="Each picked harness becomes one variant. Bare is the baseline; comparing >1 makes the diagnostic profile interesting."
        />
        {harnesses.length === 0 ? (
          <div className="text-sm text-fg-muted">Loading harnesses…</div>
        ) : (
          <div className="grid gap-2 md:grid-cols-2">
            {harnesses.map((h) => (
              <label
                key={h.id}
                className={
                  selectedHarnesses.includes(h.id)
                    ? 'flex cursor-pointer items-start gap-3 rounded-lg border-2 border-fg bg-bg-elev-2 p-3'
                    : 'flex cursor-pointer items-start gap-3 rounded-lg border border-border p-3 hover:border-border-strong'
                }
              >
                <input
                  type="checkbox"
                  checked={selectedHarnesses.includes(h.id)}
                  onChange={() => toggleHarness(h.id)}
                  className="mt-0.5"
                />
                <div>
                  <div className="font-medium text-fg">{h.name}</div>
                  <div className="mt-0.5 text-xs text-fg-muted">{h.description}</div>
                </div>
              </label>
            ))}
          </div>
        )}
        <div className="mt-2 text-xs text-fg-muted">
          {selectedHarnesses.length} harness(es) selected · this will create{' '}
          {selectedHarnesses.length} variant(s) × {runsPerVariant} runs ={' '}
          <strong>{selectedHarnesses.length * runsPerVariant}</strong> total runs.
        </div>
      </Card>

      <Card>
        <CardHeader
          title="3. Executor & model"
          description="Which agent CLI runs inside the sandbox, and which model it talks to."
        />
        <div className="grid gap-4 md:grid-cols-2">
          <div>
            <div className="mb-1 text-xs uppercase tracking-wider text-fg-muted">Executor</div>
            <div className="flex flex-wrap gap-2">
              {executors.map((e) => (
                <button
                  key={e.id}
                  type="button"
                  onClick={() => setExecutorID(e.id)}
                  className={
                    executorID === e.id
                      ? 'rounded-md bg-fg px-3 py-1.5 text-sm font-medium text-white'
                      : 'rounded-md border border-border bg-bg-elev-1 px-3 py-1.5 text-sm font-medium text-fg-muted hover:border-border-strong'
                  }
                >
                  {e.id}
                </button>
              ))}
            </div>
          </div>
          <div>
            <div className="mb-1 text-xs uppercase tracking-wider text-fg-muted">Model</div>
            <select
              value={modelID}
              onChange={(event) => setModelID(event.target.value)}
              className="w-full rounded-md border border-border bg-bg-elev-1 px-3 py-1.5 text-sm"
            >
              {filteredModels.map((m) => (
                <option key={m.id} value={m.model_id}>
                  {m.display_name}
                </option>
              ))}
            </select>
            <div className="mt-1 text-xs text-fg-muted">
              Showing models compatible with {executorID || 'the selected executor'}. Aider runs
              local Ollama models; Cursor uses its own cloud.
            </div>
          </div>
        </div>
      </Card>

      <Card>
        <CardHeader
          title="4. Sample size"
          description={`Minimum ${MIN_RUNS_PER_VARIANT} runs per variant for statistical power. Bigger = slower but cleaner profile.`}
        />
        <div className="flex flex-wrap items-center gap-3">
          <input
            type="range"
            min={MIN_RUNS_PER_VARIANT}
            max={30}
            value={runsPerVariant}
            onChange={(event) => setRunsPerVariant(Number(event.target.value))}
            className="w-56"
          />
          <span className="text-sm font-medium text-fg">{runsPerVariant} runs per variant</span>
        </div>
        <div className="mt-3">
          <div className="mb-1 text-xs uppercase tracking-wider text-fg-muted">
            Name (optional)
          </div>
          <Input
            value={name}
            onChange={(event) => setName(event.target.value)}
            placeholder={
              selectedTask ? `Diagnostic ${selectedTask.name} · auto-named` : 'Diagnostic auto-named'
            }
          />
        </div>
      </Card>

      <div className="flex items-center justify-between gap-3">
        <Link to="/" className="text-sm text-fg-muted hover:text-fg">
          ← back
        </Link>
        <div className="flex items-center gap-3">
          {launch.isError && (
            <div className="text-sm text-danger-fg">
              {launch.error instanceof Error ? launch.error.message : 'Launch failed'}
            </div>
          )}
          <Button onClick={handleLaunch} disabled={!canSubmit} size="lg">
            {launch.isPending ? 'Launching…' : 'Launch diagnostic run'}
          </Button>
        </div>
      </div>
    </div>
  );
}
