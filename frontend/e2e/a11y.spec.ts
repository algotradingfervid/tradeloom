import AxeBuilder from '@axe-core/playwright';
import { expect, test } from '@playwright/test';
import { go, openApp, P0_VIEWS } from './helpers';

const TAGS = ['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa'];

for (const theme of ['light', 'dark'] as const) {
  test(`FR-SHELL-011 AC3 + FR-SHELL-013 AC1 + NFR-11: no axe violations on any P0 view (${theme})`, async ({ page }) => {
    test.setTimeout(120_000);
    await openApp(page);
    if (theme === 'dark') {
      await page.getByRole('button', { name: 'Account menu' }).click();
      await page.getByRole('menuitemcheckbox', { name: 'Dark theme' }).click();
      await expect(page.locator('html')).toHaveAttribute('data-theme', 'dark');
    }
    const violations: string[] = [];
    for (const route of P0_VIEWS) {
      await go(page, route);
      const result = await new AxeBuilder({ page }).withTags(TAGS).analyze();
      violations.push(...result.violations.map((v) => `${route}: ${v.id} (${v.nodes.length})`));
    }
    expect(violations).toEqual([]);
  });
}

for (const seed of ['fresh', 'nokeyring', 'locked', 'loggedout', 'unacked']) {
  test(`no axe violations on the onboarding screen (${seed})`, async ({ page }) => {
    await page.goto(`/?seed=${seed}`);
    await expect(page.getByRole('heading', { level: 1 })).toBeVisible();
    const result = await new AxeBuilder({ page }).withTags(TAGS).analyze();
    expect(result.violations.map((v) => v.id)).toEqual([]);
  });
}
