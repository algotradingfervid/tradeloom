import { Children, cloneElement, useEffect, useId, useRef, useState, type FocusEvent, type KeyboardEvent, type MouseEvent, type ReactElement } from 'react';

export const TOOLTIP_DELAY_MS = 500;

interface TriggerProps {
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
  const timer = useRef<number | undefined>(undefined);
  const clear = () => {
    if (timer.current !== undefined) window.clearTimeout(timer.current);
    timer.current = undefined;
  };
  useEffect(() => clear, []);
  const child = Children.only(children);
  const p = child.props;
  const trigger = cloneElement(child, {
    'aria-describedby': open ? id : p['aria-describedby'],
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
    <span className={`tooltip-anchor tooltip-anchor--${placement}`}>
      {trigger}
      {open && <span role="tooltip" id={id} className="tooltip">{label}</span>}
    </span>
  );
}
