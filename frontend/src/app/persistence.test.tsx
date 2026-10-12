import { screen, waitFor } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { demoSeed } from '../api/mock/fixtures';
import { createMockBackend } from '../api/mock/index';
import { resetAllStores } from '../store/reset';
import { renderApp } from '../test/render';

describe('settings-backed shell state', () => {
  it('FR-SHELL-002 AC4: the collapsed sidebar survives a restart', async () => {
    const backend = createMockBackend(demoSeed());
    const first = renderApp({ backend });
    await first.user.click(await screen.findByRole('button', { name: 'Collapse sidebar' }));
    expect(backend.state.settings['appearance.sidebar_collapsed']).toBe('true');
    first.unmount();
    resetAllStores();
    renderApp({ backend });
    expect(await screen.findByRole('button', { name: 'Expand sidebar' })).toBeInTheDocument();
  });
  it('applies the persisted dark theme on start', async () => {
    const seed = demoSeed();
    seed.settings = { 'appearance.theme': '"dark"' };
    renderApp({ backend: createMockBackend(seed) });
    await waitFor(() => expect(document.documentElement.dataset.theme).toBe('dark'));
  });
});
