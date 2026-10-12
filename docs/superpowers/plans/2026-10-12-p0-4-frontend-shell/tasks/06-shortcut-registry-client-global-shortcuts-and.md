> **P0-4 Frontend Shell, Account & Data UI Implementation Plan — Task 6.** Read [`../MANIFEST.md`](../MANIFEST.md) first: its Global Constraints,
> Review Focus and Consumes/Produces apply to this task. Other tasks live beside this file in `tasks/`.

<!-- task body (verbatim from the plan) -->
### Task 6: Shortcut registry client, global shortcuts and window state (last view)

**Files:**
- Create: `frontend/src/shortcuts/registry.ts`, `frontend/src/shortcuts/store.ts`, `frontend/src/shortcuts/ShortcutProvider.tsx`, `frontend/src/shortcuts/GlobalShortcuts.tsx`, `frontend/src/app/lastRoute.ts`
- Modify: `frontend/src/app/RootLayout.tsx`, `frontend/src/app/ShellLayout.tsx`
- Test: `frontend/src/shortcuts/registry.test.ts`, `frontend/src/shortcuts/ShortcutProvider.test.tsx`, `frontend/src/app/lastRoute.test.tsx`

**Interfaces:**
- Consumes: Task 2 `Backend.settings.shortcuts()`, `ShortcutAction`, `ShortcutScope`, `useSetting`, `writeSetting`, `useSettingsStore`; Task 4 `focusNewestToast`; Task 1 `visibleNavEntries`; canonical registry A8 (action ids, contract §E).
- Produces:
  - `registry.ts`: `interface KeyBinding { key; ctrl; alt; shift; meta }`, `interface KeyLike { key; code; ctrlKey; altKey; shiftKey; metaKey }`, `parseBinding(text): KeyBinding | null`, `formatBinding(text): string`, `keyOfEvent(e: KeyLike): string`, `bindingMatches(binding, e): boolean`, `bindingFromEvent(e): string | null`, `scopeStackFor(el: Element | null): ShortcutScope[]`, `resolveAction(actions, stack, e, hasHandler?): ShortcutAction | undefined`, `isTypingTarget(t): boolean`.
  - `store.ts`: `useShortcutStore` (`actions: ShortcutAction[]`, `setActions`).
  - `ShortcutProvider({ children })`, `useShortcut(actionId: string, handler: () => void): void`, `reloadShortcuts(backend): Promise<void>`. Scopes come from the nearest `data-shortcut-scope` attributes around the focused element (P0-5 sets `data-shortcut-scope="chart"` on the chart container).
  - `GlobalShortcuts()` — binds `nav.N` for visible sidebar entries, `toasts.focus`, `help.open`.
  - `lastRoute.ts`: `isRememberable(path): boolean`, `safeLastRoute(value: unknown): string`, `useRememberLastRoute(): void` (writes `ui.last_route`).

- [ ] **Step 1: Write the failing tests**

Create `frontend/src/shortcuts/registry.test.ts`:

```ts
import { describe, expect, it } from 'vitest';
import type { ShortcutAction } from '../api/backend';
import { bindingFromEvent, bindingMatches, formatBinding, parseBinding, resolveAction, scopeStackFor } from './registry';

const ev = (key: string, code: string, mods: Partial<{ ctrlKey: boolean; altKey: boolean; shiftKey: boolean; metaKey: boolean }> = {}) => ({
  key, code, ctrlKey: false, altKey: false, shiftKey: false, metaKey: false, ...mods,
});

const actions: ShortcutAction[] = [
  { id: 'toasts.focus', label: 'Focus newest notification', scope: 'global', defaultBinding: 'Alt+T', binding: 'Alt+T' },
  { id: 'chart.trendLine', label: 'Trend line tool', scope: 'chart', defaultBinding: 'Alt+T', binding: 'Alt+T' },
];

describe('shortcut registry', () => {
  it('parses and formats bindings', () => {
    expect(parseBinding('Ctrl+Shift+K')).toEqual({ key: 'K', ctrl: true, alt: false, shift: true, meta: false });
    expect(parseBinding('Alt+↑')?.key).toBe('ArrowUp');
    expect(parseBinding('F11')?.key).toBe('F11');
    expect(parseBinding('')).toBeNull();
    expect(parseBinding('Ctrl+')).toBeNull();
    expect(formatBinding('ctrl+shift+k')).toBe('Ctrl+Shift+K');
  });
  it('matches by physical key so Alt+letter works on any layout', () => {
    expect(bindingMatches('Alt+T', ev('†', 'KeyT', { altKey: true }))).toBe(true);
    expect(bindingMatches('Alt+1', ev('1', 'Digit1', { altKey: true }))).toBe(true);
    expect(bindingMatches('Alt+1', ev('1', 'Digit1', { altKey: true, shiftKey: true }))).toBe(false);
  });
  it('builds a binding from a key event and ignores lone modifiers', () => {
    expect(bindingFromEvent(ev('Shift', 'ShiftLeft', { shiftKey: true }))).toBeNull();
    expect(bindingFromEvent(ev('x', 'KeyX', { ctrlKey: true, shiftKey: true }))).toBe('Ctrl+Shift+X');
  });
  it('innermost scope wins; falls back when that scope has no handler', () => {
    const e = ev('t', 'KeyT', { altKey: true });
    expect(resolveAction(actions, ['global', 'view', 'chart'], e)?.id).toBe('chart.trendLine');
    expect(resolveAction(actions, ['global', 'view'], e)?.id).toBe('toasts.focus');
    expect(resolveAction(actions, ['global', 'view', 'chart'], e, (id) => id !== 'chart.trendLine')?.id).toBe('toasts.focus');
  });
  it('derives the scope stack from data-shortcut-scope ancestors', () => {
    const outer = document.createElement('div');
    outer.dataset.shortcutScope = 'chart';
    const inner = document.createElement('button');
    outer.appendChild(inner);
    expect(scopeStackFor(inner)).toEqual(['global', 'view', 'chart']);
    expect(scopeStackFor(null)).toEqual(['global', 'view']);
  });
});
```

Create `frontend/src/shortcuts/ShortcutProvider.test.tsx`:

```tsx
import { act, render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import type { ReactNode } from 'react';
import { createMemoryRouter, RouterProvider, useLocation } from 'react-router-dom';
import { describe, expect, it, vi } from 'vitest';
import { BackendProvider } from '../api/BackendContext';
import { demoSeed } from '../api/mock/fixtures';
import { createMockBackend } from '../api/mock/index';
import { ToastRegion } from '../components/ToastRegion';
import { pushToast } from '../store/toasts';
import { GlobalShortcuts } from './GlobalShortcuts';
import { reloadShortcuts, ShortcutProvider, useShortcut } from './ShortcutProvider';
import { useShortcutStore } from './store';

function Where() {
  return <output data-testid="where">{useLocation().pathname}</output>;
}

function Probe({ id, onFire }: { id: string; onFire: () => void }) {
  useShortcut(id, onFire);
  return null;
}

function setup(extra?: ReactNode) {
  const backend = createMockBackend(demoSeed());
  const router = createMemoryRouter(
    [{ path: '*', element: (<ShortcutProvider><GlobalShortcuts /><Where /><input aria-label="Notes" />{extra}<ToastRegion /></ShortcutProvider>) }],
    { initialEntries: ['/orders'] },
  );
  render(<BackendProvider backend={backend}><RouterProvider router={router} /></BackendProvider>);
  return { backend, user: userEvent.setup() };
}

const loaded = () => waitFor(() => expect(useShortcutStore.getState().actions.length).toBeGreaterThan(0));
const where = () => screen.getByTestId('where');

describe('FR-SHELL-019 global shortcuts', () => {
  it('Alt+1 goes Home; Alt+3 (a later-phase sidebar entry) does nothing in P0', async () => {
    const { user } = setup();
    await loaded();
    await user.keyboard('{Alt>}3{/Alt}');
    expect(where()).toHaveTextContent('/orders');
    await user.keyboard('{Alt>}1{/Alt}');
    expect(where()).toHaveTextContent('/home');
  });
  it('F1 opens help, even while typing in a field', async () => {
    const { user } = setup();
    await loaded();
    await user.click(screen.getByLabelText('Notes'));
    await user.keyboard('{F1}');
    expect(where()).toHaveTextContent('/help');
  });
  it('plain-key bindings are ignored while typing', async () => {
    const fire = vi.fn();
    const { user } = setup(<div data-shortcut-scope="chart"><input aria-label="Chart note" /><Probe id="order.new" onFire={fire} /></div>);
    await loaded();
    await user.click(screen.getByLabelText('Chart note'));
    await user.keyboard('{Shift>}t{/Shift}');
    expect(fire).not.toHaveBeenCalled();
  });
  it('FR-SHELL-009 AC5 + FR-SHELL-019 AC4: Alt+T focuses the newest toast, but on a chart it selects the trend line', async () => {
    const trend = vi.fn();
    const { user } = setup(<div data-shortcut-scope="chart" tabIndex={0} data-testid="chart"><Probe id="chart.trendLine" onFire={trend} /></div>);
    await loaded();
    act(() => { pushToast({ level: 'error', message: 'Holdings could not be loaded' }); });
    const toast = screen.getByText('Holdings could not be loaded').closest('.toast') as HTMLElement;
    screen.getByTestId('chart').focus();
    await user.keyboard('{Alt>}t{/Alt}');
    expect(trend).toHaveBeenCalledTimes(1);
    expect(toast).not.toHaveFocus();
    act(() => { (document.activeElement as HTMLElement).blur(); });
    await user.keyboard('{Alt>}t{/Alt}');
    expect(toast).toHaveFocus();
  });
  it('FR-SHELL-019 AC1 (client part): after remapping the kill switch only the new binding fires', async () => {
    const kill = vi.fn();
    const { backend, user } = setup(<Probe id="kill.switch" onFire={kill} />);
    await loaded();
    await backend.settings.rebind('kill.switch', 'Ctrl+Shift+X');
    await act(() => reloadShortcuts(backend));
    await user.keyboard('{Control>}{Shift>}k{/Shift}{/Control}');
    expect(kill).not.toHaveBeenCalled();
    await user.keyboard('{Control>}{Shift>}x{/Shift}{/Control}');
    expect(kill).toHaveBeenCalledTimes(1);
  });
});
```

Create `frontend/src/app/lastRoute.test.tsx`:

```tsx
import { act, waitFor } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { renderApp } from '../test/render';
import { isRememberable, safeLastRoute } from './lastRoute';

describe('FR-SHELL-019 window state — last view', () => {
  it('remembers only real P0 shell views', () => {
    expect(isRememberable('/holdings')).toBe(true);
    expect(isRememberable('/settings/data')).toBe(true);
    expect(isRememberable('/login')).toBe(false);
    expect(isRememberable('/scalper')).toBe(false);
    expect(safeLastRoute('/scalper')).toBe('/home');
    expect(safeLastRoute(42)).toBe('/home');
    expect(safeLastRoute('/orders')).toBe('/orders');
  });
  it('writes ui.last_route as the user moves and ignores unknown views', async () => {
    const { backend, router } = renderApp({ route: '/home' });
    await waitFor(() => expect(backend.state.settings['ui.last_route']).toBe('"/home"'));
    await act(() => router.navigate('/no-such-view'));
    expect(backend.state.settings['ui.last_route']).toBe('"/home"');
  });
});
```

- [ ] **Step 2: Run them to verify they fail**

Run: `cd frontend && npx vitest run src/shortcuts src/app/lastRoute.test.tsx`
Expected: FAIL — `Failed to resolve import "./registry"`, `"./ShortcutProvider"`, `"./lastRoute"`.

- [ ] **Step 3: Implement the registry client, provider, global shortcuts and last-view memory**

Create `frontend/src/shortcuts/registry.ts`:

```ts
import type { ShortcutAction, ShortcutScope } from '../api/backend';

export interface KeyBinding { key: string; ctrl: boolean; alt: boolean; shift: boolean; meta: boolean }
export interface KeyLike { key: string; code: string; ctrlKey: boolean; altKey: boolean; shiftKey: boolean; metaKey: boolean }

const ALIASES: Record<string, string> = {
  up: 'ArrowUp', down: 'ArrowDown', left: 'ArrowLeft', right: 'ArrowRight',
  '↑': 'ArrowUp', '↓': 'ArrowDown', '←': 'ArrowLeft', '→': 'ArrowRight', esc: 'Escape', space: 'Space',
};

function normaliseKey(k: string): string {
  const alias = ALIASES[k.toLowerCase()];
  if (alias) return alias;
  if (/^f\d{1,2}$/i.test(k)) return k.toUpperCase();
  if (k.length === 1) return k.toUpperCase();
  return k[0].toUpperCase() + k.slice(1);
}

export function parseBinding(text: string): KeyBinding | null {
  if (!text) return null;
  const b: KeyBinding = { key: '', ctrl: false, alt: false, shift: false, meta: false };
  for (const raw of text.split('+')) {
    const part = raw.trim();
    if (!part) continue;
    const l = part.toLowerCase();
    if (l === 'ctrl' || l === 'control') b.ctrl = true;
    else if (l === 'alt') b.alt = true;
    else if (l === 'shift') b.shift = true;
    else if (l === 'meta' || l === 'super') b.meta = true;
    else b.key = normaliseKey(part);
  }
  return b.key ? b : null;
}

export function formatBinding(text: string): string {
  const b = parseBinding(text);
  if (!b) return '';
  return [b.ctrl && 'Ctrl', b.alt && 'Alt', b.shift && 'Shift', b.meta && 'Meta', b.key].filter(Boolean).join('+');
}

/** Physical key for letters and digits (Alt+letter differs per layout), logical key otherwise. */
export function keyOfEvent(e: KeyLike): string {
  if (/^Key[A-Z]$/.test(e.code)) return e.code.slice(3);
  if (/^Digit\d$/.test(e.code)) return e.code.slice(5);
  if (e.key === ' ') return 'Space';
  return e.key.length === 1 ? e.key.toUpperCase() : e.key;
}

export function bindingMatches(binding: string, e: KeyLike): boolean {
  const b = parseBinding(binding);
  return b !== null && keyOfEvent(e) === b.key && e.ctrlKey === b.ctrl && e.altKey === b.alt && e.shiftKey === b.shift && e.metaKey === b.meta;
}

const MODIFIERS = new Set(['Control', 'Alt', 'Shift', 'Meta', 'AltGraph']);

export function bindingFromEvent(e: KeyLike): string | null {
  if (MODIFIERS.has(e.key)) return null;
  const parts: string[] = [];
  if (e.ctrlKey) parts.push('Ctrl');
  if (e.altKey) parts.push('Alt');
  if (e.shiftKey) parts.push('Shift');
  if (e.metaKey) parts.push('Meta');
  parts.push(keyOfEvent(e));
  return parts.join('+');
}

export function scopeStackFor(el: Element | null): ShortcutScope[] {
  const inner: ShortcutScope[] = [];
  for (let n: Element | null = el; n; n = n.parentElement) {
    const s = (n as HTMLElement).dataset?.shortcutScope as ShortcutScope | undefined;
    if (s) inner.unshift(s);
  }
  return ['global', 'view', ...inner];
}

export function resolveAction(
  actions: readonly ShortcutAction[],
  stack: readonly ShortcutScope[],
  e: KeyLike,
  hasHandler: (id: string) => boolean = () => true,
): ShortcutAction | undefined {
  for (let i = stack.length - 1; i >= 0; i--) {
    const hit = actions.find((a) => a.scope === stack[i] && hasHandler(a.id) && bindingMatches(a.binding, e));
    if (hit) return hit;
  }
  return undefined;
}

export function isTypingTarget(t: EventTarget | null): boolean {
  if (!(t instanceof HTMLElement)) return false;
  if (t.isContentEditable || t instanceof HTMLTextAreaElement || t instanceof HTMLSelectElement) return true;
  return t instanceof HTMLInputElement && !['checkbox', 'radio', 'button', 'submit', 'reset'].includes(t.type);
}
```

Create `frontend/src/shortcuts/store.ts`:

```ts
import { create } from 'zustand';
import type { ShortcutAction } from '../api/backend';
import { registerReset } from '../store/reset';

interface ShortcutStore { actions: ShortcutAction[]; setActions(a: ShortcutAction[]): void }

export const useShortcutStore = create<ShortcutStore>()((set) => ({ actions: [], setActions: (actions) => set({ actions }) }));

registerReset(() => useShortcutStore.setState({ actions: [] }));
```

Create `frontend/src/shortcuts/ShortcutProvider.tsx`:

```tsx
import { createContext, useContext, useEffect, useLayoutEffect, useMemo, useRef, type ReactNode } from 'react';
import type { Backend } from '../api/backend';
import { useBackend } from '../api/BackendContext';
import { isTypingTarget, resolveAction, scopeStackFor } from './registry';
import { useShortcutStore } from './store';

interface ShortcutContextValue { register(actionId: string, handler: () => void): () => void }

const ShortcutContext = createContext<ShortcutContextValue | null>(null);

/** The registry lives in Go (ARCHITECTURE §8.8); the UI only mirrors it. */
export async function reloadShortcuts(backend: Backend): Promise<void> {
  useShortcutStore.getState().setActions(await backend.settings.shortcuts());
}

export function ShortcutProvider({ children }: { children: ReactNode }) {
  const backend = useBackend();
  const handlers = useRef(new Map<string, () => void>());

  useEffect(() => {
    reloadShortcuts(backend).catch(() => useShortcutStore.getState().setActions([]));
  }, [backend]);

  useEffect(() => {
    const onKeyDown = (e: KeyboardEvent) => {
      if (e.defaultPrevented || e.repeat) return;
      const typing = isTypingTarget(e.target);
      if (typing && !e.ctrlKey && !e.altKey && !e.metaKey && !/^F\d{1,2}$/.test(e.key)) return;
      const action = resolveAction(useShortcutStore.getState().actions, scopeStackFor(document.activeElement), e, (id) => handlers.current.has(id));
      if (!action) return;
      e.preventDefault();
      handlers.current.get(action.id)?.();
    };
    window.addEventListener('keydown', onKeyDown);
    return () => window.removeEventListener('keydown', onKeyDown);
  }, []);

  const value = useMemo<ShortcutContextValue>(
    () => ({
      register: (actionId, handler) => {
        handlers.current.set(actionId, handler);
        return () => {
          if (handlers.current.get(actionId) === handler) handlers.current.delete(actionId);
        };
      },
    }),
    [],
  );

  return <ShortcutContext.Provider value={value}>{children}</ShortcutContext.Provider>;
}

export function useShortcut(actionId: string, handler: () => void): void {
  const ctx = useContext(ShortcutContext);
  if (!ctx) throw new Error('useShortcut must be used inside <ShortcutProvider>');
  const latest = useRef(handler);
  useLayoutEffect(() => {
    latest.current = handler;
  });
  useEffect(() => ctx.register(actionId, () => latest.current()), [ctx, actionId]);
}
```

Create `frontend/src/shortcuts/GlobalShortcuts.tsx`:

```tsx
import { useNavigate } from 'react-router-dom';
import { visibleNavEntries } from '../app/features';
import { focusNewestToast } from '../store/toasts';
import { useShortcut } from './ShortcutProvider';

function NavShortcut({ actionId, path }: { actionId: string; path: string }) {
  const navigate = useNavigate();
  useShortcut(actionId, () => void navigate(path));
  return null;
}

/** Hidden (later-phase) sidebar entries get no handler, so their Alt+N does nothing. */
export function GlobalShortcuts() {
  const navigate = useNavigate();
  useShortcut('toasts.focus', () => {
    focusNewestToast();
  });
  useShortcut('help.open', () => void navigate('/help'));
  return (
    <>
      {visibleNavEntries().map((e) => <NavShortcut key={e.id} actionId={e.shortcutId} path={e.path} />)}
    </>
  );
}
```

Create `frontend/src/app/lastRoute.ts`:

```ts
import { useEffect } from 'react';
import { useLocation } from 'react-router-dom';
import { useBackend } from '../api/BackendContext';
import { useSettingsStore, writeSetting } from '../store/settings';

// P0 shell views; later phases append their routes here when their views ship.
const REMEMBERABLE: RegExp[] = [
  /^\/home$/, /^\/orders$/, /^\/positions$/, /^\/holdings$/, /^\/profile$/,
  /^\/settings\/[a-z-]+$/, /^\/help(\/[a-z0-9-]+)?$/, /^\/chart-full$/,
];

export function isRememberable(path: string): boolean {
  return REMEMBERABLE.some((r) => r.test(path));
}

export function safeLastRoute(value: unknown): string {
  return typeof value === 'string' && isRememberable(value) ? value : '/home';
}

/** Persists the current view so the app reopens on it (FR-SHELL-019 window state). */
export function useRememberLastRoute(): void {
  const { pathname } = useLocation();
  const backend = useBackend();
  const loaded = useSettingsStore((s) => s.loaded);
  useEffect(() => {
    if (!loaded || !isRememberable(pathname)) return;
    if (useSettingsStore.getState().values['ui.last_route'] === pathname) return;
    void writeSetting(backend, 'ui.last_route', pathname);
  }, [pathname, loaded, backend]);
}
```

Overwrite `frontend/src/app/RootLayout.tsx`:

```tsx
import { useEffect, useLayoutEffect, useRef } from 'react';
import { Outlet, useNavigate } from 'react-router-dom';
import { useBackend } from '../api/BackendContext';
import { bootstrap, startEventBridge } from '../api/events';
import { ToastRegion } from '../components/ToastRegion';
import { ShortcutProvider } from '../shortcuts/ShortcutProvider';
import { useTrackHistoryDepth } from '../store/nav';

export function RootLayout() {
  const backend = useBackend();
  const navigate = useNavigate();
  const navRef = useRef(navigate);
  useLayoutEffect(() => {
    navRef.current = navigate;
  });
  useEffect(() => {
    const stop = startEventBridge(backend, { navigate: (route) => void navRef.current(route) });
    void bootstrap(backend);
    return stop;
  }, [backend]);
  useTrackHistoryDepth();
  return (
    <ShortcutProvider>
      <Outlet />
      <ToastRegion />
    </ShortcutProvider>
  );
}
```

Overwrite `frontend/src/app/ShellLayout.tsx`:

```tsx
import { Outlet } from 'react-router-dom';
import { GlobalShortcuts } from '../shortcuts/GlobalShortcuts';
import { useSetting } from '../store/settings';
import { AppShell } from './AppShell';
import { visibleNavEntries } from './features';
import { useRememberLastRoute } from './lastRoute';
import { Sidebar } from './Sidebar';
import { TopBar } from './TopBar';

export function ShellLayout() {
  const [collapsed, setCollapsed] = useSetting<boolean>('appearance.sidebar_collapsed', false);
  useRememberLastRoute();
  return (
    <AppShell
      topBar={<TopBar ticker={null} />}
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
Expected: PASS (25 test files), `tsc` exits 0.

- [ ] **Step 5: Commit**

```bash
git add frontend/src
git commit -m "feat(frontend): shortcut registry client, scoped global shortcuts, last-view memory (P0-4 Task 6)" \
  -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---
