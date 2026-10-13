import { GlobalShortcuts } from '../shortcuts/GlobalShortcuts';
import { useRememberLastRoute } from './lastRoute';

/**
 * The shell's non-visual services: remembers the current view (FR-SHELL-019) and registers the app-wide shortcuts
 * (F1, Alt+1…8, Alt+T; later the kill switch). Rendered by ShellLayout and by the full-view chart, which sits outside
 * the shell frame, so the two cannot drift apart (Ruling 119 I1).
 */
export function ShellServices() {
  useRememberLastRoute();
  return <GlobalShortcuts />;
}
