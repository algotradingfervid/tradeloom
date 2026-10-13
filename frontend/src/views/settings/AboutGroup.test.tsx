import { screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { CHART_ATTRIBUTION } from '../../chart/attribution';
import { useToastStore } from '../../store/toasts';
import { renderApp } from '../../test/render';

describe('Settings → About: chart library attribution (Ruling 110 I4, S0-5 G8)', () => {
  it('shows the lightweight-charts name, licence and NOTICE text', async () => {
    renderApp({ route: '/settings/about' });
    const section = await screen.findByRole('region', { name: 'Chart library' });
    expect(section).toHaveTextContent(CHART_ATTRIBUTION.name);
    expect(section).toHaveTextContent(CHART_ATTRIBUTION.licence);
    expect(section).toHaveTextContent(CHART_ATTRIBUTION.notice);
  });

  it('links to the attribution URL and opens it in the system browser, not in the app window', async () => {
    const { user, backend } = renderApp({ route: '/settings/about' });
    const link = await screen.findByRole('link', { name: /TradingView website/ });
    expect(link).toHaveAttribute('href', CHART_ATTRIBUTION.url);
    const before = window.location.href;
    await user.click(link);
    expect(backend.callsTo('system.openExternal').map((c) => c.args)).toEqual([[CHART_ATTRIBUTION.url]]);
    expect(window.location.href).toBe(before);
  });

  it('a refused open shows Go\'s reason as an error toast', async () => {
    const { user, backend } = renderApp({ route: '/settings/about' });
    const link = await screen.findByRole('link', { name: /TradingView website/ });
    backend.failNext('system.openExternal', 'No web browser is available.', 'unavailable');
    await user.click(link);
    await screen.findByRole('link', { name: /TradingView website/ });
    await expect.poll(() => useToastStore.getState().visible.map((t) => `${t.level}: ${t.message}`))
      .toContain('error: No web browser is available.');
  });
});
