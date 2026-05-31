import { useEffect, useMemo, useRef, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { api } from './api';
import type {
  AgentInfo,
  APIKey,
  ArtifactVersion,
  Diagnostic,
  DockerStatus,
  Experiment,
  ExecutorInfo,
  Grade,
  HarnessInfo,
  LaunchDiagnosticRequest,
  LaunchDiagnosticResponse,
  LaunchDiagnosticSuiteRequest,
  LaunchDiagnosticSuiteResponse,
  LLMSettings,
  ModelConfig,
  ParsedTurn,
  QueueStatus,
  Rubric,
  Run,
  SpecKitExtensionPublic,
  Task,
  Transcript,
  Variant,
} from './types';

export function useExperiments() {
  return useQuery({ queryKey: ['experiments'], queryFn: () => api.get<Experiment[]>('/experiments') });
}

export function useExperiment(id?: string) {
  return useQuery({ queryKey: ['experiment', id], enabled: Boolean(id), queryFn: () => api.get<Experiment>(`/experiments/${id}`) });
}

export function useRuns(experimentId?: string) {
  return useQuery({ queryKey: ['runs', experimentId], enabled: Boolean(experimentId), queryFn: () => api.get<Run[]>(`/experiments/${experimentId}/runs`) });
}

// useRunsForExperiments fans out across N experiments and returns
// {expId, runs}[] in the same order as the input. Used by the
// Compare page when the launcher fired a matrix of experiments and
// passed them via `?experiments=id1,id2,...`. A 404 on one expId
// resolves to an empty run list rather than failing the whole
// query — partial matrix results are still useful.
export function useRunsForExperiments(experimentIds: string[]) {
  return useQuery({
    queryKey: ['compare-runs', experimentIds],
    enabled: experimentIds.length > 0,
    queryFn: () =>
      Promise.all(
        experimentIds.map(async (id) => {
          try {
            const runs = await api.get<Run[]>(`/experiments/${id}/runs`);
            return { experimentId: id, runs };
          } catch {
            return { experimentId: id, runs: [] as Run[] };
          }
        }),
      ),
  });
}

// useExperimentsForIds — same shape as useExperiment but fans out
// for the Compare matrix view; lets us label each run with its
// experiment's name/agent_cli/model.
export function useExperimentsForIds(experimentIds: string[]) {
  return useQuery({
    queryKey: ['compare-experiments', experimentIds],
    enabled: experimentIds.length > 0,
    queryFn: () =>
      Promise.all(
        experimentIds.map(async (id) => {
          try {
            return await api.get<Experiment>(`/experiments/${id}`);
          } catch {
            return null;
          }
        }),
      ),
  });
}

export function useRun(runId?: string) {
  return useQuery({
    queryKey: ['run', runId],
    enabled: Boolean(runId),
    queryFn: () => api.get<Run>(`/runs/${runId}`),
    // While the run is non-terminal, refetch every 3s so callers (notably
    // useGrade's polling, which keys off run.status) see status flips
    // without manual invalidation.
    refetchInterval: (query) => {
      const data = query.state.data as Run | undefined;
      if (!data) return false;
      const terminal = data.status === 'completed' || data.status === 'failed' || data.status === 'cancelled';
      return terminal ? false : 3000;
    },
  });
}

export function useTranscript(runId?: string) {
  return useQuery({ queryKey: ['transcript', runId], enabled: Boolean(runId), queryFn: () => api.get<Transcript>(`/runs/${runId}/transcript`) });
}

/**
 * Inspector V2 data hook: fetch ONLY the structured ParsedTurns for a
 * run. Cheaper than the full transcript when the consumer (turn-list
 * UI, tool histogram) doesn't need raw_output / filesystem_diff.
 *
 * Returns an empty array when the run has no transcript yet — safe to
 * call during a live run.
 */
export function useRunTurns(runId?: string) {
  return useQuery({
    queryKey: ['run-turns', runId],
    enabled: Boolean(runId),
    queryFn: () => api.get<ParsedTurn[]>(`/runs/${runId}/turns`),
  });
}

/**
 * Compare V2 bulk fetch: turns grouped by run_id for every run in an
 * experiment, served in a single round-trip via the engine's JOIN
 * implementation (no N+1).
 */
export function useExperimentTurns(experimentId?: string) {
  return useQuery({
    queryKey: ['experiment-turns', experimentId],
    enabled: Boolean(experimentId),
    queryFn: () => api.get<Record<string, ParsedTurn[]>>(`/experiments/${experimentId}/turns`),
  });
}

export function useTranscripts(runIds: string[]) {
  return useQuery({
    queryKey: ['transcripts', ...runIds],
    enabled: runIds.length > 0,
    queryFn: async () => {
      const results = await Promise.all(
        runIds.map((id) => api.get<Transcript>(`/runs/${id}/transcript`).catch(() => null)),
      );
      return results.filter((t): t is Transcript => t !== null);
    },
  });
}

export function useDiagnostic(runId?: string) {
  return useQuery({
    queryKey: ['diagnostic', runId],
    enabled: Boolean(runId),
    queryFn: () => api.get<Diagnostic>(`/runs/${runId}/diagnostic`),
  });
}

export function useCompareDiagnostics(runIds: string[]) {
  return useQuery({
    queryKey: ['diagnostics', ...runIds],
    enabled: runIds.length > 0,
    queryFn: () =>
      Promise.all(runIds.map((id) => api.get<Diagnostic>(`/runs/${id}/diagnostic`))),
  });
}

export function useGrade(runId?: string, runStatus?: string) {
  return useQuery({
    queryKey: ['grade', runId],
    enabled: Boolean(runId),
    queryFn: () => api.get<Grade>(`/runs/${runId}/grade`),
    // Poll while the LLM judge is still in flight. The backend persists a
    // partial grade row immediately after sandbox verifications; the judge
    // adds judge_scores when it returns 30-90s later. Stop polling when:
    //  - judge_scores has any entries (judge ran successfully), OR
    //  - raw_judge_responses contains the "llm_judge_disabled" sentinel
    //    (server-side disabled_judge_result() — judge_scores stays empty
    //    forever in that case, so we'd otherwise poll until the run
    //    transitions to a terminal state — and if the run never does,
    //    forever), OR
    //  - the run reached a terminal status, OR
    //  - we've polled 300 times (10 min hard ceiling — safety net against
    //    a hung run that never reaches "completed").
    refetchInterval: (query) => {
      const data = query.state.data as Grade | undefined;
      if (data) {
        if (data.judge_scores && Object.keys(data.judge_scores).length > 0) return false;
        const firstRaw = data.raw_judge_responses?.[0];
        if (firstRaw && firstRaw.startsWith('llm_judge_disabled')) return false;
      }
      if (runStatus === 'completed' || runStatus === 'failed' || runStatus === 'cancelled') return false;
      if ((query.state.dataUpdateCount ?? 0) > 300) return false;
      return 2000;
    },
  });
}

export function useRegradeRun() {
  const client = useQueryClient();
  return useMutation({
    mutationFn: (runId: string) => api.post<void>(`/runs/${runId}/regrade`, null),
    onSuccess: (_, runId) => {
      client.invalidateQueries({ queryKey: ['grade', runId] });
      client.invalidateQueries({ queryKey: ['diagnostic', runId] });
    },
  });
}

// useCompareGrades parallels useCompareDiagnostics: one query that
// fans out to N runs' grade endpoints and resolves to a Grade[] in
// the same order as the input ids. A missing grade (run not yet
// graded, 404) resolves to null so the caller can render a
// placeholder row rather than blowing up the whole panel.
export function useCompareGrades(runIds: string[]) {
  return useQuery({
    queryKey: ['compare-grades', runIds],
    enabled: runIds.length > 0,
    queryFn: () =>
      Promise.all(
        runIds.map((id) =>
          api.get<Grade>(`/runs/${id}/grade`).catch(() => null),
        ),
      ),
  });
}

export function useTasks() {
  return useQuery({ queryKey: ['tasks'], queryFn: () => api.get<Task[]>('/tasks') });
}

export function useTask(id?: string) {
  return useQuery({ queryKey: ['task', id], enabled: Boolean(id), queryFn: () => api.get<Task>(`/tasks/${id}`) });
}

export function useVariants(experimentId?: string) {
  return useQuery({ queryKey: ['variants', experimentId], enabled: Boolean(experimentId), queryFn: () => api.get<Variant[]>(`/experiments/${experimentId}/variants`) });
}

export function useArtifacts(variantId?: string) {
  return useQuery({ queryKey: ['artifacts', variantId], enabled: Boolean(variantId), queryFn: () => api.get<ArtifactVersion[]>(`/variants/${variantId}/artifacts`) });
}

export function useModels() {
  return useQuery({ queryKey: ['models'], queryFn: () => api.get<ModelConfig[]>('/config/models') });
}

export function useAgents() {
  return useQuery({ queryKey: ['agents'], queryFn: () => api.get<AgentInfo[]>('/config/agents') });
}

export function useHarnesses() {
  return useQuery({ queryKey: ['harnesses'], queryFn: () => api.get<HarnessInfo[]>('/config/harnesses') });
}

export function useSpecKitCatalog() {
  return useQuery({
    queryKey: ['speckit', 'catalog'] as const,
    queryFn: () => api.get<SpecKitExtensionPublic[]>('/harnesses/speckit/catalog'),
    staleTime: 24 * 60 * 60 * 1000, // catalog is static within a process
  });
}

export function useExecutors() {
  return useQuery({ queryKey: ['executors'], queryFn: () => api.get<ExecutorInfo[]>('/config/executors') });
}

/**
 * Re-runs the executor's ParseTranscript on an already-captured
 * raw_output and refreshes the persisted ParsedTurns. Used to retro-
 * actively apply parser improvements to existing transcripts without
 * a full agent re-execute.
 */
export function useReparseRun() {
  const client = useQueryClient();
  return useMutation({
    mutationFn: (runId: string) =>
      api.post<{ turn_count: number }>(`/runs/${runId}/reparse`, {}),
    onSuccess: (_data, runId) => {
      client.invalidateQueries({ queryKey: ['run-turns', runId] });
      client.invalidateQueries({ queryKey: ['transcript', runId] });
    },
  });
}

export function useLaunchDiagnostic() {
  const client = useQueryClient();
  return useMutation({
    mutationFn: (payload: LaunchDiagnosticRequest) =>
      api.post<LaunchDiagnosticResponse>('/diagnostic/launch', payload),
    onSuccess: () => {
      client.invalidateQueries({ queryKey: ['experiments'] });
      client.invalidateQueries({ queryKey: ['runs'] });
    },
  });
}

export function useLaunchDiagnosticSuite() {
  const client = useQueryClient();
  return useMutation({
    mutationFn: (payload: LaunchDiagnosticSuiteRequest) =>
      api.post<LaunchDiagnosticSuiteResponse>('/diagnostic/launch-suite', payload),
    onSuccess: () => {
      client.invalidateQueries({ queryKey: ['experiments'] });
      client.invalidateQueries({ queryKey: ['runs'] });
    },
  });
}

export function useAPIKeys() {
  return useQuery({ queryKey: ['config', 'api-keys'], queryFn: () => api.get<APIKey[]>('/config/api-keys') });
}

export function useDockerStatus() {
  return useQuery({ queryKey: ['docker-status'], queryFn: () => api.get<DockerStatus>('/system/docker') });
}

export function useQueueStatus() {
  return useQuery({ queryKey: ['queue-status'], queryFn: () => api.get<QueueStatus>('/system/queue') });
}

export function useCreateExperiment() {
  const client = useQueryClient();
  return useMutation({
    mutationFn: (payload: unknown) => api.post<Experiment>('/experiments', payload),
    onSuccess: () => client.invalidateQueries({ queryKey: ['experiments'] }),
  });
}

export function useStartExperiment() {
  const client = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => api.post(`/experiments/${id}/start`),
    onSuccess: () => client.invalidateQueries({ queryKey: ['experiments'] }),
  });
}

export function useEstimateExperiment() {
  return useMutation({ mutationFn: (id: string) => api.post<{ estimated_cost_usd: number }>(`/experiments/${id}/estimate`) });
}

export function useWebSocket() {
  const wsBase = import.meta.env.VITE_WS_BASE_URL || `${window.location.origin.replace(/^http/, 'ws')}/ws`;
  const nextEventID = useRef(0);
  const [events, setEvents] = useState<Array<{ id: number; type: string; payload: unknown }>>([]);
  useEffect(() => {
    const socket = new WebSocket(wsBase);
    socket.onmessage = (event) => {
      const data = JSON.parse(event.data) as { type: string; payload: unknown };
      const eventID = nextEventID.current;
      nextEventID.current += 1;
      setEvents((current) => {
        const next = [...current, { ...data, id: eventID }];
        const runLogs = next.filter((item) => item.type === 'run.log');
        const otherEvents = next.filter((item) => item.type !== 'run.log').slice(-199);
        return [...runLogs, ...otherEvents].sort((a, b) => a.id - b.id);
      });
    };
    return () => socket.close();
  }, [wsBase]);
  return useMemo(() => ({ events }), [events]);
}

export function useLLMSettings() {
  return useQuery({
    queryKey: ['config', 'llm-settings'],
    queryFn: () => api.get<LLMSettings>('/config/llm-settings'),
  });
}

export function useSaveLLMSettings() {
  const client = useQueryClient();
  return useMutation({
    mutationFn: (payload: Pick<LLMSettings, 'provider' | 'model' | 'enabled'>) =>
      api.put<LLMSettings>('/config/llm-settings', payload),
    onSuccess: () => {
      client.invalidateQueries({ queryKey: ['config', 'llm-settings'] });
      client.invalidateQueries({ queryKey: ['config', 'api-keys'] });
    },
  });
}

export function useUpsertAPIKey() {
  const client = useQueryClient();
  return useMutation({
    mutationFn: (payload: { provider: string; api_key: string }) =>
      api.post<void>('/config/api-keys', payload),
    onSuccess: () => {
      client.invalidateQueries({ queryKey: ['config', 'api-keys'] });
      client.invalidateQueries({ queryKey: ['config', 'llm-settings'] });
    },
  });
}

export function useDeleteAPIKey() {
  const client = useQueryClient();
  return useMutation({
    mutationFn: (provider: string) =>
      api.delete<void>(`/config/api-keys/${encodeURIComponent(provider)}`),
    onSuccess: () => {
      client.invalidateQueries({ queryKey: ['config', 'api-keys'] });
      client.invalidateQueries({ queryKey: ['config', 'llm-settings'] });
    },
  });
}

export function useRubrics() {
  return useQuery({
    queryKey: ['config', 'rubrics'],
    queryFn: () => api.get<Rubric[]>('/config/rubrics'),
  });
}

export function useRubric(key?: string) {
  return useQuery({
    queryKey: ['config', 'rubrics', key],
    enabled: Boolean(key),
    queryFn: () => api.get<Rubric>(`/config/rubrics/${key}`),
  });
}

export function useCreateRubric() {
  const client = useQueryClient();
  return useMutation({
    mutationFn: (payload: Pick<Rubric, 'key' | 'display_name' | 'prompt'>) =>
      api.post<Rubric>('/config/rubrics', payload),
    onSuccess: () => client.invalidateQueries({ queryKey: ['config', 'rubrics'] }),
  });
}

export function useUpdateRubric() {
  const client = useQueryClient();
  return useMutation({
    mutationFn: ({ key, ...payload }: Pick<Rubric, 'key' | 'display_name' | 'prompt'>) =>
      api.put<Rubric>(`/config/rubrics/${key}`, payload),
    onSuccess: () => client.invalidateQueries({ queryKey: ['config', 'rubrics'] }),
  });
}

export function useDeleteRubric() {
  const client = useQueryClient();
  return useMutation({
    mutationFn: (key: string) => api.delete<void>(`/config/rubrics/${key}`),
    onSuccess: () => client.invalidateQueries({ queryKey: ['config', 'rubrics'] }),
  });
}
