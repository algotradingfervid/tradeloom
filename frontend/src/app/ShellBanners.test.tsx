import { act, renderHook, screen, waitFor } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { demoSeed } from '../api/mock/fixtures';
import { createMockBackend } from '../api/mock/index';
import { istDayKey } from '../i18n/format';
import { useToastStore } from '../store/toasts';
import { nestedLiveRegions } from '../test/liveRegions';
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
      return <div>later banner</div>;
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
  it('minor 4: banners whose function names collide (minified builds) all render without key warnings', async () => {
    const make = (text: string) => {
      const C = () => <div>{text}</div>;
      Object.defineProperty(C, 'name', { value: 'a' });
      return C;
    };
    const errors = vi.spyOn(console, 'error');
    const offs = [registerShellBanner(make('first banner')), registerShellBanner(make('second banner'))];
    try {
      renderApp({ route: '/home' });
      expect(await screen.findByText('first banner')).toBeInTheDocument();
      expect(screen.getByText('second banner')).toBeInTheDocument();
      expect(errors.mock.calls.flat().join(' ')).not.toMatch(/same key/i);
    } finally {
      offs.forEach((off) => off());
    }
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

  describe('Ruling 86 M5: banners that mount with their text are announced through always-mounted live regions', () => {
    it('the expiry banner appears inside a polite region that existed before it and has no live role of its own', async () => {
      const backend = createMockBackend(demoSeed());
      const { container } = renderApp({ route: '/holdings', backend });
      await screen.findByRole('heading', { level: 1, name: 'Holdings' });
      const regions = [...container.querySelectorAll('#main-content [aria-live="polite"]')];
      expect(regions.length).toBeGreaterThan(0);
      act(() => backend.emit('session.state', { ...backend.state.session, state: 'expired' }));
      const banner = (await screen.findByText('Kite session expired — log in to continue')).closest('.banner') as HTMLElement;
      expect(banner).not.toHaveAttribute('role');
      expect(regions).toContain(banner.closest('[aria-live]'));
      expect(nestedLiveRegions(container)).toEqual([]);
    });
    it('the crash notice appears inside a polite region that existed before it; the boot alert is not nested', async () => {
      const backend = createMockBackend(demoSeed());
      let resolveCrash: (c: { path: string; atMs: number }) => void = () => undefined;
      vi.spyOn(backend.diagnostics, 'lastCrash').mockReturnValue(new Promise((r) => { resolveCrash = r; }));
      backend.failNext('system.info', 'info unavailable');
      const { container } = renderApp({ route: '/home', backend });
      await screen.findByText('Some app data could not be loaded');
      const regions = [...container.querySelectorAll('#main-content [aria-live="polite"]')];
      await act(async () => resolveCrash({ path: '/tmp/crash-1.txt', atMs: Date.UTC(2026, 9, 11, 10, 0) }));
      const banner = (await screen.findByText(/crash/i, { selector: '.banner--error span' })).closest('.banner') as HTMLElement;
      expect(banner).not.toHaveAttribute('role');
      expect(regions).toContain(banner.closest('[aria-live]'));
      expect(screen.getByRole('alert')).toHaveTextContent('Some app data could not be loaded');
      expect(nestedLiveRegions(container)).toEqual([]);
    });
  });
});
