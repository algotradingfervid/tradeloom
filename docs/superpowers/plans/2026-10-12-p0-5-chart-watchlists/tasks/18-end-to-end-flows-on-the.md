> **P0-5 Chart, Watchlists & Ticker UI Implementation Plan — Task 18.** Read [`../MANIFEST.md`](../MANIFEST.md) first: its Global Constraints,
> Review Focus and Consumes/Produces apply to this task. Other tasks live beside this file in `tasks/`.

<!-- task body (verbatim from the plan) -->
### Task 18: End-to-end flows on the mock backend (Playwright)

**Files:**
- Create: `frontend/e2e/chart-watchlists.spec.ts`, `frontend/e2e/layout-1280.spec.ts`, `frontend/e2e/naming-consistency.spec.ts`
- Modify: `frontend/e2e/helpers.ts` (P0-4 Task 19: append `'/chart-full'` to `P0_VIEWS`, so the axe, overflow and no-B/S guards cover the full view)

**Interfaces:**
- Consumes: P0-4 `playwright.config.ts` (projects `chromium`, `webkit`; `webServer` runs `npm run dev:mock`, i.e. `vite --mode mock` with `VITE_BACKEND=mock`), `e2e/helpers.ts` (`openApp(page, query?)`, `go(page, route)`, `P0_VIEWS`), `window.__tradeloomMock.emit(name, payload)` (the `MockBackend` from `defaultBackend()`), seeds `?seed=personal` (ready session, `dataPlan: 'none'`). Task 2 mock market (snapshot LTP = base price, previous close = base × 0.995; NIFTY 50 base 22 500; mock "now" 09 Oct 2026 11:30 IST; live budget 200). Task 6 `tick` helper. The accessible names produced by Tasks 8–17.
- Produces: the `ui-e2e` CI coverage (P0-4 Task 19's job) for this plan's P0 exit-criteria ACs.

- [ ] **Step 1: Write the specs**

In `frontend/e2e/helpers.ts` (P0-4) append `'/chart-full'` to the `P0_VIEWS` array (before `'/no-such-view'`). `go(page, '/chart-full')` lands on NIFTY 50 at 1 Hour (no navigation state) and finds the view's `h1` "Full-view chart".

`frontend/e2e/chart-watchlists.spec.ts`:

```ts
import { expect, test, type Page } from '@playwright/test';
import { tick } from '../src/market/testing';
import { openApp } from './helpers';

const emit = (page: Page, name: string, payload: unknown) =>
  page.evaluate(([n, p]) => window.__tradeloomMock!.emit(n as never, p as never), [name, payload] as const);
const header = (page: Page) => page.getByRole('button', { name: 'Change symbol' });
const strip = (page: Page) => page.getByRole('group', { name: 'Index ticker' });
const panel = (page: Page) => page.getByRole('region', { name: 'Watchlist', exact: true });
const now = (t: ReturnType<typeof tick>) => ({ ...t, tsMs: Date.now() });

test.beforeEach(async ({ page }) => {
  await page.setViewportSize({ width: 1440, height: 900 });
  await openApp(page);
  await expect(page.getByRole('region', { name: /Price chart/ })).toBeVisible();
});

test('Home: NIFTY 50 at 1 Hour, three pills with the snapshot values, 8 timeframes', async ({ page }) => {
  await expect(header(page)).toContainText('NIFTY 50');
  await expect(header(page)).toContainText('INDICES');
  await expect(page.getByLabel('Timeframe')).toHaveValue('1h');
  await expect(page.getByLabel('Timeframe').locator('option')).toHaveText(['1 Minute', '3 Minutes', '5 Minutes', '10 Minutes', '15 Minutes', '30 Minutes', '1 Hour', '1 Day']);
  await expect(strip(page).getByRole('button')).toHaveCount(3);
  await expect(strip(page).getByRole('button', { name: /^NIFTY 50/ })).toContainText('22,500.00');
});

test('FR-SHELL-004 AC1: a tick updates the NIFTY 50 pill within 500 ms', async ({ page }) => {
  await emit(page, 'tick.batch', [now(tick(256265, 22600, 22500))]);
  await expect(strip(page).getByRole('button', { name: /^NIFTY 50/ })).toContainText('22,600.00', { timeout: 500 });
  await expect(strip(page).getByRole('button', { name: /^NIFTY 50/ })).toContainText('+100.00 (+0.44%)');
});

test('FR-SHELL-004 AC3: clicking SENSEX loads it on the Home chart', async ({ page }) => {
  await strip(page).getByRole('button', { name: /^SENSEX/ }).click();
  await expect(header(page)).toContainText('SENSEX');
});

test('FR-CHART-005 AC1/AC4/AC5: search RELIANCE (≤ 1 s), Escape keeps the symbol', async ({ page }) => {
  await header(page).click();
  const input = page.getByRole('combobox', { name: 'Search instruments' });
  await expect(input).toBeFocused();
  for (const chip of ['NIFTY 50', 'NIFTY BANK', 'SENSEX', 'BANKEX']) await expect(page.getByRole('button', { name: chip, exact: true })).toBeVisible();
  await expect(page.getByText('Type to search')).toBeVisible();
  await input.fill('reliance');
  await expect(page.getByRole('option', { name: /RELIANCE/ }).first()).toBeVisible();
  const t0 = Date.now();
  await input.press('Enter');
  await expect(header(page)).toContainText('RELIANCE');
  expect(Date.now() - t0).toBeLessThan(1000);
  await header(page).click();
  await page.keyboard.press('Escape');
  await expect(page.getByRole('combobox', { name: 'Search instruments' })).toHaveCount(0);
  await expect(header(page)).toContainText('RELIANCE');
});

test('P0 demo path: NIFTY OCT FUT + RELIANCE into a watchlist, ticks update rows, row click charts', async ({ page }) => {
  const search = panel(page).getByRole('combobox', { name: 'Search instruments to add' });
  await search.fill('nifty oct fut');
  await expect(panel(page).getByRole('option', { name: /NIFTY OCT FUT/ })).toBeVisible();
  await search.press('Enter');
  await search.fill('reliance');
  await expect(panel(page).getByRole('option', { name: /RELIANCE/ }).first()).toBeVisible();
  await search.press('Enter');
  await search.press('Escape');
  const fut = panel(page).getByRole('row', { name: 'NIFTY OCT FUT' });
  const rel = panel(page).getByRole('row', { name: 'RELIANCE' });
  await expect(fut).toBeVisible();
  await expect(fut).toContainText('F');
  await emit(page, 'tick.batch', [now(tick(13238530, 22650.1, 22600)), now(tick(738561, 1301, 1290))]);
  await expect(fut).toContainText('22,650.10');
  await expect(rel).toContainText('(+0.85%)');
  await fut.getByRole('button', { name: /^NIFTY OCT FUT/ }).click();
  await expect(header(page)).toContainText('NIFTY OCT FUT');
  await expect(fut).toHaveAttribute('aria-selected', 'true');
});

test('FR-CHART-003 AC2 (UI): a forming candle (coalesced candle.forming array) is drawn without reload', async ({ page }) => {
  await page.getByLabel('Timeframe').selectOption('5m');
  const legend = page.getByTestId('chart-legend');
  await expect(legend).toContainText('C ');
  await emit(page, 'candle.forming', [{
    token: 256265, interval: '5minute', partial: false,
    candle: { t: Date.UTC(2026, 9, 9, 6, 0), o: 22500, h: 22777.7, l: 22490, c: 22777.7, v: 0, oi: 0, complete: false },
  }]);
  await expect(legend).toContainText('C 22,777.70');
});

test('FR-CHART-017 AC3: full view renders outside the shell; F11 toggles between it and Home; Back returns', async ({ page }) => {
  await page.getByRole('button', { name: 'More chart actions' }).click();
  await page.getByRole('menuitem', { name: 'Open in full view' }).click();
  await expect(page.getByRole('button', { name: 'Back' })).toBeVisible();
  await expect(strip(page)).toHaveCount(0); // no top bar, no sidebar: the route lives in rootChildren
  await page.getByRole('region', { name: /Price chart/ }).focus();
  await page.keyboard.press('F11');
  await expect(strip(page)).toBeVisible();
  await page.getByRole('region', { name: /Price chart/ }).focus();
  await page.keyboard.press('F11');
  await expect(page.getByRole('button', { name: 'Back' })).toBeVisible();
  await page.getByRole('button', { name: 'Back' }).click();
  await expect(panel(page)).toBeVisible();
});

test('NFR-07: timeframe change paints < 100 ms during a 10 Hz × 250-token tick storm', async ({ page }) => {
  await page.evaluate(() => {
    const toks = [256265, 260105, 265, 738561, 779521];
    const w = window as unknown as { __storm?: ReturnType<typeof setInterval> };
    w.__storm = setInterval(() => window.__tradeloomMock!.emit('tick.batch',
      Array.from({ length: 250 }, (_, i) => {
        const ltp = 1000 + Math.random();
        return { token: toks[i % toks.length], ltp, close: 1000, change: ltp - 1000, changePct: (ltp - 1000) / 10, volume: 0, tsMs: Date.now(), polled: false };
      })), 100);
  });
  const worst = await page.evaluate(async () => {
    const sel = document.querySelector('select[aria-label="Timeframe"]') as HTMLSelectElement;
    const samples: number[] = [];
    for (let k = 0; k < 10; k++) {
      const t0 = performance.now();
      sel.value = k % 2 ? '1h' : '15m';
      sel.dispatchEvent(new Event('change', { bubbles: true }));
      await new Promise((r) => requestAnimationFrame(() => r(null)));
      samples.push(performance.now() - t0);
      await new Promise((r) => setTimeout(r, 150));
    }
    return Math.max(...samples);
  });
  await page.evaluate(() => clearInterval((window as unknown as { __storm?: ReturnType<typeof setInterval> }).__storm));
  expect(worst).toBeLessThan(100);
});

test('FR-CHART-003 AC4 (no data plan, ?seed=personal): data-plan message, no chart history', async ({ page }) => {
  await openApp(page, '?seed=personal');
  await expect(page.getByText('Charts need a Kite Connect data plan')).toBeVisible();
  const candleCalls = await page.evaluate(() => window.__tradeloomMock!.callsTo('chart.candles').length);
  expect(candleCalls).toBe(0);
});

test('FR-SHELL-011 AC2: the dark chart variables switch within 300 ms', async ({ page }) => {
  const before = await page.evaluate(() => getComputedStyle(document.documentElement).getPropertyValue('--chart-grid').trim());
  const ms = await page.evaluate(async () => {
    const t0 = performance.now();
    document.documentElement.setAttribute('data-theme', 'dark');
    await new Promise((r) => requestAnimationFrame(() => requestAnimationFrame(() => r(null))));
    return performance.now() - t0;
  });
  const after = await page.evaluate(() => getComputedStyle(document.documentElement).getPropertyValue('--chart-grid').trim());
  expect(after).not.toBe(before);
  expect(ms).toBeLessThan(300);
});
```

The last spec only shows that the chart-only variables switch cheaply. The full theme toggle goes through P0-4's profile-menu "Dark theme" toggle, which updates `useTheme().tokens` → `ChartView` → `adapter.setTheme(readChartTheme(tokens))`. That path is covered by the Task 10 unit test and checked by eye in demo step 11 (Task 19).

`frontend/e2e/layout-1280.spec.ts`:

```ts
import { expect, test } from '@playwright/test';
import { openApp } from './helpers';

for (const theme of ['light', 'dark'] as const) {
  test(`NFR-11 / FR-SHELL-001 AC2 / FR-HOME-001 AC3 at 1280×720 (${theme})`, async ({ page }) => {
    await page.setViewportSize({ width: 1280, height: 720 });
    await openApp(page);
    await page.evaluate((t) => document.documentElement.setAttribute('data-theme', t), theme);
    await expect(page.getByRole('region', { name: /Price chart/ })).toBeVisible();
    const overflow = await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth);
    expect(overflow).toBeLessThanOrEqual(0);
    for (const pill of await page.getByRole('group', { name: 'Index ticker' }).getByRole('button').all()) {
      const b = (await pill.boundingBox())!;
      expect(b.x).toBeGreaterThanOrEqual(0);
      expect(b.x + b.width).toBeLessThanOrEqual(1280);
    }
    // < 1440 px: pills show LTP and percent only (mock NIFTY 50 change is +112.50)
    await expect(page.getByRole('group', { name: 'Index ticker' }).getByRole('button', { name: /^NIFTY 50/ })).not.toContainText('+112.50');
    const header = page.getByRole('toolbar', { name: 'Chart controls' });
    const hb = (await header.boundingBox())!;
    expect(hb.x + hb.width).toBeLessThanOrEqual(1280);
    const panel = page.getByRole('region', { name: 'Watchlist', exact: true });
    await panel.scrollIntoViewIfNeeded();
    await expect(panel).toBeInViewport();
    // keyboard reachability: Tab reaches the symbol button, refresh and timeframe in order
    await expect(page.getByRole('button', { name: 'Refresh chart data' })).toBeEnabled(); // disabled buttons are skipped by Tab
    await page.getByRole('button', { name: 'Change symbol' }).focus();
    await page.keyboard.press('Tab');
    await expect(page.getByRole('button', { name: 'Refresh chart data' })).toBeFocused();
    await page.keyboard.press('Tab');
    await expect(page.getByLabel('Timeframe')).toBeFocused();
  });
}
```

`frontend/e2e/naming-consistency.spec.ts`:

```ts
import { expect, test, type Page } from '@playwright/test';
import { ALL_IN_SCOPE } from '../src/instruments/fixtures';
import { openApp } from './helpers';

type Seen = { token: string | null; tag: string | null; name: string | null; where: string };
const collect = (page: Page, where: string): Promise<Seen[]> =>
  page.$$eval('[data-instrument-token]', (els, w) => els.map((e) => ({
    token: e.getAttribute('data-instrument-token'),
    tag: e.getAttribute('data-exchange-tag'),
    name: (e.querySelector('[data-instrument-name]') ?? e).textContent,
    where: w,
  })), where);

test('FR-WATCH-013 AC3 + FR-WATCH-014 AC1/AC2: one formatter, one tag per instrument everywhere', async ({ page }) => {
  await page.setViewportSize({ width: 1440, height: 900 });
  await openApp(page);
  const panel = page.getByRole('region', { name: 'Watchlist', exact: true });
  const search = panel.getByRole('combobox', { name: 'Search instruments to add' });
  for (const q of ['reliance', 'nifty oct fut', 'sbin']) {
    await search.fill(q);
    await expect(panel.getByRole('option').first()).toBeVisible();
    await search.press('Enter');
  }
  await search.press('Escape');
  const seen: Seen[] = [...await collect(page, 'home')];

  await page.getByRole('button', { name: 'Change symbol' }).click();
  await page.getByRole('combobox', { name: 'Search instruments' }).fill('reliance');
  await expect(page.getByRole('option').first()).toBeVisible();
  seen.push(...await collect(page, 'chart-search'));
  await page.keyboard.press('Escape');

  await panel.getByRole('button', { name: 'Add instruments…' }).click();
  await page.getByRole('dialog').getByRole('button', { name: 'Watchlist 1' }).click();
  await expect(page.getByRole('dialog').getByRole('checkbox', { name: /^Select / }).first()).toBeVisible();
  seen.push(...await collect(page, 'picker'));

  const expected = new Map(ALL_IN_SCOPE.map((i) => [String(i.token), i.displayName]));
  const tags = new Map<string, Set<string>>();
  for (const s of seen) {
    expect(expected.get(s.token!), `unknown instrument rendered in ${s.where}`).toBeDefined();
    expect(s.name, `${s.token} in ${s.where}`).toBe(expected.get(s.token!));
    if (s.tag) tags.set(s.token!, (tags.get(s.token!) ?? new Set()).add(s.tag));
  }
  for (const [token, set] of tags) expect([...set], `token ${token}`).toHaveLength(1);
  const relianceNse = tags.get('738561');
  expect(relianceNse && [...relianceNse]).toEqual(['NSE']);
  expect(new Set(seen.filter((s) => s.token === '738561').map((s) => s.where))).toEqual(new Set(['home', 'chart-search', 'picker']));
});
```

The watchlist rows label their instrument from the `WatchlistItem` (`itemTag`), the search and picker rows from the `InstrumentDTO` (`tag`); this spec is what proves the two paths agree.

- [ ] **Step 2: Run them**

Run: `cd frontend && npx playwright test e2e/chart-watchlists.spec.ts e2e/layout-1280.spec.ts e2e/naming-consistency.spec.ts e2e/a11y.spec.ts --project=chromium --project=webkit --reporter=line`
Expected: all pass in both engines (13 tests of this plan × 2, plus P0-4's a11y spec now including `/chart-full`). If one fails, fix the component, never the assertion. These specs check acceptance criteria, and the only allowed edits are to accessible names that a sibling plan defines differently (record those in the consistency notes).

- [ ] **Step 3: Run the whole frontend gate (the CI `frontend` and `ui-e2e` jobs)**

Run: `cd frontend && npm run typecheck && npm run lint && npm test && npm run build && npx playwright test --reporter=line`
Expected: every command exits 0.

- [ ] **Step 4: Commit**

```bash
git add frontend/e2e/chart-watchlists.spec.ts frontend/e2e/layout-1280.spec.ts frontend/e2e/naming-consistency.spec.ts frontend/e2e/helpers.ts
git commit -m "$(cat <<'EOF'
test(e2e): P0 chart/watchlist/ticker flows, 1280 px layout and naming consistency on the mock backend

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
)"
```

---