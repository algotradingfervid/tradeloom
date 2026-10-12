import { render, waitFor } from '@testing-library/react';
import { StrictMode } from 'react';
import { describe, expect, it, vi } from 'vitest';
import type { QuoteConsumer, WatchResult } from '../api/quotes';

// The real coordinator and the real useSubscription, with a recording Quotes.Watch: what Go would be told.
const watch = vi.hoisted(() => vi.fn(async (_c: QuoteConsumer, tokens: number[]): Promise<WatchResult> => ({ live: tokens, polled: [], snapshot: [] })));
vi.mock('../market/index', async () => {
  const { SubscriptionCoordinator } = await import('../market/subscriptions');
  return { subscriptions: new SubscriptionCoordinator({ watch, onSnapshot: () => {}, releaseDelayMs: 0, debounceMs: 0 }) };
});
vi.mock('react-router-dom', () => ({ useNavigate: () => vi.fn() }));
vi.mock('../api/instruments', async () => {
  const f = await import('../instruments/fixtures');
  return { instrumentByIdentifier: vi.fn(async (id: string) => f.ALL_IN_SCOPE.find((i) => `${i.exchange}:${i.tradingsymbol}` === id)), instrumentsByTokens: vi.fn(async () => []) };
});

import { IndexTickerStrip } from './IndexTickerStrip';
import { NIFTY50, NIFTYBANK, SENSEX } from '../instruments/fixtures';

const lastWidget = () => watch.mock.calls.filter(([c]) => c === 'widget').at(-1)?.[1];

describe('IndexTickerStrip subscriptions', () => {
  it('unmounting releases every index token (no leaked widget subscription), also under StrictMode', async () => {
    for (let round = 0; round < 3; round += 1) {
      const { unmount } = render(<StrictMode><IndexTickerStrip /></StrictMode>);
      await waitFor(() => expect(lastWidget()?.slice().sort((a, b) => a - b)).toEqual([NIFTY50.token, NIFTYBANK.token, SENSEX.token].sort((a, b) => a - b)));
      unmount();
      await waitFor(() => expect(lastWidget()).toEqual([]));
    }
    expect(watch.mock.calls.every(([c]) => c === 'widget')).toBe(true);
  });
});
