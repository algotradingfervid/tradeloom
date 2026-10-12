import { useCallback, useEffect, useLayoutEffect, useRef, useState, type KeyboardEvent, type ReactNode } from 'react';
import { createPortal } from 'react-dom';
import { listCopy } from '../i18n/copy';
import { Icon, type IconName } from './Icon';
import { IconButton } from './IconButton';
import './list.css';

export interface Column<T> {
  id: string;
  header: string;
  cell: (row: T) => ReactNode;
  align?: 'start' | 'end';
  width?: string;
  /** When set, the cell truncates with an ellipsis and this full text goes into `title`. */
  title?: (row: T) => string;
}

export interface RowAction<T> { id: string; label: string; icon: IconName; onSelect: (row: T) => void }

export interface DataTableProps<T> {
  caption: string;
  columns: Column<T>[];
  rows: T[];
  rowKey: (row: T) => string;
  rowLabel: (row: T) => string;
  rowActions?: RowAction<T>[];
  rowClassName?: (row: T) => string | undefined;
  footer?: ReactNode;
}

/** Icon buttons are 32 px wide with a 4 px gap; the cell has 8 px padding on each side (Ruling 84 M1). */
function actionsColumnWidth(n: number): string {
  return `${n * 32 + Math.max(0, n - 1) * 4 + 16}px`;
}

/**
 * The active row and the open row menu are tracked by `rowKey`, never by index, so a live update that inserts or
 * removes rows cannot move them onto another instrument (Ruling 84 I1). `index` is only the fallback position used
 * when the active row itself disappears.
 */
interface ActiveRow { key: string | null; index: number }

export function DataTable<T>({ caption, columns, rows, rowKey, rowLabel, rowActions, rowClassName, footer }: DataTableProps<T>) {
  const [active, setActive] = useState<ActiveRow>({ key: null, index: 0 });
  const [menuKey, setMenuKey] = useState<string | null>(null);
  const menuKeyRef = useRef<string | null>(null);
  const rowRefs = useRef(new Map<string, HTMLTableRowElement>());
  const keys = rows.map(rowKey);
  const found = active.key === null ? -1 : keys.indexOf(active.key);
  const activeIndex = found >= 0 ? found : Math.min(active.index, Math.max(0, rows.length - 1));
  const actionCount = rowActions?.length ?? 0;
  const hasActions = actionCount > 0;
  const menuOpen = menuKey !== null && keys.includes(menuKey);
  const activeRowKey = keys[activeIndex];

  // The menu's row disappeared (live update): close the menu and, if focus was inside it, put focus on a row.
  useLayoutEffect(() => {
    if (menuKey === null || menuOpen) return;
    menuKeyRef.current = null;
    setMenuKey(null);
    const focused = document.activeElement;
    if ((!focused || focused === document.body) && activeRowKey !== undefined) rowRefs.current.get(activeRowKey)?.focus();
  }, [menuKey, menuOpen, activeRowKey]);

  const focusRow = (i: number) => {
    const j = Math.max(0, Math.min(rows.length - 1, i));
    const key = keys[j];
    if (key === undefined) return;
    setActive({ key, index: j });
    rowRefs.current.get(key)?.focus();
  };
  const openMenu = (i: number) => {
    const key = keys[i];
    setActive({ key, index: i });
    menuKeyRef.current = key;
    setMenuKey(key);
  };
  const closeMenu = useCallback((refocus: boolean) => {
    const key = menuKeyRef.current;
    menuKeyRef.current = null;
    setMenuKey(null);
    if (refocus && key !== null) rowRefs.current.get(key)?.focus();
  }, []);

  const onRowKeyDown = (e: KeyboardEvent<HTMLTableRowElement>, i: number) => {
    if (e.target !== e.currentTarget) return;
    const menuKeyPressed = e.key === 'ContextMenu' || (e.key === 'F10' && e.shiftKey);
    if (menuKeyPressed && hasActions) {
      e.preventDefault();
      openMenu(i);
      return;
    }
    const moves: Record<string, number> = { ArrowDown: i + 1, ArrowUp: i - 1, Home: 0, End: rows.length - 1 };
    const target = moves[e.key];
    if (target === undefined) return;
    e.preventDefault();
    focusRow(target);
  };

  return (
    <table className="data-table">
      <caption className="visually-hidden">{caption}</caption>
      <colgroup>
        {columns.map((c) => <col key={c.id} style={c.width ? { width: c.width } : undefined} />)}
        {hasActions && <col style={{ width: actionsColumnWidth(actionCount) }} />}
      </colgroup>
      <thead>
        <tr>
          {columns.map((c) => <th key={c.id} scope="col" className={c.align === 'end' ? 'num' : undefined}>{c.header}</th>)}
          {hasActions && <th scope="col"><span className="visually-hidden">{listCopy.actions}</span></th>}
        </tr>
      </thead>
      <tbody>
        {rows.map((row, i) => {
          const key = keys[i];
          return (
            <tr
              key={key}
              ref={(el) => {
                if (el) rowRefs.current.set(key, el);
                else rowRefs.current.delete(key);
              }}
              tabIndex={i === activeIndex ? 0 : -1}
              aria-label={rowLabel(row)}
              className={rowClassName?.(row)}
              onFocus={(e) => { if (e.target === e.currentTarget) setActive({ key, index: i }); }}
              onKeyDown={(e) => onRowKeyDown(e, i)}
              onContextMenu={hasActions ? (e) => { e.preventDefault(); openMenu(i); } : undefined}
            >
              {columns.map((c) => (
                <td key={c.id} className={c.align === 'end' ? 'num' : undefined}>
                  {c.title ? <span className="cell-truncate" title={c.title(row)}>{c.cell(row)}</span> : c.cell(row)}
                </td>
              ))}
              {hasActions && rowActions && (
                <td className="row-actions-cell">
                  <div className="row-actions">
                    {rowActions.map((a) => (
                      <IconButton key={a.id} label={listCopy.rowAction(a.label, rowLabel(row))} tooltip={a.label} icon={a.icon} size={16} onClick={() => a.onSelect(row)} />
                    ))}
                  </div>
                  {menuOpen && menuKey === key && <RowMenu row={row} label={listCopy.rowActions(rowLabel(row))} actions={rowActions} onClose={closeMenu} />}
                </td>
              )}
            </tr>
          );
        })}
      </tbody>
      {footer && <tfoot>{footer}</tfoot>}
    </table>
  );
}

interface RowMenuProps<T> { row: T; label: string; actions: RowAction<T>[]; onClose: (refocus: boolean) => void }

/**
 * Rendered in a portal (Task 3 tooltip pattern) so no scroll container clips it; it is placed under the row's
 * actions cell, or above it when there is no room below (Ruling 84 M2).
 */
function RowMenu<T>({ row, label, actions, onClose }: RowMenuProps<T>) {
  const anchor = useRef<HTMLSpanElement>(null);
  const ref = useRef<HTMLUListElement>(null);

  useLayoutEffect(() => {
    const place = () => {
      const menu = ref.current;
      const cell = anchor.current?.closest('td');
      if (!menu || !cell) return;
      const r = cell.getBoundingClientRect();
      const h = menu.getBoundingClientRect().height;
      const up = r.bottom + h > window.innerHeight && r.top - h >= 0;
      menu.dataset.placement = up ? 'up' : 'down';
      menu.style.left = `${r.right}px`;
      menu.style.top = `${up ? r.top : r.bottom}px`;
    };
    place();
    window.addEventListener('resize', place);
    window.addEventListener('scroll', place, true);
    return () => {
      window.removeEventListener('resize', place);
      window.removeEventListener('scroll', place, true);
    };
  }, []);

  useEffect(() => {
    ref.current?.querySelector<HTMLElement>('[role="menuitem"]')?.focus();
    const onDown = (e: MouseEvent) => {
      if (!ref.current?.contains(e.target as Node)) onClose(false);
    };
    document.addEventListener('mousedown', onDown);
    return () => document.removeEventListener('mousedown', onDown);
  }, [onClose]);

  const onKeyDown = (e: KeyboardEvent<HTMLUListElement>) => {
    const items = [...(ref.current?.querySelectorAll<HTMLElement>('[role="menuitem"]') ?? [])];
    const i = items.indexOf(document.activeElement as HTMLElement);
    if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
      e.preventDefault();
      items[(i + (e.key === 'ArrowDown' ? 1 : -1) + items.length) % items.length]?.focus();
    } else if (e.key === 'Escape') {
      e.preventDefault();
      e.stopPropagation();
      onClose(true);
    } else if (e.key === 'Tab') {
      // The menu lives at the end of <body>: hand focus back to its row rather than letting Tab leave the page.
      e.preventDefault();
      onClose(true);
    }
  };
  return (
    <>
      <span ref={anchor} className="row-menu-anchor" />
      {createPortal(
        <ul ref={ref} role="menu" aria-label={label} className="menu row-menu" data-placement="down" onKeyDown={onKeyDown}>
          {actions.map((a) => (
            <li key={a.id} role="none">
              <button type="button" role="menuitem" tabIndex={-1} className="menu__item" onClick={() => { onClose(true); a.onSelect(row); }}>
                <Icon name={a.icon} size={16} />
                <span>{a.label}</span>
              </button>
            </li>
          ))}
        </ul>,
        document.body,
      )}
    </>
  );
}
