import type { Bar, InstrumentSpec, RangeInfo, TimeRange, UpdateOutcome } from './types';
import type { Instrument } from '../instruments/types';
import { tickRupees } from '../instruments/naming';
import { timeframeById, type KiteInterval, type Timeframe, type TimeframeId } from './timeframes';
import { precisionForTick } from './format';
import { mergeBars } from './bars';
import type { BackfillProgress, CandleEvent, CandlesPage } from '../api/chart';
import { BindingError } from '../api/call';
import { registerReset } from '../store/reset';
import { useSessionStore } from '../store/session';

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
 * The part of the plan's ChartAdapter (ARCHITECTURE §8.5, Tasks 3–4) this controller drives. Any ChartAdapter
 * satisfies it structurally; the controller never sees lightweight-charts.
 */
export interface ChartDataSink {
  setInstrument(spec: InstrumentSpec): void;
  setCandles(bars: readonly Bar[], opts?: { keepView?: boolean }): void;
  prependHistory(bars: readonly Bar[]): number;
  updateLastBar(bar: Bar): UpdateOutcome;
  bars(): readonly Bar[];
  onVisibleRangeChange(cb: (r: RangeInfo) => void): () => void;
  visibleTimeRange(): TimeRange | null;
}

/**
 * Sign-out and a change of user (the event bridge's resetOnLogout, Ruling 98) cancel every controller's in-flight
 * work. Session expiry is not a reset: an answer that arrives while the same user's session is expired still applies.
 */
const resetHooks = new Set<() => void>();
registerReset(() => { for (const reset of [...resetHooks]) reset(); });

const sessionUser = (): string => useSessionStore.getState().session?.userId ?? '';

/** Captured before every await; a continuation applies only while it is still current. */
interface Ticket { seq: number; userId: string }
interface JobWaiter { jobId: string; finish: () => void }

/**
 * Owns one chart's data lifecycle. Every async continuation checks its ticket (request sequence + user) so a slow
 * response for an abandoned symbol, timeframe, window or user can never reach the chart (Review Focus 2).
 * Live candles are built by Go's candle builder (session-anchored IST buckets) and arrive as candle.forming /
 * candle.closed; the controller applies them, it never buckets ticks itself.
 */
export class ChartDataController {
  private seq = 0;
  private disposed = false;
  private inst: Instrument | null = null;
  private tf: Timeframe | null = null;
  private live: Promise<string> | null = null;
  private reachedStart = false;
  private olderBlockedUntil = 0;
  private pending: CandleEvent[] = [];
  private state: ChartLoadState = INITIAL_CHART_STATE;
  /** Backfill jobs that already reported done/failed, and the waits currently pending. */
  private readonly finishedJobs = new Set<string>();
  private readonly waiters = new Set<JobWaiter>();
  private readonly offCandle: () => void;
  private readonly offRange: () => void;
  private readonly offBackfill: () => void;
  private readonly onReset = () => this.reset();

  constructor(
    private readonly adapter: ChartDataSink,
    private readonly api: ChartDataApi,
    private readonly onState: (s: ChartLoadState) => void,
    private readonly onLiveBar?: () => void,
  ) {
    this.offCandle = api.onCandle((e) => this.onCandleEvent(e));
    this.offBackfill = api.onBackfillProgress((list) => {
      for (const p of list) {
        if (p.state === 'running') continue;
        this.finishedJobs.add(p.jobId);
        for (const w of [...this.waiters]) if (w.jobId === p.jobId) w.finish();
      }
    });
    this.offRange = adapter.onVisibleRangeChange((r) => {
      if (r.barsBefore < LAZY_LOAD_THRESHOLD) void this.loadOlder();
    });
    resetHooks.add(this.onReset);
  }

  get current(): ChartLoadState { return this.state; }

  async load(inst: Instrument, tfId: TimeframeId): Promise<void> {
    if (this.disposed) return;
    const tf = timeframeById(tfId);
    this.cancel();
    const t = this.ticket();
    this.inst = inst;
    this.tf = tf;
    this.reachedStart = false;
    this.olderBlockedUntil = 0;
    this.finishedJobs.clear();
    const tick = tickRupees(inst);
    this.adapter.setInstrument({ intraday: tf.intraday, hasVolume: !inst.isIndex, pricePrecision: precisionForTick(tick), minMove: tick });
    this.adapter.setCandles([]);
    this.set({ ...INITIAL_CHART_STATE, status: 'loading' });
    // Subscribe before fetching so a candle that closes during the fetch is buffered, not lost.
    this.live = this.api.openLive(inst.token, tf.kite);
    this.live.catch(() => undefined); // Go refuses no_data_plan / not_found / budget: history still loads
    try {
      const page = await this.fetchWindow(t, inst, tf, 0);
      if (!page) return;
      if (page.needsDataPlan) {
        this.releaseLive();
        this.pending = [];
        this.set({ ...INITIAL_CHART_STATE, status: 'no_data_plan', message: NO_DATA_PLAN_MESSAGE });
        return;
      }
      this.reachedStart = page.complete && page.bars.length === 0;
      this.adapter.setCandles(page.bars);
      for (const e of this.pending) this.adapter.updateLastBar(this.liveBar(e));
      this.pending = [];
      const empty = this.adapter.bars().length === 0;
      if (empty && page.backfillJobId) {
        this.set({ ...INITIAL_CHART_STATE, status: 'error', message: BACKFILL_TIMEOUT_MESSAGE });
        return;
      }
      this.set({ ...INITIAL_CHART_STATE, status: empty ? 'empty' : 'ready', offline: page.offline, message: empty ? EMPTY_MESSAGE : null });
    } catch (e) {
      if (!this.isCurrent(t)) { this.dropped(t); return; }
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
    const t = this.ticket();
    this.set({ ...this.state, loadingOlder: true });
    try {
      const page = await this.fetchWindow(t, inst, tf, first.time - 1);
      if (!page) return;
      const added = this.adapter.prependHistory(page.bars);
      if (page.complete && page.bars.length === 0) this.reachedStart = true; // contract: reachedStart = complete && no candles
      else if (page.bars.length > 0 && added === 0) this.reachedStart = true; // nothing older than what is loaded
      else if (added === 0) this.olderBlockedUntil = Date.now() + OLDER_RETRY_MS; // backfill still running: back off
    } catch {
      if (!this.isCurrent(t)) { this.dropped(t); return; }
      this.olderBlockedUntil = Date.now() + OLDER_RETRY_MS;
    } finally {
      if (t.seq === this.seq) this.set({ ...this.state, loadingOlder: false });
    }
  }

  async refresh(): Promise<void> {
    const { inst, tf } = this;
    const first = this.adapter.bars()[0];
    if (!inst || !tf || !first || this.state.refreshing || this.state.status !== 'ready') return;
    const t = this.ticket();
    const from = this.adapter.visibleTimeRange()?.from ?? first.time;
    this.set({ ...this.state, refreshing: true, message: null });
    try {
      const r = await this.api.refreshRange(inst.token, tf.kite, from, Date.now());
      if (!this.isCurrent(t)) { this.dropped(t); return; }
      this.adapter.setCandles(mergeBars(this.adapter.bars(), r.bars), { keepView: true });
      this.set({ ...this.state, refreshing: false, offline: r.offline });
    } catch (e) {
      if (!this.isCurrent(t)) { this.dropped(t); return; }
      this.set({ ...this.state, refreshing: false, message: e instanceof Error ? e.message : String(e) });
    }
  }

  dispose(): void {
    if (this.disposed) return;
    this.disposed = true;
    resetHooks.delete(this.onReset);
    this.offCandle();
    this.offRange();
    this.offBackfill();
    this.cancel();
  }

  /**
   * One history window. If Go submitted a backfill job, wait for its backfill.progress (≤ BACKFILL_WAIT_MS in total)
   * and ask again; a job id is waited for once. Returns null when this request is no longer current.
   */
  private async fetchWindow(t: Ticket, inst: Instrument, tf: Timeframe, toMs: number): Promise<CandlesPage | null> {
    let page = await this.api.fetchHistory(inst.token, tf.kite, toMs, tf.initialTradingDays);
    const deadline = Date.now() + BACKFILL_WAIT_MS;
    const waited = new Set<string>();
    while (this.isCurrent(t) && page.backfillJobId && !waited.has(page.backfillJobId) && Date.now() < deadline) {
      waited.add(page.backfillJobId);
      await this.waitForJob(page.backfillJobId, deadline - Date.now());
      if (!this.isCurrent(t)) break;
      page = await this.api.fetchHistory(inst.token, tf.kite, toMs, tf.initialTradingDays);
    }
    if (this.isCurrent(t)) return page;
    this.dropped(t);
    return null;
  }

  private waitForJob(jobId: string, ms: number): Promise<void> {
    if (this.finishedJobs.has(jobId) || ms <= 0) return Promise.resolve();
    return new Promise((resolve) => {
      const w: JobWaiter = {
        jobId,
        finish: () => {
          clearTimeout(timer);
          this.waiters.delete(w);
          resolve();
        },
      };
      const timer = setTimeout(w.finish, ms);
      this.waiters.add(w);
    });
  }

  private onCandleEvent(e: CandleEvent): void {
    if (!this.inst || !this.tf || e.token !== this.inst.token || e.interval !== this.tf.kite) return;
    if (this.state.status === 'loading') { this.pending.push(e); return; }
    if (this.state.status !== 'ready' && this.state.status !== 'empty') return;
    const outcome = this.adapter.updateLastBar(this.liveBar(e));
    if (this.state.status === 'empty' && outcome === 'appended') this.set({ ...this.state, status: 'ready', message: null });
    if (outcome !== 'ignored') this.onLiveBar?.();
  }

  /**
   * Go marks a candle partial when tracking began after it started: its open and volume are unknown. When that bar is
   * already loaded (history served it), keep the loaded open and volume and widen high/low; otherwise show Go's bar.
   */
  private liveBar(e: CandleEvent): Bar {
    if (!e.partial) return e.bar;
    const loaded = this.adapter.bars();
    let i = loaded.length - 1;
    while (i >= 0 && loaded[i].time > e.bar.time) i--;
    const have = i >= 0 && loaded[i].time === e.bar.time ? loaded[i] : null;
    if (!have) return e.bar;
    return {
      time: e.bar.time, open: have.open, high: Math.max(have.high, e.bar.high), low: Math.min(have.low, e.bar.low),
      close: e.bar.close, volume: Math.max(have.volume, e.bar.volume),
    };
  }

  private ticket(): Ticket { return { seq: this.seq, userId: sessionUser() }; }

  /** Current = no newer load/reset/dispose, and no different user signed in since the ticket was taken. */
  private isCurrent(t: Ticket): boolean {
    if (t.seq !== this.seq) return false;
    const now = sessionUser();
    return t.userId === '' || now === '' || now === t.userId;
  }

  /** A dropped answer whose sequence is still current means the user changed without a reset: reset now. */
  private dropped(t: Ticket): void {
    if (t.seq === this.seq && !this.disposed) this.reset();
  }

  /** Sign-out or a change of user: drop in-flight work, release live, clear the chart. retry() reloads. */
  private reset(): void {
    this.cancel();
    this.adapter.setCandles([]);
    this.set({ ...INITIAL_CHART_STATE });
  }

  /** Supersedes every in-flight request: bumps the sequence, ends pending backfill waits, releases live. */
  private cancel(): void {
    this.seq++;
    for (const w of [...this.waiters]) w.finish();
    this.releaseLive();
    this.pending = [];
  }

  private releaseLive(): void {
    const live = this.live;
    this.live = null;
    if (live) void live.then((id) => this.api.closeLive(id)).catch(() => undefined);
  }

  private fail(e: unknown): void {
    this.releaseLive();
    this.pending = [];
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
