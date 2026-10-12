import { useEffect, useId, useLayoutEffect, useRef, type KeyboardEvent, type ReactNode, type RefObject } from 'react';
import { createPortal } from 'react-dom';
import { componentCopy } from '../i18n/copy';
import { firstFocusable, focusables } from './focus';
import { IconButton } from './IconButton';
import './components.css';

export interface DialogProps {
  open: boolean;
  title: string;
  description?: string;
  onClose: () => void;
  children?: ReactNode;
  footer?: ReactNode;
  size?: 'sm' | 'md' | 'lg' | 'side';
  initialFocus?: RefObject<HTMLElement | null>;
  closeLabel?: string;
  /** id of an element inside the dialog that describes it (used when `description` is not given). */
  describedById?: string;
  /** false while an irreversible operation runs: Escape, overlay click and × do nothing. */
  dismissible?: boolean;
}

export function Dialog(props: DialogProps) {
  if (!props.open) return null;
  return createPortal(<DialogPanel {...props} />, document.body);
}

function DialogPanel({ title, description, onClose, children, footer, size = 'md', initialFocus, closeLabel, describedById, dismissible = true }: DialogProps) {
  const panel = useRef<HTMLDivElement>(null);
  const titleId = useId();
  const descId = useId();

  useLayoutEffect(() => {
    const previous = document.activeElement instanceof HTMLElement ? document.activeElement : null;
    const root = panel.current;
    const target =
      initialFocus?.current ??
      firstFocusable(root?.querySelector('.dialog__body') ?? null) ??
      firstFocusable(root?.querySelector('.dialog__footer') ?? null) ??
      root;
    target?.focus();
    return () => {
      if (previous && document.contains(previous)) previous.focus();
    };
    // focus is captured once per opening
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // If live updates remove the focused control (e.g. an event storm re-renders the body), keep focus in the dialog.
  useEffect(() => {
    const root = panel.current;
    if (!root) return;
    const observer = new MutationObserver(() => {
      const a = document.activeElement;
      if (a === null || a === document.body || (a instanceof HTMLButtonElement && a.disabled)) root.focus();
    });
    observer.observe(root, { childList: true, subtree: true, attributes: true, attributeFilter: ['disabled'] });
    return () => observer.disconnect();
  }, []);

  const onKeyDown = (e: KeyboardEvent<HTMLDivElement>) => {
    if (e.key === 'Escape') {
      e.preventDefault();
      e.stopPropagation();
      if (dismissible) onClose();
      return;
    }
    if (e.key !== 'Tab') return;
    const items = focusables(panel.current);
    if (items.length === 0) {
      e.preventDefault();
      return;
    }
    const first = items[0];
    const last = items[items.length - 1];
    const active = document.activeElement;
    if (e.shiftKey && (active === first || active === panel.current)) {
      e.preventDefault();
      last.focus();
    } else if (!e.shiftKey && (active === last || active === panel.current)) {
      e.preventDefault();
      first.focus();
    }
  };

  return (
    <div className="dialog-overlay" onMouseDown={(e) => { if (e.target === e.currentTarget && dismissible) onClose(); }}>
      <div
        ref={panel}
        role="dialog"
        aria-modal="true"
        aria-labelledby={titleId}
        aria-describedby={description ? descId : describedById}
        className={`dialog dialog--${size}`}
        tabIndex={-1}
        onKeyDown={onKeyDown}
      >
        <div className="dialog__header">
          <h2 id={titleId} className="dialog__title">{title}</h2>
          <IconButton label={closeLabel ?? componentCopy.close} icon="close" onClick={onClose} disabled={!dismissible} />
        </div>
        {description && <p id={descId} className="dialog__description">{description}</p>}
        <div className="dialog__body">{children}</div>
        {footer && <div className="dialog__footer">{footer}</div>}
      </div>
    </div>
  );
}
