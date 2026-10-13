import { expect, test } from '@playwright/test';

// Task 7 / Ruling 83-F I2 carry-forward: the boot-failure notice lives in the AppShell banner slot, so it no longer adds
// a page scrollbar above the 100vh shell; Retry reloads the failed slice. Task 9: the onboarding frame shows it too.
const noPageScroll = (page: import('@playwright/test').Page) => page.evaluate(() => ({
  x: document.documentElement.scrollWidth - document.documentElement.clientWidth,
  y: document.documentElement.scrollHeight - document.documentElement.clientHeight,
}));

test('boot notice in the shell at 1280×720: visible, no page scrollbar, Retry clears it', async ({ page }) => {
  await page.setViewportSize({ width: 1280, height: 720 });
  await page.goto('/?fail=system.info');
  const notice = page.getByRole('alert').filter({ hasText: 'Some app data could not be loaded' });
  await expect(notice).toBeVisible();
  await expect(page.getByRole('button', { name: 'Account menu' })).toBeVisible();
  expect(await noPageScroll(page)).toEqual({ x: 0, y: 0 });
  const box = (await notice.boundingBox())!;
  expect(box.y + box.height).toBeLessThanOrEqual(720);
  await page.evaluate(() => window.__tradeloomMock?.clearFailures());
  await notice.getByRole('button', { name: 'Retry' }).click();
  await expect(notice).toHaveCount(0);
});

test('boot notice on the onboarding screen (logged out)', async ({ page }) => {
  await page.setViewportSize({ width: 1280, height: 720 });
  await page.goto('/?seed=loggedout&fail=system.info');
  await expect(page.getByRole('heading', { name: 'Log in to Kite' })).toBeVisible();
  const notice = page.getByRole('alert').filter({ hasText: 'Some app data could not be loaded' });
  await expect(notice).toBeVisible();
  expect((await noPageScroll(page)).x).toBe(0);
  await page.evaluate(() => window.__tradeloomMock?.clearFailures());
  await notice.getByRole('button', { name: 'Retry' }).click();
  await expect(notice).toHaveCount(0);
});
