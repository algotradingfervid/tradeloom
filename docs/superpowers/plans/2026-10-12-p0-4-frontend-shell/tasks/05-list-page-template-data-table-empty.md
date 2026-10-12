> **P0-4 Frontend Shell, Account & Data UI Implementation Plan — Task 5.** Read [`../MANIFEST.md`](../MANIFEST.md) first: its Global Constraints,
> Review Focus and Consumes/Produces apply to this task. Other tasks live beside this file in `tasks/`.

<!-- task body (verbatim from the plan) -->
### Task 5: List-page template, data table, empty/loading/error states, unknown-view fallback

**Files:**
- Create: `frontend/src/components/ListPage.tsx`, `frontend/src/components/ListBody.tsx`, `frontend/src/components/DataTable.tsx`, `frontend/src/components/filterRows.ts`, `frontend/src/components/list.css`
- Create: `frontend/src/store/nav.ts`, `frontend/src/views/NotFoundView.tsx`
- Modify: `frontend/src/app/routes.tsx` (`*` fallback inside the shell), `frontend/src/app/RootLayout.tsx` (history depth), `frontend/src/i18n/copy.ts`
- Test: `frontend/src/components/ListPage.test.tsx`, `frontend/src/components/DataTable.test.tsx`, `frontend/src/views/NotFoundView.test.tsx`

**Interfaces:**
- Consumes: Task 3 `IconButton`, `Button`, `Skeleton`, `Icon`; Task 2 `registerReset`, `renderApp`.
- Produces:
  - `ListPage({ title: string; description: string; onRefresh?: () => void; refreshing?: boolean; search?: { value: string; onChange(v: string): void; placeholder?: string }; tabs?: ReactNode; children: ReactNode })` — back arrow (previous view, else Home), H1, description, Refresh, Search (FR-SHELL-014).
  - `type ListStatus = 'loading' | 'error' | 'empty' | 'ready'`; `ListBody({ status; error?: string | null; onRetry?: () => void; empty: { message: string; action?: { label: string; onSelect(): void } }; loadingLabel: string; children?: ReactNode })`.
  - `interface Column<T> { id; header; cell(row): ReactNode; align?: 'start' | 'end'; width?: string; title?(row): string }`, `interface RowAction<T> { id; label; icon: IconName; onSelect(row: T): void }`, `DataTable<T>({ caption; columns; rows; rowKey(row); rowLabel(row); rowActions?; rowClassName?(row); footer?: ReactNode })`.
  - `filterRows<T>(rows: T[], query: string, fields: (row: T) => (string | undefined)[]): T[]`.
  - `useNavStore` (`depth`), `useTrackHistoryDepth()`.
  - `type MissingKind = 'page' | 'strategy' | 'scanner' | 'backtest'`; `NotFoundView({ kind?: MissingKind })`.

- [ ] **Step 1: Write the failing tests**

Create `frontend/src/components/DataTable.test.tsx`:

```tsx
import { fireEvent, render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';
import { DataTable, type Column, type RowAction } from './DataTable';

interface Row { id: string; name: string }
const rows: Row[] = [{ id: '1', name: 'RELIANCE' }, { id: '2', name: 'TCS' }, { id: '3', name: 'INFY' }];
const columns: Column<Row>[] = [{ id: 'name', header: 'Symbol', cell: (r) => r.name, title: (r) => r.name }];

function setup() {
  const onChart = vi.fn();
  const onRemove = vi.fn();
  const actions: RowAction<Row>[] = [
    { id: 'chart', label: 'Open chart', icon: 'positions', onSelect: onChart },
    { id: 'remove', label: 'Remove', icon: 'trash', onSelect: onRemove },
  ];
  render(
    <>
      <button>before</button>
      <DataTable caption="Test rows" columns={columns} rows={rows} rowKey={(r) => r.id} rowLabel={(r) => r.name} rowActions={actions} />
    </>,
  );
  return { onChart, onRemove, user: userEvent.setup() };
}

describe('DataTable — review focus: keyboard-only user (FR-SHELL-014, FR-SHELL-015 AC1)', () => {
  it('Tab enters the active row, arrows move rows, Tab then reaches that row’s actions', async () => {
    const { user, onChart } = setup();
    screen.getByText('before').focus();
    await user.tab();
    expect(screen.getByRole('row', { name: 'RELIANCE' })).toHaveFocus();
    await user.keyboard('{ArrowDown}');
    expect(screen.getByRole('row', { name: 'TCS' })).toHaveFocus();
    await user.tab();
    expect(screen.getByRole('button', { name: 'Open chart TCS' })).toHaveFocus();
    await user.keyboard('{Enter}');
    expect(onChart).toHaveBeenCalledWith(rows[1]);
  });
  it('Home and End jump to the first and last rows', async () => {
    const { user } = setup();
    screen.getByRole('row', { name: 'RELIANCE' }).focus();
    await user.keyboard('{End}');
    expect(screen.getByRole('row', { name: 'INFY' })).toHaveFocus();
    await user.keyboard('{Home}');
    expect(screen.getByRole('row', { name: 'RELIANCE' })).toHaveFocus();
  });
  it('Shift+F10 opens a context menu with the same actions; Escape returns focus to the row', async () => {
    const { user, onRemove } = setup();
    screen.getByRole('row', { name: 'TCS' }).focus();
    await user.keyboard('{Shift>}{F10}{/Shift}');
    const menu = screen.getByRole('menu', { name: 'Actions for TCS' });
    expect(screen.getByRole('menuitem', { name: 'Open chart' })).toHaveFocus();
    await user.keyboard('{Escape}');
    expect(menu).not.toBeInTheDocument();
    expect(screen.getByRole('row', { name: 'TCS' })).toHaveFocus();
    await user.keyboard('{ContextMenu}');
    await user.keyboard('{ArrowDown}{Enter}');
    expect(onRemove).toHaveBeenCalledWith(rows[1]);
  });
  it('right-click opens the same menu', () => {
    setup();
    fireEvent.contextMenu(screen.getByRole('row', { name: 'INFY' }));
    expect(screen.getByRole('menu', { name: 'Actions for INFY' })).toBeInTheDocument();
  });
  it('truncating cells carry the full text in title (long instrument names)', () => {
    setup();
    expect(screen.getByText('RELIANCE')).toHaveAttribute('title', 'RELIANCE');
  });
});
```

Create `frontend/src/components/ListPage.test.tsx`:

```tsx
import { act, render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { useState } from 'react';
import { createMemoryRouter, Outlet, RouterProvider } from 'react-router-dom';
import { describe, expect, it, vi } from 'vitest';
import { useTrackHistoryDepth } from '../store/nav';
import { DataTable } from './DataTable';
import { filterRows } from './filterRows';
import { ListBody } from './ListBody';
import { ListPage } from './ListPage';

interface Item { id: string; symbol: string; name: string }
const items: Item[] = [
  ...Array.from({ length: 18 }, (_, i) => ({ id: `s${i}`, symbol: `SYM${i}`, name: `Company ${i}` })),
  { id: 'tcs', symbol: 'TCS', name: 'Tata Consultancy' },
  { id: 'x', symbol: 'NETX', name: 'Abtcs Networks' },
];

function SearchHost() {
  const [q, setQ] = useState('');
  const visible = filterRows(items, q, (r) => [r.symbol, r.name]);
  return (
    <ListPage title="Holdings" description="Your holdings" search={{ value: q, onChange: setQ }}>
      <DataTable caption="Holdings" columns={[{ id: 's', header: 'Symbol', cell: (r) => r.symbol }]} rows={visible} rowKey={(r) => r.id} rowLabel={(r) => r.symbol} />
    </ListPage>
  );
}

function Tracker() {
  useTrackHistoryDepth();
  return <Outlet />;
}

describe('FR-SHELL-014 list template', () => {
  it('AC2: typing "TCS" keeps only rows whose symbol or name contains "tcs"', async () => {
    const user = userEvent.setup();
    render(<RouterProvider router={createMemoryRouter([{ path: '/', element: <SearchHost /> }])} />);
    expect(screen.getAllByRole('row')).toHaveLength(21);
    await user.type(screen.getByRole('searchbox', { name: 'Search holdings' }), 'TCS');
    const body = screen.getAllByRole('rowgroup')[1];
    expect(within(body).getAllByRole('row').map((r) => r.textContent)).toEqual(['TCS', 'NETX']);
  });
  it('AC3: a failed load shows the error state with Retry instead of the empty state', async () => {
    const onRetry = vi.fn();
    const user = userEvent.setup();
    render(<ListBody status="error" error="Network unavailable" onRetry={onRetry} empty={{ message: 'You have no holdings' }} loadingLabel="Loading holdings" />);
    expect(screen.getByRole('alert')).toHaveTextContent('Network unavailable');
    expect(screen.queryByText('You have no holdings')).toBeNull();
    await user.click(screen.getByRole('button', { name: 'Retry' }));
    expect(onRetry).toHaveBeenCalledTimes(1);
  });
  it('AC1: the empty state shows the view-specific message and at most one call to action', () => {
    render(<ListBody status="empty" empty={{ message: 'No holdings', action: { label: 'Refresh', onSelect: vi.fn() } }} loadingLabel="Loading holdings" />);
    expect(screen.getByText('No holdings')).toBeInTheDocument();
    expect(screen.getAllByRole('button')).toHaveLength(1);
  });
  it('shows skeleton rows while loading', () => {
    render(<ListBody status="loading" empty={{ message: 'No holdings' }} loadingLabel="Loading holdings" />);
    expect(screen.getByRole('status', { name: 'Loading holdings' })).toBeInTheDocument();
  });
  it('AC4: the back arrow restores the previous view, and goes Home when there is none', async () => {
    const user = userEvent.setup();
    const router = createMemoryRouter(
      [{
        element: <Tracker />,
        children: [
          { path: '/a', element: <ListPage title="Orders" description="d"><p>orders body</p></ListPage> },
          { path: '/b', element: <ListPage title="Positions" description="d"><p>positions body</p></ListPage> },
          { path: '/home', element: <p>home body</p> },
        ],
      }],
      { initialEntries: ['/a'] },
    );
    render(<RouterProvider router={router} />);
    await act(() => router.navigate('/b'));
    await user.click(screen.getByRole('button', { name: 'Back' }));
    expect(screen.getByRole('heading', { name: 'Orders' })).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'Back' }));
    expect(screen.getByText('home body')).toBeInTheDocument();
  });
});
```

Create `frontend/src/views/NotFoundView.test.tsx`:

```tsx
import { render, screen } from '@testing-library/react';
import { createMemoryRouter, RouterProvider } from 'react-router-dom';
import { describe, expect, it } from 'vitest';
import { renderApp } from '../test/render';
import { NotFoundView } from './NotFoundView';

describe('FR-SHELL-012 unknown-view fallback', () => {
  it('AC1: an unregistered route shows "Page not found" with a working Go to Home', async () => {
    const { user, router } = renderApp({ route: '/scalper' }); // Scalper is a P6 view: not registered in P0
    expect(await screen.findByRole('heading', { name: 'Page not found' })).toBeInTheDocument();
    expect(screen.getByRole('navigation', { name: 'Main navigation' })).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'Go to Home' }));
    expect(router.state.location.pathname).toBe('/home');
  });
  it('AC2: names the missing item type', () => {
    render(<RouterProvider router={createMemoryRouter([{ path: '*', element: <NotFoundView kind="scanner" /> }])} />);
    expect(screen.getByRole('heading', { name: 'This scanner no longer exists' })).toBeInTheDocument();
  });
});
```

- [ ] **Step 2: Run them to verify they fail**

Run: `cd frontend && npx vitest run src/components/DataTable.test.tsx src/components/ListPage.test.tsx src/views/NotFoundView.test.tsx`
Expected: FAIL — `Failed to resolve import "./DataTable"`, `"../store/nav"`, `"./NotFoundView"`.

- [ ] **Step 3: Implement the template, table, states and fallback**

Append to `frontend/src/i18n/copy.ts`:

```ts
export const listCopy = {
  back: 'Back',
  refresh: (title: string) => `Refresh ${title.toLowerCase()}`,
  search: (title: string) => `Search ${title.toLowerCase()}`,
  searchPlaceholder: 'Search by symbol or name',
  retry: 'Retry',
  actions: 'Actions',
  rowActions: (row: string) => `Actions for ${row}`,
  rowAction: (action: string, row: string) => `${action} ${row}`,
} as const;

export const notFoundCopy = {
  page: 'Page not found',
  strategy: 'This strategy no longer exists',
  scanner: 'This scanner no longer exists',
  backtest: 'This backtest no longer exists',
  explanation: 'The view you asked for is not part of this version of Tradeloom, or the item was removed.',
  goHome: 'Go to Home',
} as const;
```

Create `frontend/src/store/nav.ts`:

```ts
import { useEffect, useRef } from 'react';
import { useLocation, useNavigationType } from 'react-router-dom';
import { create } from 'zustand';
import { registerReset } from './reset';

interface NavStore { depth: number; setDepth(n: number): void }

export const useNavStore = create<NavStore>()((set) => ({ depth: 0, setDepth: (depth) => set({ depth }) }));

registerReset(() => useNavStore.setState({ depth: 0 }));

/** Counts in-app history entries so "Back" can fall back to Home when there is nothing to go back to. */
export function useTrackHistoryDepth(): void {
  const location = useLocation();
  const type = useNavigationType();
  const first = useRef(true);
  useEffect(() => {
    const st = useNavStore.getState();
    if (first.current) {
      first.current = false;
      st.setDepth(1);
      return;
    }
    if (type === 'PUSH') st.setDepth(st.depth + 1);
    else if (type === 'POP') st.setDepth(Math.max(1, st.depth - 1));
  }, [location.key, type]);
}
```

Create `frontend/src/components/filterRows.ts`:

```ts
/** Client-side, case-insensitive "contains" filter over the given fields (FR-SHELL-014). */
export function filterRows<T>(rows: T[], query: string, fields: (row: T) => (string | undefined)[]): T[] {
  const q = query.trim().toLowerCase();
  if (!q) return rows;
  return rows.filter((row) => fields(row).some((f) => f?.toLowerCase().includes(q)));
}
```

Create `frontend/src/components/ListPage.tsx`:

```tsx
import type { ReactNode } from 'react';
import { useNavigate } from 'react-router-dom';
import { listCopy } from '../i18n/copy';
import { useNavStore } from '../store/nav';
import { Icon } from './Icon';
import { IconButton } from './IconButton';
import './list.css';

export interface ListPageProps {
  title: string;
  description: string;
  onRefresh?: () => void;
  refreshing?: boolean;
  search?: { value: string; onChange: (v: string) => void; placeholder?: string };
  tabs?: ReactNode;
  children: ReactNode;
}

export function ListPage({ title, description, onRefresh, refreshing, search, tabs, children }: ListPageProps) {
  const navigate = useNavigate();
  const goBack = () => {
    if (useNavStore.getState().depth > 1) void navigate(-1);
    else void navigate('/home');
  };
  return (
    <div className="list-page">
      <header className="list-page__header">
        <IconButton label={listCopy.back} icon="back" onClick={goBack} />
        <div className="list-page__titles">
          <h1>{title}</h1>
          <p className="muted">{description}</p>
        </div>
        <div className="list-page__tools">
          {onRefresh && <IconButton label={listCopy.refresh(title)} icon="refresh" onClick={onRefresh} disabled={refreshing} />}
          {search && (
            <label className="search-box">
              <Icon name="search" size={16} />
              <input
                type="search"
                aria-label={listCopy.search(title)}
                placeholder={search.placeholder ?? listCopy.searchPlaceholder}
                value={search.value}
                onChange={(e) => search.onChange(e.target.value)}
              />
            </label>
          )}
        </div>
      </header>
      {tabs}
      <div className="list-page__body">{children}</div>
    </div>
  );
}
```

Create `frontend/src/components/ListBody.tsx`:

```tsx
import type { ReactNode } from 'react';
import { listCopy } from '../i18n/copy';
import { Button } from './Button';
import { Skeleton } from './Skeleton';
import './list.css';

export type ListStatus = 'loading' | 'error' | 'empty' | 'ready';

export interface ListBodyProps {
  status: ListStatus;
  error?: string | null;
  onRetry?: () => void;
  empty: { message: string; action?: { label: string; onSelect: () => void } };
  loadingLabel: string;
  children?: ReactNode;
}

function EmptyArt() {
  return (
    <svg className="list-state__art" width="96" height="72" viewBox="0 0 96 72" fill="none" stroke="currentColor" strokeWidth="2" aria-hidden="true" focusable="false">
      <rect x="8" y="12" width="80" height="52" rx="6" />
      <path d="M8 26h80M24 40h28M24 50h44" strokeLinecap="round" />
    </svg>
  );
}

export function ListBody({ status, error, onRetry, empty, loadingLabel, children }: ListBodyProps) {
  if (status === 'loading') return <Skeleton rows={6} label={loadingLabel} />;
  if (status === 'error') {
    return (
      <div className="list-state" role="alert">
        <p>{error}</p>
        {onRetry && <Button onClick={onRetry}>{listCopy.retry}</Button>}
      </div>
    );
  }
  if (status === 'empty') {
    return (
      <div className="list-state">
        <EmptyArt />
        <p>{empty.message}</p>
        {empty.action && <Button variant="primary" onClick={empty.action.onSelect}>{empty.action.label}</Button>}
      </div>
    );
  }
  return <>{children}</>;
}
```

Create `frontend/src/components/DataTable.tsx`:

```tsx
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
```

Create `frontend/src/components/list.css`:

```css
.list-page { display: flex; flex-direction: column; min-height: 0; min-width: 0; padding: var(--space-4) var(--space-6); gap: var(--space-4); }
.list-page__header { display: flex; align-items: flex-start; gap: var(--space-3); }
.list-page__titles { flex: 1 1 auto; min-width: 0; display: flex; flex-direction: column; gap: var(--space-1); }
.list-page__tools { flex: none; display: flex; align-items: center; gap: var(--space-2); }
.list-page__body { min-width: 0; overflow-x: hidden; display: flex; flex-direction: column; gap: var(--space-4); }
.search-box { display: flex; align-items: center; gap: var(--space-2); padding: 0 var(--space-2); border: 1px solid var(--border); border-radius: var(--radius-1); background: var(--bg); color: var(--text-muted); }
.search-box input { width: 220px; padding: var(--space-2) 0; border: none; background: transparent; color: var(--text); outline: none; }
.search-box:focus-within { outline: 2px solid var(--focus); outline-offset: 1px; }

.data-table { width: 100%; table-layout: fixed; border-collapse: collapse; }
.data-table th {
  text-align: left; padding: var(--space-2); font-size: var(--fs-xs); font-weight: 600; color: var(--text-muted);
  background: var(--surface); border-bottom: 1px solid var(--border);
}
.data-table th.num { text-align: right; }
.data-table td { padding: var(--space-2); border-bottom: 1px solid var(--border); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.data-table tbody tr:hover, .data-table tbody tr:focus-within { background: var(--surface-2); }
.data-table tbody tr:focus-visible { outline: 2px solid var(--focus); outline-offset: -2px; }
.data-table tfoot td { font-weight: 600; border-top: 2px solid var(--border); }
.cell-truncate { display: block; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.row-actions-cell { position: relative; overflow: visible !important; }
.row-actions { display: flex; justify-content: flex-end; gap: var(--space-1); visibility: hidden; }
.data-table tr:hover .row-actions, .data-table tr:focus-within .row-actions { visibility: visible; }
.row-menu { right: 0; top: 100%; }
.row--muted td { color: var(--text-muted); }

.list-state { display: flex; flex-direction: column; align-items: center; gap: var(--space-3); padding: var(--space-8) var(--space-4); text-align: center; }
.list-state__art { color: var(--accent); }
.list-section { display: flex; flex-direction: column; gap: var(--space-2); }
.not-found { display: flex; flex-direction: column; align-items: center; justify-content: center; gap: var(--space-3); padding: var(--space-8); text-align: center; }
```

Create `frontend/src/views/NotFoundView.tsx`:

```tsx
import { useNavigate } from 'react-router-dom';
import { Button } from '../components/Button';
import { notFoundCopy } from '../i18n/copy';
import '../components/list.css';

export type MissingKind = 'page' | 'strategy' | 'scanner' | 'backtest';

export function NotFoundView({ kind = 'page' }: { kind?: MissingKind }) {
  const navigate = useNavigate();
  return (
    <section className="not-found">
      <h1>{notFoundCopy[kind]}</h1>
      <p className="muted">{notFoundCopy.explanation}</p>
      <Button variant="primary" onClick={() => void navigate('/home')}>{notFoundCopy.goHome}</Button>
    </section>
  );
}
```

In `frontend/src/app/routes.tsx`, add the import `import { NotFoundView } from '../views/NotFoundView';` and change the shell element in `buildRoutes` to:

```tsx
{ element: <ShellLayout />, children: [...shellChildren, { path: '*', element: <NotFoundView /> }] },
```

Overwrite `frontend/src/app/RootLayout.tsx`:

```tsx
import { useEffect, useLayoutEffect, useRef } from 'react';
import { Outlet, useNavigate } from 'react-router-dom';
import { useBackend } from '../api/BackendContext';
import { bootstrap, startEventBridge } from '../api/events';
import { ToastRegion } from '../components/ToastRegion';
import { useTrackHistoryDepth } from '../store/nav';

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
  useTrackHistoryDepth();
  return (
    <>
      <Outlet />
      <ToastRegion />
    </>
  );
}
```

- [ ] **Step 4: Run the suite to verify it passes**

Run: `cd frontend && npx vitest run && npm run typecheck`
Expected: PASS (22 test files), `tsc` exits 0.

- [ ] **Step 5: Commit**

```bash
git add frontend/src
git commit -m "feat(frontend): list-page template, keyboard data table, fallback view (P0-4 Task 5)" \
  -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---
