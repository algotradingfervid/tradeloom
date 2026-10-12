> **P0-5 Chart, Watchlists & Ticker UI Implementation Plan — Task 5.** Read [`../MANIFEST.md`](../MANIFEST.md) first: its Global Constraints,
> Review Focus and Consumes/Produces apply to this task. Other tasks live beside this file in `tasks/`.

<!-- task body (verbatim from the plan) -->
### Task 5: `ChartDataController` — history, backfill wait, lazy loading, live candles, refresh, race guard

**Files:**
- Create: `frontend/src/chart/ChartDataController.ts`, `frontend/src/chart/testing/FakeAdapter.ts`
- Test: `frontend/src/chart/ChartDataController.test.ts`

**Interfaces:**
- Consumes: `ChartAdapter` (Tasks 3–4), `timeframeById`, `precisionForTick`, `mergeBars`, `tickRupees` (Task 1), `BindingError`, `CandleEvent`, `CandlesPage`, `BackfillProgress` (Task 2), `Instrument`.
- Produces:
  - `type ChartStatus = 'idle'|'loading'|'ready'|'empty'|'error'|'no_data_plan'`
  - `interface ChartLoadState { status; offline: boolean; refreshing: boolean; loadingOlder: boolean; message: string | null }`, `INITIAL_CHART_STATE`
  - `interface ChartDataApi { fetchHistory; refreshRange; openLive; closeLive; onCandle; onBackfillProgress }` (same signatures as `api/chart.ts`)
  - `class ChartDataController { constructor(adapter, api, onState, onLiveBar?: () => void); load(inst, tf): Promise<void>; loadOlder(): Promise<void>; refresh(): Promise<void>; retry(): Promise<void>; dispose(): void; readonly current: ChartLoadState }`
  - `LAZY_LOAD_THRESHOLD = 10`, `BACKFILL_WAIT_MS = 30_000`, `NO_DATA_PLAN_MESSAGE = 'Charts need a Kite Connect data plan'`, `EMPTY_MESSAGE = 'No candles yet for this instrument'`, `BACKFILL_TIMEOUT_MESSAGE = 'Candles are still downloading. Retry in a moment.'`
  - `class FakeAdapter implements ChartAdapter` with helpers `emitRange(barsBefore)`, `emitCrosshair(d)`, `calls: string[]`, `setCandlesOpts`, `spec`, `theme`, `scale`, `view`

The backfill wait (contract §C): `Chart.Candles` may answer with a `backfillJobId` instead of (or alongside) the candles. The controller then waits for `backfill.progress` with that `jobId` to reach `done` or `failed`, at most 30 s in total per window, and asks `Candles` again. It listens to `backfill.progress` for its whole life and remembers finished jobs, so a job that finishes before the answer arrives is not waited for. A job id it already waited for is never waited for twice.

- [ ] **Step 1: Write the failing test**

`frontend/src/chart/ChartDataController.test.ts`:

```ts
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import {
  BACKFILL_TIMEOUT_MESSAGE, ChartDataController, EMPTY_MESSAGE, NO_DATA_PLAN_MESSAGE, type ChartLoadState,
} from './ChartDataController';
import { FakeAdapter } from './testing/FakeAdapter';
import type { Bar } from './types';
import type { BackfillProgress, CandleEvent, CandlesPage } from '../api/chart';
import { BindingError } from '../api/call';
import { NIFTY50, RELIANCE, SBIN } from '../instruments/fixtures';

const STEP = 300_000;
const T0 = Date.UTC(2026, 9, 1, 3, 45);
const bars = (start: number, n: number, close = 100): Bar[] =>
  Array.from({ length: n }, (_, i) => ({ time: start + i * STEP, open: 100, high: 101, low: 99, close, volume: 10 }));
const ok = (b: Bar[], extra: Partial<CandlesPage> = {}): CandlesPage =>
  ({ bars: b, complete: true, backfillJobId: '', needsDataPlan: false, offline: false, ...extra });
const progress = (jobId: string, state: BackfillProgress['state']): BackfillProgress =>
  ({ jobId, token: RELIANCE.token, interval: '5minute', done: 1, failed: 0, total: 1, etaNs: 0, state });
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
let api: {
  fetchHistory: ReturnType<typeof vi.fn>; refreshRange: ReturnType<typeof vi.fn>;
  openLive: ReturnType<typeof vi.fn>; closeLive: ReturnType<typeof vi.fn>;
  onCandle: ReturnType<typeof vi.fn>; onBackfillProgress: ReturnType<typeof vi.fn>;
};
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
    fetchHistory: vi.fn(), refreshRange: vi.fn(),
    openLive: vi.fn(async () => `live-${++liveSeq}`), closeLive: vi.fn(async () => undefined),
    onCandle: vi.fn((cb: (e: CandleEvent) => void) => { candleCbs.add(cb); return () => candleCbs.delete(cb); }),
    onBackfillProgress: vi.fn((cb: (p: BackfillProgress[]) => void) => { progressCbs.add(cb); return () => progressCbs.delete(cb); }),
  };
  liveBars = 0;
  ctl = new ChartDataController(adapter, api, (s) => states.push(s), () => { liveBars++; });
});
afterEach(() => vi.useRealTimers());

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

  it('FR-CHART-003 AC4: needsDataPlan → message, live released, no further calls', async () => {
    api.fetchHistory.mockResolvedValue(ok([], { complete: false, needsDataPlan: true }));
    await ctl.load(RELIANCE, '1h');
    await flush();
    expect(states.at(-1)).toMatchObject({ status: 'no_data_plan', message: NO_DATA_PLAN_MESSAGE });
    expect(api.closeLive).toHaveBeenCalledWith('live-1');
    expect(api.fetchHistory).toHaveBeenCalledTimes(1);
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

  it('dispose closes live and stops listening to candles and backfill progress', async () => {
    api.fetchHistory.mockResolvedValue(ok(bars(T0, 2)));
    await ctl.load(RELIANCE, '5m');
    ctl.dispose();
    await flush();
    expect(api.closeLive).toHaveBeenCalledWith('live-1');
    expect(candleCbs.size).toBe(0);
    expect(progressCbs.size).toBe(0);
  });
});
```

- [ ] **Step 2: Run the test to check it fails**

Run: `cd frontend && npx vitest run src/chart/ChartDataController.test.ts`
Expected: FAIL with `Failed to resolve import "./ChartDataController"`.

- [ ] **Step 3: Implement**

`frontend/src/chart/testing/FakeAdapter.ts`:

```ts
import type { ChartAdapter } from '../ChartAdapter';
import { dedupeSorted } from '../bars';
import type {
  Bar, ChartTheme, Handle, InstrumentSpec, LegendData, PriceBandSpec, RangeInfo, StudySpec, TimeRange, UpdateOutcome,
} from '../types';

/** In-memory ChartAdapter for controller and view tests. */
export class FakeAdapter implements ChartAdapter {
  data: Bar[] = [];
  spec: InstrumentSpec | null = null;
  theme: ChartTheme | null = null;
  scale = { log: false, auto: true };
  view: TimeRange | null = null;
  studies = new Map<string, StudySpec>();
  calls: string[] = [];
  setCandlesOpts: ({ keepView?: boolean } | undefined)[] = [];
  destroyed = false;
  private rangeCbs = new Set<(r: RangeInfo) => void>();
  private crossCbs = new Set<(d: LegendData | null) => void>();

  setInstrument(spec: InstrumentSpec): void { this.spec = spec; this.calls.push('setInstrument'); }
  setCandles(bars: readonly Bar[], opts?: { keepView?: boolean }): void { this.data = dedupeSorted(bars); this.setCandlesOpts.push(opts); this.calls.push('setCandles'); }
  prependHistory(bars: readonly Bar[]): number {
    const first = this.data[0]?.time ?? Number.POSITIVE_INFINITY;
    const older = dedupeSorted(bars).filter((b) => b.time < first);
    this.data = [...older, ...this.data];
    this.calls.push('prependHistory');
    return older.length;
  }
  updateLastBar(bar: Bar): UpdateOutcome {
    this.calls.push('updateLastBar');
    const last = this.data[this.data.length - 1];
    if (!last || bar.time > last.time) { this.data.push(bar); return 'appended'; }
    if (bar.time === last.time) { this.data[this.data.length - 1] = bar; return 'replaced'; }
    const i = this.data.findIndex((b) => b.time === bar.time);
    if (i < 0) return 'ignored';
    this.data[i] = bar;
    return 'historical';
  }
  bars(): readonly Bar[] { return this.data; }
  addStudy(spec: StudySpec): Handle { this.studies.set(spec.id, spec); return { id: spec.id }; }
  removeStudy(h: Handle): void { this.studies.delete(h.id); }
  addPriceBand(spec: PriceBandSpec): Handle { return { id: spec.id }; }
  removePriceBand(): void {}
  setTheme(t: ChartTheme): void { this.theme = t; this.calls.push('setTheme'); }
  setPriceScale(o: { log?: boolean; auto?: boolean }): void { this.scale = { log: o.log ?? this.scale.log, auto: o.auto ?? this.scale.auto }; }
  priceScaleState(): { log: boolean; auto: boolean } { return { ...this.scale }; }
  panBars(delta: number): void { this.calls.push(`pan:${delta}`); }
  zoom(factor: number): void { this.calls.push(`zoom:${factor}`); }
  scrollToLatest(): void { this.calls.push('scrollToLatest'); }
  onVisibleRangeChange(cb: (r: RangeInfo) => void): () => void { this.rangeCbs.add(cb); return () => { this.rangeCbs.delete(cb); }; }
  onCrosshairMove(cb: (d: LegendData | null) => void): () => void { this.crossCbs.add(cb); return () => { this.crossCbs.delete(cb); }; }
  visibleTimeRange(): TimeRange | null { return this.view; }
  destroy(): void { this.destroyed = true; }
  emitRange(barsBefore: number): void { this.rangeCbs.forEach((cb) => cb({ barsBefore })); }
  emitCrosshair(d: LegendData | null): void { this.crossCbs.forEach((cb) => cb(d)); }
}
```

`frontend/src/chart/ChartDataController.ts`:

```ts
import type { ChartAdapter } from './ChartAdapter';
import type { Bar } from './types';
import type { Instrument } from '../instruments/types';
import { tickRupees } from '../instruments/naming';
import { timeframeById, type KiteInterval, type Timeframe, type TimeframeId } from './timeframes';
import { precisionForTick } from './format';
import { mergeBars } from './bars';
import type { BackfillProgress, CandleEvent, CandlesPage } from '../api/chart';
import { BindingError } from '../api/call';

export const LAZY_LOAD_THRESHOLD = 10;
export const BACKFILL_WAIT_MS = 30_000;
export const NO_DATA_PLAN_MESSAGE = 'Charts need a Kite Connect data plan';
export const EMPTY_MESSAGE = 'No candles yet for this instrument';
export const BACKFILL_TIMEOUT_MESSAGE = 'Candles are still downloading. Retry in a moment.';
const OLDER_RETRY_MS = 5000;

export type ChartStatus = 'idle' | 'loading' | 'ready' | 'empty' | 'error' | 'no_data_plan';
export interface ChartLoadState { status: ChartStatus; offline: boolean; refreshing: boolean; loadingOlder: boolean; message: string | null }
export const INITIAL_CHART_STATE: ChartLoadState = { status: 'idle', offline: false, refreshing: false, loadingOlder: false, message: null };

export interface ChartDataApi {
  fetchHistory(token: number, interval: KiteInterval, toMs: number, tradingDays: number): Promise<CandlesPage>;
  refreshRange(token: number, interval: KiteInterval, fromMs: number, toMs: number): Promise<CandlesPage>;
  openLive(token: number, interval: KiteInterval): Promise<string>;
  closeLive(id: string): Promise<void>;
  onCandle(cb: (e: CandleEvent) => void): () => void;
  onBackfillProgress(cb: (progress: BackfillProgress[]) => void): () => void;
}

/**
 * Owns one chart's data lifecycle. Every async continuation checks `seq` so a slow response for an
 * abandoned symbol/timeframe can never reach the chart (Review Focus 2).
 */
export class ChartDataController {
  private seq = 0;
  private inst: Instrument | null = null;
  private tf: Timeframe | null = null;
  private live: Promise<string> | null = null;
  private reachedStart = false;
  private olderBlockedUntil = 0;
  private pending: Bar[] = [];
  private state: ChartLoadState = INITIAL_CHART_STATE;
  /** Backfill jobs that already reported done/failed, and the waits currently pending per job id. */
  private readonly finishedJobs = new Set<string>();
  private readonly jobWaiters = new Map<string, () => void>();
  private readonly offCandle: () => void;
  private readonly offRange: () => void;
  private readonly offBackfill: () => void;

  constructor(
    private readonly adapter: ChartAdapter,
    private readonly api: ChartDataApi,
    private readonly onState: (s: ChartLoadState) => void,
    private readonly onLiveBar?: () => void,
  ) {
    this.offCandle = api.onCandle((e) => this.onCandleEvent(e));
    this.offBackfill = api.onBackfillProgress((list) => {
      for (const p of list) {
        if (p.state === 'running') continue;
        this.finishedJobs.add(p.jobId);
        this.jobWaiters.get(p.jobId)?.();
      }
    });
    this.offRange = adapter.onVisibleRangeChange((r) => {
      if (r.barsBefore < LAZY_LOAD_THRESHOLD) void this.loadOlder();
    });
  }

  get current(): ChartLoadState { return this.state; }

  async load(inst: Instrument, tfId: TimeframeId): Promise<void> {
    const seq = ++this.seq;
    this.releaseLive();
    const tf = timeframeById(tfId);
    this.inst = inst;
    this.tf = tf;
    this.reachedStart = false;
    this.olderBlockedUntil = 0;
    this.pending = [];
    this.finishedJobs.clear();
    const tick = tickRupees(inst);
    this.adapter.setInstrument({ intraday: tf.intraday, hasVolume: !inst.isIndex, pricePrecision: precisionForTick(tick), minMove: tick });
    this.adapter.setCandles([]);
    this.set({ ...INITIAL_CHART_STATE, status: 'loading' });
    // Subscribe before fetching so a candle that closes during the fetch is buffered, not lost.
    this.live = this.api.openLive(inst.token, tf.kite);
    this.live.catch(() => undefined);
    try {
      const page = await this.fetchWindow(seq, inst, tf, 0);
      if (!page) return;
      if (page.needsDataPlan) {
        this.releaseLive();
        this.set({ ...INITIAL_CHART_STATE, status: 'no_data_plan', message: NO_DATA_PLAN_MESSAGE });
        return;
      }
      this.reachedStart = page.complete && page.bars.length === 0;
      this.adapter.setCandles(page.bars);
      for (const b of this.pending) this.adapter.updateLastBar(b);
      this.pending = [];
      const empty = this.adapter.bars().length === 0;
      if (empty && page.backfillJobId) {
        this.set({ ...INITIAL_CHART_STATE, status: 'error', message: BACKFILL_TIMEOUT_MESSAGE });
        return;
      }
      this.set({ ...INITIAL_CHART_STATE, status: empty ? 'empty' : 'ready', offline: page.offline, message: empty ? EMPTY_MESSAGE : null });
    } catch (e) {
      if (seq !== this.seq) return;
      this.fail(e);
    }
  }

  retry(): Promise<void> {
    return this.inst && this.tf ? this.load(this.inst, this.tf.id) : Promise.resolve();
  }

  async loadOlder(): Promise<void> {
    const { inst, tf } = this;
    const first = this.adapter.bars()[0];
    if (!inst || !tf || !first || this.reachedStart || this.state.loadingOlder || this.state.status !== 'ready') return;
    if (Date.now() < this.olderBlockedUntil) return;
    const seq = this.seq;
    this.set({ ...this.state, loadingOlder: true });
    try {
      const page = await this.fetchWindow(seq, inst, tf, first.time - 1);
      if (!page) return;
      const added = this.adapter.prependHistory(page.bars);
      if (page.complete && page.bars.length === 0) this.reachedStart = true; // contract: reachedStart = complete && no candles
      else if (page.bars.length > 0 && added === 0) this.reachedStart = true; // nothing older than what is loaded
      else if (added === 0) this.olderBlockedUntil = Date.now() + OLDER_RETRY_MS; // backfill still running: back off
    } catch {
      if (seq !== this.seq) return;
      this.olderBlockedUntil = Date.now() + OLDER_RETRY_MS;
    } finally {
      if (seq === this.seq) this.set({ ...this.state, loadingOlder: false });
    }
  }

  async refresh(): Promise<void> {
    const { inst, tf } = this;
    const first = this.adapter.bars()[0];
    if (!inst || !tf || !first || this.state.refreshing || this.state.status !== 'ready') return;
    const seq = this.seq;
    const from = this.adapter.visibleTimeRange()?.from ?? first.time;
    this.set({ ...this.state, refreshing: true, message: null });
    try {
      const r = await this.api.refreshRange(inst.token, tf.kite, from, Date.now());
      if (seq !== this.seq) return;
      this.adapter.setCandles(mergeBars(this.adapter.bars(), r.bars), { keepView: true });
      this.set({ ...this.state, refreshing: false, offline: r.offline });
    } catch (e) {
      if (seq !== this.seq) return;
      this.set({ ...this.state, refreshing: false, message: e instanceof Error ? e.message : String(e) });
    }
  }

  dispose(): void {
    this.seq++;
    this.offCandle();
    this.offRange();
    this.offBackfill();
    for (const finish of [...this.jobWaiters.values()]) finish();
    this.releaseLive();
  }

  /**
   * One history window. If Go submitted a backfill job, wait for its backfill.progress (≤ BACKFILL_WAIT_MS in total)
   * and ask again; a job id is waited for once. Returns null when this load was superseded.
   */
  private async fetchWindow(seq: number, inst: Instrument, tf: Timeframe, toMs: number): Promise<CandlesPage | null> {
    let page = await this.api.fetchHistory(inst.token, tf.kite, toMs, tf.initialTradingDays);
    const deadline = Date.now() + BACKFILL_WAIT_MS;
    const waited = new Set<string>();
    while (seq === this.seq && page.backfillJobId && !waited.has(page.backfillJobId) && Date.now() < deadline) {
      waited.add(page.backfillJobId);
      await this.waitForJob(page.backfillJobId, deadline - Date.now());
      if (seq !== this.seq) return null;
      page = await this.api.fetchHistory(inst.token, tf.kite, toMs, tf.initialTradingDays);
    }
    return seq === this.seq ? page : null;
  }

  private waitForJob(jobId: string, ms: number): Promise<void> {
    if (this.finishedJobs.has(jobId) || ms <= 0) return Promise.resolve();
    return new Promise((resolve) => {
      const finish = () => {
        clearTimeout(timer);
        this.jobWaiters.delete(jobId);
        resolve();
      };
      const timer = setTimeout(finish, ms);
      this.jobWaiters.set(jobId, finish);
    });
  }

  private onCandleEvent(e: CandleEvent): void {
    if (!this.inst || !this.tf || e.token !== this.inst.token || e.interval !== this.tf.kite) return;
    if (this.state.status === 'loading') { this.pending.push(e.bar); return; }
    if (this.state.status !== 'ready' && this.state.status !== 'empty') return;
    const outcome = this.adapter.updateLastBar(e.bar);
    if (this.state.status === 'empty' && outcome === 'appended') this.set({ ...this.state, status: 'ready', message: null });
    if (outcome !== 'ignored') this.onLiveBar?.();
  }

  private releaseLive(): void {
    const live = this.live;
    this.live = null;
    if (live) void live.then((id) => this.api.closeLive(id)).catch(() => undefined);
  }

  private fail(e: unknown): void {
    this.releaseLive();
    if (e instanceof BindingError && e.code === 'no_data_plan') {
      this.set({ ...INITIAL_CHART_STATE, status: 'no_data_plan', message: NO_DATA_PLAN_MESSAGE });
      return;
    }
    this.set({ ...INITIAL_CHART_STATE, status: 'error', message: e instanceof Error ? e.message : 'Could not load candles' });
  }

  private set(s: ChartLoadState): void {
    this.state = s;
    this.onState(s);
  }
}
```

- [ ] **Step 4: Run the test to check it passes**

Run: `cd frontend && npx vitest run src/chart/ChartDataController.test.ts`
Expected: PASS (14 tests).

- [ ] **Step 5: Commit**

```bash
git add frontend/src/chart/ChartDataController.ts frontend/src/chart/ChartDataController.test.ts frontend/src/chart/testing/FakeAdapter.ts
git commit -m "$(cat <<'EOF'
feat(chart): data controller with backfill wait, lazy history, live candles, refresh and race guard

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
)"
```

---