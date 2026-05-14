import '@testing-library/jest-dom/vitest';
import { cleanup } from '@testing-library/react';
import { afterAll, afterEach, beforeAll } from 'vitest';

import { server } from './msw/server';

/**
 * Global Vitest setup.
 *
 * - Boots an MSW server before the suite so fetch() calls never hit a real
 *   network. The default handler set lives in `test/msw/handlers.ts`; tests
 *   override individual endpoints via `server.use(...)`.
 * - Resets any per-test handler overrides between tests so a test's mock
 *   does not bleed into the next.
 * - Tears the MSW server down after the suite finishes.
 * - Imports @testing-library/jest-dom matchers so `expect(...).toBeInTheDocument()`
 *   et al. are available in every test without per-file imports.
 */

beforeAll(() => {
  // Fail closed: an unhandled fetch should explode rather than hit an
  // unmocked endpoint silently. Tests that intentionally call new
  // endpoints must add a handler — either to the defaults file or via
  // `server.use(...)` in the test body.
  //
  // Note: MSW only intercepts fetch(). Native WebSocket connections
  // (e.g., the engine's /ws stream consumed by useWebSocket) are not
  // intercepted. Tests that render components calling useWebSocket
  // will see a WebSocket connect-error in the happy-dom environment;
  // mock the hook directly or stub global.WebSocket per test until
  // an MSW WebSocket handler ships in a follow-up.
  server.listen({ onUnhandledRequest: 'error' });
});

afterEach(() => {
  // Tear down anything React rendered into the test DOM. The automatic
  // cleanup that @testing-library/react ships with only fires when
  // `globals: true` is set on the vitest config; we keep globals off
  // for stricter imports, so we wire cleanup manually.
  cleanup();
  server.resetHandlers();
});

afterAll(() => {
  server.close();
});
