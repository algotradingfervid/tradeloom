> **P0-4 Frontend Shell, Account & Data UI Implementation Plan — Task 19.** Read [`../MANIFEST.md`](../MANIFEST.md) first: its Global Constraints,
> Review Focus and Consumes/Produces apply to this task. Other tasks live beside this file in `tasks/`.

<!-- task body (verbatim from the plan) -->
### Task 19: End-to-end sweep — accessibility, dialogs, keyboard-only path, theme, P0 guards

**Files:**
- Create: `frontend/e2e/helpers.ts`, `frontend/e2e/a11y.spec.ts`, `frontend/e2e/dialogs.spec.ts`, `frontend/e2e/keyboard.spec.ts`, `frontend/e2e/guards.spec.ts`
- Modify: `.github/workflows/ci.yml` (add the `ui-e2e` job; add `npm run lint` to the P0-1 `frontend` job)
- Test: the files above (Playwright, webkit + chromium projects, mock backend via `npm run dev:mock`)

**Interfaces:**
- Consumes: every view of Tasks 1–18; `window.__tradeloomMock` (`emit`, `calls`, `state`, `simulateBrowserLogin`) and `?seed=<name>` incl. `personal` from Task 2; `@axe-core/playwright` 4.13.0 `AxeBuilder({ page }).withTags([...]).analyze()`; P0-1 `.github/workflows/ci.yml` (`env.NODE_VERSION`, the `frontend` job, `actions/checkout@v7`, `actions/setup-node@v7`, `actions/upload-artifact@v7`).
- Produces: `P0_VIEWS`, `openApp(page, query?)`, `go(page, route)`, `tabTo(page, locator, max?)` in `e2e/helpers.ts`; CI job `ui-e2e` (P0-1 has none — this task adds it) running these specs against `VITE_BACKEND=mock`. P0-5 adds its chart/watchlist views to `P0_VIEWS`.

- [ ] **Step 1: Write the helpers and the specs**

Create `frontend/e2e/helpers.ts`:

```ts
import { expect, type Locator, type Page } from '@playwright/test';

export const P0_VIEWS = [
  '/home', '/holdings', '/positions', '/orders', '/profile',
  '/settings/kite', '/settings/appearance', '/settings/chart', '/settings/calendar', '/settings/data',
  '/settings/shortcuts', '/settings/diagnostics', '/settings/about',
  '/help', '/help/kite-setup', '/help/keyboard-shortcuts', '/legal/terms', '/legal/privacy', '/legal/risk', '/no-such-view',
];

export async function openApp(page: Page, query = ''): Promise<void> {
  await page.goto(`/${query}`);
  await expect(page.getByRole('button', { name: 'Account menu' })).toBeVisible();
}

/** In-app navigation through the backend's `app.navigate` event (the memory router has no URL bar). */
export async function go(page: Page, route: string): Promise<void> {
  await page.evaluate((r) => window.__tradeloomMock?.emit('app.navigate', { route: r }), route);
  await page.evaluate(() => new Promise((r) => requestAnimationFrame(() => requestAnimationFrame(r))));
  await expect(page.getByRole('heading', { level: 1 }).first()).toBeAttached();
}

export async function tabTo(page: Page, target: Locator, max = 30): Promise<void> {
  for (let i = 0; i < max; i++) {
    if (await target.evaluate((el) => el === document.activeElement)) return;
    await page.keyboard.press('Tab');
  }
  await expect(target).toBeFocused();
}
```

Create `frontend/e2e/a11y.spec.ts`:

```ts
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
```

Create `frontend/e2e/dialogs.spec.ts`:

```ts
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
```

Create `frontend/e2e/keyboard.spec.ts`:

```ts
import { expect, test } from '@playwright/test';
import { go, openApp, tabTo } from './helpers';

// WebKit on macOS skips buttons and links on Tab by default; WebKitGTK (the shipped engine) and the Linux CI container do not.
test.skip(process.platform === 'darwin', 'run in the Linux container: macOS WebKit Tab order differs from WebKitGTK');

test('review focus: a keyboard-only user goes from first run to the Holdings table', async ({ page }) => {
  await page.goto('/?seed=fresh');
  await expect(page.getByRole('heading', { name: 'Connect your Kite account' })).toBeVisible();
  await tabTo(page, page.getByLabel('API key'));
  await page.keyboard.type('kiteapikey9f2c');
  await page.keyboard.press('Tab');
  await page.keyboard.type('s3cret-value');
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
```

Create `frontend/e2e/guards.spec.ts`:

```ts
import { expect, test } from '@playwright/test';
import { go, openApp, P0_VIEWS } from './helpers';

test('FR-SHELL-011 AC2: dark theme applies within 300 ms and survives a restart', async ({ page }) => {
  await openApp(page, '?persist=1');
  await page.getByRole('button', { name: 'Account menu' }).click();
  const elapsed = await page.evaluate(async () => {
    const toggle = document.querySelector<HTMLElement>('[role="menuitemcheckbox"]');
    const t0 = performance.now();
    toggle?.click();
    await new Promise<void>((resolve) => {
      const check = () => (document.documentElement.dataset.theme === 'dark' ? resolve() : requestAnimationFrame(check));
      check();
    });
    return performance.now() - t0;
  });
  expect(elapsed).toBeLessThan(300);
  await page.reload();
  await expect(page.locator('html')).toHaveAttribute('data-theme', 'dark');
});

test('NFR-11 + FR-SHELL-001 AC2: every P0 view fits 1280×720 without a horizontal scrollbar', async ({ page }) => {
  await page.setViewportSize({ width: 1280, height: 720 });
  await openApp(page);
  for (const route of P0_VIEWS) {
    await go(page, route);
    const overflow = await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth);
    expect(overflow, route).toBe(0);
  }
});

test('Global constraint: a P0 build renders no Buy/Sell order buttons anywhere', async ({ page }) => {
  await openApp(page);
  for (const route of P0_VIEWS) {
    await go(page, route);
    await expect(page.getByRole('button', { name: /^(B|S|Buy|Sell)$/ }), route).toHaveCount(0);
  }
});

test('FR-ACCT-007 AC1 (UI): a Personal-plan key (?seed=personal) shows "No data plan" while the portfolio still works', async ({ page }) => {
  await openApp(page, '?seed=personal');
  await expect(page.getByRole('button', { name: /^Market data status: No data plan/ })).toBeVisible();
  await go(page, '/holdings');
  await expect(page.getByRole('row', { name: 'RELIANCE' })).toBeVisible();
});

test('FR-ACCT-006 AC3 (UI): the 06:00 expiry event shows the banner and "Logged out" without leaving the view', async ({ page }) => {
  await openApp(page);
  await go(page, '/holdings');
  await page.evaluate(() => {
    const m = window.__tradeloomMock;
    if (m) m.emit('session.state', { ...m.state.session, state: 'expired' });
  });
  await expect(page.getByText('Kite session expired — log in to continue')).toBeVisible();
  await expect(page.getByRole('button', { name: /^Market data status: Logged out/ })).toBeVisible();
  await expect(page.getByRole('heading', { level: 1, name: 'Holdings' })).toBeVisible();
});
```

- [ ] **Step 2: Run the specs**

Run: `cd frontend && VITE_BACKEND=mock npx playwright test`
Expected: PASS in both projects (webkit, chromium) — all specs of Tasks 1, 8, 9, 11, 12, 16 and this task (`shell` 1, `onboarding` 2, `portfolio-layout` 3, `home-layout` 1, `settings-data` 1, `a11y` 7, `dialogs` 9, `keyboard` 3 on Linux / skipped on macOS, `guards` 5). If axe reports a violation, fix the component (never disable the rule) and re-run.

- [ ] **Step 3: Add the `ui-e2e` CI job and lint the frontend in CI**

P0-1's `.github/workflows/ci.yml` has no Playwright job. In the existing `frontend` job insert `- run: npm run lint` directly after `- run: npm run typecheck`, then add this job under `jobs:` directly after the `frontend` job (before `licence`):

```yaml
  ui-e2e:
    runs-on: ubuntu-24.04
    defaults:
      run:
        working-directory: frontend
    steps:
      - uses: actions/checkout@v7
      - uses: actions/setup-node@v7
        with:
          node-version: ${{ env.NODE_VERSION }}
          cache: npm
          cache-dependency-path: frontend/package-lock.json
      - run: npm ci
      - name: Playwright browsers (chromium + WebKit, with their system libraries)
        run: npx playwright install --with-deps chromium webkit
      - name: Playwright against the mock backend (playwright.config.ts starts `npm run dev:mock`)
        run: VITE_BACKEND=mock npx playwright test
      - if: failure()
        uses: actions/upload-artifact@v7
        with:
          name: playwright-report
          path: frontend/playwright-report
          retention-days: 14
```

`upload-artifact` paths are relative to the repository root (the `working-directory` default applies only to `run` steps), hence `frontend/playwright-report`; `playwright.config.ts` (Task 1) writes the HTML report there and `reuseExistingServer` is off because GitHub sets `CI=true`.

Run: `python3 -c 'import sys,yaml; d=yaml.safe_load(open(sys.argv[1])); j=d["jobs"]["ui-e2e"]; print(j["runs-on"], [s.get("run", s.get("uses")) for s in j["steps"]]); print("lint" in str(d["jobs"]["frontend"]["steps"]))' .github/workflows/ci.yml`
Expected: `ubuntu-24.04 ['actions/checkout@v7', 'actions/setup-node@v7', 'npm ci', 'npx playwright install --with-deps chromium webkit', 'VITE_BACKEND=mock npx playwright test', 'actions/upload-artifact@v7']` and `True`.

- [ ] **Step 4: Run the whole frontend gate as CI does**

Run: `cd frontend && npm run typecheck && npm run lint && npx vitest run && npm run build`
Expected: all four commands exit 0; Vitest reports 45 passing test files.

- [ ] **Step 5: Commit**

```bash
git add frontend/e2e .github/workflows/ci.yml
git commit -m "test(frontend): P0-4 end-to-end sweep and ui-e2e CI job — axe, dialogs, keyboard-only path, theme, P0 guards (P0-4 Task 19)" \
  -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---
