import { beforeEach, describe, expect, it, vi } from 'vitest';

vi.mock('../api/instruments', () => ({ instrumentsByTokens: vi.fn(), instrumentByIdentifier: vi.fn() }));
import { instrumentByIdentifier, instrumentsByTokens } from '../api/instruments';
import { ensureInstruments, resolveIdentifier, useInstruments } from './instrumentsStore';
import { NIFTY50, NIFTY_OPTION, RELIANCE, SBIN } from '../instruments/fixtures';
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
