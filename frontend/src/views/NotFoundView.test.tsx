import { render, screen } from '@testing-library/react';
import { createMemoryRouter, RouterProvider } from 'react-router-dom';
import { describe, expect, it } from 'vitest';
import { renderApp } from '../test/render';
import { NotFoundView } from './NotFoundView';

describe('FR-SHELL-012 unknown-view fallback', () => {
  it('AC1: an unregistered route shows "Page not found" with a working Go to Home', async () => {
    const { user, router } = renderApp({ route: '/scalper' }); // Scalper is a P6 view: not registered in P0
    expect(await screen.findByRole('heading', { name: 'Page not found' })).toBeInTheDocument();
    expect(screen.getByRole('navigation', { name: 'Main navigation' })).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'Go to Home' }));
    expect(router.state.location.pathname).toBe('/home');
  });
  it('AC2: names the missing item type', () => {
    render(<RouterProvider router={createMemoryRouter([{ path: '*', element: <NotFoundView kind="scanner" /> }])} />);
    expect(screen.getByRole('heading', { name: 'This scanner no longer exists' })).toBeInTheDocument();
  });
});
