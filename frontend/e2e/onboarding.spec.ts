import { expect, test } from '@playwright/test';

test('first run: setup → paste login → risk disclosure → Home (FR-ACCT-005 AC1, FR-ACCT-006 AC2, FR-SHELL-020 AC1)', async ({ page }) => {
  await page.goto('/?seed=fresh');
  await expect(page.getByRole('heading', { name: 'Connect your Kite account' })).toBeVisible();
  await page.getByLabel('API key').fill('kiteapikey9f2c');
  await page.getByLabel('API secret').fill('s3cret-value');
  await page.getByRole('button', { name: 'Save and continue' }).click();
  await expect(page.getByRole('heading', { name: 'Log in to Kite' })).toBeVisible();
  await page.getByRole('button', { name: 'Login did not come back to Tradeloom?' }).click();
  await page.getByLabel('Redirect URL or request token').fill('http://127.0.0.1:7420/kite/callback?request_token=abc123XYZ&status=success');
  await page.getByRole('button', { name: 'Complete login' }).click();
  await expect(page.getByRole('heading', { name: 'Before you start' })).toBeVisible();
  await page.getByRole('button', { name: 'I understand' }).click();
  await expect(page.getByRole('heading', { name: 'Home' })).toBeAttached();
  await expect(page.getByRole('button', { name: /^Market data status: Live/ })).toBeVisible();
});

test('loopback login completes from the browser callback (FR-ACCT-006 AC1, UI part)', async ({ page }) => {
  await page.goto('/?seed=loggedout');
  await page.getByRole('button', { name: 'Log in with Kite' }).click();
  await expect(page.getByText(/Waiting for the Kite login/)).toBeVisible();
  await page.evaluate(() => window.__tradeloomMock?.simulateBrowserLogin());
  await expect(page.getByRole('heading', { name: 'Before you start' })).toBeVisible();
});
