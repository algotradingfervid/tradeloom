import { create } from 'zustand';
import type { TickDTO } from '../api/backend';
import { registerReset } from './reset';
import { useStatusStore } from './status';

export interface ApplyOptions {
  /**
   * false for writes that are not real ticks (Quotes.Watch snapshots): they neither bypass the ordering rule
   * through the self-heal nor use up a token's heal, so the first real tick after →live still heals it.
   */
  heal?: boolean;
}

interface TicksStore {
  latest: Record<number, TickDTO>;
  applyBatch(ticks: TickDTO[], opts?: ApplyOptions): void;
}

/**
 * Self-heal (Ruling 92b): on the feed's transition to live the first tick per token is accepted whatever its tsMs,
 * so a future-dated quote (bad clock, stale cache) cannot block a token forever.
 */
let healEpoch = 0;
const healedAt = new Map<number, number>();

/**
 * Higher tsMs wins, as in Go's tick.batch coalescing (internal/service/events.go). On a tie Go takes the later
 * arrival; Ruling 92b adds that a live tick beats a polled one, so only same-flag ties go to the later arrival.
 */
function replaces(next: TickDTO, prev: TickDTO | undefined, heal: boolean): boolean {
  if (prev === undefined) return true;
  if (heal && healEpoch > 0 && healedAt.get(next.token) !== healEpoch) return true;
  if (next.tsMs !== prev.tsMs) return next.tsMs > prev.tsMs;
  return !(next.polled && !prev.polled);
}

export const useTicksStore = create<TicksStore>()((set) => ({
  latest: {},
  applyBatch: (ticks, opts = {}) =>
    set((st) => {
      const heal = opts.heal ?? true;
      let latest: Record<number, TickDTO> | null = null;
      for (const t of ticks) {
        const cur = latest ?? st.latest;
        if (!replaces(t, cur[t.token], heal)) continue;
        if (heal && healEpoch > 0) healedAt.set(t.token, healEpoch);
        latest ??= { ...st.latest };
        latest[t.token] = t;
      }
      // A batch with nothing accepted returns the same state: zustand notifies nobody.
      return latest === null ? st : { latest };
    }),
}));

useStatusStore.subscribe((s, prev) => {
  if (s.feed.state === 'live' && prev.feed.state !== 'live') healEpoch += 1;
});

/** Latest LTP (rupees) for one token; re-renders only when that token's tick changes. */
export function useLtp(token: number | undefined): number | undefined {
  return useTicksStore((s) => (token === undefined ? undefined : s.latest[token]?.ltp));
}

registerReset(() => {
  healEpoch = 0;
  healedAt.clear();
  useTicksStore.setState({ latest: {} });
}, { onLogout: false, inspect: () => ({ ...useTicksStore.getState(), healedAt }) });
