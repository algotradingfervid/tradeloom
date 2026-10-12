import { expect, test } from '@playwright/test';

test('FR-SHELL-001 AC2: at 1280×720 no horizontal scrollbar; status, quick links and avatar fully visible', async ({ page }) => {
  await page.setViewportSize({ width: 1280, height: 720 });
  await page.goto('/');
  await expect(page.getByRole('navigation', { name: 'Main navigation' })).toBeVisible();
  const overflow = await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth);
  expect(overflow).toBe(0);
  const targets = [
    page.getByRole('button', { name: /^Market data status/ }),
    page.getByRole('link', { name: 'Order Book' }),
    page.getByRole('link', { name: 'Positions' }),
    page.getByRole('button', { name: 'Account menu' }),
  ];
  // The bell (P3) joins this list when it ships.
  for (const t of targets) {
    const box = await t.boundingBox();
    expect(box).not.toBeNull();
    expect(box!.x).toBeGreaterThanOrEqual(0);
    expect(box!.y).toBeGreaterThanOrEqual(0);
    expect(box!.x + box!.width).toBeLessThanOrEqual(1280);
  }
  // P0-5 Task 12: the three index ticker pills. .topbar__ticker clips its overflow and boundingBox() ignores clipping,
  // so each pill must lie inside the ticker container's box, not merely inside the window.
  const ticker = await page.getByTestId('topbar-ticker').boundingBox();
  expect(ticker).not.toBeNull();
  for (const name of [/^NIFTY 50,/, /^NIFTY BANK,/, /^SENSEX,/]) {
    const box = await page.getByRole('button', { name }).boundingBox();
    expect(box).not.toBeNull();
    expect(box!.x).toBeGreaterThanOrEqual(ticker!.x);
    expect(box!.x + box!.width).toBeLessThanOrEqual(ticker!.x + ticker!.width);
    expect(box!.x + box!.width).toBeLessThanOrEqual(1280);
  }
});
