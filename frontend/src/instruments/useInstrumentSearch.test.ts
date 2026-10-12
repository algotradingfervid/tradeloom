import { act, renderHook } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

vi.mock('../api/instruments', () => ({ searchInstruments: vi.fn(), instrumentByIdentifier: vi.fn(), instrumentsByTokens: vi.fn(async () => []) }));
import { searchInstruments } from '../api/instruments';
import { BindingError } from '../api/call';
import { resetOnLogout } from '../store/reset';
import { NIFTY_OPTION, RELIANCE, SBIN } from './fixtures';
import type { Instrument } from './types';
import { instrumentSearchCopy, SEARCH_DEBOUNCE_MS, useInstrumentSearch } from './useInstrumentSearch';

const flush = (ms = SEARCH_DEBOUNCE_MS) => act(async () => { await vi.advanceTimersByTimeAsync(ms); });

function deferred<T>() {
  let resolve!: (v: T) => void;
  const promise = new Promise<T>((res) => { resolve = res; });
  return { promise, resolve };
}

const mount = (query: string, opts: Parameters<typeof useInstrumentSearch>[1] = { limit: 50 }) =>
  renderHook(({ q }) => useInstrumentSearch(q, opts), { initialProps: { q: query } });

beforeEach(() => {
  vi.useFakeTimers();
  vi.clearAllMocks();
  vi.mocked(searchInstruments).mockImplementation(async (q: string) => (q === 'r' ? [RELIANCE, SBIN] : []));
});
afterEach(() => { vi.useRealTimers(); });

describe('useInstrumentSearch', () => {
  it('exports the shared debounce and copy', () => {
    expect(SEARCH_DEBOUNCE_MS).toBe(120);
    expect(instrumentSearchCopy.noMatch).toBe('No instruments match');
    expect(instrumentSearchCopy.count(1)).toBe('1 result');
    expect(instrumentSearchCopy.count(2)).toBe('2 results');
  });

  it('never sends an empty query; debounces to one call for the last query with the given limit', async () => {
    const { result, rerender } = mount('  ', { limit: 7 });
    await flush(500);
    expect(searchInstruments).not.toHaveBeenCalled();
    expect(result.current).toMatchObject({ q: '', rows: [], fresh: false, error: null, status: '' });
    rerender({ q: 'x' });
    await flush(SEARCH_DEBOUNCE_MS - 1);
    rerender({ q: ' r ' });
    await flush(SEARCH_DEBOUNCE_MS - 1);
    expect(searchInstruments).not.toHaveBeenCalled();
    await flush(1);
    expect(searchInstruments).toHaveBeenCalledTimes(1);
    expect(searchInstruments).toHaveBeenCalledWith('r', ['INDICES', 'NSE', 'BSE', 'NFO-FUT', 'BFO-FUT'], 7);
    expect(result.current).toMatchObject({ q: 'r', rows: [RELIANCE, SBIN], fresh: true, error: null, status: '2 results' });
  });

  it('drops out-of-scope rows (NFR-15) and reports no match', async () => {
    vi.mocked(searchInstruments).mockResolvedValue([NIFTY_OPTION]);
    const { result } = mount('nifty');
    await flush();
    expect(result.current).toMatchObject({ rows: [], fresh: true, status: 'No instruments match' });
  });

  it('a stale answer is dropped: the older query never replaces the newer one', async () => {
    const slow = deferred<Instrument[]>();
    vi.mocked(searchInstruments).mockImplementation((q: string) => (q === 'r' ? slow.promise : Promise.resolve([SBIN])));
    const { result, rerender } = mount('r');
    await flush();
    rerender({ q: 'sb' });
    await flush();
    expect(result.current.rows).toEqual([SBIN]);
    await act(async () => { slow.resolve([RELIANCE]); });
    expect(result.current).toMatchObject({ q: 'sb', rows: [SBIN], fresh: true });
  });

  it('rows of the previous query stay shown but are not fresh while the next one is pending', async () => {
    const { result, rerender } = mount('r');
    await flush();
    rerender({ q: 'rel' });
    expect(result.current).toMatchObject({ q: 'rel', rows: [RELIANCE, SBIN], fresh: false, status: '' });
  });

  it('a binding failure is tied to its query and hides the previous rows', async () => {
    const { result, rerender } = mount('r');
    await flush();
    vi.mocked(searchInstruments).mockRejectedValueOnce(new BindingError('internal', 'bridge down'));
    rerender({ q: 'x' });
    await flush();
    expect(result.current).toMatchObject({ q: 'x', rows: [], fresh: false, error: 'bridge down' });
    rerender({ q: 'r' });
    expect(result.current.error).toBeNull();
  });

  it('a reset (logout or user switch) clears the rows, drops the in-flight answer and calls onReset', async () => {
    const onReset = vi.fn();
    const { result, rerender } = mount('r', { limit: 50, onReset });
    await flush();
    expect(result.current.isCurrent()).toBe(true);
    const late = deferred<Instrument[]>();
    vi.mocked(searchInstruments).mockReturnValue(late.promise);
    rerender({ q: 'sb' });
    await flush();
    const before = result.current;
    act(() => resetOnLogout());
    expect(onReset).toHaveBeenCalledTimes(1);
    expect(before.isCurrent()).toBe(false);
    expect(result.current).toMatchObject({ rows: [], fresh: false, error: null, status: '' });
    await act(async () => { late.resolve([SBIN]); });
    await flush(0);
    expect(result.current.rows).toEqual([]);
  });

  it('unmount during the debounce leaves no timer and sends nothing', async () => {
    const { unmount } = mount('r');
    await flush(SEARCH_DEBOUNCE_MS / 2);
    unmount();
    expect(vi.getTimerCount()).toBe(0);
    await flush(1_000);
    expect(searchInstruments).not.toHaveBeenCalled();
  });

  it('unmount after the reset subscription leaves no listener: a later reset calls nothing', async () => {
    const onReset = vi.fn();
    const { unmount } = mount('r', { limit: 50, onReset });
    unmount();
    resetOnLogout();
    expect(onReset).not.toHaveBeenCalled();
  });
});
