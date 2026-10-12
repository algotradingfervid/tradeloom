import { beforeEach, describe, expect, it, vi } from 'vitest';

const saved = vi.hoisted(() => ({ value: null as unknown }));
vi.mock('../api/uiState', () => ({ readUiState: vi.fn(async () => saved.value), writeUiState: vi.fn(async () => undefined) }));
import { readUiState, writeUiState } from '../api/uiState';
import { __resetChartContexts, DASHBOARD_CONTEXT, hydrateDashboard, showOnDashboard, useChartContexts } from './chartContextsStore';
import { registeredResets, resetAllStores, resetOnLogout } from './reset';

beforeEach(() => { __resetChartContexts(); saved.value = null; vi.clearAllMocks(); });

describe('chartContextsStore', () => {
  it('FR-HOME-001 AC1: first run → NIFTY 50 at the default timeframe', async () => {
    await hydrateDashboard('1h');
    expect(useChartContexts.getState().contexts[DASHBOARD_CONTEXT]).toEqual({ identifier: 'NSE:NIFTY 50', timeframe: '1h' });
    expect(readUiState).toHaveBeenCalledWith('chart.dashboard', null);
  });

  it('FR-CHART-016 AC1: restores the saved symbol/timeframe and persists changes under chart.dashboard', async () => {
    saved.value = { identifier: 'NSE:RELIANCE', timeframe: '15m' };
    await hydrateDashboard('1h');
    expect(useChartContexts.getState().contexts.dashboard).toEqual({ identifier: 'NSE:RELIANCE', timeframe: '15m' });
    useChartContexts.getState().setTimeframe(DASHBOARD_CONTEXT, '1d');
    expect(writeUiState).toHaveBeenCalledWith('chart.dashboard', { identifier: 'NSE:RELIANCE', timeframe: '1d' });
  });

  it('FR-CHART-016 AC2: hydrating again (Home → Scanners → Home) keeps the in-memory state', async () => {
    await hydrateDashboard('1h');
    useChartContexts.getState().setSymbol(DASHBOARD_CONTEXT, 'NSE:SBIN');
    await hydrateDashboard('1h');
    expect(useChartContexts.getState().contexts.dashboard.identifier).toBe('NSE:SBIN');
    expect(readUiState).toHaveBeenCalledTimes(1);
  });

  it('a junk saved timeframe falls back to the shipped default', async () => {
    saved.value = { identifier: 'NSE:TCS', timeframe: '2h' };
    await hydrateDashboard('1d');
    expect(useChartContexts.getState().contexts.dashboard.timeframe).toBe('1h');
  });

  it('FR-CHART-001 AC1: contexts are independent and only the dashboard is persisted', async () => {
    await hydrateDashboard('1h');
    useChartContexts.getState().init('full', { identifier: 'NSE:TCS', timeframe: '5m' });
    useChartContexts.getState().setTimeframe('full', '1d');
    expect(useChartContexts.getState().contexts.dashboard.timeframe).toBe('1h');
    expect(writeUiState).not.toHaveBeenCalled();
  });

  it('showOnDashboard before Home has hydrated is applied on hydrate', async () => {
    showOnDashboard('BSE:SENSEX');
    await hydrateDashboard('1h');
    expect(useChartContexts.getState().contexts.dashboard.identifier).toBe('BSE:SENSEX');
  });

  it('showOnDashboard after hydrate switches the dashboard symbol and persists it', async () => {
    await hydrateDashboard('1h');
    showOnDashboard('NSE:NIFTY BANK');
    expect(useChartContexts.getState().contexts.dashboard.identifier).toBe('NSE:NIFTY BANK');
    expect(writeUiState).toHaveBeenCalledWith('chart.dashboard', { identifier: 'NSE:NIFTY BANK', timeframe: '1h' });
  });

  it('carry-forward f: registers {name, inspect}; install-wide (window_state has no user), so sign-out keeps it', async () => {
    const reg = registeredResets().find((r) => r.name === 'chartContexts');
    expect(reg).toBeDefined();
    expect(reg!.onLogout).toBe(false);
    await hydrateDashboard('1h');
    expect(reg!.inspect!()).toMatchObject({ contexts: { dashboard: { identifier: 'NSE:NIFTY 50' } }, epoch: expect.any(Number) });
    resetOnLogout();
    expect(useChartContexts.getState().contexts.dashboard.identifier).toBe('NSE:NIFTY 50');
    resetAllStores();
    expect(useChartContexts.getState().contexts).toEqual({});
  });

  it('a hydrate still in flight when the store is reset does not repopulate it', async () => {
    let release!: (v: unknown) => void;
    vi.mocked(readUiState).mockImplementationOnce(() => new Promise((r) => { release = r; }));
    showOnDashboard('BSE:SENSEX');
    const p = hydrateDashboard('1h');
    resetAllStores();
    release({ identifier: 'NSE:TCS', timeframe: '5m' });
    await p;
    expect(useChartContexts.getState().contexts).toEqual({});
  });
});
