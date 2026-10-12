> **P0-5 Chart, Watchlists & Ticker UI Implementation Plan — Task 15.** Read [`../MANIFEST.md`](../MANIFEST.md) first: its Global Constraints,
> Review Focus and Consumes/Produces apply to this task. Other tasks live beside this file in `tasks/`.

<!-- task body (verbatim from the plan) -->
### Task 15: Inline search and add to watchlist (FR-WATCH-004)

**Files:**
- Create: `frontend/src/views/watchlists/WatchlistSearch.tsx`
- Test: `frontend/src/views/watchlists/WatchlistSearch.test.tsx`

**Interfaces:**
- Consumes: `searchInstruments` (empty query → `[]`, no call), `EXCHANGE_TAGS`, `useWatchlists`, `activeList`, `tokensOf`, `useInstruments` (search results are cached for later lookups), `<InstrumentLabel>`, mock `watchlistItemOf`, `setBackend`. P0-4 `pushToast`, `useToastStore`, `createMockBackend`, `demoSeed`.
- Produces: `<WatchlistSearch />`. Its input is a combobox named "Search instruments to add". Its dropdown is a listbox named "Search results". Nothing is searched until the user types.

- [ ] **Step 1: Write the failing test**

`frontend/src/views/watchlists/WatchlistSearch.test.tsx`:

```tsx
import { fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';

vi.mock('../../api/instruments', () => ({ searchInstruments: vi.fn() }));

import { searchInstruments } from '../../api/instruments';
import type { Watchlist } from '../../api/backend';
import { setBackend } from '../../api/current';
import { demoSeed } from '../../api/mock/fixtures';
import { createMockBackend, type MockBackend } from '../../api/mock/index';
import { watchlistItemOf } from '../../api/mock/market';
import { useToastStore } from '../../store/toasts';
import { tokensOf, useWatchlists } from '../../store/watchlistsStore';
import { WatchlistSearch } from './WatchlistSearch';
import { NIFTY_OCT_FUT, SBIN, TCS } from '../../instruments/fixtures';

let backend: MockBackend;
const seed = (insts: (typeof SBIN)[]) => {
  const list: Watchlist = { id: 'a', name: 'Watchlist 1', items: insts.map(watchlistItemOf) };
  backend.state.watchlists = JSON.parse(JSON.stringify({ lists: [list], activeId: 'a' }));
  useWatchlists.setState({ lists: [list], activeId: 'a', selectedToken: null, status: 'ready', error: null });
};
const input = () => screen.getByRole('combobox', { name: 'Search instruments to add' });

beforeEach(() => {
  vi.clearAllMocks();
  backend = createMockBackend(demoSeed());
  setBackend(backend);
});

describe('WatchlistSearch', () => {
  it('AC1: "sbin" + Enter appends SBIN to the active list and keeps the dropdown open', async () => {
    seed([TCS]);
    vi.mocked(searchInstruments).mockResolvedValue([SBIN]);
    render(<WatchlistSearch />);
    fireEvent.change(input(), { target: { value: 'sbin' } });
    await screen.findByRole('option', { name: /SBIN/ });
    fireEvent.keyDown(input(), { key: 'Enter' });
    await waitFor(() => expect(backend.callsTo('watchlists.add')[0]?.args).toEqual(['a', SBIN.token, -1]));
    await waitFor(() => expect(tokensOf(useWatchlists.getState().lists[0])).toEqual([TCS.token, SBIN.token]));
    expect(screen.getByRole('listbox', { name: 'Search results' })).toBeInTheDocument();
  });

  it('AC2: an instrument already in the list shows a check and adding says "Already in Watchlist 1"', async () => {
    seed([SBIN]);
    vi.mocked(searchInstruments).mockResolvedValue([SBIN]);
    render(<WatchlistSearch />);
    fireEvent.change(input(), { target: { value: 'sbin' } });
    const opt = await screen.findByRole('option', { name: /SBIN/ });
    expect(within(opt).getByLabelText('Already in Watchlist 1')).toHaveTextContent('✓');
    fireEvent.click(opt);
    await waitFor(() => expect(useToastStore.getState().visible[0]).toMatchObject({ level: 'info', message: 'Already in Watchlist 1' }));
    expect(backend.callsTo('watchlists.add')).toHaveLength(0);
  });

  it('AC3: "nifty oct fut" finds the current-month future and a click adds it', async () => {
    seed([]);
    vi.mocked(searchInstruments).mockResolvedValue([NIFTY_OCT_FUT]);
    render(<WatchlistSearch />);
    fireEvent.change(input(), { target: { value: 'nifty oct fut' } });
    const opt = await screen.findByRole('option', { name: /NIFTY OCT FUT/ });
    expect(within(opt).getByLabelText('Add to Watchlist 1')).toHaveTextContent('+');
    fireEvent.click(opt);
    await waitFor(() => expect(backend.callsTo('watchlists.add')[0]?.args).toEqual(['a', NIFTY_OCT_FUT.token, -1]));
  });

  it('Escape closes the dropdown; ArrowDown moves the highlight; an empty box searches nothing', async () => {
    seed([]);
    vi.mocked(searchInstruments).mockResolvedValue([SBIN, TCS]);
    render(<WatchlistSearch />);
    fireEvent.change(input(), { target: { value: 's' } });
    await screen.findByRole('option', { name: /TCS/ });
    fireEvent.keyDown(input(), { key: 'ArrowDown' });
    expect(screen.getByRole('option', { name: /TCS/ })).toHaveAttribute('aria-selected', 'true');
    fireEvent.keyDown(input(), { key: 'Escape' });
    expect(screen.queryByRole('listbox')).toBeNull();
    vi.mocked(searchInstruments).mockClear();
    fireEvent.change(input(), { target: { value: '   ' } });
    await new Promise((r) => setTimeout(r, 200));
    expect(searchInstruments).not.toHaveBeenCalled();
  });
});
```

- [ ] **Step 2: Run the test to check it fails**

Run: `cd frontend && npx vitest run src/views/watchlists/WatchlistSearch.test.tsx`
Expected: FAIL with `Failed to resolve import "./WatchlistSearch"`.

- [ ] **Step 3: Implement**

`frontend/src/views/watchlists/WatchlistSearch.tsx`:

```tsx
import { useEffect, useRef, useState, type KeyboardEvent } from 'react';
import { searchInstruments } from '../../api/instruments';
import { InstrumentLabel } from '../../components/instrument/InstrumentLabel';
import { EXCHANGE_TAGS } from '../../instruments/naming';
import type { Instrument } from '../../instruments/types';
import { useInstruments } from '../../store/instrumentsStore';
import { pushToast } from '../../store/toasts';
import { activeList, useWatchlists } from '../../store/watchlistsStore';
import './watchlists.css';

const DEBOUNCE_MS = 120;

export function WatchlistSearch() {
  const list = useWatchlists((s) => activeList(s));
  const [query, setQuery] = useState('');
  const [open, setOpen] = useState(false);
  const [results, setResults] = useState<Instrument[]>([]);
  const [active, setActive] = useState(0);
  const seq = useRef(0);

  useEffect(() => {
    const q = query.trim();
    const my = ++seq.current;
    if (!q) { setResults([]); setOpen(false); return; } // Instruments.Search answers [] for an empty query
    const t = setTimeout(() => {
      searchInstruments(q, EXCHANGE_TAGS, 20)
        .then((r) => { if (my === seq.current) { setResults(r); setActive(0); setOpen(true); } })
        .catch(() => undefined);
    }, DEBOUNCE_MS);
    return () => clearTimeout(t);
  }, [query]);

  const add = async (inst: Instrument) => {
    if (!list) return;
    useInstruments.getState().put([inst]);
    const outcome = await useWatchlists.getState().addItem(inst.token);
    if (outcome === 'duplicate') pushToast({ level: 'info', message: `Already in ${list.name}` });
  };

  const onKeyDown = (e: KeyboardEvent<HTMLInputElement>) => {
    if (e.key === 'ArrowDown') { e.preventDefault(); setOpen(true); setActive((a) => Math.min(a + 1, Math.max(results.length - 1, 0))); }
    else if (e.key === 'ArrowUp') { e.preventDefault(); setActive((a) => Math.max(a - 1, 0)); }
    else if (e.key === 'Enter') { e.preventDefault(); const r = results[active]; if (r) void add(r); }
    else if (e.key === 'Escape') { e.preventDefault(); setOpen(false); }
  };

  const showList = open && results.length > 0;
  return (
    <div className="watchlist-search" style={{ position: 'relative', flex: 1 }}>
      <input role="combobox" aria-label="Search instruments to add" aria-expanded={showList} aria-controls="watchlist-search-results"
        aria-activedescendant={showList && results[active] ? `wl-opt-${results[active].token}` : undefined}
        placeholder="Search to add, e.g. SBIN" value={query} onChange={(e) => setQuery(e.target.value)} onKeyDown={onKeyDown}
        onFocus={() => { if (results.length > 0) setOpen(true); }} style={{ width: '100%' }} />
      {showList && (
        <ul id="watchlist-search-results" role="listbox" aria-label="Search results" className="watchlist-search__list">
          {results.map((r, k) => {
            const inList = !!list?.items.some((i) => i.token === r.token);
            return (
              <li key={r.token} id={`wl-opt-${r.token}`} role="option" aria-selected={k === active}
                className={`watchlist-search__row${k === active ? ' is-active' : ''}`}
                onMouseEnter={() => setActive(k)} onMouseDown={(e) => e.preventDefault()} onClick={() => void add(r)}>
                <InstrumentLabel instrument={r} showName />
                <span className={`watchlist-search__add${inList ? ' is-in-list' : ''}`}
                  aria-label={inList ? `Already in ${list?.name}` : `Add to ${list?.name}`}>{inList ? '✓' : '+'}</span>
              </li>
            );
          })}
        </ul>
      )}
    </div>
  );
}
```

- [ ] **Step 4: Run the test to check it passes**

Run: `cd frontend && npx vitest run src/views/watchlists/WatchlistSearch.test.tsx`
Expected: PASS (4 tests).

- [ ] **Step 5: Commit**

```bash
git add frontend/src/views/watchlists/WatchlistSearch.tsx frontend/src/views/watchlists/WatchlistSearch.test.tsx
git commit -m "$(cat <<'EOF'
feat(watchlists): inline search-and-add with duplicate check and keyboard add

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
)"
```

---