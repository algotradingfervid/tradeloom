> **P0-4 Frontend Shell, Account & Data UI Implementation Plan — Task 14.** Read [`../MANIFEST.md`](../MANIFEST.md) first: its Global Constraints,
> Review Focus and Consumes/Produces apply to this task. Other tasks live beside this file in `tasks/`.

<!-- task body (verbatim from the plan) -->
### Task 14: Settings view — shell, Kite connection, Appearance, Chart, About

**Files:**
- Create: `frontend/src/views/settings/groups.ts`, `SettingsView.tsx`, `SettingRow.tsx`, `KiteGroup.tsx`, `AppearanceGroup.tsx`, `ChartGroup.tsx`, `AboutGroup.tsx`, `settings.css` (under `frontend/src/views/settings/`)
- Modify: `frontend/src/app/routes.tsx` (`settings`, `settings/:group`), `frontend/src/i18n/copy.ts`
- Test: `frontend/src/views/settings/SettingsView.test.tsx`

**Interfaces:**
- Consumes: Task 2 `useSetting`, `useSessionStore`, `useAppStore`, `Backend.session.{credentials,secretStorage,saveCredentials,removeCredentials,setRedirectPort,migrateToKeyring}`, `CredentialsInfo`, `SecretStorageInfo`; settings keys `appearance.theme`, `appearance.follow_system`, `appearance.sidebar_collapsed`, `chart.default_timeframe` (contract §E; `kite.redirect_port` is not a `Settings.Set` key — `Session.SetRedirectPort` owns it); Task 3 primitives; Task 5 `NotFoundView`; Task 1 `useTheme`, `BUILD_PHASE`, `Phase`.
- Produces:
  - `groups.ts`: `type SettingsGroupId = 'kite' | 'trading' | 'appearance' | 'chart' | 'home' | 'universes' | 'calendar' | 'data' | 'notifications' | 'shortcuts' | 'limits' | 'updates' | 'diagnostics' | 'tours' | 'about'`; `SETTINGS_GROUPS: readonly { id; label; phase: Phase }[]` in FR-ACCT-012 order.
  - `SettingsView.tsx`: `GROUP_COMPONENTS: Partial<Record<SettingsGroupId, ComponentType>>` (Tasks 15–17 add `shortcuts`, `diagnostics`, `data`, `calendar`), `visibleSettingsGroups()`, `SettingsView()` at `/settings/:group` (`/settings` → `/settings/kite`; hidden or unknown group → "Page not found").
  - `SettingRow.tsx`: `SettingSection({ title; children })`, `Fact({ label; children })`, `NumberCommitField({ label; value: number; onCommit(n): Promise<string | null>; hint? })` (commits on blur/Enter, Escape reverts, refusal reason shown inline and the old value kept — FR-ACCT-012 AC2 pattern), `SettingNumberField({ settingKey; label; fallback: number; hint? })` (a `NumberCommitField` over `useSetting`).
  - `KiteGroup.tsx`: `KiteGroup()` — reads `Session.Credentials()` and `Session.SecretStorage()` (re-read on every `session.state`), port via `Session.SetRedirectPort`.
  - `ChartGroup.tsx`: `TIMEFRAME_OPTIONS` (Kite interval ids `minute`…`day`) — **P0-5 must read `chart.default_timeframe` with these ids**.

- [ ] **Step 1: Write the failing test**

Create `frontend/src/views/settings/SettingsView.test.tsx`:

```tsx
import { act, screen, waitFor, within } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { demoSeed, readySession } from '../../api/mock/fixtures';
import { createMockBackend } from '../../api/mock/index';
import { setSystemDark } from '../../test/matchMedia';
import { renderApp } from '../../test/render';

const groupsNav = () => screen.findByRole('navigation', { name: 'Settings groups' });

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
  });
  it('a hidden group cannot be opened by route', async () => {
    renderApp({ route: '/settings/trading' });
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
    await user.clear(port);
    await user.type(port, '8080{Enter}');
    await waitFor(() => expect(backend.state.session.credentials.port).toBe(8080));
    expect(backend.callsTo('session.setRedirectPort').map((c) => c.args)).toEqual([[80], [8080]]);
    expect(await screen.findByDisplayValue('http://127.0.0.1:8080/kite/callback')).toBeInTheDocument();
  });
  it('FR-ACCT-005 AC3: Remove credentials → confirm → setup flow', async () => {
    const { user, backend } = renderApp({ route: '/settings/kite' });
    await user.click(await screen.findByRole('button', { name: 'Remove credentials' }));
    const dialog = screen.getByRole('dialog', { name: 'Remove Kite credentials?' });
    await user.click(within(dialog).getByRole('button', { name: 'Remove credentials' }));
    expect(backend.callsTo('session.removeCredentials')).toHaveLength(1);
    expect(await screen.findByRole('heading', { name: 'Connect your Kite account' })).toBeInTheDocument();
  });
  it('Change saves new credentials and asks for a fresh login', async () => {
    const { user, backend } = renderApp({ route: '/settings/kite' });
    await user.click(await screen.findByRole('button', { name: 'Change' }));
    const dialog = screen.getByRole('dialog', { name: 'Change Kite credentials' });
    await user.type(within(dialog).getByLabelText('API key'), 'newkey77aa');
    await user.type(within(dialog).getByLabelText('API secret'), 'new-secret');
    await user.click(within(dialog).getByRole('button', { name: 'Save' }));
    expect(backend.callsTo('session.saveCredentials')[0].args).toEqual([{ apiKey: 'newkey77aa', apiSecret: 'new-secret' }]);
    expect(await screen.findByRole('heading', { name: 'Log in to Kite' })).toBeInTheDocument();
  });
  it('FR-ACCT-013 AC3 (UI part): moves secrets from the encrypted file to the keyring when it becomes available', async () => {
    const seed = demoSeed();
    seed.session = readySession({
      secrets: { mode: 'file', backend: 'file', keyringAvailable: true, fileExists: true, choices: [] },
      credentials: { ...readySession().credentials, secretBackend: 'file' },
    });
    const { user, backend } = renderApp({ route: '/settings/kite', backend: createMockBackend(seed) });
    expect(await screen.findByText('Encrypted file (secrets.enc)')).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'Move secrets to the system keyring' }));
    expect(backend.callsTo('session.migrateToKeyring')).toHaveLength(1);
    expect(await screen.findByText('System keyring')).toBeInTheDocument();
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
  it('stores the default chart timeframe as a Kite interval id', async () => {
    const { user, backend } = renderApp({ route: '/settings/chart' });
    const select = await screen.findByLabelText('Default chart timeframe');
    expect(select).toHaveValue('60minute');
    await user.selectOptions(select, '15minute');
    await waitFor(() => expect(backend.state.settings['chart.default_timeframe']).toBe('"15minute"'));
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
```

- [ ] **Step 2: Run it to verify it fails**

Run: `cd frontend && npx vitest run src/views/settings`
Expected: FAIL — `/settings` renders "Page not found".

- [ ] **Step 3: Implement the settings shell and the four groups**

Append to `frontend/src/i18n/copy.ts`:

```ts
export const settingsViewCopy = {
  nav: 'Settings groups',
  groups: {
    kite: 'Kite connection', trading: 'Trading', appearance: 'Appearance', chart: 'Chart', home: 'Home',
    universes: 'Universes & baskets', calendar: 'Market calendar', data: 'Data', notifications: 'Notifications',
    shortcuts: 'Shortcuts', limits: 'Limits', updates: 'Updates', diagnostics: 'Diagnostics', tours: 'Tours', about: 'About',
  },
  wholeNumber: 'Enter a whole number',
} as const;

export const kiteSettingsCopy = {
  credentials: 'Credentials',
  apiKey: 'API key',
  dataPlan: 'Data plan',
  plan: { connect: 'Kite Connect (market data available)', none: 'Orders and portfolio only (no data plan)', unknown: 'Checked after the next login' },
  secretStorage: 'Secret storage',
  // Keys are P0-1 secrets backends as `CredentialsInfo.secretBackend` / `SecretStorageInfo.backend` report them.
  backend: { keyring: 'System keyring', file: 'Encrypted file (secrets.enc)', locked: 'Encrypted file (locked)', none: 'Not stored' } as Record<string, string>,
  change: 'Change',
  changeTitle: 'Change Kite credentials',
  save: 'Save',
  saved: 'Kite credentials saved. Log in again to use them.',
  remove: 'Remove credentials',
  removeTitle: 'Remove Kite credentials?',
  removeMessage: "The API key, the API secret and today's access token are deleted from this computer. You will need to set them up again before using Kite.",
  migrate: 'Move secrets to the system keyring',
  migrated: 'Secrets moved to the system keyring; the encrypted file was deleted.',
  login: 'Login',
  port: 'Login callback port',
  portHint: 'Register the matching redirect URL in the Kite developer console after changing this.',
} as const;

export const appearanceCopy = {
  title: 'Theme and layout',
  darkTheme: 'Dark theme',
  followSystem: 'Follow system theme',
  followSystemHint: 'Use the light or dark preference of your desktop.',
  followingHint: 'Controlled by your desktop while "Follow system theme" is on.',
  sidebarCollapsed: 'Collapse the sidebar to icons',
} as const;

export const chartSettingsCopy = {
  title: 'Chart defaults',
  timeframe: 'Default chart timeframe',
  hint: 'Used the first time an instrument is charted; afterwards each chart remembers its own timeframe.',
} as const;

export const aboutCopy = {
  app: 'Tradeloom',
  version: 'Version',
  build: 'Build',
  schema: 'Database schema',
  legal: 'Legal',
  terms: 'Terms of use',
  privacy: 'Privacy notice',
  risk: 'Risk disclosure',
  licences: 'Third-party licences',
  component: 'Component',
  licenceVersion: 'Version',
  licence: 'Licence',
} as const;
```

Create `frontend/src/views/settings/groups.ts`:

```ts
import type { Phase } from '../../app/features';
import { settingsViewCopy as copy } from '../../i18n/copy';

export type SettingsGroupId =
  | 'kite' | 'trading' | 'appearance' | 'chart' | 'home' | 'universes' | 'calendar' | 'data'
  | 'notifications' | 'shortcuts' | 'limits' | 'updates' | 'diagnostics' | 'tours' | 'about';

export interface SettingsGroupDef { id: SettingsGroupId; label: string; phase: Phase }

// FR-ACCT-012 order; phases per ROADMAP §7 GATE row for FR-ACCT-012.
export const SETTINGS_GROUPS: readonly SettingsGroupDef[] = [
  { id: 'kite', label: copy.groups.kite, phase: 0 },
  { id: 'trading', label: copy.groups.trading, phase: 4 },
  { id: 'appearance', label: copy.groups.appearance, phase: 0 },
  { id: 'chart', label: copy.groups.chart, phase: 0 },
  { id: 'home', label: copy.groups.home, phase: 2 },
  { id: 'universes', label: copy.groups.universes, phase: 1 },
  { id: 'calendar', label: copy.groups.calendar, phase: 0 },
  { id: 'data', label: copy.groups.data, phase: 0 },
  { id: 'notifications', label: copy.groups.notifications, phase: 3 },
  { id: 'shortcuts', label: copy.groups.shortcuts, phase: 0 },
  { id: 'limits', label: copy.groups.limits, phase: 4 },
  { id: 'updates', label: copy.groups.updates, phase: 6 },
  { id: 'diagnostics', label: copy.groups.diagnostics, phase: 0 },
  { id: 'tours', label: copy.groups.tours, phase: 6 },
  { id: 'about', label: copy.groups.about, phase: 0 },
];
```

Create `frontend/src/views/settings/SettingRow.tsx`:

```tsx
import { useEffect, useId, useState, type ReactNode } from 'react';
import { TextField } from '../../components/TextField';
import { settingsViewCopy } from '../../i18n/copy';
import { useSetting } from '../../store/settings';
import './settings.css';

export function SettingSection({ title, children }: { title: string; children: ReactNode }) {
  const id = useId();
  return (
    <section className="settings__section" aria-labelledby={id}>
      <h2 id={id}>{title}</h2>
      {children}
    </section>
  );
}

export function Fact({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div className="fact">
      <dt>{label}</dt>
      <dd>{children}</dd>
    </div>
  );
}

export interface NumberCommitFieldProps { label: string; value: number; onCommit: (n: number) => Promise<string | null>; hint?: string }

/** Commits on blur or Enter; Escape reverts; a refusal reason is shown inline and the stored value is kept. */
export function NumberCommitField({ label, value, onCommit, hint }: NumberCommitFieldProps) {
  const [draft, setDraft] = useState(String(value));
  const [error, setError] = useState<string | null>(null);
  useEffect(() => setDraft(String(value)), [value]);

  const commit = async () => {
    if (draft === String(value)) {
      setError(null);
      return;
    }
    const n = Number(draft);
    if (!Number.isInteger(n)) {
      setError(settingsViewCopy.wholeNumber);
      return;
    }
    const reason = await onCommit(n);
    setError(reason);
    if (reason) setDraft(String(value));
  };

  return (
    <TextField
      label={label}
      hint={hint}
      inputMode="numeric"
      value={draft}
      error={error}
      onChange={(e) => setDraft(e.target.value)}
      onBlur={() => void commit()}
      onKeyDown={(e) => {
        if (e.key === 'Enter') void commit();
        if (e.key === 'Escape') {
          setDraft(String(value));
          setError(null);
        }
      }}
    />
  );
}

export interface SettingNumberFieldProps { settingKey: string; label: string; fallback: number; hint?: string }

/** A `Settings.Set`-backed number (Go validates; the refusal reason is shown inline). */
export function SettingNumberField({ settingKey, label, fallback, hint }: SettingNumberFieldProps) {
  const [value, setValue] = useSetting<number>(settingKey, fallback);
  return <NumberCommitField label={label} value={value} hint={hint} onCommit={async (n) => (await setValue(n))?.reason ?? null} />;
}
```

Create `frontend/src/views/settings/KiteGroup.tsx`:

```tsx
import { useCallback, useEffect, useState, type FormEvent } from 'react';
import type { CredentialsInfo, SecretStorageInfo } from '../../api/backend';
import { useBackend } from '../../api/BackendContext';
import { parseBackendError } from '../../api/errors';
import { Button } from '../../components/Button';
import { ConfirmDialog } from '../../components/ConfirmDialog';
import { Dialog } from '../../components/Dialog';
import { TextField } from '../../components/TextField';
import { componentCopy, kiteSettingsCopy as copy, setupCopy } from '../../i18n/copy';
import { DASH } from '../../i18n/format';
import { useSessionStore } from '../../store/session';
import { pushToast } from '../../store/toasts';
import { Fact, NumberCommitField, SettingSection } from './SettingRow';

function ChangeCredentialsDialog({ open, onClose }: { open: boolean; onClose: () => void }) {
  const backend = useBackend();
  const [apiKey, setApiKey] = useState('');
  const [secret, setSecret] = useState('');
  const [error, setError] = useState<string | null>(null);
  const submit = async (e?: FormEvent) => {
    e?.preventDefault();
    try {
      await backend.session.saveCredentials({ apiKey: apiKey.trim(), apiSecret: secret });
      pushToast({ level: 'success', message: copy.saved });
      onClose();
    } catch (err) {
      setError(parseBackendError(err).reason);
    } finally {
      setSecret('');
    }
  };
  return (
    <Dialog
      open={open}
      title={copy.changeTitle}
      onClose={onClose}
      footer={<><Button onClick={onClose}>{componentCopy.cancel}</Button><Button variant="primary" onClick={() => void submit()}>{copy.save}</Button></>}
    >
      <form onSubmit={(e) => void submit(e)} noValidate className="settings__section">
        <TextField label={setupCopy.apiKey} value={apiKey} onChange={(e) => setApiKey(e.target.value)} autoComplete="off" spellCheck={false} />
        <TextField label={setupCopy.apiSecret} type="password" value={secret} onChange={(e) => setSecret(e.target.value)} autoComplete="off" error={error} />
      </form>
    </Dialog>
  );
}

export function KiteGroup() {
  const backend = useBackend();
  const session = useSessionStore((s) => s.session);
  const [creds, setCreds] = useState<CredentialsInfo | null>(null);
  const [storage, setStorage] = useState<SecretStorageInfo | null>(null);
  const [changing, setChanging] = useState(false);
  const [removing, setRemoving] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async () => {
    try {
      const [c, s] = await Promise.all([backend.session.credentials(), backend.session.secretStorage()]);
      setCreds(c);
      setStorage(s);
    } catch (e) {
      pushToast({ level: 'error', message: parseBackendError(e).reason });
    }
  }, [backend]);

  // Every credentials/secrets change is followed by a "session.state" event, so re-read on each new session object.
  useEffect(() => {
    void load();
  }, [load, session]);

  if (!session || !creds) return null;

  const savePort = async (port: number): Promise<string | null> => {
    try {
      setCreds(await backend.session.setRedirectPort(port));
      return null;
    } catch (e) {
      return parseBackendError(e).reason;
    }
  };

  const remove = async () => {
    setBusy(true);
    setError(null);
    try {
      await backend.session.removeCredentials();
    } catch (e) {
      setError(parseBackendError(e).reason);
    } finally {
      setBusy(false);
    }
  };
  const migrate = async () => {
    try {
      setStorage(await backend.session.migrateToKeyring());
      pushToast({ level: 'success', message: copy.migrated });
    } catch (e) {
      pushToast({ level: 'error', message: parseBackendError(e).reason });
    }
  };

  return (
    <>
      <SettingSection title={copy.credentials}>
        <dl className="facts">
          <Fact label={copy.apiKey}>{creds.apiKeyMasked || DASH}</Fact>
          <Fact label={copy.dataPlan}>{copy.plan[session.dataPlan]}</Fact>
          <Fact label={copy.secretStorage}>{copy.backend[creds.secretBackend] ?? creds.secretBackend}</Fact>
        </dl>
        <div className="settings__actions">
          <Button onClick={() => setChanging(true)}>{copy.change}</Button>
          <Button variant="danger" onClick={() => setRemoving(true)}>{copy.remove}</Button>
          {storage?.backend === 'file' && storage.keyringAvailable && <Button onClick={() => void migrate()}>{copy.migrate}</Button>}
        </div>
      </SettingSection>
      <SettingSection title={copy.login}>
        <TextField label={setupCopy.redirectLabel} value={creds.redirectUrl} readOnly />
        <NumberCommitField label={copy.port} value={creds.port} hint={copy.portHint} onCommit={savePort} />
      </SettingSection>
      {changing && <ChangeCredentialsDialog open onClose={() => setChanging(false)} />}
      <ConfirmDialog
        open={removing}
        title={copy.removeTitle}
        message={copy.removeMessage}
        confirmLabel={copy.remove}
        tone="danger"
        busy={busy}
        error={error}
        onConfirm={() => void remove()}
        onCancel={() => setRemoving(false)}
      />
    </>
  );
}
```

Create `frontend/src/views/settings/AppearanceGroup.tsx`:

```tsx
import { useTheme } from '../../app/theme';
import { Toggle } from '../../components/Toggle';
import type { ThemeName } from '../../design/tokens';
import { appearanceCopy as copy } from '../../i18n/copy';
import { useSetting } from '../../store/settings';
import { SettingSection } from './SettingRow';

export function AppearanceGroup() {
  const { theme } = useTheme();
  const [, setTheme] = useSetting<ThemeName>('appearance.theme', 'light');
  const [follow, setFollow] = useSetting<boolean>('appearance.follow_system', false);
  const [collapsed, setCollapsed] = useSetting<boolean>('appearance.sidebar_collapsed', false);
  return (
    <SettingSection title={copy.title}>
      <Toggle label={copy.darkTheme} checked={theme === 'dark'} disabled={follow} description={follow ? copy.followingHint : undefined} onChange={(on) => void setTheme(on ? 'dark' : 'light')} />
      <Toggle label={copy.followSystem} checked={follow} description={copy.followSystemHint} onChange={(on) => void setFollow(on)} />
      <Toggle label={copy.sidebarCollapsed} checked={collapsed} onChange={(on) => void setCollapsed(on)} />
    </SettingSection>
  );
}
```

Create `frontend/src/views/settings/ChartGroup.tsx`:

```tsx
import { useState } from 'react';
import { SelectField } from '../../components/SelectField';
import { chartSettingsCopy as copy } from '../../i18n/copy';
import { useSetting } from '../../store/settings';
import { SettingSection } from './SettingRow';

/** Kite interval ids (FR-DATA-002 native intervals). P0-5 reads chart.default_timeframe with these ids. */
export const TIMEFRAME_OPTIONS = [
  { value: 'minute', label: '1 minute' },
  { value: '3minute', label: '3 minutes' },
  { value: '5minute', label: '5 minutes' },
  { value: '10minute', label: '10 minutes' },
  { value: '15minute', label: '15 minutes' },
  { value: '30minute', label: '30 minutes' },
  { value: '60minute', label: '1 hour' },
  { value: 'day', label: '1 day' },
];

export function ChartGroup() {
  const [timeframe, setTimeframe] = useSetting<string>('chart.default_timeframe', '60minute');
  const [error, setError] = useState<string | null>(null);
  return (
    <SettingSection title={copy.title}>
      <SelectField
        label={copy.timeframe}
        options={TIMEFRAME_OPTIONS}
        value={timeframe}
        error={error}
        onChange={async (e) => setError((await setTimeframe(e.target.value))?.reason ?? null)}
      />
      <p className="muted">{copy.hint}</p>
    </SettingSection>
  );
}
```

Create `frontend/src/views/settings/AboutGroup.tsx`:

```tsx
import { Link } from 'react-router-dom';
import { aboutCopy as copy, profileMenuCopy } from '../../i18n/copy';
import { DASH } from '../../i18n/format';
import { useAppStore } from '../../store/app';
import { Fact, SettingSection } from './SettingRow';
import '../../components/list.css';

export function AboutGroup() {
  const info = useAppStore((s) => s.info);
  return (
    <>
      <SettingSection title={copy.app}>
        <dl className="facts">
          <Fact label={copy.version}>{info ? profileMenuCopy.version(info.version) : DASH}</Fact>
          <Fact label={copy.build}>{info?.build ?? DASH}</Fact>
          <Fact label={copy.schema}>{info ? String(info.schemaVersion) : DASH}</Fact>
        </dl>
      </SettingSection>
      <SettingSection title={copy.legal}>
        <ul className="settings__links">
          <li><Link to="/legal/terms">{copy.terms}</Link></li>
          <li><Link to="/legal/privacy">{copy.privacy}</Link></li>
          <li><Link to="/legal/risk">{copy.risk}</Link></li>
        </ul>
      </SettingSection>
      <SettingSection title={copy.licences}>
        <table className="data-table">
          <caption className="visually-hidden">{copy.licences}</caption>
          <thead>
            <tr><th scope="col">{copy.component}</th><th scope="col">{copy.licenceVersion}</th><th scope="col">{copy.licence}</th></tr>
          </thead>
          <tbody>
            {(info?.licences ?? []).map((l) => (
              <tr key={l.name}><td>{l.name}</td><td>{l.version}</td><td>{l.licence}</td></tr>
            ))}
          </tbody>
        </table>
      </SettingSection>
    </>
  );
}
```

Create `frontend/src/views/settings/SettingsView.tsx`:

```tsx
import type { ComponentType } from 'react';
import { NavLink, useParams } from 'react-router-dom';
import { BUILD_PHASE } from '../../app/features';
import { settingsViewCopy as copy } from '../../i18n/copy';
import { NotFoundView } from '../NotFoundView';
import { AboutGroup } from './AboutGroup';
import { AppearanceGroup } from './AppearanceGroup';
import { ChartGroup } from './ChartGroup';
import { SETTINGS_GROUPS, type SettingsGroupDef, type SettingsGroupId } from './groups';
import { KiteGroup } from './KiteGroup';
import './settings.css';

/** Later tasks add `calendar`, `data`, `shortcuts`, `diagnostics`; later phases add theirs. */
export const GROUP_COMPONENTS: Partial<Record<SettingsGroupId, ComponentType>> = {
  kite: KiteGroup,
  appearance: AppearanceGroup,
  chart: ChartGroup,
  about: AboutGroup,
};

export function visibleSettingsGroups(): SettingsGroupDef[] {
  return SETTINGS_GROUPS.filter((g) => BUILD_PHASE >= g.phase && GROUP_COMPONENTS[g.id] !== undefined);
}

export function SettingsView() {
  const { group } = useParams();
  const groups = visibleSettingsGroups();
  const current = groups.find((g) => g.id === group);
  const Group = current ? GROUP_COMPONENTS[current.id] : undefined;
  if (!current || !Group) return <NotFoundView />;
  return (
    <div className="settings">
      <nav className="settings__nav" aria-label={copy.nav}>
        <ul>
          {groups.map((g) => (
            <li key={g.id}><NavLink to={`/settings/${g.id}`}>{g.label}</NavLink></li>
          ))}
        </ul>
      </nav>
      <section className="settings__panel" aria-labelledby="settings-title">
        <h1 id="settings-title">{current.label}</h1>
        <Group />
      </section>
    </div>
  );
}
```

Create `frontend/src/views/settings/settings.css`:

```css
.settings { display: grid; grid-template-columns: 220px minmax(0, 1fr); gap: var(--space-6); padding: var(--space-4) var(--space-6); min-height: 0; }
.settings__nav ul { position: sticky; top: 0; margin: 0; padding: 0; list-style: none; display: flex; flex-direction: column; gap: var(--space-1); }
.settings__nav a { display: block; padding: var(--space-2) var(--space-3); border-radius: var(--radius-1); color: var(--text); text-decoration: none; }
.settings__nav a[aria-current="page"] { background: var(--surface-2); color: var(--accent); font-weight: 600; }
.settings__panel { display: flex; flex-direction: column; gap: var(--space-6); min-width: 0; max-width: 860px; }
.settings__section { display: flex; flex-direction: column; gap: var(--space-3); }
.settings__actions { display: flex; flex-wrap: wrap; gap: var(--space-2); }
.settings__links { margin: 0; padding-left: var(--space-4); display: flex; flex-direction: column; gap: var(--space-1); }
.facts { display: grid; grid-template-columns: max-content minmax(0, 1fr); gap: var(--space-2) var(--space-6); margin: 0; }
.fact { display: contents; }
.fact dt { color: var(--text-muted); }
.fact dd { margin: 0; overflow-wrap: anywhere; }
.capture { padding: var(--space-4); border: 2px dashed var(--border); border-radius: var(--radius-2); text-align: center; font-family: var(--font-mono); }
.capture:focus { border-color: var(--focus); outline: none; }
kbd { padding: 1px var(--space-1); border: 1px solid var(--border); border-radius: var(--radius-1); background: var(--surface); font-family: var(--font-mono); font-size: var(--fs-xs); }
```

In `frontend/src/app/routes.tsx` add `import { Navigate } from 'react-router-dom';` (merge into the existing `react-router-dom` type import line as `import { Navigate, type RouteObject } from 'react-router-dom';`) and `import { SettingsView } from '../views/settings/SettingsView';`, then append to `shellChildren`:

```tsx
  { path: 'settings', element: <Navigate to="/settings/kite" replace /> },
  { path: 'settings/:group', element: <SettingsView /> },
```

- [ ] **Step 4: Run the suite to verify it passes**

Run: `cd frontend && npx vitest run && npm run typecheck`
Expected: PASS (37 test files), `tsc` exits 0.

- [ ] **Step 5: Commit**

```bash
git add frontend/src
git commit -m "feat(frontend): Settings view with Kite connection, appearance, chart and about groups (P0-4 Task 14)" \
  -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---
