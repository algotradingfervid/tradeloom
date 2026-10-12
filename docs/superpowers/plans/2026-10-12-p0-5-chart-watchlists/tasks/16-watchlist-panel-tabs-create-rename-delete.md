> **P0-5 Chart, Watchlists & Ticker UI Implementation Plan — Task 16.** Read [`../MANIFEST.md`](../MANIFEST.md) first: its Global Constraints,
> Review Focus and Consumes/Produces apply to this task. Other tasks live beside this file in `tasks/`.

<!-- task body (verbatim from the plan) -->
### Task 16: Watchlist panel — tabs, create, rename, delete; mount on Home

**Files:**
- Create: `frontend/src/views/watchlists/WatchlistTabs.tsx`, `frontend/src/views/watchlists/WatchlistPanel.tsx`
- Test: `frontend/src/views/watchlists/WatchlistPanel.test.tsx`
- Modify: `frontend/src/views/home/slots.tsx` (P0-4 Task 12: replace the body of `HomeWatchlistArea`)

**Interfaces:**
- Consumes: `useWatchlists`, `activeList`, `nextWatchlistName`, `<WatchlistRows>`, `<WatchlistSearch>`, `showOnDashboard`, `itemIdentifier`, mock `watchlistItemOf`, `setBackend`. P0-4 `IconButton` (icon `trash`), `ConfirmDialog`, `Button`, `homeCopy.watchlistArea`, `HomeWatchlistArea` (`src/views/home/slots.tsx`), `createMockBackend`, `demoSeed`.
- Produces: `<WatchlistPanel onChartItem(item: WatchlistItem) />` (a `<section aria-label="Watchlist">`); `<WatchlistTabs />`.

- [ ] **Step 1: Write the failing test**

`frontend/src/views/watchlists/WatchlistPanel.test.tsx` (P0-4's mock backend with this plan's watchlist domain):

```tsx
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';

vi.mock('../../shortcuts/ShortcutProvider', () => ({ useShortcut: vi.fn() }));
vi.mock('../../market/hooks', () => ({ useSubscription: vi.fn(), usePolled: () => false }));
vi.mock('../../api/instruments', async () => {
  const f = await import('../../instruments/fixtures');
  return { searchInstruments: vi.fn(async () => []), instrumentsByTokens: vi.fn(async (t: number[]) => f.ALL_IN_SCOPE.filter((i) => t.includes(i.token))) };
});

import type { Watchlist } from '../../api/backend';
import { setBackend } from '../../api/current';
import { demoSeed } from '../../api/mock/fixtures';
import { createMockBackend, type MockBackend } from '../../api/mock/index';
import { watchlistItemOf } from '../../api/mock/market';
import { WatchlistPanel } from './WatchlistPanel';
import { useWatchlists } from '../../store/watchlistsStore';
import { SBIN, TCS } from '../../instruments/fixtures';

let backend: MockBackend;
const lists = (...names: string[]): Watchlist[] =>
  names.map((name, i) => ({ id: `w${i + 1}`, name, items: i === 0 ? [SBIN, TCS].map(watchlistItemOf) : [] }));
const seed = (ls: Watchlist[]) => { backend.state.watchlists = { lists: ls, activeId: ls[0].id }; };
const renderPanel = () => render(<WatchlistPanel onChartItem={vi.fn()} />);

beforeEach(() => {
  backend = createMockBackend(demoSeed());
  setBackend(backend);
  useWatchlists.setState({ lists: [], activeId: null, selectedToken: null, status: 'idle', error: null });
});

describe('WatchlistPanel', () => {
  it('FR-WATCH-001 AC1: fresh install shows one empty "Watchlist 1" with the hint', async () => {
    renderPanel();
    expect(await screen.findByRole('tab', { name: 'Watchlist 1' })).toHaveAttribute('aria-selected', 'true');
    expect(screen.getByText('Search to add instruments')).toBeInTheDocument();
    expect(screen.getByRole('region', { name: 'Watchlist' })).toBeInTheDocument();
  });

  it('FR-WATCH-001 AC2: clicking the second tab shows it and persists it as active in Go', async () => {
    seed(lists('Watchlist 1', 'Watchlist 2', 'Watchlist 3'));
    renderPanel();
    fireEvent.click(await screen.findByRole('tab', { name: 'Watchlist 2' }));
    await waitFor(() => expect(backend.callsTo('watchlists.setActive').at(-1)?.args).toEqual(['w2']));
    expect(backend.state.watchlists.activeId).toBe('w2');
    expect(screen.getByRole('tab', { name: 'Watchlist 2' })).toHaveAttribute('aria-selected', 'true');
    expect(screen.getByText('Search to add instruments')).toBeInTheDocument();
  });

  it('FR-WATCH-002 AC1: "New watchlist +" pre-fills "Watchlist 3"; Enter creates and activates it', async () => {
    seed(lists('Watchlist 1', 'Watchlist 2'));
    renderPanel();
    fireEvent.click(await screen.findByRole('button', { name: 'New watchlist +' }));
    const name = screen.getByRole('textbox', { name: 'New watchlist name' });
    expect(name).toHaveValue('Watchlist 3');
    fireEvent.keyDown(name, { key: 'Enter' });
    expect(await screen.findByRole('tab', { name: 'Watchlist 3' })).toHaveAttribute('aria-selected', 'true');
    expect(backend.callsTo('watchlists.create')[0].args).toEqual(['Watchlist 3']);
  });

  it('FR-WATCH-002 AC3: empty name is refused; Escape cancels', async () => {
    seed(lists('Watchlist 1'));
    renderPanel();
    fireEvent.click(await screen.findByRole('button', { name: 'New watchlist +' }));
    const name = screen.getByRole('textbox', { name: 'New watchlist name' });
    fireEvent.change(name, { target: { value: '  ' } });
    fireEvent.keyDown(name, { key: 'Enter' });
    expect(await screen.findByRole('alert')).toHaveTextContent('Enter a name');
    expect(backend.callsTo('watchlists.create')).toHaveLength(0);
    fireEvent.keyDown(name, { key: 'Escape' });
    expect(screen.queryByRole('textbox', { name: 'New watchlist name' })).toBeNull();
  });

  it('FR-WATCH-002 AC2: renaming (double-click) to an existing name shows the inline error and changes nothing', async () => {
    seed(lists('Watchlist 1', 'Banks'));
    renderPanel();
    fireEvent.doubleClick(await screen.findByRole('tab', { name: 'Banks' }));
    const field = screen.getByRole('textbox', { name: 'Watchlist name' });
    fireEvent.change(field, { target: { value: 'watchlist 1' } });
    fireEvent.keyDown(field, { key: 'Enter' });
    expect(await screen.findByRole('alert')).toHaveTextContent('A watchlist with this name exists');
    expect(backend.callsTo('watchlists.rename')).toHaveLength(0);
  });

  it('context menu "Rename" renames', async () => {
    seed(lists('Watchlist 1'));
    renderPanel();
    fireEvent.contextMenu(await screen.findByRole('tab', { name: 'Watchlist 1' }));
    fireEvent.click(screen.getByRole('menuitem', { name: 'Rename' }));
    const field = screen.getByRole('textbox', { name: 'Watchlist name' });
    fireEvent.change(field, { target: { value: 'Core' } });
    fireEvent.keyDown(field, { key: 'Enter' });
    expect(await screen.findByRole('tab', { name: 'Core' })).toBeInTheDocument();
    expect(backend.callsTo('watchlists.rename')[0].args).toEqual(['w1', 'Core']);
  });

  it('FR-WATCH-003 AC1/AC2: delete asks with name and count; Cancel keeps it; Delete on the only list leaves "Watchlist 1"', async () => {
    seed(lists('Mine'));
    renderPanel();
    fireEvent.click(await screen.findByRole('button', { name: 'Delete watchlist Mine' }));
    expect(screen.getByText('Delete watchlist Mine and its 2 instruments?')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Cancel' }));
    expect(backend.callsTo('watchlists.delete')).toHaveLength(0);
    expect(screen.getByRole('tab', { name: 'Mine' })).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Delete watchlist Mine' }));
    fireEvent.click(screen.getByRole('button', { name: 'Delete' }));
    expect(await screen.findByRole('tab', { name: 'Watchlist 1' })).toHaveAttribute('aria-selected', 'true');
    expect(backend.callsTo('watchlists.delete')[0].args).toEqual(['w1']);
  });
});
```

- [ ] **Step 2: Run the test to check it fails**

Run: `cd frontend && npx vitest run src/views/watchlists/WatchlistPanel.test.tsx`
Expected: FAIL with `Failed to resolve import "./WatchlistPanel"`.

- [ ] **Step 3: Implement**

`frontend/src/views/watchlists/WatchlistTabs.tsx`:

```tsx
import { useState, type KeyboardEvent } from 'react';
import { activeList, useWatchlists } from '../../store/watchlistsStore';
import { nextWatchlistName } from './names';
import './watchlists.css';

export function WatchlistTabs() {
  const lists = useWatchlists((s) => s.lists);
  const activeId = useWatchlists((s) => activeList(s)?.id);
  const [creating, setCreating] = useState<string | null>(null);
  const [renaming, setRenaming] = useState<{ id: string; value: string } | null>(null);
  const [menuFor, setMenuFor] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  const submitCreate = async () => {
    const err = await useWatchlists.getState().create(creating ?? '');
    if (err) setError(err); else { setCreating(null); setError(null); }
  };
  const submitRename = async () => {
    if (!renaming) return;
    const err = await useWatchlists.getState().rename(renaming.id, renaming.value);
    if (err) setError(err); else { setRenaming(null); setError(null); }
  };
  const keys = (submit: () => Promise<void>, cancel: () => void) => (e: KeyboardEvent<HTMLInputElement>) => {
    if (e.key === 'Enter') { e.preventDefault(); void submit(); }
    else if (e.key === 'Escape') { e.preventDefault(); cancel(); setError(null); }
  };
  const startRename = (id: string) => {
    const l = lists.find((x) => x.id === id);
    if (l) { setRenaming({ id: l.id, value: l.name }); setError(null); }
    setMenuFor(null);
  };

  return (
    <div className="watchlist-tabs-wrap">
      <div className="watchlist-panel__header">
        <h2>Watchlist</h2>
        <button type="button" className="watchlist-new" onClick={() => { setCreating(nextWatchlistName(lists.map((l) => l.name))); setError(null); }}>
          New watchlist +
        </button>
      </div>
      <div role="tablist" aria-label="Watchlists" className="watchlist-tabs">
        {lists.map((l) => (renaming?.id === l.id ? (
          <input key={l.id} aria-label="Watchlist name" autoFocus value={renaming.value}
            onChange={(e) => setRenaming({ id: l.id, value: e.target.value })} onKeyDown={keys(submitRename, () => setRenaming(null))} />
        ) : (
          <button key={l.id} type="button" role="tab" aria-selected={l.id === activeId} className="watchlist-tab"
            title="Double-click or F2 to rename"
            onClick={() => void useWatchlists.getState().setActive(l.id)}
            onDoubleClick={() => startRename(l.id)}
            onContextMenu={(e) => { e.preventDefault(); setMenuFor(l.id); }}
            onKeyDown={(e) => { if (e.key === 'F2') { e.preventDefault(); startRename(l.id); } }}>
            {l.name}
          </button>
        )))}
      </div>
      {menuFor && (
        <ul role="menu" className="watchlist-menu" onKeyDown={(e) => { if (e.key === 'Escape') setMenuFor(null); }}>
          <li role="none"><button type="button" role="menuitem" autoFocus onClick={() => startRename(menuFor)}>Rename</button></li>
        </ul>
      )}
      {creating !== null && (
        <input aria-label="New watchlist name" autoFocus value={creating}
          onChange={(e) => setCreating(e.target.value)} onKeyDown={keys(submitCreate, () => setCreating(null))} />
      )}
      {error && <p role="alert" className="watchlist-error">{error}</p>}
    </div>
  );
}
```

`frontend/src/views/watchlists/WatchlistPanel.tsx`:

```tsx
import { useEffect, useState } from 'react';
import type { WatchlistItem } from '../../api/backend';
import { Button } from '../../components/Button';
import { ConfirmDialog } from '../../components/ConfirmDialog';
import { IconButton } from '../../components/IconButton';
import { activeList, useWatchlists } from '../../store/watchlistsStore';
import { WatchlistRows } from './WatchlistRows';
import { WatchlistSearch } from './WatchlistSearch';
import { WatchlistTabs } from './WatchlistTabs';
import './watchlists.css';

export function WatchlistPanel({ onChartItem }: { onChartItem(item: WatchlistItem): void }) {
  const status = useWatchlists((s) => s.status);
  const error = useWatchlists((s) => s.error);
  const active = useWatchlists((s) => activeList(s));
  const [confirmDelete, setConfirmDelete] = useState(false);

  useEffect(() => {
    if (useWatchlists.getState().status === 'idle') void useWatchlists.getState().load();
  }, []);

  return (
    <section className="watchlist-panel" aria-label="Watchlist">
      <WatchlistTabs />
      <div className="watchlist-panel__tools">
        <WatchlistSearch />
        {active && <IconButton label={`Delete watchlist ${active.name}`} icon="trash" onClick={() => setConfirmDelete(true)} />}
      </div>
      {status === 'error' && (
        <p role="alert" className="watchlist-error">
          {error} <Button onClick={() => void useWatchlists.getState().load()}>Retry</Button>
        </p>
      )}
      {active && <WatchlistRows list={active} onChartItem={onChartItem} />}
      {active && (
        <ConfirmDialog open={confirmDelete} title="Delete watchlist" tone="danger"
          message={`Delete watchlist ${active.name} and its ${active.items.length} instruments?`} confirmLabel="Delete"
          onConfirm={() => { setConfirmDelete(false); void useWatchlists.getState().deleteList(active.id); }}
          onCancel={() => setConfirmDelete(false)} />
      )}
    </section>
  );
}
```

In `frontend/src/views/home/slots.tsx` (P0-4 Task 12) add `import { WatchlistPanel } from '../watchlists/WatchlistPanel';`, `import { showOnDashboard } from '../../store/chartContextsStore';` and `import { itemIdentifier } from '../../instruments/naming';`, and replace the body of `HomeWatchlistArea`. The slot keeps P0-4's card styling but becomes a plain `div`, so the page has one "Watchlist" region (the panel's own `section`), not two nested regions:

```tsx
export function HomeWatchlistArea() {
  return (
    <div className="card home-slot">
      <WatchlistPanel onChartItem={(item) => showOnDashboard(itemIdentifier(item))} />
    </div>
  );
}
```

`homeCopy.watchlistArea` is no longer used by `slots.tsx`; leave it in P0-4's `copy.ts` (other P0-4 code may reference it).

- [ ] **Step 4: Run the tests to check they pass**

Run: `cd frontend && npx vitest run src/views/watchlists && npm run typecheck`
Expected: PASS (the 7 panel tests plus the Task 13–15 tests). Typecheck exits 0.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/views/watchlists/WatchlistTabs.tsx frontend/src/views/watchlists/WatchlistPanel.tsx frontend/src/views/watchlists/WatchlistPanel.test.tsx frontend/src/views/home/slots.tsx
git commit -m "$(cat <<'EOF'
feat(watchlists): panel with tabs, create/rename/delete and the Home slot wired to the dashboard chart

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
)"
```

---