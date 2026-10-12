import { render, screen, waitFor, within } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { beforeEach, describe, expect, it, vi } from 'vitest';

vi.mock('../../market/hooks', () => ({ useSubscription: vi.fn(), usePolled: () => false }));
vi.mock('../LightweightChartsAdapter', async () => ({ LightweightChartsAdapter: (await import('../testing/FakeAdapter')).FakeAdapter }));

import { DashboardChart } from './DashboardChart';
import { BackendProvider } from '../../api/BackendContext';
import { setBackend } from '../../api/current';
import { demoSeed, readySession } from '../../api/mock/fixtures';
import { createMockBackend, type MockBackend } from '../../api/mock/index';
import { __resetChartContexts } from '../../store/chartContextsStore';
import { useSessionStore } from '../../store/session';
import { useSettingsStore } from '../../store/settings';
import { renderApp } from '../../test/render';

let backend: MockBackend;
const renderDash = () => render(<BackendProvider backend={backend}><MemoryRouter><DashboardChart /></MemoryRouter></BackendProvider>);

beforeEach(() => {
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
