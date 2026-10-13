import { expect, test, type Page } from '@playwright/test';
import { go, openApp } from './helpers';

interface Case { route: string; opener: (p: Page) => Promise<void>; name: string }

const button = (name: string) => (p: Page) => p.getByRole('button', { name, exact: true }).first().click();

const CASES: Case[] = [
  { route: '/home', opener: (p) => p.getByRole('button', { name: /^Market data status/ }).click(), name: 'Connection status' },
  { route: '/profile', opener: button('Log out'), name: 'Log out of Kite?' },
  { route: '/settings/kite', opener: button('Change'), name: 'Change Kite credentials' },
  { route: '/settings/kite', opener: button('Remove credentials'), name: 'Remove Kite credentials?' },
  { route: '/settings/data', opener: button('Clear cached candles'), name: 'Clear cached candles?' },
  { route: '/settings/data', opener: button('Restore from a backup…'), name: 'Restore this backup?' },
  { route: '/settings/shortcuts', opener: button('Change shortcut for Go to Home'), name: 'Change shortcut: Go to Home' },
  { route: '/settings/shortcuts', opener: button('Reset to defaults'), name: 'Reset to defaults' },
  { route: '/settings/calendar', opener: button('Add entry'), name: 'Add calendar entry' },
];

for (const c of CASES) {
  test(`FR-SHELL-015 AC2/AC3: "${c.name}" shows a close × and Escape closes it without applying anything`, async ({ page }) => {
    await openApp(page);
    await go(page, c.route);
    await c.opener(page);
    const dialog = page.getByRole('dialog', { name: c.name });
    await expect(dialog).toBeVisible();
    await expect(dialog.getByRole('button', { name: 'Close' })).toBeVisible();
    const callsWhileOpen = await page.evaluate(() => window.__tradeloomMock?.calls.length ?? -1);
    await page.keyboard.press('Escape');
    await expect(dialog).toBeHidden();
    expect(await page.evaluate(() => window.__tradeloomMock?.calls.length ?? -1)).toBe(callsWhileOpen);
  });
}
