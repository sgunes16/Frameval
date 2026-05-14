import { http, HttpResponse } from 'msw';

import { emptyExperimentList } from '../fixtures/experiments';

/**
 * Default MSW handlers for the engine's REST surface. Every test starts with
 * these in place; individual tests override specific endpoints via
 * `server.use(http.get(...))` to vary the response per scenario.
 *
 * Coverage: only the endpoints currently consumed by the frontend hooks.
 * Add new defaults here when a new endpoint goes live; tests that mock the
 * endpoint can keep working against the default until they need a specific
 * response shape.
 */
export const defaultHandlers = [
  // api.ts builds URLs as `${API_BASE}${path}` without normalizing trailing
  // slashes; every current hook uses the no-trailing-slash form, so we only
  // mock that. If a future API change introduces trailing slashes the test
  // will fail loudly under `onUnhandledRequest: 'error'` rather than mask
  // the inconsistency behind a duplicate handler.
  http.get('/api/experiments', () => HttpResponse.json(emptyExperimentList)),
  http.get('/api/tasks', () => HttpResponse.json([])),
  http.get('/api/config/models', () => HttpResponse.json([])),
  http.get('/api/config/agents', () => HttpResponse.json([])),
  http.get('/api/config/harnesses', () => HttpResponse.json([])),
  http.get('/api/config/executors', () => HttpResponse.json([])),
  http.get('/api/config/api-keys', () => HttpResponse.json([])),
  http.get('/api/system/docker', () =>
    HttpResponse.json({ healthy: true, mode: 'docker', sandbox_image: 'frameval-sandbox:local' }),
  ),
  http.get('/api/system/queue', () =>
    HttpResponse.json({ depth: 0, active_workers: 0, max_workers: 1 }),
  ),
  http.get('/api/health', () => HttpResponse.json({ ok: true })),
];
