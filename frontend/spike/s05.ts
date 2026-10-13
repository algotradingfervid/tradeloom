import type { IChartApi } from 'lightweight-charts';
import '../src/design/tokens.css';
import { applyTheme } from '../src/app/theme';
import { colourTokens, type ThemeName } from '../src/design/tokens';
import { LightweightChartsAdapter } from '../src/chart/LightweightChartsAdapter';
import { realLwc, type LwcLib } from '../src/chart/lwc';
import { readChartTheme } from '../src/chart/theme';
import { toChartTime } from '../src/chart/time';
import type { Bar, StudyPoint } from '../src/chart/types';

const FIVE_MIN = 300_000;
const END = Date.UTC(2026, 9, 9, 10, 0);
const token = (name: string) => getComputedStyle(document.documentElement).getPropertyValue(name).trim();

function synth(endMs: number, count: number, seed: number): Bar[] {
  const out: Bar[] = [];
  let p = 1000 + seed;
  for (let i = count; i > 0; i--) {
    const t = endMs - i * FIVE_MIN;
    const d = Math.sin((t / FIVE_MIN) * 0.21 + seed) * 4;
    const o = p; const c = p + d;
    out.push({ time: t, open: o, high: Math.max(o, c) + 1, low: Math.min(o, c) - 1, close: c, volume: 1000 + (i % 37) * 25 });
    p = c;
  }
  return out;
}

function rsi(bars: Bar[], n = 14): StudyPoint[] {
  let gain = 0; let loss = 0;
  return bars.map((b, i) => {
    if (i === 0) return { time: b.time, value: Number.NaN };
    const ch = b.close - bars[i - 1].close;
    const g = Math.max(ch, 0); const l = Math.max(-ch, 0);
    if (i <= n) { gain += g / n; loss += l / n; return { time: b.time, value: i < n ? Number.NaN : 100 - 100 / (1 + gain / (loss || 1e-9)) }; }
    gain = (gain * (n - 1) + g) / n; loss = (loss * (n - 1) + l) / n;
    return { time: b.time, value: 100 - 100 / (1 + gain / (loss || 1e-9)) };
  });
}

let chartRef: IChartApi | null = null;
const lib: LwcLib = { ...realLwc, createChart: ((c, o) => (chartRef = realLwc.createChart(c, o))) as LwcLib['createChart'] };
const adapter = new LightweightChartsAdapter(document.getElementById('chart')!, { lib });
applyTheme('light');
adapter.setTheme(readChartTheme(colourTokens.light));

const metrics = {
  setCandles20kMs: 0, update1000Ms: 0, themeSwitchMs: 0, prepends: 0,
  lastPrependKeptView: false, lastPrependShiftPx: Number.NaN, lastPrependToKept: false,
  lastPrependSyncShiftPx: Number.NaN, lastPrependShift2FramesPx: Number.NaN,
  lastPrependFramePainted: false, lastPrependRepainted2: false, lastPrependFirstFrameSettled: false,
};

let t0 = performance.now();
adapter.setCandles(synth(END, 20_000, 1));
metrics.setCandles20kMs = performance.now() - t0;

const bars = synth(END, 2000, 1);
adapter.setCandles(bars);
adapter.addStudy({ id: 'rsi', kind: 'line', pane: 'separate', color: token('--chart-crosshair'), points: rsi(bars) });
const lo = Math.min(...bars.slice(-100).map((b) => b.low));
const hi = Math.max(...bars.slice(-100).map((b) => b.high));
const bandFrom = lo + (hi - lo) * 0.4;
const bandTo = lo + (hi - lo) * 0.6;
const band = { id: 'band', from: bandFrom, to: bandTo, color: token('--chart-band') };
adapter.addPriceBand(band);

// raw library update cost for the forming bar (Task 4 wraps this in updateLastBar)
const candleSeries = () => chartRef!.panes()[0].getSeries()[0];
const last = bars[bars.length - 1];
t0 = performance.now();
for (let i = 0; i < 1000; i++) {
  candleSeries().update({ time: (Math.floor(last.time / 1000) + 19_800) as never, open: last.open, high: last.high + i * 0.01, low: last.low, close: last.close + i * 0.01 });
}
metrics.update1000Ms = performance.now() - t0;

let oldest = bars[0].time;
let loading = false;
adapter.onVisibleRangeChange(({ barsBefore }) => {
  if (barsBefore > 10 || loading || metrics.prepends >= 5) return;
  loading = true;
  const older = synth(oldest, 500, 1);
  // Wait one frame so the scroll that triggered the load has been painted before the sentinel is planted.
  setTimeout(() => requestAnimationFrame(() => {
    // getVisibleRange() is clamped to the loaded data, so its `from` moves when older bars arrive even though the
    // view did not. Measure an anchor bar's x coordinate instead (Ruling 110 I2), and record `to` as a second check.
    const ts = chartRef!.timeScale();
    const anchor = anchorBarTime();
    const xAt = () => (anchor === null ? null : ts.timeToCoordinate(toChartTime(anchor)));
    const shift = (x0: number | null, x1: number | null) => (x0 === null || x1 === null ? Number.NaN : x1 - x0);
    const xBefore = xAt();
    const toBefore = adapter.visibleTimeRange()?.to;
    // One-frame check (Task 19). A sentinel pixel that only a repaint clears is planted on the main pane canvas. In the
    // first frame after the prepend the sentinel must be gone (the pane was repainted in that frame). Those pixels are
    // then compared with a forced full repaint of the settled state one frame later: if the first painted frame had
    // shown an intermediate window (data prepended but the logical range not yet shifted), the two would differ.
    // Pixels before the prepend are not compared: blank space left of the oldest bar fills with the new bars and the
    // price scale re-fits to them, which is correct behaviour.
    const pane = mainPaneCanvas();
    if (pane) plantSentinel(pane);
    adapter.prependHistory(older);
    oldest = older[0].time;
    // Logged only: the library applies logical-range changes lazily, so a synchronous read can miss a wrong range.
    metrics.lastPrependSyncShiftPx = shift(xBefore, xAt());
    requestAnimationFrame(() => {
      metrics.lastPrependShiftPx = shift(xBefore, xAt());
      metrics.lastPrependKeptView = Math.abs(metrics.lastPrependShiftPx) <= 0.5;
      metrics.lastPrependToKept = toBefore !== undefined && adapter.visibleTimeRange()?.to === toBefore;
      metrics.lastPrependFramePainted = pane !== null && !sentinelPresent(pane);
      const firstFrame = pane ? canvasPixels(pane) : null;
      if (pane) plantSentinel(pane);
      chartRef!.applyOptions({}); // full invalidation: the next frame repaints the settled state from the model
      requestAnimationFrame(() => {
        metrics.lastPrependShift2FramesPx = shift(xBefore, xAt());
        metrics.lastPrependRepainted2 = pane !== null && !sentinelPresent(pane);
        metrics.lastPrependFirstFrameSettled = pane !== null && firstFrame !== null && canvasPixels(pane) === firstFrame;
        metrics.prepends++;
        loading = false;
      });
    });
  }), 50);
});

const SENTINEL = [255, 0, 255, 255];

/** The main (series) canvas of pane 0: the widest canvas in the pane row; the crosshair canvas follows it in the DOM. */
function mainPaneCanvas(): HTMLCanvasElement | null {
  const row = chartRef!.panes()[0].getHTMLElement();
  const canvases = row ? Array.from(row.querySelectorAll('canvas')) : [];
  return canvases.reduce<HTMLCanvasElement | null>((best, c) => (best === null || c.width > best.width ? c : best), null);
}

function canvasPixels(c: HTMLCanvasElement): string {
  return c.getContext('2d')!.getImageData(0, 0, c.width, c.height).data.join(',');
}

function plantSentinel(c: HTMLCanvasElement): void {
  const ctx = c.getContext('2d')!;
  const img = ctx.createImageData(1, 1);
  img.data.set(SENTINEL);
  ctx.putImageData(img, 0, 0);
}

function sentinelPresent(c: HTMLCanvasElement): boolean {
  const px = c.getContext('2d')!.getImageData(0, 0, 1, 1).data;
  return SENTINEL.every((v, i) => px[i] === v);
}

/** Time (ms) of the loaded bar in the middle of the visible logical range, or null when nothing is visible. */
function anchorBarTime(): number | null {
  const r = chartRef!.timeScale().getVisibleLogicalRange();
  const loaded = adapter.bars();
  if (!r || loaded.length === 0) return null;
  const i = Math.min(loaded.length - 1, Math.max(0, Math.round((r.from + r.to) / 2)));
  return loaded[i].time;
}

function switchTheme(): Promise<number> {
  return new Promise((resolve) => {
    const start = performance.now();
    const root = document.documentElement;
    const next: ThemeName = root.dataset.theme === 'dark' ? 'light' : 'dark';
    applyTheme(next);
    adapter.setTheme(readChartTheme(colourTokens[next]));
    requestAnimationFrame(() => requestAnimationFrame(() => {
      metrics.themeSwitchMs = performance.now() - start;
      resolve(metrics.themeSwitchMs);
    }));
  });
}

/** Every pixel row of a fresh screenshot, one string per bitmap row (takeScreenshot repaints pending changes first). */
function screenshotRows(): { rows: string[]; ratio: number } {
  const canvas = chartRef!.takeScreenshot();
  const data = canvas.getContext('2d')!.getImageData(0, 0, canvas.width, canvas.height).data;
  const stride = canvas.width * 4;
  const rows: string[] = [];
  for (let r = 0; r < canvas.height; r++) rows.push(data.subarray(r * stride, (r + 1) * stride).join(','));
  return { rows, ratio: canvas.height / chartRef!.chartElement().clientHeight };
}

const BAND_MIN_ROW_SHARE = 0.9;
const BAND_EDGE_PX = 2;
const bandCheck = { inBandRows: 0, changedInBand: 0, changedOutside: 0 };

/**
 * A/B check (Ruling 110 I3): screenshots with the band attached and after removePriceBand. At least 90 % of the in-band
 * pixel rows must differ, and no row more than 2 px outside the band may differ (so the band is drawn where it belongs).
 * Whole rows are compared, so candles covering part of a row cannot hide the band. Not every in-band row: the band's
 * zOrder is 'bottom', which lightweight-charts paints before the opaque grid, so a horizontal grid line inside the band
 * covers it on that row. Grid rows follow the price-axis font metrics, so they differ per engine (Task 19: WebKit put
 * one on the band's middle row, which the earlier single-row sample read). The band is re-attached afterwards.
 */
function bandVisible(): boolean {
  const yFrom = candleSeries().priceToCoordinate(bandFrom);
  const yTo = candleSeries().priceToCoordinate(bandTo);
  if (yFrom === null || yTo === null) return false;
  const a = screenshotRows();
  adapter.removePriceBand({ id: band.id });
  let b: { rows: string[] };
  try {
    b = screenshotRows();
  } finally {
    adapter.addPriceBand(band);
  }
  const top = Math.ceil(Math.min(yFrom, yTo) * a.ratio) + 1;
  const bottom = Math.floor(Math.max(yFrom, yTo) * a.ratio) - 1;
  const edge = Math.ceil(BAND_EDGE_PX * a.ratio);
  Object.assign(bandCheck, { inBandRows: 0, changedInBand: 0, changedOutside: 0 });
  a.rows.forEach((row, r) => {
    const changed = row !== b.rows[r];
    if (r >= top && r <= bottom) {
      bandCheck.inBandRows++;
      if (changed) bandCheck.changedInBand++;
    } else if ((r < top - edge || r > bottom + edge) && changed) {
      bandCheck.changedOutside++;
    }
  });
  return bandCheck.inBandRows > 0 && bandCheck.changedInBand >= BAND_MIN_ROW_SHARE * bandCheck.inBandRows
    && bandCheck.changedOutside === 0;
}

Object.assign(window, { __s05: { adapter, chart: () => chartRef, metrics, switchTheme, bandVisible, bandCheck, band, ready: true } });
