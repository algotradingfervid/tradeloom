/** One candle. time = candle start in ms since epoch (UTC); prices in rupees. */
export interface Bar { time: number; open: number; high: number; low: number; close: number; volume: number }

export interface InstrumentSpec { intraday: boolean; hasVolume: boolean; pricePrecision: number; minMove: number }

export interface ChartTheme {
  background: string; text: string; grid: string; crosshair: string; separator: string;
  up: string; down: string; volumeUp: string; volumeDown: string;
}

export interface StudyPoint { time: number; value: number } // NaN value = gap (warm-up / session reset)
export interface StudySpec { id: string; kind: 'line' | 'histogram'; pane: 'overlay' | 'separate'; color: string; points: readonly StudyPoint[] }
export interface PriceBandSpec { id: string; from: number; to: number; color: string }
export interface Handle { readonly id: string }
/** Legend for one bar; previousClose is the close of the bar before it (null for the first loaded bar). */
export interface LegendData { bar: Bar; previousClose: number | null }
export interface RangeInfo { barsBefore: number }
export interface TimeRange { from: number; to: number } // ms
export type UpdateOutcome = 'appended' | 'replaced' | 'historical' | 'ignored';
