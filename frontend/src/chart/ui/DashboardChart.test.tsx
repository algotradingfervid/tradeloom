import { act, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { MemoryRouter, Route, Routes, useLocation } from 'react-router-dom';
import { beforeEach, describe, expect, it, vi } from 'vitest';

vi.mock('../../market/hooks', () => ({ useSubscription: vi.fn(), usePolled: () => false }));
vi.mock('../LightweightChartsAdapter', async () => ({ LightweightChartsAdapter: (await import('../testing/FakeAdapter')).FakeAdapter }));
// DashboardChart calls useShortcut. Only the hook is replaced: renderApp below still needs the real ShortcutProvider.
const shortcuts = vi.hoisted(() => ({ handlers: {} as Record<string, () => void> }));
vi.mock('../../shortcuts/ShortcutProvider', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../../shortcuts/ShortcutProvider')>()),
  useShortcut: (id: string, h: () => void) => { shortcuts.handlers[id] = h; },
}));

import { DashboardChart } from './DashboardChart';
import { BackendProvider } from '../../api/BackendContext';
import { setBackend } from '../../api/current';
import { demoSeed, readySession } from '../../api/mock/fixtures';
import { createMockBackend, type MockBackend } from '../../api/mock/index';
import { __resetChartContexts } from '../../store/chartContextsStore';
import { useSessionStore } from '../../store/session';
import { useSettingsStore } from '../../store/settings';
import { renderApp } from '../../test/render';
import { ChartFullView } from '../../views/chart-full/ChartFullView';

let backend: MockBackend;
const renderDash = () => render(<BackendProvider backend={backend}><MemoryRouter><DashboardChart /></MemoryRouter></BackendProvider>);

beforeEach(() => {
  shortcuts.handlers = {};
  __resetChartContexts();
  backend = createMockBackend(demoSeed());
  setBackend(backend);
  useSessionStore.setState({ session: readySession() });
  useSettingsStore.getState().hydrate([]);
});

describe('DashboardChart', () => {
  it('FR-HOME-001 AC1 / FR-CHART-004 default: NIFTY 50 at 1 Hour on first run', async () => {
    renderDash();
    expect(await screen.findByText('NIFTY 50')).toBeInTheDocument();
    expect((screen.getByLabelText('Timeframe') as HTMLSelectElement).value).toBe('1h');
  });

  it('honours a changed Settings → Chart default timeframe (chart.default_timeframe) on first run', async () => {
    useSettingsStore.getState().hydrate([{ name: 'chart.default_timeframe', valueJson: '"day"' }]);
    renderDash();
    await waitFor(() => expect((screen.getByLabelText('Timeframe') as HTMLSelectElement).value).toBe('1d'));
  });

  it('FR-CHART-016 AC1: restores RELIANCE at 15 Minutes from the chart.dashboard UI state', async () => {
    await backend.settings.setUiState('chart.dashboard', JSON.stringify({ identifier: 'NSE:RELIANCE', timeframe: '15m' }));
    renderDash();
    expect(await screen.findByText('RELIANCE')).toBeInTheDocument();
    expect((screen.getByLabelText('Timeframe') as HTMLSelectElement).value).toBe('15m');
  });

  const Probe = () => { const l = useLocation(); return <p>at {l.pathname} {JSON.stringify(l.state)}</p>; };
  const renderWithFullRoute = () => render(
    <BackendProvider backend={backend}>
      <MemoryRouter><Routes><Route path="/" element={<DashboardChart />} /><Route path="/chart-full" element={<Probe />} /></Routes></MemoryRouter>
    </BackendProvider>,
  );

  it('FR-CHART-017: ⋯ → Open in full view navigates with the current symbol and timeframe', async () => {
    await backend.settings.setUiState('chart.dashboard', JSON.stringify({ identifier: 'NSE:RELIANCE', timeframe: '15m' }));
    renderWithFullRoute();
    fireEvent.click(await screen.findByRole('button', { name: 'More chart actions' }));
    fireEvent.click(screen.getByRole('menuitem', { name: 'Open in full view' }));
    expect(await screen.findByText('at /chart-full {"identifier":"NSE:RELIANCE","timeframe":"15m"}')).toBeInTheDocument();
  });

  it('FR-CHART-017 AC3: F11 (chart.fullView) on Home opens the full view', async () => {
    renderWithFullRoute();
    await screen.findByText('NIFTY 50');
    act(() => shortcuts.handlers['chart.fullView']());
    expect(await screen.findByText('at /chart-full {"identifier":"NSE:NIFTY 50","timeframe":"1h"}')).toBeInTheDocument();
  });

  const renderRoundTrip = () => render(
    <BackendProvider backend={backend}>
      <MemoryRouter><Routes><Route path="/" element={<DashboardChart />} /><Route path="/chart-full" element={<ChartFullView />} /></Routes></MemoryRouter>
    </BackendProvider>,
  );
  const fullHeading = () => screen.findByRole('heading', { level: 1, name: 'Full-view chart' });
  const more = () => screen.findByRole('button', { name: 'More chart actions' });

  it.each([
    ['Back', () => fireEvent.click(screen.getByRole('button', { name: 'Back' }))],
    ['Escape', () => fireEvent.keyDown(document.activeElement ?? document.body, { key: 'Escape' })],
    ['F11', () => act(() => shortcuts.handlers['chart.fullView']())],
  ] as const)('%s in the full view returns to Home and focus goes back to "More chart actions"', async (_name, leave) => {
    renderRoundTrip();
    fireEvent.click(await more());
    fireEvent.click(screen.getByRole('menuitem', { name: 'Open in full view' }));
    await fullHeading();
    expect(screen.queryByRole('button', { name: 'More chart actions' })).toBeNull(); // the full view has no ⋯ menu
    await waitFor(() => expect(document.activeElement).toBe(screen.getByRole('region', { name: /Price chart/ })));
    leave();
    const trigger = await more();
    await waitFor(() => expect(document.activeElement).toBe(trigger));
  });
});

describe('DashboardChart in the shell', () => {
  it('carry-forward e: Home keeps the "Main chart" region and mounts the dashboard chart in it', async () => {
    renderApp({ backend });
    const region = await screen.findByRole('region', { name: 'Main chart' });
    expect(await within(region).findByRole('region', { name: /Price chart/ })).toBeInTheDocument();
    expect(region.querySelector('[data-context="dashboard"][data-shortcut-scope="chart"]')).not.toBeNull();
  });

  it('carry-forward c: ShellLayout wires the ticker strip, so clicking a pill changes the dashboard chart', async () => {
    const { user } = renderApp({ backend });
    const region = await screen.findByRole('region', { name: 'Main chart' });
    const symbol = () => within(region).getByRole('button', { name: 'Change symbol' });
    await waitFor(() => expect(symbol()).toHaveTextContent('NIFTY 50'));
    const strip = screen.getByRole('group', { name: 'Index ticker' });
    await waitFor(() => expect(strip.querySelectorAll('[data-instrument-name]')).toHaveLength(3));
    await user.click(within(strip).getByRole('button', { name: /^SENSEX/ }));
    await waitFor(() => expect(symbol()).toHaveTextContent('SENSEX'));
    await waitFor(() => expect(JSON.parse(backend.state.uiState['chart.dashboard'])).toMatchObject({ identifier: 'BSE:SENSEX' }));
  });
});
