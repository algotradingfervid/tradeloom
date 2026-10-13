import { expect, test, type Page } from '@playwright/test';
import { tick } from '../src/market/testing';
import { openApp } from './helpers';

const emit = (page: Page, name: string, payload: unknown) =>
  page.evaluate(([n, p]) => window.__tradeloomMock!.emit(n as never, p as never), [name, payload] as const);
const header = (page: Page) => page.getByRole('button', { name: 'Change symbol' });
const strip = (page: Page) => page.getByRole('group', { name: 'Index ticker' });
const panel = (page: Page) => page.getByRole('region', { name: 'Watchlist', exact: true });
const now = (t: ReturnType<typeof tick>) => ({ ...t, tsMs: Date.now() });
// Pick a search row by instrument token, not by position: search ranking against the mock is not asserted (Ruling 87 M4).
const optionFor = (page: Page, token: number) =>
  panel(page).getByRole('option').filter({ has: page.locator(`[data-instrument-token="${token}"]`) });

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
  const input = page.getByRole('combobox', { name: 'Search instruments', exact: true });
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
  await expect(page.getByRole('combobox', { name: 'Search instruments', exact: true })).toHaveCount(0);
  await expect(header(page)).toContainText('RELIANCE');
});

test('P0 demo path: NIFTY OCT FUT + RELIANCE into a watchlist, ticks update rows, row click charts', async ({ page }) => {
  const search = panel(page).getByRole('combobox', { name: 'Search instruments to add' });
  await search.fill('nifty oct fut');
  await expect(optionFor(page, 13238530)).toBeVisible();
  await optionFor(page, 13238530).click();
  await search.fill('reliance');
  await expect(optionFor(page, 738561)).toBeVisible();
  await optionFor(page, 738561).click();
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

// NFR-07 (P0-5 Task 18 plan text, carried by Task 19). Additions to the plan text, all stricter: each change must reach
// the backend (one more chart.candles call) so a no-op cannot pass, the storm must reach the UI (NIFTY 50 pill shows a
// storm LTP), and the time runs to the first task after the frame's paint instead of to the rAF callback.
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
  try {
    await expect(strip(page).getByRole('button', { name: /^NIFTY 50/ })).toContainText(/1,000\.\d\d/);
    const run = await page.evaluate(async () => {
      const sel = document.querySelector('select[aria-label="Timeframe"]') as HTMLSelectElement;
      const candleCalls = () => window.__tradeloomMock!.callsTo('chart.candles').length;
      const afterPaint = () => new Promise((r) => requestAnimationFrame(() => {
        const ch = new MessageChannel();
        ch.port1.onmessage = () => r(null);
        ch.port2.postMessage(null);
      }));
      const samples: number[] = [];
      let applied = 0;
      for (let k = 0; k < 10; k++) {
        const before = candleCalls();
        const t0 = performance.now();
        sel.value = k % 2 ? '1h' : '15m';
        sel.dispatchEvent(new Event('change', { bubbles: true }));
        await afterPaint();
        samples.push(performance.now() - t0);
        await new Promise((r) => setTimeout(r, 150));
        if (candleCalls() > before && sel.value === (k % 2 ? '1h' : '15m')) applied++;
      }
      return { samples, applied };
    });
    console.log(`NFR-07 ${test.info().project.name}: worst ${Math.max(...run.samples).toFixed(1)} ms, samples ${run.samples.map((s) => s.toFixed(1)).join(', ')}`);
    expect(run.applied).toBe(10);
    expect(Math.max(...run.samples)).toBeLessThan(100);
  } finally {
    await page.evaluate(() => clearInterval((window as unknown as { __storm?: ReturnType<typeof setInterval> }).__storm));
  }
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

// Checks the P0-5 ledger hands to e2e: behaviour jsdom cannot show, run in both real engines.
test.describe('real-engine keyboard and layout checks (P0-5 ledger)', () => {
  const addToWatchlist = async (page: Page, query: string, token: number) => {
    await panel(page).getByRole('combobox', { name: 'Search instruments to add' }).fill(query);
    await expect(optionFor(page, token)).toBeVisible();
    await optionFor(page, token).click();
  };
  const openPicker = async (page: Page) => {
    await panel(page).getByRole('button', { name: 'Add instruments…' }).click();
    const dialog = page.getByRole('dialog', { name: 'Add to Watchlist 1' });
    await expect(dialog).toBeVisible();
    return dialog;
  };

  test('Task 17: Space on a picker row toggles it once (no double toggle by the native checkbox)', async ({ page }) => {
    const dialog = await openPicker(page);
    await dialog.getByLabel('Search instruments', { exact: true }).fill('reliance');
    const first = dialog.getByRole('grid', { name: 'Instruments' }).getByRole('checkbox').first();
    await expect(first).toBeVisible();
    await dialog.getByLabel('Search instruments', { exact: true }).press('ArrowDown');
    await expect(first).toBeFocused();
    await page.keyboard.press('Space');
    await expect(first).toBeChecked();
    await expect(dialog.getByRole('heading', { name: /^Selected \(1\b/ })).toBeVisible();
    await page.keyboard.press('Space');
    await expect(first).not.toBeChecked();
    await expect(dialog.getByRole('heading', { name: /^Selected \(0\b/ })).toBeVisible();
    await page.keyboard.press('Enter');
    await expect(first).toBeChecked();
    await expect(first).toBeFocused();
  });

  for (const vp of [{ width: 1280, height: 720 }, { width: 1440, height: 900 }]) {
    test(`Task 17: picker panes share the dialog width and nothing overflows sideways at ${vp.width}×${vp.height}`, async ({ page }) => {
      await page.setViewportSize(vp);
      await addToWatchlist(page, 'reliance', 738561);
      const dialog = await openPicker(page);
      await dialog.getByLabel('Search instruments', { exact: true }).fill('reliance');
      await expect(dialog.getByRole('grid', { name: 'Instruments' }).getByRole('checkbox').first()).toBeVisible();
      await dialog.getByRole('grid', { name: 'Instruments' }).getByRole('checkbox').first().check();
      const box = (await dialog.boundingBox())!;
      expect(box.x).toBeGreaterThanOrEqual(0);
      expect(box.x + box.width).toBeLessThanOrEqual(vp.width);
      const panes = await dialog.evaluate((d) => ['.picker__left', '.picker__right'].map((s) => {
        const el = d.querySelector<HTMLElement>(s)!;
        const r = el.getBoundingClientRect();
        return { left: r.left, right: r.right, width: r.width, overflow: el.scrollWidth - el.clientWidth };
      }));
      const [left, right] = panes;
      expect(Math.abs(left.width - right.width)).toBeLessThanOrEqual(1); // two equal columns
      for (const p of panes) {
        expect(p.width).toBeGreaterThanOrEqual(300); // room for symbol, tag and a truncated company name
        expect(p.overflow).toBeLessThanOrEqual(0);
        expect(p.left).toBeGreaterThanOrEqual(box.x);
        expect(p.right).toBeLessThanOrEqual(box.x + box.width);
      }
      // Every result and selected row stays inside its pane (the company name truncates instead of pushing out).
      const rowsOut = await dialog.evaluate((d) => {
        const out: string[] = [];
        for (const [pane, row] of [['.picker__left', '.picker__row'], ['.picker__right', '.picker__selected-row']]) {
          const pr = d.querySelector(pane)!.getBoundingClientRect();
          d.querySelectorAll<HTMLElement>(`${pane} ${row}`).forEach((el) => {
            const r = el.getBoundingClientRect();
            if (r.left < pr.left - 0.5 || r.right > pr.right + 0.5 || el.scrollWidth > el.clientWidth) out.push(el.textContent ?? '');
          });
        }
        return out;
      });
      expect(rowsOut).toEqual([]);
    });
  }

  test('Task 11: Escape on the full view timeframe select (list closed) returns to Home', async ({ page }) => {
    await page.getByRole('button', { name: 'More chart actions' }).click();
    await page.getByRole('menuitem', { name: 'Open in full view' }).click();
    await expect(page.getByRole('button', { name: 'Back' })).toBeVisible();
    const tf = page.getByLabel('Timeframe');
    await tf.focus();
    await expect(tf).toBeFocused();
    await page.keyboard.press('Escape');
    await expect(page.getByRole('button', { name: 'Back' })).toHaveCount(0);
    await expect(panel(page)).toBeVisible();
    await expect(page.getByRole('button', { name: 'More chart actions' })).toBeFocused(); // focus returns to the opener
  });

  test('Task 14: keyboard focus follows a watchlist row moved with Alt+ArrowDown / Alt+ArrowUp', async ({ page }) => {
    await addToWatchlist(page, 'reliance', 738561);
    await addToWatchlist(page, 'sbin', 779521);
    await addToWatchlist(page, 'nifty oct fut', 13238530);
    await panel(page).getByRole('combobox', { name: 'Search instruments to add' }).press('Escape');
    const rows = panel(page).locator('tbody tr[data-token]');
    await expect(rows).toHaveCount(3);
    const order = () => rows.evaluateAll((els) => els.map((e) => e.getAttribute('data-token')));
    expect(await order()).toEqual(['738561', '779521', '13238530']);
    const rel = panel(page).locator('tr[data-token="738561"]');
    await rel.focus();
    await page.keyboard.press('Alt+ArrowDown');
    await expect.poll(order).toEqual(['779521', '738561', '13238530']);
    await expect(rel).toBeFocused();
    await expect(panel(page).getByRole('status').filter({ hasText: /2 of 3/ })).toHaveCount(1);
    await page.keyboard.press('Alt+ArrowDown');
    await expect.poll(order).toEqual(['779521', '13238530', '738561']);
    await expect(rel).toBeFocused();
    await page.keyboard.press('Alt+ArrowUp');
    await expect.poll(order).toEqual(['779521', '738561', '13238530']);
    await expect(rel).toBeFocused();
  });

  test('Task 7: an out-of-scope watchlist item carries data-out-of-scope, no data-exchange-tag, and the View only flag', async ({ page }) => {
    await emit(page, 'watchlists.changed', {
      activeId: 'wl-1',
      lists: [{ id: 'wl-1', name: 'Watchlist 1', items: [
        { token: 738561, exchange: 'NSE', symbol: 'RELIANCE', displayName: 'RELIANCE', name: 'RELIANCE INDUSTRIES', isIndex: false, missing: false },
        { token: 53505799, exchange: 'MCX', symbol: 'GOLDM26OCTFUT', displayName: 'GOLDM OCT FUT', name: 'GOLDM', isIndex: false, missing: false },
      ] }],
    });
    const label = panel(page).locator('[data-instrument-token="53505799"]');
    await expect(label).toHaveAttribute('data-out-of-scope', 'true');
    await expect(label).not.toHaveAttribute('data-exchange-tag');
    await expect(label).toContainText('View only');
    await expect(label).toContainText('MCX');
    const inScope = panel(page).locator('[data-instrument-token="738561"]');
    await expect(inScope).toHaveAttribute('data-exchange-tag', 'NSE');
    await expect(inScope).not.toHaveAttribute('data-out-of-scope');
  });
});
