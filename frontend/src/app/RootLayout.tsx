import { useEffect, useLayoutEffect, useRef } from 'react';
import { Outlet, useNavigate } from 'react-router-dom';
import { useBackend } from '../api/BackendContext';
import { bootstrap, startEventBridge } from '../api/events';
import { useAppStore } from '../store/app';
import './shell.css';
import { useTrackHistoryDepth } from '../store/nav';
import { ToastRegion } from '../components/ToastRegion';
import { ShortcutProvider } from '../shortcuts/ShortcutProvider';

export function RootLayout() {
  const backend = useBackend();
  const navigate = useNavigate();
  const navRef = useRef(navigate);
  const attempt = useAppStore((s) => s.bootAttempt);
  useLayoutEffect(() => {
    navRef.current = navigate;
  });
  // The bridge subscribes before bootstrap fetches, so an event that lands mid-fetch wins (Ruling 83-F I1).
  useEffect(() => startEventBridge(backend, { navigate: (route) => void navRef.current(route) }), [backend]);
  useEffect(() => {
    const ctl = new AbortController();
    void bootstrap(backend, { signal: ctl.signal });
    return () => ctl.abort();
  }, [backend, attempt]);
  useTrackHistoryDepth();
  return (
    <ShortcutProvider>
      {/* The boot-failure notice renders in the AppShell banner slot (BootNotice, ShellLayout). */}
      <Outlet />
      <ToastRegion />
    </ShortcutProvider>
  );
}
