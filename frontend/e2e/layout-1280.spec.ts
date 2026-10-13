import { expect, test } from '@playwright/test';
import { ALL_IN_SCOPE } from '../src/instruments/fixtures';
import { openApp } from './helpers';

for (const theme of ['light', 'dark'] as const) {
  test(`NFR-11 / FR-SHELL-001 AC2 / FR-HOME-001 AC3 at 1280×720 (${theme})`, async ({ page, browserName }) => {
    await page.setViewportSize({ width: 1280, height: 720 });
    await openApp(page);
    await page.evaluate((t) => document.documentElement.setAttribute('data-theme', t), theme);
    await expect(page.getByRole('region', { name: /Price chart/ })).toBeVisible();
    const overflow = await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth);
    expect(overflow).toBeLessThanOrEqual(0);
    for (const pill of await page.getByRole('group', { name: 'Index ticker' }).getByRole('button').all()) {
      const b = (await pill.boundingBox())!;
      expect(b.x).toBeGreaterThanOrEqual(0);
      expect(b.x + b.width).toBeLessThanOrEqual(1280);
    }
    // < 1440 px: pills show LTP and percent only (mock NIFTY 50 change is +112.50)
    await expect(page.getByRole('group', { name: 'Index ticker' }).getByRole('button', { name: /^NIFTY 50/ })).not.toContainText('+112.50');
    const header = page.getByRole('toolbar', { name: 'Chart controls' });
    const hb = (await header.boundingBox())!;
    expect(hb.x + hb.width).toBeLessThanOrEqual(1280);
    const panel = page.getByRole('region', { name: 'Watchlist', exact: true });
    await panel.scrollIntoViewIfNeeded();
    await expect(panel).toBeInViewport();
    // keyboard reachability: Tab reaches the symbol button, refresh and timeframe in order
    await expect(page.getByRole('button', { name: 'Refresh chart data' })).toBeEnabled(); // disabled buttons are skipped by Tab
    // macOS WebKit moves plain Tab between text fields only (the platform default); Option+Tab reaches every control.
    // WebKitGTK (Wails on Linux) and Chromium reach buttons with Tab.
    const TAB = browserName === 'webkit' && process.platform === 'darwin' ? 'Alt+Tab' : 'Tab';
    await page.getByRole('button', { name: 'Change symbol' }).focus();
    await page.keyboard.press(TAB);
    await expect(page.getByRole('button', { name: 'Refresh chart data' })).toBeFocused();
    await page.keyboard.press(TAB);
    await expect(page.getByLabel('Timeframe')).toBeFocused();
  });
}

test('FR-HOME-001 AC3 at 1280×720: a long watchlist scrolls inside the right column; the chart stays put', async ({ page }) => {
  await page.setViewportSize({ width: 1280, height: 720 });
  await openApp(page);
  const chart = page.getByRole('region', { name: /Price chart/ });
  await expect(chart).toBeVisible();
  // Every in-scope instrument in Watchlist 1, in Go's watchlists.changed shape.
  const items = ALL_IN_SCOPE.map((i) => ({
    token: i.token, exchange: i.exchange, symbol: i.tradingsymbol, displayName: i.displayName, name: i.name, isIndex: i.isIndex, missing: false,
  }));
  await page.evaluate((its) => window.__tradeloomMock!.emit('watchlists.changed', {
    activeId: 'wl-1', lists: [{ id: 'wl-1', name: 'Watchlist 1', items: its }],
  }), items);
  const panel = page.getByRole('region', { name: 'Watchlist', exact: true });
  const rows = panel.locator('tbody tr[data-token]');
  await expect(rows).toHaveCount(items.length);
  const right = page.getByTestId('home-right');
  expect(await right.evaluate((el) => el.scrollHeight - el.clientHeight)).toBeGreaterThan(0);
  const before = await chart.boundingBox();
  const last = rows.last();
  await last.scrollIntoViewIfNeeded();
  expect(await right.evaluate((el) => el.scrollTop)).toBeGreaterThan(0);
  await expect(last).toBeInViewport();
  // Nothing paints over the last row (no sibling overlaps the overflowing panel).
  const covered = await last.evaluate((el) => {
    const r = el.getBoundingClientRect();
    const hit = document.elementFromPoint(r.left + 8, r.top + r.height / 2);
    return !(hit && el.contains(hit));
  });
  expect(covered).toBe(false);
  expect(await chart.boundingBox()).toEqual(before);
  expect(await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth)).toBeLessThanOrEqual(0);
});
