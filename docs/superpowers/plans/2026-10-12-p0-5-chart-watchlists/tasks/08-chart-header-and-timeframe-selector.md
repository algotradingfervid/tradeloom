> **P0-5 Chart, Watchlists & Ticker UI Implementation Plan — Task 8.** Read [`../MANIFEST.md`](../MANIFEST.md) first: its Global Constraints,
> Review Focus and Consumes/Produces apply to this task. Other tasks live beside this file in `tasks/`.

<!-- task body (verbatim from the plan) -->
### Task 8: Chart header and timeframe selector

**Files:**
- Create: `frontend/src/chart/ui/TimeframeSelect.tsx`, `frontend/src/chart/ui/ChartHeader.tsx`, `frontend/src/chart/ui/chart.css`
- Test: `frontend/src/chart/ui/ChartHeader.test.tsx`

**Interfaces:**
- Consumes: `TIMEFRAMES`, `TimeframeId`, `precisionForTick`, `tickRupees`, `<InstrumentLabel>`, `<PriceChange>`, `useSubscription`, `tick` (tests), P0-4 `IconButton` (icons `back`, `refresh`, `more`), `Icon` (`search`), `useTicksStore`.
- Produces:
  - `<TimeframeSelect value onChange />`
  - `<ChartHeader contextId instrument timeframe refreshing refreshDisabled onOpenSearch onRefresh onTimeframe onOpenFullView? onBack? />`. Subscribes its token as consumer `chart` under source id `chart-header:<contextId>`.

- [ ] **Step 1: Write the failing test**

`frontend/src/chart/ui/ChartHeader.test.tsx`:

```tsx
import { fireEvent, render, screen, within } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';

const useSubscription = vi.fn();
vi.mock('../../market/hooks', () => ({ useSubscription: (...a: unknown[]) => useSubscription(...a), usePolled: () => false }));

import { ChartHeader, type ChartHeaderProps } from './ChartHeader';
import { useTicksStore } from '../../store/ticks';
import { tick } from '../../market/testing';
import { RELIANCE } from '../../instruments/fixtures';

const props = (over: Partial<ChartHeaderProps> = {}): ChartHeaderProps => ({
  contextId: 'dashboard', instrument: RELIANCE, timeframe: '1h', refreshing: false, refreshDisabled: false,
  onOpenSearch: vi.fn(), onRefresh: vi.fn(), onTimeframe: vi.fn(), ...over,
});

beforeEach(() => { useTicksStore.setState({ latest: {} }); useSubscription.mockClear(); });

describe('ChartHeader', () => {
  it('FR-CHART-002 AC1: RELIANCE NSE <LTP> (<pct>%) coloured, LTP live from the chart subscription', () => {
    useTicksStore.getState().applyBatch([tick(RELIANCE.token, 1000, 990)]);
    render(<ChartHeader {...props()} />);
    const symbol = screen.getByRole('button', { name: 'Change symbol' });
    expect(within(symbol).getByText('RELIANCE')).toBeInTheDocument();
    expect(within(symbol).getByText('NSE')).toBeInTheDocument();
    expect(within(symbol).getByText('1,000.00')).toBeInTheDocument();
    expect(within(symbol).getByText('(+1.01%)')).toBeInTheDocument();
    expect(useSubscription).toHaveBeenCalledWith('chart-header:dashboard', 'chart', [RELIANCE.token]);
  });

  it('FR-CHART-002 AC2: every control has an accessible name/tooltip', () => {
    render(<ChartHeader {...props({ onOpenFullView: vi.fn() })} />);
    expect(screen.getByRole('button', { name: 'Change symbol' })).toHaveAttribute('title', 'Search instruments');
    expect(screen.getByRole('button', { name: 'Refresh chart data' })).toBeInTheDocument();
    expect(screen.getByLabelText('Timeframe')).toHaveAttribute('title', 'Timeframe');
    expect(screen.getByRole('button', { name: 'More chart actions' })).toBeInTheDocument();
  });

  it('FR-CHART-004 AC1: exactly 8 timeframes in order; change reports the id', () => {
    const onTimeframe = vi.fn();
    render(<ChartHeader {...props({ onTimeframe })} />);
    const select = screen.getByLabelText('Timeframe') as HTMLSelectElement;
    expect([...select.options].map((o) => o.text)).toEqual(['1 Minute', '3 Minutes', '5 Minutes', '10 Minutes', '15 Minutes', '30 Minutes', '1 Hour', '1 Day']);
    expect(select.value).toBe('1h');
    fireEvent.change(select, { target: { value: '1d' } });
    expect(onTimeframe).toHaveBeenCalledWith('1d');
  });

  it('FR-CHART-006 AC2: refresh is disabled and marked spinning while in progress', () => {
    const onRefresh = vi.fn();
    render(<ChartHeader {...props({ refreshing: true, onRefresh })} />);
    const btn = screen.getByRole('button', { name: 'Refresh chart data' });
    expect(btn).toBeDisabled();
    expect(btn.className).toContain('is-spinning');
    fireEvent.click(btn);
    expect(onRefresh).not.toHaveBeenCalled();
  });

  it('FR-CHART-017: ⋯ menu offers "Open in full view"', () => {
    const onOpenFullView = vi.fn();
    render(<ChartHeader {...props({ onOpenFullView })} />);
    fireEvent.click(screen.getByRole('button', { name: 'More chart actions' }));
    fireEvent.click(screen.getByRole('menuitem', { name: 'Open in full view' }));
    expect(onOpenFullView).toHaveBeenCalledTimes(1);
    expect(screen.queryByRole('menu')).toBeNull();
  });
});
```

- [ ] **Step 2: Run the test to check it fails**

Run: `cd frontend && npx vitest run src/chart/ui/ChartHeader.test.tsx`
Expected: FAIL with `Failed to resolve import "./ChartHeader"`.

- [ ] **Step 3: Implement**

`frontend/src/chart/ui/TimeframeSelect.tsx`:

```tsx
import { TIMEFRAMES, type TimeframeId } from '../timeframes';

/** Native select: arrows/Enter work out of the box (FR-CHART-004 keyboard). */
export function TimeframeSelect({ value, onChange }: { value: TimeframeId; onChange(tf: TimeframeId): void }) {
  return (
    <select className="chart-header__tf" aria-label="Timeframe" title="Timeframe" value={value}
      onChange={(e) => onChange(e.target.value as TimeframeId)}>
      {TIMEFRAMES.map((t) => <option key={t.id} value={t.id}>{t.label}</option>)}
    </select>
  );
}
```

`frontend/src/chart/ui/ChartHeader.tsx`:

```tsx
import { useState } from 'react';
import { Icon } from '../../components/Icon';
import { IconButton } from '../../components/IconButton';
import { InstrumentLabel } from '../../components/instrument/InstrumentLabel';
import { PriceChange } from '../../components/instrument/PriceChange';
import { tickRupees } from '../../instruments/naming';
import type { Instrument } from '../../instruments/types';
import { useSubscription } from '../../market/hooks';
import { precisionForTick } from '../format';
import type { TimeframeId } from '../timeframes';
import { TimeframeSelect } from './TimeframeSelect';
import './chart.css';

export interface ChartHeaderProps {
  contextId: string;
  instrument: Instrument | undefined;
  timeframe: TimeframeId;
  refreshing: boolean;
  refreshDisabled: boolean;
  onOpenSearch(): void;
  onRefresh(): void;
  onTimeframe(tf: TimeframeId): void;
  onOpenFullView?: () => void;
  onBack?: () => void;
}

export function ChartHeader(p: ChartHeaderProps) {
  useSubscription(`chart-header:${p.contextId}`, 'chart', p.instrument ? [p.instrument.token] : []);
  const [menuOpen, setMenuOpen] = useState(false);
  return (
    <div className="chart-header" role="toolbar" aria-label="Chart controls">
      {p.onBack && <IconButton label="Back" icon="back" onClick={p.onBack} />}
      <button type="button" className="chart-header__symbol" title="Search instruments" aria-label="Change symbol" onClick={p.onOpenSearch}>
        <span aria-hidden="true" className="chart-header__search-icon"><Icon name="search" size={16} /></span>
        {p.instrument ? (
          <>
            <InstrumentLabel instrument={p.instrument} />
            <PriceChange token={p.instrument.token} decimals={precisionForTick(tickRupees(p.instrument))} />
          </>
        ) : <span>—</span>}
      </button>
      <IconButton label="Refresh chart data" icon="refresh" className={p.refreshing ? 'is-spinning' : undefined}
        disabled={p.refreshing || p.refreshDisabled} onClick={p.onRefresh} />
      <TimeframeSelect value={p.timeframe} onChange={p.onTimeframe} />
      {p.onOpenFullView && (
        <div className="chart-header__menu">
          <IconButton label="More chart actions" icon="more" aria-expanded={menuOpen} onClick={() => setMenuOpen((v) => !v)} />
          {menuOpen && (
            <ul role="menu" className="chart-header__menu-list" onKeyDown={(e) => { if (e.key === 'Escape') setMenuOpen(false); }}>
              <li role="none">
                <button type="button" role="menuitem" autoFocus onClick={() => { setMenuOpen(false); p.onOpenFullView?.(); }}>
                  Open in full view
                </button>
              </li>
            </ul>
          )}
        </div>
      )}
    </div>
  );
}
```

`frontend/src/chart/ui/chart.css`:

```css
.chart-view { display: flex; flex-direction: column; min-width: 0; height: 100%; min-height: 360px; background: var(--chart-bg); }
.chart-header { display: flex; align-items: center; gap: 8px; padding: 6px 8px; border-bottom: 1px solid var(--chart-separator); flex-wrap: nowrap; min-width: 0; }
.chart-header__symbol { display: inline-flex; align-items: center; gap: 8px; min-width: 0; background: none; border: 1px solid transparent; border-radius: 6px; padding: 4px 8px; color: inherit; cursor: pointer; }
.chart-header__symbol:hover, .chart-header__symbol:focus-visible { border-color: var(--chart-separator); }
.chart-header__tf { padding: 4px 6px; border-radius: 6px; border: 1px solid var(--chart-separator); background: var(--chart-bg); color: var(--chart-text); }
.chart-header__menu { position: relative; margin-left: auto; }
.chart-header__menu-list { position: absolute; right: 0; top: 100%; z-index: 10; margin: 0; padding: 4px; list-style: none; background: var(--chart-bg); border: 1px solid var(--chart-separator); border-radius: 6px; }
.chart-header__menu-list button { white-space: nowrap; background: none; border: 0; color: var(--chart-text); padding: 6px 10px; cursor: pointer; }
.is-spinning svg, .is-spinning > span { animation: chart-spin 0.9s linear infinite; }
@keyframes chart-spin { to { transform: rotate(360deg); } }
@media (prefers-reduced-motion: reduce) { .is-spinning svg, .is-spinning > span { animation: none; } }
.chart-view__body { position: relative; flex: 1; min-height: 0; }
.chart-view__canvas { position: absolute; inset: 0; outline: none; }
.chart-view__canvas:focus-visible { box-shadow: inset 0 0 0 2px var(--chart-crosshair); }
.chart-view__overlay { position: absolute; inset: 0; display: grid; place-items: center; gap: 8px; color: var(--chart-text); pointer-events: none; }
.chart-view__overlay > * { pointer-events: auto; }
.chart-view__badge { position: absolute; top: 34px; left: 8px; font-size: 12px; padding: 2px 6px; border-radius: 4px; border: 1px solid var(--chart-separator); color: var(--chart-text); }
.chart-legend { position: absolute; top: 6px; left: 8px; z-index: 2; display: flex; gap: 8px; font-size: 12px; font-variant-numeric: tabular-nums; color: var(--chart-text); pointer-events: none; }
.chart-legend--up b { color: var(--up); }
.chart-legend--down b { color: var(--down); }
.chart-scale-toggles { position: absolute; right: 4px; bottom: 28px; z-index: 2; display: flex; gap: 2px; }
.chart-scale-toggles button { min-width: 22px; font-size: 11px; border: 1px solid var(--chart-separator); background: var(--chart-bg); color: var(--chart-text); border-radius: 4px; cursor: pointer; }
.chart-scale-toggles button[aria-pressed="true"] { border-color: var(--chart-crosshair); font-weight: 700; }
```

- [ ] **Step 4: Run the test to check it passes**

Run: `cd frontend && npx vitest run src/chart/ui/ChartHeader.test.tsx`
Expected: PASS (5 tests).

- [ ] **Step 5: Commit**

```bash
git add frontend/src/chart/ui
git commit -m "$(cat <<'EOF'
feat(chart): header with live LTP, refresh, timeframe selector and full-view menu

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
)"
```

---
