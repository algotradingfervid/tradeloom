import type { Backend } from './backend';

let current: Backend | null = null;

/** Registered once by <MarketDataBridge> (and directly by tests); the typed market helpers read it. */
export function setBackend(b: Backend): void {
  current = b;
}

export function currentBackend(): Backend {
  if (!current) throw new Error('No backend registered: render <MarketDataBridge /> inside <BackendProvider>');
  return current;
}
