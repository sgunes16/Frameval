import { useEffect, useMemo, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { api } from './api';
import type { AgentInfo, APIKey, ArtifactVersion, Baseline, CatalogResponse, Experiment, ExperimentStat, Grade, ModelConfig, Run, Task, Transcript, Variant } from './types';

export function useExperiments() {
  return useQuery({ queryKey: ['experiments'], queryFn: () => api.get<Experiment[]>('/experiments') });
}

export function useExperiment(id?: string) {
  return useQuery({ queryKey: ['experiment', id], enabled: Boolean(id), queryFn: () => api.get<Experiment>(`/experiments/${id}`) });
}

export function useRuns(experimentId?: string) {
  return useQuery({ queryKey: ['runs', experimentId], enabled: Boolean(experimentId), queryFn: () => api.get<Run[]>(`/experiments/${experimentId}/runs`) });
}

export function useExperimentStats(experimentId?: string) {
  return useQuery({ queryKey: ['experiment-stats', experimentId], enabled: Boolean(experimentId), queryFn: () => api.get<ExperimentStat[]>(`/experiments/${experimentId}/stats`) });
}

export function useRun(runId?: string) {
  return useQuery({ queryKey: ['run', runId], enabled: Boolean(runId), queryFn: () => api.get<Run>(`/runs/${runId}`) });
}

export function useTranscript(runId?: string) {
  return useQuery({ queryKey: ['transcript', runId], enabled: Boolean(runId), queryFn: () => api.get<Transcript>(`/runs/${runId}/transcript`) });
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

export function useBaselines() {
  return useQuery({ queryKey: ['baselines'], queryFn: () => api.get<Baseline[]>('/baselines') });
}

export function useBaseline(id?: string) {
  return useQuery({ queryKey: ['baseline', id], enabled: Boolean(id), queryFn: () => api.get<Baseline>(`/baselines/${id}`) });
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

export function useAPIKeys() {
  return useQuery({ queryKey: ['api-keys'], queryFn: () => api.get<APIKey[]>('/config/api-keys') });
}

export function useCatalogExtensions() {
  return useQuery({ queryKey: ['catalog-extensions'], queryFn: () => api.get<CatalogResponse>('/config/catalog/extensions') });
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

export function useImportCatalogExtensions() {
  const client = useQueryClient();
  return useMutation({
    mutationFn: ({ variantId, extensionIds }: { variantId: string; extensionIds: string[] }) =>
      api.post<ArtifactVersion[]>(`/variants/${variantId}/catalog-extensions`, { extension_ids: extensionIds }),
    onSuccess: (_data, variables) => client.invalidateQueries({ queryKey: ['artifacts', variables.variantId] }),
  });
}

export function useWebSocket() {
  const wsBase = import.meta.env.VITE_WS_BASE_URL || `${window.location.origin.replace(/^http/, 'ws')}/ws`;
  const [events, setEvents] = useState<Array<{ type: string; payload: unknown }>>([]);
  useEffect(() => {
    const socket = new WebSocket(wsBase);
    socket.onmessage = (event) => {
      const data = JSON.parse(event.data) as { type: string; payload: unknown };
      setEvents((current) => [...current.slice(-49), data]);
    };
    return () => socket.close();
  }, [wsBase]);
  return useMemo(() => ({ events }), [events]);
}
