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
  setInfo(i: AppInfo): void;
  setBootError(e: BootError | null): void;
}

const initial = { info: null, bootError: null };

export const useAppStore = create<AppStore>()((set) => ({
  ...initial,
  setInfo: (info) => set({ info }),
  setBootError: (bootError) => set({ bootError }),
}));

registerReset(() => useAppStore.setState(initial), { onLogout: false });
