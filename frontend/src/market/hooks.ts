import { useEffect, useId, useSyncExternalStore } from 'react';
import type { QuoteConsumer } from '../api/quotes';
import { useTicksStore } from '../store/ticks';
import { subscriptions } from './index';

/** Registers this component's token set. The source id is suffixed with useId() so two mounted users of one sourceId never overwrite each other (M3). */
export function useSubscription(sourceId: string, consumer: QuoteConsumer, tokens: readonly number[]): void {
  const id = `${sourceId}#${useId()}`;
  const key = tokens.join(',');
  useEffect(() => {
    subscriptions.set(id, consumer, key ? key.split(',').map(Number) : []);
  }, [id, consumer, key]);
  useEffect(() => () => subscriptions.clear(id), [id]);
}

const subscribeCoordinator = (listener: () => void) => subscriptions.subscribe(listener);

/**
 * Polled = Go's Watch answer put the token on the 5 s poller, unless a tick stamped at or after that answer says
 * otherwise (Ruling 95 I3); with no answer the latest tick decides.
 */
export function usePolled(token: number): boolean {
  const fromWatch = useSyncExternalStore(subscribeCoordinator, () => subscriptions.isPolled(token));
  const answeredAt = useSyncExternalStore(subscribeCoordinator, () => subscriptions.answeredAtMs(token));
  const fromTick = useTicksStore((s) => {
    const t = s.latest[token];
    return t !== undefined && t.tsMs >= answeredAt ? t.polled : undefined;
  });
  return fromTick ?? fromWatch;
}
