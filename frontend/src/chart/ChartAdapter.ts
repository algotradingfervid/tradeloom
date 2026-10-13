import type { Bar, ChartTheme, Handle, InstrumentSpec, LegendData, PriceBandSpec, RangeInfo, StudySpec, TimeRange, UpdateOutcome } from './types';

/** FR-CHART-001 swap point (ARCHITECTURE §8.5). Host views talk only to this interface. */
export interface ChartAdapter {
  setCandles(bars: readonly Bar[], opts?: { keepView?: boolean }): void;
  /** Lazy history: prepends only bars older than the first loaded one, without moving the view. Returns the count added. */
  prependHistory(bars: readonly Bar[]): number;
  bars(): readonly Bar[];
  addStudy(spec: StudySpec): Handle;
  removeStudy(h: Handle): void;
  addPriceBand(spec: PriceBandSpec): Handle;
  removePriceBand(h: Handle): void;
  setTheme(theme: ChartTheme): void;
  onVisibleRangeChange(cb: (r: RangeInfo) => void): () => void;
  visibleTimeRange(): TimeRange | null;
  destroy(): void;
  setInstrument(spec: InstrumentSpec): void;
  /** Live candle from the Go builder: append / replace last / update an older loaded bar / ignore unknown older bar. */
  updateLastBar(bar: Bar): UpdateOutcome;
  /** Bar under the crosshair (last bar when outside the chart). */
  onCrosshairMove(cb: (d: LegendData | null) => void): () => void;
  setPriceScale(opts: { log?: boolean; auto?: boolean }): void;
  priceScaleState(): { log: boolean; auto: boolean };
  panBars(delta: number): void;
  zoom(factor: number): void;
  scrollToLatest(): void;
}

export class StudyLimitError extends Error {
  constructor(readonly max: number) {
    super(`This chart allows up to ${max} indicators`);
    this.name = 'StudyLimitError';
  }
}
