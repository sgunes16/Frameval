import { useMemo, useState, useEffect } from 'react';
import { Link, useNavigate, useSearchParams } from 'react-router-dom';
import { Badge } from '../../components/ui/badge';
import { Button } from '../../components/ui/button';
import { Card, CardHeader } from '../../components/ui/card';
import { Input } from '../../components/ui/input';
import { cn } from '../../lib/utils';
import {
  useExecutors,
  useHarnesses,
  useLaunchDiagnostic,
  useModels,
  useTasks,
} from '../../lib/hooks';
import type { ExecutorInfo, ModelConfig } from '../../lib/types';

const DEFAULT_RUNS_PER_VARIANT = 5;
const MIN_RUNS_PER_VARIANT = 1;

function allowedProvidersFor(executorId: string): string[] {
  switch (executorId) {
    case 'cursor':
      return ['cursor'];
    case 'aider':
      return ['ollama', 'openai', 'anthropic', 'google'];
    case 'opencode':
      return ['opencode', 'ollama'];
    default:
      return [];
  }
}

interface Variant {
  harness: string;
  executor: string;
  model: string;
  modelDisplay: string;
}

/**
 * DiagnosticLaunchPage — matrix builder for cross-cutting agent runs.
 *
 * The form is a 12-column grid: tasks list on the left (≤ 5 cols),
 * the harness × executor × model matrix on the right (≥ 7 cols), and
 * a sticky action bar at the bottom with the live variant count and
 * the launch button. Every section is intentionally one-card-deep so
 * the page reads top-to-bottom on a small laptop without horizontal
 * scroll, but density is high enough that the whole experiment fits
 * above the fold on a 1440px display.
 */
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
  const [selectedExecutors, setSelectedExecutors] = useState<string[]>([]);
  const [selectedModels, setSelectedModels] = useState<string[]>([]);
  const [runsPerVariant, setRunsPerVariant] = useState(DEFAULT_RUNS_PER_VARIANT);
  const [name, setName] = useState('');
  const [partialError, setPartialError] = useState<string | null>(null);

  useEffect(() => {
    if (selectedExecutors.length === 0 && executors.length > 0) {
      setSelectedExecutors([executors[0].id]);
    }
  }, [executors, selectedExecutors.length]);
  useEffect(() => {
    if (!taskID && tasks.length > 0 && !initialTask) setTaskID(tasks[0].id);
  }, [tasks, taskID, initialTask]);

  const visibleModels = useMemo<ModelConfig[]>(() => {
    if (selectedExecutors.length === 0) return [];
    const allowed = new Set<string>();
    for (const eid of selectedExecutors) {
      for (const p of allowedProvidersFor(eid)) allowed.add(p);
    }
    if (allowed.size === 0) return models;
    return models.filter((m) => allowed.has(m.provider));
  }, [models, selectedExecutors]);

  useEffect(() => {
    setSelectedModels((prev) =>
      prev.filter((mid) => visibleModels.some((m) => m.model_id === mid)),
    );
  }, [visibleModels]);

  const variants = useMemo<Variant[]>(() => {
    const out: Variant[] = [];
    for (const h of selectedHarnesses) {
      for (const eid of selectedExecutors) {
        const allowed = new Set(allowedProvidersFor(eid));
        for (const mid of selectedModels) {
          const model = models.find((m) => m.model_id === mid);
          if (!model) continue;
          if (allowed.size > 0 && !allowed.has(model.provider)) continue;
          out.push({
            harness: h,
            executor: eid,
            model: mid,
            modelDisplay: model.display_name,
          });
        }
      }
    }
    return out;
  }, [selectedHarnesses, selectedExecutors, selectedModels, models]);

  const totalRuns = variants.length * runsPerVariant;

  const toggleHarness = (id: string) =>
    setSelectedHarnesses((prev) =>
      prev.includes(id) ? prev.filter((h) => h !== id) : [...prev, id],
    );
  const toggleExecutor = (id: string) =>
    setSelectedExecutors((prev) =>
      prev.includes(id) ? prev.filter((e) => e !== id) : [...prev, id],
    );
  const toggleModel = (id: string) =>
    setSelectedModels((prev) =>
      prev.includes(id) ? prev.filter((m) => m !== id) : [...prev, id],
    );

  const selectedTask = useMemo(() => tasks.find((t) => t.id === taskID), [tasks, taskID]);
  const canSubmit = Boolean(taskID) && variants.length > 0 && !launch.isPending;

  const handleLaunch = async () => {
    if (!canSubmit) return;
    setPartialError(null);
    const baseName = name.trim();
    const results = await Promise.allSettled(
      variants.map((v, i) => {
        const variantName = baseName
          ? `${baseName} · ${v.harness}/${v.executor}/${v.modelDisplay}`
          : '';
        return launch.mutateAsync({
          task_id: taskID,
          executor_id: v.executor,
          harness_ids: [v.harness],
          model: v.model,
          runs_per_variant: runsPerVariant,
          name: variantName || `Variant ${i + 1}`,
        });
      }),
    );
    const ok: string[] = [];
    const errs: string[] = [];
    results.forEach((r, i) => {
      if (r.status === 'fulfilled') ok.push(r.value.experiment_id);
      else {
        const v = variants[i];
        errs.push(
          `${v.harness}/${v.executor}/${v.modelDisplay}: ${
            r.reason instanceof Error ? r.reason.message : 'unknown'
          }`,
        );
      }
    });
    if (ok.length === 0) {
      setPartialError(errs.join(' · ') || 'All variants failed to launch.');
      return;
    }
    if (errs.length > 0) {
      setPartialError(`${errs.length} of ${variants.length} failed: ${errs.join(' · ')}`);
    }
    if (ok.length === 1) navigate(`/experiments/${ok[0]}/monitor`);
    else navigate(`/diagnostic/compare?experiments=${ok.join(',')}`);
  };

  return (
    <div className="space-y-4 pb-24">
      {/* Top intro strip — one line, no padding bloat. */}
      <div className="flex items-baseline justify-between gap-3">
        <div>
          <h1 className="text-lg font-semibold text-fg">Launch diagnostic run</h1>
          <p className="text-xs text-fg-muted">
            Pick a task, build a matrix: every (harness × executor × model) becomes one variant.
          </p>
        </div>
        <Link to="/" className="text-xs text-fg-muted hover:text-fg">
          ← back
        </Link>
      </div>

      {/* Two-column body: Task on the left (≤5 col), matrix on the
          right (≥7 col). Matrix is the focal point, so it gets the
          larger column on every breakpoint. */}
      <div className="grid gap-4 lg:grid-cols-12">
        <Card className="lg:col-span-5">
          <CompactHeader title="Task" hint="agent receives this prompt" />
          {tasks.length === 0 ? (
            <div className="text-xs text-fg-muted">
              No tasks. Add a directory under <code className="font-mono">tasks/</code> and restart.
            </div>
          ) : (
            <ul className="space-y-1.5">
              {tasks.map((task) => (
                <li key={task.id}>
                  <button
                    type="button"
                    onClick={() => setTaskID(task.id)}
                    className={cn(
                      'w-full rounded-md px-2.5 py-2 text-left text-xs transition',
                      taskID === task.id
                        ? 'border border-fg bg-bg-elev-2'
                        : 'border border-border hover:border-border-strong hover:bg-bg-elev-1/50',
                    )}
                  >
                    <div className="flex items-baseline justify-between gap-2">
                      <span className="truncate font-medium text-fg">{task.name}</span>
                      <span className="flex shrink-0 items-center gap-1.5 text-[10px] uppercase tracking-wider text-fg-subtle">
                        <Badge tone="neutral">{task.category}</Badge>
                        <span>{task.codebase_type}</span>
                      </span>
                    </div>
                    <div className="mt-0.5 line-clamp-1 text-[11px] text-fg-muted">
                      {task.description}
                    </div>
                  </button>
                </li>
              ))}
            </ul>
          )}
        </Card>

        <Card className="lg:col-span-7">
          <CompactHeader
            title="Variant matrix"
            hint="cartesian product · incompatible pairs auto-filtered"
          />
          <div className="space-y-3">
            <MatrixGroup label="Harnesses">
              {harnesses.map((h) => (
                <Chip
                  key={h.id}
                  label={h.id}
                  title={h.description}
                  checked={selectedHarnesses.includes(h.id)}
                  onToggle={() => toggleHarness(h.id)}
                />
              ))}
            </MatrixGroup>
            <MatrixGroup label="Executors">
              {executors.map((e) => (
                <Chip
                  key={e.id}
                  label={e.id}
                  title={describeExecutor(e)}
                  checked={selectedExecutors.includes(e.id)}
                  onToggle={() => toggleExecutor(e.id)}
                />
              ))}
            </MatrixGroup>
            <MatrixGroup
              label="Models"
              hint={
                selectedExecutors.length === 0
                  ? 'pick an executor to filter'
                  : `${visibleModels.length} available`
              }
            >
              {visibleModels.length === 0 ? (
                <span className="text-[11px] text-fg-subtle">
                  {selectedExecutors.length === 0
                    ? 'no executors selected'
                    : 'no compatible models'}
                </span>
              ) : (
                visibleModels.map((m) => (
                  <Chip
                    key={m.model_id}
                    label={m.display_name}
                    title={m.model_id}
                    checked={selectedModels.includes(m.model_id)}
                    onToggle={() => toggleModel(m.model_id)}
                  />
                ))
              )}
            </MatrixGroup>
          </div>
        </Card>
      </div>

      {/* Preview + sample size, side-by-side. Variant preview lives
          here (not under the matrix) so the user sees the live cost
          of the matrix while tuning sample size — direct feedback
          loop between "what I picked" and "what it'll cost". */}
      <div className="grid gap-4 lg:grid-cols-12">
        <Card className="lg:col-span-7">
          <CompactHeader
            title="Preview"
            hint={`${variants.length} variant${variants.length === 1 ? '' : 's'} · ${totalRuns} total run${totalRuns === 1 ? '' : 's'}`}
          />
          <VariantPreview variants={variants} />
        </Card>
        <Card className="lg:col-span-5">
          <CompactHeader title="Sample size" hint={`${runsPerVariant} runs / variant`} />
          <input
            type="range"
            min={MIN_RUNS_PER_VARIANT}
            max={30}
            value={runsPerVariant}
            onChange={(event) => setRunsPerVariant(Number(event.target.value))}
            className="w-full accent-accent"
          />
          <div className="mt-1 flex justify-between text-[10px] uppercase tracking-wider text-fg-subtle">
            <span>1 — smoke</span>
            <span>5 — recommended</span>
            <span>30 — saturated</span>
          </div>
          <div className="mt-3">
            <div className="mb-1 text-[10px] uppercase tracking-wider text-fg-muted">
              Name prefix (optional)
            </div>
            <Input
              value={name}
              onChange={(event) => setName(event.target.value)}
              placeholder={
                selectedTask ? `${selectedTask.name} matrix` : 'Matrix run · auto-named'
              }
            />
            <div className="mt-1 text-[10.5px] text-fg-subtle">
              Each variant →{' '}
              <code className="font-mono">{`<prefix> · <harness>/<executor>/<model>`}</code>
            </div>
          </div>
        </Card>
      </div>

      {/* Sticky action bar — always visible while scrolling the form
          so the operator can see how many runs they're about to start
          and hit Launch without scrolling back to the top. */}
      <div className="fixed inset-x-0 bottom-0 z-30 border-t border-border bg-bg-elev-1/95 backdrop-blur supports-[backdrop-filter]:bg-bg-elev-1/80">
        <div className="mx-auto flex max-w-6xl items-center justify-between gap-4 px-6 py-3">
          <div className="flex items-baseline gap-4 text-xs text-fg-muted">
            <span>
              <span className="font-mono text-sm font-semibold text-fg">{variants.length}</span>{' '}
              variant{variants.length === 1 ? '' : 's'}
            </span>
            <span className="text-fg-subtle">·</span>
            <span>
              <span className="font-mono text-sm font-semibold text-fg">{totalRuns}</span>{' '}
              total run{totalRuns === 1 ? '' : 's'}
            </span>
            {variants.length > 8 && (
              <span className="text-warning-fg">⚠ large matrix — may queue for a while</span>
            )}
          </div>
          <div className="flex items-center gap-3">
            {(launch.isError || partialError) && (
              <div className="max-w-md truncate text-right text-xs text-danger-fg">
                {partialError ??
                  (launch.error instanceof Error ? launch.error.message : 'Launch failed')}
              </div>
            )}
            <Button onClick={handleLaunch} disabled={!canSubmit} size="lg">
              {launch.isPending
                ? `Launching ${variants.length}…`
                : variants.length === 0
                ? 'Pick a variant'
                : variants.length === 1
                ? 'Launch 1 variant'
                : `Launch ${variants.length} variants`}
            </Button>
          </div>
        </div>
      </div>
    </div>
  );
}

function describeExecutor(e: ExecutorInfo): string {
  switch (e.id) {
    case 'opencode':
      return 'sst/opencode — bundled cloud free + Ollama';
    case 'aider':
      return 'aider — Ollama / OpenAI / Anthropic / Google';
    case 'cursor':
      return 'Cursor cloud agent';
    default:
      return `Modes: ${e.modes.join(', ')}`;
  }
}

function CompactHeader({ title, hint }: { title: string; hint?: string }) {
  return (
    <div className="mb-3 flex items-baseline justify-between gap-2">
      <h2 className="text-xs font-semibold uppercase tracking-wider text-fg">{title}</h2>
      {hint && <span className="text-[10px] text-fg-subtle">{hint}</span>}
    </div>
  );
}

function MatrixGroup({
  label,
  hint,
  children,
}: {
  label: string;
  hint?: string;
  children: React.ReactNode;
}) {
  return (
    <div>
      <div className="mb-1.5 flex items-baseline justify-between gap-2">
        <span className="text-[10px] font-medium uppercase tracking-wider text-fg-muted">
          {label}
        </span>
        {hint && <span className="text-[10px] text-fg-subtle">{hint}</span>}
      </div>
      <div className="flex flex-wrap gap-1.5">{children}</div>
    </div>
  );
}

interface ChipProps {
  label: string;
  title?: string;
  checked: boolean;
  onToggle: () => void;
}

function Chip({ label, title, checked, onToggle }: ChipProps) {
  return (
    <button
      type="button"
      onClick={onToggle}
      title={title}
      aria-pressed={checked}
      className={cn(
        'inline-flex items-center gap-1.5 rounded-md border px-2.5 py-1 text-xs transition',
        checked
          ? 'border-fg bg-fg text-bg'
          : 'border-border bg-bg-elev-1 text-fg-muted hover:border-border-strong hover:text-fg',
      )}
    >
      <span
        aria-hidden
        className={cn(
          'flex h-3 w-3 items-center justify-center rounded-sm border',
          checked ? 'border-bg bg-bg text-fg' : 'border-border bg-bg-elev-2',
        )}
      >
        {checked && <span className="text-[10px] font-bold leading-none">✓</span>}
      </span>
      <span className="truncate">{label}</span>
    </button>
  );
}

function VariantPreview({ variants }: { variants: Variant[] }) {
  if (variants.length === 0) {
    return (
      <div className="rounded-md border border-dashed border-border bg-bg-elev-1 px-3 py-3 text-center text-[11px] text-fg-muted">
        No valid variants yet. Tick at least one harness, executor, and model.
      </div>
    );
  }
  return (
    <ul className="max-h-56 space-y-0.5 overflow-auto rounded-md border border-border bg-bg-elev-1 px-2 py-1.5 font-mono text-[11px]">
      {variants.map((v, i) => (
        <li
          key={`${v.harness}-${v.executor}-${v.model}-${i}`}
          className="flex items-baseline gap-2 px-1 py-0.5 leading-5"
        >
          <span className="w-6 select-none text-right text-fg-subtle">
            {String(i + 1).padStart(2, '0')}
          </span>
          <span className="text-fg">{v.harness}</span>
          <span className="text-fg-subtle">·</span>
          <span className="text-fg">{v.executor}</span>
          <span className="text-fg-subtle">·</span>
          <span className="truncate text-fg-muted">{v.modelDisplay}</span>
        </li>
      ))}
    </ul>
  );
}
