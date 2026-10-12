import { act, screen, waitFor, within } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { demoSeed, readySession } from '../../api/mock/fixtures';
import { createMockBackend } from '../../api/mock/index';
import { useAppStore } from '../../store/app';
import { useNavStore } from '../../store/nav';
import { useSessionStore } from '../../store/session';
import { useSettingsStore } from '../../store/settings';
import { useStatusStore } from '../../store/status';
import { useTicksStore } from '../../store/ticks';
import { useToastStore } from '../../store/toasts';
import { setSystemDark } from '../../test/matchMedia';
import { renderApp } from '../../test/render';

const groupsNav = () => screen.findByRole('navigation', { name: 'Settings groups' });

function fileBackedSeed() {
  const seed = demoSeed();
  seed.session = readySession({
    secrets: { mode: 'file', backend: 'file', keyringAvailable: true, fileExists: true, choices: [] },
    credentials: { ...readySession().credentials, secretBackend: 'file' },
  });
  return seed;
}

/** Every zustand store the app holds, serialised (functions drop out). */
function allStoreState(): string {
  return JSON.stringify([
    useAppStore.getState(), useNavStore.getState(), useSessionStore.getState(), useSettingsStore.getState(),
    useStatusStore.getState(), useTicksStore.getState(), useToastStore.getState(),
  ]);
}

function webStorage(): string {
  const dump = (s: Storage) => Array.from({ length: s.length }, (_, i) => `${s.key(i)}=${s.getItem(s.key(i) ?? '')}`).join('\n');
  return `${dump(window.localStorage)}\n${dump(window.sessionStorage)}`;
}

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
  it('FR-ACCT-012 AC2 pattern: an invalid port shows an inline error and keeps the old value; a valid one updates the redirect URL', async () => {
    const { user, backend } = renderApp({ route: '/settings/kite' });
    const port = await screen.findByLabelText('Login callback port');
    await user.clear(port);
    await user.type(port, '80');
    await user.tab();
    expect(await screen.findByText('Port must be a whole number between 1024 and 65535')).toBeInTheDocument();
    expect(port).toHaveValue('7420');
    expect(backend.state.session.credentials.port).toBe(7420);
    expect(screen.queryByText(/Update the redirect URL/)).toBeNull();
    await user.clear(port);
    await user.type(port, '8080{Enter}');
    await waitFor(() => expect(backend.state.session.credentials.port).toBe(8080));
    expect(backend.callsTo('session.setRedirectPort').map((c) => c.args)).toEqual([[80], [8080]]);
    expect(await screen.findByDisplayValue('http://127.0.0.1:8080/kite/callback')).toBeInTheDocument();
  });
  it('P0-3 Task 10 carry-forward: after a port change it tells the user to update the Kite app redirect URL', async () => {
    const { user } = renderApp({ route: '/settings/kite' });
    const port = await screen.findByLabelText('Login callback port');
    await user.clear(port);
    await user.type(port, '8080{Enter}');
    const notice = await screen.findByRole('status', { name: 'Redirect URL changed' });
    expect(notice).toHaveTextContent(
      'Update the redirect URL of your app in the Kite developer console to http://127.0.0.1:8080/kite/callback, or the next login will fail.',
    );
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
  it('Change saves new credentials and asks for a fresh login', async () => {
    const { user, backend } = renderApp({ route: '/settings/kite' });
    await user.click(await screen.findByRole('button', { name: 'Change' }));
    const dialog = screen.getByRole('dialog', { name: 'Change Kite credentials' });
    await user.type(within(dialog).getByLabelText('API key'), 'newkey77aa');
    await user.type(within(dialog).getByLabelText('API secret'), 'new-secret');
    await user.click(within(dialog).getByRole('button', { name: 'Save' }));
    expect(backend.callsTo('session.saveCredentials')[0].args).toEqual([{ apiKey: 'newkey77aa', apiSecret: 'new-secret' }]);
    expect(await screen.findByText('Kite credentials saved. Log in again to use them.')).toBeInTheDocument();
    await waitFor(() => expect(useSessionStore.getState().session?.state).toBe('logged_out'));
    expect(await screen.findByText('••••77aa')).toBeInTheDocument();
  });
  it('FR-ACCT-013 AC3 (UI part): moves secrets from the encrypted file to the keyring when it becomes available', async () => {
    const { user, backend } = renderApp({ route: '/settings/kite', backend: createMockBackend(fileBackedSeed()) });
    expect(await screen.findByText('Encrypted file (secrets.enc)')).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'Move secrets to the system keyring' }));
    expect(backend.callsTo('session.migrateToKeyring')).toHaveLength(1);
    expect(await screen.findByText('System keyring')).toBeInTheDocument();
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
    const warning = await screen.findByRole('alert', { name: 'Old secrets file not removed' });
    expect(warning).toHaveTextContent('Your secrets are now in the system keyring, but the old encrypted file (secrets.enc) is still on disk.');
    expect(await screen.findByText('System keyring')).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Move secrets to the system keyring' })).toBeNull();
    await user.click(within(warning).getByRole('button', { name: 'Try removing it again' }));
    expect(backend.callsTo('session.migrateToKeyring')).toHaveLength(2);
    await waitFor(() => expect(screen.queryByRole('alert', { name: 'Old secrets file not removed' })).toBeNull());
    expect(await screen.findByText('Secrets moved to the system keyring; the encrypted file was deleted.')).toBeInTheDocument();
  });
  it('Ruling 68: moving to the keyring is offered for a file backend even when the keyring was not detected; a refusal is shown', async () => {
    const seed = fileBackedSeed();
    seed.session.secrets.keyringAvailable = false;
    const { user, backend } = renderApp({ route: '/settings/kite', backend: createMockBackend(seed) });
    await user.click(await screen.findByRole('button', { name: 'Move secrets to the system keyring' }));
    expect(backend.callsTo('session.migrateToKeyring')).toHaveLength(1);
    expect(await screen.findByText('The system keyring is not available')).toBeInTheDocument();
    expect(screen.getByText('Encrypted file (secrets.enc)')).toBeInTheDocument();
  });
});

describe('Settings → Kite connection: the API secret stays out of state (security)', () => {
  it('the secret field is a non-autocompleting password field and never echoes the secret', async () => {
    const { user } = renderApp({ route: '/settings/kite' });
    await user.click(await screen.findByRole('button', { name: 'Change' }));
    const dialog = screen.getByRole('dialog', { name: 'Change Kite credentials' });
    const secret = within(dialog).getByLabelText('API secret');
    expect(secret).toHaveAttribute('type', 'password');
    expect(['off', 'new-password']).toContain(secret.getAttribute('autocomplete'));
    await user.type(secret, 'top-secret-value');
    expect(document.body.textContent).not.toContain('top-secret-value');
  });
  it('after a successful save the secret is in no store, no web storage, no URL and not in the mock persisted state; the field reopens empty', async () => {
    window.localStorage.clear();
    const backend = createMockBackend(demoSeed(), { persistKey: 'tl-test-settings' });
    const { user, router } = renderApp({ route: '/settings/kite', backend });
    await user.click(await screen.findByRole('button', { name: 'Change' }));
    let dialog = screen.getByRole('dialog', { name: 'Change Kite credentials' });
    await user.type(within(dialog).getByLabelText('API key'), 'newkey77aa');
    await user.type(within(dialog).getByLabelText('API secret'), 'S3cr3t-xyz-123');
    await user.click(within(dialog).getByRole('button', { name: 'Save' }));
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull());
    expect(allStoreState()).not.toContain('S3cr3t-xyz-123');
    expect(webStorage()).not.toContain('S3cr3t-xyz-123');
    expect(JSON.stringify(backend.state)).not.toContain('S3cr3t-xyz-123');
    expect(`${router.state.location.pathname}${router.state.location.search}${router.state.location.hash}`).not.toContain('S3cr3t');
    expect(document.body.innerHTML).not.toContain('S3cr3t-xyz-123');
    await user.click(screen.getByRole('button', { name: 'Change' }));
    dialog = screen.getByRole('dialog', { name: 'Change Kite credentials' });
    expect(within(dialog).getByLabelText('API secret')).toHaveValue('');
    window.localStorage.clear();
  });
  it('after a refused save the dialog stays open with the reason and an empty secret field; nothing kept anywhere', async () => {
    const backend = createMockBackend(demoSeed());
    backend.failNext('session.saveCredentials', 'Kite rejected the API key', 'invalid_credentials');
    const { user } = renderApp({ route: '/settings/kite', backend });
    await user.click(await screen.findByRole('button', { name: 'Change' }));
    const dialog = screen.getByRole('dialog', { name: 'Change Kite credentials' });
    await user.type(within(dialog).getByLabelText('API key'), 'newkey77aa');
    await user.type(within(dialog).getByLabelText('API secret'), 'S3cr3t-fail-9');
    await user.click(within(dialog).getByRole('button', { name: 'Save' }));
    expect(await within(dialog).findByText('Kite rejected the API key')).toBeInTheDocument();
    expect(within(dialog).getByLabelText('API secret')).toHaveValue('');
    expect(within(dialog).getByLabelText('API key')).toHaveValue('newkey77aa');
    expect(allStoreState()).not.toContain('S3cr3t-fail-9');
    expect(webStorage()).not.toContain('S3cr3t-fail-9');
    expect(JSON.stringify(backend.state)).not.toContain('S3cr3t-fail-9');
  });
  it('cancelling the dialog discards the typed secret', async () => {
    const { user } = renderApp({ route: '/settings/kite' });
    await user.click(await screen.findByRole('button', { name: 'Change' }));
    let dialog = screen.getByRole('dialog', { name: 'Change Kite credentials' });
    await user.type(within(dialog).getByLabelText('API secret'), 'discard-me');
    await user.click(within(dialog).getByRole('button', { name: 'Cancel' }));
    await user.click(screen.getByRole('button', { name: 'Change' }));
    dialog = screen.getByRole('dialog', { name: 'Change Kite credentials' });
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
  it('AC3: shows the version and lists third-party licences', async () => {
    renderApp({ route: '/settings/about' });
    expect(await screen.findByText('v0.1.0')).toBeInTheDocument();
    expect(screen.getByRole('row', { name: /lightweight-charts 5\.2\.1 Apache-2\.0/ })).toBeInTheDocument();
  });
  it('links to the legal pages and the risk disclosure', async () => {
    renderApp({ route: '/settings/about' });
    expect(await screen.findByRole('link', { name: 'Terms of use' })).toHaveAttribute('href', '/legal/terms');
    expect(screen.getByRole('link', { name: 'Privacy notice' })).toHaveAttribute('href', '/legal/privacy');
    expect(screen.getByRole('link', { name: 'Risk disclosure' })).toHaveAttribute('href', '/legal/risk');
  });
});
