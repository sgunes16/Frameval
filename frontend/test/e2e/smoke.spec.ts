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
  // Stub every /api/* call with an empty-ish default. Playwright matches
  // routes in LIFO order — the *last* registered handler for a URL wins.
  // Per-test overrides should therefore be added INSIDE the `test()` body
  // (or via a `beforeEach` that runs after this one); they will take
  // precedence over this generic catch-all by virtue of being registered
  // later.
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

  // The dashboard renders three links that resolve to /experiments: the
  // hero-card CTA at the top of the page, the "View all" link in the
  // recent-experiments card header, and the empty-state CTA. They all
  // route to the same place, so any one is a valid target for this nav
  // smoke test. We pick the hero CTA explicitly (it is the most prominent
  // user-facing entry point and the one most likely to regress visibly).
  await page.locator('a[href="/experiments"]', { hasText: /view experiments/i }).first().click();
  await expect(page).toHaveURL(/\/experiments$/);
});
