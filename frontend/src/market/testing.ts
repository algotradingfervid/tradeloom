import type { TickDTO } from '../api/backend';

/** A TickDTO with change/changePct derived the way Go derives them from the previous close. */
export function tick(token: number, ltp: number, close: number, over: Partial<TickDTO> = {}): TickDTO {
  const change = ltp - close;
  return { token, ltp, close, change, changePct: close > 0 ? (change / close) * 100 : 0, volume: 0, tsMs: 1, polled: false, ...over };
}
