import { useCallback, useEffect, useRef, useState, type KeyboardEvent, type ReactNode } from 'react';
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

export function DataTable<T>({ caption, columns, rows, rowKey, rowLabel, rowActions, rowClassName, footer }: DataTableProps<T>) {
  const [active, setActive] = useState(0);
  const [menuRow, setMenuRow] = useState<number | null>(null);
  const menuRowRef = useRef<number | null>(null);
  const rowRefs = useRef<(HTMLTableRowElement | null)[]>([]);
  const activeIndex = Math.min(active, Math.max(0, rows.length - 1));
  const hasActions = (rowActions?.length ?? 0) > 0;

  const focusRow = (i: number) => {
    const j = Math.max(0, Math.min(rows.length - 1, i));
    setActive(j);
    rowRefs.current[j]?.focus();
  };
  const openMenu = (i: number) => {
    setActive(i);
    menuRowRef.current = i;
    setMenuRow(i);
  };
  const closeMenu = useCallback((refocus: boolean) => {
    const i = menuRowRef.current;
    menuRowRef.current = null;
    setMenuRow(null);
    if (refocus && i !== null) rowRefs.current[i]?.focus();
  }, []);

  const onRowKeyDown = (e: KeyboardEvent<HTMLTableRowElement>, i: number) => {
    if (e.target !== e.currentTarget) return;
    const menuKey = e.key === 'ContextMenu' || (e.key === 'F10' && e.shiftKey);
    if (menuKey && hasActions) {
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
        {hasActions && <col style={{ width: `${(rowActions?.length ?? 0) * 36 + 8}px` }} />}
      </colgroup>
      <thead>
        <tr>
          {columns.map((c) => <th key={c.id} scope="col" className={c.align === 'end' ? 'num' : undefined}>{c.header}</th>)}
          {hasActions && <th scope="col"><span className="visually-hidden">{listCopy.actions}</span></th>}
        </tr>
      </thead>
      <tbody>
        {rows.map((row, i) => (
          <tr
            key={rowKey(row)}
            ref={(el) => { rowRefs.current[i] = el; }}
            tabIndex={i === activeIndex ? 0 : -1}
            aria-label={rowLabel(row)}
            className={rowClassName?.(row)}
            onFocus={(e) => { if (e.target === e.currentTarget) setActive(i); }}
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
                {menuRow === i && <RowMenu row={row} label={listCopy.rowActions(rowLabel(row))} actions={rowActions} onClose={closeMenu} />}
              </td>
            )}
          </tr>
        ))}
      </tbody>
      {footer && <tfoot>{footer}</tfoot>}
    </table>
  );
}

interface RowMenuProps<T> { row: T; label: string; actions: RowAction<T>[]; onClose: (refocus: boolean) => void }

function RowMenu<T>({ row, label, actions, onClose }: RowMenuProps<T>) {
  const ref = useRef<HTMLUListElement>(null);
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
      onClose(false);
    }
  };
  return (
    <ul ref={ref} role="menu" aria-label={label} className="menu row-menu" onKeyDown={onKeyDown}>
      {actions.map((a) => (
        <li key={a.id} role="none">
          <button type="button" role="menuitem" tabIndex={-1} className="menu__item" onClick={() => { onClose(true); a.onSelect(row); }}>
            <Icon name={a.icon} size={16} />
            <span>{a.label}</span>
          </button>
        </li>
      ))}
    </ul>
  );
}
