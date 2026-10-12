import { act, screen, waitFor, within } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import type { Profile } from '../api/backend';
import { demoSeed, profileFixture, readySession } from '../api/mock/fixtures';
import { createMockBackend } from '../api/mock/index';
import { resetAllStores } from '../store/reset';
import { useSessionStore } from '../store/session';
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

function deferred<T>() {
  let resolve!: (v: T) => void;
  let reject!: (e: unknown) => void;
  const promise = new Promise<T>((res, rej) => {
    resolve = res;
    reject = rej;
  });
  return { promise, resolve, reject };
}

const otherProfile: Profile = { ...profileFixture, userId: 'CD5678', name: 'Ravi Menon', initials: 'RM' };

describe('Ruling 90 (I1): a late profile response never lands on another session', () => {
  it('a profile() that resolves after logged_out and a login as another user is discarded', async () => {
    const backend = createMockBackend(demoSeed());
    const first = deferred<Profile>();
    const second = deferred<Profile>();
    const spy = vi.spyOn(backend.session, 'profile').mockReturnValueOnce(first.promise).mockReturnValueOnce(second.promise);
    const { user } = renderApp({ backend });
    await waitFor(() => expect(spy).toHaveBeenCalledTimes(1));
    act(() => backend.emit('session.state', readySession({ state: 'logged_out', sessionId: '' })));
    act(() => backend.emit('session.state', readySession({ userId: 'CD5678', sessionId: 'sess-2' })));
    await waitFor(() => expect(spy).toHaveBeenCalledTimes(2));
    await act(async () => first.resolve(profileFixture));
    expect(useSessionStore.getState().profile).toBeNull();
    await act(async () => second.resolve(otherProfile));
    await openMenu(user);
    const menu = screen.getByRole('menu', { name: 'Account menu' });
    expect(await within(menu).findByText('Ravi Menon')).toBeInTheDocument();
    expect(within(menu).queryByText('Asha Rao')).toBeNull();
  });
  it('a profile() that fails after logged_out shows no toast', async () => {
    const backend = createMockBackend(demoSeed());
    const first = deferred<Profile>();
    const spy = vi.spyOn(backend.session, 'profile').mockReturnValueOnce(first.promise);
    renderApp({ backend });
    await waitFor(() => expect(spy).toHaveBeenCalledTimes(1));
    act(() => backend.emit('session.state', readySession({ state: 'logged_out', sessionId: '' })));
    await act(async () => first.reject(new Error('network down')));
    expect(screen.queryByText(/could not be loaded/)).toBeNull();
  });
});

describe('Ruling 90 (M1, M2): profile failure and logged-out avatar', () => {
  it('M1: a failed load while ready shows a neutral placeholder and Retry, not "Not logged in"', async () => {
    const backend = createMockBackend(demoSeed());
    backend.failNext('session.profile', 'Kite is unreachable', 'network');
    const { user } = renderApp({ backend });
    expect(await screen.findByText('Your Kite profile could not be loaded: Kite is unreachable')).toBeInTheDocument();
    await openMenu(user);
    const menu = screen.getByRole('menu', { name: 'Account menu' });
    expect(within(menu).queryByText('Not logged in')).toBeNull();
    expect(within(menu).getByText('Profile not loaded')).toBeInTheDocument();
    expect(within(menu).queryByRole('menuitem', { name: 'Log in' })).toBeNull();
    await user.click(within(menu).getByRole('menuitem', { name: 'Retry loading profile' }));
    await openMenu(user);
    expect(await within(screen.getByRole('menu', { name: 'Account menu' })).findByText('Asha Rao')).toBeInTheDocument();
  });
  it('M2: the logged-out avatar is hidden from assistive technology', async () => {
    const { backend } = renderApp();
    const button = await screen.findByRole('button', { name: 'Account menu' });
    act(() => backend.emit('session.state', readySession({ state: 'logged_out', sessionId: '' })));
    expect(button.querySelector('.avatar')).toHaveAttribute('aria-hidden', 'true');
  });
});

describe('Ruling 90 (I2): theme toggle writes', () => {
  it('writes appearance.theme before appearance.follow_system and awaits both', async () => {
    const { user, backend } = renderApp();
    await openMenu(user);
    await user.click(screen.getByRole('menuitemcheckbox', { name: 'Dark theme' }));
    await waitFor(() => expect(backend.callsTo('settings.set').map((c) => c.args[0])).toEqual(['appearance.theme', 'appearance.follow_system']));
  });
  it('a refused theme write is reported with the parsed reason and rolled back', async () => {
    const { user, backend } = renderApp();
    await openMenu(user);
    backend.failNext('settings.set', 'The settings database is read-only', 'io');
    await user.click(screen.getByRole('menuitemcheckbox', { name: 'Dark theme' }));
    expect(await screen.findByText('The theme could not be saved: The settings database is read-only')).toBeInTheDocument();
    await waitFor(() => expect(document.documentElement.dataset.theme).toBe('light'));
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
