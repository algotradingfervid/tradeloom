import { useEffect, useState } from 'react';
import { istDayKey } from '../i18n/format';
import { useStatusStore } from '../store/status';

const DAY_MS = 86_400_000;
const IST_OFFSET_MS = 19_800_000;

function msUntilNextIstMidnight(now: number): number {
  const next = (Math.floor((now + IST_OFFSET_MS) / DAY_MS) + 1) * DAY_MS - IST_OFFSET_MS;
  return next - now;
}

/**
 * The IST calendar day on the app clock (system time + exchange offset, FR-DATA-004). It re-checks at the next IST
 * midnight and whenever the window regains focus (a suspended machine misses timers) — Ruling 100 M6, used by
 * the calendar-stale check (CalendarStaleCheck in the ShellBanners slot).
 */
export function useIstDay(): string {
  const offset = useStatusStore((s) => s.clock?.millis ?? 0);
  const [day, setDay] = useState(() => istDayKey(Date.now() + offset));
  useEffect(() => {
    let timer: number | undefined;
    const check = () => {
      const now = Date.now() + offset;
      setDay(istDayKey(now));
      window.clearTimeout(timer);
      timer = window.setTimeout(check, msUntilNextIstMidnight(now) + 1000);
    };
    check();
    window.addEventListener('focus', check);
    return () => {
      window.clearTimeout(timer);
      window.removeEventListener('focus', check);
    };
  }, [offset]);
  return day;
}
