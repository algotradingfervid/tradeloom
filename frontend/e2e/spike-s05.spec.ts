import { expect, test } from '@playwright/test';

/* eslint-disable @typescript-eslint/no-explicit-any */
declare global { interface Window { __s05: any } }

test.describe('S0-5 lightweight-charts 5.2.1 go/no-go', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/spike/s05.html');
    await page.waitForFunction(() => window.__s05?.ready === true);
  });

  test('G1+G2 candles, volume pane and a separate study pane', async ({ page }) => {
    const panes = await page.evaluate(() => window.__s05.chart().panes().map((p: any) => p.getSeries().map((s: any) => s.seriesType())));
    expect(panes).toEqual([['Candlestick'], ['Histogram'], ['Line']]);
  });

  test('G3 custom primitive is drawn', async ({ page }) => {
    expect(await page.evaluate(() => window.__s05.bandVisible())).toBe(true);
  });

  test('G4 theme switch repaints within 300 ms (FR-SHELL-011 AC2)', async ({ page }) => {
    const ms = await page.evaluate(() => window.__s05.switchTheme());
    expect(ms).toBeLessThan(300);
    const [bg, token] = await page.evaluate(() => [
      window.__s05.chart().options().layout.background.color,
      getComputedStyle(document.documentElement).getPropertyValue('--chart-bg').trim(),
    ]);
    expect(bg).toBe(token);
  });

  test('G5 lazy history prepends without moving the visible window', async ({ page }) => {
    await page.evaluate(() => window.__s05.chart().timeScale().scrollToPosition(-1950, false));
    await page.waitForFunction(() => window.__s05.metrics.prepends >= 1);
    expect(await page.evaluate(() => window.__s05.metrics.lastPrependKeptView)).toBe(true);
  });

  test('G6 performance budget', async ({ page }) => {
    const m = await page.evaluate(() => window.__s05.metrics);
    console.log(`S05 metrics ${test.info().project.name}: ${JSON.stringify(m)}`);
    expect(m.setCandles20kMs).toBeLessThan(250);
    expect(m.update1000Ms).toBeLessThan(200);
  });
});
