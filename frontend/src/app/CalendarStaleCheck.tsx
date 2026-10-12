import { useEffect, useRef } from 'react';
import { useNavigate } from 'react-router-dom';
import { useBackend } from '../api/BackendContext';
import { calendarCopy } from '../i18n/copy';
import { formatDateLong, istDayKey } from '../i18n/format';
import { useSettingsStore, writeSetting } from '../store/settings';
import { pushToast } from '../store/toasts';
import { isoDayMs } from '../views/settings/CalendarGroup';

export const STALE_TOAST_KEY = 'calendar.stale_toast_day';

/** FR-DATA-001 AC6: one WARNING toast per IST day while `Calendar.Info().stale` (no cover for the next 30 days). */
export function CalendarStaleCheck() {
  const backend = useBackend();
  const navigate = useNavigate();
  const loaded = useSettingsStore((s) => s.loaded);
  const checked = useRef(false);
  useEffect(() => {
    if (!loaded || checked.current) return;
    checked.current = true;
    backend.calendar.info().then((info) => {
      if (!info.stale) return;
      const today = istDayKey(Date.now());
      if (useSettingsStore.getState().values[STALE_TOAST_KEY] === today) return;
      pushToast({
        level: 'warning',
        message: calendarCopy.staleToast(formatDateLong(isoDayMs(info.coversUntil))),
        action: { label: calendarCopy.open, onSelect: () => void navigate('/settings/calendar') },
      });
      void writeSetting(backend, STALE_TOAST_KEY, today);
    }, () => undefined);
  }, [loaded, backend, navigate]);
  return null;
}
