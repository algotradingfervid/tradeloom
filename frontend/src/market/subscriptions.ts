import type { TickDTO } from '../api/backend';
import { BindingError } from '../api/call';
import { parseBackendError } from '../api/errors';
import { QUOTE_CONSUMERS, type QuoteConsumer, type WatchResult } from '../api/quotes';

export const RELEASE_DELAY_MS = 30_000; // FR-WATCH-006: unsubscribe instruments no longer needed after 30 s
const DEBOUNCE_MS = 50;
/** A refused Watch is retried after these delays while its set is unsent; then on the next change or →live (Ruling 95 I2). */
export const RETRY_BACKOFF_MS: readonly number[] = [2_000, 5_000, 10_000];

/** What a refused Watch logs: the consumer, Go's code and stage, never the reason (it may carry account data). */
export interface WatchFailureLog { consumer: QuoteConsumer; code: string; stage: string; attempt: number }

const defaultLog = (message: string, fields: WatchFailureLog) => console.warn(message, fields);

function refusal(e: unknown): { code: string; stage: string } {
  if (e instanceof BindingError) return { code: e.code, stage: e.stage };
  const { code, stage } = parseBackendError(e);
  return { code, stage };
}

export interface CoordinatorDeps {
  watch(consumer: QuoteConsumer, tokens: number[]): Promise<WatchResult>;
  onSnapshot(quotes: TickDTO[]): void;
  releaseDelayMs?: number;
  debounceMs?: number;
  log?(message: string, fields: WatchFailureLog): void;
}

/** Go's answer for one token of one consumer and when that answer last changed (Unix ms). */
interface Decision { polled: boolean; atMs: number }

/**
 * One place that decides which tokens each ticker consumer (chart/watchlist/widget) needs.
 * UI sources register their token sets; the coordinator sends one merged Quotes.Watch per consumer,
 * keeps released tokens for 30 s, and remembers which tokens Go had to poll (budget, C-07).
 */
export class SubscriptionCoordinator {
  private readonly sources = new Map<string, { consumer: QuoteConsumer; tokens: Set<number> }>();
  private readonly lingering = new Map<QuoteConsumer, Map<number, number>>();
  private readonly lastSent = new Map<QuoteConsumer, string>();
  private readonly decided = new Map<QuoteConsumer, Map<number, Decision>>();
  /** Ruling 95 I1: consumers for which Go may still hold a non-empty set (set on a non-empty send, cleared by a successful empty one). */
  private readonly mayHold = new Set<QuoteConsumer>();
  private readonly failures = new Map<QuoteConsumer, number>();
  private readonly retryTimers = new Map<QuoteConsumer, ReturnType<typeof setTimeout>>();
  private readonly listeners = new Set<() => void>();
  private flushTimer: ReturnType<typeof setTimeout> | null = null;
  private releaseTimer: ReturnType<typeof setTimeout> | null = null;
  private flushing: Promise<void> = Promise.resolve();

  constructor(private readonly deps: CoordinatorDeps) {}

  set(sourceId: string, consumer: QuoteConsumer, tokens: readonly number[]): void {
    const prev = this.sources.get(sourceId);
    const before = new Map<QuoteConsumer, Set<number>>();
    if (prev) before.set(prev.consumer, this.wanted(prev.consumer));
    if (!before.has(consumer)) before.set(consumer, this.wanted(consumer));
    this.sources.set(sourceId, { consumer, tokens: new Set(tokens) });
    for (const [c, b] of before) { this.trackReleases(c, b); this.failures.delete(c); }
    this.schedule();
  }

  clear(sourceId: string): void {
    const prev = this.sources.get(sourceId);
    if (!prev) return;
    const before = this.wanted(prev.consumer);
    this.sources.delete(sourceId);
    this.trackReleases(prev.consumer, before);
    this.failures.delete(prev.consumer);
    this.schedule();
  }

  /** After a reconnect: re-send every consumer's current set (Go also resubscribes; this is idempotent). */
  resendAll(): void {
    this.lastSent.clear();
    this.failures.clear();
    this.schedule();
  }

  isPolled(token: number): boolean {
    let polled = false;
    for (const c of QUOTE_CONSUMERS) {
      const d = this.decided.get(c)?.get(token);
      if (d && !d.polled) return false;
      if (d?.polled) polled = true;
    }
    return polled;
  }

  /** When Go's live/polled answer for this token last changed (Unix ms, latest over consumers); 0 = never answered. */
  answeredAtMs(token: number): number {
    let at = 0;
    for (const c of QUOTE_CONSUMERS) at = Math.max(at, this.decided.get(c)?.get(token)?.atMs ?? 0);
    return at;
  }

  subscribe(listener: () => void): () => void {
    this.listeners.add(listener);
    return () => { this.listeners.delete(listener); };
  }

  idle(): Promise<void> { return this.flushing; }

  private wanted(c: QuoteConsumer): Set<number> {
    const out = new Set<number>();
    for (const s of this.sources.values()) if (s.consumer === c) s.tokens.forEach((t) => out.add(t));
    return out;
  }

  private trackReleases(c: QuoteConsumer, before: Set<number>): void {
    const after = this.wanted(c);
    const m = this.lingering.get(c) ?? new Map<number, number>();
    const releaseAt = Date.now() + (this.deps.releaseDelayMs ?? RELEASE_DELAY_MS);
    for (const t of before) if (!after.has(t) && !m.has(t)) m.set(t, releaseAt);
    for (const t of after) m.delete(t);
    this.lingering.set(c, m);
  }

  /** Stores Go's answer per token; true when any token's live/polled status changed (M4: notify only then). */
  private record(c: QuoteConsumer, r: WatchResult): boolean {
    const prev = this.decided.get(c) ?? new Map<number, Decision>();
    const next = new Map<number, Decision>();
    const now = Date.now();
    let changed = false;
    const put = (t: number, polled: boolean) => {
      const p = prev.get(t);
      if (p && p.polled === polled) next.set(t, p);
      else { next.set(t, { polled, atMs: now }); changed = true; }
    };
    r.live.forEach((t) => put(t, false));
    r.polled.forEach((t) => put(t, true));
    for (const t of prev.keys()) if (!next.has(t)) changed = true;
    this.decided.set(c, next);
    return changed;
  }

  private retryLater(c: QuoteConsumer, attempt: number): void {
    this.cancelRetry(c);
    const delay = RETRY_BACKOFF_MS[attempt - 1];
    if (delay === undefined) return; // bounded: wait for the next set change or the feed going live (resendAll)
    this.retryTimers.set(c, setTimeout(() => { this.retryTimers.delete(c); this.schedule(); }, delay));
  }

  private cancelRetry(c: QuoteConsumer): void {
    const t = this.retryTimers.get(c);
    if (t !== undefined) { clearTimeout(t); this.retryTimers.delete(c); }
  }

  private schedule(): void {
    if (this.flushTimer !== null) return;
    this.flushTimer = setTimeout(() => {
      this.flushTimer = null;
      this.flushing = this.flushing.then(() => this.flush());
    }, this.deps.debounceMs ?? DEBOUNCE_MS);
  }

  private async flush(): Promise<void> {
    const now = Date.now();
    let nextRelease = Number.POSITIVE_INFINITY;
    let changed = false;
    for (const c of QUOTE_CONSUMERS) {
      const set = this.wanted(c);
      const m = this.lingering.get(c);
      if (m) {
        for (const [t, at] of [...m]) {
          if (at <= now) m.delete(t);
          else { set.add(t); nextRelease = Math.min(nextRelease, at); }
        }
      }
      const tokens = [...set].sort((a, b) => a - b);
      const key = tokens.join(',');
      if (this.lastSent.get(c) === key) continue;
      if (tokens.length === 0 && !this.mayHold.has(c)) continue; // Go holds nothing for c: nothing to replace
      this.lastSent.set(c, key);
      if (tokens.length > 0) this.mayHold.add(c);
      try {
        const r = await this.deps.watch(c, tokens);
        if (tokens.length === 0) this.mayHold.delete(c);
        this.failures.delete(c);
        this.cancelRetry(c);
        if (this.record(c, r)) changed = true;
        if (r.snapshot.length > 0) this.deps.onSnapshot(r.snapshot);
      } catch (e) {
        this.lastSent.delete(c); // unsent: retried with backoff, then on the next change or resendAll()
        const attempt = (this.failures.get(c) ?? 0) + 1;
        this.failures.set(c, attempt);
        (this.deps.log ?? defaultLog)('Quotes.Watch refused', { consumer: c, ...refusal(e), attempt });
        this.retryLater(c, attempt);
      }
    }
    if (changed) this.listeners.forEach((l) => l());
    if (this.releaseTimer !== null) { clearTimeout(this.releaseTimer); this.releaseTimer = null; }
    if (nextRelease !== Number.POSITIVE_INFINITY) {
      this.releaseTimer = setTimeout(() => { this.releaseTimer = null; this.schedule(); }, nextRelease - now);
    }
  }
}
