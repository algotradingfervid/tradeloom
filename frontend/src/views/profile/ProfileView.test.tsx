import { act, screen, waitFor, within } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { Profile } from '../../api/backend';
import { demoSeed, FIXED_NOW, profileFixture, readySession } from '../../api/mock/fixtures';
import { createMockBackend } from '../../api/mock/index';
import { loadProfile } from '../../app/useProfile';
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
    // Fix round 1 (Minor 5): a gentle info note, the login screen, and no dialog left behind.
    expect(useToastStore.getState().visible.map((t) => t.level)).toEqual(['info']);
    expect(await screen.findByRole('heading', { name: 'Log in to Kite' })).toBeInTheDocument();
    expect(screen.queryByRole('dialog')).toBeNull();
  });
  it('Escape cancels the log-out confirmation', async () => {
    const { user, backend } = renderApp({ route: '/profile' });
    await user.click(await screen.findByRole('button', { name: 'Log out' }));
    await user.keyboard('{Escape}');
    expect(screen.queryByRole('dialog')).toBeNull();
    expect(backend.callsTo('session.logout')).toHaveLength(0);
  });
  it('Cancel closes the confirmation and returns focus to Log out (fix round 1, Minor 5)', async () => {
    const { user } = renderApp({ route: '/profile' });
    const trigger = await screen.findByRole('button', { name: 'Log out' });
    await user.click(trigger);
    await user.click(within(screen.getByRole('dialog', { name: 'Log out of Kite?' })).getByRole('button', { name: 'Cancel' }));
    expect(screen.queryByRole('dialog')).toBeNull();
    expect(trigger).toHaveFocus();
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
    // Ruling 116 I1: the top-bar menu and this view share one load per user, so one rejection fails both.
    const refused = new Error(JSON.stringify({ code: 'network', stage: 'session', reason: 'Kite is unreachable' }));
    const spy = vi.spyOn(backend.session, 'profile').mockRejectedValueOnce(refused);
    const { user } = renderApp({ route: '/profile', backend });
    const note = await screen.findByText('Your profile could not be loaded from Kite.');
    expect(spy).toHaveBeenCalledTimes(1);
    expect(useToastStore.getState().visible.map((t) => t.level)).toEqual(['error']);
    // Minor 3: the notice mounts with its text, so it is not a live region.
    expect(note.closest('[role="status"]')).toBeNull();
    expect(screen.getByRole('button', { name: 'Log out' })).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'Retry' }));
    expect(await screen.findByLabelText('Name')).toHaveValue('Asha Rao');
  });
  it('Retry while a load is in flight adds no call, and Refresh shows it is busy (Ruling 116 I1, Minor 2)', async () => {
    const backend = createMockBackend(demoSeed());
    const refused = new Error(JSON.stringify({ code: 'network', stage: 'session', reason: 'Kite is unreachable' }));
    const pending = deferred<Profile>();
    const spy = vi.spyOn(backend.session, 'profile').mockRejectedValueOnce(refused).mockReturnValueOnce(pending.promise);
    const { user } = renderApp({ route: '/profile', backend });
    await screen.findByText('Your profile could not be loaded from Kite.');
    await user.click(screen.getByRole('button', { name: 'Retry' }));
    expect(spy).toHaveBeenCalledTimes(2);
    const refresh = screen.getByRole('button', { name: 'Refresh profile' });
    expect(refresh).toHaveAttribute('aria-busy', 'true');
    await user.click(refresh);
    expect(spy).toHaveBeenCalledTimes(2);
    await act(async () => pending.resolve(profileFixture));
    expect(await screen.findByLabelText('Name')).toHaveValue('Asha Rao');
    expect(refresh).not.toHaveAttribute('aria-busy');
  });
  it('concurrent loads for the same user share one Profile call and one promise (Ruling 116 I1)', async () => {
    const backend = createMockBackend(demoSeed());
    renderApp({ route: '/profile', backend });
    await screen.findByLabelText('Name');
    const pending = deferred<Profile>();
    const spy = vi.spyOn(backend.session, 'profile').mockReturnValueOnce(pending.promise);
    const first = loadProfile(backend);
    const second = loadProfile(backend);
    expect(second).toBe(first);
    expect(spy).toHaveBeenCalledTimes(1);
    await act(async () => {
      pending.resolve(profileFixture);
      await first;
    });
    void loadProfile(backend);
    expect(spy).toHaveBeenCalledTimes(2); // once settled, a new load is a new call
  });
  it('a failed Refresh keeps the shown profile and shows exactly one toast (fix round 1, Minor 5)', async () => {
    const backend = createMockBackend(demoSeed());
    const { user } = renderApp({ route: '/profile', backend });
    await screen.findByLabelText('Name');
    backend.failNext('session.profile', 'Kite is unreachable', 'network');
    await user.click(screen.getByRole('button', { name: 'Refresh profile' }));
    await waitFor(() => expect(useToastStore.getState().visible).toHaveLength(1));
    expect(useToastStore.getState().visible[0]).toMatchObject({ level: 'error', message: 'Your Kite profile could not be loaded: Kite is unreachable' });
    expect(screen.getByLabelText('Name')).toHaveValue('Asha Rao');
    expect(screen.queryByText('Your profile could not be loaded from Kite.')).toBeNull();
  });
  it('the confirmation closes when the session leaves ready and does not reopen on the next login (fix round 1, Minor 1)', async () => {
    const backend = createMockBackend(demoSeed());
    const { user } = renderApp({ route: '/profile', backend });
    await user.click(await screen.findByRole('button', { name: 'Log out' }));
    expect(screen.getByRole('dialog', { name: 'Log out of Kite?' })).toBeInTheDocument();
    act(() => backend.emit('session.state', readySession({ state: 'expired' })));
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull());
    act(() => backend.emit('session.state', readySession()));
    await screen.findByLabelText('Name');
    expect(screen.queryByRole('dialog')).toBeNull();
  });
  it('the client id and the broker chip are read as separate words (fix round 1, Minor 4)', async () => {
    renderApp({ route: '/profile' });
    const chip = await screen.findByText('Zerodha', { selector: '.chip' });
    expect(chip.parentElement?.textContent).toBe('AB1234 Zerodha');
  });
  it('an expired session says so in text instead of loading forever, and links to the login', async () => {
    const backend = createMockBackend(demoSeed());
    const { user } = renderApp({ route: '/profile', backend });
    await screen.findByLabelText('Name');
    act(() => backend.emit('session.state', readySession({ state: 'expired' })));
    const note = await screen.findByText('Your Kite session has ended. Log in again to see your profile.');
    expect(note.closest('[role="status"]')).toBeNull();
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
