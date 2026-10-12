import { useEffect, useLayoutEffect, useRef } from 'react';
import { Outlet, useNavigate } from 'react-router-dom';
import { useBackend } from '../api/BackendContext';
import { bootstrap, startEventBridge } from '../api/events';
import { ToastRegion } from '../components/ToastRegion';

export function RootLayout() {
  const backend = useBackend();
  const navigate = useNavigate();
  const navRef = useRef(navigate);
  useLayoutEffect(() => {
    navRef.current = navigate;
  });
  useEffect(() => {
    const stop = startEventBridge(backend, { navigate: (route) => void navRef.current(route) });
    void bootstrap(backend);
    return stop;
  }, [backend]);
  return (
    <>
      <Outlet />
      <ToastRegion />
    </>
  );
}
