import { act, screen, waitFor, within } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { demoSeed, readySession } from '../api/mock/fixtures';
import { createMockBackend } from '../api/mock/index';
import { resetAllStores } from '../store/reset';
import { renderApp } from '../test/render';

const openMenu = async (user: ReturnType<typeof renderApp>['user']) =>
  user.click(await screen.findByRole('button', { name: 'Account menu' }));

describe('FR-SHELL-006 quick links', () => {
  it('AC1 + FR-SHELL-001 AC3: Order Book opens the order book and only that link is active', async () => {
    const { user, router } = renderApp();
    const links = await screen.findByRole('navigation', { name: 'Quick links' });
    await user.click(await within(links).findByRole('link', { name: 'Order Book' }));
    expect(router.state.location.pathname).toBe('/orders');
    expect(within(links).getByRole('link', { name: 'Order Book' })).toHaveAttribute('aria-current', 'page');
    expect(within(links).getByRole('link', { name: 'Positions' })).not.toHaveAttribute('aria-current');
  });
  it('each link has a tooltip; P0 shows no bell and no order buttons', async () => {
    renderApp();
    const link = await screen.findByRole('link', { name: 'Positions' });
    act(() => link.focus());
    expect(screen.getByRole('tooltip')).toHaveTextContent('Open positions');
    expect(screen.queryByRole('link', { name: /Notifications/ })).toBeNull();
    expect(screen.queryByRole('button', { name: /^(B|S|Buy|Sell)$/ })).toBeNull();
  });
});

describe('FR-SHELL-010 profile menu', () => {
  it('AC1: shows the name and client ID from the profile', async () => {
    const { user } = renderApp();
    await openMenu(user);
    const menu = screen.getByRole('menu', { name: 'Account menu' });
    expect(await within(menu).findByText('Asha Rao')).toBeInTheDocument();
    expect(within(menu).getByText('AB1234')).toBeInTheDocument();
    expect(within(menu).getAllByRole('menuitem').map((m) => m.textContent)).toEqual(['Profile', 'Holdings', 'Settings', 'Help']);
  });
  it('AC2: Escape closes the menu and returns focus to the avatar', async () => {
    const { user } = renderApp();
    await openMenu(user);
    await user.keyboard('{Escape}');
    expect(screen.queryByRole('menu')).toBeNull();
    expect(screen.getByRole('button', { name: 'Account menu' })).toHaveFocus();
  });
  it('AC3: shows the build version', async () => {
    const seed = demoSeed();
    seed.info.version = '1.2.3';
    const { user } = renderApp({ backend: createMockBackend(seed) });
    await openMenu(user);
    expect(await screen.findByText('v1.2.3')).toBeInTheDocument();
  });
  it('AC4: Help opens the built-in help inside the app', async () => {
    const { user, router } = renderApp();
    await openMenu(user);
    await user.click(screen.getByRole('menuitem', { name: 'Help' }));
    expect(router.state.location.pathname).toBe('/help');
  });
  it('when the session has expired: "Not logged in" and a Log in item', async () => {
    const { user, backend, router } = renderApp();
    await screen.findByRole('button', { name: 'Account menu' });
    act(() => backend.emit('session.state', readySession({ state: 'expired' })));
    await openMenu(user);
    expect(screen.getByText('Not logged in')).toBeInTheDocument();
    await user.click(screen.getByRole('menuitem', { name: 'Log in' }));
    expect(router.state.location.pathname).toBe('/login');
  });
});

describe('profile across sign-out (carry-forward Task 8)', () => {
  it('the profile is cleared on logout and fetched again when the same user logs back in', async () => {
    const { user, backend } = renderApp();
    await openMenu(user);
    expect(await screen.findByText('Asha Rao')).toBeInTheDocument();
    await user.keyboard('{Escape}');
    act(() => backend.emit('session.state', readySession({ state: 'logged_out', sessionId: '' })));
    await openMenu(user);
    expect(screen.getByText('Not logged in')).toBeInTheDocument();
    expect(screen.queryByText('Asha Rao')).toBeNull();
    await user.keyboard('{Escape}');
    act(() => backend.emit('session.state', readySession()));
    await openMenu(user);
    expect(await screen.findByText('Asha Rao')).toBeInTheDocument();
  });
});

describe('FR-SHELL-011 theme toggle', () => {
  it('AC1: a fresh install is light with the toggle off; AC2: switching applies at once and survives a restart', async () => {
    const backend = createMockBackend(demoSeed());
    const first = renderApp({ backend });
    await openMenu(first.user);
    const toggle = screen.getByRole('menuitemcheckbox', { name: 'Dark theme' });
    expect(toggle).toHaveAttribute('aria-checked', 'false');
    expect(document.documentElement.dataset.theme).toBe('light');
    await first.user.click(toggle);
    expect(document.documentElement.dataset.theme).toBe('dark');
    expect(backend.state.settings['appearance.theme']).toBe('"dark"');
    first.unmount();
    resetAllStores();
    const second = renderApp({ backend });
    await waitFor(() => expect(document.documentElement.dataset.theme).toBe('dark'));
    await openMenu(second.user);
    expect(screen.getByRole('menuitemcheckbox', { name: 'Dark theme' })).toHaveAttribute('aria-checked', 'true');
    expect(document.documentElement.dataset.theme).toBe('dark');
  });
});
