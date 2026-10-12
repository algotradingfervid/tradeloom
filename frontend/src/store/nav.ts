import { useEffect, useRef } from 'react';
import { useLocation, useNavigationType } from 'react-router-dom';
import { create } from 'zustand';
import { registerReset } from './reset';

interface NavStore { depth: number; setDepth(n: number): void }

export const useNavStore = create<NavStore>()((set) => ({ depth: 0, setDepth: (depth) => set({ depth }) }));

registerReset(() => useNavStore.setState({ depth: 0 }), { inspect: () => useNavStore.getState() });

/** Counts in-app history entries so "Back" can fall back to Home when there is nothing to go back to. */
export function useTrackHistoryDepth(): void {
  const location = useLocation();
  const type = useNavigationType();
  const first = useRef(true);
  useEffect(() => {
    const st = useNavStore.getState();
    if (first.current) {
      first.current = false;
      st.setDepth(1);
      return;
    }
    if (type === 'PUSH') st.setDepth(st.depth + 1);
    // POP is treated as one step back (ListPage's Back calls navigate(-1)). A memory router has no forward button,
    // so a POP that goes forward is not expected; depth never drops below 1 (the entry we started on).
    // REPLACE leaves the depth unchanged.
    else if (type === 'POP') st.setDepth(Math.max(1, st.depth - 1));
  }, [location.key, type]);
}
