import { expect, test } from '@playwright/test';
import { go, openApp, P0_VIEWS } from './helpers';

// Ruling 86 M5 in the real engines: no live region inside another (double announcements), with the toasts and the
// expiry banner on screen. The unit tests (ToastRegion, ShellBanners) pin the always-mounted regions themselves.
const NESTED = () => {
  const LIVE = '[aria-live]:not([aria-live="off"]), [role="status"], [role="alert"], [role="log"], [role="marquee"], [role="timer"]';
  return [...document.querySelectorAll(LIVE)]
    .filter((el) => el.parentElement?.closest(LIVE))
    .map((el) => `${el.tagName.toLowerCase()}.${el.className} in ${el.parentElement!.closest(LIVE)!.className}`);
};

test('Ruling 86 M5: no nested live regions on any P0 view, with toasts and the expiry banner showing', async ({ page }) => {
  await openApp(page);
  await go(page, '/settings/data');
  await page.getByRole('button', { name: 'Back up now' }).click(); // a success toast
  await expect(page.getByText(/^Backup saved to /)).toBeVisible();
  await page.evaluate(() => {
    const m = window.__tradeloomMock;
    if (m) m.emit('session.state', { ...m.state.session, state: 'expired' });
  });
  await expect(page.getByText('Kite session expired — log in to continue')).toBeVisible();
  const found: string[] = [];
  for (const route of P0_VIEWS) {
    await go(page, route);
    found.push(...(await page.evaluate(NESTED)).map((n) => `${route}: ${n}`));
  }
  expect(found).toEqual([]);
});

test('Ruling 117 I1: help and legal pages are framed by the shell once signed in, and stand alone before', async ({ page }) => {
  await openApp(page);
  for (const route of ['/help', '/legal/terms']) {
    await go(page, route);
    await expect(page.getByRole('navigation', { name: 'Main navigation' })).toBeVisible();
  }
  await page.goto('/?seed=loggedout');
  await expect(page.getByRole('heading', { name: 'Log in to Kite' })).toBeVisible();
  for (const [route, title] of [['/help', 'Help'], ['/legal/terms', 'Terms of use']]) {
    await page.evaluate((r) => window.__tradeloomMock?.emit('app.navigate', { route: r }), route);
    await expect(page.getByRole('heading', { level: 1, name: title })).toBeVisible();
    await expect(page.getByRole('navigation', { name: 'Main navigation' })).toHaveCount(0);
    await expect(page.getByRole('button', { name: 'Account menu' })).toHaveCount(0);
  }
});
