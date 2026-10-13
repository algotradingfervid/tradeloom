import { PriceScaleMode } from 'lightweight-charts';
import { beforeEach, describe, expect, it } from 'vitest';
import { LightweightChartsAdapter } from './LightweightChartsAdapter';
import { StudyLimitError } from './ChartAdapter';
import { makeFakeLwc, type FakeChart } from './testing/fakeLwc';
import { toChartTime } from './time';
import type { Bar } from './types';

const STEP = 300_000;
const T0 = Date.UTC(2026, 9, 1, 4, 30); // 10:00 IST
const mk = (i: number, close = 100 + i): Bar => ({ time: T0 + i * STEP, open: 100, high: 110 + i, low: 90, close, volume: 5 });

let chart: FakeChart;
let a: LightweightChartsAdapter;
const setup = (maxStudies?: number) => {
  const fake = makeFakeLwc();
  a = new LightweightChartsAdapter(document.createElement('div'), { lib: fake.lib, maxStudies });
  chart = fake.charts[0];
};
beforeEach(() => setup());

describe('adapter completion', () => {
  it('index instrument: volume series and its pane are removed; precision and daily axis applied', () => {
    a.setInstrument({ intraday: false, hasVolume: false, pricePrecision: 2, minMove: 0.05 });
    expect(chart.paneTypes()).toEqual([['Candlestick']]);
    expect(chart.seriesOfType('Candlestick').options.priceFormat).toEqual({ type: 'price', precision: 2, minMove: 0.05 });
    expect(chart.options.timeScale.timeVisible).toBe(false);
    a.setInstrument({ intraday: true, hasVolume: true, pricePrecision: 2, minMove: 0.1 });
    expect(chart.paneTypes()).toEqual([['Candlestick'], ['Histogram']]);
  });

  it('live forming bar: append then replace', () => {
    a.setCandles([mk(0), mk(1)]);
    expect(a.updateLastBar(mk(2))).toBe('appended');
    expect(a.updateLastBar(mk(2, 150))).toBe('replaced');
    expect(a.bars().at(-1)!.close).toBe(150);
    expect(chart.seriesOfType('Candlestick').data.at(-1).close).toBe(150);
  });

  it('late close for an older loaded bar updates in place; unknown older bar is ignored without throwing', () => {
    a.setCandles([mk(0), mk(1), mk(2)]);
    expect(a.updateLastBar(mk(1, 777))).toBe('historical');
    const series = chart.seriesOfType('Candlestick');
    expect(series.updates.at(-1)).toEqual({ bar: expect.objectContaining({ close: 777 }), historical: true });
    expect(a.bars()[1].close).toBe(777);
    expect(() => a.updateLastBar(mk(-5))).not.toThrow();
    expect(a.updateLastBar(mk(-5))).toBe('ignored');
    expect(a.bars()).toHaveLength(3);
  });

  it('FR-CHART-011 AC1: crosshair reports the hovered bar and its previous close; outside = last bar', () => {
    a.setCandles([mk(0), mk(1), mk(2)]);
    const seen: unknown[] = [];
    a.onCrosshairMove((d) => seen.push(d));
    chart.fireCrosshair({ time: toChartTime(mk(1).time), seriesData: new Map() });
    chart.fireCrosshair({ seriesData: new Map() });
    expect(seen).toEqual([{ bar: mk(1), previousClose: mk(0).close }, { bar: mk(2), previousClose: mk(1).close }]);
  });

  it('FR-CHART-011 AC2: L toggles log scale, A re-enables auto-scale', () => {
    a.setPriceScale({ log: true });
    expect(chart.priceScale('right', 0).options().mode).toBe(PriceScaleMode.Logarithmic);
    expect(a.priceScaleState()).toEqual({ log: true, auto: true });
    chart.priceScale('right', 0).applyOptions({ autoScale: false }); // user dragged the price scale
    a.setPriceScale({ auto: true });
    expect(a.priceScaleState()).toEqual({ log: true, auto: true });
    a.setPriceScale({ log: false });
    expect(chart.priceScale('right', 0).options().mode).toBe(PriceScaleMode.Normal);
  });

  it('FR-CHART-011 AC4 + keyboard nav: pan, zoom around the right edge, Home = latest', () => {
    chart.ts.position = 0;
    a.panBars(-10);
    expect(chart.ts.position).toBe(-10);
    chart.ts.logical = { from: 0, to: 100 };
    a.zoom(2);
    expect(chart.ts.logical).toEqual({ from: 50, to: 100 });
    a.scrollToLatest();
    expect(chart.ts.realTimeCalls).toBe(1);
  });

  it('FR-CHART-001 AC2: a host limit of 5 refuses the 6th study', () => {
    setup(5);
    for (let i = 0; i < 5; i++) a.addStudy({ id: `s${i}`, kind: 'line', pane: 'overlay', color: 'c', points: [] });
    expect(() => a.addStudy({ id: 's5', kind: 'line', pane: 'overlay', color: 'c', points: [] })).toThrow(StudyLimitError);
    expect(() => a.addStudy({ id: 's5', kind: 'line', pane: 'overlay', color: 'c', points: [] })).toThrow('This chart allows up to 5 indicators');
  });
});
