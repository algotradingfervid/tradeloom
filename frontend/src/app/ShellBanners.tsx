import { useSyncExternalStore, type ComponentType } from 'react';
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
 * useIstDay — Ruling 100 M6). Each renders null when it has nothing to say. Later tasks list theirs here; tests and
 * runtime additions use registerShellBanner (Ruling 108 M13) instead of mutating the list.
 */
const slot: ComponentType[] = [CalendarStaleCheck];
const listeners = new Set<() => void>();
let version = 0;
const notify = () => {
  version += 1;
  listeners.forEach((l) => l());
};
const subscribe = (l: () => void) => {
  listeners.add(l);
  return () => void listeners.delete(l);
};

/** Adds a banner to the slot; returns the function that removes it again. */
export function registerShellBanner(banner: ComponentType): () => void {
  slot.push(banner);
  notify();
  return () => {
    const i = slot.indexOf(banner);
    if (i >= 0) slot.splice(i, 1);
    notify();
  };
}

/** The banners currently in the slot (a copy). */
export function shellBannerSlot(): ComponentType[] {
  return [...slot];
}

/**
 * Shell-wide notices in the AppShell banner slot, in this order: crash report of the previous run (FR-SHELL-018, Task
 * 15), boot failure, Kite session expiry (FR-ACCT-006), then the slot (calendar-stale check).
 */
export function ShellBanners() {
  const navigate = useNavigate();
  const expired = useSessionStore((s) => s.session?.state === 'expired');
  useSyncExternalStore(subscribe, () => version);
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
      {shellBannerSlot().map((Banner, i) => <Banner key={Banner.displayName ?? Banner.name ?? i} />)}
    </>
  );
}
