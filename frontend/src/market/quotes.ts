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

// Go persists the last quote per token (QuoteCacheKey) and serves it in Watch snapshots after a restart, so a
// snapshot quote can come from a previous run. Ticks of this run are stamped at or after the moment it started.
let startedMs = Date.now();

/** When this app run started (Unix ms); quotes stamped before it come from a previous run. */
export function runStartedMs(): number {
  return startedMs;
}

/** Overrides the run start (tests and the mock backend's fake clock, Ruling 95 I4). */
export function setRunStartedMs(ms: number): void {
  startedMs = ms;
}

/** A quote from a previous run (persisted snapshot): stale until a fresh tick for its token arrives. */
export function isStaleQuote(q: TickDTO | undefined): boolean {
  return q !== undefined && q.tsMs < startedMs;
}

/** Whether the token's latest quote is stale; re-renders only when that answer changes. */
export function useQuoteStale(token: number | undefined): boolean {
  return useTicksStore((s) => token !== undefined && isStaleQuote(s.latest[token]));
}
