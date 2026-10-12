import { useCallback } from 'react';
import { create } from 'zustand';
import type { Backend, SettingValue } from '../api/backend';
import { useBackend } from '../api/BackendContext';
import { parseBackendError, type BackendError } from '../api/errors';
import { settingsCopy } from '../i18n/copy';
import { registerReset } from './reset';

interface SettingsStore {
  /** Decoded values by key (Go sends JSON in `SettingValue.valueJson`). */
  values: Record<string, unknown>;
  loaded: boolean;
  writesSuspended: boolean;
  hydrate(all: SettingValue[]): void;
  put(key: string, value: unknown): void;
  remove(key: string): void;
  setSuspended(v: boolean): void;
}

function parse(raw: string): unknown {
  try {
    return JSON.parse(raw);
  } catch {
    return raw;
  }
}

const initial = { values: {} as Record<string, unknown>, loaded: false, writesSuspended: false };

export const useSettingsStore = create<SettingsStore>()((set) => ({
  ...initial,
  hydrate: (all) => set({ values: Object.fromEntries(all.map((v) => [v.name, parse(v.valueJson)])), loaded: true }),
  put: (key, value) => set((st) => ({ values: { ...st.values, [key]: value } })),
  remove: (key) =>
    set((st) => {
      const values = { ...st.values };
      delete values[key];
      return { values };
    }),
  setSuspended: (writesSuspended) => set({ writesSuspended }),
}));

registerReset(() => useSettingsStore.setState(initial));

/** Optimistic write; on refusal the previous value is restored and the reason returned (inline error). */
export async function writeSetting(backend: Backend, key: string, value: unknown): Promise<BackendError | null> {
  const st = useSettingsStore.getState();
  if (st.writesSuspended) return { code: 'suspended', stage: 'settings', reason: settingsCopy.restoreInProgress };
  const had = key in st.values;
  const previous = st.values[key];
  st.put(key, value);
  try {
    await backend.settings.set(key, JSON.stringify(value));
    return null;
  } catch (e) {
    const now = useSettingsStore.getState();
    if (had) now.put(key, previous);
    else now.remove(key);
    return parseBackendError(e);
  }
}

export async function reloadSettings(backend: Backend): Promise<void> {
  useSettingsStore.getState().hydrate(await backend.settings.all());
}

export const suspendSettingsWrites = (): void => useSettingsStore.getState().setSuspended(true);
export const resumeSettingsWrites = (): void => useSettingsStore.getState().setSuspended(false);

/** `fallback` must be a primitive (it is returned from a store selector). */
export function useSetting<T extends string | number | boolean>(key: string, fallback: T): [T, (v: T) => Promise<BackendError | null>] {
  const backend = useBackend();
  const value = useSettingsStore((s) => (key in s.values ? (s.values[key] as T) : fallback));
  const set = useCallback((v: T) => writeSetting(backend, key, v), [backend, key]);
  return [value, set];
}
