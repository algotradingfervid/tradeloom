import type { IChartApi } from 'lightweight-charts';
import '../src/design/tokens.css';
import { applyTheme } from '../src/app/theme';
import { colourTokens, type ThemeName } from '../src/design/tokens';
import { LightweightChartsAdapter } from '../src/chart/LightweightChartsAdapter';
import { realLwc, type LwcLib } from '../src/chart/lwc';
import { readChartTheme } from '../src/chart/theme';
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

const metrics = { setCandles20kMs: 0, update1000Ms: 0, themeSwitchMs: 0, prepends: 0, lastPrependKeptView: false };

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
adapter.addPriceBand({ id: 'band', from: bandFrom, to: bandTo, color: token('--chart-band') });

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
  const before = adapter.visibleTimeRange();
  const older = synth(oldest, 500, 1);
  setTimeout(() => {
    adapter.prependHistory(older);
    oldest = older[0].time;
    const after = adapter.visibleTimeRange();
    metrics.lastPrependKeptView = !!before && !!after && before.from === after.from && before.to === after.to;
    metrics.prepends++;
    loading = false;
  }, 50);
});

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

/** Samples the screenshot inside the band vs above it: different pixels = primitive drawn. */
function bandVisible(): boolean {
  const canvas = chartRef!.takeScreenshot();
  const ctx = canvas.getContext('2d')!;
  const ratio = canvas.height / chartRef!.chartElement().clientHeight;
  const yIn = candleSeries().priceToCoordinate((bandFrom + bandTo) / 2)!;
  const yOut = candleSeries().priceToCoordinate(hi + (hi - lo))!;
  const px = (y: number) => Array.from(ctx.getImageData(2, Math.round(y * ratio), 1, 1).data).join(',');
  return px(yIn) !== px(yOut);
}

Object.assign(window, { __s05: { adapter, chart: () => chartRef, metrics, switchTheme, bandVisible, ready: true } });
