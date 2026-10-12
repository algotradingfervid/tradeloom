import type { TickDTO } from '../api/backend';
import { QUOTE_CONSUMERS, type QuoteConsumer, type WatchResult } from '../api/quotes';

export const RELEASE_DELAY_MS = 30_000; // FR-WATCH-006: unsubscribe instruments no longer needed after 30 s
const DEBOUNCE_MS = 50;

export interface CoordinatorDeps {
  watch(consumer: QuoteConsumer, tokens: number[]): Promise<WatchResult>;
  onSnapshot(quotes: TickDTO[]): void;
  releaseDelayMs?: number;
  debounceMs?: number;
}

/**
 * One place that decides which tokens each ticker consumer (chart/watchlist/widget) needs.
 * UI sources register their token sets; the coordinator sends one merged Quotes.Watch per consumer,
 * keeps released tokens for 30 s, and remembers which tokens Go had to poll (budget, C-07).
 */
export class SubscriptionCoordinator {
  private readonly sources = new Map<string, { consumer: QuoteConsumer; tokens: Set<number> }>();
  private readonly lingering = new Map<QuoteConsumer, Map<number, number>>();
  private readonly lastSent = new Map<QuoteConsumer, string>();
  private readonly liveBy = new Map<QuoteConsumer, Set<number>>();
  private readonly polledBy = new Map<QuoteConsumer, Set<number>>();
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
    for (const [c, b] of before) this.trackReleases(c, b);
    this.schedule();
  }

  clear(sourceId: string): void {
    const prev = this.sources.get(sourceId);
    if (!prev) return;
    const before = this.wanted(prev.consumer);
    this.sources.delete(sourceId);
    this.trackReleases(prev.consumer, before);
    this.schedule();
  }

  /** After a reconnect: re-send every consumer's current set (Go also resubscribes; this is idempotent). */
  resendAll(): void {
    this.lastSent.clear();
    this.schedule();
  }

  isPolled(token: number): boolean {
    let polled = false;
    for (const c of QUOTE_CONSUMERS) {
      if (this.liveBy.get(c)?.has(token)) return false;
      if (this.polledBy.get(c)?.has(token)) polled = true;
    }
    return polled;
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
      if (!this.lastSent.has(c) && tokens.length === 0) continue;
      this.lastSent.set(c, key);
      try {
        const r = await this.deps.watch(c, tokens);
        this.liveBy.set(c, new Set(r.live));
        this.polledBy.set(c, new Set(r.polled));
        if (r.snapshot.length > 0) this.deps.onSnapshot(r.snapshot);
        changed = true;
      } catch {
        this.lastSent.delete(c); // retried on the next change or resendAll()
      }
    }
    if (changed) this.listeners.forEach((l) => l());
    if (this.releaseTimer !== null) { clearTimeout(this.releaseTimer); this.releaseTimer = null; }
    if (nextRelease !== Number.POSITIVE_INFINITY) {
      this.releaseTimer = setTimeout(() => { this.releaseTimer = null; this.schedule(); }, nextRelease - now);
    }
  }
}
