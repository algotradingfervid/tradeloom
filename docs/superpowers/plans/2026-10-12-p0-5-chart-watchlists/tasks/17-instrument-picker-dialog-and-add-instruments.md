> **P0-5 Chart, Watchlists & Ticker UI Implementation Plan — Task 17.** Read [`../MANIFEST.md`](../MANIFEST.md) first: its Global Constraints,
> Review Focus and Consumes/Produces apply to this task. Other tasks live beside this file in `tasks/`.

<!-- task body (verbatim from the plan) -->
### Task 17: Instrument picker dialog (FR-WATCH-007) and "Add instruments…" on the panel

**Files:**
- Create: `frontend/src/views/watchlists/InstrumentPicker.tsx`
- Test: `frontend/src/views/watchlists/InstrumentPicker.test.tsx`
- Modify: `frontend/src/views/watchlists/WatchlistPanel.tsx` (add the button and the picker), `frontend/src/views/watchlists/WatchlistPanel.test.tsx`

**Interfaces:**
- Consumes: `searchInstruments` (empty query → `[]`), `EXCHANGE_TAGS`, `ExchangeTag`, `ensureInstruments` (watchlist items carry display data only, so the picker loads full instruments for a watchlist chip), `useInstruments`, `useWatchlists`, `tokensOf`, `nextWatchlistName`, `useSubscription` (consumer `widget`), `<InstrumentLabel>`, `<PriceChange>`, `displayName`, mock `watchlistItemOf`, `setBackend`. P0-4 `Dialog`, `Button`, `IconButton` (icons `close`, `plus`), `pushToast`, `useToastStore`, `createMockBackend`, `demoSeed`.
- Produces: `<InstrumentPicker open title? max? segments? initial? onClose onDone(selected: Instrument[]) />`. The equity builder (P1) and scanner custom symbols (P3) reuse it. Universe chips are added in P1, when the universe store exists. With an empty search box and no list chip chosen it shows "Type to search".

- [ ] **Step 1: Write the failing test**

`frontend/src/views/watchlists/InstrumentPicker.test.tsx`:

```tsx
import { fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';

const h = vi.hoisted(() => ({ extra: [] as unknown[] }));
vi.mock('../../market/hooks', () => ({ useSubscription: vi.fn(), usePolled: () => false }));
vi.mock('../../api/instruments', async () => {
  const f = await import('../../instruments/fixtures');
  const all = () => [...f.ALL_IN_SCOPE, ...(h.extra as typeof f.ALL_IN_SCOPE)];
  return {
    searchInstruments: vi.fn(async (q: string, tags: string[]) => all().filter((i) => tags.includes(i.tag) && i.tradingsymbol.toLowerCase().includes(q.toLowerCase()))),
    instrumentsByTokens: vi.fn(async (t: number[]) => all().filter((i) => t.includes(i.token))),
  };
});

import { searchInstruments } from '../../api/instruments';
import type { Watchlist } from '../../api/backend';
import { setBackend } from '../../api/current';
import { demoSeed } from '../../api/mock/fixtures';
import { createMockBackend, type MockBackend } from '../../api/mock/index';
import { watchlistItemOf } from '../../api/mock/market';
import { InstrumentPicker } from './InstrumentPicker';
import { useInstruments } from '../../store/instrumentsStore';
import { useToastStore } from '../../store/toasts';
import { useWatchlists } from '../../store/watchlistsStore';
import { RELIANCE, RELIANCE_OCT_FUT, SBIN } from '../../instruments/fixtures';
import type { Instrument } from '../../instruments/types';

let backend: MockBackend;
const many = (n: number, base = 50_000): Instrument[] =>
  Array.from({ length: n }, (_, i) => ({ ...SBIN, token: base + i, tradingsymbol: `PK${i}`, displayName: `PK${i}` }));
const wl = (id: string, name: string, insts: Instrument[] = []): Watchlist => ({ id, name, items: insts.map(watchlistItemOf) });
const seedLists = (lists: Watchlist[]) => {
  backend.state.watchlists = JSON.parse(JSON.stringify({ lists, activeId: lists[0].id }));
  useWatchlists.setState({ lists, activeId: lists[0].id, selectedToken: null, status: 'ready', error: null });
};
const setup = (props: Partial<Parameters<typeof InstrumentPicker>[0]> = {}) => {
  const onClose = vi.fn(); const onDone = vi.fn();
  render(<InstrumentPicker open onClose={onClose} onDone={onDone} {...props} />);
  return { onClose, onDone };
};

beforeEach(() => {
  vi.clearAllMocks();
  h.extra = [];
  backend = createMockBackend(demoSeed());
  setBackend(backend);
  useInstruments.setState({ byToken: {}, byId: {} });
  seedLists([wl('w1', 'Watchlist 1')]);
});

describe('InstrumentPicker', () => {
  it('an empty search box shows "Type to search" and sends no search', async () => {
    setup();
    expect(screen.getByText('Type to search')).toBeInTheDocument();
    await new Promise((r) => setTimeout(r, 200));
    expect(searchInstruments).not.toHaveBeenCalled();
  });

  it('AC3: without a caller maximum, 300 instruments can be selected (watchlist chip + select all)', async () => {
    h.extra = many(300);
    seedLists([wl('w1', 'Watchlist 1'), wl('big', 'Big', many(300))]);
    setup();
    fireEvent.click(screen.getByRole('button', { name: 'Big' }));
    await waitFor(() => expect(screen.getAllByRole('checkbox', { name: /^Select PK/ })).toHaveLength(300));
    fireEvent.click(screen.getByRole('checkbox', { name: 'Select all' }));
    expect(screen.getByRole('heading', { name: 'Selected (300)' })).toBeInTheDocument();
  });

  it('AC2: with max 100 and 100 selected, another tick is refused with "Up to 100 instruments"', async () => {
    h.extra = many(100);
    setup({ max: 100, initial: many(100) });
    expect(screen.getByRole('heading', { name: 'Selected (100 / 100)' })).toBeInTheDocument();
    fireEvent.change(screen.getByRole('textbox', { name: 'Search instruments' }), { target: { value: 'reliance' } });
    fireEvent.click((await screen.findAllByRole('checkbox', { name: 'Select RELIANCE' }))[0]); // NSE and BSE listings share the name
    expect(screen.getByRole('alert')).toHaveTextContent('Up to 100 instruments');
    expect(screen.getByRole('heading', { name: 'Selected (100 / 100)' })).toBeInTheDocument();
  });

  it('caller tag restriction hides other tags; duplicates are ignored; the remove button removes', async () => {
    seedLists([wl('w1', 'Watchlist 1', [RELIANCE, SBIN])]);
    setup({ segments: ['NSE', 'INDICES'], initial: [RELIANCE] });
    fireEvent.change(screen.getByRole('textbox', { name: 'Search instruments' }), { target: { value: 'reliance' } });
    expect(await screen.findByRole('checkbox', { name: 'Select RELIANCE' })).toBeChecked(); // only the NSE listing: BSE and NFO-FUT filtered
    expect(screen.queryByText(RELIANCE_OCT_FUT.displayName)).toBeNull();
    fireEvent.click(screen.getByRole('button', { name: 'Watchlist 1' }));
    await screen.findByRole('checkbox', { name: 'Select SBIN' });
    fireEvent.click(screen.getByRole('checkbox', { name: 'Select all' }));
    expect(screen.getByRole('heading', { name: 'Selected (2)' })).toBeInTheDocument();
    fireEvent.click(within(screen.getByRole('list', { name: 'Selected instruments' })).getByRole('button', { name: 'Remove RELIANCE' }));
    expect(screen.getByRole('heading', { name: 'Selected (1)' })).toBeInTheDocument();
  });

  it('AC4: "Save as watchlist" named "Banks" creates it from the selection without switching the active list', async () => {
    setup({ initial: [SBIN, RELIANCE] });
    fireEvent.click(screen.getByRole('button', { name: 'Save as watchlist' }));
    const name = screen.getByRole('textbox', { name: 'Watchlist name' });
    expect(name).toHaveValue('Watchlist 2');
    fireEvent.change(name, { target: { value: 'Banks' } });
    fireEvent.keyDown(name, { key: 'Enter' });
    await waitFor(() => expect(useToastStore.getState().visible[0]).toMatchObject({ level: 'info', message: 'Watchlist Banks created' }));
    expect(backend.callsTo('watchlists.create')[0].args).toEqual(['Banks']);
    const banks = useWatchlists.getState().lists.find((l) => l.name === 'Banks')!;
    expect(backend.callsTo('watchlists.add').map((c) => c.args)).toEqual([[banks.id, SBIN.token, -1], [banks.id, RELIANCE.token, -1]]);
    expect(useWatchlists.getState().activeId).toBe('w1');
  });

  it('Done returns the selection', () => {
    const { onDone } = setup({ initial: [SBIN] });
    fireEvent.click(screen.getByRole('button', { name: 'Done' }));
    expect(onDone).toHaveBeenCalledWith([SBIN]);
  });

  it('AC5: Escape closes without applying', async () => {
    const { onClose, onDone } = setup({ initial: [SBIN] });
    fireEvent.keyDown(screen.getByRole('textbox', { name: 'Search instruments' }), { key: 'Escape' });
    await waitFor(() => expect(onClose).toHaveBeenCalled());
    expect(onDone).not.toHaveBeenCalled();
  });
});
```

- [ ] **Step 2: Run the test to check it fails**

Run: `cd frontend && npx vitest run src/views/watchlists/InstrumentPicker.test.tsx`
Expected: FAIL with `Failed to resolve import "./InstrumentPicker"`.

- [ ] **Step 3: Implement**

`frontend/src/views/watchlists/InstrumentPicker.tsx`:

```tsx
import { useEffect, useRef, useState } from 'react';
import { Button } from '../../components/Button';
import { Dialog } from '../../components/Dialog';
import { IconButton } from '../../components/IconButton';
import { InstrumentLabel } from '../../components/instrument/InstrumentLabel';
import { PriceChange } from '../../components/instrument/PriceChange';
import { searchInstruments } from '../../api/instruments';
import { displayName, EXCHANGE_TAGS, type ExchangeTag } from '../../instruments/naming';
import type { Instrument } from '../../instruments/types';
import { useSubscription } from '../../market/hooks';
import { ensureInstruments, useInstruments } from '../../store/instrumentsStore';
import { pushToast } from '../../store/toasts';
import { tokensOf, useWatchlists } from '../../store/watchlistsStore';
import { nextWatchlistName } from './names';
import './watchlists.css';

export interface InstrumentPickerProps {
  open: boolean;
  title?: string;
  max?: number;
  segments?: readonly ExchangeTag[];
  initial?: readonly Instrument[];
  onClose(): void;
  onDone(selected: Instrument[]): void;
}

type Source = { kind: 'search' } | { kind: 'watchlist'; id: string };

export function InstrumentPicker({ open, title = 'Add instruments', max, segments = EXCHANGE_TAGS, initial = [], onClose, onDone }: InstrumentPickerProps) {
  const lists = useWatchlists((s) => s.lists);
  const [query, setQuery] = useState('');
  const [source, setSource] = useState<Source>({ kind: 'search' });
  const [rows, setRows] = useState<Instrument[]>([]);
  const [selected, setSelected] = useState<Instrument[]>([...initial]);
  const [limitMsg, setLimitMsg] = useState<string | null>(null);
  const [saving, setSaving] = useState<string | null>(null);
  const [saveError, setSaveError] = useState<string | null>(null);
  const seq = useRef(0);
  const segKey = segments.join(',');

  useEffect(() => {
    if (!open) return;
    setSelected([...initial]); setQuery(''); setSource({ kind: 'search' }); setLimitMsg(null); setSaving(null); setSaveError(null);
    // eslint-disable-next-line react-hooks/exhaustive-deps -- `initial` is read when the dialog opens
  }, [open]);

  useEffect(() => {
    if (!open) return;
    const my = ++seq.current;
    const allowed = segKey.split(',');
    if (source.kind === 'search' && query.trim() === '') { setRows([]); return; } // Go answers [] for an empty query
    const load = async (): Promise<Instrument[]> => {
      if (source.kind === 'watchlist') {
        const list = useWatchlists.getState().lists.find((l) => l.id === source.id);
        const tokens = list ? tokensOf(list) : [];
        await ensureInstruments(tokens);
        const byToken = useInstruments.getState().byToken;
        return tokens.map((t) => byToken[t]).filter((i): i is Instrument => i !== undefined);
      }
      return searchInstruments(query.trim(), allowed as ExchangeTag[], 100);
    };
    const t = setTimeout(() => {
      load().then((r) => { if (my === seq.current) setRows(r.filter((i) => allowed.includes(i.tag))); }).catch(() => undefined);
    }, source.kind === 'search' ? 120 : 0);
    return () => clearTimeout(t);
  }, [open, query, source, segKey]);

  useSubscription('instrument-picker', 'widget', open ? selected.map((s) => s.token) : []);

  const isSelected = (token: number) => selected.some((s) => s.token === token);
  const addMany = (items: readonly Instrument[]) => {
    const next = [...selected];
    let refused = false;
    for (const i of items) {
      if (next.some((s) => s.token === i.token)) continue; // duplicates ignored
      if (max !== undefined && next.length >= max) { refused = true; break; }
      next.push(i);
    }
    setSelected(next);
    setLimitMsg(refused ? `Up to ${max} instruments` : null);
  };
  const remove = (token: number) => { setSelected(selected.filter((s) => s.token !== token)); setLimitMsg(null); };
  const allListed = rows.length > 0 && rows.every((r) => isSelected(r.token));

  const saveAsWatchlist = async () => {
    const name = saving ?? '';
    const err = await useWatchlists.getState().create(name, selected.map((s) => s.token), false);
    if (err) { setSaveError(err); return; }
    pushToast({ level: 'info', message: `Watchlist ${name.trim()} created` });
    setSaving(null);
    setSaveError(null);
  };

  const footer = (
    <>
      {saving === null ? (
        <Button disabled={selected.length === 0} onClick={() => { setSaving(nextWatchlistName(lists.map((l) => l.name))); setSaveError(null); }}>
          Save as watchlist
        </Button>
      ) : (
        <span className="picker__save">
          <input aria-label="Watchlist name" autoFocus value={saving} onChange={(e) => setSaving(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === 'Enter') { e.preventDefault(); void saveAsWatchlist(); }
              if (e.key === 'Escape') { e.preventDefault(); e.stopPropagation(); setSaving(null); setSaveError(null); }
            }} />
          <Button onClick={() => void saveAsWatchlist()}>Save</Button>
        </span>
      )}
      <Button variant="primary" onClick={() => onDone(selected)}>Done</Button>
    </>
  );

  return (
    <Dialog open={open} title={title} onClose={onClose} footer={footer} size="lg">
      <div className="picker" style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 16, width: 'min(900px, 90vw)' }}>
        <div className="picker__left">
          <input type="text" aria-label="Search instruments" placeholder="Search, e.g. SBIN, TCS" value={query}
            onChange={(e) => { setQuery(e.target.value); setSource({ kind: 'search' }); }}
            onKeyDown={(e) => { if (e.key === 'Escape') onClose(); }} style={{ width: '100%' }} />
          <div className="picker__chips" role="group" aria-label="Lists">
            {lists.map((l) => (
              <button key={l.id} type="button" aria-pressed={source.kind === 'watchlist' && source.id === l.id}
                className="watchlist-tab" onClick={() => setSource({ kind: 'watchlist', id: l.id })}>{l.name}</button>
            ))}
          </div>
          {source.kind === 'search' && query.trim() === '' && <p className="watchlist-search__hint">Type to search</p>}
          <label className="picker__all">
            <input type="checkbox" aria-label="Select all" checked={allListed}
              onChange={() => (allListed ? setSelected(selected.filter((s) => !rows.some((r) => r.token === s.token))) : addMany(rows))} />
            {' '}Select all
          </label>
          <ul aria-label="Instruments" style={{ listStyle: 'none', margin: 0, padding: 0, maxHeight: '50vh', overflowY: 'auto' }}>
            {rows.map((r) => (
              <li key={r.token}>
                <label style={{ display: 'flex', gap: 8, alignItems: 'center' }}>
                  <input type="checkbox" aria-label={`Select ${displayName(r)}`} checked={isSelected(r.token)}
                    onChange={() => (isSelected(r.token) ? remove(r.token) : addMany([r]))} />
                  <InstrumentLabel instrument={r} showName />
                </label>
              </li>
            ))}
          </ul>
        </div>
        <div className="picker__right">
          <h3 aria-live="polite">{max === undefined ? `Selected (${selected.length})` : `Selected (${selected.length} / ${max})`}</h3>
          {limitMsg && <p role="alert">{limitMsg}</p>}
          <ul aria-label="Selected instruments" style={{ listStyle: 'none', margin: 0, padding: 0, maxHeight: '50vh', overflowY: 'auto' }}>
            {selected.map((s) => (
              <li key={s.token} style={{ display: 'flex', gap: 8, alignItems: 'center', justifyContent: 'space-between' }}>
                <InstrumentLabel instrument={s} />
                <PriceChange token={s.token} />
                <IconButton label={`Remove ${displayName(s)}`} icon="close" onClick={() => remove(s.token)} />
              </li>
            ))}
          </ul>
          {saveError && <p role="alert">{saveError}</p>}
        </div>
      </div>
    </Dialog>
  );
}
```

Modify `frontend/src/views/watchlists/WatchlistPanel.tsx`:
- Add the import `import { InstrumentPicker } from './InstrumentPicker';`.
- Add the state `const [pickerOpen, setPickerOpen] = useState(false);`.
- Inside `.watchlist-panel__tools`, before the delete button, add:

```tsx
        {active && <IconButton label="Add instruments…" icon="plus" onClick={() => setPickerOpen(true)} />}
```

- Before `</section>`, add:

```tsx
      <InstrumentPicker open={pickerOpen} onClose={() => setPickerOpen(false)}
        onDone={(sel) => { setPickerOpen(false); void useWatchlists.getState().addItems(sel.map((i) => i.token)); }} />
```

Append this test to `frontend/src/views/watchlists/WatchlistPanel.test.tsx`. Also add `within` to its `@testing-library/react` import:

```tsx
  it('FR-WATCH-007 via panel: "Add instruments…" → search → pick → Done appends to the active list', async () => {
    const { searchInstruments } = await import('../../api/instruments');
    vi.mocked(searchInstruments).mockResolvedValue([SBIN]);
    renderPanel();
    fireEvent.click(await screen.findByRole('button', { name: 'Add instruments…' }));
    const dialog = await screen.findByRole('dialog');
    fireEvent.change(within(dialog).getByRole('textbox', { name: 'Search instruments' }), { target: { value: 'sb' } });
    fireEvent.click(await within(dialog).findByRole('checkbox', { name: 'Select SBIN' }));
    fireEvent.click(within(dialog).getByRole('button', { name: 'Done' }));
    await waitFor(() => expect(backend.callsTo('watchlists.add')[0]?.args).toEqual(['wl-1', SBIN.token, -1]));
  });
```

- [ ] **Step 4: Run the tests to check they pass**

Run: `cd frontend && npx vitest run src/views/watchlists && npm run typecheck && npm run lint`
Expected: PASS (7 picker tests + 8 panel tests + earlier). Typecheck and lint exit 0.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/views/watchlists/InstrumentPicker.tsx frontend/src/views/watchlists/InstrumentPicker.test.tsx frontend/src/views/watchlists/WatchlistPanel.tsx frontend/src/views/watchlists/WatchlistPanel.test.tsx
git commit -m "$(cat <<'EOF'
feat(watchlists): shared multi-select instrument picker with caller max, save-as-watchlist

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
)"
```

---