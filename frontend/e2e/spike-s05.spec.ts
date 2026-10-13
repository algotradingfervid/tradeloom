import { expect, test } from '@playwright/test';

/* eslint-disable @typescript-eslint/no-explicit-any */
declare global { interface Window { __s05: any } }

test.describe('S0-5 lightweight-charts 5.2.1 go/no-go', () => {
  // Real-render go/no-go: run by hand in P0-5 Task 19 (S05=1), not in the ui-e2e CI job (Ruling 110 I7).
  test.skip(!!process.env.CI && !process.env.S05, 'S0-5 spike: run in Task 19, not CI');

  test.beforeEach(async ({ page }) => {
    await page.goto('/spike/s05.html');
    await page.waitForFunction(() => window.__s05?.ready === true);
  });

  test('G1+G2 candles, volume pane and a separate study pane', async ({ page }) => {
    const panes = await page.evaluate(() => window.__s05.chart().panes().map((p: any) => p.getSeries().map((s: any) => s.seriesType())));
    expect(panes).toEqual([['Candlestick'], ['Histogram'], ['Line']]);
  });

  test('G3 custom primitive is drawn', async ({ page }) => {
    const visible = await page.evaluate(() => window.__s05.bandVisible());
    const c = await page.evaluate(() => window.__s05.bandCheck);
    console.log(`S05 G3 ${test.info().project.name}: ${c.changedInBand}/${c.inBandRows} in-band rows changed, ${c.changedOutside} outside`);
    expect(visible).toBe(true);
  });

  test('G3 check fails when the band paints nothing (negative control)', async ({ page }) => {
    const visible = await page.evaluate(() => {
      const { adapter, band } = window.__s05;
      adapter.removePriceBand({ id: band.id });
      adapter.addPriceBand({ ...band, color: 'rgba(0, 0, 0, 0)' });
      return window.__s05.bandVisible();
    });
    expect(visible).toBe(false);
  });

  test('G4 theme switch repaints within 300 ms (FR-SHELL-011 AC2)', async ({ page }) => {
    const ms = await page.evaluate(() => window.__s05.switchTheme());
    console.log(`S05 G4 ${test.info().project.name}: theme switch ${ms} ms`);
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
    const m = await page.evaluate(() => window.__s05.metrics);
    console.log(`S05 G5 ${test.info().project.name}: anchor shift sync ${m.lastPrependSyncShiftPx}px, 1 frame ${m.lastPrependShiftPx}px, `
      + `2 frames ${m.lastPrependShift2FramesPx}px, to kept ${m.lastPrependToKept}, `
      + `frame 1 painted ${m.lastPrependFramePainted}, frame 2 repainted ${m.lastPrependRepainted2}, `
      + `frame 1 = settled ${m.lastPrependFirstFrameSettled}`);
    expect(m.lastPrependKeptView).toBe(true); // anchor bar's x moved by at most 0.5 px
    expect(Math.abs(m.lastPrependShift2FramesPx)).toBeLessThanOrEqual(0.5);
    // One-frame timing: the pane was repainted in the first frame after the prepend, and that frame already equals a
    // forced full repaint of the settled state (no intermediate frame with the old logical range on the new data).
    expect(m.lastPrependFramePainted).toBe(true);
    expect(m.lastPrependRepainted2).toBe(true);
    expect(m.lastPrependFirstFrameSettled).toBe(true);
  });

  test('G5 one-frame check catches a wrong intermediate frame (negative control)', async ({ page }) => {
    // Simulate a broken prepend: the old (unshifted) logical range is applied at once and corrected one frame later.
    await page.evaluate(() => {
      const ts = window.__s05.chart().timeScale();
      const orig = ts.setVisibleLogicalRange.bind(ts);
      ts.setVisibleLogicalRange = (r: { from: number; to: number }) => {
        orig({ from: r.from - 500, to: r.to - 500 });
        requestAnimationFrame(() => orig(r));
      };
      ts.scrollToPosition(-1950, false);
    });
    await page.waitForFunction(() => window.__s05.metrics.prepends >= 1);
    const m = await page.evaluate(() => window.__s05.metrics);
    expect(m.lastPrependFramePainted).toBe(true);
    expect(m.lastPrependFirstFrameSettled).toBe(false);
    expect(m.lastPrependKeptView).toBe(false);
  });

  test('G6 performance budget', async ({ page }) => {
    const m = await page.evaluate(() => window.__s05.metrics);
    console.log(`S05 metrics ${test.info().project.name}: ${JSON.stringify(m)}`);
    expect(m.setCandles20kMs).toBeLessThan(250);
    expect(m.update1000Ms).toBeLessThan(200);
  });
});
