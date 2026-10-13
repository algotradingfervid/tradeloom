import { expect, test } from '@playwright/test';

test('Settings → Data: Back up now and Restore against the mock job runner (FR-DATA-005 UI path)', async ({ page }) => {
  await page.goto('/');
  await expect(page.getByRole('button', { name: 'Account menu' })).toBeVisible();
  await page.evaluate(() => window.__tradeloomMock?.emit('app.navigate', { route: '/settings/data' }));
  await expect(page.getByText('700.0 MB of 10.0 GB')).toBeVisible();
  await page.getByRole('button', { name: 'Back up now' }).click();
  await expect(page.getByText(/^Backup saved to \/home\/asha\/Backups\//)).toBeVisible();
  await page.getByRole('button', { name: 'Restore from a backup…' }).click();
  const dialog = page.getByRole('dialog', { name: 'Restore this backup?' });
  await dialog.getByLabel('Type RESTORE to confirm').fill('RESTORE');
  await dialog.getByRole('button', { name: 'Restore' }).click();
  await expect(page.getByText('Restore complete. Tradeloom is restarting…')).toBeVisible();
});
