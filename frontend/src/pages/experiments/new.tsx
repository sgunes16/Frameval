import { useEffect, useMemo, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { StepAgent } from '../../components/experiment-wizard/step-agent';
import { AddComparisonVariantButton, StepContextSource, type DraftVariantContext } from '../../components/experiment-wizard/step-context-source';
import { StepParams } from '../../components/experiment-wizard/step-params';
import { StepTask } from '../../components/experiment-wizard/step-task';
import { StepWorkspaceSource } from '../../components/experiment-wizard/step-workspace-source';
import { Badge } from '../../components/ui/badge';
import { Button } from '../../components/ui/button';
import { Card, CardHeader } from '../../components/ui/card';
import { Input } from '../../components/ui/input';
import {
  useAgents,
  useCatalogExtensions,
  useCreateArtifact,
  useCreateExperiment,
  useDockerStatus,
  useImportCatalogExtensions,
  useModels,
  useStartExperiment,
  useTasks,
} from '../../lib/hooks';

export function NewExperimentPage() {
  const navigate = useNavigate();
  const { data: tasks = [] } = useTasks();
  const { data: agents = [] } = useAgents();
  const { data: models = [] } = useModels();
  const { data: catalog } = useCatalogExtensions();
  const { data: dockerStatus } = useDockerStatus();
  const createExperiment = useCreateExperiment();
  const createArtifact = useCreateArtifact();
  const importCatalogExtensions = useImportCatalogExtensions();
  const startExperiment = useStartExperiment();

  const [name, setName] = useState('');
  const [description, setDescription] = useState('');
  const [taskCategory, setTaskCategory] = useState('');
  const [selectedTaskId, setSelectedTaskId] = useState('');
  const [workspaceSourceType, setWorkspaceSourceType] = useState('task_codebase');
  const [localPath, setLocalPath] = useState('');
  const [gitURL, setGitURL] = useState('');
  const [gitRef, setGitRef] = useState('');
  const [selectedAgent, setSelectedAgent] = useState('cursor');
  const [selectedModel, setSelectedModel] = useState('gpt-5.4');
  const [runsPerVariant, setRunsPerVariant] = useState(5);
  const [timeoutSeconds, setTimeoutSeconds] = useState(600);
  const [maxConcurrent] = useState(1);
  const [variants, setVariants] = useState<DraftVariantContext[]>([createDraftVariant('Control', true)]);
  const [submitError, setSubmitError] = useState('');

  useEffect(() => {
    if (!selectedTaskId && tasks[0]?.id) {
      setSelectedTaskId(tasks[0].id);
    }
  }, [selectedTaskId, tasks]);

  const selectedTask = useMemo(
    () => tasks.find((task) => task.id === selectedTaskId) ?? tasks[0],
    [selectedTaskId, tasks],
  );

  const payload = useMemo(
    () => ({
      name,
      description,
      task_id: selectedTaskId || tasks[0]?.id,
      workspace_source_type: workspaceSourceType,
      local_path: localPath,
      git_url: gitURL,
      git_ref: gitRef,
      model: selectedModel || models[0]?.model_id,
      agent_cli: selectedAgent || agents[0]?.name,
      execution_mode: 'cli',
      runs_per_variant: runsPerVariant,
      temperature: 0,
      timeout_seconds: timeoutSeconds,
      max_concurrent: maxConcurrent,
      variants: variants.map((variant, index) => ({
        name: variant.name || `Variant ${index + 1}`,
        description: variant.description,
        is_control: variant.is_control,
        ordering: index,
      })),
    }),
    [
      agents,
      description,
      gitRef,
      gitURL,
      localPath,
      models,
      maxConcurrent,
      name,
      runsPerVariant,
      selectedAgent,
      selectedModel,
      selectedTaskId,
      tasks,
      timeoutSeconds,
      variants,
      workspaceSourceType,
    ],
  );

  function updateVariant(index: number, nextVariant: DraftVariantContext) {
    setVariants((current) =>
      current.map((item, itemIndex) => {
        if (itemIndex !== index) {
          return nextVariant.is_control ? { ...item, is_control: false } : item;
        }
        if (!nextVariant.is_control) {
          return nextVariant;
        }
        return { ...nextVariant, catalogExtensionIds: [], customFiles: [] };
      }),
    );
  }

  function removeVariant(index: number) {
    setVariants((current) => {
      const next = current.filter((_, itemIndex) => itemIndex !== index);
      if (next.some((variant) => variant.is_control) || next.length === 0) {
        return next;
      }
      return next.map((variant, itemIndex) => (itemIndex === 0 ? { ...variant, is_control: true } : variant));
    });
  }

  async function handleSubmit(startAfterCreate: boolean) {
    setSubmitError('');
    try {
      const experiment = await createExperiment.mutateAsync(payload);
      const createdVariants = [...(experiment.variants ?? [])].sort((left, right) => left.ordering - right.ordering);

      for (const [index, variant] of variants.entries()) {
        const createdVariant = createdVariants[index];
        if (!createdVariant) {
          continue;
        }
        for (const file of variant.customFiles) {
          if (!file.file_path.trim() || !file.content.trim()) {
            continue;
          }
          await createArtifact.mutateAsync({
            variantId: createdVariant.id,
            payload: {
              artifact_type: file.artifact_type,
              source_kind: 'custom_file',
              display_name: file.file_path,
              file_path: file.file_path,
              content: file.content,
            },
          });
        }
        if (variant.catalogExtensionIds.length) {
          await importCatalogExtensions.mutateAsync({
            variantId: createdVariant.id,
            extensionIds: variant.catalogExtensionIds,
          });
        }
      }

      if (startAfterCreate) {
        await startExperiment.mutateAsync(experiment.id);
      }
      navigate(`/experiments/${experiment.id}/${startAfterCreate ? 'monitor' : 'results'}`);
    } catch (error) {
      setSubmitError(error instanceof Error ? error.message : 'Could not create experiment.');
    }
  }

  const submitting =
    createExperiment.isPending || createArtifact.isPending || importCatalogExtensions.isPending || startExperiment.isPending;

  return (
    <div className="space-y-5">
      <Card>
        <CardHeader
          title="Identify this experiment"
          description="A short name and description help you find it later on the dashboard."
        />
        <div className="grid gap-3 sm:grid-cols-2">
          <Input value={name} onChange={(event) => setName(event.target.value)} placeholder="Experiment name" />
          <Input
            value={description}
            onChange={(event) => setDescription(event.target.value)}
            placeholder="Short description (optional)"
          />
        </div>
      </Card>

      <StepCard step={1} title="Workspace source" description="Pick the repository the agent will operate on.">
        <StepWorkspaceSource
          workspaceSourceType={workspaceSourceType}
          localPath={localPath}
          gitURL={gitURL}
          gitRef={gitRef}
          onChange={(next) => {
            setWorkspaceSourceType(next.workspaceSourceType);
            setLocalPath(next.localPath);
            setGitURL(next.gitURL);
            setGitRef(next.gitRef);
          }}
        />
      </StepCard>

      <StepCard step={2} title="Task" description="Filter by category and choose the prompt the agent will try to solve.">
        <StepTask
          tasks={tasks}
          selectedTaskId={selectedTaskId}
          selectedCategory={taskCategory}
          onCategoryChange={setTaskCategory}
          onChange={setSelectedTaskId}
        />
        {selectedTask && (
          <div className="mt-4 rounded-lg border border-slate-200 bg-slate-50/60 p-3 text-xs text-slate-600">
            <div className="text-sm font-medium text-slate-900">{selectedTask.name}</div>
            <div className="mt-2 whitespace-pre-wrap leading-5">{selectedTask.task_prompt}</div>
          </div>
        )}
      </StepCard>

      <StepCard
        step={3}
        title="Context per variant"
        description="Import spec-kit extensions and attach custom files. Every variant runs against the same task."
        action={
          variants.length < 4 ? (
            <AddComparisonVariantButton
              onClick={() =>
                setVariants((current) => [...current, createDraftVariant(`Variant ${current.length + 1}`, false)])
              }
            />
          ) : null
        }
      >
        <div className="space-y-4">
          {variants.map((variant, index) => (
            <Card key={`${variant.name}-${index}`} className="border-slate-200">
              <div className="mb-3 flex items-center justify-between gap-3">
                <div>
                  <div className="flex items-center gap-2">
                    <div className="text-sm font-semibold text-slate-900">Variant {index + 1}</div>
                    {variant.is_control && <Badge tone="info">Control</Badge>}
                  </div>
                  <div className="text-xs text-slate-500">
                    {variant.is_control
                      ? 'Baseline reference. Context extensions are disabled for control variants.'
                      : 'Comparison variant with added context.'}
                  </div>
                </div>
                {variants.length > 1 && (
                  <Button
                    type="button"
                    variant="ghost"
                    size="sm"
                    onClick={() => removeVariant(index)}
                  >
                    Remove
                  </Button>
                )}
              </div>
              <StepContextSource
                variant={variant}
                catalogExtensions={catalog?.extensions ?? []}
                contextDisabled={variant.is_control}
                onChange={(nextVariant) => updateVariant(index, nextVariant)}
              />
            </Card>
          ))}
        </div>
      </StepCard>

      <StepCard step={4} title="Run parameters" description="Pick the agent, model, repetitions, and timeout per run.">
        <div className="space-y-3">
          <StepAgent
            agents={agents}
            models={models}
            agent={selectedAgent}
            model={selectedModel}
            onAgentChange={setSelectedAgent}
            onModelChange={setSelectedModel}
          />
          <StepParams
            runsPerVariant={runsPerVariant}
            timeoutSeconds={timeoutSeconds}
            onRunsChange={setRunsPerVariant}
            onTimeoutChange={setTimeoutSeconds}
          />
          <div className="rounded-lg border border-amber-200 bg-amber-50 p-3 text-xs text-amber-800">
            Runs execute one at a time to avoid local CLI credential conflicts.
          </div>
          <div className="rounded-lg border border-slate-200 bg-white p-3 text-xs text-slate-600">
            <div className="flex flex-wrap items-center gap-2">
              <span className="font-medium text-slate-900">Sandbox runtime</span>
              <Badge tone={dockerStatus?.healthy ? 'success' : 'warning'}>
                {dockerStatus?.healthy ? 'Docker sandbox' : 'Local fallback'}
              </Badge>
              {dockerStatus?.sandbox_image && <span>{dockerStatus.sandbox_image}</span>}
            </div>
            {dockerStatus?.message && <div className="mt-1 text-slate-500">{dockerStatus.message}</div>}
          </div>
        </div>
      </StepCard>

      {submitError && (
        <Card className="border-red-200 bg-red-50 text-sm text-red-700">{submitError}</Card>
      )}

      <div className="flex flex-wrap justify-end gap-2">
        <Button type="button" variant="outline" disabled={submitting || !selectedTaskId} onClick={() => handleSubmit(false)}>
          Save draft
        </Button>
        <Button type="button" disabled={submitting || !selectedTaskId} onClick={() => handleSubmit(true)}>
          {submitting ? 'Creating...' : 'Create & start'}
        </Button>
      </div>
    </div>
  );
}

function StepCard({
  step,
  title,
  description,
  action,
  children,
}: {
  step: number;
  title: string;
  description?: string;
  action?: React.ReactNode;
  children: React.ReactNode;
}) {
  return (
    <Card>
      <div className="mb-4 flex items-start justify-between gap-3">
        <div className="flex items-start gap-3">
          <div className="flex h-7 w-7 items-center justify-center rounded-full bg-slate-900 text-xs font-semibold text-white">
            {step}
          </div>
          <div>
            <div className="text-sm font-semibold text-slate-900">{title}</div>
            {description && <div className="text-xs text-slate-500">{description}</div>}
          </div>
        </div>
        {action}
      </div>
      {children}
    </Card>
  );
}

function createDraftVariant(name: string, isControl: boolean): DraftVariantContext {
  return {
    name,
    description: isControl ? 'Baseline run with no extra context files.' : '',
    is_control: isControl,
    catalogExtensionIds: [],
    customFiles: [],
  };
}
