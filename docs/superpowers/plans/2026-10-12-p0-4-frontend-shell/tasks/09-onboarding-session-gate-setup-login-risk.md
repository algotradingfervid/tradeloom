> **P0-4 Frontend Shell, Account & Data UI Implementation Plan — Task 9.** Read [`../MANIFEST.md`](../MANIFEST.md) first: its Global Constraints,
> Review Focus and Consumes/Produces apply to this task. Other tasks live beside this file in `tasks/`.

<!-- task body (verbatim from the plan) -->
### Task 9: Onboarding — session gate, setup (incl. secret-storage fallback), login, risk disclosure, expiry banner, last-view restore

**Files:**
- Create: `frontend/src/app/gate.ts`, `frontend/src/app/SessionGate.tsx`, `frontend/src/app/RestoreLastRoute.tsx`, `frontend/src/app/ShellBanners.tsx`
- Create: `frontend/src/views/onboarding/OnboardingFrame.tsx`, `SetupView.tsx`, `LoginView.tsx`, `RiskDisclosureView.tsx`, `onboarding.css` (under `frontend/src/views/onboarding/`)
- Create: `frontend/src/legal/content.ts`
- Overwrite: `frontend/src/app/routes.tsx`
- Modify: `frontend/src/app/RootLayout.tsx`, `frontend/src/app/ShellLayout.tsx` (`banner`), `frontend/src/app/shell.css`, `frontend/src/i18n/copy.ts`
- Test: `frontend/src/app/gate.test.ts`, `frontend/src/views/onboarding/onboarding.test.tsx`, `frontend/e2e/onboarding.spec.ts`

**Interfaces:**
- Consumes: Task 2 `Backend.session.{saveCredentials,secretStorage,useEncryptedFile,refuseSecretStorage,unlock,login,completeLogin,cancelLogin,ackRiskDisclosure}`, `SessionStatus` (`state`, `riskAckRequired`, `credentials.redirectUrl`, `secrets.mode`), `SecretStorageInfo.choices`, `LoginStart` (`listenerOk`, `listenerError`), `useSessionStore`, `useSetting`, `useSettingsStore`; Task 3 `TextField`, `Button`; Task 4 `pushToast`; Task 6 `safeLastRoute`; canonical facts A2 (session states) and A3 (login).
- Produces:
  - `gate.ts`: `ONBOARDING_ROUTES`, `gateTarget(session: SessionStatus, path: string): string | null`.
  - `SessionGate({ children })` — renders a "Starting…" status until the session is known and while redirecting.
  - `RestoreLastRoute()` — index route; navigates to `safeLastRoute(ui.last_route)` once settings are loaded.
  - `ShellBanners()` — "Kite session expired — log in to continue" banner with Log in (FR-ACCT-006); Tasks 15 and 17 add the crash notice and the calendar-stale check here.
  - `SetupView()`, `LoginView()`, `RiskDisclosureView()`, `OnboardingFrame({ title; children })`.
  - `legal/content.ts`: `interface LegalPage { title: string; updatedFor: string; paragraphs: string[] }`, `riskDisclosure: LegalPage` (Task 18 adds `termsOfUse`, `privacyNotice`, `LEGAL_PAGES`).
  - Routes outside the shell: `/setup`, `/login`, `/risk`; index `/` = `RestoreLastRoute`. `/legal/*` and `/help/*` stay reachable before login.

- [ ] **Step 1: Write the failing tests**

Create `frontend/src/app/gate.test.ts`:

```ts
import { describe, expect, it } from 'vitest';
import { readySession } from '../api/mock/fixtures';
import { gateTarget } from './gate';

describe('session gate', () => {
  it('FR-ACCT-005 AC1: no credentials → setup before any other view', () => {
    expect(gateTarget(readySession({ state: 'no_credentials' }), '/home')).toBe('/setup');
    expect(gateTarget(readySession({ state: 'locked' }), '/')).toBe('/setup');
    expect(gateTarget(readySession({ state: 'no_credentials' }), '/setup')).toBeNull();
  });
  it('logged out (or a login in progress) → login screen', () => {
    expect(gateTarget(readySession({ state: 'logged_out' }), '/holdings')).toBe('/login');
    expect(gateTarget(readySession({ state: 'logging_in' }), '/home')).toBe('/login');
    expect(gateTarget(readySession({ state: 'logging_in' }), '/login')).toBeNull();
  });
  it('FR-SHELL-020 AC1: a fresh login shows the risk disclosure before any other view', () => {
    expect(gateTarget(readySession({ riskAckRequired: true }), '/home')).toBe('/risk');
    expect(gateTarget(readySession({ riskAckRequired: true }), '/risk')).toBeNull();
  });
  it('FR-SHELL-020 AC3 / FR-ACCT-006 AC4: an acknowledged, still-valid session goes straight in', () => {
    expect(gateTarget(readySession(), '/home')).toBeNull();
    expect(gateTarget(readySession(), '/login')).toBe('/');
  });
  it('an expiry while running keeps the user where they are (banner instead)', () => {
    expect(gateTarget(readySession({ state: 'expired' }), '/holdings')).toBeNull();
    expect(gateTarget(readySession({ state: 'expired' }), '/login')).toBeNull();
  });
  it('legal pages and help are reachable before login', () => {
    expect(gateTarget(readySession({ state: 'no_credentials' }), '/legal/terms')).toBeNull();
    expect(gateTarget(readySession({ state: 'logged_out' }), '/help/kite-setup')).toBeNull();
  });
});
```

Create `frontend/src/views/onboarding/onboarding.test.tsx`:

```tsx
import { act, screen, waitFor } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import {
  demoSeed, freshSeed, lockedSeed, loggedOutSeed, noKeyringSeed, readySession, unackedSeed,
} from '../../api/mock/fixtures';
import { createMockBackend, type MockState } from '../../api/mock/index';
import { renderApp } from '../../test/render';

const start = (seed: MockState) => renderApp({ route: '/', backend: createMockBackend(seed) });

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
    await user.type(screen.getByLabelText('API secret'), 's3cret-value');
    await user.click(screen.getByRole('button', { name: 'Save and continue' }));
    expect(backend.callsTo('session.saveCredentials')[0].args).toEqual([{ apiKey: 'kiteapikey9f2c', apiSecret: 's3cret-value' }]);
    expect(await screen.findByRole('heading', { name: 'Log in to Kite' })).toBeInTheDocument();
  });
  it('copies the redirect URL', async () => {
    const { user } = start(freshSeed());
    await user.click(await screen.findByRole('button', { name: 'Copy' }));
    expect(await screen.findByText('Redirect URL copied')).toBeInTheDocument();
    await expect(navigator.clipboard.readText()).resolves.toBe('http://127.0.0.1:7420/kite/callback');
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
  it('a locked encrypted file asks for the passphrase and rejects a wrong one inline', async () => {
    const { user } = start(lockedSeed());
    await user.type(await screen.findByLabelText('Passphrase'), 'wrong passphrase');
    await user.click(screen.getByRole('button', { name: 'Unlock' }));
    expect(await screen.findByText('Wrong passphrase')).toBeInTheDocument();
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
    expect(await screen.findAllByText("That isn't a Kite redirect URL or request token")).toHaveLength(2);
    await user.clear(field);
    await user.type(field, 'http://127.0.0.1:7420/kite/callback?request_token=abc123XYZ&status=success');
    await user.click(screen.getByRole('button', { name: 'Complete login' }));
    expect(await screen.findByRole('heading', { name: 'Before you start' })).toBeInTheDocument();
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
  it('restores ui.last_route on start', async () => {
    const seed = demoSeed();
    seed.settings = { 'ui.last_route': '"/positions"' };
    const { router } = start(seed);
    await waitFor(() => expect(router.state.location.pathname).toBe('/positions'));
  });
  it('falls back to Home for a view this build does not have', async () => {
    const seed = demoSeed();
    seed.settings = { 'ui.last_route': '"/scalper"' };
    const { router } = start(seed);
    await waitFor(() => expect(router.state.location.pathname).toBe('/home'));
  });
});
```

- [ ] **Step 2: Run them to verify they fail**

Run: `cd frontend && npx vitest run src/app/gate.test.ts src/views/onboarding`
Expected: FAIL — `Failed to resolve import "./gate"`; onboarding tests time out (no setup/login views).

- [ ] **Step 3: Implement the gate, onboarding views, banner and routes**

Append to `frontend/src/i18n/copy.ts`:

```ts
export const gateCopy = { starting: 'Starting Tradeloom…' } as const;

export const setupCopy = {
  brand: 'Tradeloom',
  title: 'Connect your Kite account',
  intro: 'Tradeloom talks to Zerodha Kite through your own Kite Connect app. Charts, live prices and backtests need the paid Connect plan; holdings, positions and orders also work on the free Personal plan.',
  step1: 'In the Kite developer console, create an app and register this redirect URL:',
  redirectLabel: 'Redirect URL',
  redirectHint: 'You can change the port later in Settings → Kite connection.',
  copy: 'Copy',
  copied: 'Redirect URL copied',
  copyUnavailable: 'Copy is not available here — select the URL and copy it',
  step2: "Then enter the app's API key and API secret. The secret is kept in your system keyring, never in files or logs.",
  apiKey: 'API key',
  apiSecret: 'API secret',
  apiKeyRequired: 'Enter the API key',
  secretRequired: 'Enter the API secret',
  save: 'Save and continue',
  helpLink: 'How to create a Kite Connect app',
  storeTitle: 'Choose where to keep your Kite secret',
  noKeyring: 'No system keyring was found on this computer. Choose one of these two options.',
  storeLegend: 'Secret storage',
  encrypted_file: 'Encrypted file protected by a passphrase',
  encfileHint: 'The secret is encrypted on disk. You enter the passphrase once each time Tradeloom starts.',
  no_storage: "Don't store credentials",
  refuseHint: 'Tradeloom keeps no Kite credentials and therefore cannot connect to Kite.',
  refusedNotice: 'Credentials are not stored, so Tradeloom cannot connect to Kite. Choose the encrypted file to continue, or close the app.',
  passphrase: 'Passphrase',
  passphraseRepeat: 'Repeat passphrase',
  passphraseShort: 'Use at least 8 characters',
  passphraseMismatch: 'The passphrases do not match',
  continue: 'Continue',
  unlockTitle: 'Unlock your Kite credentials',
  unlockIntro: 'Enter the passphrase that protects your encrypted secrets file.',
  unlock: 'Unlock',
} as const;

export const loginCopy = {
  title: 'Log in to Kite',
  intro: 'Kite sessions end every morning, so you log in once a day. Your password and TOTP are entered only on the Kite page in your browser.',
  expired: 'Your Kite session has ended. Log in to continue.',
  button: 'Log in with Kite',
  waiting: 'Waiting for the Kite login to finish in your browser…',
  cancel: 'Cancel login',
  pasteToggle: 'Login did not come back to Tradeloom?',
  pasteLabel: 'Redirect URL or request token',
  pasteHint: 'After logging in, copy the full address from the browser tab (it contains request_token=) and paste it here.',
  pasteRequired: 'Paste the redirect URL or the request token',
  pasteSubmit: 'Complete login',
} as const;

export const riskCopy = { heading: 'Before you start', terms: 'Terms of use', privacy: 'Privacy notice', accept: 'I understand' } as const;

export const bannerCopy = { expired: 'Kite session expired — log in to continue', logIn: 'Log in' } as const;
```

Create `frontend/src/legal/content.ts`:

```ts
// Our own text (FR-ACCT-015, FR-SHELL-020). No sentence is taken from any other product.
export interface LegalPage { title: string; updatedFor: string; paragraphs: string[] }

export const riskDisclosure: LegalPage = {
  title: 'Risk disclosure',
  updatedFor: '0.1.0',
  paragraphs: [
    'Tradeloom is an educational and decision-support tool. It helps you study markets, test ideas and follow your Kite account; it does not give investment advice.',
    'Backtest results and paper trades are hypothetical. They are computed on past or simulated prices and cannot include every real cost, delay or rejection. Past results do not predict future returns.',
    'You are responsible for every order sent from your Kite account, including orders you confirm in Tradeloom. Trading equities and futures can lose more money than you expect.',
    'Tradeloom never sends an order on its own unless you explicitly arm automatic execution for a deployment, and arming ends with each Kite session.',
  ],
};
```

Create `frontend/src/app/gate.ts`:

```ts
import type { SessionStatus } from '../api/backend';

export const ONBOARDING_ROUTES: readonly string[] = ['/setup', '/login', '/risk'];
const ALWAYS_ALLOWED = [/^\/legal\/[a-z-]+$/, /^\/help(\/[a-z0-9-]+)?$/];

/** Where the session requires the user to be, or null to stay (FR-ACCT-005/006, FR-SHELL-020). */
export function gateTarget(session: SessionStatus, path: string): string | null {
  if (ALWAYS_ALLOWED.some((r) => r.test(path))) return null;
  switch (session.state) {
    case 'no_credentials':
    case 'locked':
      return path === '/setup' ? null : '/setup';
    case 'logged_out':
    case 'logging_in':
      return path === '/login' ? null : '/login';
    case 'expired':
      return null;
    case 'ready':
      if (session.riskAckRequired) return path === '/risk' ? null : '/risk';
      return ONBOARDING_ROUTES.includes(path) ? '/' : null;
  }
}
```

Create `frontend/src/app/SessionGate.tsx`:

```tsx
import { useEffect, type ReactNode } from 'react';
import { useLocation, useNavigate } from 'react-router-dom';
import { gateCopy } from '../i18n/copy';
import { useSessionStore } from '../store/session';
import { gateTarget } from './gate';

export function SessionGate({ children }: { children: ReactNode }) {
  const session = useSessionStore((s) => s.session);
  const { pathname } = useLocation();
  const navigate = useNavigate();
  const target = session ? gateTarget(session, pathname) : null;
  useEffect(() => {
    if (target) void navigate(target, { replace: true });
  }, [target, navigate]);
  if (!session || target) return <div className="boot" role="status">{gateCopy.starting}</div>;
  return <>{children}</>;
}
```

Create `frontend/src/app/RestoreLastRoute.tsx`:

```tsx
import { Navigate } from 'react-router-dom';
import { gateCopy } from '../i18n/copy';
import { useSetting, useSettingsStore } from '../store/settings';
import { safeLastRoute } from './lastRoute';

export function RestoreLastRoute() {
  const loaded = useSettingsStore((s) => s.loaded);
  const [last] = useSetting<string>('ui.last_route', '/home');
  if (!loaded) return <div className="boot" role="status">{gateCopy.starting}</div>;
  return <Navigate to={safeLastRoute(last)} replace />;
}
```

Create `frontend/src/app/ShellBanners.tsx`:

```tsx
import { useNavigate } from 'react-router-dom';
import { Button } from '../components/Button';
import { Icon } from '../components/Icon';
import { bannerCopy } from '../i18n/copy';
import { useSessionStore } from '../store/session';

export function ShellBanners() {
  const navigate = useNavigate();
  const expired = useSessionStore((s) => s.session?.state === 'expired');
  return (
    <>
      {expired && (
        <div className="banner banner--warn" role="status">
          <Icon name="warning" />
          <span>{bannerCopy.expired}</span>
          <Button variant="primary" onClick={() => void navigate('/login')}>{bannerCopy.logIn}</Button>
        </div>
      )}
    </>
  );
}
```

Create `frontend/src/views/onboarding/OnboardingFrame.tsx`:

```tsx
import type { ReactNode } from 'react';
import { setupCopy } from '../../i18n/copy';
import './onboarding.css';

export function OnboardingFrame({ title, children }: { title: string; children: ReactNode }) {
  return (
    <main className="onboarding">
      <div className="onboarding__card">
        <span className="onboarding__brand">{setupCopy.brand}</span>
        <h1>{title}</h1>
        {children}
      </div>
    </main>
  );
}
```

Create `frontend/src/views/onboarding/SetupView.tsx`:

```tsx
import { useEffect, useId, useState, type FormEvent } from 'react';
import { Link } from 'react-router-dom';
import type { SecretChoice } from '../../api/backend';
import { useBackend } from '../../api/BackendContext';
import { parseBackendError } from '../../api/errors';
import { Button } from '../../components/Button';
import { TextField } from '../../components/TextField';
import { setupCopy as copy } from '../../i18n/copy';
import { useSessionStore } from '../../store/session';
import { pushToast } from '../../store/toasts';
import { OnboardingFrame } from './OnboardingFrame';

function reportFailure(e: unknown): string {
  const reason = parseBackendError(e).reason;
  pushToast({ level: 'error', message: reason });
  return reason;
}

function CredentialsForm({ redirectUrl }: { redirectUrl: string }) {
  const backend = useBackend();
  const [apiKey, setApiKey] = useState('');
  const [secret, setSecret] = useState('');
  const [errors, setErrors] = useState<{ apiKey?: string; secret?: string; form?: string }>({});
  const [busy, setBusy] = useState(false);

  const copyRedirect = () => {
    if (!navigator.clipboard) {
      pushToast({ level: 'warning', message: copy.copyUnavailable });
      return;
    }
    navigator.clipboard.writeText(redirectUrl).then(
      () => pushToast({ level: 'success', message: copy.copied }),
      () => pushToast({ level: 'warning', message: copy.copyUnavailable }),
    );
  };

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    const next = { apiKey: apiKey.trim() ? undefined : copy.apiKeyRequired, secret: secret ? undefined : copy.secretRequired };
    setErrors(next);
    if (next.apiKey || next.secret) return;
    setBusy(true);
    try {
      await backend.session.saveCredentials({ apiKey: apiKey.trim(), apiSecret: secret });
    } catch (err) {
      setErrors({ form: reportFailure(err) });
    } finally {
      setSecret('');
      setBusy(false);
    }
  };

  return (
    <form onSubmit={submit} noValidate>
      <p>{copy.intro}</p>
      <p>{copy.step1}</p>
      <div className="row">
        <TextField label={copy.redirectLabel} value={redirectUrl} readOnly hint={copy.redirectHint} />
        <Button icon="copy" onClick={copyRedirect}>{copy.copy}</Button>
      </div>
      <p>{copy.step2}</p>
      <TextField label={copy.apiKey} value={apiKey} onChange={(e) => setApiKey(e.target.value)} error={errors.apiKey} autoComplete="off" spellCheck={false} />
      <TextField label={copy.apiSecret} type="password" value={secret} onChange={(e) => setSecret(e.target.value)} error={errors.secret} autoComplete="off" />
      {errors.form && <p className="field__error" role="alert">{errors.form}</p>}
      <Button type="submit" variant="primary" disabled={busy}>{copy.save}</Button>
      <Link to="/help/kite-setup">{copy.helpLink}</Link>
    </form>
  );
}

const CHOICE_HINTS: Record<SecretChoice, string> = { encrypted_file: copy.encfileHint, no_storage: copy.refuseHint };

/** FR-ACCT-013: renders exactly the choices Go's `secrets.Detect` offers (`Session.SecretStorage().choices`). */
function SecretStoreChoice({ refused }: { refused: boolean }) {
  const backend = useBackend();
  const hintIds: Record<SecretChoice, string> = { encrypted_file: useId(), no_storage: useId() };
  const [choices, setChoices] = useState<SecretChoice[]>([]);
  const [choice, setChoice] = useState<SecretChoice>('encrypted_file');
  const [pass, setPass] = useState('');
  const [repeat, setRepeat] = useState('');
  const [errors, setErrors] = useState<{ pass?: string; repeat?: string; form?: string }>({});

  useEffect(() => {
    let live = true;
    backend.session.secretStorage().then(
      (info) => {
        if (live) setChoices(info.choices);
      },
      (err: unknown) => {
        if (live) setErrors({ form: reportFailure(err) });
      },
    );
    return () => {
      live = false;
    };
  }, [backend]);

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    try {
      if (choice === 'no_storage') {
        await backend.session.refuseSecretStorage();
        return;
      }
      const next = {
        pass: pass.length >= 8 ? undefined : copy.passphraseShort,
        repeat: pass.length >= 8 && pass !== repeat ? copy.passphraseMismatch : undefined,
      };
      setErrors(next);
      if (next.pass || next.repeat) return;
      await backend.session.useEncryptedFile(pass);
    } catch (err) {
      setErrors({ form: reportFailure(err) });
    }
  };

  return (
    <form onSubmit={submit} noValidate>
      <p>{copy.noKeyring}</p>
      {refused && <p className="onboarding__notice" role="status">{copy.refusedNotice}</p>}
      <fieldset>
        <legend>{copy.storeLegend}</legend>
        {choices.map((c) => (
          <div className="choice" key={c}>
            <label>
              <input type="radio" name="secret-store" value={c} checked={choice === c} onChange={() => setChoice(c)} aria-describedby={hintIds[c]} />
              {copy[c]}
            </label>
            <span id={hintIds[c]} className="muted">{CHOICE_HINTS[c]}</span>
          </div>
        ))}
      </fieldset>
      {choice === 'encrypted_file' && (
        <>
          <TextField label={copy.passphrase} type="password" value={pass} onChange={(e) => setPass(e.target.value)} error={errors.pass} autoComplete="new-password" />
          <TextField label={copy.passphraseRepeat} type="password" value={repeat} onChange={(e) => setRepeat(e.target.value)} error={errors.repeat} autoComplete="new-password" />
        </>
      )}
      {errors.form && <p className="field__error" role="alert">{errors.form}</p>}
      <Button type="submit" variant="primary">{copy.continue}</Button>
    </form>
  );
}

function UnlockForm() {
  const backend = useBackend();
  const [pass, setPass] = useState('');
  const [error, setError] = useState<string | null>(null);
  const submit = async (e: FormEvent) => {
    e.preventDefault();
    try {
      await backend.session.unlock(pass);
    } catch (err) {
      setError(parseBackendError(err).reason);
    }
  };
  return (
    <form onSubmit={submit} noValidate>
      <p>{copy.unlockIntro}</p>
      <TextField label={copy.passphrase} type="password" value={pass} onChange={(e) => setPass(e.target.value)} error={error} autoComplete="current-password" />
      <Button type="submit" variant="primary">{copy.unlock}</Button>
    </form>
  );
}

export function SetupView() {
  const session = useSessionStore((s) => s.session);
  if (!session) return null;
  if (session.state === 'locked') {
    return <OnboardingFrame title={copy.unlockTitle}><UnlockForm /></OnboardingFrame>;
  }
  // mode "choose": no keyring and no secrets file yet; mode "none": the user refused storage (Kite cannot be used).
  const needsStore = session.secrets.mode === 'choose' || session.secrets.mode === 'none';
  return (
    <OnboardingFrame title={needsStore ? copy.storeTitle : copy.title}>
      {needsStore ? <SecretStoreChoice refused={session.secrets.mode === 'none'} /> : <CredentialsForm redirectUrl={session.credentials.redirectUrl} />}
    </OnboardingFrame>
  );
}
```

Create `frontend/src/views/onboarding/LoginView.tsx`:

```tsx
import { useState, type FormEvent } from 'react';
import { useBackend } from '../../api/BackendContext';
import { parseBackendError } from '../../api/errors';
import { Button } from '../../components/Button';
import { TextField } from '../../components/TextField';
import { loginCopy as copy } from '../../i18n/copy';
import { useSessionStore } from '../../store/session';
import { pushToast } from '../../store/toasts';
import { OnboardingFrame } from './OnboardingFrame';

export function LoginView() {
  const backend = useBackend();
  const session = useSessionStore((s) => s.session);
  const [waiting, setWaiting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [showPaste, setShowPaste] = useState(false);
  const [pasted, setPasted] = useState('');
  const [pasteError, setPasteError] = useState<string | null>(null);

  // Login() opens the system browser and returns LoginStart; success arrives as a "session.state" event (A3).
  const login = async () => {
    setError(null);
    try {
      const start = await backend.session.login();
      if (start.listenerOk) {
        setWaiting(true);
        return;
      }
      // The loopback listener could not start (port busy): the browser still opens, the user pastes the redirect.
      setWaiting(false);
      setError(start.listenerError);
      setShowPaste(true);
    } catch (e) {
      const reason = parseBackendError(e).reason;
      setWaiting(false);
      setError(reason);
      setShowPaste(true);
      pushToast({ level: 'error', message: reason });
    }
  };

  const cancel = async () => {
    setWaiting(false);
    try {
      await backend.session.cancelLogin();
    } catch (e) {
      pushToast({ level: 'error', message: parseBackendError(e).reason });
    }
  };

  const submitPaste = async (e: FormEvent) => {
    e.preventDefault();
    const value = pasted.trim();
    if (!value) {
      setPasteError(copy.pasteRequired);
      return;
    }
    setPasteError(null);
    try {
      await backend.session.completeLogin(value);
    } catch (err) {
      const reason = parseBackendError(err).reason;
      setPasteError(reason);
      pushToast({ level: 'error', message: reason });
    }
  };

  return (
    <OnboardingFrame title={copy.title}>
      {session?.state === 'expired' && <p className="onboarding__notice" role="status">{copy.expired}</p>}
      <p>{copy.intro}</p>
      <Button variant="primary" onClick={login}>{copy.button}</Button>
      {waiting && (
        <div className="row">
          <p role="status">{copy.waiting}</p>
          <Button onClick={cancel}>{copy.cancel}</Button>
        </div>
      )}
      {error && <p className="field__error" role="alert">{error}</p>}
      <button type="button" className="link-button" aria-expanded={showPaste} onClick={() => setShowPaste((v) => !v)}>
        {copy.pasteToggle}
      </button>
      {showPaste && (
        <form onSubmit={submitPaste} noValidate>
          <TextField label={copy.pasteLabel} value={pasted} onChange={(e) => setPasted(e.target.value)} error={pasteError} hint={copy.pasteHint} autoComplete="off" spellCheck={false} />
          <Button type="submit">{copy.pasteSubmit}</Button>
        </form>
      )}
    </OnboardingFrame>
  );
}
```

Create `frontend/src/views/onboarding/RiskDisclosureView.tsx`:

```tsx
import { useState } from 'react';
import { Link } from 'react-router-dom';
import { useBackend } from '../../api/BackendContext';
import { parseBackendError } from '../../api/errors';
import { Button } from '../../components/Button';
import { riskCopy } from '../../i18n/copy';
import { riskDisclosure } from '../../legal/content';
import { pushToast } from '../../store/toasts';
import { OnboardingFrame } from './OnboardingFrame';

export function RiskDisclosureView() {
  const backend = useBackend();
  const [busy, setBusy] = useState(false);
  const ack = async () => {
    setBusy(true);
    try {
      await backend.session.ackRiskDisclosure();
    } catch (e) {
      pushToast({ level: 'error', message: parseBackendError(e).reason });
      setBusy(false);
    }
  };
  return (
    <OnboardingFrame title={riskCopy.heading}>
      {riskDisclosure.paragraphs.map((p) => <p key={p}>{p}</p>)}
      <p>
        <Link to="/legal/terms">{riskCopy.terms}</Link>
        {' · '}
        <Link to="/legal/privacy">{riskCopy.privacy}</Link>
      </p>
      <Button variant="primary" onClick={ack} disabled={busy}>{riskCopy.accept}</Button>
    </OnboardingFrame>
  );
}
```

Create `frontend/src/views/onboarding/onboarding.css`:

```css
.onboarding { min-height: 100vh; display: flex; align-items: center; justify-content: center; padding: var(--space-8); background: var(--surface); }
.onboarding__card {
  width: 560px; max-width: 100%; display: flex; flex-direction: column; gap: var(--space-4); padding: var(--space-8);
  background: var(--bg); border: 1px solid var(--border); border-radius: var(--radius-2); box-shadow: var(--shadow);
}
.onboarding__brand { font-weight: 700; color: var(--accent); letter-spacing: 0.02em; }
.onboarding__notice { padding: var(--space-3); border-radius: var(--radius-1); background: var(--surface-2); }
.onboarding form { display: flex; flex-direction: column; gap: var(--space-3); }
.onboarding fieldset { display: flex; flex-direction: column; gap: var(--space-3); margin: 0; padding: var(--space-3); border: 1px solid var(--border); border-radius: var(--radius-1); }
.choice { display: flex; flex-direction: column; gap: var(--space-1); }
.choice label { display: flex; align-items: center; gap: var(--space-2); font-weight: 600; }
.row { display: flex; align-items: flex-start; gap: var(--space-2); }
.row > .field { flex: 1 1 auto; }
.row > .btn { margin-top: 22px; }
.link-button { align-self: flex-start; padding: 0; border: none; background: none; color: var(--accent); text-decoration: underline; cursor: pointer; }
```

Append to `frontend/src/app/shell.css`:

```css
.boot { display: flex; align-items: center; justify-content: center; height: 100vh; color: var(--text-muted); }
.banner { flex: none; display: flex; align-items: center; gap: var(--space-3); padding: var(--space-2) var(--space-4); background: var(--surface-2); border-bottom: 1px solid var(--border); }
.banner > span { flex: 1 1 auto; }
.banner--warn { border-left: 4px solid var(--warn); }
.banner--error { border-left: 4px solid var(--down); }
```

Overwrite `frontend/src/app/routes.tsx`:

```tsx
import type { RouteObject } from 'react-router-dom';
import { HomeView } from '../views/home/HomeView';
import { NotFoundView } from '../views/NotFoundView';
import { LoginView } from '../views/onboarding/LoginView';
import { RiskDisclosureView } from '../views/onboarding/RiskDisclosureView';
import { SetupView } from '../views/onboarding/SetupView';
import { RestoreLastRoute } from './RestoreLastRoute';
import { RootLayout } from './RootLayout';
import { ShellLayout } from './ShellLayout';

/** Views rendered inside the shell frame. Later tasks (and P0-5: `chart-full`) append entries here. */
export const shellChildren: RouteObject[] = [
  { path: 'home', element: <HomeView /> },
];

/** Root-level children rendered without the shell. */
export const rootChildren: RouteObject[] = [
  { index: true, element: <RestoreLastRoute /> },
  { path: 'setup', element: <SetupView /> },
  { path: 'login', element: <LoginView /> },
  { path: 'risk', element: <RiskDisclosureView /> },
];

export function buildRoutes(): RouteObject[] {
  return [
    {
      path: '/',
      element: <RootLayout />,
      children: [...rootChildren, { element: <ShellLayout />, children: [...shellChildren, { path: '*', element: <NotFoundView /> }] }],
    },
  ];
}
```

In `frontend/src/app/RootLayout.tsx`, add `import { SessionGate } from './SessionGate';` and replace `<Outlet />` inside `<ShortcutProvider>` with `<SessionGate><Outlet /></SessionGate>` (the `ToastRegion` stays outside the gate so onboarding errors are shown).

In `frontend/src/app/ShellLayout.tsx`, add `import { ShellBanners } from './ShellBanners';` and pass `banner={<ShellBanners />}` to `<AppShell>`.

- [ ] **Step 4: Run the suite to verify it passes**

Run: `cd frontend && npx vitest run && npm run typecheck`
Expected: PASS (30 test files), `tsc` exits 0.

- [ ] **Step 5: Add the onboarding Playwright flow and run it**

Create `frontend/e2e/onboarding.spec.ts`:

```ts
import { expect, test } from '@playwright/test';

test('first run: setup → paste login → risk disclosure → Home (FR-ACCT-005 AC1, FR-ACCT-006 AC2, FR-SHELL-020 AC1)', async ({ page }) => {
  await page.goto('/?seed=fresh');
  await expect(page.getByRole('heading', { name: 'Connect your Kite account' })).toBeVisible();
  await page.getByLabel('API key').fill('kiteapikey9f2c');
  await page.getByLabel('API secret').fill('s3cret-value');
  await page.getByRole('button', { name: 'Save and continue' }).click();
  await expect(page.getByRole('heading', { name: 'Log in to Kite' })).toBeVisible();
  await page.getByRole('button', { name: 'Login did not come back to Tradeloom?' }).click();
  await page.getByLabel('Redirect URL or request token').fill('http://127.0.0.1:7420/kite/callback?request_token=abc123XYZ&status=success');
  await page.getByRole('button', { name: 'Complete login' }).click();
  await expect(page.getByRole('heading', { name: 'Before you start' })).toBeVisible();
  await page.getByRole('button', { name: 'I understand' }).click();
  await expect(page.getByRole('heading', { name: 'Home' })).toBeAttached();
  await expect(page.getByRole('button', { name: /^Market data status: Live/ })).toBeVisible();
});

test('loopback login completes from the browser callback (FR-ACCT-006 AC1, UI part)', async ({ page }) => {
  await page.goto('/?seed=loggedout');
  await page.getByRole('button', { name: 'Log in with Kite' }).click();
  await expect(page.getByText(/Waiting for the Kite login/)).toBeVisible();
  await page.evaluate(() => window.__tradeloomMock?.simulateBrowserLogin());
  await expect(page.getByRole('heading', { name: 'Before you start' })).toBeVisible();
});
```

Run: `cd frontend && npx playwright test e2e/onboarding.spec.ts`
Expected: PASS (2 tests × 2 projects: webkit, chromium).

- [ ] **Step 6: Commit**

```bash
git add frontend/src frontend/e2e
git commit -m "feat(frontend): session gate, setup/login/risk views, expiry banner, last-view restore (P0-4 Task 9)" \
  -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---
