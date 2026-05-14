import { expect, test } from '@playwright/test';

/**
 * Smoke specs — prove the Playwright harness is wired end-to-end.
 *
 * The Vite preview server (configured in playwright.config.ts as the
 * `webServer`) boots before tests run. Network calls to `/api/*` are
 * intercepted with Playwright's `page.route()` and replied to with
 * canned empty responses — there is no real backend involved, so these
 * tests do not require docker-compose or the engine binary.
 *
 * Future specs that need realistic backend behavior (run launch, live
 * streaming, etc.) belong under `test/e2e/integration/` and will run
 * against a real docker-compose stack as part of the nightly workflow.
 */

test.beforeEach(async ({ page }) => {
  // Stub every /api/* call with an empty-ish default. Per-test overrides
  // go through `page.route('**/api/specific-endpoint', ...)` _before_ this
  // generic catch-all is registered, so Playwright tries the more specific
  // route first.
  await page.route('**/api/**', async (route) => {
    const url = route.request().url();
    // Status check endpoints return shaped responses; everything else gets [].
    if (url.includes('/system/docker')) {
      await route.fulfill({
        json: { healthy: true, mode: 'docker', sandbox_image: 'frameval-sandbox:local' },
      });
      return;
    }
    if (url.includes('/system/queue')) {
      await route.fulfill({ json: { depth: 0, active_workers: 0, max_workers: 1 } });
      return;
    }
    if (url.includes('/health')) {
      await route.fulfill({ json: { ok: true } });
      return;
    }
    await route.fulfill({ json: [] });
  });
});

test('dashboard renders the empty-state CTA', async ({ page }) => {
  await page.goto('/');
  await expect(page.getByText(/no experiments yet/i)).toBeVisible();
});

test('clicking "View experiments" navigates to /experiments', async ({ page }) => {
  await page.goto('/');
  await page.getByRole('link', { name: /view experiments/i }).first().click();
  await expect(page).toHaveURL(/\/experiments$/);
});
