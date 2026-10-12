import { act, screen, waitFor, within } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { Profile } from '../../api/backend';
import { demoSeed, FIXED_NOW, profileFixture, readySession } from '../../api/mock/fixtures';
import { createMockBackend } from '../../api/mock/index';
import { useSessionStore } from '../../store/session';
import { useToastStore } from '../../store/toasts';
import { renderApp } from '../../test/render';

beforeEach(() => {
  vi.useFakeTimers({ toFake: ['Date'] });
  vi.setSystemTime(FIXED_NOW);
});
afterEach(() => vi.useRealTimers());

function deferred<T>() {
  let resolve!: (v: T) => void;
  const promise = new Promise<T>((res) => {
    resolve = res;
  });
  return { promise, resolve };
}

describe('FR-ACCT-001 Profile view', () => {
  it('AC1: name, email and client ID come from the Kite profile and are read-only', async () => {
    renderApp({ route: '/profile' });
    const name = await screen.findByLabelText('Name');
    expect(name).toHaveValue('Asha Rao');
    expect(name).toHaveAttribute('readonly');
    expect(screen.getByLabelText('Email')).toHaveValue('asha@example.com');
    expect(screen.getByLabelText('Client ID')).toHaveValue('AB1234');
    expect(screen.getByLabelText('Broker')).toHaveValue('Zerodha');
    expect(screen.getByLabelText('Exchanges')).toHaveValue('NSE, BSE, NFO, BFO');
    expect(screen.getByLabelText('Products')).toHaveValue('CNC, MIS, NRML');
    expect(screen.getByLabelText('Order types')).toHaveValue('MARKET, LIMIT, SL, SL-M');
  });
  it('AC3: a 09:02 login shows "Expires 06:00 tomorrow"', async () => {
    renderApp({ route: '/profile' });
    expect(await screen.findByText('Logged in at 12 Oct 2026, 09:02')).toBeInTheDocument();
    expect(screen.getByText('Expires 06:00 tomorrow')).toBeInTheDocument();
  });
  it('Refresh re-fetches the profile', async () => {
    const { user, backend } = renderApp({ route: '/profile' });
    await screen.findByLabelText('Name');
    const before = backend.callsTo('session.profile').length;
    await user.click(screen.getByRole('button', { name: 'Refresh profile' }));
    expect(backend.callsTo('session.profile').length).toBe(before + 1);
  });
  it('FR-ACCT-004 AC1 (UI part): Log out asks for confirmation, then the login screen shows', async () => {
    const { user, backend } = renderApp({ route: '/profile' });
    await user.click(await screen.findByRole('button', { name: 'Log out' }));
    const dialog = screen.getByRole('dialog', { name: 'Log out of Kite?' });
    await user.click(within(dialog).getByRole('button', { name: 'Log out' }));
    expect(backend.callsTo('session.logout')).toHaveLength(1);
    expect(await screen.findByRole('heading', { name: 'Log in to Kite' })).toBeInTheDocument();
  });
  it('FR-ACCT-004: when Kite could not be reached the local logout still happens and the reason is shown', async () => {
    const { user, backend } = renderApp({ route: '/profile' });
    const logout = backend.session.logout;
    backend.session.logout = async () => ({ ...(await logout()), remoteInvalidated: false, message: 'Kite was not reachable; the token was deleted on this computer' });
    await user.click(await screen.findByRole('button', { name: 'Log out' }));
    await user.click(within(screen.getByRole('dialog', { name: 'Log out of Kite?' })).getByRole('button', { name: 'Log out' }));
    expect(await screen.findByText('Kite was not reachable; the token was deleted on this computer')).toBeInTheDocument();
  });
  it('Escape cancels the log-out confirmation', async () => {
    const { user, backend } = renderApp({ route: '/profile' });
    await user.click(await screen.findByRole('button', { name: 'Log out' }));
    await user.keyboard('{Escape}');
    expect(screen.queryByRole('dialog')).toBeNull();
    expect(backend.callsTo('session.logout')).toHaveLength(0);
  });
});

describe('Task 13 adaptations (rulings and brief)', () => {
  it('after log out focus is on the login heading, not stranded on the page body', async () => {
    const { user } = renderApp({ route: '/profile' });
    await user.click(await screen.findByRole('button', { name: 'Log out' }));
    await user.click(within(screen.getByRole('dialog', { name: 'Log out of Kite?' })).getByRole('button', { name: 'Log out' }));
    const heading = await screen.findByRole('heading', { name: 'Log in to Kite' });
    await waitFor(() => expect(heading).toHaveFocus());
  });
  it('Ruling 98: no account data survives log out', async () => {
    const { user } = renderApp({ route: '/profile' });
    await user.click(await screen.findByRole('button', { name: 'Log out' }));
    await user.click(within(screen.getByRole('dialog', { name: 'Log out of Kite?' })).getByRole('button', { name: 'Log out' }));
    await screen.findByRole('heading', { name: 'Log in to Kite' });
    expect(useSessionStore.getState().profile).toBeNull();
    expect(document.body.textContent).not.toMatch(/Asha Rao|asha@example\.com|AB1234/);
  });
  it('a refused log out keeps the dialog open with Go\'s reason (logOut() returned the failure)', async () => {
    const { user, backend } = renderApp({ route: '/profile' });
    backend.failNext('session.logout', 'The secret store is not available', 'secret_store');
    await user.click(await screen.findByRole('button', { name: 'Log out' }));
    const dialog = screen.getByRole('dialog', { name: 'Log out of Kite?' });
    await user.click(within(dialog).getByRole('button', { name: 'Log out' }));
    expect(await within(dialog).findByRole('alert')).toHaveTextContent('The secret store is not available');
    expect(useToastStore.getState().visible.map((t) => t.level)).toEqual(['error']);
  });
  it('Ruling 90: a refresh that resolves after log out does not bring the profile back', async () => {
    const backend = createMockBackend(demoSeed());
    const { user } = renderApp({ route: '/profile', backend });
    await screen.findByLabelText('Name');
    const late = deferred<Profile>();
    vi.spyOn(backend.session, 'profile').mockReturnValueOnce(late.promise);
    await user.click(screen.getByRole('button', { name: 'Refresh profile' }));
    await user.click(screen.getByRole('button', { name: 'Log out' }));
    await user.click(within(screen.getByRole('dialog', { name: 'Log out of Kite?' })).getByRole('button', { name: 'Log out' }));
    await screen.findByRole('heading', { name: 'Log in to Kite' });
    await act(async () => late.resolve(profileFixture));
    expect(useSessionStore.getState().profile).toBeNull();
  });
  it('a failed profile load says so in text and offers Retry (Ruling 90 M1), and Log out stays available', async () => {
    const backend = createMockBackend(demoSeed());
    // The top-bar menu and this view each load once (two useProfile instances): fail both.
    const refused = new Error(JSON.stringify({ code: 'network', stage: 'session', reason: 'Kite is unreachable' }));
    vi.spyOn(backend.session, 'profile').mockRejectedValueOnce(refused).mockRejectedValueOnce(refused);
    const { user } = renderApp({ route: '/profile', backend });
    expect(await screen.findByText('Your profile could not be loaded from Kite.')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Log out' })).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'Retry' }));
    expect(await screen.findByLabelText('Name')).toHaveValue('Asha Rao');
  });
  it('an expired session says so in text instead of loading forever, and links to the login', async () => {
    const backend = createMockBackend(demoSeed());
    const { user } = renderApp({ route: '/profile', backend });
    await screen.findByLabelText('Name');
    act(() => backend.emit('session.state', readySession({ state: 'expired' })));
    const note = await screen.findByText('Your Kite session has ended. Log in again to see your profile.');
    expect(screen.queryByLabelText('Name')).toBeNull();
    expect(screen.queryByRole('status', { name: 'Loading profile' })).toBeNull();
    expect(screen.queryByRole('button', { name: 'Log out' })).toBeNull();
    // The shell's expiry banner has its own "Log in"; use the one in the view.
    await user.click(within(note.closest('.profile-notice') as HTMLElement).getByRole('button', { name: 'Log in' }));
    expect(await screen.findByRole('heading', { name: 'Log in to Kite' })).toBeInTheDocument();
  });
  it('shows no credential (the profile carries none) and no order buttons', async () => {
    renderApp({ route: '/profile' });
    await screen.findByLabelText('Name');
    expect(document.body.innerHTML).not.toContain('9f2c');
    expect(document.body.innerHTML).not.toMatch(/secret/i);
    expect(screen.queryAllByRole('button', { name: /\b(buy|sell|b|s|exit|convert)\b/i })).toHaveLength(0);
  });
});
