> **P0-5 Chart, Watchlists & Ticker UI Implementation Plan — Task 1.** Read [`../MANIFEST.md`](../MANIFEST.md) first: its Global Constraints,
> Review Focus and Consumes/Produces apply to this task. Other tasks live beside this file in `tasks/`.

<!-- task body (verbatim from the plan) -->
### Task 1: Timeframes, instrument model, naming and scope guard

Pure TypeScript with no React and no Wails. Everything later depends on these names.

**Files:**
- Create: `frontend/src/chart/timeframes.ts`, `frontend/src/chart/timeframes.test.ts`
- Create: `frontend/src/instruments/types.ts`, `frontend/src/instruments/naming.ts`, `frontend/src/instruments/naming.test.ts`, `frontend/src/instruments/fixtures.ts`

**Interfaces:**
- Consumes: nothing.
- Produces:
  - `type TimeframeId = '1m'|'3m'|'5m'|'10m'|'15m'|'30m'|'1h'|'1d'`; `type KiteInterval = 'minute'|'3minute'|'5minute'|'10minute'|'15minute'|'30minute'|'60minute'|'day'`
  - `interface Timeframe { id: TimeframeId; label: string; kite: KiteInterval; intraday: boolean; initialTradingDays: number }`
  - `TIMEFRAMES: readonly Timeframe[]`, `SHIPPED_DEFAULT_TIMEFRAME: TimeframeId`, `timeframeById(id: string): Timeframe`, `timeframeFromKite(k: string): Timeframe | undefined`, `parseTimeframe(v: unknown): TimeframeId`
  - `interface Instrument` = the canonical `service.InstrumentDTO` field set: `{ token; exchange; segment; tag; tradingsymbol; name; displayName; instrumentType; expiryMs /*0 = none*/; tickSizePaise; lotSize; isIndex; isFuture; underlying }`
  - `EXCHANGE_TAGS`, `type ExchangeTag`, `isInScope(i): boolean` and `exchangeTag(i): ExchangeTag` (both read `tag`), `isFuture(i): boolean`, `instrumentId(i): string`, `displayName(i): string`, `tickRupees(i): number`, `OutOfScopeError`
  - Watchlist-item helpers (structural, no Backend import): `itemTag(item: { exchange; isIndex }): ExchangeTag`, `itemIdentifier(item: { exchange; symbol }): string`, `labelOfItem(item): LabelInstrument`, `type LabelInstrument = Pick<Instrument, 'token' | 'tag' | 'displayName' | 'tradingsymbol' | 'name'>`
  - Fixtures: `NIFTY50, NIFTYBANK, SENSEX, BANKEX, RELIANCE, RELIANCE_BSE, SBIN, TCS, INFY, ADANIENT, RELIANCE_OCT_FUT, NIFTY_OCT_FUT, BANKNIFTY_OCT_FUT, SENSEX_OCT_FUT, NEWCO, NIFTY_OPTION, ALL_IN_SCOPE`

- [ ] **Step 1: Write the failing tests**

`frontend/src/chart/timeframes.test.ts`:

```ts
import { describe, expect, it } from 'vitest';
import { TIMEFRAMES, SHIPPED_DEFAULT_TIMEFRAME, parseTimeframe, timeframeById, timeframeFromKite } from './timeframes';

describe('timeframes', () => {
  it('lists exactly the 8 FR-CHART-004 options in order with their Kite intervals', () => {
    expect(TIMEFRAMES.map((t) => t.label)).toEqual([
      '1 Minute', '3 Minutes', '5 Minutes', '10 Minutes', '15 Minutes', '30 Minutes', '1 Hour', '1 Day',
    ]);
    expect(TIMEFRAMES.map((t) => t.kite)).toEqual([
      'minute', '3minute', '5minute', '10minute', '15minute', '30minute', '60minute', 'day',
    ]);
  });

  it('uses the FR-CHART-003 initial windows', () => {
    expect(TIMEFRAMES.map((t) => [t.id, t.initialTradingDays])).toEqual([
      ['1m', 5], ['3m', 15], ['5m', 15], ['10m', 30], ['15m', 30], ['30m', 120], ['1h', 120], ['1d', 750],
    ]);
  });

  it('ships 1 Hour as default and parses ids, kite intervals and junk', () => {
    expect(SHIPPED_DEFAULT_TIMEFRAME).toBe('1h');
    expect(parseTimeframe('60minute')).toBe('1h');
    expect(parseTimeframe('15m')).toBe('15m');
    expect(parseTimeframe('day')).toBe('1d');
    expect(parseTimeframe('weekly')).toBe('1h');
    expect(parseTimeframe(undefined)).toBe('1h');
    expect(timeframeFromKite('3minute')?.id).toBe('3m');
    expect(() => timeframeById('2h')).toThrow('unknown timeframe 2h');
  });
});
```

`frontend/src/instruments/naming.test.ts`:

```ts
import { describe, expect, it } from 'vitest';
import {
  displayName, exchangeTag, instrumentId, isFuture, isInScope, itemIdentifier, itemTag, labelOfItem, OutOfScopeError, tickRupees,
} from './naming';
import { ALL_IN_SCOPE, NIFTY50, NIFTY_OPTION, RELIANCE, RELIANCE_BSE, RELIANCE_OCT_FUT, SENSEX, SENSEX_OCT_FUT } from './fixtures';

describe('instrument naming (FR-WATCH-013) and tagging (FR-WATCH-014)', () => {
  it('AC1 future: display "RELIANCE OCT FUT", F badge, identifier NFO:RELIANCE26OCTFUT', () => {
    expect(displayName(RELIANCE_OCT_FUT)).toBe('RELIANCE OCT FUT');
    expect(isFuture(RELIANCE_OCT_FUT)).toBe(true);
    expect(instrumentId(RELIANCE_OCT_FUT)).toBe('NFO:RELIANCE26OCTFUT');
    expect(exchangeTag(RELIANCE_OCT_FUT)).toBe('NFO-FUT');
  });

  it('AC2 index: NIFTY 50 tagged INDICES', () => {
    expect(displayName(NIFTY50)).toBe('NIFTY 50');
    expect(exchangeTag(NIFTY50)).toBe('INDICES');
    expect(instrumentId(NIFTY50)).toBe('NSE:NIFTY 50');
  });

  it('an F&O stock is tagged NSE on its cash listing; BSE listing tagged BSE; BFO future tagged BFO-FUT', () => {
    expect(exchangeTag(RELIANCE)).toBe('NSE');
    expect(exchangeTag(RELIANCE_BSE)).toBe('BSE');
    expect(exchangeTag(SENSEX_OCT_FUT)).toBe('BFO-FUT');
  });

  it('NFR-15: options, MCX and currency are out of scope', () => {
    expect(isInScope(NIFTY_OPTION)).toBe(false);
    expect(isInScope({ tag: 'MCX-FUT' })).toBe(false);
    expect(isInScope({ tag: 'CDS-FUT' })).toBe(false);
    expect(() => exchangeTag(NIFTY_OPTION)).toThrow(OutOfScopeError);
  });

  it('every in-scope fixture has exactly one tag from the allowed set', () => {
    for (const i of ALL_IN_SCOPE) {
      expect(['INDICES', 'NSE', 'BSE', 'NFO-FUT', 'BFO-FUT']).toContain(exchangeTag(i));
    }
  });

  it('tick size is carried in paise: 5 paise = 0.05 rupees', () => {
    expect(tickRupees(RELIANCE)).toBe(0.05);
    expect(tickRupees(RELIANCE_OCT_FUT)).toBe(0.1);
  });

  it('watchlist items (no tag field) map to the same tag and identifier as the instrument', () => {
    const item = { token: SENSEX.token, exchange: 'BSE', symbol: 'SENSEX', displayName: 'SENSEX', name: 'SENSEX', isIndex: true, missing: false };
    expect(itemTag(item)).toBe('INDICES');
    expect(itemIdentifier(item)).toBe('BSE:SENSEX');
    expect(itemTag({ exchange: 'NFO', isIndex: false })).toBe('NFO-FUT');
    expect(itemTag({ exchange: 'BSE', isIndex: false })).toBe('BSE');
    expect(labelOfItem(item)).toEqual({ token: SENSEX.token, tag: 'INDICES', displayName: 'SENSEX', tradingsymbol: 'SENSEX', name: 'SENSEX' });
  });
});
```

- [ ] **Step 2: Run the tests to check they fail**

Run: `cd frontend && npx vitest run src/chart/timeframes.test.ts src/instruments/naming.test.ts`
Expected: FAIL with `Failed to resolve import "./timeframes"` and `Failed to resolve import "./naming"`.

- [ ] **Step 3: Implement**

`frontend/src/chart/timeframes.ts`:

```ts
export type TimeframeId = '1m' | '3m' | '5m' | '10m' | '15m' | '30m' | '1h' | '1d';
export type KiteInterval =
  | 'minute' | '3minute' | '5minute' | '10minute' | '15minute' | '30minute' | '60minute' | 'day';

export interface Timeframe {
  readonly id: TimeframeId;
  readonly label: string;
  readonly kite: KiteInterval;
  readonly intraday: boolean;
  /** FR-CHART-003 initial window, also used as the lazy-load chunk. */
  readonly initialTradingDays: number;
}

export const TIMEFRAMES: readonly Timeframe[] = [
  { id: '1m', label: '1 Minute', kite: 'minute', intraday: true, initialTradingDays: 5 },
  { id: '3m', label: '3 Minutes', kite: '3minute', intraday: true, initialTradingDays: 15 },
  { id: '5m', label: '5 Minutes', kite: '5minute', intraday: true, initialTradingDays: 15 },
  { id: '10m', label: '10 Minutes', kite: '10minute', intraday: true, initialTradingDays: 30 },
  { id: '15m', label: '15 Minutes', kite: '15minute', intraday: true, initialTradingDays: 30 },
  { id: '30m', label: '30 Minutes', kite: '30minute', intraday: true, initialTradingDays: 120 },
  { id: '1h', label: '1 Hour', kite: '60minute', intraday: true, initialTradingDays: 120 },
  { id: '1d', label: '1 Day', kite: 'day', intraday: false, initialTradingDays: 750 },
];

/** OQ-23: shipped value of Settings → Chart → default timeframe. */
export const SHIPPED_DEFAULT_TIMEFRAME: TimeframeId = '1h';

export function timeframeById(id: string): Timeframe {
  const tf = TIMEFRAMES.find((t) => t.id === id);
  if (!tf) throw new Error(`unknown timeframe ${id}`);
  return tf;
}

export function timeframeFromKite(k: string): Timeframe | undefined {
  return TIMEFRAMES.find((t) => t.kite === k);
}

/** Accepts a TimeframeId or a Kite interval (the setting stores Kite form); anything else → shipped default. */
export function parseTimeframe(v: unknown): TimeframeId {
  if (typeof v === 'string') {
    const byId = TIMEFRAMES.find((t) => t.id === v);
    if (byId) return byId.id;
    const byKite = timeframeFromKite(v);
    if (byKite) return byKite.id;
  }
  return SHIPPED_DEFAULT_TIMEFRAME;
}
```

`frontend/src/instruments/types.ts`:

```ts
/**
 * Frontend view of an instrument-master row (FR-WATCH-008). Field-for-field the canonical
 * service.InstrumentDTO JSON (contract §C); src/api/instruments.ts copies DTOs into it.
 */
export interface Instrument {
  token: number;
  exchange: string; // NSE | BSE | NFO | BFO
  segment: string; // Kite segment, e.g. NSE, INDICES, NFO-FUT
  tag: string; // P0-2 instruments.Tag: INDICES | NSE | BSE | NFO-FUT | BFO-FUT (anything else is out of scope)
  tradingsymbol: string;
  name: string;
  displayName: string; // derived by Go at import (ARCHITECTURE §5.7)
  instrumentType: string; // EQ | FUT
  expiryMs: number; // 0 = no expiry
  tickSizePaise: number; // integer paise, e.g. 5 = 0.05 rupees
  lotSize: number;
  isIndex: boolean;
  isFuture: boolean;
  underlying: string; // futures: the underlying symbol; '' otherwise
}
```

`frontend/src/instruments/naming.ts`:

```ts
import type { Instrument } from './types';

export const EXCHANGE_TAGS = ['INDICES', 'NSE', 'BSE', 'NFO-FUT', 'BFO-FUT'] as const;
export type ExchangeTag = (typeof EXCHANGE_TAGS)[number];

export class OutOfScopeError extends Error {
  constructor(tag: string) {
    super(`tag ${tag} is out of scope (NFR-15)`);
    this.name = 'OutOfScopeError';
  }
}

/** What InstrumentLabel / InstrumentRow need; a full Instrument and a watchlist item both provide it. */
export type LabelInstrument = Pick<Instrument, 'token' | 'tag' | 'displayName' | 'tradingsymbol' | 'name'>;

/** NFR-15 guard: true only for indices, NSE/BSE cash and NFO/BFO futures. */
export function isInScope(i: Pick<Instrument, 'tag'>): boolean {
  return (EXCHANGE_TAGS as readonly string[]).includes(i.tag);
}

/** FR-WATCH-014: the tag is the instrument's own master tag (P0-2 instruments.Tag), never inferred from context. */
export function exchangeTag(i: Pick<Instrument, 'tag'>): ExchangeTag {
  if (!isInScope(i)) throw new OutOfScopeError(i.tag);
  return i.tag as ExchangeTag;
}

export function isFuture(i: Pick<Instrument, 'tag'>): boolean {
  return i.tag === 'NFO-FUT' || i.tag === 'BFO-FUT';
}

/** Tick size in rupees (the DTO carries integer paise). */
export function tickRupees(i: Pick<Instrument, 'tickSizePaise'>): number {
  return i.tickSizePaise / 100;
}

/**
 * Watchlist items (service.WatchlistItem) carry exchange + isIndex, not the tag. Inside the NFR-15 scope the
 * mapping is exact: indices → INDICES, NFO/BFO rows are futures, NSE/BSE rows are cash.
 */
export function itemTag(item: { exchange: string; isIndex: boolean }): ExchangeTag {
  if (item.isIndex) return 'INDICES';
  if (item.exchange === 'NFO') return 'NFO-FUT';
  if (item.exchange === 'BFO') return 'BFO-FUT';
  if (item.exchange === 'NSE' || item.exchange === 'BSE') return item.exchange;
  throw new OutOfScopeError(item.exchange);
}

export function itemIdentifier(item: { exchange: string; symbol: string }): string {
  return `${item.exchange}:${item.symbol}`;
}

export function labelOfItem(item: { token: number; exchange: string; symbol: string; displayName: string; name: string; isIndex: boolean }): LabelInstrument {
  return { token: item.token, tag: itemTag(item), displayName: item.displayName, tradingsymbol: item.symbol, name: item.name };
}

/** FR-WATCH-013: Kite EXCHANGE:TRADINGSYMBOL form, used internally and in deep links. */
export function instrumentId(i: Pick<Instrument, 'exchange' | 'tradingsymbol'>): string {
  return `${i.exchange}:${i.tradingsymbol}`;
}

/** FR-WATCH-013: the one name formatter. Go derives displayName at import; the symbol is the fallback. */
export function displayName(i: Pick<Instrument, 'displayName' | 'tradingsymbol'>): string {
  return i.displayName || i.tradingsymbol;
}
```

`frontend/src/instruments/fixtures.ts` (used by unit tests, the mock backend and e2e):

```ts
import type { Instrument } from './types';

const eq = (token: number, exchange: 'NSE' | 'BSE', symbol: string, name: string): Instrument => ({
  token, exchange, segment: exchange, tag: exchange, tradingsymbol: symbol, name, displayName: symbol,
  instrumentType: 'EQ', expiryMs: 0, tickSizePaise: 5, lotSize: 1, isIndex: false, isFuture: false, underlying: '',
});
const index = (token: number, exchange: 'NSE' | 'BSE', symbol: string): Instrument => ({
  token, exchange, segment: 'INDICES', tag: 'INDICES', tradingsymbol: symbol, name: symbol, displayName: symbol,
  instrumentType: 'EQ', expiryMs: 0, tickSizePaise: 5, lotSize: 0, isIndex: true, isFuture: false, underlying: '',
});
// 27 Oct 2026 15:30 IST — fixture expiry only.
const OCT_2026_EXPIRY = Date.UTC(2026, 9, 27, 10, 0);
const fut = (token: number, exchange: 'NFO' | 'BFO', symbol: string, underlying: string, lot: number): Instrument => ({
  token, exchange, segment: `${exchange}-FUT`, tag: `${exchange}-FUT`, tradingsymbol: symbol, name: underlying,
  displayName: `${underlying} OCT FUT`, instrumentType: 'FUT', expiryMs: OCT_2026_EXPIRY,
  tickSizePaise: exchange === 'NFO' ? 10 : 5, lotSize: lot, isIndex: false, isFuture: true, underlying,
});

export const NIFTY50 = index(256265, 'NSE', 'NIFTY 50');
export const NIFTYBANK = index(260105, 'NSE', 'NIFTY BANK');
export const SENSEX = index(265, 'BSE', 'SENSEX');
export const BANKEX = index(274441, 'BSE', 'BANKEX');
export const RELIANCE = eq(738561, 'NSE', 'RELIANCE', 'RELIANCE INDUSTRIES');
export const RELIANCE_BSE = eq(128083204, 'BSE', 'RELIANCE', 'RELIANCE INDUSTRIES');
export const SBIN = eq(779521, 'NSE', 'SBIN', 'STATE BANK OF INDIA');
export const TCS = eq(2953217, 'NSE', 'TCS', 'TATA CONSULTANCY SERVICES');
export const INFY = eq(408065, 'NSE', 'INFY', 'INFOSYS');
export const ADANIENT = eq(6401, 'NSE', 'ADANIENT', 'ADANI ENTERPRISES');
export const NEWCO = eq(999001, 'NSE', 'NEWCO', 'NEWLY LISTED CO'); // no history (Review Focus 1)
export const RELIANCE_OCT_FUT = fut(13368834, 'NFO', 'RELIANCE26OCTFUT', 'RELIANCE', 500);
export const NIFTY_OCT_FUT = fut(13238530, 'NFO', 'NIFTY26OCTFUT', 'NIFTY', 75);
export const BANKNIFTY_OCT_FUT = fut(13237250, 'NFO', 'BANKNIFTY26OCTFUT', 'BANKNIFTY', 35);
export const SENSEX_OCT_FUT = fut(285769221, 'BFO', 'SENSEX26OCTFUT', 'SENSEX', 20);
/** Out of scope — must never render anywhere (NFR-15). */
export const NIFTY_OPTION: Instrument = {
  token: 12345678, exchange: 'NFO', segment: 'NFO-OPT', tag: 'NFO-OPT', tradingsymbol: 'NIFTY26OCT22500CE', name: 'NIFTY',
  displayName: 'NIFTY OCT 22500 CE', instrumentType: 'CE', expiryMs: OCT_2026_EXPIRY, tickSizePaise: 5, lotSize: 75,
  isIndex: false, isFuture: false, underlying: 'NIFTY',
};

export const ALL_IN_SCOPE: readonly Instrument[] = [
  NIFTY50, NIFTYBANK, SENSEX, BANKEX, RELIANCE, RELIANCE_BSE, SBIN, TCS, INFY, ADANIENT, NEWCO,
  RELIANCE_OCT_FUT, NIFTY_OCT_FUT, BANKNIFTY_OCT_FUT, SENSEX_OCT_FUT,
];
```

- [ ] **Step 4: Run the tests to check they pass**

Run: `cd frontend && npx vitest run src/chart/timeframes.test.ts src/instruments/naming.test.ts`
Expected: PASS (10 tests: timeframes 3, naming 7).

- [ ] **Step 5: Commit**

```bash
git add frontend/src/chart/timeframes.ts frontend/src/chart/timeframes.test.ts frontend/src/instruments
git commit -m "$(cat <<'EOF'
feat(frontend): timeframes, instrument model, naming and NFR-15 scope guard

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
)"
```

---
