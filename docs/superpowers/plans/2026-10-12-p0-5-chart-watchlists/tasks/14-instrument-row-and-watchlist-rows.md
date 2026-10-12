> **P0-5 Chart, Watchlists & Ticker UI Implementation Plan — Task 14.** Read [`../MANIFEST.md`](../MANIFEST.md) first: its Global Constraints,
> Review Focus and Consumes/Produces apply to this task. Other tasks live beside this file in `tasks/`.

<!-- task body (verbatim from the plan) -->
### Task 14: Instrument row and watchlist rows (live prices, polled hint, remove/undo, reorder)

**Files:**
- Create: `frontend/src/components/instrument/InstrumentRow.tsx`, `frontend/src/views/watchlists/WatchlistRows.tsx`, `frontend/src/views/watchlists/watchlists.css`
- Test: `frontend/src/views/watchlists/WatchlistRows.test.tsx`

**Interfaces:**
- Consumes: `useWatchlists`, `tokensOf`, `Watchlist`, `WatchlistItem`, `labelOfItem`, `displayName`, `LabelInstrument`, `useSubscription`, `usePolled`, `<InstrumentLabel>`, `<PriceChange>`, mock `watchlistItemOf`, `setBackend`. P0-4 `IconButton` (icons `chevronRight`, `close`), `pushToast({ level, message, action: { label, onSelect } })`, `useToastStore`, `TOAST_DURATION_MS`, `useShortcut('watchlist.moveUp' | 'watchlist.moveDown', handler)`, `createMockBackend`, `demoSeed`.
- Produces:
  - `<InstrumentRow instrument decimals? selected polled? onSelect onChart? onRemove? onFocusRow? dragHandlers? trailing? />` renders a `<tr>`; `instrument: LabelInstrument`, so P3 reuses it with full `Instrument`s for scanner results and the Live Scanners widget.
  - `<WatchlistRows list onChartItem />` subscribes the list's tokens (except `missing` items) as consumer `watchlist` under source id `watchlist-panel`. Rows render straight from the `WatchlistItem` display data; no instrument lookup.

- [ ] **Step 1: Write the failing test**

`frontend/src/views/watchlists/WatchlistRows.test.tsx`:

```tsx
import { act, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';

const h = vi.hoisted(() => ({ shortcuts: {} as Record<string, () => void>, sub: vi.fn(), polledAbove: Number.POSITIVE_INFINITY }));
vi.mock('../../shortcuts/ShortcutProvider', () => ({ useShortcut: (id: string, fn: () => void) => { h.shortcuts[id] = fn; } }));
vi.mock('../../market/hooks', () => ({ useSubscription: (...a: unknown[]) => h.sub(...a), usePolled: (t: number) => t > h.polledAbove }));

import { WatchlistRows } from './WatchlistRows';
import type { Watchlist, WatchlistItem } from '../../api/backend';
import { setBackend } from '../../api/current';
import { demoSeed } from '../../api/mock/fixtures';
import { createMockBackend, type MockBackend } from '../../api/mock/index';
import { watchlistItemOf } from '../../api/mock/market';
import { TOAST_DURATION_MS, useToastStore } from '../../store/toasts';
import { tokensOf, useWatchlists } from '../../store/watchlistsStore';
import { RELIANCE, SBIN, TCS } from '../../instruments/fixtures';

let backend: MockBackend;
const synthetic = (n: number): WatchlistItem[] => Array.from({ length: n }, (_, i) => ({
  token: 10_000 + i, exchange: 'NSE', symbol: `SYM${i}`, displayName: `SYM${i}`, name: `SYNTHETIC ${i}`, isIndex: false, missing: false,
}));
const seed = (items: WatchlistItem[]) => {
  const list: Watchlist = { id: 'a', name: 'Watchlist 1', items };
  backend.state.watchlists = JSON.parse(JSON.stringify({ lists: [list], activeId: 'a' }));
  useWatchlists.setState({ lists: [list], activeId: 'a', selectedToken: null, status: 'ready', error: null });
};
const current = () => useWatchlists.getState().lists[0];
const renderRows = (onChart = vi.fn()) => render(<WatchlistRows list={current()} onChartItem={onChart} />);
const items = (...insts: (typeof SBIN)[]) => insts.map(watchlistItemOf);

beforeEach(() => {
  vi.clearAllMocks();
  h.shortcuts = {};
  h.polledAbove = Number.POSITIVE_INFINITY;
  backend = createMockBackend(demoSeed());
  setBackend(backend);
});

describe('WatchlistRows', () => {
  it('FR-WATCH-001 AC1: empty list hint', () => {
    seed([]);
    renderRows();
    expect(screen.getByText('Search to add instruments')).toBeInTheDocument();
  });

  it('FR-WATCH-001 AC3: 60 instruments all render, no cap message', () => {
    seed(synthetic(60));
    renderRows();
    expect(screen.getAllByRole('row')).toHaveLength(61); // header + 60
    expect(screen.queryByText(/limit|cap|maximum/i)).toBeNull();
  });

  it('FR-WATCH-006: subscribes the visible list as the watchlist consumer, and re-subscribes on list switch', () => {
    seed(items(SBIN, TCS));
    const { rerender } = renderRows();
    expect(h.sub).toHaveBeenLastCalledWith('watchlist-panel', 'watchlist', [SBIN.token, TCS.token]);
    rerender(<WatchlistRows list={{ id: 'b', name: 'B', items: items(RELIANCE) }} onChartItem={vi.fn()} />);
    expect(h.sub).toHaveBeenLastCalledWith('watchlist-panel', 'watchlist', [RELIANCE.token]);
  });

  it('FR-WATCH-005 AC1: clicking the symbol charts it and highlights the row', () => {
    seed(items(SBIN, RELIANCE));
    const onChart = vi.fn();
    renderRows(onChart);
    fireEvent.click(screen.getByRole('button', { name: /^RELIANCE/ }));
    expect(onChart).toHaveBeenCalledWith(watchlistItemOf(RELIANCE));
    expect(screen.getByRole('row', { name: /RELIANCE/ })).toHaveAttribute('aria-selected', 'true');
    expect(screen.getByRole('row', { name: /SBIN/ })).toHaveAttribute('aria-selected', 'false');
  });

  it('FR-WATCH-005 AC3: remove shows a 5 s Undo toast that restores the row at its position', async () => {
    seed(items(SBIN, TCS, RELIANCE));
    renderRows();
    fireEvent.click(screen.getByRole('button', { name: 'Remove TCS' }));
    await waitFor(() => expect(useToastStore.getState().visible).toHaveLength(1));
    const toast = useToastStore.getState().visible[0];
    expect(toast).toMatchObject({ level: 'info', message: 'Removed TCS from Watchlist 1', action: { label: 'Undo' } });
    expect(TOAST_DURATION_MS.info).toBe(5000);
    expect(tokensOf(current())).toEqual([SBIN.token, RELIANCE.token]);
    await act(async () => { toast.action!.onSelect(); });
    await waitFor(() => expect(tokensOf(current())).toEqual([SBIN.token, TCS.token, RELIANCE.token]));
    expect(backend.callsTo('watchlists.add').at(-1)!.args).toEqual(['a', TCS.token, 1]);
  });

  it('FR-WATCH-001 AC4: Alt+Up (registry action watchlist.moveUp) moves the focused row', async () => {
    seed(items(SBIN, TCS));
    renderRows();
    fireEvent.focus(screen.getByRole('row', { name: /TCS/ }));
    await act(async () => { h.shortcuts['watchlist.moveUp'](); });
    expect(backend.callsTo('watchlists.reorder')[0].args).toEqual(['a', [TCS.token, SBIN.token]]);
  });

  it('drag-and-drop reorders', async () => {
    seed(items(SBIN, TCS, RELIANCE));
    renderRows();
    const from = screen.getByRole('row', { name: /RELIANCE/ });
    const to = screen.getByRole('row', { name: /SBIN/ });
    fireEvent.dragStart(from);
    fireEvent.dragOver(to);
    fireEvent.drop(to);
    await waitFor(() => expect(backend.callsTo('watchlists.reorder')[0]?.args).toEqual(['a', [RELIANCE.token, SBIN.token, TCS.token]]));
  });

  it('rows beyond the budget show the polled hint', () => {
    h.polledAbove = 10_000 + 199;
    seed(synthetic(250));
    renderRows();
    expect(screen.getAllByText('polled')).toHaveLength(50);
    const row200 = screen.getByRole('row', { name: /SYM200/ });
    expect(within(row200).getByText('polled')).toHaveAttribute('title', 'Live subscription limit reached — updates every 5 s');
    expect(within(screen.getByRole('row', { name: /SYM0\b/ })).queryByText('polled')).toBeNull();
  });

  it('an instrument no longer in the master (missing item, e.g. expired contract) shows as unavailable, is not subscribed and can be removed', async () => {
    seed([watchlistItemOf(SBIN), { token: 424242, exchange: 'NFO', symbol: 'OLDFUT', displayName: '', name: '', isIndex: false, missing: true }]);
    renderRows();
    expect(screen.getByText('Instrument no longer available')).toBeInTheDocument();
    expect(h.sub).toHaveBeenLastCalledWith('watchlist-panel', 'watchlist', [SBIN.token]);
    fireEvent.click(screen.getByRole('button', { name: 'Remove unavailable instrument' }));
    await waitFor(() => expect(backend.callsTo('watchlists.remove')[0]?.args).toEqual(['a', 424242]));
  });
});
```

- [ ] **Step 2: Run the test to check it fails**

Run: `cd frontend && npx vitest run src/views/watchlists/WatchlistRows.test.tsx`
Expected: FAIL with `Failed to resolve import "./WatchlistRows"`.

- [ ] **Step 3: Implement**

`frontend/src/components/instrument/InstrumentRow.tsx`:

```tsx
import type { DragEvent, ReactNode } from 'react';
import { IconButton } from '../IconButton';
import { displayName, type LabelInstrument } from '../../instruments/naming';
import { InstrumentLabel } from './InstrumentLabel';
import { PriceChange } from './PriceChange';
import './instrument.css';

export interface InstrumentRowProps {
  instrument: LabelInstrument;
  /** Price decimals; 2 for every in-scope tick size (0.05 / 0.10). */
  decimals?: number;
  selected: boolean;
  polled?: boolean;
  onSelect(): void;
  onChart?: () => void;
  onRemove?: () => void;
  onFocusRow?: () => void;
  dragHandlers?: {
    draggable: true;
    onDragStart(e: DragEvent<HTMLTableRowElement>): void;
    onDragOver(e: DragEvent<HTMLTableRowElement>): void;
    onDrop(e: DragEvent<HTMLTableRowElement>): void;
  };
  /** Extra trailing cell content (B/S in P4; nothing in P0). */
  trailing?: ReactNode;
}

/** FR-WATCH-005 standard instrument row. Hover-only actions are also revealed on keyboard focus (NFR-11). */
export function InstrumentRow(p: InstrumentRowProps) {
  const name = displayName(p.instrument);
  return (
    <tr aria-selected={p.selected} aria-label={name} tabIndex={0} data-token={p.instrument.token}
      className={`instrument-row${p.selected ? ' is-selected' : ''}`} onFocus={p.onFocusRow}
      onKeyDown={(e) => { if (e.target === e.currentTarget && e.key === 'Enter') p.onSelect(); }} {...p.dragHandlers}>
      <td>
        <button type="button" className="instrument-row__symbol" onClick={p.onSelect}>
          <span className="instrument-row__avatar" aria-hidden="true">{name.charAt(0)}</span>
          <InstrumentLabel instrument={p.instrument} />
        </button>
      </td>
      <td className="instrument-row__price">
        <PriceChange token={p.instrument.token} decimals={p.decimals ?? 2} />
        {p.polled && <span className="instrument-row__polled" title="Live subscription limit reached — updates every 5 s">polled</span>}
      </td>
      <td className="instrument-row__actions">
        {p.onChart && <IconButton label={`Show ${name} on chart`} icon="chevronRight" onClick={p.onChart} />}
        {p.onRemove && <IconButton label={`Remove ${name}`} icon="close" onClick={p.onRemove} />}
        {p.trailing}
      </td>
    </tr>
  );
}
```

`frontend/src/views/watchlists/WatchlistRows.tsx`:

```tsx
import { useRef, type MutableRefObject } from 'react';
import type { WatchlistItem } from '../../api/backend';
import { InstrumentRow } from '../../components/instrument/InstrumentRow';
import { IconButton } from '../../components/IconButton';
import { displayName, labelOfItem } from '../../instruments/naming';
import { usePolled, useSubscription } from '../../market/hooks';
import { useShortcut } from '../../shortcuts/ShortcutProvider';
import { pushToast } from '../../store/toasts';
import { useWatchlists, type Watchlist } from '../../store/watchlistsStore';
import './watchlists.css';

export function WatchlistRows({ list, onChartItem }: { list: Watchlist; onChartItem(item: WatchlistItem): void }) {
  // Missing items (token no longer in the master) cannot stream; everything else is the watchlist consumer's set.
  useSubscription('watchlist-panel', 'watchlist', list.items.filter((i) => !i.missing).map((i) => i.token));
  const selected = useWatchlists((s) => s.selectedToken);
  const focused = useRef<number | null>(null);
  const dragged = useRef<number | null>(null);
  useShortcut('watchlist.moveUp', () => { if (focused.current !== null) void useWatchlists.getState().move(focused.current, -1); });
  useShortcut('watchlist.moveDown', () => { if (focused.current !== null) void useWatchlists.getState().move(focused.current, 1); });

  if (list.items.length === 0) return <p className="watchlist-empty">Search to add instruments</p>;

  return (
    <table className="watchlist-table" role="grid" aria-label={`${list.name} instruments`}>
      <thead>
        <tr><th scope="col">Symbol</th><th scope="col">LTP / Change</th><th scope="col"><span className="visually-hidden">Actions</span></th></tr>
      </thead>
      <tbody>
        {list.items.map((item, index) => (
          <WatchlistRow key={item.token} item={item} index={index} listName={list.name} selected={selected === item.token}
            onChartItem={onChartItem} onFocusRow={() => { focused.current = item.token; }} dragged={dragged} />
        ))}
      </tbody>
    </table>
  );
}

interface RowProps {
  item: WatchlistItem; index: number; listName: string; selected: boolean;
  onChartItem(item: WatchlistItem): void; onFocusRow(): void; dragged: MutableRefObject<number | null>;
}

function WatchlistRow({ item, index, listName, selected, onChartItem, onFocusRow, dragged }: RowProps) {
  const polled = usePolled(item.token);
  const remove = async (label: string) => {
    const r = await useWatchlists.getState().removeItem(item.token);
    if (r) {
      // P0-4 shows info toasts for TOAST_DURATION_MS.info = 5 s, the FR-WATCH-005 AC3 undo window.
      pushToast({ level: 'info', message: `Removed ${label} from ${listName}`, action: { label: 'Undo', onSelect: () => void useWatchlists.getState().restoreItem(r) } });
    }
  };
  if (item.missing) {
    return (
      <tr aria-label="Instrument no longer available" data-token={item.token}>
        <td colSpan={2} title={item.symbol}>Instrument no longer available</td>
        <td><IconButton label="Remove unavailable instrument" icon="close" onClick={() => void remove(item.symbol)} /></td>
      </tr>
    );
  }
  const label = labelOfItem(item);
  const select = () => { useWatchlists.getState().select(item.token); onChartItem(item); };
  return (
    <InstrumentRow instrument={label} selected={selected} polled={polled} onSelect={select} onChart={select}
      onRemove={() => void remove(displayName(label))} onFocusRow={onFocusRow}
      dragHandlers={{
        draggable: true,
        onDragStart: (e) => { dragged.current = item.token; e.dataTransfer?.setData('text/plain', String(item.token)); },
        onDragOver: (e) => e.preventDefault(),
        onDrop: (e) => {
          e.preventDefault();
          const from = dragged.current;
          dragged.current = null;
          if (from !== null && from !== item.token) void useWatchlists.getState().moveTo(from, index);
        },
      }} />
  );
}
```

`frontend/src/views/watchlists/watchlists.css` (colours only through P0-4's CSS variables):

```css
.watchlist-panel { display: flex; flex-direction: column; gap: 8px; min-width: 0; }
.watchlist-panel__header { display: flex; align-items: center; justify-content: space-between; }
.watchlist-panel__header h2 { margin: 0; font-size: 15px; }
.watchlist-panel__tools { display: flex; gap: 6px; align-items: center; position: relative; }
.watchlist-tabs { display: flex; gap: 4px; overflow-x: auto; scrollbar-width: thin; }
.watchlist-tab { white-space: nowrap; padding: 3px 10px; border-radius: 999px; border: 1px solid var(--border); background: none; color: inherit; cursor: pointer; }
.watchlist-tab[aria-selected="true"] { border-color: var(--accent); font-weight: 600; }
.watchlist-menu { position: absolute; z-index: 10; margin: 0; padding: 4px; list-style: none; background: var(--surface); border: 1px solid var(--border); border-radius: 6px; }
.watchlist-menu button { white-space: nowrap; background: none; border: 0; color: var(--text); padding: 6px 10px; cursor: pointer; }
.watchlist-error { margin: 0; font-size: 12px; color: var(--down); }
.watchlist-empty { color: var(--text-muted); padding: 12px 4px; }
.watchlist-table { width: 100%; border-collapse: collapse; table-layout: fixed; }
.watchlist-table th { text-align: left; font-size: 12px; font-weight: 500; color: var(--text-muted); padding: 4px; }
.watchlist-table th:nth-child(3) { width: 72px; }
.instrument-row td { padding: 4px; border-top: 1px solid var(--border); }
.instrument-row.is-selected { box-shadow: inset 3px 0 0 var(--accent); }
.instrument-row__symbol { display: inline-flex; gap: 8px; align-items: center; background: none; border: 0; color: inherit; padding: 0; cursor: pointer; min-width: 0; max-width: 100%; }
.instrument-row__avatar { display: inline-grid; place-items: center; width: 22px; height: 22px; border-radius: 50%; border: 1px solid var(--border); font-size: 11px; }
.instrument-row__price { white-space: nowrap; }
.instrument-row__polled { margin-left: 6px; font-size: 10px; color: var(--text-muted); border: 1px dashed currentColor; border-radius: 3px; padding: 0 3px; }
.instrument-row__actions { visibility: hidden; text-align: right; white-space: nowrap; }
.instrument-row:hover .instrument-row__actions, .instrument-row:focus-within .instrument-row__actions { visibility: visible; }
.watchlist-search__list { position: absolute; z-index: 20; top: 100%; left: 0; right: 0; margin: 0; padding: 4px; list-style: none; max-height: 320px; overflow-y: auto; background: var(--surface); border: 1px solid var(--border); border-radius: 6px; }
.watchlist-search__row { display: flex; align-items: center; gap: 8px; padding: 4px 6px; border-radius: 4px; cursor: pointer; }
.watchlist-search__row.is-active { outline: 1px solid var(--accent); }
.watchlist-search__hint { margin: 4px 0 0; font-size: 12px; color: var(--text-muted); }
.watchlist-search__add { margin-left: 8px; width: 18px; text-align: center; visibility: hidden; }
.watchlist-search__row:hover .watchlist-search__add, .watchlist-search__row.is-active .watchlist-search__add, .watchlist-search__add.is-in-list { visibility: visible; }
```

- [ ] **Step 4: Run the test to check it passes**

Run: `cd frontend && npx vitest run src/views/watchlists/WatchlistRows.test.tsx && npm run typecheck`
Expected: PASS (9 tests). Typecheck exits 0.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/components/instrument/InstrumentRow.tsx frontend/src/views/watchlists/WatchlistRows.tsx frontend/src/views/watchlists/WatchlistRows.test.tsx frontend/src/views/watchlists/watchlists.css
git commit -m "$(cat <<'EOF'
feat(watchlists): instrument row with live price, polled hint, remove+undo, keyboard and drag reorder

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
)"
```

---