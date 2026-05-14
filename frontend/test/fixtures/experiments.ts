import type { Experiment } from '../../src/lib/types';

/**
 * Canned Experiment fixtures for use across page and hook tests.
 * Keep these minimal — extend only when a specific test demands a new field.
 */

export function makeExperiment(overrides: Partial<Experiment> = {}): Experiment {
  return {
    id: 'exp-test-1',
    name: 'Test experiment',
    description: 'fixture',
    status: 'draft',
    task_id: 'task-1',
    model: 'qwen2.5-coder:7b',
    agent_cli: 'aider',
    execution_mode: 'cli',
    runs_per_variant: 5,
    temperature: 0,
    timeout_seconds: 600,
    max_concurrent: 1,
    created_at: '2026-05-14T09:00:00Z',
    variants: [],
    ...overrides,
  };
}

export const emptyExperimentList: Experiment[] = [];

export const oneExperimentList: Experiment[] = [makeExperiment()];
