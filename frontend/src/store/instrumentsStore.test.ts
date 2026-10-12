import { act, renderHook, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';

vi.mock('../api/instruments', () => ({ instrumentsByTokens: vi.fn(), instrumentByIdentifier: vi.fn() }));
import { instrumentByIdentifier, instrumentsByTokens } from '../api/instruments';
import { ensureInstruments, resolveIdentifier, useInstrument, useInstruments, useResolvedInstrument } from './instrumentsStore';
import { INFY, NIFTY50, NIFTY_OPTION, RELIANCE, SBIN, TCS } from '../instruments/fixtures';
import { BindingError } from '../api/call';
import type { Instrument } from '../instruments/types';
import type { SessionStatus } from '../api/backend';
import { resetOnLogout } from './reset';
import { useSessionStore } from './session';

beforeEach(() => {
  useInstruments.setState({ byToken: {}, byId: {} });
  vi.clearAllMocks();
});

describe('instrumentsStore', () => {
  it('dedupes concurrent loads of the same tokens', async () => {
    vi.mocked(instrumentsByTokens).mockResolvedValue([RELIANCE, SBIN]);
    await Promise.all([ensureInstruments([RELIANCE.token, SBIN.token]), ensureInstruments([SBIN.token])]);
    expect(instrumentsByTokens).toHaveBeenCalledTimes(1);
    expect(useInstruments.getState().byToken[SBIN.token]).toEqual(SBIN);
  });
  it('NFR-15: put() never stores an out-of-scope instrument', () => {
    useInstruments.getState().put([NIFTY_OPTION, RELIANCE]);
    expect(useInstruments.getState().byToken[NIFTY_OPTION.token]).toBeUndefined();
  });
  it('resolveIdentifier caches by EXCHANGE:SYMBOL', async () => {
    vi.mocked(instrumentByIdentifier).mockResolvedValue(NIFTY50);
    expect(await resolveIdentifier('NSE:NIFTY 50')).toEqual(NIFTY50);
    expect(await resolveIdentifier('NSE:NIFTY 50')).toEqual(NIFTY50);
    expect(instrumentByIdentifier).toHaveBeenCalledTimes(1);
  });
});

function deferred<T>() {
  let resolve!: (v: T) => void;
  let reject!: (e: unknown) => void;
  const promise = new Promise<T>((res, rej) => { resolve = res; reject = rej; });
  return { promise, resolve, reject };
}

const session = (state: string, userId: string): SessionStatus => ({ state, userId } as SessionStatus);

describe('instrumentsStore guards (Rulings 96/98/102)', () => {
  it('known tokens are not fetched again; a failed load can be retried', async () => {
    vi.mocked(instrumentsByTokens).mockRejectedValueOnce(new Error('boom')).mockResolvedValueOnce([SBIN]);
    await expect(ensureInstruments([SBIN.token])).rejects.toThrow('boom');
    await ensureInstruments([SBIN.token]);
    await ensureInstruments([SBIN.token]);
    expect(instrumentsByTokens).toHaveBeenCalledTimes(2);
    expect(useInstruments.getState().byToken[SBIN.token]).toEqual(SBIN);
  });

  it('a load that answers after a logout reset is dropped quietly, and the next load asks again', async () => {
    const d = deferred<Instrument[]>();
    vi.mocked(instrumentsByTokens).mockReturnValueOnce(d.promise).mockResolvedValueOnce([SBIN]);
    useInstruments.getState().put([RELIANCE]);
    const late = ensureInstruments([SBIN.token]);
    resetOnLogout();
    expect(useInstruments.getState().byToken).toEqual({});
    d.resolve([SBIN]);
    await expect(late).resolves.toBeUndefined();
    expect(useInstruments.getState().byToken[SBIN.token]).toBeUndefined();
    await ensureInstruments([SBIN.token]); // the reset cleared the in-flight entry
    expect(instrumentsByTokens).toHaveBeenCalledTimes(2);
    expect(useInstruments.getState().byToken[SBIN.token]).toEqual(SBIN);
  });

  it('a rejection after a reset resolves quietly (no rethrow)', async () => {
    const d = deferred<Instrument[]>();
    vi.mocked(instrumentsByTokens).mockReturnValueOnce(d.promise);
    const late = ensureInstruments([SBIN.token]);
    resetOnLogout();
    d.reject(new Error('logged out'));
    await expect(late).resolves.toBeUndefined();
  });

  it('a load that answers after a different user signs in is dropped; expiry of the same user cancels nothing', async () => {
    useSessionStore.getState().setSession(session('ready', 'AB1234'));
    const d1 = deferred<Instrument[]>();
    vi.mocked(instrumentsByTokens).mockReturnValueOnce(d1.promise);
    const sameUser = ensureInstruments([SBIN.token]);
    useSessionStore.getState().setSession(session('expired', 'AB1234'));
    d1.resolve([SBIN]);
    await sameUser;
    expect(useInstruments.getState().byToken[SBIN.token]).toEqual(SBIN);

    const d2 = deferred<Instrument[]>();
    vi.mocked(instrumentsByTokens).mockReturnValueOnce(d2.promise);
    const otherUser = ensureInstruments([RELIANCE.token]);
    useSessionStore.getState().setSession(session('ready', 'ZZ9999'));
    d2.resolve([RELIANCE]);
    await otherUser;
    expect(useInstruments.getState().byToken[RELIANCE.token]).toBeUndefined();
  });

  it('resolveIdentifier after a reset returns the answer but does not cache it', async () => {
    const d = deferred<Instrument>();
    vi.mocked(instrumentByIdentifier).mockReturnValueOnce(d.promise);
    const late = resolveIdentifier('NSE:NIFTY 50');
    resetOnLogout();
    d.resolve(NIFTY50);
    expect(await late).toEqual(NIFTY50);
    expect(useInstruments.getState().byId['NSE:NIFTY 50']).toBeUndefined();
  });

  it('resolveIdentifier dedupes concurrent lookups of the same identifier', async () => {
    vi.mocked(instrumentByIdentifier).mockResolvedValue(NIFTY50);
    await Promise.all([resolveIdentifier('NSE:NIFTY 50'), resolveIdentifier('NSE:NIFTY 50')]);
    expect(instrumentByIdentifier).toHaveBeenCalledTimes(1);
  });
});

describe('Ruling 107 I1: a dropped load does not leave the hooks stuck', () => {
  it('useInstrument: deferred load → reset → resolve → a second fetch fills the hook', async () => {
    const d = deferred<Instrument[]>();
    vi.mocked(instrumentsByTokens).mockReturnValueOnce(d.promise).mockResolvedValueOnce([SBIN]);
    const { result } = renderHook(() => useInstrument(SBIN.token));
    expect(instrumentsByTokens).toHaveBeenCalledTimes(1);
    act(() => resetOnLogout());
    await act(async () => { d.resolve([SBIN]); await d.promise; });
    await waitFor(() => expect(result.current).toEqual(SBIN));
    expect(instrumentsByTokens).toHaveBeenCalledTimes(2);
  });

  it('useInstrument: a load dropped by a user change (no reset) is retried', async () => {
    useSessionStore.getState().setSession(session('ready', 'AB1234'));
    const d = deferred<Instrument[]>();
    vi.mocked(instrumentsByTokens).mockReturnValueOnce(d.promise).mockResolvedValueOnce([SBIN]);
    const { result } = renderHook(() => useInstrument(SBIN.token));
    act(() => useSessionStore.getState().setSession(session('ready', 'ZZ9999')));
    await act(async () => { d.resolve([SBIN]); await d.promise; });
    await waitFor(() => expect(result.current).toEqual(SBIN));
    expect(instrumentsByTokens).toHaveBeenCalledTimes(2);
  });

  it('useResolvedInstrument resolves, reports Go\'s reason, and re-resolves after a reset (m4)', async () => {
    const d = deferred<Instrument>();
    vi.mocked(instrumentByIdentifier)
      .mockReturnValueOnce(d.promise)
      .mockResolvedValueOnce(NIFTY50)
      .mockRejectedValueOnce(new BindingError('not_found', 'No instrument NSE:NOPE in the instrument list.'));
    const { result, rerender } = renderHook(({ id }) => useResolvedInstrument(id), { initialProps: { id: 'NSE:NIFTY 50' } });
    expect(result.current).toEqual({});
    act(() => resetOnLogout());
    await act(async () => { d.reject(new Error('stale error from before the reset')); await d.promise.catch(() => undefined); });
    await waitFor(() => expect(result.current.instrument).toEqual(NIFTY50));
    expect(result.current.error).toBeUndefined();
    rerender({ id: 'NSE:NOPE' });
    await waitFor(() => expect(result.current.error).toBe('No instrument NSE:NOPE in the instrument list.'));
  });
});

describe('Ruling 107 I2: negative cache and quiet put', () => {
  it('put() with nothing in scope does not notify', () => {
    const seen = vi.fn();
    const unsub = useInstruments.subscribe(seen);
    useInstruments.getState().put([]);
    useInstruments.getState().put([NIFTY_OPTION]);
    unsub();
    expect(seen).not.toHaveBeenCalled();
  });

  it('a token Go omitted is not asked for again, and no notification is sent', async () => {
    vi.mocked(instrumentsByTokens).mockResolvedValue([]);
    const seen = vi.fn();
    const unsub = useInstruments.subscribe(seen);
    for (let n = 0; n < 5; n++) await ensureInstruments([424242]);
    unsub();
    expect(instrumentsByTokens).toHaveBeenCalledTimes(1);
    expect(seen).not.toHaveBeenCalled();
  });

  it('the unknown set is cleared on reset and on every transition to ready (Go re-imports per login session)', async () => {
    vi.mocked(instrumentsByTokens).mockResolvedValue([]);
    useSessionStore.getState().setSession(session('ready', 'AB1234'));
    await ensureInstruments([424242]);
    await ensureInstruments([424242]);
    expect(instrumentsByTokens).toHaveBeenCalledTimes(1);
    useSessionStore.getState().setSession(session('expired', 'AB1234'));
    await ensureInstruments([424242]);
    expect(instrumentsByTokens).toHaveBeenCalledTimes(1);
    useSessionStore.getState().setSession(session('ready', 'AB1234')); // expired(A) → ready(A): no reset, but a new import
    await ensureInstruments([424242]);
    expect(instrumentsByTokens).toHaveBeenCalledTimes(2);
    resetOnLogout();
    await ensureInstruments([424242]);
    expect(instrumentsByTokens).toHaveBeenCalledTimes(3);
  });
});

describe('Ruling 107 m6/m7', () => {
  it('a partly overlapping call asks only for the new tokens and waits for the shared ones', async () => {
    const d = deferred<Instrument[]>();
    vi.mocked(instrumentsByTokens).mockReturnValueOnce(d.promise).mockResolvedValueOnce([INFY]);
    const first = ensureInstruments([TCS.token, SBIN.token]);
    let secondDone = false;
    const second = ensureInstruments([SBIN.token, INFY.token]).then(() => { secondDone = true; });
    expect(vi.mocked(instrumentsByTokens).mock.calls).toEqual([[[TCS.token, SBIN.token]], [[INFY.token]]]);
    await Promise.resolve();
    await Promise.resolve();
    expect(secondDone).toBe(false); // still waiting for SBIN from the first call
    d.resolve([TCS, SBIN]);
    await Promise.all([first, second]);
    expect(Object.keys(useInstruments.getState().byToken).map(Number).sort()).toEqual([TCS.token, SBIN.token, INFY.token].sort());
  });

  it('identifiers are trimmed before the cache lookup and the call (Go: ByID(strings.TrimSpace(id)))', async () => {
    vi.mocked(instrumentByIdentifier).mockResolvedValue(NIFTY50);
    expect(await resolveIdentifier('  NSE:NIFTY 50 ')).toEqual(NIFTY50);
    expect(await resolveIdentifier('NSE:NIFTY 50')).toEqual(NIFTY50);
    expect(instrumentByIdentifier).toHaveBeenCalledTimes(1);
    expect(instrumentByIdentifier).toHaveBeenCalledWith('NSE:NIFTY 50');
  });
});
