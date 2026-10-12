/* eslint-disable @typescript-eslint/no-explicit-any -- test double mirrors the untyped surface of lightweight-charts */
import type { LwcLib } from '../lwc';

type Opts = Record<string, any>;
const merge = (t: Opts, s: Opts): Opts => {
  for (const [k, v] of Object.entries(s ?? {})) {
    if (v && typeof v === 'object' && !Array.isArray(v) && typeof v !== 'function') t[k] = merge(t[k] ?? {}, v);
    else t[k] = v;
  }
  return t;
};

export class FakePriceScale {
  options_: Opts = { mode: 0, autoScale: true };
  applyOptions(o: Opts) { merge(this.options_, o); }
  options() { return this.options_; }
}

export class FakeSeries {
  data: any[] = [];
  updates: { bar: any; historical: boolean }[] = [];
  primitives: any[] = [];
  options: Opts;
  constructor(readonly type: string, options: Opts | undefined, public pane: FakePane) { this.options = merge({}, options ?? {}); }
  seriesType() { return this.type; }
  setData(d: any[]) { this.data = [...d]; }
  update(bar: any, historical = false) {
    const last = this.data[this.data.length - 1];
    if (!historical && last && bar.time < last.time) throw new Error(`Cannot update oldest data, last time=${last.time}, new time=${bar.time}`);
    if (historical) {
      const i = this.data.findIndex((x) => x.time === bar.time);
      if (i < 0) throw new Error('Cannot update non-existing data point when historicalUpdate is true');
      this.data[i] = bar;
    } else if (last && last.time === bar.time) this.data[this.data.length - 1] = bar;
    else this.data.push(bar);
    this.updates.push({ bar, historical });
  }
  applyOptions(o: Opts) { merge(this.options, o); }
  attachPrimitive(p: any) { this.primitives.push(p); }
  detachPrimitive(p: any) { this.primitives = this.primitives.filter((x) => x !== p); }
  barsInLogicalRange(r: { from: number; to: number }) { return { barsBefore: Math.max(0, Math.floor(r.from)), barsAfter: Math.max(0, this.data.length - 1 - Math.ceil(r.to)) }; }
  priceToCoordinate(p: number) { return p; }
  getPane() { return this.pane; }
}

export class FakePane {
  series: FakeSeries[] = [];
  stretch = 1;
  constructor(private readonly chart: FakeChart) {}
  paneIndex() { return this.chart.panesList.indexOf(this); }
  getSeries() { return this.series; }
  setStretchFactor(n: number) { this.stretch = n; }
  getStretchFactor() { return this.stretch; }
  addSeries(def: { type: string }, options?: Opts) { const s = new FakeSeries(def.type, options, this); this.series.push(s); return s; }
}

export class FakeTimeScale {
  logical: { from: number; to: number } | null = { from: 0, to: 100 };
  visibleTimes: { from: number; to: number } | null = null;
  position = 0;
  realTimeCalls = 0;
  setLogicalCalls: { from: number; to: number }[] = [];
  options: Opts = {};
  private handlers = new Set<(r: any) => void>();
  getVisibleLogicalRange() { return this.logical; }
  setVisibleLogicalRange(r: { from: number; to: number }) { this.logical = r; this.setLogicalCalls.push(r); this.fire(r); }
  getVisibleRange() { return this.visibleTimes; }
  subscribeVisibleLogicalRangeChange(h: (r: any) => void) { this.handlers.add(h); }
  unsubscribeVisibleLogicalRangeChange(h: (r: any) => void) { this.handlers.delete(h); }
  scrollPosition() { return this.position; }
  scrollToPosition(p: number) { this.position = p; }
  scrollToRealTime() { this.realTimeCalls++; this.position = 0; }
  applyOptions(o: Opts) { merge(this.options, o); }
  fire(r: { from: number; to: number } | null) { this.handlers.forEach((h) => h(r)); }
}

export class FakeChart {
  options: Opts;
  panesList: FakePane[] = [];
  ts = new FakeTimeScale();
  removed = false;
  private scales = new Map<string, FakePriceScale>();
  private crosshair = new Set<(p: any) => void>();
  constructor(options: Opts | undefined) { this.options = merge({}, options ?? {}); this.panesList.push(new FakePane(this)); }
  addSeries(def: { type: string }, options?: Opts, paneIndex = 0) { while (this.panesList.length <= paneIndex) this.panesList.push(new FakePane(this)); return this.panesList[paneIndex].addSeries(def, options); }
  addPane() { const p = new FakePane(this); this.panesList.push(p); return p; }
  panes() { return [...this.panesList]; }
  removePane(i: number) { this.panesList.splice(i, 1); }
  removeSeries(s: FakeSeries) { s.pane.series = s.pane.series.filter((x) => x !== s); }
  timeScale() { return this.ts; }
  priceScale(id: string, paneIndex = 0) { const k = `${id}@${paneIndex}`; if (!this.scales.has(k)) this.scales.set(k, new FakePriceScale()); return this.scales.get(k)!; }
  applyOptions(o: Opts) { merge(this.options, o); }
  subscribeCrosshairMove(h: (p: any) => void) { this.crosshair.add(h); }
  unsubscribeCrosshairMove(h: (p: any) => void) { this.crosshair.delete(h); }
  fireCrosshair(p: any) { this.crosshair.forEach((h) => h(p)); }
  remove() { this.removed = true; }
  // test helpers
  paneTypes() { return this.panesList.map((p) => p.series.map((s) => s.type)); }
  seriesOfType(type: string) { const s = this.panesList.flatMap((p) => p.series).find((x) => x.type === type); if (!s) throw new Error(`no ${type} series`); return s; }
}

export function makeFakeLwc(): { lib: LwcLib; charts: FakeChart[] } {
  const charts: FakeChart[] = [];
  const lib = {
    createChart: (_el: unknown, options?: Opts) => { const c = new FakeChart(options); charts.push(c); return c; },
    CandlestickSeries: { type: 'Candlestick' },
    HistogramSeries: { type: 'Histogram' },
    LineSeries: { type: 'Line' },
  } as unknown as LwcLib;
  return { lib, charts };
}
