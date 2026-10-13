import { expect, test } from '@playwright/test';
import { openApp } from './helpers';

// P0-5 Task 11 / Ruling 119 I1: the full-view chart sits outside the shell frame but renders ShellServices, so the
// app-wide shortcuts work there too. Shell scope only; the chart's real-render checks are P0-5 Task 19's.
const fullView = (page: import('@playwright/test').Page) => page.getByRole('main', { name: 'Full-view chart' });

async function openFullView(page: import('@playwright/test').Page): Promise<void> {
  await openApp(page);
  await expect(page.getByRole('region', { name: 'Main chart' }).getByRole('button', { name: 'Change symbol' })).toHaveText(/NIFTY 50/);
  await page.keyboard.press('F11');
  await expect(fullView(page)).toBeVisible();
  await expect(page.getByRole('navigation', { name: 'Main navigation' })).toHaveCount(0);
}

test('FR-SHELL-019 in the full view: Alt+1 opens Home in the shell', async ({ page }) => {
  await openFullView(page);
  await page.keyboard.press('Alt+1');
  await expect(fullView(page)).toHaveCount(0);
  await expect(page.getByRole('navigation', { name: 'Main navigation' }).getByRole('link', { name: 'Home' })).toHaveAttribute('aria-current', 'page');
});

test('FR-SHELL-019 in the full view: F1 opens Help', async ({ page }) => {
  await openFullView(page);
  await page.keyboard.press('F1');
  await expect(page.getByRole('heading', { level: 1, name: 'Help' })).toBeVisible();
  await expect(fullView(page)).toHaveCount(0);
});

test('FR-CHART-017 AC3 at 1280×720: the full view fills the window without a scrollbar', async ({ page }) => {
  await openFullView(page);
  const overflow = await page.evaluate(() => ({
    x: document.documentElement.scrollWidth - document.documentElement.clientWidth,
    y: document.documentElement.scrollHeight - document.documentElement.clientHeight,
  }));
  expect(overflow).toEqual({ x: 0, y: 0 });
});
