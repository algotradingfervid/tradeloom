import { afterEach, beforeEach, describe, expect, it, vi, type Mock } from 'vitest';
import {
  BACKFILL_TIMEOUT_MESSAGE, ChartDataController, EMPTY_MESSAGE, NO_DATA_PLAN_MESSAGE, type ChartDataApi, type ChartLoadState,
} from './ChartDataController';
import { FakeAdapter } from './testing/FakeAdapter';
import type { Bar } from './types';
import type { BackfillProgress, CandleEvent, CandlesPage } from '../api/chart';
import { BindingError } from '../api/call';
import { readySession } from '../api/mock/fixtures';
import { NIFTY50, RELIANCE, SBIN } from '../instruments/fixtures';
import { resetOnLogout } from '../store/reset';
import { useSessionStore } from '../store/session';

const STEP = 300_000;
const T0 = Date.UTC(2026, 9, 1, 3, 45);
const bars = (start: number, n: number, close = 100): Bar[] =>
  Array.from({ length: n }, (_, i) => ({ time: start + i * STEP, open: 100, high: 101, low: 99, close, volume: 10 }));
const ok = (b: Bar[], extra: Partial<CandlesPage> = {}): CandlesPage =>
  ({ bars: b, complete: true, backfillJobId: '', needsDataPlan: false, offline: false, ...extra });
// Ruling 87: the generated BackfillProgressDTO carries etaMs (the plan text said etaNs).
const progress = (jobId: string, state: BackfillProgress['state']): BackfillProgress =>
  ({ jobId, token: RELIANCE.token, interval: '5minute', done: 1, failed: 0, total: 1, etaMs: 0, state });
function deferred<T>() {
  let resolve!: (v: T) => void; let reject!: (e: unknown) => void;
  const promise = new Promise<T>((res, rej) => { resolve = res; reject = rej; });
  return { promise, resolve, reject };
}
const flush = () => new Promise((r) => setTimeout(r, 0));

let adapter: FakeAdapter;
let states: ChartLoadState[];
let candleCbs: Set<(e: CandleEvent) => void>;
let progressCbs: Set<(p: BackfillProgress[]) => void>;
// Vitest 5's untyped vi.fn() is not assignable to ChartDataApi; each mock carries its method's signature.
let api: { [K in keyof ChartDataApi]: Mock<ChartDataApi[K]> };
let ctl: ChartDataController;
let liveBars: number;
const emit = (e: CandleEvent) => candleCbs.forEach((cb) => cb(e));
const emitProgress = (p: BackfillProgress[]) => progressCbs.forEach((cb) => cb(p));

beforeEach(() => {
  adapter = new FakeAdapter();
  states = [];
  candleCbs = new Set();
  progressCbs = new Set();
  let liveSeq = 0;
  api = {
    fetchHistory: vi.fn<ChartDataApi['fetchHistory']>(), refreshRange: vi.fn<ChartDataApi['refreshRange']>(),
    openLive: vi.fn<ChartDataApi['openLive']>(async () => `live-${++liveSeq}`),
    closeLive: vi.fn<ChartDataApi['closeLive']>(async () => undefined),
    onCandle: vi.fn<ChartDataApi['onCandle']>((cb) => { candleCbs.add(cb); return () => { candleCbs.delete(cb); }; }),
    onBackfillProgress: vi.fn<ChartDataApi['onBackfillProgress']>((cb) => { progressCbs.add(cb); return () => { progressCbs.delete(cb); }; }),
  };
  liveBars = 0;
  ctl = new ChartDataController(adapter, api, (s) => states.push(s), () => { liveBars++; });
});
afterEach(() => {
  ctl.dispose();
  vi.useRealTimers();
});

describe('ChartDataController', () => {
  it('FR-CHART-003 AC1: 1 Day asks for 750 trading days ending now, opens live, configures the instrument', async () => {
    api.fetchHistory.mockResolvedValue(ok(bars(T0, 740)));
    await ctl.load(RELIANCE, '1d');
    expect(api.fetchHistory).toHaveBeenCalledWith(RELIANCE.token, 'day', 0, 750);
    expect(api.openLive).toHaveBeenCalledWith(RELIANCE.token, 'day');
    expect(adapter.spec).toEqual({ intraday: false, hasVolume: true, pricePrecision: 2, minMove: 0.05 });
    expect(adapter.bars()).toHaveLength(740);
    expect(states.map((s) => s.status)).toEqual(['loading', 'ready']);
  });

  it('FR-CHART-011 AC3: index charts have no volume', async () => {
    api.fetchHistory.mockResolvedValue(ok(bars(T0, 3)));
    await ctl.load(NIFTY50, '1h');
    expect(adapter.spec?.hasVolume).toBe(false);
  });

  it('out-of-order responses: only the latest load reaches the chart and stale live subs are closed', async () => {
    const a = deferred<CandlesPage>(); const b = deferred<CandlesPage>();
    api.fetchHistory.mockReturnValueOnce(a.promise).mockReturnValueOnce(b.promise);
    const pA = ctl.load(RELIANCE, '5m');
    const pB = ctl.load(SBIN, '5m');
    b.resolve(ok(bars(T0, 3, 500)));
    await pB;
    a.resolve(ok(bars(T0, 7, 900)));
    await pA;
    await flush();
    expect(adapter.bars().map((x) => x.close)).toEqual([500, 500, 500]);
    expect(api.closeLive).toHaveBeenCalledWith('live-1');
    expect(api.closeLive).not.toHaveBeenCalledWith('live-2');
    expect(states.at(-1)!.status).toBe('ready');
  });

  it('empty history shows empty state and never lazy-loads', async () => {
    api.fetchHistory.mockResolvedValue(ok([], { complete: true }));
    await ctl.load(RELIANCE, '15m');
    expect(states.at(-1)).toMatchObject({ status: 'empty', message: EMPTY_MESSAGE });
    adapter.emitRange(0);
    adapter.emitRange(0);
    await flush();
    expect(api.fetchHistory).toHaveBeenCalledTimes(1);
    emit({ token: RELIANCE.token, interval: '15minute', closed: false, bar: bars(T0, 1)[0] });
    expect(adapter.bars()).toHaveLength(1);
    expect(states.at(-1)).toMatchObject({ status: 'ready', message: null });
  });

  it('backfill: waits for backfill.progress "done" for the returned job id, then asks again and draws', async () => {
    api.fetchHistory
      .mockResolvedValueOnce(ok([], { complete: false, backfillJobId: 'bf-1' }))
      .mockResolvedValueOnce(ok(bars(T0, 4)));
    const p = ctl.load(RELIANCE, '5m');
    await flush();
    expect(api.fetchHistory).toHaveBeenCalledTimes(1);
    expect(states.at(-1)!.status).toBe('loading');
    emitProgress([progress('bf-0', 'done'), progress('bf-1', 'running')]);
    await flush();
    expect(api.fetchHistory).toHaveBeenCalledTimes(1);
    emitProgress([progress('bf-1', 'done')]);
    await p;
    expect(api.fetchHistory).toHaveBeenCalledTimes(2);
    expect(api.fetchHistory).toHaveBeenLastCalledWith(RELIANCE.token, '5minute', 0, 15);
    expect(adapter.bars()).toHaveLength(4);
    expect(states.map((s) => s.status)).toEqual(['loading', 'ready']);
  });

  it('backfill: a job that finished before the answer arrived is not waited for', async () => {
    const first = deferred<CandlesPage>();
    api.fetchHistory.mockReturnValueOnce(first.promise).mockResolvedValueOnce(ok(bars(T0, 2)));
    const p = ctl.load(RELIANCE, '5m');
    emitProgress([progress('bf-2', 'failed')]);
    first.resolve(ok([], { complete: false, backfillJobId: 'bf-2' }));
    await p;
    expect(api.fetchHistory).toHaveBeenCalledTimes(2);
    expect(states.at(-1)!.status).toBe('ready');
  });

  it('a backfill that never reports is abandoned after 30 s with a retryable message', async () => {
    vi.useFakeTimers();
    api.fetchHistory.mockResolvedValue(ok([], { complete: false, backfillJobId: 'bf-9' }));
    const p = ctl.load(RELIANCE, '5m');
    await vi.advanceTimersByTimeAsync(29_000);
    expect(states.at(-1)!.status).toBe('loading');
    await vi.advanceTimersByTimeAsync(1_000);
    await p;
    expect(api.fetchHistory).toHaveBeenCalledTimes(2); // asked once more after the wait; same job id → stop
    expect(states.at(-1)).toMatchObject({ status: 'error', message: BACKFILL_TIMEOUT_MESSAGE });
    expect(vi.getTimerCount()).toBe(0);
  });

  it('FR-CHART-003 AC3: scrolling near the left edge loads one older window ending before the first bar', async () => {
    api.fetchHistory.mockResolvedValueOnce(ok(bars(T0, 100)));
    await ctl.load(RELIANCE, '5m');
    const older = deferred<CandlesPage>();
    api.fetchHistory.mockReturnValueOnce(older.promise);
    adapter.emitRange(3);
    adapter.emitRange(2); // ignored while loading
    expect(api.fetchHistory).toHaveBeenCalledTimes(2);
    expect(api.fetchHistory).toHaveBeenLastCalledWith(RELIANCE.token, '5minute', T0 - 1, 15);
    older.resolve(ok(bars(T0 - 50 * STEP, 50)));
    await flush();
    expect(adapter.bars()).toHaveLength(150);
    expect(adapter.calls).toContain('prependHistory');
    api.fetchHistory.mockResolvedValueOnce(ok([], { complete: true })); // reachedStart = complete && no candles
    adapter.emitRange(1);
    await flush();
    adapter.emitRange(0);
    await flush();
    expect(api.fetchHistory).toHaveBeenCalledTimes(3);
  });

  it('FR-CHART-003 AC2: live candles append/replace; other tokens and intervals ignored; events during load buffered', async () => {
    const h = deferred<CandlesPage>();
    api.fetchHistory.mockReturnValue(h.promise);
    const p = ctl.load(RELIANCE, '5m');
    emit({ token: RELIANCE.token, interval: '5minute', closed: false, bar: { ...bars(T0 + 3 * STEP, 1)[0], close: 333 } });
    h.resolve(ok(bars(T0, 3)));
    await p;
    expect(adapter.bars()).toHaveLength(4);
    expect(adapter.bars()[3].close).toBe(333);
    emit({ token: SBIN.token, interval: '5minute', closed: false, bar: bars(T0 + 9 * STEP, 1)[0] });
    emit({ token: RELIANCE.token, interval: '15minute', closed: false, bar: bars(T0 + 9 * STEP, 1)[0] });
    expect(adapter.bars()).toHaveLength(4);
    emit({ token: RELIANCE.token, interval: '5minute', closed: true, bar: { ...bars(T0 + 3 * STEP, 1)[0], close: 334 } });
    emit({ token: RELIANCE.token, interval: '5minute', closed: false, bar: bars(T0 + 4 * STEP, 1)[0] });
    expect(adapter.bars().map((b) => b.close).slice(-2)).toEqual([334, 100]);
    expect(liveBars).toBe(2); // the buffered bar was applied by load(), not as a live update
  });

  it('a partial live candle (Go: open/volume unknown) keeps the loaded open and volume of the same bar', async () => {
    api.fetchHistory.mockResolvedValue(ok(bars(T0, 3)));
    await ctl.load(RELIANCE, '5m');
    const t = T0 + 2 * STEP;
    emit({ token: RELIANCE.token, interval: '5minute', closed: false, partial: true, bar: { time: t, open: 105, high: 106, low: 98, close: 104, volume: 2 } });
    expect(adapter.bars()[2]).toEqual({ time: t, open: 100, high: 106, low: 98, close: 104, volume: 10 });
    // A partial candle for a new bucket has no loaded bar to keep: it is shown as Go built it.
    emit({ token: RELIANCE.token, interval: '5minute', closed: false, partial: true, bar: { time: t + STEP, open: 104, high: 104, low: 104, close: 104, volume: 0 } });
    expect(adapter.bars().at(-1)).toEqual({ time: t + STEP, open: 104, high: 104, low: 104, close: 104, volume: 0 });
  });

  it('FR-CHART-003 AC4: needsDataPlan → message, live released, no further calls', async () => {
    api.fetchHistory.mockResolvedValue(ok([], { complete: false, needsDataPlan: true }));
    await ctl.load(RELIANCE, '1h');
    await flush();
    expect(states.at(-1)).toMatchObject({ status: 'no_data_plan', message: NO_DATA_PLAN_MESSAGE });
    expect(api.closeLive).toHaveBeenCalledWith('live-1');
    expect(api.fetchHistory).toHaveBeenCalledTimes(1);
  });

  it('a refused OpenLive (Go: no_data_plan) does not break history loading', async () => {
    api.openLive.mockRejectedValueOnce(new BindingError('no_data_plan', 'no market-data permission'));
    api.fetchHistory.mockResolvedValue(ok(bars(T0, 2)));
    await ctl.load(RELIANCE, '1h');
    ctl.dispose();
    await flush();
    expect(states.at(-1)!.status).toBe('ready');
    expect(api.closeLive).not.toHaveBeenCalled();
  });

  it('other failures show an error; retry reloads', async () => {
    api.fetchHistory.mockRejectedValueOnce(new BindingError('network', 'Network unreachable'));
    await ctl.load(RELIANCE, '1h');
    expect(states.at(-1)).toMatchObject({ status: 'error', message: 'Network unreachable' });
    api.fetchHistory.mockResolvedValueOnce(ok(bars(T0, 2)));
    await ctl.retry();
    expect(states.at(-1)!.status).toBe('ready');
  });

  it('NFR-14: the offline flag of the result is surfaced', async () => {
    api.fetchHistory.mockResolvedValue(ok(bars(T0, 2), { offline: true }));
    await ctl.load(RELIANCE, '1h');
    expect(states.at(-1)).toMatchObject({ status: 'ready', offline: true });
  });

  it('FR-CHART-006 AC1-3: refresh replaces visible-range candles, keeps the view, ignores a second click', async () => {
    api.fetchHistory.mockResolvedValue(ok(bars(T0, 100)));
    await ctl.load(RELIANCE, '5m');
    adapter.view = { from: T0 + 50 * STEP, to: T0 + 99 * STEP };
    const r = deferred<CandlesPage>();
    api.refreshRange.mockReturnValue(r.promise);
    const p1 = ctl.refresh();
    const p2 = ctl.refresh();
    expect(api.refreshRange).toHaveBeenCalledTimes(1);
    expect(api.refreshRange).toHaveBeenCalledWith(RELIANCE.token, '5minute', T0 + 50 * STEP, expect.any(Number));
    expect(states.at(-1)!.refreshing).toBe(true);
    r.resolve(ok([{ ...bars(T0 + 60 * STEP, 1)[0], close: 4242 }]));
    await Promise.all([p1, p2]);
    expect(adapter.bars()[60].close).toBe(4242);
    expect(adapter.setCandlesOpts.at(-1)).toEqual({ keepView: true });
    expect(states.at(-1)!.refreshing).toBe(false);
  });

  it('a refresh answered after a symbol switch never reaches the new chart', async () => {
    api.fetchHistory.mockResolvedValueOnce(ok(bars(T0, 10)));
    await ctl.load(RELIANCE, '5m');
    const r = deferred<CandlesPage>();
    api.refreshRange.mockReturnValue(r.promise);
    const p = ctl.refresh();
    api.fetchHistory.mockResolvedValueOnce(ok(bars(T0, 3, 500)));
    await ctl.load(SBIN, '5m');
    r.resolve(ok([{ ...bars(T0, 1)[0], close: 4242 }]));
    await p;
    expect(adapter.bars().map((b) => b.close)).toEqual([500, 500, 500]);
    expect(states.at(-1)).toMatchObject({ status: 'ready', refreshing: false });
  });

  it('a load superseded while waiting for a backfill leaves no wait or timer behind', async () => {
    vi.useFakeTimers();
    api.fetchHistory.mockResolvedValueOnce(ok([], { complete: false, backfillJobId: 'bf-3' }));
    const pA = ctl.load(RELIANCE, '5m');
    await vi.advanceTimersByTimeAsync(0);
    expect(vi.getTimerCount()).toBe(1);
    api.fetchHistory.mockResolvedValueOnce(ok(bars(T0, 2)));
    await ctl.load(SBIN, '5m');
    await pA;
    expect(api.fetchHistory).toHaveBeenCalledTimes(2); // RELIANCE is not asked again
    expect(vi.getTimerCount()).toBe(0);
    expect(states.at(-1)!.status).toBe('ready');
  });

  it('a history answer that arrives after logout or a user switch is dropped; live is released', async () => {
    const h = deferred<CandlesPage>();
    api.fetchHistory.mockReturnValueOnce(h.promise);
    const p = ctl.load(RELIANCE, '5m');
    resetOnLogout(); // what the event bridge calls on sign-out and on a change of user (Ruling 98)
    h.resolve(ok(bars(T0, 5)));
    await p;
    await flush();
    expect(adapter.bars()).toHaveLength(0);
    expect(states.at(-1)).toMatchObject({ status: 'idle', message: null });
    expect(api.closeLive).toHaveBeenCalledWith('live-1');
    emit({ token: RELIANCE.token, interval: '5minute', closed: false, bar: bars(T0, 1)[0] });
    expect(adapter.bars()).toHaveLength(0);
    // The host reloads after the next login; retry() remembers the instrument and timeframe.
    api.fetchHistory.mockResolvedValueOnce(ok(bars(T0, 2)));
    await ctl.retry();
    expect(states.at(-1)!.status).toBe('ready');
  });

  it('a lazy-load answer that arrives after a user switch is dropped', async () => {
    useSessionStore.getState().setSession(readySession({ userId: 'AB1234' }));
    api.fetchHistory.mockResolvedValueOnce(ok(bars(T0, 20)));
    await ctl.load(RELIANCE, '5m');
    const older = deferred<CandlesPage>();
    api.fetchHistory.mockReturnValueOnce(older.promise);
    adapter.emitRange(1);
    useSessionStore.getState().setSession(readySession({ userId: 'ZX9999' })); // no reset seen: the user check still drops it
    older.resolve(ok(bars(T0 - 10 * STEP, 10)));
    await flush();
    expect(adapter.calls).not.toContain('prependHistory');
    expect(adapter.bars()).toHaveLength(0); // treated like the reset the bridge would have sent
    expect(states.at(-1)).toMatchObject({ status: 'idle', loadingOlder: false });
    expect(api.closeLive).toHaveBeenCalledWith('live-1');
  });

  it('session expiry does not cancel: an answer arriving after expired(same user) still applies', async () => {
    useSessionStore.getState().setSession(readySession());
    const h = deferred<CandlesPage>();
    api.fetchHistory.mockReturnValueOnce(h.promise);
    const p = ctl.load(RELIANCE, '5m');
    useSessionStore.getState().setSession(readySession({ state: 'expired' }));
    h.resolve(ok(bars(T0, 5)));
    await p;
    expect(adapter.bars()).toHaveLength(5);
    expect(states.at(-1)!.status).toBe('ready');
  });

  it('dispose closes live and stops listening to candles and backfill progress', async () => {
    api.fetchHistory.mockResolvedValue(ok(bars(T0, 2)));
    await ctl.load(RELIANCE, '5m');
    ctl.dispose();
    await flush();
    expect(api.closeLive).toHaveBeenCalledWith('live-1');
    expect(candleCbs.size).toBe(0);
    expect(progressCbs.size).toBe(0);
  });

  it('dispose during a backfill wait clears its timer and ends the load quietly', async () => {
    vi.useFakeTimers();
    api.fetchHistory.mockResolvedValue(ok([], { complete: false, backfillJobId: 'bf-5' }));
    const p = ctl.load(RELIANCE, '5m');
    await vi.advanceTimersByTimeAsync(0);
    expect(vi.getTimerCount()).toBe(1);
    ctl.dispose();
    await p;
    expect(vi.getTimerCount()).toBe(0);
    expect(api.fetchHistory).toHaveBeenCalledTimes(1);
    expect(states.at(-1)!.status).toBe('loading');
  });
});
