import { useEffect, useRef } from 'react';
import { useLocation, useNavigationType } from 'react-router-dom';
import { create } from 'zustand';
import { registerReset } from './reset';

interface NavStore { depth: number; setDepth(n: number): void }

export const useNavStore = create<NavStore>()((set) => ({ depth: 0, setDepth: (depth) => set({ depth }) }));

registerReset(() => useNavStore.setState({ depth: 0 }));

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
    else if (type === 'POP') st.setDepth(Math.max(1, st.depth - 1));
  }, [location.key, type]);
}
