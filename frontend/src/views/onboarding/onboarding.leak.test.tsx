import { act, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi, type MockInstance } from 'vitest';
import { freshSeed, lockedSeed, loggedOutSeed, noKeyringSeed } from '../../api/mock/fixtures';
import { createMockBackend, type MockBackend, type MockState } from '../../api/mock/index';
import { usePortfolioStore } from '../../store/portfolio';
import { useAppStore } from '../../store/app';
import { useNavStore } from '../../store/nav';
import { useSessionStore } from '../../store/session';
import { useSettingsStore } from '../../store/settings';
import { useStatusStore } from '../../store/status';
import { useTicksStore } from '../../store/ticks';
import { useToastStore } from '../../store/toasts';
import { useShortcutStore } from '../../shortcuts/store';
import { renderApp } from '../../test/render';

// Ruling 101: the API secret, the passphrases and a pasted redirect URL (it carries request_token) live only in
// component-local state, are cleared after every submit and on unmount, and never reach a store, web storage, the
// URL, the console, a toast or the mock's persisted state. Style follows the Task 14 leak tests.

const PERSIST_KEY = 'tl-test-onboarding';

/** Every zustand store the app holds, serialised (functions drop out). Toasts are in useToastStore. */
function allStoreState(): string {
  return JSON.stringify([
    useAppStore.getState(), useNavStore.getState(), useSessionStore.getState(), useSettingsStore.getState(),
    useStatusStore.getState(), useTicksStore.getState(), useToastStore.getState(), usePortfolioStore.getState(),
    useShortcutStore.getState(),
  ]);
}

function webStorage(): string {
  const dump = (s: Storage) => Array.from({ length: s.length }, (_, i) => `${s.key(i)}=${s.getItem(s.key(i) ?? '')}`).join('\n');
  return `${dump(window.localStorage)}\n${dump(window.sessionStorage)}`;
}

/**
 * The mock's state as it would persist. `encPassphrase` stands in for Go's secrets.enc (the mock needs it to answer
 * Unlock like Go) and is therefore not UI state; it is blanked before the check.
 */
const mockState = (b: MockBackend) => JSON.stringify({ ...b.state, encPassphrase: '' });

const CONSOLE_METHODS = ['log', 'info', 'warn', 'error', 'debug', 'trace'] as const;
let consoleSpies: MockInstance[] = [];
const consoleText = () => JSON.stringify(consoleSpies.flatMap((s) => s.mock.calls.map((c) => c.map(String))));

beforeEach(() => {
  window.localStorage.clear();
  window.sessionStorage.clear();
  consoleSpies = CONSOLE_METHODS.map((m) => vi.spyOn(console, m));
});

afterEach(() => {
  for (const s of consoleSpies) s.mockRestore();
  window.localStorage.clear();
  window.sessionStorage.clear();
});

function startLeak(seed: MockState) {
  const backend = createMockBackend(seed, { persistKey: PERSIST_KEY });
  return renderApp({ route: '/', backend });
}

function expectNowhere(value: string, backend: MockBackend, router: ReturnType<typeof renderApp>['router']) {
  expect(allStoreState()).not.toContain(value);
  expect(useToastStore.getState().visible.concat(useToastStore.getState().queued).map((t) => t.message).join('\n')).not.toContain(value);
  expect(webStorage()).not.toContain(value);
  expect(mockState(backend)).not.toContain(value);
  const loc = router.state.location;
  expect(`${loc.pathname}${loc.search}${loc.hash}${JSON.stringify(loc.state ?? null)}`).not.toContain(value);
  expect(window.location.href).not.toContain(value);
  expect(consoleText()).not.toContain(value);
  expect(document.body.textContent).not.toContain(value);
}

describe('setup: the API secret stays out of state (security)', () => {
  it('is a non-autocompleting password field that never echoes the secret', async () => {
    const { user } = startLeak(freshSeed());
    const secret = await screen.findByLabelText('API secret');
    expect(secret).toHaveAttribute('type', 'password');
    expect(secret).toHaveAttribute('autocomplete', 'off');
    await user.type(secret, 'top-secret-value');
    expect(document.body.textContent).not.toContain('top-secret-value');
  });
  it('after a successful save the secret is nowhere', async () => {
    const { user, backend, router } = startLeak(freshSeed());
    await user.type(await screen.findByLabelText('API key'), 'kiteapikey9f2c');
    await user.type(screen.getByLabelText('API secret'), 'S3cr3tXyz123');
    await user.click(screen.getByRole('button', { name: 'Save and continue' }));
    await screen.findByRole('heading', { name: 'Log in to Kite' });
    expectNowhere('S3cr3tXyz123', backend, router);
  });
  it('after a refused save the secret field is empty and the secret is nowhere', async () => {
    const backend = createMockBackend(freshSeed(), { persistKey: PERSIST_KEY });
    backend.failNext('session.saveCredentials', 'Kite rejected the API key', 'invalid_credentials');
    const { user, router } = renderApp({ route: '/', backend });
    await user.type(await screen.findByLabelText('API key'), 'kiteapikey9f2c');
    await user.type(screen.getByLabelText('API secret'), 'S3cr3tFail9');
    await user.click(screen.getByRole('button', { name: 'Save and continue' }));
    expect(await screen.findAllByText('Kite rejected the API key')).not.toHaveLength(0);
    expect(screen.getByLabelText('API secret')).toHaveValue('');
    expect(screen.getByLabelText('API key')).toHaveValue('kiteapikey9f2c');
    expectNowhere('S3cr3tFail9', backend, router);
  });
  it('leaving setup discards the typed secret (unmount)', async () => {
    const { user, backend, router } = startLeak(freshSeed());
    await user.type(await screen.findByLabelText('API secret'), 'discard-me-77');
    await act(() => router.navigate('/legal/terms'));
    await waitFor(() => expect(screen.queryByLabelText('API secret')).toBeNull());
    await act(() => router.navigate('/setup'));
    expect(await screen.findByLabelText('API secret')).toHaveValue('');
    expectNowhere('discard-me-77', backend, router);
  });
});

describe('secret storage: the passphrases stay out of state (security)', () => {
  it('both passphrase fields are password fields; a successful set-up keeps the passphrase nowhere in the UI', async () => {
    const { user, backend, router } = startLeak(noKeyringSeed());
    const pass = await screen.findByLabelText('Passphrase');
    expect(pass).toHaveAttribute('type', 'password');
    expect(screen.getByLabelText('Repeat passphrase')).toHaveAttribute('type', 'password');
    await user.type(pass, 'pass-phrase-ok-1');
    await user.type(screen.getByLabelText('Repeat passphrase'), 'pass-phrase-ok-1');
    await user.click(screen.getByRole('button', { name: 'Continue' }));
    await screen.findByRole('heading', { name: 'Connect your Kite account' });
    expectNowhere('pass-phrase-ok-1', backend, router);
  });
  it('a refused set-up empties both passphrase fields and keeps the passphrase nowhere', async () => {
    const backend = createMockBackend(noKeyringSeed(), { persistKey: PERSIST_KEY });
    backend.failNext('session.useEncryptedFile', 'A secrets file already exists; unlock it with its passphrase instead.', 'file_exists');
    const { user, router } = renderApp({ route: '/', backend });
    await user.type(await screen.findByLabelText('Passphrase'), 'pass-phrase-no-2');
    await user.type(screen.getByLabelText('Repeat passphrase'), 'pass-phrase-no-2');
    await user.click(screen.getByRole('button', { name: 'Continue' }));
    expect(await screen.findAllByText(/A secrets file already exists/)).not.toHaveLength(0);
    expect(screen.getByLabelText('Passphrase')).toHaveValue('');
    expect(screen.getByLabelText('Repeat passphrase')).toHaveValue('');
    expectNowhere('pass-phrase-no-2', backend, router);
  });
  it('a wrong unlock passphrase is cleared from the field and kept nowhere', async () => {
    const { user, backend, router } = startLeak(lockedSeed());
    const pass = await screen.findByLabelText('Passphrase');
    expect(pass).toHaveAttribute('type', 'password');
    await user.type(pass, 'wrong-unlock-3');
    await user.click(screen.getByRole('button', { name: 'Unlock' }));
    await screen.findByText(/wrong passphrase or damaged file/);
    expect(screen.getByLabelText('Passphrase')).toHaveValue('');
    expectNowhere('wrong-unlock-3', backend, router);
  });
  it('a successful unlock keeps the passphrase nowhere in the UI', async () => {
    const seed = lockedSeed();
    seed.encPassphrase = 'right-unlock-4';
    const { user, backend, router } = startLeak(seed);
    await user.type(await screen.findByLabelText('Passphrase'), 'right-unlock-4');
    await user.click(screen.getByRole('button', { name: 'Unlock' }));
    await screen.findByRole('heading', { name: 'Log in to Kite' });
    expectNowhere('right-unlock-4', backend, router);
  });
  it('leaving the unlock form discards the typed passphrase (unmount)', async () => {
    const { user, backend, router } = startLeak(lockedSeed());
    await user.type(await screen.findByLabelText('Passphrase'), 'typed-not-sent-5');
    await act(() => router.navigate('/legal/terms'));
    await waitFor(() => expect(screen.queryByLabelText('Passphrase')).toBeNull());
    await act(() => router.navigate('/setup'));
    expect(await screen.findByLabelText('Passphrase')).toHaveValue('');
    expectNowhere('typed-not-sent-5', backend, router);
  });
});

describe('login: the pasted redirect URL stays out of state (security)', () => {
  const TOKEN = 'reqTok9Leak77';
  const URL_WITH_TOKEN = `http://127.0.0.1:7420/kite/callback?request_token=${TOKEN}&status=success`;

  it('a successful paste login keeps the URL and its request token nowhere', async () => {
    const { user, backend, router } = startLeak(loggedOutSeed());
    await user.click(await screen.findByRole('button', { name: 'Login did not come back to Tradeloom?' }));
    const field = screen.getByLabelText('Redirect URL or request token');
    expect(field).toHaveAttribute('autocomplete', 'off');
    await user.type(field, URL_WITH_TOKEN);
    await user.click(screen.getByRole('button', { name: 'Complete login' }));
    await screen.findByRole('heading', { name: 'Before you start' });
    expectNowhere(TOKEN, backend, router);
  });
  it('a refused paste empties the field, never shows the URL back and keeps it nowhere', async () => {
    const backend = createMockBackend(loggedOutSeed(), { persistKey: PERSIST_KEY });
    backend.failNext('session.completeLogin', 'This login link was already used or has expired (Kite request tokens last only a few minutes). Click "Log in with Kite" again.', 'request_token_invalid');
    const { user, router } = renderApp({ route: '/', backend });
    await user.click(await screen.findByRole('button', { name: 'Login did not come back to Tradeloom?' }));
    await user.type(screen.getByLabelText('Redirect URL or request token'), URL_WITH_TOKEN);
    await user.click(screen.getByRole('button', { name: 'Complete login' }));
    expect(await screen.findAllByText(/This login link was already used/)).not.toHaveLength(0);
    expect(screen.getByLabelText('Redirect URL or request token')).toHaveValue('');
    expectNowhere(TOKEN, backend, router);
  });
  it('leaving the login view discards the pasted text (unmount)', async () => {
    const { user, backend, router } = startLeak(loggedOutSeed());
    await user.click(await screen.findByRole('button', { name: 'Login did not come back to Tradeloom?' }));
    await user.type(screen.getByLabelText('Redirect URL or request token'), URL_WITH_TOKEN);
    await act(() => router.navigate('/legal/terms'));
    await waitFor(() => expect(screen.queryByLabelText('Redirect URL or request token')).toBeNull());
    await act(() => router.navigate('/login'));
    await user.click(await screen.findByRole('button', { name: 'Login did not come back to Tradeloom?' }));
    expect(screen.getByLabelText('Redirect URL or request token')).toHaveValue('');
    expectNowhere(TOKEN, backend, router);
  });
});
