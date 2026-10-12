import { beforeEach, describe, expect, it } from 'vitest';
import { TickMarkType } from 'lightweight-charts';
import { LightweightChartsAdapter } from './LightweightChartsAdapter';
import { makeFakeLwc, type FakeChart } from './testing/fakeLwc';
import type { Bar, ChartTheme } from './types';

const STEP = 300_000;
const T0 = Date.UTC(2026, 9, 1, 3, 45);
const mk = (i: number, close = 100 + i): Bar => ({ time: T0 + i * STEP, open: 100 + i, high: 102 + i, low: 99 + i, close, volume: 10 * i });
const range = (from: number, n: number) => Array.from({ length: n }, (_, k) => mk(from + k));
const THEME: ChartTheme = { background: 'bg', text: 'tx', grid: 'gr', crosshair: 'cr', separator: 'sp', up: 'up', down: 'dn', volumeUp: 'vu', volumeDown: 'vd' };

let chart: FakeChart;
let adapter: LightweightChartsAdapter;
beforeEach(() => {
  const fake = makeFakeLwc();
  adapter = new LightweightChartsAdapter(document.createElement('div'), { lib: fake.lib });
  chart = fake.charts[0];
});

describe('LightweightChartsAdapter (S0-5 core)', () => {
  it('creates candles in pane 0 and volume in pane 1; attribution logo and conflation off', () => {
    expect(chart.options.layout.attributionLogo).toBe(false);
    expect(chart.options.timeScale.enableConflation).toBe(false);
    expect(chart.paneTypes()).toEqual([['Candlestick'], ['Histogram']]);
  });

  it('setCandles sorts, de-duplicates and maps times to IST chart time', () => {
    adapter.setCandles([mk(2), mk(0), mk(1), mk(2, 999)]);
    const candles = chart.seriesOfType('Candlestick');
    expect(candles.data.map((d) => d.time)).toEqual([0, 1, 2].map((i) => (T0 + i * STEP) / 1000 + 19800));
    expect(candles.data[2].close).toBe(999);
    expect(adapter.bars()).toHaveLength(3);
  });

  it('colours volume bars from the theme', () => {
    adapter.setTheme(THEME);
    adapter.setCandles([mk(0, 101), mk(1, 50)]);
    expect(chart.seriesOfType('Histogram').data.map((d) => d.color)).toEqual(['vu', 'vd']);
  });

  it('FR-CHART-003 AC3: prependHistory adds only older bars and keeps the visible window', () => {
    adapter.setCandles(range(100, 100));
    chart.ts.logical = { from: 2, to: 60 };
    const added = adapter.prependHistory([...range(50, 60)]); // 50..109 — only 50..99 are older
    expect(added).toBe(50);
    expect(adapter.bars()).toHaveLength(150);
    expect(chart.ts.setLogicalCalls.at(-1)).toEqual({ from: 52, to: 110 });
    expect(adapter.prependHistory(range(100, 5))).toBe(0);
  });

  it('separate-pane study adds and removes its pane; NaN becomes whitespace', () => {
    const h = adapter.addStudy({ id: 'rsi', kind: 'line', pane: 'separate', color: 'c', points: [{ time: T0, value: Number.NaN }, { time: T0 + STEP, value: 55 }] });
    expect(chart.paneTypes()).toEqual([['Candlestick'], ['Histogram'], ['Line']]);
    expect(chart.seriesOfType('Line').data).toEqual([{ time: T0 / 1000 + 19800 }, { time: (T0 + STEP) / 1000 + 19800, value: 55 }]);
    adapter.removeStudy(h);
    expect(chart.paneTypes()).toEqual([['Candlestick'], ['Histogram']]);
  });

  it('attaches and detaches a price-band primitive on the candle series', () => {
    const h = adapter.addPriceBand({ id: 'b', from: 1, to: 2, color: 'c' });
    expect(chart.seriesOfType('Candlestick').primitives).toHaveLength(1);
    adapter.removePriceBand(h);
    expect(chart.seriesOfType('Candlestick').primitives).toHaveLength(0);
  });

  it('setTheme applies layout, grid, crosshair and candle colours', () => {
    adapter.setTheme(THEME);
    expect(chart.options.layout.background).toEqual({ type: 'solid', color: 'bg' });
    expect(chart.options.layout.textColor).toBe('tx');
    expect(chart.options.grid.vertLines.color).toBe('gr');
    expect(chart.seriesOfType('Candlestick').options).toMatchObject({ upColor: 'up', downColor: 'dn', wickUpColor: 'up', wickDownColor: 'dn' });
  });

  it('reports barsBefore on visible range change and unsubscribes', () => {
    adapter.setCandles(range(0, 100));
    const seen: number[] = [];
    const off = adapter.onVisibleRangeChange((r) => seen.push(r.barsBefore));
    chart.ts.fire({ from: 7, to: 50 });
    off();
    chart.ts.fire({ from: 1, to: 50 });
    expect(seen).toEqual([7]);
  });

  it('visibleTimeRange converts back to epoch ms; destroy removes the chart', () => {
    chart.ts.visibleTimes = { from: T0 / 1000 + 19800, to: (T0 + STEP) / 1000 + 19800 };
    expect(adapter.visibleTimeRange()).toEqual({ from: T0, to: T0 + STEP });
    adapter.destroy();
    expect(chart.removed).toBe(true);
  });
});

describe('LightweightChartsAdapter (S0-5 fix round 1, Ruling 110)', () => {
  const line = (id: string) => ({ id, kind: 'line' as const, pane: 'separate' as const, color: id, points: [{ time: T0, value: 1 }] });

  it('removing a middle study pane keeps the panes on either side', () => {
    const a = adapter.addStudy(line('a'));
    adapter.addStudy(line('b'));
    expect(chart.paneTypes()).toEqual([['Candlestick'], ['Histogram'], ['Line'], ['Line']]);
    adapter.removeStudy(a);
    expect(chart.paneTypes()).toEqual([['Candlestick'], ['Histogram'], ['Line']]);
    expect(chart.panesList[2].series[0].options.color).toBe('b');
  });

  it('removes a separate study pane that the library kept (preserved, empty)', () => {
    const h = adapter.addStudy(line('a'));
    chart.panesList[2].preserve = true; // as if the library had not auto-removed it
    adapter.removeStudy(h);
    expect(chart.paneTypes()).toEqual([['Candlestick'], ['Histogram']]);
  });

  it('addStudy sorts and de-duplicates points (last wins) before setData', () => {
    adapter.addStudy({
      id: 's', kind: 'line', pane: 'overlay', color: 'c',
      points: [{ time: T0 + STEP, value: 2 }, { time: T0, value: 1 }, { time: T0 + STEP, value: 3 }],
    });
    expect(chart.seriesOfType('Line').data).toEqual([
      { time: T0 / 1000 + 19800, value: 1 }, { time: (T0 + STEP) / 1000 + 19800, value: 3 },
    ]);
  });

  it('wires the IST time, tick-mark and Indian price formatters into the chart options', () => {
    const t = T0 / 1000 + 19800; // 01 Oct 2026 09:15 IST
    expect(chart.options.localization.timeFormatter(t)).toBe("01 Oct '26 09:15");
    expect(chart.options.timeScale.tickMarkFormatter(t, TickMarkType.Time)).toBe('09:15');
    expect(chart.options.timeScale.tickMarkFormatter(t, TickMarkType.DayOfMonth)).toBe('01');
    expect(chart.options.localization.priceFormatter(2252045.5)).toBe('22,52,045.50');
    expect(chart.options.localization.locale).toBe('en-IN');
  });
});
