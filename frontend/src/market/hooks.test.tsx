import { act, render } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';

const set = vi.fn();
const clear = vi.fn();
vi.mock('./index', () => ({
  subscriptions: { set: (...a: unknown[]) => set(...a), clear: (...a: unknown[]) => clear(...a), subscribe: () => () => {}, isPolled: () => false },
}));

import { useTicksStore } from '../store/ticks';
import { usePolled, useSubscription } from './hooks';
import { useQuote, useQuoteStale, runStartedMs } from './quotes';
import { tick } from './testing';

const apply = (...t: Parameters<typeof tick>[]) => act(() => useTicksStore.getState().applyBatch(t.map((a) => tick(...a))));

describe('selectors re-render only for their own token', () => {
  it('useQuote: a tick for another token does not re-render', () => {
    let renders = 0;
    function Row() { renders++; const q = useQuote(1); return <span>{q?.ltp ?? '—'}</span>; }
    const { container } = render(<Row />);
    expect(renders).toBe(1);
    apply([2, 20, 19], [3, 30, 29]);
    expect(renders).toBe(1);
    apply([1, 10, 9]);
    expect(renders).toBe(2);
    expect(container.textContent).toBe('10');
    apply([2, 21, 19]);
    expect(renders).toBe(2);
  });

  it('usePolled and useQuoteStale: unrelated ticks do not re-render', () => {
    let renders = 0;
    function Hint() { renders++; const p = usePolled(1); const s = useQuoteStale(1); return <span>{`${p}/${s}`}</span>; }
    const { container } = render(<Hint />);
    apply([2, 20, 19, { polled: true }]);
    expect(renders).toBe(1);
    apply([1, 10, 9, { polled: true, tsMs: runStartedMs() - 1 }]);
    expect(container.textContent).toBe('true/true');
    const r = renders;
    apply([2, 22, 19, { polled: true }]);
    expect(renders).toBe(r);
    apply([1, 11, 9, { polled: false, tsMs: runStartedMs() + 1 }]);
    expect(container.textContent).toBe('false/false');
  });
});

describe('useSubscription', () => {
  it('registers the source set, re-sends only when the token list changes, clears on unmount', () => {
    function Src({ tokens }: { tokens: number[] }) { useSubscription('wl', 'watchlist', tokens); return null; }
    const { rerender, unmount } = render(<Src tokens={[1, 2]} />);
    expect(set).toHaveBeenLastCalledWith('wl', 'watchlist', [1, 2]);
    rerender(<Src tokens={[1, 2]} />);
    expect(set).toHaveBeenCalledTimes(1);
    rerender(<Src tokens={[]} />);
    expect(set).toHaveBeenLastCalledWith('wl', 'watchlist', []);
    unmount();
    expect(clear).toHaveBeenCalledWith('wl');
  });
});
