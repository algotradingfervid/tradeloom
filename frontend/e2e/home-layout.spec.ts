import { expect, test } from '@playwright/test';

test('FR-SHELL-001 AC4 + FR-HOME-001 AC3: the right column scrolls on its own; nothing scrolls horizontally at 1280×720', async ({ page }) => {
  await page.setViewportSize({ width: 1280, height: 720 });
  await page.goto('/');
  const right = page.getByTestId('home-right');
  await expect(right).toBeVisible();
  await right.evaluate((el) => {
    const filler = document.createElement('div');
    filler.style.height = '3000px';
    // The column is a flex container: an empty div would shrink to fit (min-height auto = 0) and nothing would scroll.
    // Real widgets have content, so their automatic minimum height keeps them full size; give the filler the same.
    filler.style.flexShrink = '0';
    el.appendChild(filler);
  });
  const chart = page.getByRole('region', { name: 'Main chart' });
  const before = await chart.boundingBox();
  await right.evaluate((el) => { el.scrollTop = 1500; });
  expect(await right.evaluate((el) => el.scrollTop)).toBeGreaterThan(0);
  expect(await chart.boundingBox()).toEqual(before);
  expect(await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth)).toBe(0);
});
