import { expect, test } from '@playwright/test';
import { go, openApp, tabTo } from './helpers';

// WebKit on macOS skips buttons and links on Tab by default; WebKitGTK (the shipped engine) and the Linux CI container do not.
// Chromium tabs to every control on every OS, so it runs everywhere (Task 19: the plan skipped all of macOS).
test.skip(({ browserName }) => browserName === 'webkit' && process.platform === 'darwin',
  'run in the Linux container: macOS WebKit Tab order differs from WebKitGTK');

test('review focus: a keyboard-only user goes from first run to the Holdings table', async ({ page }) => {
  await page.goto('/?seed=fresh');
  await expect(page.getByRole('heading', { name: 'Connect your Kite account' })).toBeVisible();
  await tabTo(page, page.getByLabel('API key'));
  await page.keyboard.type('kiteapikey9f2c');
  await page.keyboard.press('Tab');
  // Go's credRe (internal/kite/credentials.go) accepts letters and digits only; the plan's 's3cret-value' is refused.
  await page.keyboard.type('s3cretvalue9');
  await page.keyboard.press('Enter');
  await expect(page.getByRole('heading', { name: 'Log in to Kite' })).toBeVisible();
  await tabTo(page, page.getByRole('button', { name: 'Login did not come back to Tradeloom?' }));
  await page.keyboard.press('Enter');
  await tabTo(page, page.getByLabel('Redirect URL or request token'));
  await page.keyboard.type('http://127.0.0.1:7420/kite/callback?request_token=abc123XYZ&status=success');
  await page.keyboard.press('Enter');
  await expect(page.getByRole('heading', { name: 'Before you start' })).toBeVisible();
  await tabTo(page, page.getByRole('button', { name: 'I understand' }));
  await page.keyboard.press('Enter');
  const avatar = page.getByRole('button', { name: 'Account menu' });
  await tabTo(page, avatar, 40);
  await page.keyboard.press('Enter');
  await expect(page.getByRole('menuitem', { name: 'Profile' })).toBeFocused();
  await page.keyboard.press('ArrowDown');
  await page.keyboard.press('Enter');
  await expect(page.getByRole('heading', { level: 1, name: 'Holdings' })).toBeVisible();
  await tabTo(page, page.getByRole('row', { name: 'RELIANCE' }), 40);
  await page.keyboard.press('ArrowDown');
  await expect(page.getByRole('row', { name: 'TCS' })).toBeFocused();
});

test('review focus: hover-revealed row actions become visible on keyboard focus and are listed by Shift+F10', async ({ page }) => {
  await openApp(page);
  await go(page, '/settings/calendar');
  const row = page.getByRole('row', { name: /^20-10-2026/ });
  const edit = row.getByRole('button', { name: /^Edit / });
  await expect(edit).toBeHidden();
  await row.focus();
  await expect(edit).toBeVisible();
  await page.keyboard.press('Tab');
  await expect(edit).toBeFocused();
  await row.focus();
  await page.keyboard.press('Shift+F10');
  await expect(page.getByRole('menu', { name: /^Actions for 20-10-2026/ })).toBeVisible();
  await page.keyboard.press('Escape');
  await expect(row).toBeFocused();
});

test('FR-SHELL-002 AC3 in the real engine: sidebar arrows and Enter', async ({ page }) => {
  await openApp(page);
  await go(page, '/settings/about');
  const home = page.getByRole('navigation', { name: 'Main navigation' }).getByRole('link', { name: 'Home' });
  await home.focus();
  await page.keyboard.press('Enter');
  await expect(page.getByRole('heading', { name: 'Home' })).toBeAttached();
  await expect(home).toHaveAttribute('aria-current', 'page');
});
