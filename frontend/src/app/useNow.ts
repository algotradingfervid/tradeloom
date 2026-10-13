import { useEffect, useState } from 'react';
import { useStatusStore } from '../store/status';

/** App-clock "now" (system time + measured exchange offset `clock.offset.millis`, FR-DATA-004), refreshed every `intervalMs`. */
export function useNow(intervalMs = 30_000): number {
  const offset = useStatusStore((s) => s.clock?.millis ?? 0);
  const [now, setNow] = useState(() => Date.now());
  useEffect(() => {
    const id = window.setInterval(() => setNow(Date.now()), intervalMs);
    return () => window.clearInterval(id);
  }, [intervalMs]);
  return now + offset;
}
