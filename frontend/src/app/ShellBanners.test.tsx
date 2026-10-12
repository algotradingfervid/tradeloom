import { act, renderHook, screen, waitFor } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { demoSeed } from '../api/mock/fixtures';
import { createMockBackend } from '../api/mock/index';
import { istDayKey } from '../i18n/format';
import { useToastStore } from '../store/toasts';
import { renderApp } from '../test/render';
import { CalendarStaleCheck } from './CalendarStaleCheck';
import { LATER_SHELL_BANNERS } from './ShellBanners';
import { useIstDay } from './useIstDay';

const DAY_MS = 86_400_000;
const staleToasts = () => useToastStore.getState().visible.filter((t) => t.message.startsWith('Calendar may be stale — it ends'));

describe('ShellBanners', () => {
  afterEach(() => {
    vi.useRealTimers();
    vi.restoreAllMocks();
  });
  it('the slot holds Task 17\'s calendar-stale check', () => {
    expect(LATER_SHELL_BANNERS).toContain(CalendarStaleCheck);
  });
  it('renders the banners later tasks add to its slot (Task 15 crash notice)', async () => {
    function TestBanner() {
      return <div role="status">later banner</div>;
    }
    LATER_SHELL_BANNERS.push(TestBanner);
    try {
      renderApp({ route: '/home' });
      expect(await screen.findByText('later banner')).toBeInTheDocument();
    } finally {
      LATER_SHELL_BANNERS.splice(LATER_SHELL_BANNERS.indexOf(TestBanner), 1);
    }
  });
  it('useIstDay re-checks at the next IST midnight and on window focus (Ruling 100 M6)', () => {
    vi.useFakeTimers();
    // 12 Oct 2026 23:59:00 IST = 18:29:00 UTC
    vi.setSystemTime(Date.UTC(2026, 9, 12, 18, 29, 0));
    const { result } = renderHook(() => useIstDay());
    expect(result.current).toBe('2026-10-12');
    act(() => {
      vi.advanceTimersByTime(61_000);
    });
    expect(result.current).toBe('2026-10-13');
    // A suspended machine: timers did not fire, the clock jumped two days; focus brings the day up to date.
    vi.setSystemTime(Date.UTC(2026, 9, 15, 6, 0, 0));
    act(() => {
      window.dispatchEvent(new Event('focus'));
    });
    expect(result.current).toBe('2026-10-15');
  });
  it('FR-DATA-001 AC6 + Ruling 100 M6: the stale-calendar check runs again on a new IST day (window focus) and warns once more', async () => {
    const seed = demoSeed();
    seed.calendar = { ...seed.calendar, stale: true, coversUntil: '2026-10-31' };
    const backend = createMockBackend(seed);
    renderApp({ route: '/home', backend });
    const day1 = istDayKey(Date.now() + 120); // demo clock offset is 120 ms
    await waitFor(() => expect(backend.state.settings['calendar.stale_toast_day']).toBe(JSON.stringify(day1)));
    expect(staleToasts()).toHaveLength(1);
    const calls = backend.callsTo('calendar.info').length;

    // Same day: focus does not re-check.
    act(() => {
      window.dispatchEvent(new Event('focus'));
    });
    expect(backend.callsTo('calendar.info')).toHaveLength(calls);

    // The machine wakes on the next IST day.
    const realNow = Date.now.bind(Date);
    vi.spyOn(Date, 'now').mockImplementation(() => realNow() + DAY_MS);
    const day2 = istDayKey(Date.now() + 120);
    act(() => {
      window.dispatchEvent(new Event('focus'));
    });
    await waitFor(() => expect(backend.state.settings['calendar.stale_toast_day']).toBe(JSON.stringify(day2)));
    expect(backend.callsTo('calendar.info').length).toBe(calls + 1);
    expect(staleToasts()).toHaveLength(2);
  });
});
