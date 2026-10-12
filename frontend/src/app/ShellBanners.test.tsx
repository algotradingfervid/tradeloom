import { act, renderHook, screen } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { renderApp } from '../test/render';
import { LATER_SHELL_BANNERS, useIstDay } from './ShellBanners';

describe('ShellBanners', () => {
  afterEach(() => {
    LATER_SHELL_BANNERS.length = 0;
    vi.useRealTimers();
  });
  it('renders the banners later tasks add to its slot (Task 17 calendar-stale, Task 15 crash notice)', async () => {
    LATER_SHELL_BANNERS.push(function TestBanner() {
      return <div role="status">later banner</div>;
    });
    renderApp({ route: '/home' });
    expect(await screen.findByText('later banner')).toBeInTheDocument();
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
});
