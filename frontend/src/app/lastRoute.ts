import { useEffect } from 'react';
import { useLocation } from 'react-router-dom';
import { useBackend } from '../api/BackendContext';
import { useSettingsStore, writeSetting } from '../store/settings';

// P0 shell views; later phases append their routes here when their views ship.
const REMEMBERABLE: RegExp[] = [
  /^\/home$/, /^\/orders$/, /^\/positions$/, /^\/holdings$/, /^\/profile$/,
  /^\/settings\/[a-z-]+$/, /^\/help(\/[a-z0-9-]+)?$/, /^\/chart-full$/,
];

export function isRememberable(path: string): boolean {
  return REMEMBERABLE.some((r) => r.test(path));
}

export function safeLastRoute(value: unknown): string {
  return typeof value === 'string' && isRememberable(value) ? value : '/home';
}

/** Persists the current view so the app reopens on it (FR-SHELL-019 window state). */
export function useRememberLastRoute(): void {
  const { pathname } = useLocation();
  const backend = useBackend();
  const loaded = useSettingsStore((s) => s.loaded);
  useEffect(() => {
    if (!loaded || !isRememberable(pathname)) return;
    if (useSettingsStore.getState().values['ui.last_route'] === pathname) return;
    void writeSetting(backend, 'ui.last_route', pathname);
  }, [pathname, loaded, backend]);
}
