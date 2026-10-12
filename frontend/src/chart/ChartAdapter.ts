import type { Bar, ChartTheme, Handle, PriceBandSpec, RangeInfo, StudySpec, TimeRange } from './types';

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
}
