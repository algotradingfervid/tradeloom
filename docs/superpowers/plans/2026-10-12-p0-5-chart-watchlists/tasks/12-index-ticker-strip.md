> **P0-5 Chart, Watchlists & Ticker UI Implementation Plan — Task 12.** Read [`../MANIFEST.md`](../MANIFEST.md) first: its Global Constraints,
> Review Focus and Consumes/Produces apply to this task. Other tasks live beside this file in `tasks/`.

<!-- task body (verbatim from the plan) -->
### Task 12: Index ticker strip (FR-SHELL-004)

**Files:**
- Create: `frontend/src/app/IndexTickerStrip.tsx`, `frontend/src/app/tickerStrip.css`
- Test: `frontend/src/app/IndexTickerStrip.test.tsx`
- Modify: `frontend/src/app/ShellLayout.tsx` (P0-4: pass `ticker={<IndexTickerStrip />}` to `TopBar` instead of `ticker={null}`; nothing else in that file changes)

**Interfaces:**
- Consumes: `resolveIdentifier`, `useSubscription` (consumer `widget`), `useQuote`, `quoteChange`, `tick` (tests), `displayName`, `showOnDashboard`, P0-4 `formatPrice` / `formatSigned` / `formatPercent`, `useTicksStore`, `TopBar({ ticker })`, react-router `useNavigate`.
- Produces: `<IndexTickerStrip />`, `INDEX_PILLS`, `NARROW_BELOW_PX = 1440`

- [ ] **Step 1: Write the failing test**

`frontend/src/app/IndexTickerStrip.test.tsx`:

```tsx
import { act, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';

const nav = vi.hoisted(() => ({ navigate: vi.fn() }));
vi.mock('react-router-dom', () => ({ useNavigate: () => nav.navigate }));
const useSubscription = vi.fn();
vi.mock('../market/hooks', () => ({ useSubscription: (...a: unknown[]) => useSubscription(...a), usePolled: () => false }));
const showOnDashboard = vi.fn();
vi.mock('../store/chartContextsStore', () => ({ showOnDashboard: (id: string) => showOnDashboard(id) }));
vi.mock('../api/instruments', async () => {
  const f = await import('../instruments/fixtures');
  return { instrumentByIdentifier: vi.fn(async (id: string) => f.ALL_IN_SCOPE.find((i) => `${i.exchange}:${i.tradingsymbol}` === id)), instrumentsByTokens: vi.fn(async () => []) };
});

import { IndexTickerStrip } from './IndexTickerStrip';
import { useTicksStore } from '../store/ticks';
import { tick } from '../market/testing';
import { NIFTY50, NIFTYBANK, SENSEX } from '../instruments/fixtures';

const setWidth = (w: number) => { Object.defineProperty(window, 'innerWidth', { configurable: true, value: w }); window.dispatchEvent(new Event('resize')); };
const pill = (name: string) => screen.getByRole('button', { name: new RegExp(`^${name}`) });

beforeEach(() => { useTicksStore.setState({ latest: {} }); vi.clearAllMocks(); setWidth(1600); });

describe('IndexTickerStrip', () => {
  it('subscribes the three indices as a widget consumer', async () => {
    render(<IndexTickerStrip />);
    await waitFor(() => expect(useSubscription).toHaveBeenLastCalledWith('index-ticker-strip', 'widget', [NIFTY50.token, NIFTYBANK.token, SENSEX.token]));
  });

  it('AC4: no data → each pill shows — and never 0.00', async () => {
    render(<IndexTickerStrip />);
    await screen.findByText('SENSEX');
    expect(screen.getAllByText('—')).toHaveLength(3);
    expect(screen.queryByText(/0\.00/)).toBeNull();
  });

  it('AC1+AC2: a tick updates the pill to 22,600.00 ▲ +100.00 (+0.44%) in the up style', async () => {
    render(<IndexTickerStrip />);
    await screen.findByText('NIFTY 50');
    act(() => useTicksStore.getState().applyBatch([tick(NIFTY50.token, 22600, 22500)]));
    const p = pill('NIFTY 50');
    expect(p).toHaveTextContent('22,600.00');
    expect(p).toHaveTextContent('▲+100.00 (+0.44%)');
    expect(p.className).toContain('ticker-pill--up');
  });

  it('narrower than 1440 px drops the absolute change', async () => {
    setWidth(1300);
    render(<IndexTickerStrip />);
    await screen.findByText('NIFTY BANK');
    act(() => useTicksStore.getState().applyBatch([tick(NIFTYBANK.token, 47000, 48000)]));
    const p = pill('NIFTY BANK');
    expect(p).toHaveTextContent('▼(-2.08%)');
    expect(p).not.toHaveTextContent('-1,000.00');
    expect(p.className).toContain('ticker-pill--down');
  });

  it('AC3: clicking SENSEX loads it on the dashboard chart and navigates Home', async () => {
    render(<IndexTickerStrip />);
    fireEvent.click(await screen.findByRole('button', { name: /^SENSEX/ }));
    expect(showOnDashboard).toHaveBeenCalledWith('BSE:SENSEX');
    expect(nav.navigate).toHaveBeenCalledWith('/home');
  });

  it('AC5: outside market hours the last quote from the Watch snapshot (in-memory, this session) is shown', async () => {
    useTicksStore.getState().applyBatch([tick(SENSEX.token, 74100.25, 74000)]);
    render(<IndexTickerStrip />);
    await waitFor(() => expect(pill('SENSEX')).toHaveTextContent('74,100.25'));
  });
});
```

- [ ] **Step 2: Run the test to check it fails**

Run: `cd frontend && npx vitest run src/app/IndexTickerStrip.test.tsx`
Expected: FAIL with `Failed to resolve import "./IndexTickerStrip"`.

- [ ] **Step 3: Implement**

`frontend/src/app/IndexTickerStrip.tsx`:

```tsx
import { useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { formatPercent, formatPrice, formatSigned } from '../i18n/format';
import type { Instrument } from '../instruments/types';
import { displayName } from '../instruments/naming';
import { useSubscription } from '../market/hooks';
import { quoteChange, useQuote } from '../market/quotes';
import { resolveIdentifier } from '../store/instrumentsStore';
import { showOnDashboard } from '../store/chartContextsStore';
import './tickerStrip.css';

export const INDEX_PILLS = ['NSE:NIFTY 50', 'NSE:NIFTY BANK', 'BSE:SENSEX'] as const;
export const NARROW_BELOW_PX = 1440;

function useWideWindow(): boolean {
  const [wide, setWide] = useState(() => window.innerWidth >= NARROW_BELOW_PX);
  useEffect(() => {
    const on = () => setWide(window.innerWidth >= NARROW_BELOW_PX);
    window.addEventListener('resize', on);
    return () => window.removeEventListener('resize', on);
  }, []);
  return wide;
}

export function IndexTickerStrip() {
  const [instruments, setInstruments] = useState<(Instrument | null)[]>([null, null, null]);
  const navigate = useNavigate();
  const wide = useWideWindow();

  useEffect(() => {
    let alive = true;
    void Promise.all(INDEX_PILLS.map((id) => resolveIdentifier(id).catch(() => null))).then((list) => { if (alive) setInstruments(list); });
    return () => { alive = false; };
  }, []);

  useSubscription('index-ticker-strip', 'widget', instruments.flatMap((i) => (i ? [i.token] : [])));

  return (
    <div className="ticker-strip" role="group" aria-label="Index ticker">
      {INDEX_PILLS.map((id, k) => (
        <TickerPill key={id} identifier={id} instrument={instruments[k]} wide={wide}
          onOpen={() => { showOnDashboard(id); navigate('/home'); }} />
      ))}
    </div>
  );
}

function TickerPill({ identifier, instrument, wide, onOpen }: { identifier: string; instrument: Instrument | null; wide: boolean; onOpen(): void }) {
  const q = useQuote(instrument?.token);
  const ch = quoteChange(q);
  const dir = ch && ch.abs < 0 ? 'down' : 'up';
  const label = instrument ? displayName(instrument) : identifier.slice(identifier.indexOf(':') + 1);
  return (
    <button type="button" className={`ticker-pill ticker-pill--${q ? dir : 'empty'}`} onClick={onOpen} title={`Show ${label} on the Home chart`}>
      <span className="ticker-pill__name" {...(instrument ? { 'data-instrument-name': '', 'data-instrument-token': instrument.token } : {})}>{label}</span>
      <span className="ticker-pill__ltp">{q ? formatPrice(q.ltp) : '—'}</span>
      {q && ch && (
        <span className="ticker-pill__change">
          <span aria-hidden="true">{dir === 'up' ? '▲' : '▼'}</span>
          {wide ? `${formatSigned(ch.abs)} ` : ''}({formatPercent(ch.pct)})
        </span>
      )}
    </button>
  );
}
```

The pill label falls back to the symbol part of the hard-coded identifier only until `resolveIdentifier` returns. The resolved name always comes from `displayName`. Only the resolved form carries `data-instrument-name`, which the FR-WATCH-013 AC3 check scans.

`frontend/src/app/tickerStrip.css`:

```css
.ticker-strip { display: flex; gap: 6px; min-width: 0; flex-shrink: 1; }
.ticker-pill { display: inline-flex; align-items: baseline; gap: 6px; padding: 3px 8px; border-radius: 999px; border: 1px solid var(--border); background: none; color: inherit; font-size: 12px; white-space: nowrap; cursor: pointer; font-variant-numeric: tabular-nums; }
.ticker-pill__name { font-weight: 600; }
.ticker-pill--up .ticker-pill__change { color: var(--up); }
.ticker-pill--down .ticker-pill__change { color: var(--down); }
```

In `frontend/src/app/ShellLayout.tsx` (P0-4), add `import { IndexTickerStrip } from './IndexTickerStrip';` and change the `TopBar` element's `ticker={null}` to `ticker={<IndexTickerStrip />}`. `TopBar` already renders its `ticker` prop inside `<div className="topbar__ticker" data-testid="topbar-ticker">`; `TopBar.tsx` itself is not edited.

- [ ] **Step 4: Run the test to check it passes**

Run: `cd frontend && npx vitest run src/app/IndexTickerStrip.test.tsx && npm run typecheck`
Expected: PASS (6 tests). Typecheck exits 0.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/app/IndexTickerStrip.tsx frontend/src/app/IndexTickerStrip.test.tsx frontend/src/app/tickerStrip.css frontend/src/app/ShellLayout.tsx
git commit -m "$(cat <<'EOF'
feat(shell): clickable index ticker strip with live LTP/change and narrow-window format

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
)"
```

---
