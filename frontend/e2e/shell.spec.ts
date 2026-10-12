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
    // P0-5 Task 12: the three index ticker pills (accessible name starts with the index name).
    page.getByRole('button', { name: /^NIFTY 50,/ }),
    page.getByRole('button', { name: /^NIFTY BANK,/ }),
    page.getByRole('button', { name: /^SENSEX,/ }),
  ];
  // The bell (P3) joins this list when it ships.
  for (const t of targets) {
    const box = await t.boundingBox();
    expect(box).not.toBeNull();
    expect(box!.x).toBeGreaterThanOrEqual(0);
    expect(box!.y).toBeGreaterThanOrEqual(0);
    expect(box!.x + box!.width).toBeLessThanOrEqual(1280);
  }
});
