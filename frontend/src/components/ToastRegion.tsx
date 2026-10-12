import { useCallback, useEffect, useRef } from 'react';
import { toastCopy } from '../i18n/copy';
import { restoreFocusFromToasts, TOAST_DURATION_MS, useToastStore, type Toast, type ToastLevel } from '../store/toasts';
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
  const self = useRef<HTMLDivElement>(null);

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

  // A closing toast that holds focus hands it to the next toast, or back to where Alt+T took it from.
  const close = () => {
    const el = self.current;
    if (el?.contains(document.activeElement)) {
      const next = Array.from(el.parentElement?.querySelectorAll<HTMLElement>('.toast') ?? []).find((t) => t !== el);
      if (next) next.focus();
      else restoreFocusFromToasts();
    }
    dismiss(toast.id);
  };

  return (
    <div
      ref={self}
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
        <button type="button" className="toast__action" onClick={() => { toast.action?.onSelect(); close(); }}>
          {toast.action.label}
        </button>
      )}
      <IconButton label={toastCopy.dismiss} icon="close" size={16} onClick={close} />
    </div>
  );
}
