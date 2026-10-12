import { fireEvent, screen, waitFor, within } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { renderApp } from '../../test/render';

// The real app: P0-4's ShortcutProvider resolves F11 from the mock backend's registry (chart.fullView, global).
describe('full-view chart in the app (FR-CHART-017)', () => {
  it('F11 on Home opens the full view outside the shell; F11 again returns to Home with focus on "More chart actions"', async () => {
    const { router } = renderApp();
    const region = await screen.findByRole('region', { name: 'Main chart' });
    await waitFor(() => expect(within(region).getByRole('button', { name: 'Change symbol' })).toHaveTextContent('NIFTY 50'));
    expect(screen.getByRole('group', { name: 'Index ticker' })).toBeInTheDocument();

    await waitFor(() => {
      fireEvent.keyDown(document.body, { key: 'F11' });
      expect(router.state.location.pathname).toBe('/chart-full');
    });
    const main = await screen.findByRole('main', { name: 'Full-view chart' });
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
    await screen.findByRole('region', { name: 'Main chart' });
    await waitFor(() => {
      fireEvent.keyDown(document.body, { key: 'F11' });
      expect(router.state.location.pathname).toBe('/chart-full');
    });
    await waitFor(() => expect(backend.state.settings['ui.last_route']).toBe('"/chart-full"'));
  });
});
