import { act, fireEvent, render, screen, within } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

vi.mock('../../api/instruments', () => ({ searchInstruments: vi.fn() }));

import { searchInstruments } from '../../api/instruments';
import type { Watchlist, WatchlistsState } from '../../api/backend';
import { setBackend } from '../../api/current';
import { demoSeed } from '../../api/mock/fixtures';
import { createMockBackend, type MockBackend } from '../../api/mock/index';
import { watchlistItemOf } from '../../api/mock/market';
import type { Instrument } from '../../instruments/types';
import { useInstruments } from '../../store/instrumentsStore';
import { resetOnLogout } from '../../store/reset';
import { useToastStore } from '../../store/toasts';
import { tokensOf, useWatchlists } from '../../store/watchlistsStore';
import { SEARCH_DEBOUNCE_MS, WatchlistSearch } from './WatchlistSearch';
import { NIFTY_OCT_FUT, NIFTY_OPTION, SBIN, TCS } from '../../instruments/fixtures';

let backend: MockBackend;
const seed = (insts: Instrument[]) => {
  const list: Watchlist = { id: 'a', name: 'Watchlist 1', items: insts.map(watchlistItemOf) };
  backend.state.watchlists = JSON.parse(JSON.stringify({ lists: [list], activeId: 'a' }));
  useWatchlists.setState({ lists: [list], activeId: 'a', selectedToken: null, status: 'ready', error: null });
};
const input = () => screen.getByRole('combobox', { name: 'Search instruments to add' });
/** Lets pending promises (mocked search, mock backend) settle without moving the clock. */
const flush = () => act(async () => { await vi.advanceTimersByTimeAsync(0); });
/** Types a query and lets the debounce fire and the search answer arrive. */
const type = async (value: string) => {
  fireEvent.change(input(), { target: { value } });
  await act(async () => { await vi.advanceTimersByTimeAsync(SEARCH_DEBOUNCE_MS); });
};
const deferred = <T,>() => {
  let resolve!: (v: T) => void;
  let reject!: (e: unknown) => void;
  const promise = new Promise<T>((res, rej) => { resolve = res; reject = rej; });
  return { promise, resolve, reject };
};
const toasts = () => useToastStore.getState().visible;

beforeEach(() => {
  vi.useFakeTimers();
  vi.clearAllMocks();
  backend = createMockBackend(demoSeed());
  setBackend(backend);
});

afterEach(() => {
  vi.useRealTimers();
  setBackend(null);
});

describe('WatchlistSearch', () => {
  it('AC1: "sbin" + Enter appends SBIN to the active list, keeps the dropdown open and focus in the box', async () => {
    seed([TCS]);
    vi.mocked(searchInstruments).mockResolvedValue([SBIN]);
    render(<WatchlistSearch />);
    input().focus();
    await type('sbin');
    expect(screen.getByRole('option', { name: /SBIN/ })).toBeInTheDocument();
    fireEvent.keyDown(input(), { key: 'Enter' });
    await flush();
    expect(backend.callsTo('watchlists.add')[0]?.args).toEqual(['a', SBIN.token, -1]);
    expect(tokensOf(useWatchlists.getState().lists[0])).toEqual([TCS.token, SBIN.token]);
    expect(screen.getByRole('listbox', { name: 'Search results' })).toBeInTheDocument();
    expect(within(screen.getByRole('option', { name: /SBIN/ })).getByLabelText('Already in Watchlist 1')).toHaveTextContent('✓');
    expect(input()).toHaveFocus();
    expect(toasts()).toHaveLength(0);
  });

  it('AC2: an instrument already in the list shows a check and adding says "Already in Watchlist 1"', async () => {
    seed([SBIN]);
    vi.mocked(searchInstruments).mockResolvedValue([SBIN]);
    render(<WatchlistSearch />);
    await type('sbin');
    const opt = screen.getByRole('option', { name: /SBIN/ });
    expect(within(opt).getByLabelText('Already in Watchlist 1')).toHaveTextContent('✓');
    fireEvent.click(opt);
    await flush();
    expect(toasts()[0]).toMatchObject({ level: 'info', message: 'Already in Watchlist 1' });
    expect(backend.callsTo('watchlists.add')).toHaveLength(0);
  });

  it("AC2: Go's already_in_list refusal also says \"Already in Watchlist 1\"", async () => {
    seed([]);
    vi.mocked(searchInstruments).mockResolvedValue([SBIN]);
    backend.failNext('watchlists.add', 'Already in Watchlist 1', 'already_in_list');
    render(<WatchlistSearch />);
    await type('sbin');
    fireEvent.keyDown(input(), { key: 'Enter' });
    await flush();
    expect(toasts()).toHaveLength(1);
    expect(toasts()[0]).toMatchObject({ level: 'info', message: 'Already in Watchlist 1' });
  });

  it('AC3: "nifty oct fut" finds the current-month future and a click adds it', async () => {
    seed([]);
    vi.mocked(searchInstruments).mockResolvedValue([NIFTY_OCT_FUT]);
    render(<WatchlistSearch />);
    await type('nifty oct fut');
    const opt = screen.getByRole('option', { name: /NIFTY OCT FUT/ });
    expect(within(opt).getByLabelText('Add to Watchlist 1')).toHaveTextContent('+');
    fireEvent.click(opt);
    await flush();
    expect(backend.callsTo('watchlists.add')[0]?.args).toEqual(['a', NIFTY_OCT_FUT.token, -1]);
  });

  it('searches the trimmed query over all five tags with Go\'s default limit of 20', async () => {
    seed([]);
    vi.mocked(searchInstruments).mockResolvedValue([SBIN]);
    render(<WatchlistSearch />);
    await type('  sbin ');
    expect(searchInstruments).toHaveBeenCalledWith('sbin', ['INDICES', 'NSE', 'BSE', 'NFO-FUT', 'BFO-FUT'], 20);
  });

  it('debounces typing: one search for the last value, none before the delay', async () => {
    seed([]);
    vi.mocked(searchInstruments).mockResolvedValue([SBIN]);
    render(<WatchlistSearch />);
    fireEvent.change(input(), { target: { value: 's' } });
    act(() => { vi.advanceTimersByTime(SEARCH_DEBOUNCE_MS - 1); });
    fireEvent.change(input(), { target: { value: 'sb' } });
    act(() => { vi.advanceTimersByTime(SEARCH_DEBOUNCE_MS - 1); });
    fireEvent.change(input(), { target: { value: 'sbi' } });
    expect(searchInstruments).not.toHaveBeenCalled();
    await act(async () => { await vi.advanceTimersByTimeAsync(SEARCH_DEBOUNCE_MS); });
    expect(searchInstruments).toHaveBeenCalledTimes(1);
    expect(vi.mocked(searchInstruments).mock.calls[0][0]).toBe('sbi');
  });

  it('a late answer for an older query never replaces the newer one', async () => {
    seed([]);
    const older = deferred<Instrument[]>();
    const newer = deferred<Instrument[]>();
    vi.mocked(searchInstruments).mockReturnValueOnce(older.promise).mockReturnValueOnce(newer.promise);
    render(<WatchlistSearch />);
    await type('sb');
    await type('tc');
    await act(async () => { newer.resolve([TCS]); });
    expect(screen.getByRole('option', { name: /TCS/ })).toBeInTheDocument();
    await act(async () => { older.resolve([SBIN]); });
    expect(screen.queryByRole('option', { name: /SBIN/ })).toBeNull();
    expect(screen.getByRole('option', { name: /TCS/ })).toBeInTheDocument();
  });

  it('clearing the box drops an answer still in flight', async () => {
    seed([]);
    const pending = deferred<Instrument[]>();
    vi.mocked(searchInstruments).mockReturnValueOnce(pending.promise);
    render(<WatchlistSearch />);
    await type('sb');
    await type('');
    await act(async () => { pending.resolve([SBIN]); });
    expect(screen.queryByRole('listbox')).toBeNull();
  });

  it('Escape closes the dropdown, a second Escape clears the box; ArrowDown/ArrowUp move the highlight; an empty box searches nothing', async () => {
    seed([]);
    vi.mocked(searchInstruments).mockResolvedValue([SBIN, TCS]);
    render(<WatchlistSearch />);
    await type('s');
    expect(screen.getByRole('option', { name: /SBIN/ })).toHaveAttribute('aria-selected', 'true');
    fireEvent.keyDown(input(), { key: 'ArrowDown' });
    expect(screen.getByRole('option', { name: /TCS/ })).toHaveAttribute('aria-selected', 'true');
    fireEvent.keyDown(input(), { key: 'ArrowDown' }); // stays on the last row
    expect(screen.getByRole('option', { name: /TCS/ })).toHaveAttribute('aria-selected', 'true');
    fireEvent.keyDown(input(), { key: 'ArrowUp' });
    expect(screen.getByRole('option', { name: /SBIN/ })).toHaveAttribute('aria-selected', 'true');
    fireEvent.keyDown(input(), { key: 'Escape' });
    expect(screen.queryByRole('listbox')).toBeNull();
    expect(input()).toHaveValue('s');
    fireEvent.keyDown(input(), { key: 'ArrowDown' }); // reopens the same results
    expect(screen.getByRole('listbox', { name: 'Search results' })).toBeInTheDocument();
    fireEvent.keyDown(input(), { key: 'Escape' });
    fireEvent.keyDown(input(), { key: 'Escape' });
    expect(input()).toHaveValue('');
    vi.mocked(searchInstruments).mockClear();
    fireEvent.change(input(), { target: { value: '   ' } });
    await act(async () => { await vi.advanceTimersByTimeAsync(SEARCH_DEBOUNCE_MS * 3); });
    expect(searchInstruments).not.toHaveBeenCalled();
    expect(screen.queryByRole('listbox')).toBeNull();
  });

  it('Enter with the dropdown closed adds nothing; IME composition keys are ignored', async () => {
    seed([]);
    vi.mocked(searchInstruments).mockResolvedValue([SBIN]);
    render(<WatchlistSearch />);
    await type('sbin');
    fireEvent.keyDown(input(), { key: 'Enter', isComposing: true });
    fireEvent.keyDown(input(), { key: 'Enter', keyCode: 229 });
    fireEvent.keyDown(input(), { key: 'Escape' });
    fireEvent.keyDown(input(), { key: 'Enter' });
    await flush();
    expect(backend.callsTo('watchlists.add')).toHaveLength(0);
  });

  it('a repeated Enter while the add is in flight sends one Add', async () => {
    seed([]);
    vi.mocked(searchInstruments).mockResolvedValue([SBIN]);
    render(<WatchlistSearch />);
    await type('sbin');
    fireEvent.keyDown(input(), { key: 'Enter' });
    fireEvent.keyDown(input(), { key: 'Enter' });
    await flush();
    expect(backend.callsTo('watchlists.add')).toHaveLength(1);
    expect(toasts()).toHaveLength(0);
  });

  it('combobox/listbox ARIA: expanded, controls, active descendant, and a polite result count', async () => {
    seed([]);
    vi.mocked(searchInstruments).mockResolvedValue([SBIN, TCS]);
    render(<WatchlistSearch />);
    expect(input()).toHaveAttribute('aria-expanded', 'false');
    expect(input()).toHaveAttribute('aria-autocomplete', 'list');
    await type('s');
    const listbox = screen.getByRole('listbox', { name: 'Search results' });
    expect(input()).toHaveAttribute('aria-expanded', 'true');
    expect(input()).toHaveAttribute('aria-controls', listbox.id);
    fireEvent.keyDown(input(), { key: 'ArrowDown' });
    expect(input()).toHaveAttribute('aria-activedescendant', screen.getByRole('option', { name: /TCS/ }).id);
    const status = screen.getByRole('status');
    expect(status).toHaveAttribute('aria-live', 'polite');
    expect(status).toHaveTextContent('2 results');
    vi.mocked(searchInstruments).mockResolvedValue([]);
    await type('zzz');
    expect(screen.queryByRole('listbox')).toBeNull();
    expect(input()).toHaveAttribute('aria-expanded', 'false');
    expect(input()).not.toHaveAttribute('aria-activedescendant');
    expect(screen.getByRole('status')).toHaveTextContent('No instruments match');
  });

  it('NFR-15: an out-of-scope row is never offered nor cached; an added instrument is cached for later lookups', async () => {
    seed([]);
    vi.mocked(searchInstruments).mockResolvedValue([NIFTY_OPTION, SBIN]);
    render(<WatchlistSearch />);
    await type('nifty');
    expect(screen.getAllByRole('option')).toHaveLength(1);
    expect(screen.queryByRole('option', { name: /NIFTY OCT 22500 CE/ })).toBeNull();
    expect(useInstruments.getState().byToken[NIFTY_OPTION.token]).toBeUndefined();
    fireEvent.keyDown(input(), { key: 'Enter' });
    await flush();
    expect(useInstruments.getState().byToken[SBIN.token]).toEqual(SBIN);
    expect(useInstruments.getState().byToken[NIFTY_OPTION.token]).toBeUndefined();
  });

  it('a refused Add shows an error toast', async () => {
    seed([]);
    vi.mocked(searchInstruments).mockResolvedValue([SBIN]);
    backend.failNext('watchlists.add', 'That instrument is not in the instrument list.', 'unknown_instrument');
    render(<WatchlistSearch />);
    await type('sbin');
    fireEvent.keyDown(input(), { key: 'Enter' });
    await flush();
    expect(toasts()[0]).toMatchObject({ level: 'error', message: 'Could not add SBIN: That instrument is not in the instrument list.' });
  });

  it('a failed search shows a polite message and no stale rows', async () => {
    seed([]);
    vi.mocked(searchInstruments).mockResolvedValueOnce([SBIN]).mockRejectedValueOnce(new Error('boom'));
    render(<WatchlistSearch />);
    await type('sb');
    await type('tc');
    expect(screen.queryByRole('listbox')).toBeNull();
    expect(screen.getByRole('status')).toHaveTextContent('Search failed. Try again.');
  });

  it('after a sign-out reset, a search or an Add that finishes resolves quietly', async () => {
    seed([]);
    const search = deferred<Instrument[]>();
    vi.mocked(searchInstruments).mockReturnValueOnce(search.promise);
    render(<WatchlistSearch />);
    await type('sb');
    act(() => resetOnLogout());
    await act(async () => { search.resolve([SBIN]); });
    expect(screen.queryByRole('listbox')).toBeNull();
    expect(input()).toHaveValue('');

    seed([]);
    vi.mocked(searchInstruments).mockResolvedValue([SBIN]);
    const add = deferred<WatchlistsState>();
    vi.spyOn(backend.watchlists, 'add').mockReturnValueOnce(add.promise);
    await type('sbin');
    fireEvent.keyDown(input(), { key: 'Enter' });
    act(() => resetOnLogout());
    await act(async () => { add.reject(JSON.stringify({ code: 'already_in_list', stage: 'watchlists', reason: 'Already in Watchlist 1' })); });
    expect(toasts()).toHaveLength(0);
  });

  it('shows no order actions (G-0)', async () => {
    seed([]);
    vi.mocked(searchInstruments).mockResolvedValue([SBIN]);
    render(<WatchlistSearch />);
    await type('sbin');
    expect(screen.queryByRole('button', { name: /buy|sell|order/i })).toBeNull();
    expect(screen.queryByText(/^(B|S)$/)).toBeNull();
  });
});
