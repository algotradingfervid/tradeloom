import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import { setBackend } from '../api/current';
import { demoSeed } from '../api/mock/fixtures';
import { createMockBackend, type MockBackend } from '../api/mock/index';
import { subscriptions } from './index';

let backend: MockBackend;
beforeEach(() => {
  vi.useFakeTimers({ now: Date.UTC(2026, 9, 9, 4, 0) });
  backend = createMockBackend(demoSeed());
  setBackend(backend);
});
afterEach(() => {
  subscriptions.clear('t');
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
