import type { ChartAdapter } from './ChartAdapter';
import type { Bar } from './types';
import type { Instrument } from '../instruments/types';
import { tickRupees } from '../instruments/naming';
import { MAX_REFRESH_SPAN_MS, timeframeById, type KiteInterval, type Timeframe, type TimeframeId } from './timeframes';
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
/** Finished backfill job ids remembered per load (oldest forgotten first). */
const FINISHED_JOBS_CAP = 256;
/** Refresh starts this much inside Go's per-request span so the request never crosses it. */
const REFRESH_SPAN_MARGIN_MS = 60_000;

export type ChartStatus = 'idle' | 'loading' | 'ready' | 'empty' | 'error' | 'no_data_plan';
export interface ChartLoadState {
  status: ChartStatus; offline: boolean; refreshing: boolean; loadingOlder: boolean; message: string | null;
  /** Why live candles are off while history shows (Go refused OpenLive: subscription_budget, not_found, …). */
  liveMessage: string | null;
  /**
   * Bumped on every sign-out / user-change reset (the controller then sits in 'idle'). Hosts reload with retry() once
   * the session is ready again, e.g. an effect keyed on [resetEpoch, session ready + userId] that calls retry() when
   * status is 'idle'.
   */
  resetEpoch: number;
}
export const INITIAL_CHART_STATE: ChartLoadState = {
  status: 'idle', offline: false, refreshing: false, loadingOlder: false, message: null, liveMessage: null, resetEpoch: 0,
};

export interface ChartDataApi {
  fetchHistory(token: number, interval: KiteInterval, toMs: number, tradingDays: number): Promise<CandlesPage>;
  refreshRange(token: number, interval: KiteInterval, fromMs: number, toMs: number): Promise<CandlesPage>;
  openLive(token: number, interval: KiteInterval): Promise<string>;
  closeLive(id: string): Promise<void>;
  onCandle(cb: (e: CandleEvent) => void): () => void;
  onBackfillProgress(cb: (progress: BackfillProgress[]) => void): () => void;
}

/** The part of ChartAdapter (ARCHITECTURE §8.5) this controller drives; the controller never sees lightweight-charts. */
export type ChartDataSink = Pick<
  ChartAdapter,
  'setInstrument' | 'setCandles' | 'prependHistory' | 'updateLastBar' | 'bars' | 'onVisibleRangeChange' | 'visibleTimeRange'
>;

/**
 * Sign-out and a change of user (the event bridge's resetOnLogout, Ruling 98) cancel every controller's in-flight
 * work. Session expiry is not a reset: an answer that arrives while the same user's session is expired still applies.
 */
const resetHooks = new Set<() => void>();
registerReset(() => { for (const reset of [...resetHooks]) reset(); });

const sessionUser = (): string => useSessionStore.getState().session?.userId ?? '';

/** Captured before every await; a continuation applies only while it is still current. */
interface Ticket { seq: number; userId: string }
interface JobWaiter { jobId: string; finish: (reported: boolean) => void }
/** One window's answer: the re-fetched page after a backfill wait, and whether that wait ran out unreported. */
interface WindowAnswer { page: CandlesPage; timedOut: boolean }

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
  /** Live events that arrived during loading: the last one per bar time. */
  private pending = new Map<number, CandleEvent>();
  /** The newest live bar applied to the chart (re-applied after a history merge so it is never rolled back). */
  private lastLive: Bar | null = null;
  private liveMessage: string | null = null;
  private resets = 0;
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
        this.finishedJobs.delete(p.jobId);
        this.finishedJobs.add(p.jobId);
        if (this.finishedJobs.size > FINISHED_JOBS_CAP) this.finishedJobs.delete(this.finishedJobs.values().next().value as string);
        for (const w of [...this.waiters]) if (w.jobId === p.jobId) w.finish(true);
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
    const live = this.api.openLive(inst.token, tf.kite);
    this.live = live;
    // Go refuses no_data_plan / not_found / subscription_budget: history still loads; say why live is off.
    live.catch((e: unknown) => {
      if (this.live !== live || !this.isCurrent(t)) return;
      this.live = null;
      this.liveMessage = e instanceof Error ? e.message : String(e);
      this.set({ ...this.state });
    });
    let drawnEarly = false;
    try {
      const answer = await this.fetchWindow(t, inst, tf, 0, (first) => {
        // Go answers a window that is still being backfilled with what it has: draw that now, merge the rest later.
        if (first.bars.length === 0) return;
        drawnEarly = true;
        this.drawHistory(first);
      });
      if (!answer) return;
      const { page, timedOut } = answer;
      if (page.needsDataPlan) {
        this.releaseLive();
        this.pending.clear();
        this.adapter.setCandles([]);
        this.set({ ...INITIAL_CHART_STATE, status: 'no_data_plan', message: NO_DATA_PLAN_MESSAGE });
        return;
      }
      if (drawnEarly) {
        this.adapter.setCandles(mergeBars(this.adapter.bars(), page.bars), { keepView: true });
        if (this.lastLive) this.adapter.updateLastBar(this.lastLive);
        if (this.state.status !== 'ready' || this.state.offline !== page.offline) {
          this.set({ ...this.state, status: 'ready', message: null, offline: page.offline });
        }
        return;
      }
      this.reachedStart = page.complete && page.bars.length === 0;
      if (page.bars.length === 0 && page.backfillJobId && timedOut) {
        this.releaseLive();
        this.pending.clear();
        this.set({ ...INITIAL_CHART_STATE, status: 'error', message: BACKFILL_TIMEOUT_MESSAGE });
        return;
      }
      this.drawHistory(page);
    } catch (e) {
      if (!this.isCurrent(t)) { this.dropped(t); return; }
      if (drawnEarly) return; // the first answer is on screen; the re-fetch after the backfill wait failed
      this.fail(e);
    }
  }

  /** Draws a first history answer, replays the live events buffered meanwhile, and leaves loading. */
  private drawHistory(page: CandlesPage): void {
    this.adapter.setCandles(page.bars);
    const buffered = [...this.pending.values()].sort((a, b) => a.bar.time - b.bar.time);
    this.pending.clear();
    for (const e of buffered) {
      const bar = this.liveBar(e);
      if (this.adapter.updateLastBar(bar) !== 'ignored') this.noteLive(bar);
    }
    const empty = this.adapter.bars().length === 0;
    this.set({ ...INITIAL_CHART_STATE, status: empty ? 'empty' : 'ready', offline: page.offline, message: empty ? EMPTY_MESSAGE : null });
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
      const answer = await this.fetchWindow(t, inst, tf, first.time - 1);
      if (!answer) return;
      const { page } = answer;
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
    const now = Date.now();
    // Go refuses a range wider than Kite serves per request (invalid_range): refresh the newest part of the view.
    const from = Math.max(this.adapter.visibleTimeRange()?.from ?? first.time, now - MAX_REFRESH_SPAN_MS[tf.kite] + REFRESH_SPAN_MARGIN_MS);
    this.set({ ...this.state, refreshing: true, message: null });
    try {
      const r = await this.api.refreshRange(inst.token, tf.kite, from, now);
      if (!this.isCurrent(t)) { this.dropped(t); return; }
      this.adapter.setCandles(mergeBars(this.adapter.bars(), r.bars), { keepView: true });
      if (this.lastLive) this.adapter.updateLastBar(this.lastLive);
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
   * One history window. If Go submitted a backfill job, `onFirst` sees the first answer, then the controller waits for
   * that job's backfill.progress (≤ BACKFILL_WAIT_MS) and asks once more — never in a loop: Go's coverage of the
   * current window stops just before now, so every Candles(toMs = 0) can carry a fresh job id (Ruling 109 C1).
   * Returns null when this request is no longer current.
   */
  private async fetchWindow(
    t: Ticket, inst: Instrument, tf: Timeframe, toMs: number, onFirst?: (first: CandlesPage) => void,
  ): Promise<WindowAnswer | null> {
    const first = await this.api.fetchHistory(inst.token, tf.kite, toMs, tf.initialTradingDays);
    if (!this.isCurrent(t)) { this.dropped(t); return null; }
    if (!first.backfillJobId || first.needsDataPlan) return { page: first, timedOut: false };
    onFirst?.(first);
    const reported = await this.waitForJob(first.backfillJobId, BACKFILL_WAIT_MS);
    if (!this.isCurrent(t)) { this.dropped(t); return null; }
    const page = await this.api.fetchHistory(inst.token, tf.kite, toMs, tf.initialTradingDays);
    if (!this.isCurrent(t)) { this.dropped(t); return null; }
    return { page, timedOut: !reported };
  }

  /** Resolves true when the job reported done/failed (or already had), false when the wait ran out or was ended. */
  private waitForJob(jobId: string, ms: number): Promise<boolean> {
    if (this.finishedJobs.has(jobId)) return Promise.resolve(true);
    return new Promise((resolve) => {
      const w: JobWaiter = {
        jobId,
        finish: (reported) => {
          clearTimeout(timer);
          this.waiters.delete(w);
          resolve(reported);
        },
      };
      const timer = setTimeout(() => w.finish(false), ms);
      this.waiters.add(w);
    });
  }

  private onCandleEvent(e: CandleEvent): void {
    if (!this.inst || !this.tf || e.token !== this.inst.token || e.interval !== this.tf.kite) return;
    if (this.state.status === 'loading') { this.pending.set(e.bar.time, e); return; }
    if (this.state.status !== 'ready' && this.state.status !== 'empty') return;
    const bar = this.liveBar(e);
    const outcome = this.adapter.updateLastBar(bar);
    if (outcome === 'ignored') return;
    this.noteLive(bar);
    if (this.state.status === 'empty' && outcome === 'appended') this.set({ ...this.state, status: 'ready', message: null });
    this.onLiveBar?.();
  }

  private noteLive(bar: Bar): void {
    if (!this.lastLive || bar.time >= this.lastLive.time) this.lastLive = bar;
  }

  /**
   * Go marks a candle partial when tracking began after it started: its open and volume are unknown (builder.go). An
   * intraday partial candle's volume is not this candle's volume (it can be the cumulative day volume), so it is never
   * shown: a loaded bar keeps its open and volume and widens high/low; a new bar shows volume 0. A day candle's OHLC and
   * volume come from the quote itself, so it is drawn as Go sends it.
   */
  private liveBar(e: CandleEvent): Bar {
    if (!e.partial || !this.tf?.intraday) return e.bar;
    const loaded = this.adapter.bars();
    let i = loaded.length - 1;
    while (i >= 0 && loaded[i].time > e.bar.time) i--;
    const have = i >= 0 && loaded[i].time === e.bar.time ? loaded[i] : null;
    if (!have) return { ...e.bar, volume: 0 };
    return {
      time: e.bar.time, open: have.open, high: Math.max(have.high, e.bar.high), low: Math.min(have.low, e.bar.low),
      close: e.bar.close, volume: have.volume,
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
    this.resets++;
    this.adapter.setCandles([]);
    this.set({ ...INITIAL_CHART_STATE });
  }

  /** Supersedes every in-flight request: bumps the sequence, ends pending backfill waits, releases live. */
  private cancel(): void {
    this.seq++;
    for (const w of [...this.waiters]) w.finish(false);
    this.releaseLive();
    this.pending.clear();
    this.lastLive = null;
    this.liveMessage = null;
  }

  private releaseLive(): void {
    const live = this.live;
    this.live = null;
    if (live) void live.then((id) => this.api.closeLive(id)).catch(() => undefined);
  }

  private fail(e: unknown): void {
    this.releaseLive();
    this.pending.clear();
    if (e instanceof BindingError && e.code === 'no_data_plan') {
      this.set({ ...INITIAL_CHART_STATE, status: 'no_data_plan', message: NO_DATA_PLAN_MESSAGE });
      return;
    }
    this.set({ ...INITIAL_CHART_STATE, status: 'error', message: e instanceof Error ? e.message : 'Could not load candles' });
  }

  private set(s: ChartLoadState): void {
    this.state = { ...s, liveMessage: this.liveMessage, resetEpoch: this.resets };
    this.onState(this.state);
  }
}
