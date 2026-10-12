import { useEffect, useState } from 'react';
import { useBackend } from '../api/BackendContext';
import { setBackend } from '../api/current';
import { useStatusStore } from '../store/status';
import { subscriptions } from './index';

/**
 * Mounted once in App. Registers the backend for the typed market helpers and re-sends every subscription when the
 * feed returns to "live" (FR-WATCH-006 AC2). tick.batch already reaches useTicksStore through P0-4's event bridge.
 */
export function MarketDataBridge(): null {
  const backend = useBackend();
  // Register during the first render: this component renders before the router, so every view's effects see it.
  useState(() => { setBackend(backend); return backend; });
  useEffect(() => {
    setBackend(backend);
    return () => setBackend(null); // M5: helpers fail loudly instead of using an unmounted backend
  }, [backend]);
  useEffect(() => useStatusStore.subscribe((s, prev) => {
    if (s.feed.state === 'live' && prev.feed.state !== 'live') subscriptions.resendAll();
  }), []);
  return null;
}
