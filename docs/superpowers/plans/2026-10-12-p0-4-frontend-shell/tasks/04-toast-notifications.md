> **P0-4 Frontend Shell, Account & Data UI Implementation Plan — Task 4.** Read [`../MANIFEST.md`](../MANIFEST.md) first: its Global Constraints,
> Review Focus and Consumes/Produces apply to this task. Other tasks live beside this file in `tasks/`.

<!-- task body (verbatim from the plan) -->
### Task 4: Toast notifications

**Files:**
- Create: `frontend/src/store/toasts.ts`, `frontend/src/components/ToastRegion.tsx`, `frontend/src/components/toast.css`
- Modify: `frontend/src/app/RootLayout.tsx`, `frontend/src/i18n/copy.ts`
- Test: `frontend/src/components/ToastRegion.test.tsx`

**Interfaces:**
- Consumes: Task 3 `IconButton`, Task 1 `Icon`.
- Produces: `type ToastLevel = 'info' | 'success' | 'warning' | 'error'`; `interface Toast { id: number; level: ToastLevel; message: string; action?: { label: string; onSelect(): void } }`; `TOAST_DURATION_MS: Record<ToastLevel, number | null>` (5000/5000/8000/null); `MAX_VISIBLE_TOASTS = 3`; `useToastStore` (`visible`, `queued`, `push`, `dismiss`); `pushToast(t: Omit<Toast, 'id'>): number`; `focusNewestToast(): boolean`; `ToastRegion()` (mounted once in `RootLayout`).

- [ ] **Step 1: Write the failing test**

Create `frontend/src/components/ToastRegion.test.tsx`:

```tsx
import { act, fireEvent, render, screen, within } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { focusNewestToast, pushToast } from '../store/toasts';
import { ToastRegion } from './ToastRegion';

beforeEach(() => vi.useFakeTimers());
afterEach(() => vi.useRealTimers());

const advance = (ms: number) => act(() => { vi.advanceTimersByTime(ms); });

describe('FR-SHELL-009 toasts', () => {
  it('AC1: SUCCESS disappears after 5 s ± 0.5 s', () => {
    render(<ToastRegion />);
    act(() => { pushToast({ level: 'success', message: 'Strategy saved' }); });
    advance(4500);
    expect(screen.getByText('Strategy saved')).toBeInTheDocument();
    advance(1000);
    expect(screen.queryByText('Strategy saved')).toBeNull();
  });
  it('WARNING stays for 8 s', () => {
    render(<ToastRegion />);
    act(() => { pushToast({ level: 'warning', message: 'Calendar may be stale' }); });
    advance(7500);
    expect(screen.getByText('Calendar may be stale')).toBeInTheDocument();
    advance(1000);
    expect(screen.queryByText('Calendar may be stale')).toBeNull();
  });
  it('AC2: ERROR is still visible after 60 s until × is clicked', () => {
    render(<ToastRegion />);
    act(() => { pushToast({ level: 'error', message: 'Holdings could not be loaded' }); });
    advance(60_000);
    const toast = screen.getByText('Holdings could not be loaded').closest('.toast') as HTMLElement;
    fireEvent.click(within(toast).getByRole('button', { name: 'Dismiss notification' }));
    expect(screen.queryByText('Holdings could not be loaded')).toBeNull();
  });
  it('hovering pauses the timer; leaving resumes the remaining time', () => {
    render(<ToastRegion />);
    act(() => { pushToast({ level: 'info', message: 'Found 5 instruments' }); });
    advance(4000);
    const toast = screen.getByText('Found 5 instruments').closest('.toast') as HTMLElement;
    fireEvent.mouseEnter(toast);
    advance(10_000);
    expect(screen.getByText('Found 5 instruments')).toBeInTheDocument();
    fireEvent.mouseLeave(toast);
    advance(900);
    expect(screen.getByText('Found 5 instruments')).toBeInTheDocument();
    advance(200);
    expect(screen.queryByText('Found 5 instruments')).toBeNull();
  });
  it('AC3: four toasts within 1 s → three visible (newest on top); the fourth appears when one is dismissed', () => {
    render(<ToastRegion />);
    act(() => {
      for (const m of ['one', 'two', 'three', 'four']) pushToast({ level: 'error', message: m });
    });
    const messages = () => screen.getAllByTestId('toast-message').map((n) => n.textContent);
    expect(messages()).toEqual(['three', 'two', 'one']);
    fireEvent.click(within(screen.getByText('two').closest('.toast') as HTMLElement).getByRole('button', { name: 'Dismiss notification' }));
    expect(messages()).toEqual(['four', 'three', 'one']);
  });
  it('is a labelled live region; focusNewestToast focuses the top toast', () => {
    render(<ToastRegion />);
    expect(screen.getByRole('region', { name: 'Notifications' })).toHaveAttribute('aria-live', 'polite');
    expect(focusNewestToast()).toBe(false);
    act(() => {
      pushToast({ level: 'error', message: 'older' });
      pushToast({ level: 'error', message: 'newer' });
    });
    act(() => { focusNewestToast(); });
    expect(screen.getByText('newer').closest('.toast')).toHaveFocus();
  });
  it('runs the action and dismisses the toast', () => {
    const onSelect = vi.fn();
    render(<ToastRegion />);
    act(() => { pushToast({ level: 'warning', message: 'The app crashed last time', action: { label: 'View report', onSelect } }); });
    fireEvent.click(screen.getByRole('button', { name: 'View report' }));
    expect(onSelect).toHaveBeenCalledTimes(1);
    expect(screen.queryByText('The app crashed last time')).toBeNull();
  });
});
```

- [ ] **Step 2: Run it to verify it fails**

Run: `cd frontend && npx vitest run src/components/ToastRegion.test.tsx`
Expected: FAIL — `Failed to resolve import "../store/toasts"`.

- [ ] **Step 3: Implement the toast store and region**

Append to `frontend/src/i18n/copy.ts`:

```ts
export const toastCopy = {
  region: 'Notifications',
  dismiss: 'Dismiss notification',
} as const;
```

Create `frontend/src/store/toasts.ts`:

```ts
import { create } from 'zustand';
import { registerReset } from './reset';

export type ToastLevel = 'info' | 'success' | 'warning' | 'error';
export interface ToastAction { label: string; onSelect: () => void }
export interface Toast { id: number; level: ToastLevel; message: string; action?: ToastAction }

export const TOAST_DURATION_MS: Record<ToastLevel, number | null> = { info: 5000, success: 5000, warning: 8000, error: null };
export const MAX_VISIBLE_TOASTS = 3;

interface ToastStore {
  visible: Toast[]; // newest first
  queued: Toast[]; // oldest first
  push(t: Omit<Toast, 'id'>): number;
  dismiss(id: number): void;
}

let nextId = 1;

export const useToastStore = create<ToastStore>()((set) => ({
  visible: [],
  queued: [],
  push: (t) => {
    const toast: Toast = { ...t, id: nextId++ };
    set((s) => (s.visible.length < MAX_VISIBLE_TOASTS ? { visible: [toast, ...s.visible] } : { queued: [...s.queued, toast] }));
    return toast.id;
  },
  dismiss: (id) =>
    set((s) => {
      const visible = s.visible.filter((x) => x.id !== id);
      const queued = s.queued.filter((x) => x.id !== id);
      if (visible.length < MAX_VISIBLE_TOASTS && queued.length > 0) {
        const [next, ...rest] = queued;
        return { visible: [next, ...visible], queued: rest };
      }
      return { visible, queued };
    }),
}));

registerReset(() => useToastStore.setState({ visible: [], queued: [] }));

export function pushToast(t: Omit<Toast, 'id'>): number {
  return useToastStore.getState().push(t);
}

/** Alt+T target (FR-SHELL-009 AC5). */
export function focusNewestToast(): boolean {
  const el = document.querySelector<HTMLElement>('[data-toast-region] .toast');
  if (!el) return false;
  el.focus();
  return true;
}
```

Create `frontend/src/components/ToastRegion.tsx`:

```tsx
import { useCallback, useEffect, useRef } from 'react';
import { toastCopy } from '../i18n/copy';
import { TOAST_DURATION_MS, useToastStore, type Toast, type ToastLevel } from '../store/toasts';
import { Icon, type IconName } from './Icon';
import { IconButton } from './IconButton';
import './toast.css';

const ICON: Record<ToastLevel, IconName> = { info: 'info', success: 'check', warning: 'warning', error: 'error' };

export function ToastRegion() {
  const visible = useToastStore((s) => s.visible);
  return (
    <section className="toast-region" aria-label={toastCopy.region} aria-live="polite" data-toast-region>
      {visible.map((t) => <ToastItem key={t.id} toast={t} />)}
    </section>
  );
}

function ToastItem({ toast }: { toast: Toast }) {
  const dismiss = useToastStore((s) => s.dismiss);
  const duration = TOAST_DURATION_MS[toast.level];
  const remaining = useRef(duration ?? 0);
  const startedAt = useRef(0);
  const timer = useRef<number | undefined>(undefined);
  const hovered = useRef(false);
  const focused = useRef(false);

  const run = useCallback(() => {
    if (duration === null || timer.current !== undefined) return;
    startedAt.current = Date.now();
    timer.current = window.setTimeout(() => dismiss(toast.id), remaining.current);
  }, [dismiss, duration, toast.id]);

  const pause = useCallback(() => {
    if (timer.current === undefined) return;
    window.clearTimeout(timer.current);
    timer.current = undefined;
    remaining.current = Math.max(0, remaining.current - (Date.now() - startedAt.current));
  }, []);

  useEffect(() => {
    run();
    return pause;
  }, [run, pause]);

  const resumeIfIdle = () => {
    if (!hovered.current && !focused.current) run();
  };

  return (
    <div
      className={`toast toast--${toast.level}`}
      role={toast.level === 'error' ? 'alert' : 'status'}
      tabIndex={-1}
      onMouseEnter={() => { hovered.current = true; pause(); }}
      onMouseLeave={() => { hovered.current = false; resumeIfIdle(); }}
      onFocus={() => { focused.current = true; pause(); }}
      onBlur={(e) => {
        if (e.currentTarget.contains(e.relatedTarget as Node | null)) return;
        focused.current = false;
        resumeIfIdle();
      }}
    >
      <Icon name={ICON[toast.level]} className={`toast__icon toast__icon--${toast.level}`} />
      <span className="toast__message" data-testid="toast-message">{toast.message}</span>
      {toast.action && (
        <button type="button" className="toast__action" onClick={() => { toast.action?.onSelect(); dismiss(toast.id); }}>
          {toast.action.label}
        </button>
      )}
      <IconButton label={toastCopy.dismiss} icon="close" size={16} onClick={() => dismiss(toast.id)} />
    </div>
  );
}
```

Create `frontend/src/components/toast.css`:

```css
.toast-region {
  position: fixed; z-index: var(--z-toast); top: var(--space-3); left: 50%; transform: translateX(-50%);
  display: flex; flex-direction: column; gap: var(--space-2); width: 440px; pointer-events: none;
}
.toast {
  pointer-events: auto; display: flex; align-items: flex-start; gap: var(--space-2); padding: var(--space-3);
  background: var(--bg); color: var(--text); border: 1px solid var(--border); border-left: 4px solid var(--accent);
  border-radius: var(--radius-2); box-shadow: var(--shadow);
}
.toast--success { border-left-color: var(--up); }
.toast--warning { border-left-color: var(--warn); }
.toast--error { border-left-color: var(--down); }
.toast__icon--info { color: var(--accent); }
.toast__icon--success { color: var(--up); }
.toast__icon--warning { color: var(--warn); }
.toast__icon--error { color: var(--down); }
.toast__message { flex: 1 1 auto; padding-top: 2px; }
.toast__action { border: none; background: none; padding: 2px 0 0; color: var(--accent); text-decoration: underline; cursor: pointer; }
```

Overwrite `frontend/src/app/RootLayout.tsx`:

```tsx
import { useEffect, useLayoutEffect, useRef } from 'react';
import { Outlet, useNavigate } from 'react-router-dom';
import { useBackend } from '../api/BackendContext';
import { bootstrap, startEventBridge } from '../api/events';
import { ToastRegion } from '../components/ToastRegion';

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
  return (
    <>
      <Outlet />
      <ToastRegion />
    </>
  );
}
```

- [ ] **Step 4: Run the suite to verify it passes**

Run: `cd frontend && npx vitest run`
Expected: PASS (19 test files).

- [ ] **Step 5: Commit**

```bash
git add frontend/src
git commit -m "feat(frontend): toast notifications with queue, timers and live region (P0-4 Task 4)" \
  -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---
