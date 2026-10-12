export type TimeframeId = '1m' | '3m' | '5m' | '10m' | '15m' | '30m' | '1h' | '1d';
export type KiteInterval =
  | 'minute' | '3minute' | '5minute' | '10minute' | '15minute' | '30minute' | '60minute' | 'day';

export interface Timeframe {
  readonly id: TimeframeId;
  readonly label: string;
  readonly kite: KiteInterval;
  readonly intraday: boolean;
  /** FR-CHART-003 initial window, also used as the lazy-load chunk. */
  readonly initialTradingDays: number;
}

export const TIMEFRAMES: readonly Timeframe[] = [
  { id: '1m', label: '1 Minute', kite: 'minute', intraday: true, initialTradingDays: 5 },
  { id: '3m', label: '3 Minutes', kite: '3minute', intraday: true, initialTradingDays: 15 },
  { id: '5m', label: '5 Minutes', kite: '5minute', intraday: true, initialTradingDays: 15 },
  { id: '10m', label: '10 Minutes', kite: '10minute', intraday: true, initialTradingDays: 30 },
  { id: '15m', label: '15 Minutes', kite: '15minute', intraday: true, initialTradingDays: 30 },
  { id: '30m', label: '30 Minutes', kite: '30minute', intraday: true, initialTradingDays: 120 },
  { id: '1h', label: '1 Hour', kite: '60minute', intraday: true, initialTradingDays: 120 },
  { id: '1d', label: '1 Day', kite: 'day', intraday: false, initialTradingDays: 750 },
];

/** OQ-23: shipped value of Settings → Chart → default timeframe. */
export const SHIPPED_DEFAULT_TIMEFRAME: TimeframeId = '1h';

export function timeframeById(id: string): Timeframe {
  const tf = TIMEFRAMES.find((t) => t.id === id);
  if (!tf) throw new Error(`unknown timeframe ${id}`);
  return tf;
}

export function timeframeFromKite(k: string): Timeframe | undefined {
  return TIMEFRAMES.find((t) => t.kite === k);
}

/** Accepts a TimeframeId or a Kite interval (the setting stores Kite form); anything else → shipped default. */
export function parseTimeframe(v: unknown): TimeframeId {
  if (typeof v === 'string') {
    const byId = TIMEFRAMES.find((t) => t.id === v);
    if (byId) return byId.id;
    const byKite = timeframeFromKite(v);
    if (byKite) return byKite.id;
  }
  return SHIPPED_DEFAULT_TIMEFRAME;
}
