import { useEffect, useLayoutEffect, useRef, useState } from 'react';
import { Outlet, useNavigate } from 'react-router-dom';
import { useBackend } from '../api/BackendContext';
import { bootstrap, startEventBridge } from '../api/events';
import { Button } from '../components/Button';
import { bootCopy } from '../i18n/copy';
import { useAppStore } from '../store/app';
import './shell.css';
import { useTrackHistoryDepth } from '../store/nav';
import { ToastRegion } from '../components/ToastRegion';
import { ShortcutProvider } from '../shortcuts/ShortcutProvider';

export function RootLayout() {
  const backend = useBackend();
  const navigate = useNavigate();
  const navRef = useRef(navigate);
  const [attempt, setAttempt] = useState(0);
  const bootError = useAppStore((s) => s.bootError);
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
      {bootError && (
        <div role="alert" className="boot-notice">
          <span>{bootCopy.failed}</span>
          <span className="boot-notice__reason">{bootError.reason}</span>
          <Button onClick={() => setAttempt((n) => n + 1)}>{bootCopy.retry}</Button>
        </div>
      )}
      <Outlet />
      <ToastRegion />
    </ShortcutProvider>
  );
}
