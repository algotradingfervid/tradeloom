import { useEffect, useRef } from 'react';
import { useNavigate } from 'react-router-dom';
import { useBackend } from '../api/BackendContext';
import { calendarCopy } from '../i18n/copy';
import { formatDateLong } from '../i18n/format';
import { useSettingsStore, writeSetting } from '../store/settings';
import { pushToast } from '../store/toasts';
import { isoDayMs } from '../views/settings/CalendarGroup';
import { useIstDay } from './useIstDay';

export const STALE_TOAST_KEY = 'calendar.stale_toast_day';

/**
 * FR-DATA-001 AC6: one WARNING toast per IST day while `Calendar.Info().stale` (no cover for the next 30 days).
 * Rendered in the ShellBanners slot; re-checked whenever the IST day changes (next IST midnight or window focus after
 * a suspend, `useIstDay`, Ruling 100 M6), so an app left open overnight warns again on the new day.
 */
export function CalendarStaleCheck() {
  const backend = useBackend();
  const navigate = useNavigate();
  const loaded = useSettingsStore((s) => s.loaded);
  const today = useIstDay();
  const checkedDay = useRef<string | null>(null);
  useEffect(() => {
    if (!loaded || checkedDay.current === today) return;
    checkedDay.current = today;
    backend.calendar.info().then((info) => {
      if (!info.stale) return;
      if (useSettingsStore.getState().values[STALE_TOAST_KEY] === today) return;
      pushToast({
        level: 'warning',
        message: calendarCopy.staleToast(formatDateLong(isoDayMs(info.coversUntil))),
        action: { label: calendarCopy.open, onSelect: () => void navigate('/settings/calendar') },
      });
      void writeSetting(backend, STALE_TOAST_KEY, today);
    }, () => undefined);
  }, [loaded, today, backend, navigate]);
  return null;
}
