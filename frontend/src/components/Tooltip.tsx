import { Children, cloneElement, useEffect, useId, useLayoutEffect, useRef, useState, type FocusEvent, type KeyboardEvent, type MouseEvent, type ReactElement } from 'react';
import { createPortal } from 'react-dom';
import './components.css';

export const TOOLTIP_DELAY_MS = 500;

interface TriggerProps {
  'aria-expanded'?: boolean | 'true' | 'false';
  'aria-haspopup'?: unknown;
  'aria-describedby'?: string;
  onMouseEnter?: (e: MouseEvent<HTMLElement>) => void;
  onMouseLeave?: (e: MouseEvent<HTMLElement>) => void;
  onFocus?: (e: FocusEvent<HTMLElement>) => void;
  onBlur?: (e: FocusEvent<HTMLElement>) => void;
  onKeyDown?: (e: KeyboardEvent<HTMLElement>) => void;
}

export interface TooltipProps { label: string; children: ReactElement<TriggerProps>; placement?: 'bottom' | 'top' | 'right' }

/** Shows after 500 ms hover or immediately on focus; hides on leave, blur or Escape (FR-SHELL-013). */
export function Tooltip({ label, children, placement = 'bottom' }: TooltipProps) {
  const id = useId();
  const [open, setOpen] = useState(false);
  const anchor = useRef<HTMLSpanElement>(null);
  const [pos, setPos] = useState<{ left: number; top: number } | null>(null);
  const timer = useRef<number | undefined>(undefined);
  const clear = () => {
    if (timer.current !== undefined) window.clearTimeout(timer.current);
    timer.current = undefined;
  };
  useEffect(() => clear, []);
  const child = Children.only(children);
  const p = child.props;
  // A trigger whose popup is open (menu button) must not also show a tooltip over it.
  const suppressed = Boolean(p['aria-haspopup']) && (p['aria-expanded'] === true || p['aria-expanded'] === 'true');
  const visible = open && !suppressed;

  useLayoutEffect(() => {
    if (!visible || !anchor.current) {
      setPos(null);
      return;
    }
    const r = anchor.current.getBoundingClientRect();
    if (placement === 'right') setPos({ left: r.right + 6, top: r.top + r.height / 2 });
    else if (placement === 'top') setPos({ left: r.left + r.width / 2, top: r.top - 6 });
    else setPos({ left: r.left + r.width / 2, top: r.bottom + 6 });
  }, [visible, placement]);
  const trigger = cloneElement(child, {
    'aria-describedby': visible ? id : p['aria-describedby'],
    onMouseEnter: (e: MouseEvent<HTMLElement>) => {
      p.onMouseEnter?.(e);
      clear();
      timer.current = window.setTimeout(() => setOpen(true), TOOLTIP_DELAY_MS);
    },
    onMouseLeave: (e: MouseEvent<HTMLElement>) => {
      p.onMouseLeave?.(e);
      clear();
      setOpen(false);
    },
    onFocus: (e: FocusEvent<HTMLElement>) => {
      p.onFocus?.(e);
      clear();
      setOpen(true);
    },
    onBlur: (e: FocusEvent<HTMLElement>) => {
      p.onBlur?.(e);
      clear();
      setOpen(false);
    },
    onKeyDown: (e: KeyboardEvent<HTMLElement>) => {
      if (e.key === 'Escape') setOpen(false); // not stopped: an enclosing dialog still closes
      p.onKeyDown?.(e);
    },
  });
  return (
    <span ref={anchor} className="tooltip-anchor">
      {trigger}
      {visible &&
        createPortal(
          <span
            role="tooltip"
            id={id}
            className={`tooltip tooltip--${placement}`}
            style={pos ? { left: pos.left, top: pos.top } : { visibility: 'hidden' }}
          >
            {label}
          </span>,
          document.body,
        )}
    </span>
  );
}
