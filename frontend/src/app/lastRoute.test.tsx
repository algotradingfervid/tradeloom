import { act, waitFor } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { renderApp } from '../test/render';
import { isRememberable, safeLastRoute } from './lastRoute';

describe('FR-SHELL-019 window state — last view', () => {
  it('remembers only real P0 shell views', () => {
    expect(isRememberable('/holdings')).toBe(true);
    expect(isRememberable('/settings/data')).toBe(true);
    expect(isRememberable('/login')).toBe(false);
    expect(isRememberable('/scalper')).toBe(false);
    expect(safeLastRoute('/scalper')).toBe('/home');
    expect(safeLastRoute(42)).toBe('/home');
    expect(safeLastRoute('/orders')).toBe('/orders');
  });
  it('writes ui.last_route as the user moves and ignores unknown views', async () => {
    const { backend, router } = renderApp({ route: '/home' });
    await waitFor(() => expect(backend.state.settings['ui.last_route']).toBe('"/home"'));
    await act(() => router.navigate('/no-such-view'));
    expect(backend.state.settings['ui.last_route']).toBe('"/home"');
  });
});
