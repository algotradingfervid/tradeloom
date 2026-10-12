import { describe, expect, it, vi } from 'vitest';
import type { TickDTO } from '../api/backend';
import { tickFixture } from '../api/mock/fixtures';
import { useStatusStore } from './status';
import { useTicksStore } from './ticks';

it('applyBatch replaces only the tokens in the batch (unchanged rows keep identity)', () => {
  useTicksStore.getState().applyBatch([tickFixture(1, 10, 9), tickFixture(2, 20, 19)]);
  const before = useTicksStore.getState().latest;
  useTicksStore.getState().applyBatch([tickFixture(2, 21, 19)]);
  const after = useTicksStore.getState().latest;
  expect(after[1]).toBe(before[1]);
  expect(after[2].ltp).toBe(21);
});

// Ruling 92b: one ordering rule for every writer, matching Go's tick.batch coalescing (internal/service/events.go).
describe('applyBatch ordering (Ruling 92b)', () => {
  const t = (token: number, ltp: number, tsMs: number, polled = false): TickDTO => ({ ...tickFixture(token, ltp, 100), tsMs, polled });
  const apply = (...ticks: TickDTO[]) => useTicksStore.getState().applyBatch(ticks);
  const ltp = (token: number) => useTicksStore.getState().latest[token]?.ltp;
  const goLive = () => useStatusStore.setState({ feed: { ...useStatusStore.getState().feed, state: 'live' } });
  const goOffline = () => useStatusStore.setState({ feed: { ...useStatusStore.getState().feed, state: 'offline' } });

  it('a tick with a lower tsMs in a later batch is rejected; a higher one wins', () => {
    apply(t(1, 10, 200));
    apply(t(1, 9, 100));
    expect(ltp(1)).toBe(10);
    apply(t(1, 11, 300));
    expect(ltp(1)).toBe(11);
  });

  it('equal tsMs: a live tick beats a polled one, in either order', () => {
    apply(t(1, 10, 200, true));
    apply(t(1, 11, 200, false));
    expect(ltp(1)).toBe(11);
    apply(t(1, 12, 200, true));
    expect(ltp(1)).toBe(11);
  });

  it('equal tsMs and the same flag: the later arrival wins', () => {
    apply(t(1, 10, 200));
    apply(t(1, 11, 200));
    expect(ltp(1)).toBe(11);
    apply(t(2, 20, 200, true));
    apply(t(2, 21, 200, true));
    expect(ltp(2)).toBe(21);
  });

  it('duplicates within one batch follow the same rule', () => {
    apply(t(1, 10, 300), t(1, 9, 100));
    expect(ltp(1)).toBe(10);
    apply(t(2, 20, 200, false), t(2, 21, 200, true));
    expect(ltp(2)).toBe(20);
    apply(t(3, 30, 200, true), t(3, 31, 200, false));
    expect(ltp(3)).toBe(31);
    apply(t(4, 40, 200), t(4, 41, 200));
    expect(ltp(4)).toBe(41);
  });

  it('a fully rejected batch keeps the same state and notifies nobody', () => {
    apply(t(1, 10, 200), t(2, 20, 200));
    const before = useTicksStore.getState();
    const listener = vi.fn();
    const off = useTicksStore.subscribe(listener);
    apply(t(1, 9, 100), t(2, 19, 199, true));
    off();
    expect(listener).not.toHaveBeenCalled();
    expect(useTicksStore.getState()).toBe(before);
  });

  it('self-heal: after the feed turns live the first tick per token is accepted whatever its tsMs', () => {
    goOffline();
    apply(t(1, 10, 9_999_999_999_999), t(2, 20, 9_999_999_999_999)); // future-dated guard (bad clock)
    apply(t(1, 11, 500));
    expect(ltp(1)).toBe(10);
    goLive();
    apply(t(1, 12, 500));
    expect(ltp(1)).toBe(12);
    apply(t(1, 13, 400));
    expect(ltp(1)).toBe(12); // only the first tick per token heals; the normal rule applies after it
    apply(t(2, 22, 450));
    expect(ltp(2)).toBe(22); // each token heals once
    goLive(); // no transition (already live): nothing re-arms
    apply(t(2, 23, 100));
    expect(ltp(2)).toBe(22);
  });
});
