import { expect, test } from '@playwright/test';
import { go, openApp, P0_VIEWS } from './helpers';

test('FR-SHELL-011 AC2: dark theme applies within 300 ms and survives a restart', async ({ page }) => {
  await openApp(page, '?persist=1');
  await page.getByRole('button', { name: 'Account menu' }).click();
  const elapsed = await page.evaluate(async () => {
    const toggle = document.querySelector<HTMLElement>('[role="menuitemcheckbox"]');
    const t0 = performance.now();
    toggle?.click();
    await new Promise<void>((resolve) => {
      const check = () => (document.documentElement.dataset.theme === 'dark' ? resolve() : requestAnimationFrame(check));
      check();
    });
    return performance.now() - t0;
  });
  expect(elapsed).toBeLessThan(300);
  await page.reload();
  await expect(page.locator('html')).toHaveAttribute('data-theme', 'dark');
});

test('NFR-11 + FR-SHELL-001 AC2: every P0 view fits 1280×720 without a horizontal scrollbar', async ({ page }) => {
  await page.setViewportSize({ width: 1280, height: 720 });
  await openApp(page);
  for (const route of P0_VIEWS) {
    await go(page, route);
    const overflow = await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth);
    expect(overflow, route).toBe(0);
  }
});

test('Global constraint: a P0 build renders no Buy/Sell order buttons anywhere', async ({ page }) => {
  await openApp(page);
  for (const route of P0_VIEWS) {
    await go(page, route);
    await expect(page.getByRole('button', { name: /^(B|S|Buy|Sell)$/ }), route).toHaveCount(0);
  }
});

test('FR-ACCT-007 AC1 (UI): a Personal-plan key (?seed=personal) shows "No data plan" while the portfolio still works', async ({ page }) => {
  await openApp(page, '?seed=personal');
  await expect(page.getByRole('button', { name: /^Market data status: No data plan/ })).toBeVisible();
  await go(page, '/holdings');
  await expect(page.getByRole('row', { name: 'RELIANCE' })).toBeVisible();
});

test('FR-ACCT-006 AC3 (UI): the 06:00 expiry event shows the banner and "Logged out" without leaving the view', async ({ page }) => {
  await openApp(page);
  await go(page, '/holdings');
  await page.evaluate(() => {
    const m = window.__tradeloomMock;
    if (m) m.emit('session.state', { ...m.state.session, state: 'expired' });
  });
  await expect(page.getByText('Kite session expired — log in to continue')).toBeVisible();
  await expect(page.getByRole('button', { name: /^Market data status: Logged out/ })).toBeVisible();
  await expect(page.getByRole('heading', { level: 1, name: 'Holdings' })).toBeVisible();
});
