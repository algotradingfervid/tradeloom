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

export const MAX_QUEUED_TOASTS = 10;

let nextId = 1;
/** When each queued toast was pushed; a queued info/success toast past its duration is dropped, not shown. */
const queuedAt = new Map<number, number>();

const isStale = (t: Toast, now: number): boolean => {
  const d = TOAST_DURATION_MS[t.level];
  return (t.level === 'info' || t.level === 'success') && !t.action && d !== null && now - (queuedAt.get(t.id) ?? now) >= d;
};

function capQueue(queued: Toast[]): Toast[] {
  if (queued.length <= MAX_QUEUED_TOASTS) return queued;
  const drop = queued.findIndex((t) => t.level !== 'error');
  const at = drop === -1 ? 0 : drop;
  queuedAt.delete(queued[at].id);
  return [...queued.slice(0, at), ...queued.slice(at + 1)];
}

export const useToastStore = create<ToastStore>()((set) => ({
  visible: [],
  queued: [],
  push: (t) => {
    const toast: Toast = { ...t, id: nextId++ };
    set((s) => {
      if (s.visible.length < MAX_VISIBLE_TOASTS) return { visible: [toast, ...s.visible] };
      queuedAt.set(toast.id, Date.now());
      return { queued: capQueue([...s.queued, toast]) };
    });
    return toast.id;
  },
  dismiss: (id) =>
    set((s) => {
      const now = Date.now();
      const visible = s.visible.filter((x) => x.id !== id);
      const queued = s.queued.filter((x) => {
        const keep = x.id !== id && !isStale(x, now);
        if (!keep) queuedAt.delete(x.id);
        return keep;
      });
      if (visible.length < MAX_VISIBLE_TOASTS && queued.length > 0) {
        const [next, ...rest] = queued;
        queuedAt.delete(next.id);
        return { visible: [next, ...visible], queued: rest };
      }
      return { visible, queued };
    }),
}));

/** Where focus was when Alt+T moved it into the toasts; it goes back there when the last focused toast closes. */
let returnFocus: HTMLElement | null = null;

// Ruling 85: exempt from the logout reset; only resetAllStores (tests) clears it.
// TODO(Ruling 85): pass `{ onLogout: false }` once Task 2's registerReset takes options.
registerReset(() => {
  useToastStore.setState({ visible: [], queued: [] });
  queuedAt.clear();
  returnFocus = null;
});

export function pushToast(t: Omit<Toast, 'id'>): number {
  return useToastStore.getState().push(t);
}

/** Alt+T target (FR-SHELL-009 AC5). */
export function focusNewestToast(): boolean {
  const el = document.querySelector<HTMLElement>('[data-toast-region] .toast');
  if (!el) return false;
  const active = document.activeElement;
  if (active instanceof HTMLElement && active !== document.body && !active.closest('[data-toast-region]')) returnFocus = active;
  el.focus();
  return true;
}

/** Focus left the toasts for somewhere else: the Alt+T origin no longer applies. */
export function forgetToastFocusOrigin(): void {
  returnFocus = null;
}

/** Fallback when there is no Alt+T origin: the view heading, else the main region. */
function fallbackFocusTarget(): HTMLElement | null {
  const el = document.querySelector<HTMLElement>('#main-content h1') ?? document.getElementById('main-content');
  if (el && !el.hasAttribute('tabindex')) el.setAttribute('tabindex', '-1');
  return el;
}

/** Called when a focused toast closes and no other toast is left to take focus. */
export function restoreFocusFromToasts(): void {
  const target = returnFocus?.isConnected ? returnFocus : fallbackFocusTarget();
  returnFocus = null;
  target?.focus();
}
