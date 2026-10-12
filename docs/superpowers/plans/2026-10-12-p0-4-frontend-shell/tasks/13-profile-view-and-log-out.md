> **P0-4 Frontend Shell, Account & Data UI Implementation Plan — Task 13.** Read [`../MANIFEST.md`](../MANIFEST.md) first: its Global Constraints,
> Review Focus and Consumes/Produces apply to this task. Other tasks live beside this file in `tasks/`.

<!-- task body (verbatim from the plan) -->
### Task 13: Profile view and log out

**Files:**
- Create: `frontend/src/views/profile/ProfileView.tsx`, `frontend/src/views/profile/profile.css`
- Modify: `frontend/src/app/routes.tsx` (`profile`), `frontend/src/i18n/copy.ts`
- Test: `frontend/src/views/profile/ProfileView.test.tsx`

**Interfaces:**
- Consumes: Task 8 `useProfile`, `Avatar`; Task 7 `useNow`; Task 5 `ListPage`; Task 3 `TextField`, `Button`, `ConfirmDialog`, `Skeleton`; Task 2 `Backend.session.logout` (→ `LogoutResult{remoteInvalidated, message}`), `Profile` (P0-3 Task 11: `userId, name, shortName, email, broker, avatarUrl, initials, exchanges, products, orderTypes`), `SessionStatus.loggedInAtMs/expiresAtMs`; Task 4 `pushToast`; Task 1 `formatDateTimeLong`, `formatExpiry`.
- Produces: `ProfileView()` at `/profile` (FR-ACCT-001; preference toggles FR-ACCT-002/003 appear in P3/P4 behind `isEnabled('profileToggles')`).

- [ ] **Step 1: Write the failing test**

Create `frontend/src/views/profile/ProfileView.test.tsx`:

```tsx
import { screen, within } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { FIXED_NOW } from '../../api/mock/fixtures';
import { renderApp } from '../../test/render';

beforeEach(() => {
  vi.useFakeTimers({ toFake: ['Date'] });
  vi.setSystemTime(FIXED_NOW);
});
afterEach(() => vi.useRealTimers());

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
```

- [ ] **Step 2: Run it to verify it fails**

Run: `cd frontend && npx vitest run src/views/profile`
Expected: FAIL — `/profile` renders "Page not found".

- [ ] **Step 3: Implement the view**

Append to `frontend/src/i18n/copy.ts`:

```ts
export const profileCopy = {
  title: 'Profile',
  description: 'Your Zerodha account details as returned by Kite.',
  loading: 'Loading profile',
  fields: { name: 'Name', email: 'Email', clientId: 'Client ID', broker: 'Broker', exchanges: 'Exchanges', products: 'Products', orderTypes: 'Order types' },
  session: 'Session',
  loggedInAt: (t: string) => `Logged in at ${t}`,
  expires: (t: string) => `Expires ${t}`,
  logOut: 'Log out',
  confirmTitle: 'Log out of Kite?',
  confirmMessage: 'Live prices and the order book stop updating until you log in again. Your watchlists, settings and cached candles stay on this computer.',
} as const;
```

Create `frontend/src/views/profile/profile.css`:

```css
.profile-card { display: flex; align-items: center; gap: var(--space-4); }
.profile-fields { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: var(--space-3); max-width: 760px; }
.profile-session { display: flex; flex-direction: column; gap: var(--space-1); }
.profile-actions { display: flex; }
```

Create `frontend/src/views/profile/ProfileView.tsx`:

```tsx
import { useState } from 'react';
import { useBackend } from '../../api/BackendContext';
import { parseBackendError } from '../../api/errors';
import { Avatar } from '../../app/ProfileMenu';
import { useNow } from '../../app/useNow';
import { useProfile } from '../../app/useProfile';
import { Button } from '../../components/Button';
import { ConfirmDialog } from '../../components/ConfirmDialog';
import { ListPage } from '../../components/ListPage';
import { Skeleton } from '../../components/Skeleton';
import { TextField } from '../../components/TextField';
import { profileCopy as copy } from '../../i18n/copy';
import { formatDateTimeLong, formatExpiry } from '../../i18n/format';
import { useSessionStore } from '../../store/session';
import { pushToast } from '../../store/toasts';
import '../../components/list.css';
import './profile.css';

export function ProfileView() {
  const backend = useBackend();
  const { profile, reload } = useProfile();
  const session = useSessionStore((s) => s.session);
  const now = useNow();
  const [confirming, setConfirming] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const logout = async () => {
    setBusy(true);
    setError(null);
    try {
      const result = await backend.session.logout();
      setConfirming(false);
      // The token is always deleted locally; Kite's own invalidation can fail (offline) and is reported, not retried.
      if (!result.remoteInvalidated && result.message) pushToast({ level: 'warning', message: result.message });
    } catch (e) {
      const reason = parseBackendError(e).reason;
      setError(reason);
      pushToast({ level: 'error', message: reason });
    } finally {
      setBusy(false);
    }
  };

  return (
    <ListPage title={copy.title} description={copy.description} onRefresh={() => void reload()}>
      {profile ? (
        <>
          <div className="card profile-card">
            <Avatar profile={profile} size={56} />
            <div>
              <strong>{profile.name}</strong>
              <div className="muted">{profile.userId}<span className="chip">{profile.broker}</span></div>
            </div>
          </div>
          <div className="profile-fields">
            <TextField label={copy.fields.name} value={profile.name} readOnly />
            <TextField label={copy.fields.email} value={profile.email} readOnly />
            <TextField label={copy.fields.clientId} value={profile.userId} readOnly />
            <TextField label={copy.fields.broker} value={profile.broker} readOnly />
            <TextField label={copy.fields.exchanges} value={profile.exchanges.join(', ')} readOnly />
            <TextField label={copy.fields.products} value={profile.products.join(', ')} readOnly />
            <TextField label={copy.fields.orderTypes} value={profile.orderTypes.join(', ')} readOnly />
          </div>
          <section className="profile-session" aria-labelledby="profile-session">
            <h2 id="profile-session">{copy.session}</h2>
            <p>{copy.loggedInAt(formatDateTimeLong(session?.loggedInAtMs))}</p>
            <p>{copy.expires(formatExpiry(session?.expiresAtMs, now))}</p>
          </section>
          <div className="profile-actions">
            <Button variant="danger" icon="logout" onClick={() => setConfirming(true)}>{copy.logOut}</Button>
          </div>
        </>
      ) : (
        <Skeleton rows={4} label={copy.loading} />
      )}
      <ConfirmDialog
        open={confirming}
        title={copy.confirmTitle}
        message={copy.confirmMessage}
        confirmLabel={copy.logOut}
        tone="danger"
        busy={busy}
        error={error}
        onConfirm={() => void logout()}
        onCancel={() => setConfirming(false)}
      />
    </ListPage>
  );
}
```

In `frontend/src/app/routes.tsx` add `import { ProfileView } from '../views/profile/ProfileView';` and append `{ path: 'profile', element: <ProfileView /> },` to `shellChildren`.

- [ ] **Step 4: Run the suite to verify it passes**

Run: `cd frontend && npx vitest run && npm run typecheck`
Expected: PASS (36 test files), `tsc` exits 0.

- [ ] **Step 5: Commit**

```bash
git add frontend/src
git commit -m "feat(frontend): read-only Profile view with session info and log out (P0-4 Task 13)" \
  -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---
