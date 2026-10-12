> **P0-4 Frontend Shell, Account & Data UI Implementation Plan — Task 8.** Read [`../MANIFEST.md`](../MANIFEST.md) first: its Global Constraints,
> Review Focus and Consumes/Produces apply to this task. Other tasks live beside this file in `tasks/`.

<!-- task body (verbatim from the plan) -->
### Task 8: Top-bar quick links and profile menu (with theme toggle)

**Files:**
- Create: `frontend/src/app/QuickLinks.tsx`, `frontend/src/app/ProfileMenu.tsx`, `frontend/src/app/useProfile.ts`
- Modify: `frontend/src/app/ShellLayout.tsx` (`links`, `profile` slots), `frontend/src/app/shell.css`, `frontend/src/i18n/copy.ts`
- Test: `frontend/src/app/TopBar.test.tsx`

**Interfaces:**
- Consumes: Task 2 `useSessionStore`, `useAppStore`, `useSetting`, `Backend.session.profile` (`Profile.name`, `userId`, `initials`, `avatarUrl` — P0-3 Task 11); Task 3 `MenuButton`, `MenuItem`, `Tooltip`; Task 4 `pushToast`; Task 1 `useTheme`, `isEnabled`.
- Produces: `QuickLinks()` (Order Book, Positions; the bell appears with `isEnabled('notificationsBell')` in P3); `ProfileMenu()`; `useProfile(): { profile: Profile | null; reload(): Promise<void> }`; `Avatar({ profile: Profile | null; size?: number })`.

- [ ] **Step 1: Write the failing test**

Create `frontend/src/app/TopBar.test.tsx`:

```tsx
import { act, screen, waitFor, within } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { demoSeed, readySession } from '../api/mock/fixtures';
import { createMockBackend } from '../api/mock/index';
import { resetAllStores } from '../store/reset';
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
```

- [ ] **Step 2: Run it to verify it fails**

Run: `cd frontend && npx vitest run src/app/TopBar.test.tsx`
Expected: FAIL — no `Quick links` links and no `Account menu` button are rendered (timeouts in `findByRole`).

- [ ] **Step 3: Implement quick links, profile loading and the profile menu**

Append to `frontend/src/i18n/copy.ts`:

```ts
export const topBarCopy = {
  orders: 'Order Book',
  ordersTip: 'Open the order book',
  positions: 'Positions',
  positionsTip: 'Open positions',
} as const;

export const profileMenuCopy = {
  menu: 'Account menu',
  notLoggedIn: 'Not logged in',
  logIn: 'Log in',
  profile: 'Profile',
  holdings: 'Holdings',
  settings: 'Settings',
  help: 'Help',
  darkTheme: 'Dark theme',
  version: (v: string) => `v${v}`,
  loadFailed: (reason: string) => `Your Kite profile could not be loaded: ${reason}`,
} as const;
```

Create `frontend/src/app/useProfile.ts`:

```ts
import { useCallback, useEffect, useRef } from 'react';
import type { Profile } from '../api/backend';
import { useBackend } from '../api/BackendContext';
import { parseBackendError } from '../api/errors';
import { profileMenuCopy } from '../i18n/copy';
import { useSessionStore } from '../store/session';
import { pushToast } from '../store/toasts';

/** Loads `GET /user/profile` once per logged-in user (FR-SHELL-010, FR-ACCT-001). */
export function useProfile(): { profile: Profile | null; reload: () => Promise<void> } {
  const backend = useBackend();
  const ready = useSessionStore((s) => s.session?.state === 'ready');
  const userId = useSessionStore((s) => s.session?.userId ?? '');
  const profile = useSessionStore((s) => s.profile);
  const requestedFor = useRef<string | null>(null);

  const reload = useCallback(async () => {
    try {
      useSessionStore.getState().setProfile(await backend.session.profile());
    } catch (e) {
      pushToast({ level: 'error', message: profileMenuCopy.loadFailed(parseBackendError(e).reason) });
    }
  }, [backend]);

  useEffect(() => {
    if (!ready || profile || requestedFor.current === userId) return;
    requestedFor.current = userId;
    void reload();
  }, [ready, userId, profile, reload]);

  return { profile, reload };
}
```

Create `frontend/src/app/QuickLinks.tsx`:

```tsx
import { NavLink } from 'react-router-dom';
import { Icon } from '../components/Icon';
import { Tooltip } from '../components/Tooltip';
import { topBarCopy } from '../i18n/copy';

/** FR-SHELL-006. The Notifications bell joins in P3 (isEnabled('notificationsBell')). */
export function QuickLinks() {
  return (
    <>
      <Tooltip label={topBarCopy.ordersTip}>
        <NavLink to="/orders" className="toplink">
          <Icon name="orders" size={16} />
          <span>{topBarCopy.orders}</span>
        </NavLink>
      </Tooltip>
      <Tooltip label={topBarCopy.positionsTip}>
        <NavLink to="/positions" className="toplink">
          <Icon name="positions" size={16} />
          <span>{topBarCopy.positions}</span>
        </NavLink>
      </Tooltip>
    </>
  );
}
```

Create `frontend/src/app/ProfileMenu.tsx`:

```tsx
import { useNavigate } from 'react-router-dom';
import type { Profile } from '../api/backend';
import { Icon } from '../components/Icon';
import { MenuButton, type MenuItem } from '../components/MenuButton';
import type { ThemeName } from '../design/tokens';
import { profileMenuCopy as copy } from '../i18n/copy';
import { useAppStore } from '../store/app';
import { useSessionStore } from '../store/session';
import { useSetting } from '../store/settings';
import { useTheme } from './theme';
import { useProfile } from './useProfile';

export function Avatar({ profile, size = 32 }: { profile: Profile | null; size?: number }) {
  if (!profile) {
    return <span className="avatar" style={{ width: size, height: size }}><Icon name="user" size={size * 0.6} /></span>;
  }
  if (profile.avatarUrl) return <img className="avatar" src={profile.avatarUrl} alt="" width={size} height={size} />;
  return <span className="avatar" style={{ width: size, height: size }} aria-hidden="true">{profile.initials}</span>;
}

export function ProfileMenu() {
  const navigate = useNavigate();
  const loggedIn = useSessionStore((s) => s.session?.state === 'ready');
  const { profile } = useProfile();
  const info = useAppStore((s) => s.info);
  const { theme } = useTheme();
  const [, setTheme] = useSetting<ThemeName>('appearance.theme', 'light');
  const [, setFollowSystem] = useSetting<boolean>('appearance.follow_system', false);
  const go = (path: string) => () => void navigate(path);

  const who: MenuItem =
    loggedIn && profile
      ? { kind: 'static', id: 'who', content: (<div className="profile-head"><strong>{profile.name}</strong><span className="muted">{profile.userId}</span></div>) }
      : { kind: 'static', id: 'who', content: <span>{copy.notLoggedIn}</span> };

  const items: MenuItem[] = [
    who,
    { kind: 'separator', id: 's1' },
    ...(loggedIn ? [] : [{ kind: 'item', id: 'login', label: copy.logIn, icon: 'user', onSelect: go('/login') } satisfies MenuItem]),
    { kind: 'item', id: 'profile', label: copy.profile, icon: 'user', onSelect: go('/profile') },
    { kind: 'item', id: 'holdings', label: copy.holdings, icon: 'holdings', onSelect: go('/holdings') },
    { kind: 'item', id: 'settings', label: copy.settings, icon: 'settings', onSelect: go('/settings/kite') },
    { kind: 'item', id: 'help', label: copy.help, icon: 'help', onSelect: go('/help') },
    { kind: 'separator', id: 's2' },
    {
      kind: 'checkbox', id: 'dark', label: copy.darkTheme, checked: theme === 'dark',
      onChange: (on) => {
        void setFollowSystem(false);
        void setTheme(on ? 'dark' : 'light');
      },
    },
    { kind: 'separator', id: 's3' },
    { kind: 'static', id: 'version', content: <span className="menu__version muted">{info ? copy.version(info.version) : ''}</span> },
  ];

  return <MenuButton label={copy.menu} trigger={<Avatar profile={loggedIn ? profile : null} />} items={items} align="end" />;
}
```

Append to `frontend/src/app/shell.css`:

```css
.toplink {
  display: inline-flex; align-items: center; gap: var(--space-1); padding: var(--space-1) var(--space-2);
  border-radius: var(--radius-1); color: var(--text); text-decoration: none; white-space: nowrap;
}
.toplink:hover { background: var(--surface-2); }
.toplink[aria-current="page"] { color: var(--accent); background: var(--surface-2); font-weight: 600; }
.avatar {
  display: inline-flex; align-items: center; justify-content: center; border-radius: 50%; background: var(--accent);
  color: var(--on-accent); font-size: var(--fs-xs); font-weight: 600; object-fit: cover;
}
.profile-head { display: flex; flex-direction: column; }
.menu__version { display: block; text-align: right; font-size: var(--fs-xs); }
```

Overwrite `frontend/src/app/ShellLayout.tsx`:

```tsx
import { Outlet } from 'react-router-dom';
import { GlobalShortcuts } from '../shortcuts/GlobalShortcuts';
import { useSetting } from '../store/settings';
import { AppShell } from './AppShell';
import { visibleNavEntries } from './features';
import { useRememberLastRoute } from './lastRoute';
import { ProfileMenu } from './ProfileMenu';
import { QuickLinks } from './QuickLinks';
import { Sidebar } from './Sidebar';
import { StatusIndicator } from './StatusIndicator';
import { TopBar } from './TopBar';

export function ShellLayout() {
  const [collapsed, setCollapsed] = useSetting<boolean>('appearance.sidebar_collapsed', false);
  useRememberLastRoute();
  return (
    <AppShell
      topBar={<TopBar ticker={null} status={<StatusIndicator />} links={<QuickLinks />} profile={<ProfileMenu />} />}
      sidebar={<Sidebar entries={visibleNavEntries()} collapsed={collapsed} onToggleCollapsed={() => void setCollapsed(!collapsed)} />}
    >
      <GlobalShortcuts />
      <Outlet />
    </AppShell>
  );
}
```

- [ ] **Step 4: Run the suite to verify it passes**

Run: `cd frontend && npx vitest run && npm run typecheck`
Expected: PASS (28 test files), `tsc` exits 0.

- [ ] **Step 5: Extend the 1280 px smoke test to the top-bar items and run it**

Replace the body of `frontend/e2e/shell.spec.ts` with:

```ts
import { expect, test } from '@playwright/test';

test('FR-SHELL-001 AC2: at 1280×720 no horizontal scrollbar; status, quick links and avatar fully visible', async ({ page }) => {
  await page.setViewportSize({ width: 1280, height: 720 });
  await page.goto('/');
  await expect(page.getByRole('navigation', { name: 'Main navigation' })).toBeVisible();
  const overflow = await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth);
  expect(overflow).toBe(0);
  const targets = [
    page.getByRole('button', { name: /^Market data status/ }),
    page.getByRole('link', { name: 'Order Book' }),
    page.getByRole('link', { name: 'Positions' }),
    page.getByRole('button', { name: 'Account menu' }),
  ];
  // The three ticker pills (P0-5) and the bell (P3) join this list when they ship.
  for (const t of targets) {
    const box = await t.boundingBox();
    expect(box).not.toBeNull();
    expect(box!.x).toBeGreaterThanOrEqual(0);
    expect(box!.y).toBeGreaterThanOrEqual(0);
    expect(box!.x + box!.width).toBeLessThanOrEqual(1280);
  }
});
```

Run: `cd frontend && npx playwright test e2e/shell.spec.ts`
Expected: PASS (1 test × 2 projects: webkit, chromium).

- [ ] **Step 6: Commit**

```bash
git add frontend/src frontend/e2e
git commit -m "feat(frontend): top-bar quick links and profile menu with theme toggle (P0-4 Task 8)" \
  -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---
