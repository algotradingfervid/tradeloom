import { Navigate } from 'react-router-dom';
import { gateCopy } from '../i18n/copy';
import { useAppStore } from '../store/app';
import { useSetting, useSettingsStore } from '../store/settings';
import { safeLastRoute } from './lastRoute';

export interface RestoreLastRouteProps {
  /** Whether this build has a view at `path` (Ruling 94 M4); `routes.tsx` passes its shell-route matcher. */
  exists?: (path: string) => boolean;
}

/** Index route: reopens the last view (FR-SHELL-019 window state) once settings are loaded. */
export function RestoreLastRoute({ exists }: RestoreLastRouteProps) {
  const loaded = useSettingsStore((s) => s.loaded);
  // Settings that failed to load never become `loaded`; open Home, where the shell shows the boot notice and Retry.
  const settingsFailed = useAppStore((s) => s.bootError?.failed.includes('settings') ?? false);
  const [last] = useSetting<string>('ui.last_route', '/home');
  if (!loaded && !settingsFailed) return <div className="boot" role="status">{gateCopy.starting}</div>;
  return <Navigate to={safeLastRoute(loaded ? last : '/home', exists)} replace />;
}
