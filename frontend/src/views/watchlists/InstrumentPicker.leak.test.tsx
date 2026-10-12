import { act, fireEvent, render, screen } from '@testing-library/react';
import { StrictMode } from 'react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { QuoteConsumer, WatchResult } from '../../api/quotes';

// The real coordinator and the real useSubscription, with a recording Quotes.Watch: what Go would be told.
const watch = vi.hoisted(() => vi.fn(async (_c: QuoteConsumer, tokens: number[]): Promise<WatchResult> => ({ live: tokens, polled: [], snapshot: [] })));
vi.mock('../../market/index', async () => {
  const { SubscriptionCoordinator } = await import('../../market/subscriptions');
  return { subscriptions: new SubscriptionCoordinator({ watch, onSnapshot: () => {}, releaseDelayMs: 0, debounceMs: 0 }) };
});
vi.mock('../../api/instruments', async () => {
  const f = await import('../../instruments/fixtures');
  return {
    searchInstruments: vi.fn(async (q: string) => f.ALL_IN_SCOPE.filter((i) => i.tradingsymbol.toLowerCase().includes(q.toLowerCase()))),
    instrumentsByTokens: vi.fn(async () => []),
  };
});

import { InstrumentPicker } from './InstrumentPicker';
import { SEARCH_DEBOUNCE_MS } from '../../instruments/useInstrumentSearch';
import { RELIANCE, SBIN, TCS } from '../../instruments/fixtures';

const lastWidget = () => watch.mock.calls.filter(([c]) => c === 'widget').at(-1)?.[1];
const sorted = (t: readonly number[] | undefined) => [...(t ?? [])].sort((a, b) => a - b);
const flush = (ms = 0) => act(async () => { await vi.advanceTimersByTimeAsync(ms); });

beforeEach(() => { vi.useFakeTimers(); watch.mockClear(); });
afterEach(() => { vi.useRealTimers(); });

describe('InstrumentPicker subscriptions', () => {
  it('closing the picker releases every selected token (no leaked widget subscription), also under StrictMode', async () => {
    const props = { onClose: () => {}, onDone: () => {}, initial: [SBIN, RELIANCE] };
    const { rerender } = render(<StrictMode><InstrumentPicker open {...props} /></StrictMode>);
    for (let round = 0; round < 3; round += 1) {
      await flush();
      expect(sorted(lastWidget())).toEqual(sorted([SBIN.token, RELIANCE.token]));
      fireEvent.change(screen.getByRole('textbox', { name: 'Search instruments' }), { target: { value: 'tcs' } });
      await flush(SEARCH_DEBOUNCE_MS);
      fireEvent.click(screen.getByRole('checkbox', { name: 'Select TCS' }));
      await flush();
      expect(sorted(lastWidget())).toEqual(sorted([SBIN.token, RELIANCE.token, TCS.token]));
      rerender(<StrictMode><InstrumentPicker open={false} {...props} /></StrictMode>);
      await flush();
      expect(lastWidget()).toEqual([]);
      if (round < 2) rerender(<StrictMode><InstrumentPicker open {...props} /></StrictMode>);
    }
    expect(watch.mock.calls.every(([c]) => c === 'widget')).toBe(true);
  });

  it('unmounting while open releases the selection too', async () => {
    const { unmount } = render(<InstrumentPicker open initial={[TCS]} onClose={() => {}} onDone={() => {}} />);
    await flush();
    expect(lastWidget()).toEqual([TCS.token]);
    unmount();
    await flush();
    expect(lastWidget()).toEqual([]);
  });
});
