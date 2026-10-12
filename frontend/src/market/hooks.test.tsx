import { act, render } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';

const set = vi.fn();
const clear = vi.fn();
const answer = { polled: false, atMs: 0 };
vi.mock('./index', () => ({
  subscriptions: {
    set: (...a: unknown[]) => set(...a), clear: (...a: unknown[]) => clear(...a), subscribe: () => () => {},
    isPolled: () => answer.polled, answeredAtMs: () => answer.atMs,
  },
}));

import { useTicksStore } from '../store/ticks';
import { usePolled, useSubscription } from './hooks';
import { useQuote, useQuoteStale, runStartedMs } from './quotes';
import { tick } from './testing';

afterEach(() => { answer.polled = false; answer.atMs = 0; set.mockClear(); clear.mockClear(); });

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

describe('usePolled (Ruling 95 I3)', () => {
  it("a tick at or after Go's Watch answer decides; an older tick does not override the answer", () => {
    answer.polled = true;
    answer.atMs = 1_000;
    function Hint() { return <span>{String(usePolled(1))}</span>; }
    const { container } = render(<Hint />);
    expect(container.textContent).toBe('true'); // before any tick: Go's answer
    apply([1, 10, 9, { polled: false, tsMs: 999 }]);
    expect(container.textContent).toBe('true');
    apply([1, 11, 9, { polled: false, tsMs: 1_000 }]);
    expect(container.textContent).toBe('false'); // the feed carries it live now
    apply([1, 12, 9, { polled: true, tsMs: 2_000 }]);
    expect(container.textContent).toBe('true');
  });
});

describe('useSubscription', () => {
  it('registers the source set, re-sends only when the token list changes, clears on unmount', () => {
    function Src({ tokens }: { tokens: number[] }) { useSubscription('wl', 'watchlist', tokens); return null; }
    const { rerender, unmount } = render(<Src tokens={[1, 2]} />);
    expect(set).toHaveBeenLastCalledWith(expect.stringMatching(/^wl#/), 'watchlist', [1, 2]);
    const id = set.mock.calls[0][0];
    rerender(<Src tokens={[1, 2]} />);
    expect(set).toHaveBeenCalledTimes(1);
    rerender(<Src tokens={[]} />);
    expect(set).toHaveBeenLastCalledWith(id, 'watchlist', []);
    unmount();
    expect(clear).toHaveBeenCalledWith(id);
  });

  it('M3: two mounted components with the same sourceId get distinct coordinator sources', () => {
    function Src({ tokens }: { tokens: number[] }) { useSubscription('row', 'watchlist', tokens); return null; }
    const { unmount } = render(<><Src tokens={[1]} /><Src tokens={[2]} /></>);
    const ids = set.mock.calls.map((c) => c[0]);
    expect(ids).toHaveLength(2);
    expect(new Set(ids).size).toBe(2);
    unmount();
    expect(clear.mock.calls.map((c) => c[0]).sort()).toEqual([...ids].sort());
  });
});
