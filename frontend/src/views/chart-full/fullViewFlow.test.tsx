import { fireEvent, screen, waitFor, within } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { createMockBackend } from '../../api/mock/index';
import { demoSeed } from '../../api/mock/fixtures';
import { useShortcutStore } from '../../shortcuts/store';
import { renderApp } from '../../test/render';

/** The registry comes from the mock backend's Settings.Shortcuts(); keys pressed before it loads resolve to nothing. */
const shortcutsLoaded = () => waitFor(() => expect(useShortcutStore.getState().actions.some((a) => a.id === 'chart.fullView')).toBe(true));

/** Opens the full view from Home with one F11 press, once the dashboard chart and the shortcut list are ready. */
async function openFullView() {
  const region = await screen.findByRole('region', { name: 'Main chart' });
  await waitFor(() => expect(within(region).getByRole('button', { name: 'Change symbol' })).toHaveTextContent('NIFTY 50'));
  await shortcutsLoaded();
  fireEvent.keyDown(document.body, { key: 'F11' });
  return screen.findByRole('main', { name: 'Full-view chart' });
}

// The real app: P0-4's ShortcutProvider resolves the keys from the mock backend's registry.
describe('full-view chart in the app (FR-CHART-017)', () => {
  it('F11 on Home opens the full view outside the shell; F11 again returns to Home with focus on "More chart actions"', async () => {
    const { router } = renderApp();
    const main = await openFullView();
    expect(router.state.location.pathname).toBe('/chart-full');
    expect(within(main).getByRole('button', { name: 'Change symbol' })).toHaveTextContent('NIFTY 50');
    // Outside the shell: no top bar, ticker strip or sidebar.
    expect(screen.queryByRole('group', { name: 'Index ticker' })).toBeNull();
    expect(screen.queryByRole('region', { name: 'Main chart' })).toBeNull();
    await waitFor(() => expect(document.activeElement).toBe(within(main).getByRole('region', { name: /Price chart/ })));

    fireEvent.keyDown(document.activeElement!, { key: 'F11' });
    await waitFor(() => expect(router.state.location.pathname).toBe('/home'));
    const trigger = await screen.findByRole('button', { name: 'More chart actions' });
    await waitFor(() => expect(document.activeElement).toBe(trigger));
  });

  it('/chart-full is remembered as the last view', async () => {
    const { router, backend } = renderApp();
    await openFullView();
    expect(router.state.location.pathname).toBe('/chart-full');
    await waitFor(() => expect(backend.state.settings['ui.last_route']).toBe('"/chart-full"'));
  });

  it('Ruling 119 I1: the app-wide shortcuts work in the full view (F1 opens Help)', async () => {
    const { router } = renderApp();
    await openFullView();
    fireEvent.keyDown(document.activeElement!, { key: 'F1' });
    await waitFor(() => expect(router.state.location.pathname).toBe('/help'));
  });

  it('Ruling 119 I1: the app-wide shortcuts work in the full view (Alt+1 goes Home)', async () => {
    const { router } = renderApp();
    await openFullView();
    fireEvent.keyDown(document.activeElement!, { key: '¡', code: 'Digit1', altKey: true });
    await waitFor(() => expect(router.state.location.pathname).toBe('/home'));
  });

  it('Ruling 119 m3: ui.last_route = /chart-full reopens the full view with the dashboard chart state', async () => {
    const backend = createMockBackend(demoSeed());
    backend.state.settings['ui.last_route'] = '"/chart-full"';
    backend.state.uiState['chart.dashboard'] = JSON.stringify({ identifier: 'NSE:RELIANCE', timeframe: '15m' });
    const { router } = renderApp({ route: '/', backend });
    const main = await screen.findByRole('main', { name: 'Full-view chart' });
    expect(router.state.location.pathname).toBe('/chart-full');
    await waitFor(() => expect(within(main).getByRole('button', { name: 'Change symbol' })).toHaveTextContent('RELIANCE'));
    expect((within(main).getByLabelText('Timeframe') as HTMLSelectElement).value).toBe('15m');
  });
});
