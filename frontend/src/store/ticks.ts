import { create } from 'zustand';
import type { TickDTO } from '../api/backend';
import { registerReset } from './reset';
import { useStatusStore } from './status';

interface TicksStore {
  latest: Record<number, TickDTO>;
  applyBatch(ticks: TickDTO[]): void;
}

/**
 * Self-heal (Ruling 92b): on the feed's transition to live the first tick per token is accepted whatever its tsMs,
 * so a future-dated quote (bad clock, stale cache) cannot block a token forever.
 */
let healEpoch = 0;
const healedAt = new Map<number, number>();

/** Go's tick.batch rule (internal/service/events.go): higher tsMs wins; on a tie live beats polled, else the later arrival. */
function replaces(next: TickDTO, prev: TickDTO | undefined): boolean {
  if (prev === undefined) return true;
  if (healEpoch > 0 && healedAt.get(next.token) !== healEpoch) return true;
  if (next.tsMs !== prev.tsMs) return next.tsMs > prev.tsMs;
  return !(next.polled && !prev.polled);
}

export const useTicksStore = create<TicksStore>()((set) => ({
  latest: {},
  applyBatch: (ticks) =>
    set((st) => {
      let latest: Record<number, TickDTO> | null = null;
      for (const t of ticks) {
        const cur = latest ?? st.latest;
        if (!replaces(t, cur[t.token])) continue;
        if (healEpoch > 0) healedAt.set(t.token, healEpoch);
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
}, { onLogout: false });
