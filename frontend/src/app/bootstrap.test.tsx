import { screen, waitFor } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { demoSeed } from '../api/mock/fixtures';
import { createMockBackend } from '../api/mock/index';
import { useSettingsStore } from '../store/settings';
import { renderApp } from '../test/render';

describe('root layout bootstrap (Ruling 83-F I2, M6)', () => {
  it('shows a retry notice when the start-up load fails and clears it after a successful retry', async () => {
    const backend = createMockBackend(demoSeed());
    backend.failNext('settings.all', 'database is locked');
    const { user } = renderApp({ backend });
    const alert = await screen.findByRole('alert');
    expect(alert).toHaveTextContent('Some app data could not be loaded');
    expect(useSettingsStore.getState().loaded).toBe(false);
    await user.click(screen.getByRole('button', { name: 'Retry' }));
    await waitFor(() => expect(screen.queryByRole('alert')).not.toBeInTheDocument());
    expect(useSettingsStore.getState().loaded).toBe(true);
    expect(backend.callsTo('settings.all')).toHaveLength(2);
  });
  it('a bootstrap still in flight at unmount applies nothing', async () => {
    const backend = createMockBackend(demoSeed());
    const view = renderApp({ backend });
    view.unmount();
    await new Promise((r) => setTimeout(r, 0));
    expect(useSettingsStore.getState().loaded).toBe(false);
  });
});
