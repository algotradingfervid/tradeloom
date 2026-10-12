> **P0-5 Chart, Watchlists & Ticker UI Implementation Plan — Task 7.** Read [`../MANIFEST.md`](../MANIFEST.md) first: its Global Constraints,
> Review Focus and Consumes/Produces apply to this task. Other tasks live beside this file in `tasks/`.

<!-- task body (verbatim from the plan) -->
### Task 7: Instrument store and shared instrument UI (`InstrumentLabel`, `PriceChange`)

**Files:**
- Create: `frontend/src/store/instrumentsStore.ts`, `frontend/src/components/instrument/InstrumentLabel.tsx`, `frontend/src/components/instrument/PriceChange.tsx`, `frontend/src/components/instrument/instrument.css`
- Test: `frontend/src/store/instrumentsStore.test.ts`, `frontend/src/components/instrument/instrument.test.tsx`

**Interfaces:**
- Consumes: Task 2 `instrumentsByTokens`, `instrumentByIdentifier`; Task 1 naming (`LabelInstrument`); Task 6 `useQuote`, `quoteChange`, `tick` (tests). P0-4 `formatPrice`, `formatSigned`, `formatPercent`, `useTicksStore`.
- Produces:
  - `useInstruments` (zustand: `{ byToken; byId; put(list) }`), `ensureInstruments(tokens): Promise<void>`, `resolveIdentifier(id): Promise<Instrument>`, `useInstrument(token?): Instrument | undefined`, `useResolvedInstrument(id?): { instrument?: Instrument; error?: string }`
  - `<InstrumentLabel instrument showName? />` where `instrument: LabelInstrument` (a full `Instrument` or `labelOfItem(watchlistItem)`). Renders `data-instrument-token`, `data-exchange-tag`, `data-instrument-name` and the `F` badge for futures.
  - `<PriceChange token decimals? showAbsolute? />`. Renders `—` without a quote. `data-direction="up|down"`.

- [ ] **Step 1: Write the failing tests**

`frontend/src/store/instrumentsStore.test.ts`:

```ts
import { beforeEach, describe, expect, it, vi } from 'vitest';

vi.mock('../api/instruments', () => ({ instrumentsByTokens: vi.fn(), instrumentByIdentifier: vi.fn() }));
import { instrumentByIdentifier, instrumentsByTokens } from '../api/instruments';
import { ensureInstruments, resolveIdentifier, useInstruments } from './instrumentsStore';
import { NIFTY50, NIFTY_OPTION, RELIANCE, SBIN } from '../instruments/fixtures';

beforeEach(() => {
  useInstruments.setState({ byToken: {}, byId: {} });
  vi.clearAllMocks();
});

describe('instrumentsStore', () => {
  it('dedupes concurrent loads of the same tokens', async () => {
    vi.mocked(instrumentsByTokens).mockResolvedValue([RELIANCE, SBIN]);
    await Promise.all([ensureInstruments([RELIANCE.token, SBIN.token]), ensureInstruments([SBIN.token])]);
    expect(instrumentsByTokens).toHaveBeenCalledTimes(1);
    expect(useInstruments.getState().byToken[SBIN.token]).toEqual(SBIN);
  });
  it('NFR-15: put() never stores an out-of-scope instrument', () => {
    useInstruments.getState().put([NIFTY_OPTION, RELIANCE]);
    expect(useInstruments.getState().byToken[NIFTY_OPTION.token]).toBeUndefined();
  });
  it('resolveIdentifier caches by EXCHANGE:SYMBOL', async () => {
    vi.mocked(instrumentByIdentifier).mockResolvedValue(NIFTY50);
    expect(await resolveIdentifier('NSE:NIFTY 50')).toEqual(NIFTY50);
    expect(await resolveIdentifier('NSE:NIFTY 50')).toEqual(NIFTY50);
    expect(instrumentByIdentifier).toHaveBeenCalledTimes(1);
  });
});
```

`frontend/src/components/instrument/instrument.test.tsx`:

```tsx
import { render, screen } from '@testing-library/react';
import { beforeEach, describe, expect, it } from 'vitest';
import { InstrumentLabel } from './InstrumentLabel';
import { PriceChange } from './PriceChange';
import { useTicksStore } from '../../store/ticks';
import { tick } from '../../market/testing';
import { NIFTY50, RELIANCE, RELIANCE_OCT_FUT, SBIN } from '../../instruments/fixtures';

beforeEach(() => useTicksStore.setState({ latest: {} }));

describe('InstrumentLabel', () => {
  it('FR-WATCH-013 AC1: future shows "RELIANCE OCT FUT", an F badge and tag NFO-FUT', () => {
    render(<InstrumentLabel instrument={RELIANCE_OCT_FUT} />);
    expect(screen.getByText('RELIANCE OCT FUT')).toHaveAttribute('data-instrument-name');
    expect(screen.getByLabelText('Future')).toHaveTextContent('F');
    expect(screen.getByText('NFO-FUT')).toBeInTheDocument();
  });
  it('FR-WATCH-013 AC2: NIFTY 50 tagged INDICES, no F badge; company name optional', () => {
    const { container } = render(<InstrumentLabel instrument={NIFTY50} />);
    expect(container.firstElementChild).toHaveAttribute('data-exchange-tag', 'INDICES');
    expect(screen.queryByLabelText('Future')).toBeNull();
    render(<InstrumentLabel instrument={RELIANCE} showName />);
    expect(screen.getByText('RELIANCE INDUSTRIES')).toBeInTheDocument();
  });
});

describe('PriceChange', () => {
  it('shows — when no quote is known (never 0.00)', () => {
    render(<PriceChange token={SBIN.token} />);
    expect(screen.getByText('—')).toBeInTheDocument();
    expect(screen.queryByText(/0\.00/)).toBeNull();
  });
  it('FR-WATCH-005 AC4: 1,000 vs 1,010 renders (-0.99%) in the down style', () => {
    useTicksStore.getState().applyBatch([tick(SBIN.token, 1000, 1010)]);
    const { container } = render(<PriceChange token={SBIN.token} />);
    expect(screen.getByText('1,000.00')).toBeInTheDocument();
    expect(screen.getByText('(-0.99%)')).toBeInTheDocument();
    expect(container.firstElementChild).toHaveAttribute('data-direction', 'down');
  });
  it('absolute change when asked (ticker pills, wide)', () => {
    useTicksStore.getState().applyBatch([tick(NIFTY50.token, 22600, 22500)]);
    render(<PriceChange token={NIFTY50.token} showAbsolute />);
    expect(screen.getByText('+100.00 (+0.44%)')).toBeInTheDocument();
  });
});
```

- [ ] **Step 2: Run the tests to check they fail**

Run: `cd frontend && npx vitest run src/store/instrumentsStore.test.ts src/components/instrument`
Expected: FAIL with `Failed to resolve import "./instrumentsStore"` / `"./InstrumentLabel"`.

- [ ] **Step 3: Implement**

`frontend/src/store/instrumentsStore.ts`:

```ts
import { useEffect, useState } from 'react';
import { create } from 'zustand';
import type { Instrument } from '../instruments/types';
import { instrumentByIdentifier, instrumentsByTokens } from '../api/instruments';
import { instrumentId, isInScope } from '../instruments/naming';

interface InstrumentsState {
  byToken: Record<number, Instrument>;
  byId: Record<string, number>;
  put(list: readonly Instrument[]): void;
}

export const useInstruments = create<InstrumentsState>()((set) => ({
  byToken: {},
  byId: {},
  put: (list) => set((s) => {
    const byToken = { ...s.byToken };
    const byId = { ...s.byId };
    for (const i of list) {
      if (!isInScope(i)) continue;
      byToken[i.token] = i;
      byId[instrumentId(i)] = i.token;
    }
    return { byToken, byId };
  }),
}));

const inflight = new Map<number, Promise<void>>();

export async function ensureInstruments(tokens: readonly number[]): Promise<void> {
  const { byToken } = useInstruments.getState();
  const waits = tokens.map((t) => inflight.get(t)).filter((p): p is Promise<void> => p !== undefined);
  const missing = [...new Set(tokens)].filter((t) => !byToken[t] && !inflight.has(t));
  if (missing.length > 0) {
    const p = instrumentsByTokens(missing)
      .then((list) => useInstruments.getState().put(list))
      .finally(() => missing.forEach((t) => inflight.delete(t)));
    missing.forEach((t) => inflight.set(t, p));
    waits.push(p);
  }
  await Promise.all(waits);
}

export async function resolveIdentifier(id: string): Promise<Instrument> {
  const s = useInstruments.getState();
  const token = s.byId[id];
  if (token !== undefined && s.byToken[token]) return s.byToken[token];
  const inst = await instrumentByIdentifier(id);
  useInstruments.getState().put([inst]);
  return inst;
}

export function useInstrument(token: number | undefined): Instrument | undefined {
  const inst = useInstruments((s) => (token === undefined ? undefined : s.byToken[token]));
  useEffect(() => {
    if (token !== undefined && !inst) void ensureInstruments([token]).catch(() => undefined);
  }, [token, inst]);
  return inst;
}

export function useResolvedInstrument(id: string | undefined): { instrument?: Instrument; error?: string } {
  const [state, setState] = useState<{ id?: string; instrument?: Instrument; error?: string }>({});
  useEffect(() => {
    if (!id) return;
    let alive = true;
    resolveIdentifier(id)
      .then((instrument) => { if (alive) setState({ id, instrument }); })
      .catch((e: unknown) => { if (alive) setState({ id, error: e instanceof Error ? e.message : String(e) }); });
    return () => { alive = false; };
  }, [id]);
  return state.id === id ? { instrument: state.instrument, error: state.error } : {};
}
```

`frontend/src/components/instrument/InstrumentLabel.tsx`:

```tsx
import { displayName, exchangeTag, isFuture, type LabelInstrument } from '../../instruments/naming';
import './instrument.css';

/** FR-WATCH-013/014: the only component that renders an instrument's name and exchange tag. */
export function InstrumentLabel({ instrument, showName = false }: { instrument: LabelInstrument; showName?: boolean }) {
  const tag = exchangeTag(instrument);
  return (
    <span className="instrument-label" data-instrument-token={instrument.token} data-exchange-tag={tag}>
      <span className="instrument-label__symbol" data-instrument-name>{displayName(instrument)}</span>
      {isFuture(instrument) && <span className="instrument-label__badge" aria-label="Future">F</span>}
      <span className="instrument-label__tag">{tag}</span>
      {showName && <span className="instrument-label__company">{instrument.name}</span>}
    </span>
  );
}
```

`frontend/src/components/instrument/PriceChange.tsx`:

```tsx
import { formatPercent, formatPrice, formatSigned } from '../../i18n/format';
import { quoteChange, useQuote } from '../../market/quotes';
import './instrument.css';

export function PriceChange({ token, decimals = 2, showAbsolute = false }: { token: number; decimals?: number; showAbsolute?: boolean }) {
  const q = useQuote(token);
  if (!q) return <span className="price-change price-change--empty">—</span>;
  const ch = quoteChange(q);
  const dir = ch && ch.abs < 0 ? 'down' : 'up';
  return (
    <span className={`price-change price-change--${dir}`} data-direction={dir}>
      <span className="price-change__ltp">{formatPrice(q.ltp, decimals)}</span>
      {ch && (
        <span className="price-change__delta">
          {showAbsolute ? `${formatSigned(ch.abs, decimals)} ` : ''}({formatPercent(ch.pct)})
        </span>
      )}
    </span>
  );
}
```

`frontend/src/components/instrument/instrument.css`:

```css
.instrument-label { display: inline-flex; align-items: baseline; gap: 6px; min-width: 0; }
.instrument-label__symbol { font-weight: 600; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.instrument-label__badge {
  display: inline-grid; place-items: center; width: 16px; height: 16px; border-radius: 50%;
  border: 1px solid currentColor; font-size: 10px; font-weight: 700;
}
.instrument-label__tag { font-size: 11px; opacity: 0.75; }
.instrument-label__company { margin-left: auto; font-size: 12px; opacity: 0.75; text-transform: uppercase; white-space: nowrap; }
.price-change { display: inline-flex; gap: 6px; font-variant-numeric: tabular-nums; white-space: nowrap; }
.price-change--up .price-change__delta { color: var(--up); }
.price-change--down .price-change__delta { color: var(--down); }
```

- [ ] **Step 4: Run the tests to check they pass**

Run: `cd frontend && npx vitest run src/store/instrumentsStore.test.ts src/components/instrument`
Expected: PASS (3 + 5 tests).

- [ ] **Step 5: Commit**

```bash
git add frontend/src/store/instrumentsStore.ts frontend/src/store/instrumentsStore.test.ts frontend/src/components/instrument
git commit -m "$(cat <<'EOF'
feat(frontend): instrument store, InstrumentLabel and PriceChange (single naming/tag formatter)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
)"
```

---
