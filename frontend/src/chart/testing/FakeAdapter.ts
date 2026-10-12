import type { ChartDataSink } from '../ChartDataController';
import { dedupeSorted } from '../bars';
import type {
  Bar, ChartTheme, Handle, InstrumentSpec, LegendData, PriceBandSpec, RangeInfo, StudySpec, TimeRange, UpdateOutcome,
} from '../types';

/**
 * In-memory chart adapter for controller and view tests. It has every member of the plan's ChartAdapter (Tasks 3–4);
 * it declares only ChartDataSink because ChartAdapter.ts lands with Task 3 (switch to `implements ChartAdapter` then).
 */
export class FakeAdapter implements ChartDataSink {
  data: Bar[] = [];
  spec: InstrumentSpec | null = null;
  theme: ChartTheme | null = null;
  scale = { log: false, auto: true };
  view: TimeRange | null = null;
  studies = new Map<string, StudySpec>();
  calls: string[] = [];
  setCandlesOpts: ({ keepView?: boolean } | undefined)[] = [];
  destroyed = false;
  private rangeCbs = new Set<(r: RangeInfo) => void>();
  private crossCbs = new Set<(d: LegendData | null) => void>();

  setInstrument(spec: InstrumentSpec): void { this.spec = spec; this.calls.push('setInstrument'); }
  setCandles(bars: readonly Bar[], opts?: { keepView?: boolean }): void { this.data = dedupeSorted(bars); this.setCandlesOpts.push(opts); this.calls.push('setCandles'); }
  prependHistory(bars: readonly Bar[]): number {
    const first = this.data[0]?.time ?? Number.POSITIVE_INFINITY;
    const older = dedupeSorted(bars).filter((b) => b.time < first);
    this.data = [...older, ...this.data];
    this.calls.push('prependHistory');
    return older.length;
  }
  updateLastBar(bar: Bar): UpdateOutcome {
    this.calls.push('updateLastBar');
    const last = this.data[this.data.length - 1];
    if (!last || bar.time > last.time) { this.data.push(bar); return 'appended'; }
    if (bar.time === last.time) { this.data[this.data.length - 1] = bar; return 'replaced'; }
    const i = this.data.findIndex((b) => b.time === bar.time);
    if (i < 0) return 'ignored';
    this.data[i] = bar;
    return 'historical';
  }
  bars(): readonly Bar[] { return this.data; }
  addStudy(spec: StudySpec): Handle { this.studies.set(spec.id, spec); return { id: spec.id }; }
  removeStudy(h: Handle): void { this.studies.delete(h.id); }
  addPriceBand(spec: PriceBandSpec): Handle { return { id: spec.id }; }
  removePriceBand(): void {}
  setTheme(t: ChartTheme): void { this.theme = t; this.calls.push('setTheme'); }
  setPriceScale(o: { log?: boolean; auto?: boolean }): void { this.scale = { log: o.log ?? this.scale.log, auto: o.auto ?? this.scale.auto }; }
  priceScaleState(): { log: boolean; auto: boolean } { return { ...this.scale }; }
  panBars(delta: number): void { this.calls.push(`pan:${delta}`); }
  zoom(factor: number): void { this.calls.push(`zoom:${factor}`); }
  scrollToLatest(): void { this.calls.push('scrollToLatest'); }
  onVisibleRangeChange(cb: (r: RangeInfo) => void): () => void { this.rangeCbs.add(cb); return () => { this.rangeCbs.delete(cb); }; }
  onCrosshairMove(cb: (d: LegendData | null) => void): () => void { this.crossCbs.add(cb); return () => { this.crossCbs.delete(cb); }; }
  visibleTimeRange(): TimeRange | null { return this.view; }
  destroy(): void { this.destroyed = true; }
  emitRange(barsBefore: number): void { this.rangeCbs.forEach((cb) => cb({ barsBefore })); }
  emitCrosshair(d: LegendData | null): void { this.crossCbs.forEach((cb) => cb(d)); }
}
