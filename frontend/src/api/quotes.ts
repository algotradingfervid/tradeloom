import type { QuoteConsumer, TickDTO, WatchResult } from './backend';
import { call } from './call';
import { currentBackend } from './current';

export type { QuoteConsumer, WatchResult };
export const QUOTE_CONSUMERS: readonly QuoteConsumer[] = ['chart', 'watchlist', 'widget'];

export async function watchQuotes(consumer: QuoteConsumer, tokens: readonly number[]): Promise<WatchResult> {
  const r = await call(currentBackend().quotes.watch(consumer, [...tokens]));
  return { live: [...(r.live ?? [])], polled: [...(r.polled ?? [])], snapshot: [...(r.snapshot ?? [])] };
}

/** Raw tick.batch access. P0-4's event bridge already feeds useTicksStore; use this only outside that store. */
export function onTickBatch(cb: (ticks: TickDTO[]) => void): () => void {
  return currentBackend().on('tick.batch', cb);
}
