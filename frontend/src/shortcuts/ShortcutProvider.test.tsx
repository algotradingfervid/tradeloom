import { act, render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import type { ReactNode } from 'react';
import { createMemoryRouter, RouterProvider, useLocation } from 'react-router-dom';
import { describe, expect, it, vi } from 'vitest';
import type { ShortcutAction } from '../api/backend';
import { BackendProvider } from '../api/BackendContext';
import { startEventBridge } from '../api/events';
import { demoSeed, readySession } from '../api/mock/fixtures';
import { createMockBackend } from '../api/mock/index';
import { ToastRegion } from '../components/ToastRegion';
import { resetOnLogout } from '../store/reset';
import { useSettingsStore } from '../store/settings';
import { pushToast } from '../store/toasts';
import { GlobalShortcuts } from './GlobalShortcuts';
import { reloadShortcuts, ShortcutProvider, useShortcut } from './ShortcutProvider';
import { useShortcutStore } from './store';

function Where() {
  return <output data-testid="where">{useLocation().pathname}</output>;
}

function Probe({ id, onFire }: { id: string; onFire: () => void }) {
  useShortcut(id, onFire);
  return null;
}

function setup(extra?: ReactNode) {
  const backend = createMockBackend(demoSeed());
  const router = createMemoryRouter(
    [{ path: '*', element: (<ShortcutProvider><GlobalShortcuts /><Where /><input aria-label="Notes" />{extra}<ToastRegion /></ShortcutProvider>) }],
    { initialEntries: ['/orders'] },
  );
  render(<BackendProvider backend={backend}><RouterProvider router={router} /></BackendProvider>);
  return { backend, user: userEvent.setup() };
}

const loaded = () => waitFor(() => expect(useShortcutStore.getState().actions.length).toBeGreaterThan(0));
const where = () => screen.getByTestId('where');

describe('FR-SHELL-019 global shortcuts', () => {
  it('Alt+1 goes Home; Alt+3 (a later-phase sidebar entry) does nothing in P0', async () => {
    const { user } = setup();
    await loaded();
    await user.keyboard('{Alt>}3{/Alt}');
    expect(where()).toHaveTextContent('/orders');
    await user.keyboard('{Alt>}1{/Alt}');
    expect(where()).toHaveTextContent('/home');
  });
  it('F1 opens help, even while typing in a field', async () => {
    const { user } = setup();
    await loaded();
    await user.click(screen.getByLabelText('Notes'));
    await user.keyboard('{F1}');
    expect(where()).toHaveTextContent('/help');
  });
  it('plain-key bindings are ignored while typing', async () => {
    const fire = vi.fn();
    const { user } = setup(<div data-shortcut-scope="chart"><input aria-label="Chart note" /><Probe id="order.new" onFire={fire} /></div>);
    await loaded();
    await user.click(screen.getByLabelText('Chart note'));
    await user.keyboard('{Shift>}t{/Shift}');
    expect(fire).not.toHaveBeenCalled();
  });
  it('FR-SHELL-009 AC5 + FR-SHELL-019 AC4: Alt+T focuses the newest toast, but on a chart it selects the trend line', async () => {
    const trend = vi.fn();
    const { user } = setup(<div data-shortcut-scope="chart" tabIndex={0} data-testid="chart"><Probe id="chart.trendLine" onFire={trend} /></div>);
    await loaded();
    act(() => { pushToast({ level: 'error', message: 'Holdings could not be loaded' }); });
    const toast = screen.getByText('Holdings could not be loaded').closest('.toast') as HTMLElement;
    screen.getByTestId('chart').focus();
    await user.keyboard('{Alt>}t{/Alt}');
    expect(trend).toHaveBeenCalledTimes(1);
    expect(toast).not.toHaveFocus();
    act(() => { (document.activeElement as HTMLElement).blur(); });
    await user.keyboard('{Alt>}t{/Alt}');
    expect(toast).toHaveFocus();
  });
  it('FR-SHELL-019 AC1 (client part): after remapping the kill switch only the new binding fires', async () => {
    const kill = vi.fn();
    const { backend, user } = setup(<Probe id="kill.switch" onFire={kill} />);
    await loaded();
    await backend.settings.rebind('kill.switch', 'Ctrl+Shift+X');
    await act(() => reloadShortcuts(backend));
    await user.keyboard('{Control>}{Shift>}k{/Shift}{/Control}');
    expect(kill).not.toHaveBeenCalled();
    await user.keyboard('{Control>}{Shift>}x{/Shift}{/Control}');
    expect(kill).toHaveBeenCalledTimes(1);
  });
});

// P0-4 carry-forwards (Ruling 88), Task 6.
describe('shortcut list sync with Go', () => {
  const only = (a: Partial<ShortcutAction>): ShortcutAction[] => [
    { id: 'kill.switch', label: 'Kill switch', scope: 'global', defaultBinding: 'Ctrl+Shift+K', binding: 'Ctrl+Shift+K', ...a },
  ];

  it('the settings.changed "shortcuts" event replaces the whole list (no merge) and is not a setting', () => {
    const backend = createMockBackend(demoSeed());
    const stop = startEventBridge(backend, { navigate: vi.fn() });
    useShortcutStore.getState().setActions(demoSeed().shortcuts);
    backend.emit('settings.changed', { key: 'shortcuts', value: only({ binding: 'Ctrl+Shift+X' }) });
    expect(useShortcutStore.getState().actions).toEqual(only({ binding: 'Ctrl+Shift+X' }));
    expect('shortcuts' in useSettingsStore.getState().values).toBe(false);
    backend.emit('settings.changed', { key: 'shortcuts', value: null });
    expect(useShortcutStore.getState().actions).toEqual([]);
    stop();
  });
  it('a rebind elsewhere (Settings → Shortcuts) reaches the provider through the event, as Go publishes it', async () => {
    const kill = vi.fn();
    const { backend, user } = setup(<Probe id="kill.switch" onFire={kill} />);
    const stop = startEventBridge(backend, { navigate: vi.fn() });
    await loaded();
    await act(async () => { await backend.settings.rebind('kill.switch', 'Ctrl+Shift+X'); });
    expect(useShortcutStore.getState().actions.find((a) => a.id === 'kill.switch')?.binding).toBe('Ctrl+Shift+X');
    await user.keyboard('{Control>}{Shift>}x{/Shift}{/Control}');
    expect(kill).toHaveBeenCalledTimes(1);
    await act(async () => { await backend.settings.resetShortcuts(); });
    expect(useShortcutStore.getState().actions.find((a) => a.id === 'kill.switch')?.binding).toBe('Ctrl+Shift+K');
    stop();
  });
  it('a fetch that started before a newer shortcuts event does not overwrite it (Ruling 83-F I1)', async () => {
    const backend = createMockBackend(demoSeed());
    const stop = startEventBridge(backend, { navigate: vi.fn() });
    const pending = reloadShortcuts(backend);
    backend.emit('settings.changed', { key: 'shortcuts', value: only({ binding: 'Ctrl+Shift+X' }) });
    await pending;
    expect(useShortcutStore.getState().actions).toEqual(only({ binding: 'Ctrl+Shift+X' }));
    stop();
  });
  it('signing out keeps the shortcut list (not account data, Ruling 85)', () => {
    const backend = createMockBackend(demoSeed());
    const stop = startEventBridge(backend, { navigate: vi.fn() });
    useShortcutStore.getState().setActions(demoSeed().shortcuts);
    backend.emit('session.state', readySession());
    backend.emit('session.state', readySession({ state: 'logged_out' }));
    resetOnLogout();
    expect(useShortcutStore.getState().actions).toHaveLength(demoSeed().shortcuts.length);
    stop();
  });
});
