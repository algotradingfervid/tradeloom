import { expect, test } from '@playwright/test';

const LONG_NAME = 'BHARATIYA INTERNATIONAL INFRASTRUCTURE AND ENGINEERING SERVICES LIMITED';

// Ruling 96 I2: instrument cells use InstrumentName (no Column.title), so the cell's title sits on `.instrument` and
// only `.instrument__name` truncates (the plan's `td .cell-truncate` selector predates that ruling).
for (const route of ['/holdings', '/positions', '/orders']) {
  test(`review focus: ${route} at 1280×720 with a ${LONG_NAME.length}-character name truncates without horizontal scroll`, async ({ page }) => {
    await page.setViewportSize({ width: 1280, height: 720 });
    await page.goto('/?seed=longnames');
    await expect(page.getByRole('button', { name: 'Account menu' })).toBeVisible();
    await page.evaluate((r) => window.__tradeloomMock?.emit('app.navigate', { route: r }), route);
    const cell = page.locator('td .instrument', { hasText: LONG_NAME }).first();
    await expect(cell).toBeVisible();
    expect(await cell.getAttribute('title')).toBe(LONG_NAME);
    const name = cell.locator('.instrument__name');
    expect(await name.evaluate((el) => el.scrollWidth > el.clientWidth)).toBe(true);
    const frameOverflow = await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth);
    expect(frameOverflow).toBe(0);
    const bodyOverflow = await page.locator('.list-page__body').evaluate((el) => el.scrollWidth - el.clientWidth);
    expect(bodyOverflow).toBeLessThanOrEqual(0);
  });
}
