import { Outlet } from 'react-router-dom';
import { useSessionStore } from '../store/session';
import { gateTarget } from './gate';
import { ShellLayout } from './ShellLayout';

/**
 * Layout for Help and the legal pages (Ruling 117 I1). One route serves both cases, because two same-path routes in
 * different branches cannot be chosen per session. A user who could open Home gets the full shell (sidebar, top bar,
 * global shortcuts); before setup, while logged out or before the risk acknowledgement the page stands alone,
 * without the shell chrome (Ruling 108 M6), so the risk screen's links work and no account data shows.
 */
export function DocsFrame() {
  const session = useSessionStore((s) => s.session);
  const inShell = session !== null && gateTarget(session, '/home') === null;
  if (inShell) return <ShellLayout />;
  return (
    <main className="standalone-page">
      <Outlet />
    </main>
  );
}
