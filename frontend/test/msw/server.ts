import { setupServer } from 'msw/node';

import { defaultHandlers } from './handlers';

/**
 * The Node-side MSW request interceptor used by every Vitest test.
 *
 * Tests get the default handler set out of the box; per-test overrides go
 * through `server.use(...)` and are reset between tests via the
 * `afterEach(server.resetHandlers)` hook wired in `vitest.setup.ts`.
 */
export const server = setupServer(...defaultHandlers);
