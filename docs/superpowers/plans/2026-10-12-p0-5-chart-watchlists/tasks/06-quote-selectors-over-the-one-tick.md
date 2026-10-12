> **P0-5 Chart, Watchlists & Ticker UI Implementation Plan — Task 6.** Read [`../MANIFEST.md`](../MANIFEST.md) first: its Global Constraints,
> Review Focus and Consumes/Produces apply to this task. Other tasks live beside this file in `tasks/`.

<!-- task body (verbatim from the plan) -->
### Task 6: Quote selectors over the one tick store, subscription coordinator and market-data bridge

There is **one** tick store: P0-4's `useTicksStore` (`latest: Record<number, TickDTO>`, `applyBatch`), already fed from `tick.batch` by P0-4's event bridge. This task adds selectors over it, the coordinator that merges every on-screen token set into one `Quotes.Watch` per consumer, and the bridge that registers the backend and re-sends subscriptions after a reconnect.

**Files:**
- Create: `frontend/src/market/quotes.ts`, `frontend/src/market/testing.ts`, `frontend/src/market/subscriptions.ts`, `frontend/src/market/index.ts`, `frontend/src/market/hooks.ts`, `frontend/src/market/MarketDataBridge.tsx`
- Test: `frontend/src/market/quotes.test.ts`, `frontend/src/market/subscriptions.test.ts`, `frontend/src/market/MarketDataBridge.test.tsx`
- Modify: `frontend/src/app/App.tsx` (render `<MarketDataBridge />` once inside `<BackendProvider>`)

**Interfaces:**
- Consumes: Task 2 `QuoteConsumer`, `QUOTE_CONSUMERS`, `WatchResult`, `watchQuotes`, `setBackend`. P0-4 `TickDTO`, `FeedState`, `useTicksStore` (`latest`, `applyBatch`), `useStatusStore` (zustand with `.subscribe((s, prev) => …)`; `feed.state` ∈ `stopped|connecting|live|reconnecting|offline`), `useBackend`, `BackendProvider`, `createMockBackend`, `demoSeed`.
- Produces:
  - `useQuote(token?: number): TickDTO | undefined`, `quoteChange(q?: TickDTO): { abs: number; pct: number } | null` (from the DTO's `change` / `changePct`; `null` when `close` is not > 0), `applySnapshot(snapshot)` (never overwrites a newer tick)
  - `tick(token, ltp, close, over?)` test helper (`market/testing.ts`)
  - `class SubscriptionCoordinator { set(sourceId, consumer, tokens): void; clear(sourceId): void; resendAll(): void; isPolled(token): boolean; subscribe(listener): () => void; idle(): Promise<void> }`, `RELEASE_DELAY_MS = 30000`
  - `subscriptions` (app singleton), `useSubscription(sourceId: string, consumer: QuoteConsumer, tokens: readonly number[]): void`, `usePolled(token: number): boolean` (the tick's `polled` flag, or Go's `Watch` answer before the first polled tick arrives)
  - `<MarketDataBridge />`

- [ ] **Step 1: Write the failing tests**

`frontend/src/market/quotes.test.ts`:

```ts
import { describe, expect, it } from 'vitest';
import { useTicksStore } from '../store/ticks';
import { applySnapshot, quoteChange } from './quotes';
import { tick } from './testing';

describe('quote selectors over useTicksStore', () => {
  it('FR-WATCH-005 AC4: LTP 1,000 vs previous close 1,010 is -0.99%', () => {
    const ch = quoteChange(tick(1, 1000, 1010))!;
    expect(ch.abs).toBe(-10);
    expect(ch.pct.toFixed(2)).toBe('-0.99');
  });
  it('no previous close → no change (never a fake 0.00%)', () => {
    expect(quoteChange(tick(1, 10, 0))).toBeNull();
    expect(quoteChange(undefined)).toBeNull();
  });
  it('a Watch snapshot never overwrites a newer live tick', () => {
    useTicksStore.getState().applyBatch([tick(1, 11, 9, { tsMs: 200 })]);
    applySnapshot([tick(1, 10, 9, { tsMs: 100 }), tick(2, 5, 4, { tsMs: 100 })]);
    expect(useTicksStore.getState().latest[1].ltp).toBe(11);
    expect(useTicksStore.getState().latest[2].ltp).toBe(5);
  });
});
```

`frontend/src/market/subscriptions.test.ts`:

```ts
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { SubscriptionCoordinator } from './subscriptions';
import type { WatchResult } from '../api/quotes';
import { tick } from './testing';

let watch: ReturnType<typeof vi.fn>;
let onSnapshot: ReturnType<typeof vi.fn>;
let c: SubscriptionCoordinator;
const allLive = async (_c: string, tokens: number[]): Promise<WatchResult> => ({ live: tokens, polled: [], snapshot: [] });
const step = (ms: number) => vi.advanceTimersByTimeAsync(ms);

beforeEach(() => {
  vi.useFakeTimers({ now: Date.UTC(2026, 9, 9, 4, 0) });
  watch = vi.fn(allLive);
  onSnapshot = vi.fn();
  c = new SubscriptionCoordinator({ watch, onSnapshot });
});
afterEach(() => vi.useRealTimers());

describe('SubscriptionCoordinator', () => {
  it('merges sources of one consumer into a single debounced Watch call', async () => {
    c.set('a', 'watchlist', [1, 2]);
    c.set('b', 'watchlist', [2, 3]);
    await step(50);
    expect(watch).toHaveBeenCalledTimes(1);
    expect(watch).toHaveBeenCalledWith('watchlist', [1, 2, 3]);
  });

  it('FR-WATCH-006 AC1: list switch subscribes B now and drops A-only tokens after 30 s', async () => {
    c.set('wl', 'watchlist', [1, 2]);
    await step(50);
    c.set('wl', 'watchlist', [3]);
    await step(50);
    expect(watch).toHaveBeenLastCalledWith('watchlist', [1, 2, 3]);
    await step(29_000);
    expect(watch).toHaveBeenCalledTimes(2);
    await step(1_100);
    expect(watch).toHaveBeenLastCalledWith('watchlist', [3]);
  });

  it('re-adding within 30 s cancels the release (no churn)', async () => {
    c.set('wl', 'watchlist', [1]);
    await step(50);
    c.set('wl', 'watchlist', []);
    await step(10_000);
    c.set('wl', 'watchlist', [1]);
    await step(40_000);
    expect(watch).toHaveBeenCalledTimes(1);
  });

  it('250 tokens: one Watch call, budget overflow marked polled', async () => {
    watch.mockImplementation(async (_c: string, tokens: number[]) => ({ live: tokens.slice(0, 180), polled: tokens.slice(180), snapshot: [] }));
    const listener = vi.fn();
    c.subscribe(listener);
    const tokens = Array.from({ length: 250 }, (_, i) => i + 1);
    c.set('wl', 'watchlist', tokens);
    await step(50);
    expect(watch).toHaveBeenCalledTimes(1);
    expect(c.isPolled(1)).toBe(false);
    expect(c.isPolled(181)).toBe(true);
    expect(c.isPolled(250)).toBe(true);
    expect(listener).toHaveBeenCalledTimes(1);
    await step(60_000);
    expect(watch).toHaveBeenCalledTimes(1); // polled tokens are not retried as live in a loop
  });

  it('a token live for the chart is not "polled" even if the watchlist consumer was refused', async () => {
    watch.mockImplementation(async (consumer: string, tokens: number[]) =>
      consumer === 'chart' ? { live: tokens, polled: [], snapshot: [] } : { live: [], polled: tokens, snapshot: [] });
    c.set('chart', 'chart', [5]);
    c.set('wl', 'watchlist', [5, 6]);
    await step(50);
    expect(c.isPolled(5)).toBe(false);
    expect(c.isPolled(6)).toBe(true);
  });

  it('hands the last-quote snapshot to the tick store (outside market hours)', async () => {
    const snap = [tick(1, 22600, 22500)];
    watch.mockImplementation(async () => ({ live: [1], polled: [], snapshot: snap }));
    c.set('strip', 'widget', [1]);
    await step(50);
    expect(onSnapshot).toHaveBeenCalledWith(snap);
  });

  it('resendAll re-sends unchanged sets (reconnect)', async () => {
    c.set('wl', 'watchlist', [1, 2]);
    await step(50);
    c.resendAll();
    await step(50);
    expect(watch).toHaveBeenCalledTimes(2);
    expect(watch).toHaveBeenLastCalledWith('watchlist', [1, 2]);
  });

  it('a failed Watch is retried on the next change', async () => {
    watch.mockRejectedValueOnce(new Error('logged out'));
    c.set('wl', 'watchlist', [1]);
    await step(50);
    c.resendAll();
    await step(50);
    expect(watch).toHaveBeenCalledTimes(2);
  });
});
```

`frontend/src/market/MarketDataBridge.test.tsx`:

```tsx
import { render } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';

const resendAll = vi.fn();
vi.mock('./index', () => ({ subscriptions: { resendAll: () => resendAll() } }));

import type { FeedState } from '../api/backend';
import { BackendProvider } from '../api/BackendContext';
import { currentBackend } from '../api/current';
import { demoSeed } from '../api/mock/fixtures';
import { createMockBackend } from '../api/mock/index';
import { useStatusStore } from '../store/status';
import { MarketDataBridge } from './MarketDataBridge';

const setFeedState = (state: FeedState['state']) => useStatusStore.setState({ feed: { ...useStatusStore.getState().feed, state } });

describe('MarketDataBridge', () => {
  it('registers the backend for the market helpers and re-sends subscriptions when the feed becomes live', () => {
    const backend = createMockBackend(demoSeed());
    setFeedState('offline');
    const { unmount } = render(<BackendProvider backend={backend}><MarketDataBridge /></BackendProvider>);
    expect(currentBackend()).toBe(backend);
    setFeedState('reconnecting');
    expect(resendAll).not.toHaveBeenCalled();
    setFeedState('live');
    expect(resendAll).toHaveBeenCalledTimes(1);
    unmount();
    setFeedState('offline');
    setFeedState('live');
    expect(resendAll).toHaveBeenCalledTimes(1);
  });
});
```

- [ ] **Step 2: Run the tests to check they fail**

Run: `cd frontend && npx vitest run src/market`
Expected: FAIL with `Failed to resolve import "./quotes"` (and likewise `./subscriptions`, `./MarketDataBridge`).

- [ ] **Step 3: Implement**

`frontend/src/market/quotes.ts`:

```ts
import type { TickDTO } from '../api/backend';
import { useTicksStore } from '../store/ticks';

/** Latest tick for one token from P0-4's single tick store; only components reading that token re-render. */
export function useQuote(token: number | undefined): TickDTO | undefined {
  return useTicksStore((s) => (token === undefined ? undefined : s.latest[token]));
}

/** Change vs the previous close as Go computed it; null without a previous close (never a fake 0.00%). */
export function quoteChange(q: TickDTO | undefined): { abs: number; pct: number } | null {
  if (!q || !(q.close > 0)) return null;
  return { abs: q.change, pct: q.changePct };
}

/** Quotes.Watch snapshot → tick store, without overwriting a newer live tick that already arrived. */
export function applySnapshot(snapshot: readonly TickDTO[]): void {
  const latest = useTicksStore.getState().latest;
  const fresh = snapshot.filter((q) => !(latest[q.token] && latest[q.token].tsMs >= q.tsMs));
  if (fresh.length > 0) useTicksStore.getState().applyBatch(fresh);
}
```

`frontend/src/market/testing.ts` (test helper; also used by later tasks' tests):

```ts
import type { TickDTO } from '../api/backend';

/** A TickDTO with change/changePct derived the way Go derives them from the previous close. */
export function tick(token: number, ltp: number, close: number, over: Partial<TickDTO> = {}): TickDTO {
  const change = ltp - close;
  return { token, ltp, close, change, changePct: close > 0 ? (change / close) * 100 : 0, volume: 0, tsMs: 1, polled: false, ...over };
}
```

`frontend/src/market/subscriptions.ts`:

```ts
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
```

`frontend/src/market/index.ts`:

```ts
import { watchQuotes } from '../api/quotes';
import { applySnapshot } from './quotes';
import { SubscriptionCoordinator } from './subscriptions';

/** The app's single coordinator: one merged Quotes.Watch per consumer (ARCHITECTURE §8.2). */
export const subscriptions = new SubscriptionCoordinator({ watch: watchQuotes, onSnapshot: applySnapshot });
```

`frontend/src/market/hooks.ts`:

```ts
import { useEffect, useSyncExternalStore } from 'react';
import type { QuoteConsumer } from '../api/quotes';
import { useTicksStore } from '../store/ticks';
import { subscriptions } from './index';

export function useSubscription(sourceId: string, consumer: QuoteConsumer, tokens: readonly number[]): void {
  const key = tokens.join(',');
  useEffect(() => {
    subscriptions.set(sourceId, consumer, key ? key.split(',').map(Number) : []);
  }, [sourceId, consumer, key]);
  useEffect(() => () => subscriptions.clear(sourceId), [sourceId]);
}

/** Polled = the latest tick says `polled`, or Go's Watch answer put the token on the 5 s poller (before its first tick). */
export function usePolled(token: number): boolean {
  const fromWatch = useSyncExternalStore((l) => subscriptions.subscribe(l), () => subscriptions.isPolled(token));
  const fromTick = useTicksStore((s) => s.latest[token]?.polled === true);
  return fromWatch || fromTick;
}
```

`frontend/src/market/MarketDataBridge.tsx`:

```tsx
import { useEffect, useState } from 'react';
import { useBackend } from '../api/BackendContext';
import { setBackend } from '../api/current';
import { useStatusStore } from '../store/status';
import { subscriptions } from './index';

/**
 * Mounted once in App. Registers the backend for the typed market helpers and re-sends every subscription when the
 * feed returns to "live" (FR-WATCH-006 AC2). tick.batch already reaches useTicksStore through P0-4's event bridge.
 */
export function MarketDataBridge(): null {
  const backend = useBackend();
  // Register during the first render: this component renders before the router, so every view's effects see it.
  useState(() => { setBackend(backend); return backend; });
  useEffect(() => { setBackend(backend); }, [backend]);
  useEffect(() => useStatusStore.subscribe((s, prev) => {
    if (s.feed.state === 'live' && prev.feed.state !== 'live') subscriptions.resendAll();
  }), []);
  return null;
}
```

In `frontend/src/app/App.tsx` (P0-4), add `import { MarketDataBridge } from '../market/MarketDataBridge';` and render `<MarketDataBridge />` as the first child of `<BackendProvider backend={backend}>`, before `<ConnectedTheme>` (beside the router, not inside a route).

- [ ] **Step 4: Run the tests to check they pass**

Run: `cd frontend && npx vitest run src/market && npm run typecheck`
Expected: PASS (3 + 8 + 1 tests). Typecheck exits 0.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/market frontend/src/app/App.tsx
git commit -m "$(cat <<'EOF'
feat(market): quote selectors over the tick store, subscription coordinator with 30 s release and polled tracking

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
)"
```

---