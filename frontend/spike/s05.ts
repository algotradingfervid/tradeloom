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
  setTimeout(() => {
    // getVisibleRange() is clamped to the loaded data, so its `from` moves when older bars arrive even though the
    // view did not. Measure an anchor bar's x coordinate instead (Ruling 110 I2), and record `to` as a second check.
    const ts = chartRef!.timeScale();
    const anchor = anchorBarTime();
    const xBefore = anchor === null ? null : ts.timeToCoordinate(toChartTime(anchor));
    const toBefore = adapter.visibleTimeRange()?.to;
    adapter.prependHistory(older);
    oldest = older[0].time;
    requestAnimationFrame(() => {
      const xAfter = anchor === null ? null : ts.timeToCoordinate(toChartTime(anchor));
      metrics.lastPrependShiftPx = xBefore === null || xAfter === null ? Number.NaN : xAfter - xBefore;
      metrics.lastPrependKeptView = Math.abs(metrics.lastPrependShiftPx) <= 0.5;
      metrics.lastPrependToKept = toBefore !== undefined && adapter.visibleTimeRange()?.to === toBefore;
      metrics.prepends++;
      loading = false;
    });
  }, 50);
});

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

/** One full pixel row of a fresh screenshot (takeScreenshot repaints pending changes first). */
function screenshotRow(y: number): string {
  const canvas = chartRef!.takeScreenshot();
  const ctx = canvas.getContext('2d')!;
  const ratio = canvas.height / chartRef!.chartElement().clientHeight;
  return Array.from(ctx.getImageData(0, Math.round(y * ratio), canvas.width, 1).data).join(',');
}

/**
 * A/B check (Ruling 110 I3): the same in-band pixel row with the band attached and after removePriceBand must differ.
 * A whole row is compared, so candles covering part of it cannot hide the band. The band is re-attached afterwards.
 */
function bandVisible(): boolean {
  const y = candleSeries().priceToCoordinate((bandFrom + bandTo) / 2);
  if (y === null) return false;
  const withBand = screenshotRow(y);
  adapter.removePriceBand({ id: band.id });
  try {
    return screenshotRow(y) !== withBand;
  } finally {
    adapter.addPriceBand(band);
  }
}

Object.assign(window, { __s05: { adapter, chart: () => chartRef, metrics, switchTheme, bandVisible, ready: true } });
