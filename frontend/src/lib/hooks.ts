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
  ModelConfig,
  ParsedTurn,
  QueueStatus,
  Run,
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

export function useRun(runId?: string) {
  return useQuery({ queryKey: ['run', runId], enabled: Boolean(runId), queryFn: () => api.get<Run>(`/runs/${runId}`) });
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

export function useGrade(runId?: string) {
  return useQuery({ queryKey: ['grade', runId], enabled: Boolean(runId), queryFn: () => api.get<Grade>(`/runs/${runId}/grade`) });
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

export function useExecutors() {
  return useQuery({ queryKey: ['executors'], queryFn: () => api.get<ExecutorInfo[]>('/config/executors') });
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

export function useAPIKeys() {
  return useQuery({ queryKey: ['api-keys'], queryFn: () => api.get<APIKey[]>('/config/api-keys') });
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

export function useCreateArtifact() {
  const client = useQueryClient();
  return useMutation({
    mutationFn: ({ variantId, payload }: { variantId: string; payload: unknown }) => api.post<ArtifactVersion>(`/variants/${variantId}/artifacts`, payload),
    onSuccess: (_data, variables) => client.invalidateQueries({ queryKey: ['artifacts', variables.variantId] }),
  });
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
