import { create } from 'zustand';
import type { TickDTO } from '../api/backend';
import { registerReset } from './reset';

interface TicksStore {
  latest: Record<number, TickDTO>;
  applyBatch(ticks: TickDTO[]): void;
}

export const useTicksStore = create<TicksStore>()((set) => ({
  latest: {},
  applyBatch: (ticks) =>
    set((st) => {
      if (ticks.length === 0) return st;
      const latest = { ...st.latest };
      for (const t of ticks) latest[t.token] = t;
      return { latest };
    }),
}));

/** Latest LTP (rupees) for one token; re-renders only when that token's tick changes. */
export function useLtp(token: number | undefined): number | undefined {
  return useTicksStore((s) => (token === undefined ? undefined : s.latest[token]?.ltp));
}

registerReset(() => useTicksStore.setState({ latest: {} }));
