import { afterEach, beforeEach, describe, expect, it, vi, type Mock } from 'vitest';
import type { TickDTO } from '../api/backend';
import { SubscriptionCoordinator } from './subscriptions';
import type { QuoteConsumer, WatchResult } from '../api/quotes';
import { tick } from './testing';

let watch: Mock<(c: QuoteConsumer, tokens: number[]) => Promise<WatchResult>>;
let onSnapshot: Mock<(quotes: TickDTO[]) => void>;
let c: SubscriptionCoordinator;
const allLive = async (_c: string, tokens: number[]): Promise<WatchResult> => ({ live: tokens, polled: [], snapshot: [] });
const step = (ms: number) => vi.advanceTimersByTimeAsync(ms);

beforeEach(() => {
  vi.useFakeTimers({ now: Date.UTC(2026, 9, 9, 4, 0) });
  watch = vi.fn(allLive);
  onSnapshot = vi.fn();
  c = new SubscriptionCoordinator({ watch, onSnapshot });
});
afterEach(() => vi.useRealTimers());

describe('SubscriptionCoordinator', () => {
  it('merges sources of one consumer into a single debounced Watch call', async () => {
    c.set('a', 'watchlist', [1, 2]);
    c.set('b', 'watchlist', [2, 3]);
    await step(50);
    expect(watch).toHaveBeenCalledTimes(1);
    expect(watch).toHaveBeenCalledWith('watchlist', [1, 2, 3]);
  });

  it('FR-WATCH-006 AC1: list switch subscribes B now and drops A-only tokens after 30 s', async () => {
    c.set('wl', 'watchlist', [1, 2]);
    await step(50);
    c.set('wl', 'watchlist', [3]);
    await step(50);
    expect(watch).toHaveBeenLastCalledWith('watchlist', [1, 2, 3]);
    await step(29_000);
    expect(watch).toHaveBeenCalledTimes(2);
    await step(1_100);
    expect(watch).toHaveBeenLastCalledWith('watchlist', [3]);
  });

  it('re-adding within 30 s cancels the release (no churn)', async () => {
    c.set('wl', 'watchlist', [1]);
    await step(50);
    c.set('wl', 'watchlist', []);
    await step(10_000);
    c.set('wl', 'watchlist', [1]);
    await step(40_000);
    expect(watch).toHaveBeenCalledTimes(1);
  });

  it('250 tokens: one Watch call, budget overflow marked polled', async () => {
    watch.mockImplementation(async (_c: string, tokens: number[]) => ({ live: tokens.slice(0, 180), polled: tokens.slice(180), snapshot: [] }));
    const listener = vi.fn();
    c.subscribe(listener);
    const tokens = Array.from({ length: 250 }, (_, i) => i + 1);
    c.set('wl', 'watchlist', tokens);
    await step(50);
    expect(watch).toHaveBeenCalledTimes(1);
    expect(c.isPolled(1)).toBe(false);
    expect(c.isPolled(181)).toBe(true);
    expect(c.isPolled(250)).toBe(true);
    expect(listener).toHaveBeenCalledTimes(1);
    await step(60_000);
    expect(watch).toHaveBeenCalledTimes(1); // polled tokens are not retried as live in a loop
  });

  it('a token live for the chart is not "polled" even if the watchlist consumer was refused', async () => {
    watch.mockImplementation(async (consumer: string, tokens: number[]) =>
      consumer === 'chart' ? { live: tokens, polled: [], snapshot: [] } : { live: [], polled: tokens, snapshot: [] });
    c.set('chart', 'chart', [5]);
    c.set('wl', 'watchlist', [5, 6]);
    await step(50);
    expect(c.isPolled(5)).toBe(false);
    expect(c.isPolled(6)).toBe(true);
  });

  it('hands the last-quote snapshot to the tick store (outside market hours)', async () => {
    const snap = [tick(1, 22600, 22500)];
    watch.mockImplementation(async () => ({ live: [1], polled: [], snapshot: snap }));
    c.set('strip', 'widget', [1]);
    await step(50);
    expect(onSnapshot).toHaveBeenCalledWith(snap);
  });

  it('resendAll re-sends unchanged sets (reconnect)', async () => {
    c.set('wl', 'watchlist', [1, 2]);
    await step(50);
    c.resendAll();
    await step(50);
    expect(watch).toHaveBeenCalledTimes(2);
    expect(watch).toHaveBeenLastCalledWith('watchlist', [1, 2]);
  });

  it('a failed Watch is retried on the next change', async () => {
    watch.mockRejectedValueOnce(new Error('logged out'));
    c.set('wl', 'watchlist', [1]);
    await step(50);
    c.resendAll();
    await step(50);
    expect(watch).toHaveBeenCalledTimes(2);
  });
});
