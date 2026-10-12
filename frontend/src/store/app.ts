import { create } from 'zustand';
import type { AppInfo } from '../api/backend';
import { registerReset } from './reset';

/** Bootstrap slices (`api/events.ts` bootstrap): one backend call each. */
export type BootSlice = 'session' | 'status' | 'settings' | 'info';
/** Last bootstrap's failures (Ruling 83-F I2); null when every slice loaded. */
export interface BootError { failed: BootSlice[]; reason: string }

interface AppStore {
  info: AppInfo | null;
  bootError: BootError | null;
  /** Bumped by the boot notice's Retry; RootLayout re-runs bootstrap when it changes. */
  bootAttempt: number;
  setInfo(i: AppInfo): void;
  setBootError(e: BootError | null): void;
  retryBoot(): void;
}

const initial = { info: null, bootError: null, bootAttempt: 0 };

export const useAppStore = create<AppStore>()((set) => ({
  ...initial,
  setInfo: (info) => set({ info }),
  setBootError: (bootError) => set({ bootError }),
  retryBoot: () => set((st) => ({ bootAttempt: st.bootAttempt + 1 })),
}));

registerReset(() => useAppStore.setState(initial), { onLogout: false });
