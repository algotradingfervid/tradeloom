import {
  ColorType, CrosshairMode, PriceScaleMode,
  type CandlestickData, type HistogramData, type IChartApi, type ISeriesApi, type LineData, type LogicalRange,
  type MouseEventParams, type TickMarkType, type Time, type UTCTimestamp, type WhitespaceData,
} from 'lightweight-charts';
import { StudyLimitError, type ChartAdapter } from './ChartAdapter';
import type {
  Bar, ChartTheme, Handle, InstrumentSpec, LegendData, PriceBandSpec, RangeInfo, StudySpec, TimeRange, UpdateOutcome,
} from './types';
import { realLwc, type LwcLib } from './lwc';
import { dedupeSorted } from './bars';
import { formatCrosshairTime, formatTickMark, fromChartTime, toChartTime } from './time';
import { chartPriceFormatter } from './format';
import { PriceBandPrimitive } from './primitives/PriceBandPrimitive';

export interface AdapterOptions { lib?: LwcLib; maxStudies?: number }
type StudySeries = ISeriesApi<'Line'> | ISeriesApi<'Histogram'>;

export class LightweightChartsAdapter implements ChartAdapter {
  protected readonly lib: LwcLib;
  protected readonly chart: IChartApi;
  protected readonly candles: ISeriesApi<'Candlestick'>;
  protected volume: ISeriesApi<'Histogram'> | null = null;
  protected data: Bar[] = [];
  protected index = new Map<number, number>();
  protected spec: InstrumentSpec = { intraday: true, hasVolume: true, pricePrecision: 2, minMove: 0.05 };
  protected theme: ChartTheme | null = null;
  protected readonly studies = new Map<string, { series: StudySeries; separate: boolean }>();
  protected readonly bands = new Map<string, PriceBandPrimitive>();

  constructor(container: HTMLElement, protected readonly opts: AdapterOptions = {}) {
    this.lib = opts.lib ?? realLwc;
    this.chart = this.lib.createChart(container, {
      autoSize: true,
      layout: { attributionLogo: false },
      timeScale: {
        timeVisible: true, secondsVisible: false, rightOffset: 5, enableConflation: false,
        tickMarkFormatter: (t: Time, type: TickMarkType) => formatTickMark(t as UTCTimestamp, type),
      },
      localization: {
        locale: 'en-IN',
        timeFormatter: (t: Time) => formatCrosshairTime(t as UTCTimestamp, this.spec.intraday),
        priceFormatter: (p: number) => chartPriceFormatter(this.spec.pricePrecision)(p),
      },
      crosshair: { mode: CrosshairMode.Normal },
      handleScroll: true,
      handleScale: { mouseWheel: true, pinch: true, axisPressedMouseMove: true, axisDoubleClickReset: true },
    });
    this.candles = this.chart.addSeries(this.lib.CandlestickSeries, { priceFormat: { type: 'price', precision: 2, minMove: 0.05 } });
    this.syncVolumeSeries();
  }

  setCandles(bars: readonly Bar[], opts: { keepView?: boolean } = {}): void {
    const view = opts.keepView ? this.chart.timeScale().getVisibleLogicalRange() : null;
    this.data = dedupeSorted(bars);
    this.reindex();
    this.redraw();
    if (view) this.chart.timeScale().setVisibleLogicalRange(view);
  }

  prependHistory(bars: readonly Bar[]): number {
    const first = this.data.length > 0 ? this.data[0].time : Number.POSITIVE_INFINITY;
    const older = dedupeSorted(bars).filter((b) => b.time < first);
    if (older.length === 0) return 0;
    const view = this.chart.timeScale().getVisibleLogicalRange();
    this.data = [...older, ...this.data];
    this.reindex();
    this.redraw();
    if (view) this.chart.timeScale().setVisibleLogicalRange({ from: view.from + older.length, to: view.to + older.length });
    return older.length;
  }

  bars(): readonly Bar[] { return this.data; }

  addStudy(spec: StudySpec): Handle {
    if (this.opts.maxStudies !== undefined && this.studies.size >= this.opts.maxStudies) throw new StudyLimitError(this.opts.maxStudies);
    const pane = spec.pane === 'separate' ? this.chart.addPane() : this.chart.panes()[0];
    // setData needs strictly ascending times: sort and de-duplicate (last point wins, as dedupeSorted does for bars).
    const byTime = new Map<number, number>();
    for (const p of spec.points) byTime.set(p.time, p.value);
    const points: (LineData<Time> | WhitespaceData<Time>)[] = [...byTime.entries()].sort((a, b) => a[0] - b[0]).map(([t, v]) =>
      Number.isFinite(v) ? { time: toChartTime(t), value: v } : { time: toChartTime(t) });
    let series: StudySeries;
    if (spec.kind === 'line') {
      const s = pane.addSeries(this.lib.LineSeries, { color: spec.color, priceLineVisible: false });
      s.setData(points);
      series = s;
    } else {
      const s = pane.addSeries(this.lib.HistogramSeries, { color: spec.color, priceLineVisible: false });
      s.setData(points);
      series = s;
    }
    this.studies.set(spec.id, { series, separate: spec.pane === 'separate' });
    return { id: spec.id };
  }

  removeStudy(h: Handle): void {
    const s = this.studies.get(h.id);
    if (!s) return;
    if (s.separate) this.removeSeriesWithPane(s.series);
    else this.chart.removeSeries(s.series);
    this.studies.delete(h.id);
  }

  addPriceBand(spec: PriceBandSpec): Handle {
    const p = new PriceBandPrimitive({ from: spec.from, to: spec.to, color: spec.color });
    this.candles.attachPrimitive(p);
    this.bands.set(spec.id, p);
    return { id: spec.id };
  }

  removePriceBand(h: Handle): void {
    const p = this.bands.get(h.id);
    if (!p) return;
    this.candles.detachPrimitive(p);
    this.bands.delete(h.id);
  }

  setTheme(t: ChartTheme): void {
    this.theme = t;
    this.chart.applyOptions({
      layout: { background: { type: ColorType.Solid, color: t.background }, textColor: t.text, panes: { separatorColor: t.separator } },
      grid: { vertLines: { color: t.grid }, horzLines: { color: t.grid } },
      crosshair: {
        vertLine: { color: t.crosshair, labelBackgroundColor: t.crosshair },
        horzLine: { color: t.crosshair, labelBackgroundColor: t.crosshair },
      },
    });
    this.candles.applyOptions({
      upColor: t.up, downColor: t.down, borderUpColor: t.up, borderDownColor: t.down, wickUpColor: t.up, wickDownColor: t.down,
    });
    this.volume?.setData(this.data.map((b) => this.toVolume(b)));
  }

  onVisibleRangeChange(cb: (r: RangeInfo) => void): () => void {
    const ts = this.chart.timeScale();
    const handler = (r: LogicalRange | null) => {
      if (!r) return;
      const info = this.candles.barsInLogicalRange(r);
      if (info) cb({ barsBefore: info.barsBefore });
    };
    ts.subscribeVisibleLogicalRangeChange(handler);
    return () => ts.unsubscribeVisibleLogicalRangeChange(handler);
  }

  visibleTimeRange(): TimeRange | null {
    const r = this.chart.timeScale().getVisibleRange();
    if (!r) return null;
    return { from: fromChartTime(r.from as UTCTimestamp), to: fromChartTime(r.to as UTCTimestamp) };
  }

  destroy(): void {
    this.chart.remove();
    this.studies.clear();
    this.bands.clear();
  }

  setInstrument(spec: InstrumentSpec): void {
    this.spec = spec;
    this.candles.applyOptions({ priceFormat: { type: 'price', precision: spec.pricePrecision, minMove: spec.minMove } });
    this.chart.applyOptions({ timeScale: { timeVisible: spec.intraday } });
    this.syncVolumeSeries();
  }

  updateLastBar(bar: Bar): UpdateOutcome {
    const last = this.data[this.data.length - 1];
    if (!last || bar.time > last.time) {
      this.data.push(bar);
      this.index.set(bar.time, this.data.length - 1);
      this.candles.update(this.toCandle(bar));
      this.volume?.update(this.toVolume(bar));
      return 'appended';
    }
    if (bar.time === last.time) {
      this.data[this.data.length - 1] = bar;
      this.candles.update(this.toCandle(bar));
      this.volume?.update(this.toVolume(bar));
      return 'replaced';
    }
    const i = this.index.get(bar.time);
    if (i === undefined) return 'ignored'; // older than anything loaded, or a gap: never throw (Review Focus 4)
    this.data[i] = bar;
    this.candles.update(this.toCandle(bar), true);
    this.volume?.update(this.toVolume(bar), true);
    return 'historical';
  }

  onCrosshairMove(cb: (d: LegendData | null) => void): () => void {
    const handler = (p: MouseEventParams<Time>) => {
      const idx = p.time === undefined ? this.data.length - 1 : this.index.get(fromChartTime(p.time as UTCTimestamp));
      if (idx === undefined || idx < 0) { cb(null); return; }
      cb({ bar: this.data[idx], previousClose: idx > 0 ? this.data[idx - 1].close : null });
    };
    this.chart.subscribeCrosshairMove(handler);
    return () => this.chart.unsubscribeCrosshairMove(handler);
  }

  setPriceScale(opts: { log?: boolean; auto?: boolean }): void {
    const patch: { mode?: PriceScaleMode; autoScale?: boolean } = {};
    if (opts.log !== undefined) patch.mode = opts.log ? PriceScaleMode.Logarithmic : PriceScaleMode.Normal;
    if (opts.auto !== undefined) patch.autoScale = opts.auto;
    this.chart.priceScale('right', 0).applyOptions(patch);
  }

  priceScaleState(): { log: boolean; auto: boolean } {
    const o = this.chart.priceScale('right', 0).options();
    return { log: o.mode === PriceScaleMode.Logarithmic, auto: o.autoScale };
  }

  /** Negative = towards older bars. */
  panBars(delta: number): void {
    const ts = this.chart.timeScale();
    ts.scrollToPosition(ts.scrollPosition() + delta, false);
  }

  /** factor > 1 zooms in, < 1 zooms out; the right edge stays fixed. */
  zoom(factor: number): void {
    const ts = this.chart.timeScale();
    const r = ts.getVisibleLogicalRange();
    if (!r || factor <= 0) return;
    const width = (r.to - r.from) / factor;
    ts.setVisibleLogicalRange({ from: r.to - width, to: r.to });
  }

  scrollToLatest(): void {
    this.chart.timeScale().scrollToRealTime();
  }

  // ---- internals -------------------------------------------------------------------------------

  protected syncVolumeSeries(): void {
    if (this.spec.hasVolume && !this.volume) {
      this.chart.panes()[0].setStretchFactor(3);
      const pane = this.chart.addPane();
      pane.setStretchFactor(1);
      this.volume = pane.addSeries(this.lib.HistogramSeries, { priceFormat: { type: 'volume' }, lastValueVisible: false, priceLineVisible: false });
      this.volume.setData(this.data.map((b) => this.toVolume(b)));
    } else if (!this.spec.hasVolume && this.volume) {
      this.removeSeriesWithPane(this.volume);
      this.volume = null;
    }
  }

  /**
   * 5.2.1 removes a pane emptied by removeSeries itself (unless it was added with preserveEmptyPane). Remove the
   * pane here only when the pane count shows the library kept it, so a neighbouring pane is never removed.
   */
  protected removeSeriesWithPane(series: StudySeries): void {
    const paneIndex = series.getPane().paneIndex();
    const before = this.chart.panes().length;
    this.chart.removeSeries(series);
    if (paneIndex === 0 || this.chart.panes().length !== before) return;
    const pane = this.chart.panes()[paneIndex];
    if (pane && pane.getSeries().length === 0) this.chart.removePane(paneIndex);
  }

  protected reindex(): void {
    this.index = new Map(this.data.map((b, i) => [b.time, i]));
  }

  protected redraw(): void {
    this.candles.setData(this.data.map((b) => this.toCandle(b)));
    this.volume?.setData(this.data.map((b) => this.toVolume(b)));
  }

  protected toCandle(b: Bar): CandlestickData<Time> {
    return { time: toChartTime(b.time), open: b.open, high: b.high, low: b.low, close: b.close };
  }

  protected toVolume(b: Bar): HistogramData<Time> {
    const time = toChartTime(b.time);
    if (!this.theme) return { time, value: b.volume };
    return { time, value: b.volume, color: b.close >= b.open ? this.theme.volumeUp : this.theme.volumeDown };
  }
}
