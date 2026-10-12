import { act, screen, waitFor } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import {
  demoSeed, freshSeed, lockedSeed, loggedOutSeed, noKeyringSeed, readySession, unackedSeed,
} from '../../api/mock/fixtures';
import { createMockBackend, type MockState } from '../../api/mock/index';
import { useAppStore } from '../../store/app';
import { resetAllStores } from '../../store/reset';
import { renderApp } from '../../test/render';

const start = (seed: MockState) => renderApp({ route: '/', backend: createMockBackend(seed) });

// Go service.msgBadPassphrase (internal/service/secrets.go) and kite.ParseRedirect's "no request_token" text
// (internal/kite/redirect.go); the mock mirrors both (Ruling 101: show Go's reason verbatim).
const BAD_PASSPHRASE = 'Unlock failed: wrong passphrase or damaged file. Check the passphrase and try again.';
const NO_REQUEST_TOKEN = "No request_token found in the pasted text. After logging in, copy the whole address from the browser's address bar.";

describe('FR-ACCT-005 credentials setup', () => {
  it('AC1: a fresh install shows setup first, with the exact redirect URL to register', async () => {
    start(freshSeed());
    expect(await screen.findByRole('heading', { name: 'Connect your Kite account' })).toBeInTheDocument();
    expect(screen.getByLabelText('Redirect URL')).toHaveValue('http://127.0.0.1:7420/kite/callback');
    expect(screen.getByLabelText('Redirect URL')).toHaveAttribute('readonly');
    expect(screen.queryByRole('navigation', { name: 'Main navigation' })).toBeNull();
  });
  it('validates inline, masks the secret and moves on to login after saving', async () => {
    const { user, backend } = start(freshSeed());
    await user.click(await screen.findByRole('button', { name: 'Save and continue' }));
    expect(screen.getByText('Enter the API key')).toBeInTheDocument();
    expect(screen.getByText('Enter the API secret')).toBeInTheDocument();
    expect(backend.callsTo('session.saveCredentials')).toHaveLength(0);
    expect(screen.getByLabelText('API secret')).toHaveAttribute('type', 'password');
    await user.type(screen.getByLabelText('API key'), 'kiteapikey9f2c');
    await user.type(screen.getByLabelText('API secret'), 's3cretvalue9');
    await user.click(screen.getByRole('button', { name: 'Save and continue' }));
    expect(backend.callsTo('session.saveCredentials')[0].args).toEqual([{ apiKey: 'kiteapikey9f2c', apiSecret: 's3cretvalue9' }]);
    expect(await screen.findByRole('heading', { name: 'Log in to Kite' })).toBeInTheDocument();
  });
  it('copies the redirect URL', async () => {
    const { user } = start(freshSeed());
    await user.click(await screen.findByRole('button', { name: 'Copy' }));
    expect(await screen.findByText('Redirect URL copied')).toBeInTheDocument();
    await expect(navigator.clipboard.readText()).resolves.toBe('http://127.0.0.1:7420/kite/callback');
  });
  it('a refused save shows Go\'s reason inline and as a toast and stays on setup', async () => {
    const backend = createMockBackend(freshSeed());
    backend.failNext('session.saveCredentials', 'The API key and the API secret are identical. Paste each from its own field in the Kite developer console.', 'invalid_credentials');
    const { user } = renderApp({ route: '/', backend });
    await user.type(await screen.findByLabelText('API key'), 'kiteapikey9f2c');
    await user.type(screen.getByLabelText('API secret'), 's3cretvalue9');
    await user.click(screen.getByRole('button', { name: 'Save and continue' }));
    expect(await screen.findAllByText(/The API key and the API secret are identical/)).toHaveLength(2);
    expect(screen.getByRole('heading', { name: 'Connect your Kite account' })).toBeInTheDocument();
  });
  it('with the encrypted-file backend the copy does not claim the keyring', async () => {
    const seed = freshSeed();
    seed.session.secrets = { mode: 'file', backend: 'file', keyringAvailable: false, fileExists: true, choices: [] };
    start(seed);
    await screen.findByRole('heading', { name: 'Connect your Kite account' });
    expect(screen.queryByText(/system keyring/)).toBeNull();
    expect(screen.getByText(/encrypted secrets file/)).toBeInTheDocument();
  });
});

describe('FR-ACCT-013 secret storage fallback (UI)', () => {
  it('AC1: without a keyring exactly the two choices Go offers are shown and no plain storage', async () => {
    const { backend } = start(noKeyringSeed());
    await screen.findByRole('heading', { name: 'Choose where to keep your Kite secret' });
    expect((await screen.findAllByRole('radio')).map((r) => r.getAttribute('value'))).toEqual(['encrypted_file', 'no_storage']);
    expect(backend.callsTo('session.secretStorage')).toHaveLength(1);
    expect(screen.getByRole('radio', { name: 'Encrypted file protected by a passphrase' })).toBeChecked();
    expect(screen.getByRole('radio', { name: "Don't store credentials" })).toBeInTheDocument();
    expect(screen.queryByText(/settings file|plain text/i)).toBeNull();
  });
  it('checks the passphrase inline, then sets up the encrypted file', async () => {
    const { user, backend } = start(noKeyringSeed());
    await user.type(await screen.findByLabelText('Passphrase'), 'short');
    await user.type(screen.getByLabelText('Repeat passphrase'), 'short');
    await user.click(screen.getByRole('button', { name: 'Continue' }));
    expect(screen.getByText('Use at least 8 characters')).toBeInTheDocument();
    await user.clear(screen.getByLabelText('Passphrase'));
    await user.type(screen.getByLabelText('Passphrase'), 'correct horse');
    await user.click(screen.getByRole('button', { name: 'Continue' }));
    expect(screen.getByText('The passphrases do not match')).toBeInTheDocument();
    await user.clear(screen.getByLabelText('Repeat passphrase'));
    await user.type(screen.getByLabelText('Repeat passphrase'), 'correct horse');
    await user.click(screen.getByRole('button', { name: 'Continue' }));
    expect(backend.callsTo('session.useEncryptedFile')[0].args).toEqual(['correct horse']);
    expect(await screen.findByRole('heading', { name: 'Connect your Kite account' })).toBeInTheDocument();
  });
  it('refusing storage explains that Kite cannot be used and stays on setup', async () => {
    const { user } = start(noKeyringSeed());
    await user.click(await screen.findByRole('radio', { name: "Don't store credentials" }));
    await user.click(screen.getByRole('button', { name: 'Continue' }));
    expect(await screen.findByText(/Credentials are not stored/)).toBeInTheDocument();
  });
  it('refusing storage updates the view from the returned status (Go publishes no session.state for it)', async () => {
    const { user, backend } = start(noKeyringSeed());
    await user.click(await screen.findByRole('radio', { name: "Don't store credentials" }));
    await user.click(screen.getByRole('button', { name: 'Continue' }));
    expect(await screen.findByText(/Credentials are not stored/)).toBeInTheDocument();
    expect(backend.state.session.secrets.mode).toBe('none');
    // Still on setup and the encrypted file can be chosen after all (Go allows UseEncryptedFile after a refusal).
    expect(screen.getByRole('radio', { name: 'Encrypted file protected by a passphrase' })).toBeInTheDocument();
  });
  it('after a refusal in this run (mode "none", Go offers no choices) the encrypted file is still offered', async () => {
    const seed = noKeyringSeed();
    seed.session.secrets = { ...seed.session.secrets, mode: 'none', choices: [] };
    start(seed);
    expect(await screen.findByText(/Credentials are not stored/)).toBeInTheDocument();
    expect((await screen.findAllByRole('radio')).map((r) => r.getAttribute('value'))).toEqual(['encrypted_file']);
  });
  it('a locked encrypted file asks for the passphrase and rejects a wrong one inline with Go\'s reason', async () => {
    const { user } = start(lockedSeed());
    await user.type(await screen.findByLabelText('Passphrase'), 'wrong passphrase');
    await user.click(screen.getByRole('button', { name: 'Unlock' }));
    expect(await screen.findByText(BAD_PASSPHRASE)).toBeInTheDocument();
    await user.clear(screen.getByLabelText('Passphrase'));
    await user.type(screen.getByLabelText('Passphrase'), 'correct horse');
    await user.click(screen.getByRole('button', { name: 'Unlock' }));
    expect(await screen.findByRole('heading', { name: 'Log in to Kite' })).toBeInTheDocument();
  });
});

describe('FR-ACCT-006 login (UI)', () => {
  it('Log in with Kite starts the browser login and waits', async () => {
    const { user, backend } = start(loggedOutSeed());
    await user.click(await screen.findByRole('button', { name: 'Log in with Kite' }));
    expect(backend.callsTo('session.login')).toHaveLength(1);
    expect(await screen.findByText(/Waiting for the Kite login/)).toBeInTheDocument();
    act(() => backend.simulateBrowserLogin());
    expect(await screen.findByRole('heading', { name: 'Before you start' })).toBeInTheDocument();
  });
  it('shows the redirect URL Kite must send the browser back to', async () => {
    start(loggedOutSeed());
    await screen.findByRole('heading', { name: 'Log in to Kite' });
    expect(screen.getByText(/http:\/\/127\.0\.0\.1:7420\/kite\/callback/)).toBeInTheDocument();
  });
  it('links to Settings → Kite connection, which the gate allows while logged out (port or credentials fixes)', async () => {
    const { user, router } = start(loggedOutSeed());
    await user.click(await screen.findByRole('link', { name: 'Change the login port or the Kite credentials' }));
    await waitFor(() => expect(router.state.location.pathname).toBe('/settings/kite'));
    expect(await screen.findByLabelText('Login callback port')).toBeInTheDocument();
  });
  it('Cancel login returns to the logged-out state', async () => {
    const { user, backend } = start(loggedOutSeed());
    await user.click(await screen.findByRole('button', { name: 'Log in with Kite' }));
    await user.click(await screen.findByRole('button', { name: 'Cancel login' }));
    expect(backend.callsTo('session.cancelLogin')).toHaveLength(1);
    expect(backend.state.session.state).toBe('logged_out');
    expect(screen.queryByText(/Waiting for the Kite login/)).toBeNull();
  });
  it('AC2: the paste fallback completes login; bad input is explained inline and as a toast', async () => {
    const { user } = start(loggedOutSeed());
    await user.click(await screen.findByRole('button', { name: 'Login did not come back to Tradeloom?' }));
    const field = screen.getByLabelText('Redirect URL or request token');
    await user.type(field, 'not a url');
    await user.click(screen.getByRole('button', { name: 'Complete login' }));
    expect(await screen.findAllByText(NO_REQUEST_TOKEN)).toHaveLength(2);
    await user.clear(field);
    await user.type(field, 'http://127.0.0.1:7420/kite/callback?request_token=abc123XYZ&status=success');
    await user.click(screen.getByRole('button', { name: 'Complete login' }));
    expect(await screen.findByRole('heading', { name: 'Before you start' })).toBeInTheDocument();
  });
  it('an empty paste is explained inline without calling Go', async () => {
    const { user, backend } = start(loggedOutSeed());
    await user.click(await screen.findByRole('button', { name: 'Login did not come back to Tradeloom?' }));
    await user.click(screen.getByRole('button', { name: 'Complete login' }));
    expect(screen.getByText('Paste the redirect URL or the request token')).toBeInTheDocument();
    expect(backend.callsTo('session.completeLogin')).toHaveLength(0);
  });
  it('when the loopback listener cannot start, shows its error and opens the paste field', async () => {
    const seed = loggedOutSeed();
    seed.loginListenerOk = false;
    const { user } = start(seed);
    await user.click(await screen.findByRole('button', { name: 'Log in with Kite' }));
    expect(await screen.findByText('Port 7420 is already in use by another program')).toBeInTheDocument();
    expect(screen.getByLabelText('Redirect URL or request token')).toBeInTheDocument();
    expect(screen.queryByText(/Waiting for the Kite login/)).toBeNull();
  });
  it('a login that Go ends with an error (listener timeout) shows Go\'s message and opens the paste field', async () => {
    const { user, backend } = start(loggedOutSeed());
    await user.click(await screen.findByRole('button', { name: 'Log in with Kite' }));
    await screen.findByText(/Waiting for the Kite login/);
    const timeout = 'The login window timed out. If you already logged in, paste the address from the browser\'s address bar below; otherwise click "Log in with Kite" again.';
    act(() => backend.emit('session.state', { ...backend.state.session, state: 'logged_out', errorCode: 'listener_timeout', errorMessage: timeout }));
    expect(await screen.findByText(timeout)).toBeInTheDocument();
    expect(screen.queryByText(/Waiting for the Kite login/)).toBeNull();
    expect(screen.getByLabelText('Redirect URL or request token')).toBeInTheDocument();
  });
  it('AC3 (UI): an expiry while running shows the banner; Log in leads to the login view', async () => {
    const { user, backend } = renderApp({ route: '/home' });
    await screen.findByRole('button', { name: 'Account menu' });
    act(() => backend.emit('session.state', readySession({ state: 'expired' })));
    expect(await screen.findByText('Kite session expired — log in to continue')).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'Log in' }));
    expect(await screen.findByText('Your Kite session has ended. Log in to continue.')).toBeInTheDocument();
  });
});

describe('FR-SHELL-020 risk disclosure', () => {
  it('AC1: shown after login before Home; "I understand" opens Home', async () => {
    const { user, backend, router } = start(unackedSeed());
    expect(await screen.findByRole('heading', { name: 'Before you start' })).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Terms of use' })).toHaveAttribute('href', '/legal/terms');
    await user.click(screen.getByRole('button', { name: 'I understand' }));
    expect(backend.callsTo('session.ackRiskDisclosure')).toHaveLength(1);
    await waitFor(() => expect(router.state.location.pathname).toBe('/home'));
  });
  it('AC3: a restart with an acknowledged token does not show it', async () => {
    const { router } = start(demoSeed());
    await waitFor(() => expect(router.state.location.pathname).toBe('/home'));
    expect(screen.queryByRole('heading', { name: 'Before you start' })).toBeNull();
  });
});

describe('FR-SHELL-019 AC3 (frontend part): reopen on the last view', () => {
  // Deviation: the plan restores "/positions"; Positions (Task 11) is not on this branch, so the test uses a view
  // that exists (Ruling 94 M4: only routes this build has are restored).
  it('restores ui.last_route on start', async () => {
    const seed = demoSeed();
    seed.settings = { 'ui.last_route': '"/holdings"' };
    const { router } = start(seed);
    await waitFor(() => expect(router.state.location.pathname).toBe('/holdings'));
  });
  it('restores a Settings group (Task 14 routes are in the allow-list and exist)', async () => {
    const seed = demoSeed();
    seed.settings = { 'ui.last_route': '"/settings/calendar"' };
    const { router } = start(seed);
    await waitFor(() => expect(router.state.location.pathname).toBe('/settings/calendar'));
  });
  it('falls back to Home for a view this build does not have', async () => {
    const seed = demoSeed();
    seed.settings = { 'ui.last_route': '"/scalper"' };
    const { router } = start(seed);
    await waitFor(() => expect(router.state.location.pathname).toBe('/home'));
  });
  it('falls back to Home for an allow-listed view whose route is not in this build (Ruling 94 M4)', async () => {
    const seed = demoSeed();
    seed.settings = { 'ui.last_route': '"/profile"' };
    const { router } = start(seed);
    await waitFor(() => expect(router.state.location.pathname).toBe('/home'));
  });
  it('opens Home when settings could not be loaded (the shell shows the boot notice)', async () => {
    const backend = createMockBackend(demoSeed());
    backend.failNext('settings.all', 'settings unavailable');
    const { router } = renderApp({ route: '/', backend });
    await waitFor(() => expect(router.state.location.pathname).toBe('/home'));
    expect(await screen.findByText('Some app data could not be loaded')).toBeInTheDocument();
  });
});

describe('boot failures before the shell (Task 7 → Task 9 carry-forward)', () => {
  it('a failed session load shows the boot notice with Retry instead of an endless "Starting…"', async () => {
    const backend = createMockBackend(freshSeed());
    backend.failNext('session.status', 'session unavailable');
    const { user } = renderApp({ route: '/', backend });
    expect(await screen.findByText('Some app data could not be loaded')).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'Retry' }));
    expect(await screen.findByRole('heading', { name: 'Connect your Kite account' })).toBeInTheDocument();
  });
  it('the onboarding layout shows the boot notice too', async () => {
    const backend = createMockBackend(freshSeed());
    backend.failNext('system.info', 'info unavailable');
    renderApp({ route: '/', backend });
    expect(await screen.findByRole('heading', { name: 'Connect your Kite account' })).toBeInTheDocument();
    expect(await screen.findByText('Some app data could not be loaded')).toBeInTheDocument();
    expect(useAppStore.getState().bootError?.failed).toEqual(['info']);
  });
});

describe('fix round 1 (Ruling 108)', () => {
  it('I1: the step heading takes focus on start and whenever the step changes', async () => {
    const { user } = start(noKeyringSeed());
    const storeHeading = await screen.findByRole('heading', { name: 'Choose where to keep your Kite secret' });
    await waitFor(() => expect(storeHeading).toHaveFocus());
    expect(storeHeading).toHaveAttribute('tabindex', '-1');
    await user.type(screen.getByLabelText('Passphrase'), 'correct horse');
    await user.type(screen.getByLabelText('Repeat passphrase'), 'correct horse');
    await user.click(screen.getByRole('button', { name: 'Continue' }));
    const credsHeading = await screen.findByRole('heading', { name: 'Connect your Kite account' });
    await waitFor(() => expect(credsHeading).toHaveFocus());
  });
  it('I1: a failed credentials check focuses the first invalid field', async () => {
    const { user } = start(freshSeed());
    await user.click(await screen.findByRole('button', { name: 'Save and continue' }));
    expect(screen.getByLabelText('API key')).toHaveFocus();
    await user.type(screen.getByLabelText('API key'), 'kiteapikey9f2c');
    await user.click(screen.getByRole('button', { name: 'Save and continue' }));
    expect(screen.getByLabelText('API secret')).toHaveFocus();
  });
  it('I1: a failed passphrase check focuses the first invalid passphrase field', async () => {
    const { user } = start(noKeyringSeed());
    await user.type(await screen.findByLabelText('Passphrase'), 'short');
    await user.click(screen.getByRole('button', { name: 'Continue' }));
    expect(screen.getByLabelText('Passphrase')).toHaveFocus();
    await user.clear(screen.getByLabelText('Passphrase'));
    await user.type(screen.getByLabelText('Passphrase'), 'correct horse');
    await user.click(screen.getByRole('button', { name: 'Continue' }));
    expect(screen.getByLabelText('Repeat passphrase')).toHaveFocus();
  });
  it('I1: an empty paste focuses the paste field', async () => {
    const { user } = start(loggedOutSeed());
    await user.click(await screen.findByRole('button', { name: 'Login did not come back to Tradeloom?' }));
    await user.click(screen.getByRole('button', { name: 'Complete login' }));
    expect(screen.getByLabelText('Redirect URL or request token')).toHaveFocus();
  });
  it.each(['session_expired', 'token_rejected'])('I2: an expired session shows exactly one expiry message (errorCode %s)', async (errorCode) => {
    const seed = demoSeed();
    seed.session = readySession({ state: 'expired', errorCode, errorMessage: 'Kite session expired — log in to continue.' });
    renderApp({ route: '/login', backend: createMockBackend(seed) });
    await screen.findByRole('heading', { name: 'Log in to Kite' });
    expect(screen.queryByText('Kite session expired — log in to continue.')).toBeNull();
    expect(screen.getAllByText(/session (has ended|expired)/i)).toHaveLength(1);
    expect(screen.queryAllByRole('alert')).toHaveLength(0);
  });
  it('I2: Go\'s session error is also dropped for session_expired/token_rejected outside the expired state', async () => {
    const seed = loggedOutSeed();
    seed.session = { ...seed.session, errorCode: 'token_rejected', errorMessage: 'Kite rejected the access token.' };
    start(seed);
    await screen.findByRole('heading', { name: 'Log in to Kite' });
    expect(screen.queryByText('Kite rejected the access token.')).toBeNull();
  });
  it('M5: a refused credentials save is tied to both fields (aria-describedby + aria-invalid)', async () => {
    const backend = createMockBackend(freshSeed());
    backend.failNext('session.saveCredentials', 'Kite rejected the API key', 'invalid_credentials');
    const { user } = renderApp({ route: '/', backend });
    await user.type(await screen.findByLabelText('API key'), 'kiteapikey9f2c');
    await user.type(screen.getByLabelText('API secret'), 's3cretvalue9');
    await user.click(screen.getByRole('button', { name: 'Save and continue' }));
    const alert = (await screen.findAllByText('Kite rejected the API key')).find((el) => el.getAttribute('role') === 'alert');
    expect(alert?.id).toBeTruthy();
    for (const label of ['API key', 'API secret']) {
      expect(screen.getByLabelText(label)).toHaveAttribute('aria-invalid', 'true');
      expect(screen.getByLabelText(label).getAttribute('aria-describedby')?.split(' ')).toContain(alert?.id);
    }
  });
  it('M5: a refused secret-storage set-up is tied to both passphrase fields', async () => {
    const backend = createMockBackend(noKeyringSeed());
    backend.failNext('session.useEncryptedFile', 'A secrets file already exists; unlock it with its passphrase instead.', 'file_exists');
    const { user } = renderApp({ route: '/', backend });
    await user.type(await screen.findByLabelText('Passphrase'), 'correct horse');
    await user.type(screen.getByLabelText('Repeat passphrase'), 'correct horse');
    await user.click(screen.getByRole('button', { name: 'Continue' }));
    const alert = (await screen.findAllByText(/A secrets file already exists/)).find((el) => el.getAttribute('role') === 'alert');
    expect(alert?.id).toBeTruthy();
    for (const label of ['Passphrase', 'Repeat passphrase']) {
      expect(screen.getByLabelText(label)).toHaveAttribute('aria-invalid', 'true');
      expect(screen.getByLabelText(label).getAttribute('aria-describedby')?.split(' ')).toContain(alert?.id);
    }
  });
  it('M7: a remembered Settings group is restored only if this build shows it', async () => {
    const hidden = demoSeed();
    hidden.settings = { 'ui.last_route': '"/settings/trading"' };
    const first = start(hidden);
    await waitFor(() => expect(first.router.state.location.pathname).toBe('/home'));
    first.unmount();
    resetAllStores();
    const shown = demoSeed();
    shown.settings = { 'ui.last_route': '"/settings/kite"' };
    const second = start(shown);
    await waitFor(() => expect(second.router.state.location.pathname).toBe('/settings/kite'));
  });
  it('M8: while waiting for the browser, Log in with Kite is disabled and Cancel stays available', async () => {
    const { user } = start(loggedOutSeed());
    await user.click(await screen.findByRole('button', { name: 'Log in with Kite' }));
    await screen.findByText(/Waiting for the Kite login/);
    expect(screen.getByRole('button', { name: 'Log in with Kite' })).toBeDisabled();
    expect(screen.getByRole('button', { name: 'Cancel login' })).toBeEnabled();
    await waitFor(() => expect(screen.getByRole('button', { name: 'Cancel login' })).toHaveFocus());
  });
  it('M9: arriving with a listener timeout already on the session opens the paste field', async () => {
    const seed = loggedOutSeed();
    seed.session = { ...seed.session, errorCode: 'listener_timeout', errorMessage: 'The login window timed out.' };
    start(seed);
    expect(await screen.findByLabelText('Redirect URL or request token')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Login did not come back to Tradeloom?' })).toHaveAttribute('aria-expanded', 'true');
  });
  it('M10: the paste toggle controls the paste form', async () => {
    const { user } = start(loggedOutSeed());
    const toggle = await screen.findByRole('button', { name: 'Login did not come back to Tradeloom?' });
    const id = toggle.getAttribute('aria-controls');
    expect(id).toBeTruthy();
    await user.click(toggle);
    expect(document.getElementById(id ?? '')).toContainElement(screen.getByLabelText('Redirect URL or request token'));
  });
  it('M11: mock refusals use Go\'s texts', async () => {
    const m = createMockBackend(noKeyringSeed());
    await expect(m.session.useEncryptedFile('short')).rejects.toThrow('Use a passphrase of at least 8 characters.');
    await expect(m.session.saveCredentials({ apiKey: 'kiteapikey9f2c', apiSecret: 's3cretvalue9' }))
      .rejects.toThrow('secret store not answering — unlock it and retry');
  });
});

describe('fix round 2 (focus when waiting ends)', () => {
  it('I1: Cancel login returns focus to Log in with Kite', async () => {
    const { user } = start(loggedOutSeed());
    await user.click(await screen.findByRole('button', { name: 'Log in with Kite' }));
    await waitFor(() => expect(screen.getByRole('button', { name: 'Cancel login' })).toHaveFocus());
    await user.click(screen.getByRole('button', { name: 'Cancel login' }));
    await waitFor(() => expect(screen.queryByRole('button', { name: 'Cancel login' })).toBeNull());
    await waitFor(() => expect(screen.getByRole('button', { name: 'Log in with Kite' })).toHaveFocus());
  });
  it('I1: a listener timeout while waiting moves focus to the paste field', async () => {
    const { user, backend } = start(loggedOutSeed());
    await user.click(await screen.findByRole('button', { name: 'Log in with Kite' }));
    await waitFor(() => expect(screen.getByRole('button', { name: 'Cancel login' })).toHaveFocus());
    act(() => backend.emit('session.state', { ...backend.state.session, state: 'logged_out', errorCode: 'listener_timeout', errorMessage: 'The login window timed out.' }));
    await waitFor(() => expect(screen.getByLabelText('Redirect URL or request token')).toHaveFocus());
  });
  it('I1: Cancel gets focus even when logging_in arrives after Login() resolved (Wails event ordering)', async () => {
    const backend = createMockBackend(loggedOutSeed());
    backend.session.login = () => Promise.resolve({
      loginUrl: 'https://kite.zerodha.com/connect/login?v=3&api_key=kiteapikey9f2c',
      redirectUrl: 'http://127.0.0.1:7420/kite/callback', listenerOk: true, listenerError: '',
    });
    const { user } = renderApp({ route: '/', backend });
    await user.click(await screen.findByRole('button', { name: 'Log in with Kite' }));
    await act(async () => {
      await new Promise((r) => setTimeout(r, 30));
      backend.emit('session.state', { ...backend.state.session, state: 'logging_in' });
    });
    await waitFor(() => expect(screen.getByRole('button', { name: 'Cancel login' })).toHaveFocus());
  });
});
