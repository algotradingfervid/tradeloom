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
import { pushToast, useToastStore } from '../../store/toasts';
import { useShortcutStore } from '../../shortcuts/store';
import { ToastRegion } from '../../components/ToastRegion';
import { tokensOf, useWatchlists } from '../../store/watchlistsStore';
import type { ShortcutAction } from '../../api/backend';
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
    const err = vi.spyOn(console, 'error').mockImplementation(() => undefined);
    rerender(<WatchlistRows list={{ id: 'b', name: 'B', items: items(RELIANCE) }} onChartItem={vi.fn()} />);
    expect(h.sub).toHaveBeenLastCalledWith('watchlist-panel', 'watchlist', [RELIANCE.token]);
    err.mockRestore();
  });

  it('FR-WATCH-006: no subscription leak — the real coordinator drops the old set on list change and everything on unmount', async () => {
    const actual = await vi.importActual<typeof import('../../market/hooks')>('../../market/hooks');
    h.sub.mockImplementation(actual.useSubscription);
    seed(items(SBIN, TCS));
    const watched = () => backend.callsTo('quotes.watch').filter((c) => c.args[0] === 'watchlist').at(-1)?.args[1];
    const { rerender, unmount } = renderRows();
    await waitFor(() => expect(watched()).toEqual([SBIN.token, TCS.token]));
    const err = vi.spyOn(console, 'error').mockImplementation(() => undefined);
    rerender(<WatchlistRows list={{ id: 'b', name: 'B', items: items(RELIANCE) }} onChartItem={vi.fn()} />);
    await waitFor(() => expect(watched()).toEqual([RELIANCE.token]));
    unmount();
    await waitFor(() => expect(watched()).toEqual([]));
    await subscriptions.idle();
    err.mockRestore();
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

  it('Ruling 86 M4: the Undo toast stays until it is used or dismissed (a plain info toast still goes after 5 s)', async () => {
    seed(items(SBIN, TCS));
    vi.useFakeTimers();
    try {
      render(<><WatchlistRows list={current()} onChartItem={vi.fn()} /><ToastRegion /></>);
      fireEvent.click(screen.getByRole('button', { name: 'Remove TCS' }));
      for (let i = 0; i < 5; i++) await act(async () => { await Promise.resolve(); });
      act(() => { pushToast({ level: 'info', message: 'control' }); });
      act(() => { vi.advanceTimersByTime(6000); });
      expect(screen.queryByText('control')).toBeNull();
      expect(screen.getByText('Removed TCS from Watchlist 1')).toBeInTheDocument();
    } finally {
      vi.useRealTimers();
    }
  });

  it('FR-WATCH-005 AC3: remove shows a 5 s Undo toast that restores the row at its position', async () => {
    seed(items(SBIN, TCS, RELIANCE));
    renderRows();
    fireEvent.click(screen.getByRole('button', { name: 'Remove TCS' }));
    await waitFor(() => expect(useToastStore.getState().visible).toHaveLength(1));
    const toast = useToastStore.getState().visible[0];
    expect(toast).toMatchObject({ level: 'info', message: 'Removed TCS from Watchlist 1', action: { label: 'Undo' } });
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
    // Chromium blurs a focused node that is re-inserted; jsdom does not. Simulate it so React's focus restore is tested.
    const original = Node.prototype.insertBefore;
    let blurred = 0;
    Node.prototype.insertBefore = function <T extends Node>(this: Node, node: T, child: Node | null): T {
      if (node instanceof Element && node.contains(document.activeElement)) { blurred++; (document.activeElement as HTMLElement).blur(); }
      return original.call(this, node, child) as T;
    };
    const restore = () => { Node.prototype.insertBefore = original; };
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
    expect(blurred).toBeGreaterThan(0); // the simulated Chromium blur ran; React restored focus to the moved row
    expect(document.activeElement).toBe(screen.getByRole('row', { name: /SBIN/ }));
    expect(screen.getAllByRole('row').slice(1).map((r) => r.getAttribute('data-token'))).toEqual([TCS, RELIANCE, SBIN].map((i) => String(i.token)));
    act(() => outside.focus());
    expect(h.shortcuts['watchlist.moveDown']).toBeUndefined();
    outside.remove();
    restore();
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
    fireEvent.click(screen.getByRole('button', { name: 'Remove unavailable instrument in row 2' }));
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

  it('I1: removing a missing item shows a toast without Undo (Go refuses unknown_instrument)', async () => {
    seed([watchlistItemOf(SBIN), { token: 424242, exchange: '', symbol: '', displayName: '', name: '', isIndex: false, missing: true }]);
    renderRows();
    fireEvent.click(screen.getByRole('button', { name: 'Remove unavailable instrument in row 2' }));
    await waitFor(() => expect(useToastStore.getState().visible).toHaveLength(1));
    expect(useToastStore.getState().visible[0]).toMatchObject({ level: 'info', message: 'Removed unavailable instrument from Watchlist 1' });
    expect(useToastStore.getState().visible[0].action).toBeUndefined();
  });

  it('I3: roving tabindex — one row in the tab order, symbol buttons out of it; Arrow/Home/End move by row', () => {
    seed(synthetic(60));
    renderRows();
    const table = screen.getByRole('grid');
    const stops = () => [...table.querySelectorAll<HTMLElement>('tr, button')].filter((el) => el.tabIndex >= 0 && (el.tagName === 'TR' || !el.closest('tr')?.matches('[tabindex="-1"]')));
    const rows = () => screen.getAllByRole('row').slice(1);
    expect(rows().filter((r) => r.tabIndex === 0)).toHaveLength(1);
    expect(rows()[0].tabIndex).toBe(0);
    expect(within(rows()[0]).getByRole('button', { name: /^SYM0/ }).tabIndex).toBe(-1);
    expect(stops().length).toBeLessThanOrEqual(3);
    act(() => rows()[0].focus());
    fireEvent.keyDown(rows()[0], { key: 'ArrowDown' });
    expect(document.activeElement).toBe(rows()[1]);
    expect(rows()[1].tabIndex).toBe(0);
    expect(rows()[0].tabIndex).toBe(-1);
    fireEvent.keyDown(rows()[1], { key: 'End' });
    expect(document.activeElement).toBe(rows()[59]);
    fireEvent.keyDown(rows()[59], { key: 'ArrowDown' });
    expect(document.activeElement).toBe(rows()[59]);
    fireEvent.keyDown(rows()[59], { key: 'Home' });
    expect(document.activeElement).toBe(rows()[0]);
    fireEvent.keyDown(rows()[0], { key: 'ArrowUp' });
    expect(document.activeElement).toBe(rows()[0]);
  });

  it('I3: the roving position follows the token when rows change above it; Alt+Arrow is left to the shortcuts', () => {
    seed(items(SBIN, TCS, RELIANCE));
    render(<LiveRows />);
    const tcs = screen.getByRole('row', { name: /TCS/ });
    act(() => tcs.focus());
    const ev = fireEvent.keyDown(tcs, { key: 'ArrowDown', altKey: true });
    expect(ev).toBe(true); // not prevented: the global shortcut listener handles Alt+ArrowDown
    expect(document.activeElement).toBe(tcs);
    act(() => useWatchlists.setState({ lists: [{ ...current(), items: items(RELIANCE, SBIN, TCS) }] }));
    expect(screen.getByRole('row', { name: /TCS/ }).tabIndex).toBe(0);
    expect(screen.getByRole('row', { name: /SBIN/ }).tabIndex).toBe(-1);
  });

  it('m4: rows carry aria-keyshortcuts from the registry, and a move is announced', async () => {
    useShortcutStore.getState().setActions([
      { id: 'watchlist.moveUp', label: 'Move item up', scope: 'view', binding: 'Alt+ArrowUp' },
      { id: 'watchlist.moveDown', label: 'Move item down', scope: 'view', binding: 'Alt+Shift+J' },
    ] as ShortcutAction[]);
    seed(items(SBIN, TCS, RELIANCE));
    render(<LiveRows />);
    const sbin = screen.getByRole('row', { name: /SBIN/ });
    expect(sbin).toHaveAttribute('aria-keyshortcuts', 'Alt+ArrowUp Alt+Shift+J');
    act(() => sbin.focus());
    await act(async () => { h.shortcuts['watchlist.moveDown'](); });
    await act(async () => { h.shortcuts['watchlist.moveDown'](); });
    await waitFor(() => expect(screen.getByRole('status')).toHaveTextContent('SBIN moved to 3 of 3'));
  });

  it('m4: remove buttons of missing rows have distinct names', () => {
    const gone = (token: number) => ({ token, exchange: '', symbol: '', displayName: '', name: '', isIndex: false, missing: true });
    seed([gone(1), watchlistItemOf(SBIN), gone(2)]);
    renderRows();
    expect(screen.getByRole('button', { name: 'Remove unavailable instrument in row 1' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Remove unavailable instrument in row 3' })).toBeInTheDocument();
  });

  it('m6: a cancelled drag (dragend) does not reorder on a later drop', async () => {
    seed(items(SBIN, TCS, RELIANCE));
    renderRows();
    fireEvent.dragStart(screen.getByRole('row', { name: /RELIANCE/ }));
    fireEvent.dragEnd(screen.getByRole('row', { name: /RELIANCE/ }));
    fireEvent.drop(screen.getByRole('row', { name: /SBIN/ }));
    await act(async () => { await Promise.resolve(); });
    expect(backend.callsTo('watchlists.reorder')).toEqual([]);
  });

  it('m7: a list that is not the active one renders read-only and logs in dev', () => {
    const err = vi.spyOn(console, 'error').mockImplementation(() => undefined);
    seed(items(SBIN, TCS));
    render(<WatchlistRows list={{ id: 'other', name: 'Other', items: items(SBIN, TCS) }} onChartItem={vi.fn()} />);
    expect(screen.queryByRole('button', { name: /^Remove/ })).toBeNull();
    expect(screen.getByRole('row', { name: /SBIN/ })).not.toHaveAttribute('draggable');
    act(() => screen.getByRole('row', { name: /SBIN/ }).focus());
    expect(h.shortcuts['watchlist.moveDown']).toBeUndefined();
    expect(screen.getByRole('button', { name: /^SBIN/ })).toBeInTheDocument();
    expect(err).toHaveBeenCalled();
    err.mockRestore();
  });

  it('m8: the polled explanation is the row description (reachable without hover)', () => {
    h.polledAbove = 10_000;
    seed(synthetic(2));
    renderRows();
    const row = screen.getByRole('row', { name: /SYM1/ });
    expect(row).toHaveAccessibleDescription(/Live subscription limit reached — updates every 5 s/);
    expect(screen.getByRole('row', { name: /SYM0\b/ })).not.toHaveAttribute('aria-describedby');
  });

  it('m9: a failed move shows an error toast', async () => {
    seed(items(SBIN, TCS));
    renderRows();
    backend.failNext('watchlists.reorder', 'disk full');
    const tcs = screen.getByRole('row', { name: /TCS/ });
    act(() => tcs.focus());
    await act(async () => { h.shortcuts['watchlist.moveUp'](); });
    await waitFor(() => expect(useToastStore.getState().visible[0]).toMatchObject({ level: 'error', message: 'Could not reorder the watchlist: disk full' }));
  });

  it('m9: a failed undo shows an error toast', async () => {
    seed(items(SBIN, TCS));
    renderRows();
    fireEvent.click(screen.getByRole('button', { name: 'Remove TCS' }));
    await waitFor(() => expect(useToastStore.getState().visible).toHaveLength(1));
    backend.failNext('watchlists.add', 'disk full');
    await act(async () => { useToastStore.getState().visible[0].action!.onSelect(); });
    await waitFor(() => expect(useToastStore.getState().visible[0]).toMatchObject({ level: 'error', message: 'Could not restore the instrument: disk full' }));
  });

  it('m9: a drop after the list changed uses the target row\'s current position', async () => {
    seed(items(SBIN, TCS, RELIANCE));
    renderRows(); // rendered with the old order on purpose: the drop must read the current one
    fireEvent.dragStart(screen.getByRole('row', { name: /RELIANCE/ }));
    const reordered: Watchlist = { id: 'a', name: 'Watchlist 1', items: items(TCS, SBIN, RELIANCE) };
    backend.state.watchlists = JSON.parse(JSON.stringify({ lists: [reordered], activeId: 'a' }));
    act(() => useWatchlists.getState().applyChanged({ lists: [reordered], activeId: 'a' }));
    fireEvent.drop(screen.getByRole('row', { name: /SBIN/ }));
    await waitFor(() => expect(backend.callsTo('watchlists.reorder')[0]?.args).toEqual(['a', [TCS.token, RELIANCE.token, SBIN.token]]));
  });
});
