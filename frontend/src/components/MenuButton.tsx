import { useEffect, useId, useRef, useState, type KeyboardEvent, type ReactNode } from 'react';
import { Icon, type IconName } from './Icon';
import { Tooltip } from './Tooltip';
import './components.css';

export type MenuItem =
  | { kind: 'item'; id: string; label: string; icon?: IconName; onSelect: () => void }
  | { kind: 'checkbox'; id: string; label: string; checked: boolean; onChange: (v: boolean) => void }
  | { kind: 'separator'; id: string }
  | { kind: 'static'; id: string; content: ReactNode };

export interface MenuButtonProps { label: string; trigger: ReactNode; items: MenuItem[]; align?: 'start' | 'end' }

const ACTIONABLE = '[role="menuitem"],[role="menuitemcheckbox"]';

export function MenuButton({ label, trigger, items, align = 'end' }: MenuButtonProps) {
  const [open, setOpen] = useState(false);
  const button = useRef<HTMLButtonElement>(null);
  const menu = useRef<HTMLUListElement>(null);
  const menuId = useId();
  const actionable = () => [...(menu.current?.querySelectorAll<HTMLElement>(ACTIONABLE) ?? [])];

  useEffect(() => {
    if (!open) return;
    actionable()[0]?.focus();
    const onDown = (e: MouseEvent) => {
      const t = e.target as Node;
      if (!menu.current?.contains(t) && !button.current?.contains(t)) setOpen(false);
    };
    document.addEventListener('mousedown', onDown);
    return () => document.removeEventListener('mousedown', onDown);
  }, [open]);

  const close = (focusTrigger: boolean) => {
    setOpen(false);
    if (focusTrigger) button.current?.focus();
  };

  const onMenuKeyDown = (e: KeyboardEvent<HTMLUListElement>) => {
    const list = actionable();
    const i = list.indexOf(document.activeElement as HTMLElement);
    const move = (j: number) => {
      e.preventDefault();
      list[(j + list.length) % list.length]?.focus();
    };
    switch (e.key) {
      case 'ArrowDown': move(i + 1); break;
      case 'ArrowUp': move(i - 1); break;
      case 'Home': move(0); break;
      case 'End': move(list.length - 1); break;
      case 'Escape':
        e.preventDefault();
        e.stopPropagation();
        close(true);
        break;
      case 'Tab': close(false); break;
      default: break;
    }
  };

  return (
    <div className="menu-anchor">
      <Tooltip label={label}>
        <button
          ref={button}
          type="button"
          className="menu-trigger"
          aria-label={label}
          aria-haspopup="menu"
          aria-expanded={open}
          aria-controls={open ? menuId : undefined}
          onClick={() => setOpen((o) => !o)}
          onKeyDown={(e) => {
            if (e.key === 'ArrowDown') {
              e.preventDefault();
              setOpen(true);
            }
          }}
        >
          {trigger}
        </button>
      </Tooltip>
      {open && (
        <ul id={menuId} ref={menu} role="menu" aria-label={label} className={`menu menu--${align}`} onKeyDown={onMenuKeyDown}>
          {items.map((it) => {
            switch (it.kind) {
              case 'separator':
                return <li key={it.id} role="separator" className="menu__separator" />;
              case 'static':
                return <li key={it.id} role="none" className="menu__static">{it.content}</li>;
              case 'item':
                return (
                  <li key={it.id} role="none">
                    <button type="button" role="menuitem" tabIndex={-1} className="menu__item" onClick={() => { close(true); it.onSelect(); }}>
                      {it.icon && <Icon name={it.icon} size={16} />}
                      <span>{it.label}</span>
                    </button>
                  </li>
                );
              case 'checkbox':
                return (
                  <li key={it.id} role="none">
                    <button type="button" role="menuitemcheckbox" aria-checked={it.checked} tabIndex={-1} className="menu__item" onClick={() => { it.onChange(!it.checked); close(true); }}>
                      <span>{it.label}</span>
                      <span className="switch" data-on={it.checked} aria-hidden="true"><span className="switch__thumb" /></span>
                    </button>
                  </li>
                );
            }
          })}
        </ul>
      )}
    </div>
  );
}
