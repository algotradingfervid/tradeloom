import { create } from 'zustand';
import type { ShortcutAction } from '../api/backend';
import { registerReset } from '../store/reset';

/** settings.changed key under which Go publishes the full action list after a rebind or reset (service/shortcuts.go). */
export const SHORTCUTS_CHANGE_KEY = 'shortcuts';

interface ShortcutStore {
  actions: ShortcutAction[];
  /** Bumped on every replacement, so a fetch that started earlier can tell it is stale (Ruling 83-F I1). */
  version: number;
  setActions(a: ShortcutAction[]): void;
}

export const useShortcutStore = create<ShortcutStore>()((set) => ({
  actions: [],
  version: 0,
  setActions: (actions) => set((st) => ({ actions, version: st.version + 1 })),
}));

/** The `shortcuts` settings.changed value is the whole list: replace, never merge (null → []). */
export function applyShortcutsEvent(value: unknown): void {
  useShortcutStore.getState().setActions(Array.isArray(value) ? (value as ShortcutAction[]) : []);
}

// Not account data: signing out keeps the list (Ruling 85).
registerReset(() => useShortcutStore.setState({ actions: [], version: 0 }), { onLogout: false });
