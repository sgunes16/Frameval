import { defineConfig, devices } from '@playwright/test';

/**
 * Playwright config for Frameval frontend E2E smoke tests.
 *
 * The `webServer` block boots a Vite preview server on port 4173 before any
 * test runs and tears it down at the end. Tests are kept hermetic by stubbing
 * `/api/**` via `page.route()` inside each spec — no backend, no Docker,
 * suitable for local dev and a headless CI runner.
 *
 * Chromium-only on purpose: cross-browser coverage is deferred to the nightly
 * workflow once richer E2E specs land (see issue #83's spec list — six
 * specs total, only the smoke pair lives here today).
 */
export default defineConfig({
  testDir: './test/e2e',
  testMatch: '**/*.spec.ts',
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 1 : 0,
  workers: process.env.CI ? 1 : undefined,
  reporter: process.env.CI ? 'github' : 'list',
  use: {
    baseURL: 'http://localhost:4173',
    trace: 'on-first-retry',
  },
  projects: [
    {
      name: 'chromium',
      use: { ...devices['Desktop Chrome'] },
    },
  ],
  webServer: {
    command: 'npm run preview -- --port 4173 --strictPort',
    url: 'http://localhost:4173',
    reuseExistingServer: !process.env.CI,
    timeout: 60_000,
  },
});
