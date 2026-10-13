import { expect, test, type Page } from '@playwright/test';
import { ALL_IN_SCOPE } from '../src/instruments/fixtures';
import { openApp } from './helpers';

type Seen = { token: string | null; tag: string | null; outOfScope: boolean; name: string | null; where: string };
// `within` scopes a collection to the open dialog, so 'chart-search' and 'picker' count only what those surfaces render.
const collect = (page: Page, where: string, within = ''): Promise<Seen[]> =>
  page.$$eval(`${within} [data-instrument-token]`.trim(), (els, w) => els.map((e) => ({
    token: e.getAttribute('data-instrument-token'),
    tag: e.getAttribute('data-exchange-tag'),
    outOfScope: e.hasAttribute('data-out-of-scope'),
    name: (e.querySelector('[data-instrument-name]') ?? e).textContent,
    where: w,
  })), where);

test('FR-WATCH-013 AC3 + FR-WATCH-014 AC1/AC2: one formatter, one tag per instrument everywhere', async ({ page }) => {
  await page.setViewportSize({ width: 1440, height: 900 });
  await openApp(page);
  const panel = page.getByRole('region', { name: 'Watchlist', exact: true });
  const search = panel.getByRole('combobox', { name: 'Search instruments to add' });
  // Add by token, not by result position: search ranking against the mock is not asserted (Ruling 87 M4).
  for (const [q, token] of [['reliance', 738561], ['nifty oct fut', 13238530], ['sbin', 779521]] as const) {
    await search.fill(q);
    const option = panel.getByRole('option').filter({ has: page.locator(`[data-instrument-token="${token}"]`) });
    await expect(option).toBeVisible();
    await option.click();
  }
  await search.press('Escape');
  const seen: Seen[] = [...await collect(page, 'home')];

  await page.getByRole('button', { name: 'Change symbol' }).click();
  await page.getByRole('combobox', { name: 'Search instruments', exact: true }).fill('reliance');
  await expect(page.getByRole('dialog').getByRole('option').first()).toBeVisible();
  seen.push(...await collect(page, 'chart-search', '[role="dialog"]'));
  await page.keyboard.press('Escape');

  await panel.getByRole('button', { name: 'Add instruments…' }).click();
  await page.getByRole('dialog').getByRole('button', { name: 'Watchlist 1' }).click();
  await expect(page.getByRole('dialog').getByRole('checkbox', { name: /^Select / }).first()).toBeVisible();
  seen.push(...await collect(page, 'picker', '[role="dialog"]'));

  const expected = new Map(ALL_IN_SCOPE.map((i) => [String(i.token), i.displayName]));
  const tags = new Map<string, Set<string>>();
  for (const s of seen) {
    // An out-of-scope label (P0-3 Ruling 38) carries data-out-of-scope and never an exchange tag.
    if (s.outOfScope) { expect(s.tag, `${s.token} in ${s.where}`).toBeNull(); continue; }
    expect(expected.get(s.token!), `unknown instrument rendered in ${s.where}`).toBeDefined();
    expect(s.name, `${s.token} in ${s.where}`).toBe(expected.get(s.token!));
    if (s.tag) tags.set(s.token!, (tags.get(s.token!) ?? new Set()).add(s.tag));
  }
  for (const [token, set] of tags) expect([...set], `token ${token}`).toHaveLength(1);
  const relianceNse = tags.get('738561');
  expect(relianceNse && [...relianceNse]).toEqual(['NSE']);
  expect(new Set(seen.filter((s) => s.token === '738561').map((s) => s.where))).toEqual(new Set(['home', 'chart-search', 'picker']));
});
