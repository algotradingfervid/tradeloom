import { act, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

const shortcuts = vi.hoisted(() => ({ handlers: {} as Record<string, () => void> }));
vi.mock('../../shortcuts/ShortcutProvider', () => ({ useShortcut: (id: string, h: () => void) => { shortcuts.handlers[id] = h; } }));
vi.mock('../../market/hooks', () => ({ useSubscription: vi.fn(), usePolled: () => false }));
vi.mock('../../chart/LightweightChartsAdapter', async () => ({ LightweightChartsAdapter: (await import('../../chart/testing/FakeAdapter')).FakeAdapter }));

import { ChartFullView } from './ChartFullView';
import { setBackend } from '../../api/current';
import { demoSeed, readySession } from '../../api/mock/fixtures';
import { createMockBackend, type MockBackend } from '../../api/mock/index';
import { isShellRoute } from '../../app/routes';
import { safeLastRoute } from '../../app/lastRoute';
import { useSessionStore } from '../../store/session';

let backend: MockBackend;

const renderAt = () => render(
  <MemoryRouter initialEntries={['/home', { pathname: '/chart-full', state: { identifier: 'NSE:ADANIENT', timeframe: '1d' } }]} initialIndex={1}>
    <Routes>
      <Route path="/home" element={<p>Home view</p>} />
      <Route path="/chart-full" element={<ChartFullView />} />
    </Routes>
  </MemoryRouter>,
);

beforeEach(() => {
  shortcuts.handlers = {};
  backend = createMockBackend(demoSeed());
  setBackend(backend);
  useSessionStore.setState({ session: readySession() });
});

afterEach(() => setBackend(null));

describe('ChartFullView', () => {
  it('AC1: opens with the caller instrument and timeframe (ADANIENT NSE, 1 Day)', async () => {
    renderAt();
    expect(await screen.findByText('ADANIENT')).toBeInTheDocument();
    expect(screen.getByText('NSE')).toBeInTheDocument();
    expect((screen.getByLabelText('Timeframe') as HTMLSelectElement).value).toBe('1d');
    expect(screen.getByRole('heading', { level: 1, name: 'Full-view chart' })).toBeInTheDocument();
  });

  it('AC2: the back arrow returns to the originating view', async () => {
    renderAt();
    fireEvent.click(await screen.findByRole('button', { name: 'Back' }));
    expect(await screen.findByText('Home view')).toBeInTheDocument();
  });

  it('AC3: F11 (chart.fullView) in the full view restores the framed view it came from', async () => {
    renderAt();
    await screen.findByText('ADANIENT');
    act(() => shortcuts.handlers['chart.fullView']());
    expect(await screen.findByText('Home view')).toBeInTheDocument();
  });

  it('AC3: the route lives outside the shell (rootChildren), so no frame is rendered around it', async () => {
    const { rootChildren, shellChildren } = await import('../../app/routes');
    expect(rootChildren.some((r) => r.path === 'chart-full')).toBe(true);
    expect(shellChildren.some((r) => r.path === 'chart-full')).toBe(false);
  });

  it('Escape returns to the originating view', async () => {
    renderAt();
    await screen.findByText('ADANIENT');
    fireEvent.keyDown(document.activeElement ?? document.body, { key: 'Escape' });
    expect(await screen.findByText('Home view')).toBeInTheDocument();
  });

  it('Escape inside the symbol search dialog only closes the dialog', async () => {
    renderAt();
    fireEvent.click(await screen.findByRole('button', { name: 'Change symbol' }));
    const dialog = await screen.findByRole('dialog');
    fireEvent.keyDown(dialog, { key: 'Escape' });
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull());
    expect(screen.getByRole('heading', { level: 1, name: 'Full-view chart' })).toBeInTheDocument();
    expect(screen.queryByText('Home view')).toBeNull();
  });

  it('moves focus into the full view on entry (the chart canvas)', async () => {
    renderAt();
    await screen.findByText('ADANIENT');
    expect(document.activeElement).toBe(screen.getByRole('region', { name: /Price chart/ }));
  });

  it('without caller state (restored last view) opens NIFTY 50 at 1 Hour and Back goes Home', async () => {
    render(
      <MemoryRouter initialEntries={['/chart-full']}>
        <Routes>
          <Route path="/home" element={<p>Home view</p>} />
          <Route path="/chart-full" element={<ChartFullView />} />
        </Routes>
      </MemoryRouter>,
    );
    expect(await screen.findByText('NIFTY 50')).toBeInTheDocument();
    expect((screen.getByLabelText('Timeframe') as HTMLSelectElement).value).toBe('1h');
    fireEvent.click(screen.getByRole('button', { name: 'Back' }));
    expect(await screen.findByText('Home view')).toBeInTheDocument();
  });

  it('G-0: no order actions in the full view', async () => {
    renderAt();
    await screen.findByText('ADANIENT');
    expect(screen.queryByRole('button', { name: /buy|sell|order/i })).toBeNull();
  });

  it('the last-view restore accepts /chart-full (isShellRoute)', () => {
    expect(isShellRoute('/chart-full')).toBe(true);
    expect(safeLastRoute('/chart-full', isShellRoute)).toBe('/chart-full');
  });

  it('unmount leaves no key listener, live subscription or chart event listener behind', async () => {
    const add = vi.spyOn(window, 'addEventListener');
    const remove = vi.spyOn(window, 'removeEventListener');
    const { unmount } = renderAt();
    await waitFor(() => expect(backend.state.openLiveIds).toHaveLength(1));
    const keydowns = add.mock.calls.filter(([t]) => t === 'keydown').map(([, fn]) => fn);
    expect(keydowns.length).toBeGreaterThan(0);
    unmount();
    const removed = remove.mock.calls.filter(([t]) => t === 'keydown').map(([, fn]) => fn);
    for (const fn of keydowns) expect(removed).toContain(fn);
    await waitFor(() => expect(backend.state.openLiveIds).toEqual([]));
    for (const n of ['candle.forming', 'candle.closed', 'backfill.progress'] as const) expect(backend.listenerCount(n)).toBe(0);
    add.mockRestore();
    remove.mockRestore();
  });
});
