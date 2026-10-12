import { act, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';

const h = vi.hoisted(() => ({ shortcuts: {} as Record<string, () => void>, sub: vi.fn(), polledAbove: Number.POSITIVE_INFINITY }));
// The mocked useShortcut also drops its handler on unmount, like the real one (the reorder shortcuts exist only while a row has focus).
vi.mock('../../shortcuts/ShortcutProvider', async () => {
  const { useEffect } = await import('react');
  return {
    useShortcut: (id: string, fn: () => void) => {
      h.shortcuts[id] = fn;
      useEffect(() => () => { if (h.shortcuts[id] === fn) delete h.shortcuts[id]; });
    },
  };
});
vi.mock('../../market/hooks', () => ({ useSubscription: (...a: unknown[]) => h.sub(...a), usePolled: (t: number) => t > h.polledAbove }));

import { WatchlistRows } from './WatchlistRows';
import type { Watchlist, WatchlistItem } from '../../api/backend';
import { setBackend } from '../../api/current';
import { demoSeed, FIXED_NOW } from '../../api/mock/fixtures';
import { createMockBackend, type MockBackend } from '../../api/mock/index';
import { watchlistItemOf } from '../../api/mock/market';
import { subscriptions } from '../../market/index';
import { tick } from '../../market/testing';
import { setRunStartedMs } from '../../market/quotes';
import { resetOnLogout } from '../../store/reset';
import { useTicksStore } from '../../store/ticks';
import { TOAST_DURATION_MS, useToastStore } from '../../store/toasts';
import { tokensOf, useWatchlists } from '../../store/watchlistsStore';
import { RELIANCE, SBIN, TCS } from '../../instruments/fixtures';

let backend: MockBackend;
const synthetic = (n: number): WatchlistItem[] => Array.from({ length: n }, (_, i) => ({
  token: 10_000 + i, exchange: 'NSE', symbol: `SYM${i}`, displayName: `SYM${i}`, name: `SYNTHETIC ${i}`, isIndex: false, missing: false,
}));
const seed = (items: WatchlistItem[]) => {
  const list: Watchlist = { id: 'a', name: 'Watchlist 1', items };
  backend.state.watchlists = JSON.parse(JSON.stringify({ lists: [list], activeId: 'a' }));
  useWatchlists.setState({ lists: [list], activeId: 'a', selectedToken: null, status: 'ready', error: null });
};
const current = () => useWatchlists.getState().lists[0];
const renderRows = (onChart = vi.fn()) => render(<WatchlistRows list={current()} onChartItem={onChart} />);
/** Re-renders with the store's list, as the panel does (Task 16). */
function LiveRows() {
  const list = useWatchlists((s) => s.lists[0]);
  return <WatchlistRows list={list} onChartItem={vi.fn()} />;
}
const items = (...insts: (typeof SBIN)[]) => insts.map(watchlistItemOf);

beforeEach(() => {
  vi.clearAllMocks();
  h.sub.mockReset();
  h.shortcuts = {};
  h.polledAbove = Number.POSITIVE_INFINITY;
  backend = createMockBackend(demoSeed());
  setBackend(backend);
});

describe('WatchlistRows', () => {
  it('FR-WATCH-001 AC1: empty list hint', () => {
    seed([]);
    renderRows();
    expect(screen.getByText('Search to add instruments')).toBeInTheDocument();
  });

  it('FR-WATCH-001 AC3: 60 instruments all render, no cap message', () => {
    seed(synthetic(60));
    renderRows();
    expect(screen.getAllByRole('row')).toHaveLength(61); // header + 60
    expect(screen.queryByText(/limit|cap|maximum/i)).toBeNull();
  });

  it('FR-WATCH-006: subscribes the visible list as the watchlist consumer, and re-subscribes on list switch', () => {
    seed(items(SBIN, TCS));
    const { rerender } = renderRows();
    expect(h.sub).toHaveBeenLastCalledWith('watchlist-panel', 'watchlist', [SBIN.token, TCS.token]);
    rerender(<WatchlistRows list={{ id: 'b', name: 'B', items: items(RELIANCE) }} onChartItem={vi.fn()} />);
    expect(h.sub).toHaveBeenLastCalledWith('watchlist-panel', 'watchlist', [RELIANCE.token]);
  });

  it('FR-WATCH-006: no subscription leak — the real coordinator drops the old set on list change and everything on unmount', async () => {
    const actual = await vi.importActual<typeof import('../../market/hooks')>('../../market/hooks');
    h.sub.mockImplementation(actual.useSubscription);
    seed(items(SBIN, TCS));
    const watched = () => backend.callsTo('quotes.watch').filter((c) => c.args[0] === 'watchlist').at(-1)?.args[1];
    const { rerender, unmount } = renderRows();
    await waitFor(() => expect(watched()).toEqual([SBIN.token, TCS.token]));
    rerender(<WatchlistRows list={{ id: 'b', name: 'B', items: items(RELIANCE) }} onChartItem={vi.fn()} />);
    await waitFor(() => expect(watched()).toEqual([RELIANCE.token]));
    unmount();
    await waitFor(() => expect(watched()).toEqual([]));
    await subscriptions.idle();
  });

  it('FR-WATCH-005 AC1: clicking the symbol charts it and highlights the row', () => {
    seed(items(SBIN, RELIANCE));
    const onChart = vi.fn();
    renderRows(onChart);
    fireEvent.click(screen.getByRole('button', { name: /^RELIANCE/ }));
    expect(onChart).toHaveBeenCalledWith(watchlistItemOf(RELIANCE));
    expect(screen.getByRole('row', { name: /RELIANCE/ })).toHaveAttribute('aria-selected', 'true');
    expect(screen.getByRole('row', { name: /SBIN/ })).toHaveAttribute('aria-selected', 'false');
  });

  it('FR-WATCH-005 AC3: remove shows a 5 s Undo toast that restores the row at its position', async () => {
    seed(items(SBIN, TCS, RELIANCE));
    renderRows();
    fireEvent.click(screen.getByRole('button', { name: 'Remove TCS' }));
    await waitFor(() => expect(useToastStore.getState().visible).toHaveLength(1));
    const toast = useToastStore.getState().visible[0];
    expect(toast).toMatchObject({ level: 'info', message: 'Removed TCS from Watchlist 1', action: { label: 'Undo' } });
    expect(TOAST_DURATION_MS.info).toBe(5000);
    expect(tokensOf(current())).toEqual([SBIN.token, RELIANCE.token]);
    await act(async () => { toast.action!.onSelect(); });
    await waitFor(() => expect(tokensOf(current())).toEqual([SBIN.token, TCS.token, RELIANCE.token]));
    expect(backend.callsTo('watchlists.add').at(-1)!.args).toEqual(['a', TCS.token, 1]);
  });

  it('Ruling 104 m3: a remove that finishes after a sign-out shows no Undo toast', async () => {
    seed(items(SBIN, TCS));
    renderRows();
    fireEvent.click(screen.getByRole('button', { name: 'Remove TCS' }));
    act(() => resetOnLogout());
    await waitFor(() => expect(backend.callsTo('watchlists.remove')).toHaveLength(1));
    await act(async () => { await Promise.resolve(); });
    expect(useToastStore.getState().visible).toEqual([]);
  });

  it('a refused remove puts the row back and reports the error', async () => {
    seed(items(SBIN, TCS));
    renderRows();
    backend.failNext('watchlists.remove', 'That instrument is not in the list.', 'not_found');
    fireEvent.click(screen.getByRole('button', { name: 'Remove TCS' }));
    await waitFor(() => expect(useToastStore.getState().visible[0]).toMatchObject({ level: 'error' }));
    expect(useToastStore.getState().visible[0].action).toBeUndefined();
    expect(tokensOf(current())).toEqual([SBIN.token, TCS.token]);
  });

  it('FR-WATCH-001 AC4: Alt+Up (registry action watchlist.moveUp) moves the focused row', async () => {
    seed(items(SBIN, TCS));
    renderRows();
    fireEvent.focus(screen.getByRole('row', { name: /TCS/ }));
    await act(async () => { h.shortcuts['watchlist.moveUp'](); });
    expect(backend.callsTo('watchlists.reorder')[0].args).toEqual(['a', [TCS.token, SBIN.token]]);
  });

  it('the reorder shortcuts exist only while a row has focus, and focus follows the moved row', async () => {
    seed(items(SBIN, TCS, RELIANCE));
    const outside = document.createElement('button');
    document.body.append(outside);
    render(<LiveRows />);
    expect(h.shortcuts['watchlist.moveDown']).toBeUndefined();
    const sbin = screen.getByRole('row', { name: /SBIN/ });
    act(() => sbin.focus());
    expect(h.shortcuts['watchlist.moveDown']).toBeDefined();
    await act(async () => { h.shortcuts['watchlist.moveDown'](); });
    await act(async () => { h.shortcuts['watchlist.moveDown'](); });
    await waitFor(() => expect(tokensOf(current())).toEqual([TCS.token, RELIANCE.token, SBIN.token]));
    expect(document.activeElement).toBe(screen.getByRole('row', { name: /SBIN/ }));
    expect(screen.getAllByRole('row').slice(1).map((r) => r.getAttribute('data-token'))).toEqual([TCS, RELIANCE, SBIN].map((i) => String(i.token)));
    act(() => outside.focus());
    expect(h.shortcuts['watchlist.moveDown']).toBeUndefined();
    outside.remove();
  });

  it('drag-and-drop reorders', async () => {
    seed(items(SBIN, TCS, RELIANCE));
    renderRows();
    const from = screen.getByRole('row', { name: /RELIANCE/ });
    const to = screen.getByRole('row', { name: /SBIN/ });
    fireEvent.dragStart(from);
    fireEvent.dragOver(to);
    fireEvent.drop(to);
    await waitFor(() => expect(backend.callsTo('watchlists.reorder')[0]?.args).toEqual(['a', [RELIANCE.token, SBIN.token, TCS.token]]));
  });

  it('rows beyond the budget show the polled hint', () => {
    h.polledAbove = 10_000 + 199;
    seed(synthetic(250));
    renderRows();
    expect(screen.getAllByText('polled')).toHaveLength(50);
    const row200 = screen.getByRole('row', { name: /SYM200/ });
    expect(within(row200).getByText('polled')).toHaveAttribute('title', 'Live subscription limit reached — updates every 5 s');
    expect(within(screen.getByRole('row', { name: /SYM0\b/ })).queryByText('polled')).toBeNull();
  });

  it('Ruling 93: a quote from a previous run shows a text stale hint until a fresh tick arrives', () => {
    setRunStartedMs(FIXED_NOW);
    useTicksStore.getState().applyBatch([tick(SBIN.token, 800, 790, { tsMs: FIXED_NOW - 86_400_000 })]);
    seed(items(SBIN, TCS));
    renderRows();
    const row = screen.getByRole('row', { name: /SBIN/ });
    expect(within(row).getByText('last known')).toHaveAttribute('title', 'Last known price, not live yet');
    expect(within(screen.getByRole('row', { name: /TCS/ })).queryByText('last known')).toBeNull();
    act(() => useTicksStore.getState().applyBatch([tick(SBIN.token, 801, 790, { tsMs: FIXED_NOW + 1000 })]));
    expect(within(row).queryByText('last known')).toBeNull();
  });

  it('an instrument no longer in the master (missing item, e.g. expired contract) shows as unavailable, is not subscribed and can be removed', async () => {
    seed([watchlistItemOf(SBIN), { token: 424242, exchange: 'NFO', symbol: 'OLDFUT', displayName: '', name: '', isIndex: false, missing: true }]);
    renderRows();
    expect(screen.getByText('Instrument no longer available')).toBeInTheDocument();
    expect(h.sub).toHaveBeenLastCalledWith('watchlist-panel', 'watchlist', [SBIN.token]);
    fireEvent.click(screen.getByRole('button', { name: 'Remove unavailable instrument' }));
    await waitFor(() => expect(backend.callsTo('watchlists.remove')[0]?.args).toEqual(['a', 424242]));
  });

  it('a missing item in Go\'s shape (no exchange or symbol) renders without throwing', () => {
    seed([{ token: 424242, exchange: '', symbol: '', displayName: '', name: '', isIndex: false, missing: true }, watchlistItemOf(TCS)]);
    renderRows();
    expect(screen.getByText('Instrument no longer available')).toBeInTheDocument();
    expect(screen.getByRole('row', { name: /TCS/ })).toBeInTheDocument();
  });

  it('no order buttons (G-0)', () => {
    seed(items(SBIN, TCS));
    renderRows();
    expect(screen.queryByRole('button', { name: /\b(buy|sell)\b|^[BS]$/i })).toBeNull();
  });
});
