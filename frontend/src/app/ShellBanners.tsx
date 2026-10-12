import type { ComponentType } from 'react';
import { useNavigate } from 'react-router-dom';
import { Button } from '../components/Button';
import { Icon } from '../components/Icon';
import { bannerCopy } from '../i18n/copy';
import { useSessionStore } from '../store/session';
import { BootNotice } from './BootNotice';
import { CalendarStaleCheck } from './CalendarStaleCheck';
import { CrashNotice } from './CrashNotice';

/**
 * Slot for banners added by other tasks: Task 17's calendar-stale check (FR-DATA-001 AC6, re-checked per IST day via
 * useIstDay — Ruling 100 M6). Each renders null when it has nothing to say. Kept as a module list so those tasks edit
 * only this file, not ShellLayout.
 */
export const LATER_SHELL_BANNERS: ComponentType[] = [CalendarStaleCheck];

/**
 * Shell-wide notices in the AppShell banner slot, in this order: crash report of the previous run (FR-SHELL-018, Task
 * 15), boot failure, Kite session expiry (FR-ACCT-006), then the slot (calendar-stale check).
 */
export function ShellBanners() {
  const navigate = useNavigate();
  const expired = useSessionStore((s) => s.session?.state === 'expired');
  return (
    <>
      <CrashNotice />
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
