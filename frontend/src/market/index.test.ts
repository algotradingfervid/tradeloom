import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import { setBackend } from '../api/current';
import { demoSeed } from '../api/mock/fixtures';
import { createMockBackend, type MockBackend } from '../api/mock/index';
import { useTicksStore } from '../store/ticks';
import { subscriptions } from './index';
import { isStaleQuote, runStartedMs, setRunStartedMs } from './quotes';

let backend: MockBackend;
beforeEach(() => {
  vi.useFakeTimers({ now: Date.UTC(2026, 9, 9, 4, 0) });
  backend = createMockBackend(demoSeed());
  setBackend(backend);
});
afterEach(async () => {
  subscriptions.clear('t');
  await vi.runOnlyPendingTimersAsync();
  vi.useRealTimers();
});

it('the app coordinator sends removals at once: Go itself lingers removed tokens for 30 s (UnsubscribeLinger)', async () => {
  subscriptions.set('t', 'widget', [101, 102]);
  await vi.advanceTimersByTimeAsync(50);
  subscriptions.set('t', 'widget', [101]);
  await vi.advanceTimersByTimeAsync(50);
  await subscriptions.idle();
  const watches = backend.callsTo('quotes.watch');
  expect(watches.at(-1)?.args).toEqual(['widget', [101]]);
});

it('I4: a quote the mock serves in this run is not stale', async () => {
  const started = runStartedMs();
  setRunStartedMs(Date.now()); // the fake clock's "now" is this test's run start
  try {
    subscriptions.set('t', 'widget', [256265]);
    await vi.advanceTimersByTimeAsync(50);
    await subscriptions.idle();
    const q = useTicksStore.getState().latest[256265];
    expect(q).toBeDefined();
    expect(isStaleQuote(q)).toBe(false);
  } finally {
    setRunStartedMs(started);
  }
});
