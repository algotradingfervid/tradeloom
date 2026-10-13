import { create } from 'zustand';
import type { ClockOffset, FeedState, MarketStatus, StatusSnapshot } from '../api/backend';
import { registerReset } from './reset';

interface StatusStore {
  feed: FeedState;
  market: MarketStatus | null;
  clock: ClockOffset | null;
  setFeed(f: FeedState): void;
  setMarket(m: MarketStatus): void;
  setClock(c: ClockOffset): void;
  setSnapshot(s: StatusSnapshot): void;
}

const initial = {
  feed: { state: 'stopped', attempt: 0, nextRetryMs: 0, lastTickMs: 0, connections: 0, tokens: 0 } as FeedState,
  market: null,
  clock: null,
};

const sameFeed = (a: FeedState, b: FeedState) =>
  a.state === b.state && a.attempt === b.attempt && a.nextRetryMs === b.nextRetryMs && a.lastTickMs === b.lastTickMs
  && a.connections === b.connections && a.tokens === b.tokens;

const sameMarket = (a: MarketStatus | null, b: MarketStatus) =>
  a !== null && a.phase === b.phase && a.dayType === b.dayType && a.label === b.label && a.tooltip === b.tooltip;

export const useStatusStore = create<StatusStore>()((set) => ({
  ...initial,
  // Returning the same state object skips the update, so a storm of identical events re-renders nothing.
  setFeed: (feed) => set((st) => (sameFeed(st.feed, feed) ? st : { feed })),
  setMarket: (market) => set((st) => (sameMarket(st.market, market) ? st : { market })),
  setClock: (clock) => set({ clock }),
  setSnapshot: (s) => set({ feed: s.feed, market: s.market, clock: s.clock }),
}));

registerReset(() => useStatusStore.setState(initial), { onLogout: false, name: 'status', inspect: () => useStatusStore.getState() });
