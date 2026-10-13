import { expect, type Locator, type Page } from '@playwright/test';

export const P0_VIEWS = [
  '/home', '/holdings', '/positions', '/orders', '/profile',
  '/settings/kite', '/settings/appearance', '/settings/chart', '/settings/calendar', '/settings/data',
  '/settings/shortcuts', '/settings/diagnostics', '/settings/about',
  '/help', '/help/kite-setup', '/help/keyboard-shortcuts', '/legal/terms', '/legal/privacy', '/legal/risk', '/no-such-view',
];

export async function openApp(page: Page, query = ''): Promise<void> {
  await page.goto(`/${query}`);
  await expect(page.getByRole('button', { name: 'Account menu' })).toBeVisible();
}

/** In-app navigation through the backend's `app.navigate` event (the memory router has no URL bar). */
export async function go(page: Page, route: string): Promise<void> {
  await page.evaluate((r) => window.__tradeloomMock?.emit('app.navigate', { route: r }), route);
  await page.evaluate(() => new Promise((r) => requestAnimationFrame(() => requestAnimationFrame(r))));
  await expect(page.getByRole('heading', { level: 1 }).first()).toBeAttached();
}

export async function tabTo(page: Page, target: Locator, max = 30): Promise<void> {
  for (let i = 0; i < max; i++) {
    if (await target.evaluate((el) => el === document.activeElement)) return;
    await page.keyboard.press('Tab');
  }
  await expect(target).toBeFocused();
}
