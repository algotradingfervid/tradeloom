import { act, renderHook, screen, waitFor } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { demoSeed } from '../api/mock/fixtures';
import { createMockBackend } from '../api/mock/index';
import { istDayKey } from '../i18n/format';
import { useToastStore } from '../store/toasts';
import { renderApp } from '../test/render';
import { CalendarStaleCheck } from './CalendarStaleCheck';
import { registerShellBanner, shellBannerSlot } from './ShellBanners';
import { useIstDay } from './useIstDay';

const DAY_MS = 86_400_000;
const staleToasts = () => useToastStore.getState().visible.filter((t) => t.message.startsWith('Calendar may be stale — it ends'));

describe('ShellBanners', () => {
  afterEach(() => {
    vi.useRealTimers();
    vi.restoreAllMocks();
  });
  it('the slot holds Task 17\'s calendar-stale check', () => {
    expect(shellBannerSlot()).toContain(CalendarStaleCheck);
  });
  it('M13: renders banners registered in its slot and forgets them when unregistered', async () => {
    function TestBanner() {
      return <div role="status">later banner</div>;
    }
    const unregister = registerShellBanner(TestBanner);
    try {
      renderApp({ route: '/home' });
      expect(await screen.findByText('later banner')).toBeInTheDocument();
    } finally {
      unregister();
    }
    expect(shellBannerSlot()).not.toContain(TestBanner);
  });
  it('order: crash notice (banner--error) first, then the boot notice, then the slot', async () => {
    const backend = createMockBackend(demoSeed());
    backend.state.crash = { path: '/tmp/crash-1.txt', atMs: Date.UTC(2026, 9, 11, 10, 0) };
    backend.failNext('system.info', 'info unavailable');
    const { container } = renderApp({ route: '/home', backend });
    await screen.findByText('Some app data could not be loaded');
    const banners = container.querySelector('#main-content');
    const crash = await screen.findByText(/crash/i, { selector: '.banner--error span' });
    const boot = screen.getByText('Some app data could not be loaded');
    expect(crash.closest('.banner')).toHaveClass('banner', 'banner--error');
    expect(banners?.querySelector('.crash-notice')).toBeNull();
    expect(crash.compareDocumentPosition(boot) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
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
