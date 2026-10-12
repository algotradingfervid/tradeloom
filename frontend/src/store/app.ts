import { create } from 'zustand';
import type { AppInfo } from '../api/backend';
import { registerReset } from './reset';

/** Bootstrap slices (`api/events.ts` bootstrap): one backend call each. */
export type BootSlice = 'session' | 'status' | 'settings' | 'info' | 'shortcuts';
const SLICE_ORDER: readonly BootSlice[] = ['session', 'status', 'settings', 'info', 'shortcuts'];
/** Last bootstrap's failures (Ruling 83-F I2); null when every slice loaded. */
export interface BootError { failed: BootSlice[]; reason: string }

interface AppStore {
  info: AppInfo | null;
  bootError: BootError | null;
  /** Failure reason per slice; `bootError` is derived from it (first failed slice's reason). */
  bootFailures: Partial<Record<BootSlice, string>>;
  /** Bumped by the boot notice's Retry; RootLayout re-runs bootstrap when it changes. */
  bootAttempt: number;
  setInfo(i: AppInfo): void;
  setBootError(e: BootError | null): void;
  /**
   * Updates only the named slices (a reason = failed, null = loaded), so bootstrap and the shortcut provider
   * (`shortcuts` slice, loaded separately) never clear each other's failures.
   */
  reportBoot(update: Partial<Record<BootSlice, string | null>>): void;
  retryBoot(): void;
}

const initial = { info: null, bootError: null, bootFailures: {}, bootAttempt: 0 };

function deriveBootError(f: Partial<Record<BootSlice, string>>): BootError | null {
  const failed = SLICE_ORDER.filter((s) => f[s] !== undefined);
  return failed.length === 0 ? null : { failed, reason: f[failed[0]] as string };
}

export const useAppStore = create<AppStore>()((set) => ({
  ...initial,
  setInfo: (info) => set({ info }),
  setBootError: (bootError) =>
    set({ bootError, bootFailures: bootError ? Object.fromEntries(bootError.failed.map((s) => [s, bootError.reason])) : {} }),
  reportBoot: (update) =>
    set((st) => {
      const bootFailures = { ...st.bootFailures };
      for (const [slice, reason] of Object.entries(update) as [BootSlice, string | null][]) {
        if (reason === null) delete bootFailures[slice];
        else bootFailures[slice] = reason;
      }
      return { bootFailures, bootError: deriveBootError(bootFailures) };
    }),
  retryBoot: () => set((st) => ({ bootAttempt: st.bootAttempt + 1 })),
}));

registerReset(() => useAppStore.setState(initial), { onLogout: false });
