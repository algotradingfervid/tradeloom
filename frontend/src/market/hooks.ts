import { useEffect, useSyncExternalStore } from 'react';
import type { QuoteConsumer } from '../api/quotes';
import { useTicksStore } from '../store/ticks';
import { subscriptions } from './index';

export function useSubscription(sourceId: string, consumer: QuoteConsumer, tokens: readonly number[]): void {
  const key = tokens.join(',');
  useEffect(() => {
    subscriptions.set(sourceId, consumer, key ? key.split(',').map(Number) : []);
  }, [sourceId, consumer, key]);
  useEffect(() => () => subscriptions.clear(sourceId), [sourceId]);
}

const subscribeCoordinator = (listener: () => void) => subscriptions.subscribe(listener);

/** Polled = the latest tick says `polled`, or Go's Watch answer put the token on the 5 s poller (before its first tick). */
export function usePolled(token: number): boolean {
  const fromWatch = useSyncExternalStore(subscribeCoordinator, () => subscriptions.isPolled(token));
  const fromTick = useTicksStore((s) => s.latest[token]?.polled === true);
  return fromWatch || fromTick;
}
