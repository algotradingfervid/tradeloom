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

/** Where focus was when Alt+T moved it into the toasts; it goes back there when the last focused toast closes. */
let returnFocus: HTMLElement | null = null;

registerReset(() => {
  useToastStore.setState({ visible: [], queued: [] });
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

/** Called when a focused toast closes and no other toast is left to take focus. */
export function restoreFocusFromToasts(): void {
  const target = returnFocus;
  returnFocus = null;
  if (target?.isConnected) target.focus();
}
