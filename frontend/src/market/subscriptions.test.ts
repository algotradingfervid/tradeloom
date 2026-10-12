import { afterEach, beforeEach, describe, expect, it, vi, type Mock } from 'vitest';
import type { TickDTO } from '../api/backend';
import { BindingError } from '../api/call';
import { RETRY_BACKOFF_MS, SubscriptionCoordinator, type WatchFailureLog } from './subscriptions';
import type { QuoteConsumer, WatchResult } from '../api/quotes';
import { tick } from './testing';

let watch: Mock<(c: QuoteConsumer, tokens: number[]) => Promise<WatchResult>>;
let onSnapshot: Mock<(quotes: TickDTO[]) => void>;
let log: Mock<(message: string, fields: WatchFailureLog) => void>;
let c: SubscriptionCoordinator;
const allLive = async (_c: string, tokens: number[]): Promise<WatchResult> => ({ live: tokens, polled: [], snapshot: [] });
const step = (ms: number) => vi.advanceTimersByTimeAsync(ms);

beforeEach(() => {
  vi.useFakeTimers({ now: Date.UTC(2026, 9, 9, 4, 0) });
  watch = vi.fn(allLive);
  onSnapshot = vi.fn();
  log = vi.fn();
  c = new SubscriptionCoordinator({ watch, onSnapshot, log });
});
// M2: run what is still pending on the fake clock before switching back, so no timer leaks into the next test.
afterEach(async () => {
  await vi.runOnlyPendingTimersAsync();
  vi.useRealTimers();
});

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

  it('I1: after a refused send, clearing the last source still sends the empty set (Go may hold the old one)', async () => {
    c = new SubscriptionCoordinator({ watch, onSnapshot, log, releaseDelayMs: 0 });
    c.set('wl', 'watchlist', [1]);
    await step(50);
    watch.mockRejectedValueOnce(new BindingError('subscription_failed', 'boom', 'quotes'));
    c.set('wl', 'watchlist', [1, 2]);
    await step(50);
    c.clear('wl');
    await step(50);
    expect(watch).toHaveBeenLastCalledWith('watchlist', []);
  });

  it('I1: an empty set is not sent when Go cannot hold anything; resendAll keeps what Go may hold', async () => {
    c = new SubscriptionCoordinator({ watch, onSnapshot, log, releaseDelayMs: 0 });
    c.set('wl', 'watchlist', []);
    await step(50);
    expect(watch).not.toHaveBeenCalled();
    c.set('wl', 'watchlist', [1]);
    await step(50);
    c.resendAll();
    c.clear('wl');
    await step(50);
    expect(watch).toHaveBeenLastCalledWith('watchlist', []);
    expect(watch).toHaveBeenCalledTimes(2);
    c.resendAll();
    await step(50);
    expect(watch).toHaveBeenCalledTimes(2); // Go now holds nothing for this consumer
  });

  it('I2: a refused Watch is logged without its reason and retried after 2, 5 and 10 s, then waits for a change', async () => {
    watch.mockRejectedValue(new BindingError('not_ready', 'token abc123 expired', 'quotes'));
    c.set('wl', 'watchlist', [1]);
    await step(50);
    expect(watch).toHaveBeenCalledTimes(1);
    expect(log).toHaveBeenCalledWith(expect.any(String), expect.objectContaining({ consumer: 'watchlist', code: 'not_ready', stage: 'quotes' }));
    expect(JSON.stringify(log.mock.calls)).not.toContain('abc123');
    expect(RETRY_BACKOFF_MS).toEqual([2000, 5000, 10000]);
    await step(1_900);
    expect(watch).toHaveBeenCalledTimes(1);
    await step(200);
    expect(watch).toHaveBeenCalledTimes(2);
    await step(5_100);
    expect(watch).toHaveBeenCalledTimes(3);
    await step(10_100);
    expect(watch).toHaveBeenCalledTimes(4);
    await step(120_000);
    expect(watch).toHaveBeenCalledTimes(4); // bounded: no retry loop
    watch.mockImplementation(allLive);
    c.resendAll(); // the feed went live again
    await step(50);
    expect(watch).toHaveBeenCalledTimes(5);
    await step(60_000);
    expect(watch).toHaveBeenCalledTimes(5);
  });

  it('I2: a retry that succeeds ends the backoff', async () => {
    watch.mockRejectedValueOnce(new Error('{"code":"offline","stage":"quotes","reason":"x"}'));
    c.set('wl', 'watchlist', [1]);
    await step(2_100);
    expect(watch).toHaveBeenCalledTimes(2);
    await step(60_000);
    expect(watch).toHaveBeenCalledTimes(2);
  });

  it('I3/M4: each token remembers when its polled status was decided; an unchanged answer notifies nobody', async () => {
    watch.mockImplementation(async (_c: string, tokens: number[]) => ({ live: tokens.filter((t) => t !== 9), polled: tokens.filter((t) => t === 9), snapshot: [] }));
    const listener = vi.fn();
    c.subscribe(listener);
    const t0 = Date.now();
    c.set('wl', 'watchlist', [1, 9]);
    await step(50);
    expect(c.isPolled(9)).toBe(true);
    expect(c.answeredAtMs(9)).toBeGreaterThanOrEqual(t0);
    const at9 = c.answeredAtMs(9);
    expect(listener).toHaveBeenCalledTimes(1);
    c.set('wl', 'watchlist', [1, 9, 2]);
    await step(50);
    expect(listener).toHaveBeenCalledTimes(2); // token 2 is new
    expect(c.answeredAtMs(9)).toBe(at9); // 9's status did not change
    c.resendAll();
    await step(50);
    expect(watch).toHaveBeenCalledTimes(3);
    expect(listener).toHaveBeenCalledTimes(2); // same sets: no notify
    expect(c.answeredAtMs(404)).toBe(0);
  });
});

describe('SubscriptionCoordinator as the app configures it (releaseDelayMs: 0, Ruling 92a)', () => {
  beforeEach(() => {
    c = new SubscriptionCoordinator({ watch, onSnapshot, log, releaseDelayMs: 0 });
  });

  it('M1: remove and re-add sends both sets (Go lingers), but a flip inside one debounce sends nothing', async () => {
    c.set('wl', 'watchlist', [1]);
    await step(50);
    c.set('wl', 'watchlist', []);
    await step(50);
    expect(watch).toHaveBeenLastCalledWith('watchlist', []);
    c.set('wl', 'watchlist', [1]);
    await step(50);
    expect(watch).toHaveBeenCalledTimes(3);
    c.set('wl', 'watchlist', []);
    c.set('wl', 'watchlist', [1]);
    await step(50);
    expect(watch).toHaveBeenCalledTimes(3);
  });

  it('M1: a token shared by two sources stays until the last source releases it', async () => {
    c.set('a', 'watchlist', [1, 2]);
    c.set('b', 'watchlist', [2, 3]);
    await step(50);
    c.clear('a');
    await step(50);
    expect(watch).toHaveBeenLastCalledWith('watchlist', [2, 3]);
    c.clear('b');
    await step(50);
    expect(watch).toHaveBeenLastCalledWith('watchlist', []);
  });
});
