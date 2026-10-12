> **P0-5 Chart, Watchlists & Ticker UI Implementation Plan — Task 9.** Read [`../MANIFEST.md`](../MANIFEST.md) first: its Global Constraints,
> Review Focus and Consumes/Produces apply to this task. Other tasks live beside this file in `tasks/`.

<!-- task body (verbatim from the plan) -->
### Task 9: Chart symbol search dialog (FR-CHART-005)

**Files:**
- Create: `frontend/src/chart/ui/SymbolSearchDialog.tsx`, `frontend/src/chart/ui/symbolSearch.css`
- Test: `frontend/src/chart/ui/SymbolSearchDialog.test.tsx`

**Interfaces:**
- Consumes: `searchInstruments`, `EXCHANGE_TAGS`, `resolveIdentifier`, `useInstruments`, `<InstrumentLabel>`, P0-4 `Dialog`.
- Produces: `<SymbolSearchDialog open onClose onSelect(inst: Instrument) />`, `SEARCH_DEBOUNCE_MS = 120`, `QUICK_CHIPS`, `TYPE_TO_SEARCH = 'Type to search'` (an empty query is never sent: `Instruments.Search` returns an empty list for it)

- [ ] **Step 1: Write the failing test**

`frontend/src/chart/ui/SymbolSearchDialog.test.tsx`:

```tsx
import { fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';

vi.mock('../../api/instruments', () => ({ searchInstruments: vi.fn(), instrumentByIdentifier: vi.fn(), instrumentsByTokens: vi.fn(async () => []) }));
import { instrumentByIdentifier, searchInstruments } from '../../api/instruments';
import { SymbolSearchDialog } from './SymbolSearchDialog';
import { BANKNIFTY_OCT_FUT, NIFTYBANK, RELIANCE, RELIANCE_OCT_FUT, SBIN, SENSEX } from '../../instruments/fixtures';
import type { Instrument } from '../../instruments/types';

const setup = () => {
  const onClose = vi.fn(); const onSelect = vi.fn();
  render(<SymbolSearchDialog open onClose={onClose} onSelect={onSelect} />);
  return { onClose, onSelect, input: screen.getByLabelText('Search instruments') as HTMLInputElement };
};

beforeEach(() => {
  vi.clearAllMocks();
  vi.mocked(searchInstruments).mockImplementation(async (q: string) => (q === 'r' ? [RELIANCE, SBIN] : []));
});

describe('SymbolSearchDialog', () => {
  it('AC1: input focused, 4 quick chips, "Type to search" until the user types (no empty-query call)', async () => {
    const { input } = setup();
    await waitFor(() => expect(input).toHaveFocus());
    for (const chip of ['NIFTY 50', 'NIFTY BANK', 'SENSEX', 'BANKEX']) expect(screen.getByRole('button', { name: chip })).toBeInTheDocument();
    expect(input).toHaveAttribute('placeholder', 'Search, e.g. SBIN, TCS');
    expect(screen.getByText('Type to search')).toBeInTheDocument();
    await new Promise((r) => setTimeout(r, 200));
    expect(searchInstruments).not.toHaveBeenCalled();
    fireEvent.change(input, { target: { value: 'r' } });
    expect(await screen.findByText('RELIANCE')).toBeInTheDocument();
    expect(searchInstruments).toHaveBeenCalledWith('r', ['INDICES', 'NSE', 'BSE', 'NFO-FUT', 'BFO-FUT'], 50);
    expect(screen.queryByText('Type to search')).toBeNull();
  });

  it('AC2: BANKNIFTY lists the index and the future', async () => {
    vi.mocked(searchInstruments).mockResolvedValue([NIFTYBANK, BANKNIFTY_OCT_FUT]);
    const { input } = setup();
    fireEvent.change(input, { target: { value: 'BANKNIFTY' } });
    const list = await screen.findByRole('listbox');
    await waitFor(() => expect(within(list).getByText('NIFTY BANK')).toBeInTheDocument());
    expect(within(list).getByText('BANKNIFTY OCT FUT')).toBeInTheDocument();
    expect(within(list).queryByText(/ CE$| PE$/)).toBeNull();
  });

  it('AC3: "reliance oct fut" shows the future with an F badge', async () => {
    vi.mocked(searchInstruments).mockResolvedValue([RELIANCE_OCT_FUT]);
    const { input } = setup();
    fireEvent.change(input, { target: { value: 'reliance oct fut' } });
    const opt = await screen.findByRole('option', { name: /RELIANCE OCT FUT/ });
    expect(within(opt).getByLabelText('Future')).toBeInTheDocument();
  });

  it('AC4: ArrowDown + Enter selects and closes', async () => {
    const { input, onSelect, onClose } = setup();
    fireEvent.change(input, { target: { value: 'r' } });
    await screen.findByText('SBIN');
    fireEvent.keyDown(input, { key: 'ArrowDown' });
    fireEvent.keyDown(input, { key: 'Enter' });
    expect(onSelect).toHaveBeenCalledWith(SBIN);
    expect(onClose).toHaveBeenCalled();
  });

  it('quick chip resolves the index and selects it', async () => {
    vi.mocked(instrumentByIdentifier).mockResolvedValue(SENSEX);
    const { onSelect } = setup();
    fireEvent.click(screen.getByRole('button', { name: 'SENSEX' }));
    await waitFor(() => expect(onSelect).toHaveBeenCalledWith(SENSEX));
    expect(instrumentByIdentifier).toHaveBeenCalledWith('BSE:SENSEX');
  });

  it('AC5: Escape closes without selecting', async () => {
    const { input, onSelect, onClose } = setup();
    fireEvent.keyDown(input, { key: 'Escape' });
    await waitFor(() => expect(onClose).toHaveBeenCalled());
    expect(onSelect).not.toHaveBeenCalled();
  });

  it('a slow earlier query never overwrites a newer one', async () => {
    let slow!: (v: Instrument[]) => void;
    vi.mocked(searchInstruments).mockImplementation((q: string) =>
      q === 'r' ? new Promise((r) => { slow = r; }) : Promise.resolve(q === 'sb' ? [SBIN] : []));
    const { input } = setup();
    fireEvent.change(input, { target: { value: 'r' } });
    await waitFor(() => expect(searchInstruments).toHaveBeenCalledWith('r', expect.anything(), 50));
    fireEvent.change(input, { target: { value: 'sb' } });
    await screen.findByText('SBIN');
    slow([RELIANCE]);
    await new Promise((r) => setTimeout(r, 20));
    expect(screen.queryByText('RELIANCE')).toBeNull();
  });
});
```

- [ ] **Step 2: Run the test to check it fails**

Run: `cd frontend && npx vitest run src/chart/ui/SymbolSearchDialog.test.tsx`
Expected: FAIL with `Failed to resolve import "./SymbolSearchDialog"`.

- [ ] **Step 3: Implement**

`frontend/src/chart/ui/SymbolSearchDialog.tsx`:

```tsx
import { useEffect, useRef, useState, type KeyboardEvent } from 'react';
import { Dialog } from '../../components/Dialog';
import { InstrumentLabel } from '../../components/instrument/InstrumentLabel';
import { searchInstruments } from '../../api/instruments';
import { EXCHANGE_TAGS } from '../../instruments/naming';
import type { Instrument } from '../../instruments/types';
import { resolveIdentifier, useInstruments } from '../../store/instrumentsStore';
import './symbolSearch.css';

export const SEARCH_DEBOUNCE_MS = 120;
export const TYPE_TO_SEARCH = 'Type to search';
export const QUICK_CHIPS = [
  { id: 'NSE:NIFTY 50', label: 'NIFTY 50' },
  { id: 'NSE:NIFTY BANK', label: 'NIFTY BANK' },
  { id: 'BSE:SENSEX', label: 'SENSEX' },
  { id: 'BSE:BANKEX', label: 'BANKEX' },
] as const;

export function SymbolSearchDialog({ open, onClose, onSelect }: { open: boolean; onClose(): void; onSelect(i: Instrument): void }) {
  const [query, setQuery] = useState('');
  const [results, setResults] = useState<Instrument[]>([]);
  const [active, setActive] = useState(0);
  const [error, setError] = useState<string | null>(null);
  const inputRef = useRef<HTMLInputElement>(null);
  const seq = useRef(0);

  useEffect(() => {
    if (!open) return;
    setQuery('');
    setActive(0);
    setError(null);
    const t = setTimeout(() => inputRef.current?.focus(), 0); // after the Dialog's own initial focus
    return () => clearTimeout(t);
  }, [open]);

  useEffect(() => {
    if (!open) return;
    const my = ++seq.current;
    if (query.trim() === '') { setResults([]); setActive(0); return; } // Go answers [] for an empty query
    const t = setTimeout(() => {
      searchInstruments(query.trim(), EXCHANGE_TAGS, 50)
        .then((r) => { if (my === seq.current) { setResults(r); setActive(0); setError(null); } })
        .catch((e: unknown) => { if (my === seq.current) setError(e instanceof Error ? e.message : String(e)); });
    }, SEARCH_DEBOUNCE_MS);
    return () => clearTimeout(t);
  }, [open, query]);

  const choose = (i: Instrument) => {
    useInstruments.getState().put([i]);
    onSelect(i);
    onClose();
  };

  const chooseChip = async (id: string) => {
    try { choose(await resolveIdentifier(id)); } catch (e) { setError(e instanceof Error ? e.message : String(e)); }
  };

  const onKeyDown = (e: KeyboardEvent<HTMLInputElement>) => {
    if (e.key === 'ArrowDown') { e.preventDefault(); setActive((a) => Math.min(a + 1, Math.max(results.length - 1, 0))); }
    else if (e.key === 'ArrowUp') { e.preventDefault(); setActive((a) => Math.max(a - 1, 0)); }
    else if (e.key === 'Enter') { e.preventDefault(); const r = results[active]; if (r) choose(r); }
    else if (e.key === 'Escape') { e.preventDefault(); onClose(); }
  };

  return (
    <Dialog open={open} title="Search instruments" onClose={onClose}>
      <div className="symbol-search">
        <input ref={inputRef} className="symbol-search__input" placeholder="Search, e.g. SBIN, TCS" aria-label="Search instruments"
          role="combobox" aria-expanded="true" aria-controls="symbol-search-results"
          aria-activedescendant={results[active] ? `symbol-opt-${results[active].token}` : undefined}
          value={query} onChange={(e) => setQuery(e.target.value)} onKeyDown={onKeyDown} />
        <div className="symbol-search__chips">
          {QUICK_CHIPS.map((c) => (
            <button key={c.id} type="button" className="symbol-search__chip" onClick={() => void chooseChip(c.id)}>{c.label}</button>
          ))}
        </div>
        {error && <p role="alert" className="symbol-search__error">{error}</p>}
        {query.trim() === '' && <p className="symbol-search__hint">{TYPE_TO_SEARCH}</p>}
        <div className="symbol-search__head" aria-hidden="true"><span>Instrument</span><span>Company name</span></div>
        <div id="symbol-search-results" role="listbox" aria-label="Results" className="symbol-search__results">
          {results.map((r, k) => (
            <div key={r.token} id={`symbol-opt-${r.token}`} role="option" aria-selected={k === active}
              className={`symbol-search__row${k === active ? ' is-active' : ''}`}
              onMouseEnter={() => setActive(k)} onClick={() => choose(r)}>
              <InstrumentLabel instrument={r} />
              <span className="symbol-search__company">{r.name}</span>
            </div>
          ))}
        </div>
      </div>
    </Dialog>
  );
}
```

The input handles Escape itself, so the dialog also closes if P0-4's `Dialog` listens for Escape only on its container. Both handlers call the same `onClose`, which is idempotent.

`frontend/src/chart/ui/symbolSearch.css`:

```css
.symbol-search { display: flex; flex-direction: column; gap: 8px; width: min(640px, 90vw); }
.symbol-search__input { padding: 8px 10px; font-size: 14px; border-radius: 6px; border: 1px solid var(--chart-separator); background: var(--chart-bg); color: var(--chart-text); }
.symbol-search__chips { display: flex; gap: 6px; flex-wrap: wrap; }
.symbol-search__chip { padding: 3px 10px; border-radius: 999px; border: 1px solid var(--chart-separator); background: none; color: inherit; cursor: pointer; }
.symbol-search__head, .symbol-search__row { display: grid; grid-template-columns: 1fr auto; gap: 12px; padding: 6px 8px; }
.symbol-search__head, .symbol-search__hint { font-size: 12px; opacity: 0.75; }
.symbol-search__hint { margin: 0; padding: 4px 8px; }
.symbol-search__results { max-height: 50vh; overflow-y: auto; }
.symbol-search__row { cursor: pointer; border-radius: 4px; }
.symbol-search__row.is-active { outline: 1px solid var(--chart-crosshair); }
.symbol-search__company { text-align: right; text-transform: uppercase; font-size: 12px; opacity: 0.75; }
```

- [ ] **Step 4: Run the test to check it passes**

Run: `cd frontend && npx vitest run src/chart/ui/SymbolSearchDialog.test.tsx`
Expected: PASS (7 tests).

- [ ] **Step 5: Commit**

```bash
git add frontend/src/chart/ui/SymbolSearchDialog.tsx frontend/src/chart/ui/SymbolSearchDialog.test.tsx frontend/src/chart/ui/symbolSearch.css
git commit -m "$(cat <<'EOF'
feat(chart): single-select symbol search dialog with quick chips and keyboard selection

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
)"
```

---
