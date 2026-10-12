import { useEffect, useState, type ComponentType } from 'react';
import { useNavigate } from 'react-router-dom';
import { Button } from '../components/Button';
import { Icon } from '../components/Icon';
import { bannerCopy } from '../i18n/copy';
import { istDayKey } from '../i18n/format';
import { useSessionStore } from '../store/session';
import { useStatusStore } from '../store/status';
import { BootNotice } from './BootNotice';

/**
 * Slot for banners later tasks add (Task 17: calendar-stale warning; Task 15: crash notice). Each renders null when it
 * has nothing to say. Kept as a module list so those tasks edit only this file, not ShellLayout.
 */
export const LATER_SHELL_BANNERS: ComponentType[] = [];

const DAY_MS = 86_400_000;
const IST_OFFSET_MS = 19_800_000;

function msUntilNextIstMidnight(now: number): number {
  const next = (Math.floor((now + IST_OFFSET_MS) / DAY_MS) + 1) * DAY_MS - IST_OFFSET_MS;
  return next - now;
}

/**
 * The IST calendar day on the app clock (system time + exchange offset, FR-DATA-004). It re-checks at the next IST
 * midnight and whenever the window regains focus (a suspended machine misses timers) — Ruling 100 M6, for the
 * calendar-stale check that moves here with Task 17.
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

/** Shell-wide notices in the AppShell banner slot: boot failure, Kite session expiry (FR-ACCT-006), later banners. */
export function ShellBanners() {
  const navigate = useNavigate();
  const expired = useSessionStore((s) => s.session?.state === 'expired');
  return (
    <>
      <BootNotice />
      {expired && (
        <div className="banner banner--warn" role="status">
          <Icon name="warning" />
          <span>{bannerCopy.expired}</span>
          <Button variant="primary" onClick={() => void navigate('/login')}>{bannerCopy.logIn}</Button>
        </div>
      )}
      {LATER_SHELL_BANNERS.map((Banner, i) => <Banner key={Banner.displayName ?? Banner.name ?? i} />)}
    </>
  );
}
