import type { BackfillProgress, CandleDTO, CandleEventDTO, CandlesResult } from './backend';
import type { Bar } from '../chart/types';
import type { KiteInterval } from '../chart/timeframes';
import { call } from './call';
import { currentBackend } from './current';

export type { BackfillProgress };
export interface CandlesPage { bars: Bar[]; complete: boolean; backfillJobId: string; needsDataPlan: boolean; offline: boolean }
/** partial (Go builder): tracking began after this candle started, so its open and volume are unknown. */
export interface CandleEvent { token: number; interval: string; bar: Bar; closed: boolean; partial?: boolean }

export const toBar = (c: CandleDTO): Bar => ({ time: c.t, open: c.o, high: c.h, low: c.l, close: c.c, volume: c.v });
const toPage = (r: CandlesResult): CandlesPage => ({
  bars: (r.candles ?? []).map(toBar), complete: r.complete, backfillJobId: r.backfillJobId ?? '',
  needsDataPlan: r.needsDataPlan, offline: r.offline,
});
const toEvent = (e: CandleEventDTO, closed: boolean): CandleEvent =>
  ({ token: e.token, interval: e.interval, bar: toBar(e.candle), closed, ...(e.partial ? { partial: true } : {}) });

/** `tradingDays` trading days of candles ending at `toMs` (0 = now); Go resolves fromMs from the calendar. */
export async function fetchHistory(token: number, interval: KiteInterval, toMs: number, tradingDays: number): Promise<CandlesPage> {
  return toPage(await call(currentBackend().chart.candles({ token, interval, fromMs: 0, toMs, tradingDays })));
}

export async function refreshRange(token: number, interval: KiteInterval, fromMs: number, toMs: number): Promise<CandlesPage> {
  return toPage(await call(currentBackend().chart.refresh({ token, interval, fromMs, toMs })));
}

export const openLive = (token: number, interval: KiteInterval): Promise<string> => call(currentBackend().chart.openLive(token, interval));
export const closeLive = (id: string): Promise<void> => call(currentBackend().chart.closeLive(id));

/** candle.forming arrives as a coalesced array, candle.closed as one event; both reach `cb` one candle at a time. */
export function onCandle(cb: (e: CandleEvent) => void): () => void {
  const b = currentBackend();
  const offForming = b.on('candle.forming', (batch) => { for (const e of batch) cb(toEvent(e, false)); });
  const offClosed = b.on('candle.closed', (e) => cb(toEvent(e, true)));
  return () => { offForming(); offClosed(); };
}

export function onBackfillProgress(cb: (progress: BackfillProgress[]) => void): () => void {
  return currentBackend().on('backfill.progress', cb);
}
