import { act, screen, waitFor, within } from '@testing-library/react';
import type { UserEvent } from '@testing-library/user-event';
import { describe, expect, it } from 'vitest';
import type { CredentialsInfo } from '../../api/backend';
import { demoSeed, loggedOutSeed, readySession } from '../../api/mock/fixtures';
import { createMockBackend, type MockBackend } from '../../api/mock/index';
import { useShortcutStore } from '../../shortcuts/store';
import { useAppStore } from '../../store/app';
import { useNavStore } from '../../store/nav';
import { useSecretsNoticeStore } from '../../store/secretsNotice';
import { useSessionStore } from '../../store/session';
import { useSettingsStore } from '../../store/settings';
import { useStatusStore } from '../../store/status';
import { useTicksStore } from '../../store/ticks';
import { useToastStore } from '../../store/toasts';
import { setSystemDark } from '../../test/matchMedia';
import { renderApp } from '../../test/render';

const groupsNav = () => screen.findByRole('navigation', { name: 'Settings groups' });

const GO_INVALID_PORT =
  'Choose a port between 1024 and 65535. Then set the redirect URL of your app in the Kite developer console to http://127.0.0.1:<port>/kite/callback with that port.';
const LEFTOVER_WARNING = 'Old secrets file not removed';
// Go texts (internal/kite/credentials.go, session.go).
const GO_BAD_KEY = 'The API key should be 6–64 letters and digits. Copy it from your app in the Kite developer console.';
const GO_BAD_SECRET = 'The API secret should be 6–64 letters and digits. Copy it from your app in the Kite developer console.';
const GO_IDENTICAL = 'The API key and the API secret are identical. Paste each from its own field in the Kite developer console.';
const GO_SECRET_STORE = 'secret store not answering — unlock it and retry';

function fileBackedSeed() {
  const seed = demoSeed();
  seed.session = readySession({
    secrets: { mode: 'file', backend: 'file', keyringAvailable: true, fileExists: true, choices: [] },
    credentials: { ...readySession().credentials, secretBackend: 'file' },
  });
  return seed;
}

function loggingInSeed() {
  const seed = demoSeed();
  seed.session = readySession({ state: 'logging_in', sessionId: '' });
  return seed;
}

/** Every zustand store the app holds, serialised (functions drop out). */
function allStoreState(): string {
  return JSON.stringify([
    useAppStore.getState(), useNavStore.getState(), useSessionStore.getState(), useSettingsStore.getState(),
    useStatusStore.getState(), useTicksStore.getState(), useToastStore.getState(), useShortcutStore.getState(),
    useSecretsNoticeStore.getState(),
  ]);
}

function webStorage(): string {
  const dump = (s: Storage) => Array.from({ length: s.length }, (_, i) => `${s.key(i)}=${s.getItem(s.key(i) ?? '')}`).join('\n');
  return `${dump(window.localStorage)}\n${dump(window.sessionStorage)}`;
}

/** Opens the change dialog; while logged in it first confirms the logout (Ruling 97 I1). */
async function openChangeDialog(user: UserEvent) {
  await user.click(await screen.findByRole('button', { name: 'Change' }));
  const confirm = screen.queryByRole('dialog', { name: 'Change Kite credentials?' });
  if (confirm) await user.click(within(confirm).getByRole('button', { name: 'Log out and continue' }));
  return screen.getByRole('dialog', { name: 'Change Kite credentials' });
}

const sessionCalls = (backend: MockBackend) => backend.calls.map((c) => c.method).filter((m) => m === 'session.logout' || m === 'session.saveCredentials');
const loginRegion = () => screen.getByRole('region', { name: 'Login' });

describe('FR-ACCT-012 Settings view', () => {
  it('opens on Kite connection and hides groups whose features ship later (no "coming soon")', async () => {
    const { router } = renderApp({ route: '/settings' });
    expect(await screen.findByRole('heading', { level: 1, name: 'Kite connection' })).toBeInTheDocument();
    expect(router.state.location.pathname).toBe('/settings/kite');
    const nav = await groupsNav();
    for (const hidden of ['Trading', 'Home', 'Universes & baskets', 'Notifications', 'Limits', 'Updates', 'Tours']) {
      expect(within(nav).queryByRole('link', { name: hidden })).toBeNull();
    }
    for (const shown of ['Kite connection', 'Appearance', 'Chart', 'About']) {
      expect(within(nav).getByRole('link', { name: shown })).toBeInTheDocument();
    }
    expect(screen.queryByText(/coming soon/i)).toBeNull();
  });
  it('a hidden group cannot be opened by route', async () => {
    renderApp({ route: '/settings/trading' });
    expect(await screen.findByRole('heading', { name: 'Page not found' })).toBeInTheDocument();
  });
  it('an unknown group shows "Page not found"', async () => {
    renderApp({ route: '/settings/nonsense' });
    expect(await screen.findByRole('heading', { name: 'Page not found' })).toBeInTheDocument();
  });
});

describe('Settings → Kite connection (FR-ACCT-005/007/013)', () => {
  it('shows the masked API key (Session.Credentials), the detected data plan and the secret backend', async () => {
    const { backend } = renderApp({ route: '/settings/kite' });
    expect(await screen.findByText('••••9f2c')).toBeInTheDocument();
    expect(screen.getByText('Kite Connect (market data available)')).toBeInTheDocument();
    expect(screen.getByText('System keyring')).toBeInTheDocument();
    expect(backend.callsTo('session.credentials').length).toBeGreaterThan(0);
  });
  it('FR-ACCT-007: a Personal-plan key shows the degraded mode', async () => {
    const seed = demoSeed();
    seed.session = readySession({ dataPlan: 'none' });
    renderApp({ route: '/settings/kite', backend: createMockBackend(seed) });
    expect(await screen.findByText('Orders and portfolio only (no data plan)')).toBeInTheDocument();
  });
  it('FR-ACCT-012 AC2 pattern: an invalid port shows Go\'s inline error and keeps the old value; a valid one updates the redirect URL', async () => {
    const { user, backend } = renderApp({ route: '/settings/kite' });
    const port = await screen.findByLabelText('Login callback port');
    await user.clear(port);
    await user.type(port, '80');
    await user.tab();
    expect(await screen.findByText(GO_INVALID_PORT)).toBeInTheDocument();
    expect(port).toHaveValue('7420');
    expect(backend.state.session.credentials.port).toBe(7420);
    await user.clear(port);
    await user.type(port, '8080{Enter}');
    await waitFor(() => expect(backend.state.session.credentials.port).toBe(8080));
    expect(backend.callsTo('session.setRedirectPort').map((c) => c.args)).toEqual([[80], [8080]]);
    expect(await screen.findByDisplayValue('http://127.0.0.1:8080/kite/callback')).toBeInTheDocument();
  });
  it('P0-3 Task 10 carry-forward: after a port change it tells the user to update the Kite app redirect URL (always-mounted live region)', async () => {
    const { user } = renderApp({ route: '/settings/kite' });
    const port = await screen.findByLabelText('Login callback port');
    const live = within(loginRegion()).getByRole('status');
    expect(live).toBeEmptyDOMElement();
    await user.clear(port);
    await user.type(port, '8080{Enter}');
    const notice = await screen.findByRole('status', { name: 'Redirect URL changed' });
    expect(notice).toBe(live);
    expect(notice).toHaveTextContent(
      'Update the redirect URL of your app in the Kite developer console to http://127.0.0.1:8080/kite/callback, or the next login will fail.',
    );
  });
  it('M6: a port change during a login is refused with Go\'s login_in_progress reason and the old port is kept', async () => {
    const backend = createMockBackend(loggingInSeed());
    const { user } = renderApp({ route: '/settings/kite', backend });
    const port = await screen.findByLabelText('Login callback port');
    await user.clear(port);
    await user.type(port, '8080{Enter}');
    expect(
      await screen.findByText('Finish or cancel the current login before changing the port. Then set the redirect URL of your app in the Kite developer console to http://127.0.0.1:8080/kite/callback.'),
    ).toBeInTheDocument();
    expect(port).toHaveValue('7420');
    expect(backend.state.session.credentials.port).toBe(7420);
    expect(screen.queryByRole('status', { name: 'Redirect URL changed' })).toBeNull();
  });
  it('M2: Enter then blur while the port write is in flight commits once', async () => {
    const backend = createMockBackend(demoSeed());
    const original = backend.session.setRedirectPort;
    const releases: (() => void)[] = [];
    let invoked = 0;
    backend.session.setRedirectPort = async (p: number): Promise<CredentialsInfo> => {
      invoked += 1;
      await new Promise<void>((r) => releases.push(r));
      return original(p);
    };
    const { user } = renderApp({ route: '/settings/kite', backend });
    const port = await screen.findByLabelText('Login callback port');
    await user.clear(port);
    await user.type(port, '8080{Enter}');
    await user.tab();
    expect(invoked).toBe(1);
    await act(async () => releases.forEach((r) => r()));
    await waitFor(() => expect(backend.state.session.credentials.port).toBe(8080));
    expect(invoked).toBe(1);
  });
  it('FR-ACCT-005 AC3: Remove credentials → confirm → no credentials (setup flow)', async () => {
    const { user, backend } = renderApp({ route: '/settings/kite' });
    await user.click(await screen.findByRole('button', { name: 'Remove credentials' }));
    const dialog = screen.getByRole('dialog', { name: 'Remove Kite credentials?' });
    await user.click(within(dialog).getByRole('button', { name: 'Remove credentials' }));
    expect(backend.callsTo('session.removeCredentials')).toHaveLength(1);
    // The session gate (Task 9) turns no_credentials into the setup view.
    await waitFor(() => expect(useSessionStore.getState().session?.state).toBe('no_credentials'));
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull());
  });
  it('Ruling 97 I1: Change while logged in confirms, logs out, then saves the new credentials (Go refuses already_logged_in otherwise)', async () => {
    const { user, backend } = renderApp({ route: '/settings/kite' });
    await user.click(await screen.findByRole('button', { name: 'Change' }));
    const confirm = screen.getByRole('dialog', { name: 'Change Kite credentials?' });
    expect(confirm).toHaveTextContent('Changing the API key logs you out of Tradeloom; you will log in to Kite again.');
    await user.click(within(confirm).getByRole('button', { name: 'Log out and continue' }));
    const dialog = screen.getByRole('dialog', { name: 'Change Kite credentials' });
    expect(backend.callsTo('session.logout')).toHaveLength(0);
    await user.type(within(dialog).getByLabelText('API key'), 'newkey77aa');
    await user.type(within(dialog).getByLabelText('API secret'), 'newsecret42');
    await user.click(within(dialog).getByRole('button', { name: 'Save' }));
    expect(await screen.findByText('Kite credentials saved. Log in again to use them.')).toBeInTheDocument();
    expect(sessionCalls(backend)).toEqual(['session.logout', 'session.saveCredentials']);
    expect(backend.callsTo('session.saveCredentials')[0].args).toEqual([{ apiKey: 'newkey77aa', apiSecret: 'newsecret42' }]);
    await waitFor(() => expect(useSessionStore.getState().session?.state).toBe('logged_out'));
    expect(await screen.findByText('••••77aa')).toBeInTheDocument();
  });
  it('Ruling 97 I1: cancelling the logout confirmation neither logs out nor opens the form', async () => {
    const { user, backend } = renderApp({ route: '/settings/kite' });
    await user.click(await screen.findByRole('button', { name: 'Change' }));
    await user.click(within(screen.getByRole('dialog', { name: 'Change Kite credentials?' })).getByRole('button', { name: 'Cancel' }));
    expect(screen.queryByRole('dialog')).toBeNull();
    expect(backend.callsTo('session.logout')).toHaveLength(0);
    expect(useSessionStore.getState().session?.state).toBe('ready');
  });
  it('Ruling 97 I1: Change while logged out saves directly (no confirmation, no logout)', async () => {
    const backend = createMockBackend(loggedOutSeed());
    const { user } = renderApp({ route: '/settings/kite', backend });
    await user.click(await screen.findByRole('button', { name: 'Change' }));
    expect(screen.queryByRole('dialog', { name: 'Change Kite credentials?' })).toBeNull();
    const dialog = screen.getByRole('dialog', { name: 'Change Kite credentials' });
    await user.type(within(dialog).getByLabelText('API key'), 'newkey77aa');
    await user.type(within(dialog).getByLabelText('API secret'), 'newsecret42');
    await user.click(within(dialog).getByRole('button', { name: 'Save' }));
    expect(await screen.findByText('Kite credentials saved. Log in again to use them.')).toBeInTheDocument();
    expect(sessionCalls(backend)).toEqual(['session.saveCredentials']);
  });
  it('Ruling 97 I1: while a login is in progress Change is disabled and says why', async () => {
    renderApp({ route: '/settings/kite', backend: createMockBackend(loggingInSeed()) });
    const change = await screen.findByRole('button', { name: 'Change' });
    expect(change).toBeDisabled();
    expect(change).toHaveAccessibleDescription('Finish or cancel the current login before changing credentials.');
    expect(screen.getByText('Finish or cancel the current login before changing credentials.')).toBeVisible();
  });
  it('FR-ACCT-013 AC3 (UI part): moves secrets to the keyring and re-reads the credentials (Go publishes no session.state)', async () => {
    const { user, backend } = renderApp({ route: '/settings/kite', backend: createMockBackend(fileBackedSeed()) });
    expect(await screen.findByText('Encrypted file (secrets.enc)')).toBeInTheDocument();
    const reads = backend.callsTo('session.credentials').length;
    await user.click(screen.getByRole('button', { name: 'Move secrets to the system keyring' }));
    expect(backend.callsTo('session.migrateToKeyring')).toHaveLength(1);
    expect(await screen.findByText('System keyring')).toBeInTheDocument();
    expect(screen.queryByText('Encrypted file (secrets.enc)')).toBeNull();
    expect(backend.callsTo('session.credentials').length).toBeGreaterThan(reads);
  });
  it('Ruling 68: a file_not_removed refusal shows the warning and Retry removes the leftover file', async () => {
    const backend = createMockBackend(fileBackedSeed());
    backend.failNext('session.migrateToKeyring', 'The secrets were moved to the system keyring, but the old secrets file could not be removed. Try again to remove it.', 'file_not_removed');
    const { user } = renderApp({ route: '/settings/kite', backend });
    const move = await screen.findByRole('button', { name: 'Move secrets to the system keyring' });
    // As in Go: when the refusal returns, the keyring is already the backend and the old file is still there.
    backend.state.session.secrets = { mode: 'keyring', backend: 'keyring', keyringAvailable: true, fileExists: true, choices: [] };
    backend.state.session.credentials.secretBackend = 'keyring';
    await user.click(move);
    const warning = await screen.findByRole('alert', { name: LEFTOVER_WARNING });
    expect(warning).toHaveTextContent('Your secrets are now in the system keyring, but the old encrypted file (secrets.enc) is still on disk.');
    expect(await screen.findByText('System keyring')).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Move secrets to the system keyring' })).toBeNull();
    await user.click(within(warning).getByRole('button', { name: 'Try removing it again' }));
    expect(backend.callsTo('session.migrateToKeyring')).toHaveLength(2);
    await waitFor(() => expect(screen.queryByRole('alert', { name: LEFTOVER_WARNING })).toBeNull());
    expect(await screen.findByText('Secrets moved to the system keyring; the encrypted file was deleted.')).toBeInTheDocument();
  });
  it('Ruling 97 I4: the leftover-file warning outlives leaving Settings and a logout (run-lifetime store)', async () => {
    const backend = createMockBackend(fileBackedSeed());
    backend.failNext('session.migrateToKeyring', 'not removed', 'file_not_removed');
    const { user, router } = renderApp({ route: '/settings/kite', backend });
    const move = await screen.findByRole('button', { name: 'Move secrets to the system keyring' });
    backend.state.session.secrets = { mode: 'keyring', backend: 'keyring', keyringAvailable: true, fileExists: true, choices: [] };
    backend.state.session.credentials.secretBackend = 'keyring';
    await user.click(move);
    await screen.findByRole('alert', { name: LEFTOVER_WARNING });
    await act(() => router.navigate('/settings/appearance'));
    await screen.findByRole('heading', { level: 1, name: 'Appearance' });
    act(() => backend.emit('session.state', { ...backend.state.session, state: 'logged_out', sessionId: '' }));
    await act(() => router.navigate('/settings/kite'));
    const warning = await screen.findByRole('alert', { name: LEFTOVER_WARNING });
    expect(within(warning).getByRole('button', { name: 'Try removing it again' })).toBeInTheDocument();
  });
  it('Ruling 97 I4: a keyring backend with secrets.enc still on disk shows an informational line without a retry', async () => {
    const seed = demoSeed();
    seed.session = readySession({ secrets: { mode: 'keyring', backend: 'keyring', keyringAvailable: true, fileExists: true, choices: [] } });
    renderApp({ route: '/settings/kite', backend: createMockBackend(seed) });
    expect(
      await screen.findByText('An old encrypted secrets file (secrets.enc) is still on disk. Tradeloom uses the system keyring and does not read it.'),
    ).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Try removing it again' })).toBeNull();
    expect(screen.queryByRole('alert', { name: LEFTOVER_WARNING })).toBeNull();
  });
  it('Ruling 68: moving to the keyring is offered for a file backend even when the keyring was not detected; the refusal describes the button', async () => {
    const seed = fileBackedSeed();
    seed.session.secrets.keyringAvailable = false;
    const { user, backend } = renderApp({ route: '/settings/kite', backend: createMockBackend(seed) });
    const move = await screen.findByRole('button', { name: 'Move secrets to the system keyring' });
    await user.click(move);
    expect(backend.callsTo('session.migrateToKeyring')).toHaveLength(1);
    const reason = 'No system keyring is available. Unlock or start it, then try again.';
    expect(await screen.findByText(reason)).toBeInTheDocument();
    expect(move).toHaveAccessibleDescription(reason);
    expect(screen.getByText('Encrypted file (secrets.enc)')).toBeInTheDocument();
  });
  it('M3: a failed load shows one error toast per failure streak and a Retry', async () => {
    const backend = createMockBackend(demoSeed());
    const original = backend.session.credentials;
    let down = true;
    backend.session.credentials = async () => {
      if (down) throw new Error(JSON.stringify({ code: 'unavailable', stage: 'session', reason: 'Session service unavailable' }));
      return original();
    };
    const { user } = renderApp({ route: '/settings/kite', backend });
    const retry = await screen.findByRole('button', { name: 'Retry loading Kite connection' });
    expect(screen.getByText('The Kite connection details could not be loaded.')).toBeInTheDocument();
    const failToasts = () => [...useToastStore.getState().visible, ...useToastStore.getState().queued].filter((t) => t.message === 'Session service unavailable');
    await waitFor(() => expect(failToasts()).toHaveLength(1));
    act(() => backend.emit('session.state', { ...backend.state.session }));
    await user.click(retry);
    await waitFor(() => expect(backend.callsTo('session.secretStorage').length).toBeGreaterThanOrEqual(3));
    expect(failToasts()).toHaveLength(1);
    down = false;
    await user.click(screen.getByRole('button', { name: 'Retry loading Kite connection' }));
    expect(await screen.findByText('••••9f2c')).toBeInTheDocument();
  });
});

describe('Settings → Kite connection: the API secret stays out of state (security)', () => {
  it('the secret field is a non-autocompleting password field and never echoes the secret', async () => {
    const { user } = renderApp({ route: '/settings/kite' });
    const dialog = await openChangeDialog(user);
    const secret = within(dialog).getByLabelText('API secret');
    expect(secret).toHaveAttribute('type', 'password');
    expect(['off', 'new-password']).toContain(secret.getAttribute('autocomplete'));
    await user.type(secret, 'topsecretvalue');
    expect(document.body.textContent).not.toContain('topsecretvalue');
  });
  it('after a successful save the secret is in no store, no web storage, no URL and not in the mock persisted state; the field reopens empty', async () => {
    window.localStorage.clear();
    const backend = createMockBackend(demoSeed(), { persistKey: 'tl-test-settings' });
    const { user, router } = renderApp({ route: '/settings/kite', backend });
    let dialog = await openChangeDialog(user);
    await user.type(within(dialog).getByLabelText('API key'), 'newkey77aa');
    await user.type(within(dialog).getByLabelText('API secret'), 'S3cr3txyz123');
    await user.click(within(dialog).getByRole('button', { name: 'Save' }));
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull());
    expect(backend.callsTo('session.saveCredentials')).toHaveLength(1);
    expect(allStoreState()).not.toContain('S3cr3txyz123');
    expect(webStorage()).not.toContain('S3cr3txyz123');
    expect(JSON.stringify(backend.state)).not.toContain('S3cr3txyz123');
    expect(`${router.state.location.pathname}${router.state.location.search}${router.state.location.hash}`).not.toContain('S3cr3t');
    expect(document.body.innerHTML).not.toContain('S3cr3txyz123');
    dialog = await openChangeDialog(user);
    expect(within(dialog).getByLabelText('API secret')).toHaveValue('');
    window.localStorage.clear();
  });
  it('after a refused save the dialog stays open with a form-level alert describing both fields and an empty secret field; nothing kept anywhere', async () => {
    const backend = createMockBackend(loggedOutSeed());
    backend.failNext('session.saveCredentials', GO_SECRET_STORE, 'secret_store');
    const { user } = renderApp({ route: '/settings/kite', backend });
    const dialog = await openChangeDialog(user);
    await user.type(within(dialog).getByLabelText('API key'), 'newkey77aa');
    await user.type(within(dialog).getByLabelText('API secret'), 'S3cr3tfail9');
    await user.click(within(dialog).getByRole('button', { name: 'Save' }));
    const alert = await within(dialog).findByRole('alert');
    expect(alert).toHaveTextContent(GO_SECRET_STORE);
    expect(alert).not.toHaveTextContent('logged out');
    expect(within(dialog).getByLabelText('API key')).toHaveAccessibleDescription(GO_SECRET_STORE);
    expect(within(dialog).getByLabelText('API secret')).toHaveAccessibleDescription(GO_SECRET_STORE);
    expect(within(dialog).getByLabelText('API secret')).toHaveValue('');
    expect(within(dialog).getByLabelText('API key')).toHaveValue('newkey77aa');
    expect(allStoreState()).not.toContain('S3cr3tfail9');
    expect(webStorage()).not.toContain('S3cr3tfail9');
    expect(JSON.stringify(backend.state)).not.toContain('S3cr3tfail9');
  });
  it('fix round 2: an invalid secret is caught before the logout (Go format rules); nothing is called and the field is cleared', async () => {
    const { user, backend } = renderApp({ route: '/settings/kite' });
    const dialog = await openChangeDialog(user);
    await user.type(within(dialog).getByLabelText('API key'), 'newkey77aa');
    await user.type(within(dialog).getByLabelText('API secret'), 'bad-secret');
    await user.click(within(dialog).getByRole('button', { name: 'Save' }));
    const secret = within(dialog).getByLabelText('API secret');
    expect(await within(dialog).findByText(GO_BAD_SECRET)).toBeInTheDocument();
    expect(secret).toHaveAccessibleDescription(GO_BAD_SECRET);
    expect(secret).toBeInvalid();
    expect(secret).toHaveValue('');
    expect(sessionCalls(backend)).toEqual([]);
    expect(useSessionStore.getState().session?.state).toBe('ready');
  });
  it('fix round 2: an invalid key, or a key equal to the secret, is caught before the logout', async () => {
    const { user, backend } = renderApp({ route: '/settings/kite' });
    const dialog = await openChangeDialog(user);
    const key = within(dialog).getByLabelText('API key');
    await user.type(key, 'abc');
    await user.type(within(dialog).getByLabelText('API secret'), 'newsecret42');
    await user.click(within(dialog).getByRole('button', { name: 'Save' }));
    expect(await within(dialog).findByText(GO_BAD_KEY)).toBeInTheDocument();
    expect(key).toHaveAccessibleDescription(GO_BAD_KEY);
    await user.clear(key);
    await user.type(key, 'same12345');
    await user.type(within(dialog).getByLabelText('API secret'), ' "same12345" ');
    await user.click(within(dialog).getByRole('button', { name: 'Save' }));
    expect(await within(dialog).findByText(GO_IDENTICAL)).toBeInTheDocument();
    expect(sessionCalls(backend)).toEqual([]);
    expect(useSessionStore.getState().session?.state).toBe('ready');
  });
  it('fix round 2: a refusal after a successful logout says the user is logged out and the old credentials are kept; the secret is cleared', async () => {
    const backend = createMockBackend(demoSeed());
    backend.failNext('session.saveCredentials', GO_SECRET_STORE, 'secret_store');
    const { user } = renderApp({ route: '/settings/kite', backend });
    const dialog = await openChangeDialog(user);
    await user.type(within(dialog).getByLabelText('API key'), 'newkey77aa');
    await user.type(within(dialog).getByLabelText('API secret'), 'S3cr3trace1');
    await user.click(within(dialog).getByRole('button', { name: 'Save' }));
    const alert = await within(dialog).findByRole('alert');
    expect(alert).toHaveTextContent('You are logged out; your previous credentials are still saved.');
    expect(alert).toHaveTextContent(GO_SECRET_STORE);
    expect(sessionCalls(backend)).toEqual(['session.logout', 'session.saveCredentials']);
    expect(useSessionStore.getState().session?.state).toBe('logged_out');
    expect(backend.state.session.credentials.apiKeyMasked).toBe('••••9f2c');
    expect(within(dialog).getByLabelText('API secret')).toHaveValue('');
    expect(allStoreState()).not.toContain('S3cr3trace1');
  });
  it('cancelling the dialog discards the typed secret', async () => {
    const backend = createMockBackend(loggedOutSeed());
    const { user } = renderApp({ route: '/settings/kite', backend });
    let dialog = await openChangeDialog(user);
    await user.type(within(dialog).getByLabelText('API secret'), 'discardme1');
    await user.click(within(dialog).getByRole('button', { name: 'Cancel' }));
    dialog = await openChangeDialog(user);
    expect(within(dialog).getByLabelText('API secret')).toHaveValue('');
  });
});

describe('Settings → Appearance and Chart', () => {
  it('FR-SHELL-011: "Follow system theme" tracks the desktop preference', async () => {
    const { user } = renderApp({ route: '/settings/appearance' });
    await user.click(await screen.findByRole('switch', { name: 'Follow system theme' }));
    act(() => setSystemDark(true));
    await waitFor(() => expect(document.documentElement.dataset.theme).toBe('dark'));
    expect(screen.getByRole('switch', { name: 'Dark theme' })).toBeDisabled();
  });
  it('the dark-theme and collapsed-sidebar switches write their settings', async () => {
    const { user, backend } = renderApp({ route: '/settings/appearance' });
    await user.click(await screen.findByRole('switch', { name: 'Dark theme' }));
    await waitFor(() => expect(backend.state.settings['appearance.theme']).toBe('"dark"'));
    await user.click(screen.getByRole('switch', { name: 'Collapse the sidebar to icons' }));
    await waitFor(() => expect(backend.state.settings['appearance.sidebar_collapsed']).toBe('true'));
  });
  it('Ruling 97 I3: a refused appearance write toasts the reason and the switch rolls back', async () => {
    const backend = createMockBackend(demoSeed());
    const { user } = renderApp({ route: '/settings/appearance', backend });
    const dark = await screen.findByRole('switch', { name: 'Dark theme' });
    backend.failNext('settings.set', 'Settings database is read-only', 'invalid');
    await user.click(dark);
    expect(await screen.findByText('The theme could not be saved: Settings database is read-only')).toBeInTheDocument();
    await waitFor(() => expect(dark).toHaveAttribute('aria-checked', 'false'));
    backend.failNext('settings.set', 'Settings database is read-only', 'invalid');
    await user.click(screen.getByRole('switch', { name: 'Follow system theme' }));
    expect(await screen.findByText('The setting could not be saved: Settings database is read-only')).toBeInTheDocument();
  });
  it('stores the default chart timeframe as a Kite interval id', async () => {
    const { user, backend } = renderApp({ route: '/settings/chart' });
    const select = await screen.findByLabelText('Default chart timeframe');
    expect(select).toHaveValue('60minute');
    await user.selectOptions(select, '15minute');
    await waitFor(() => expect(backend.state.settings['chart.default_timeframe']).toBe('"15minute"'));
  });
  it('Ruling 64: Settings never writes the backend-owned window.geometry UI state', async () => {
    const { user, backend } = renderApp({ route: '/settings/appearance' });
    await user.click(await screen.findByRole('switch', { name: 'Collapse the sidebar to icons' }));
    await user.click(screen.getByRole('link', { name: 'Chart' }));
    await screen.findByLabelText('Default chart timeframe');
    expect(backend.callsTo('settings.setUiState').filter((c) => c.args[0] === 'window.geometry')).toEqual([]);
    expect(backend.callsTo('settings.set').filter((c) => String(c.args[0]).startsWith('window.'))).toEqual([]);
  });
});

describe('Settings → About', () => {
  it('AC3: the version equals the profile-menu version and third-party licences are listed', async () => {
    const { user } = renderApp({ route: '/settings/about' });
    expect(await screen.findByText('v0.1.0')).toBeInTheDocument();
    expect(screen.getByRole('row', { name: /lightweight-charts 5\.2\.1 Apache-2\.0/ })).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'Account menu' }));
    expect(within(screen.getByRole('menu')).getByText('v0.1.0')).toBeInTheDocument();
  });
  it('links to the legal pages and the risk disclosure', async () => {
    renderApp({ route: '/settings/about' });
    expect(await screen.findByRole('link', { name: 'Terms of use' })).toHaveAttribute('href', '/legal/terms');
    expect(screen.getByRole('link', { name: 'Privacy notice' })).toHaveAttribute('href', '/legal/privacy');
    expect(screen.getByRole('link', { name: 'Risk disclosure' })).toHaveAttribute('href', '/legal/risk');
  });
});

describe('P0-4 Task 9 fix round 1 (Ruling 108 M12)', () => {
  it('Change while logged in logs out through logOut(): a failed remote invalidation is a gentle info note', async () => {
    const msg = 'Logged out of Tradeloom. Kite could not be reached to invalidate the token; it stops working at 06:00 IST.';
    const backend = createMockBackend(demoSeed());
    const realLogout = backend.session.logout;
    backend.session.logout = async () => {
      await realLogout();
      return { remoteInvalidated: false, message: msg };
    };
    const { user } = renderApp({ route: '/settings/kite', backend });
    const dialog = await openChangeDialog(user);
    await user.type(within(dialog).getByLabelText('API key'), 'newkey77aa');
    await user.type(within(dialog).getByLabelText('API secret'), 'newsecret42');
    await user.click(within(dialog).getByRole('button', { name: 'Save' }));
    expect(await screen.findByText('Kite credentials saved. Log in again to use them.')).toBeInTheDocument();
    expect(useToastStore.getState().visible.find((t) => t.message === msg)?.level).toBe('info');
  });
});
