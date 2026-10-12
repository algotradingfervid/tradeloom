import type { HoldingRow, HoldingsSummary, PositionRow, TickDTO } from '../api/backend';
import { isStaleQuote } from '../market/quotes';

/** Go sends money as integer paise (AD-04); the view layer works in rupees. Sums are always taken in paise. */
export const rupees = (paise: number): number => paise / 100;
/** Same rounding as Go `orders.Paise` (`math.Round`: half away from zero); never returns -0. */
export const toPaise = (r: number): number => {
  const p = Math.sign(r) * Math.round(Math.abs(r * 100));
  return p === 0 ? 0 : p;
};

/**
 * num/den in basis points, rounded like Go `orders.RatioBp`, then as a percentage. Deliberate divergence from Go:
 * with no base (den ≤ 0) Go returns 0 bp, which would render as "+0.00%"; the UI returns null so the cell shows "—"
 * (FR-SHELL-016: missing values never render as 0.00). The UI recomputes from paise rather than reading Go's `…Bp`
 * fields only because a live LTP changes the numerator.
 */
function ratioPct(num: number, den: number): number | null {
  if (den <= 0) return null;
  const bp = Math.sign(num) * Math.round(Math.abs(num * 10_000) / den);
  return bp / 100;
}

/**
 * The tick's LTP when it is a live quote of this run. Go replays quote snapshots persisted by an earlier run
 * (QuoteCacheKey): such a quote is stale (Ruling 93: the one shared rule, isStaleQuote) and the view keeps the LTP
 * Go priced the snapshot at until a fresh tick arrives (P0-4 Task 10 carry-forward).
 */
export function liveLtp(tick: TickDTO | undefined): number | undefined {
  if (!tick || isStaleQuote(tick) || !(tick.ltp > 0)) return undefined;
  return tick.ltp;
}

export interface LiveHolding {
  row: HoldingRow;
  ltp: number;
  invested: number;
  current: number;
  pnl: number;
  netChangePct: number | null;
  dayChange: number;
  dayChangePct: number | null;
  /** The same values in paise, so totals are summed without float drift. */
  ltpPaise: number;
  currentPaise: number;
  dayChangePaise: number;
}

/**
 * One holding at the newest LTP (`tick.batch`), falling back to the LTP Go priced the snapshot at. Mirrors Go
 * `orders.holdingRow`: a non-positive live price is ignored and there is no day change without a previous close.
 */
export function holdingRow(row: HoldingRow, live?: number): LiveHolding {
  const ltpPaise = live !== undefined && live > 0 ? toPaise(live) : row.ltpPaise;
  const currentPaise = row.qty * ltpPaise;
  const dayChangePaise = row.prevClosePaise !== 0 ? row.qty * (ltpPaise - row.prevClosePaise) : 0;
  return {
    row,
    ltp: rupees(ltpPaise),
    invested: rupees(row.investedPaise),
    current: rupees(currentPaise),
    pnl: rupees(currentPaise - row.investedPaise),
    netChangePct: ratioPct(ltpPaise - row.avgPaise, row.avgPaise),
    dayChange: rupees(dayChangePaise),
    dayChangePct: ratioPct(ltpPaise - row.prevClosePaise, row.prevClosePaise),
    ltpPaise,
    currentPaise,
    dayChangePaise,
  };
}

export interface HoldingsTotals { empty: boolean; currentValue: number; investment: number; pnl: number; pnlPct: number | null; dayChange: number }

/**
 * FR-HOME-002 card definitions, computed by Go (`orders.Summarize`, every row incl. out-of-scope ones); the UI only
 * adds each row's live-LTP difference in integer paise (current = Σ qty×LTP, investment = Σ qty×avg,
 * day's change = Σ qty×(LTP − prev close)).
 */
export function summarizeHoldings(summary: HoldingsSummary, rows: LiveHolding[]): HoldingsTotals {
  let currentPaise = summary.currentValuePaise;
  let dayPaise = summary.dayChangePaise;
  for (const r of rows) {
    currentPaise += r.currentPaise - r.row.currentPaise;
    dayPaise += r.dayChangePaise - r.row.dayChangePaise;
  }
  const pnlPaise = currentPaise - summary.investmentPaise;
  return {
    empty: summary.empty,
    currentValue: rupees(currentPaise),
    investment: rupees(summary.investmentPaise),
    pnl: rupees(pnlPaise),
    pnlPct: ratioPct(pnlPaise, summary.investmentPaise),
    dayChange: rupees(dayPaise),
  };
}

/**
 * Go's position P&L (`pnlPaise`) is priced at `ltpPaise`; a newer tick moves it by
 * qty × multiplier × (LTP − ltpPaise), the same linear adjustment orders.BuildPositions uses
 * (P0-3 Task 12). `multiplier` is Kite's; Go already maps 0 → 1.
 */
export function positionPnl(row: PositionRow, live?: number): number {
  if (live === undefined || !(live > 0)) return rupees(row.pnlPaise);
  const mult = row.multiplier > 0 ? row.multiplier : 1;
  return rupees(row.pnlPaise + row.qty * mult * (toPaise(live) - row.ltpPaise));
}

export interface LivePosition {
  row: PositionRow;
  ltp: number;
  pnl: number;
  open: boolean;
  /** The same values in paise, so totals are summed without float drift. */
  ltpPaise: number;
  pnlPaise: number;
}

/**
 * CDS/BCD prices have 4 decimals; Go sends paise, so a live overlay computed from them can differ from Kite's P&L.
 * Until Go sends the raw LTP (P0-3 follow-up, Ruling 96 M1) those rows keep Go's snapshot LTP and P&L.
 */
const SUB_PAISE_EXCHANGES = new Set(['CDS', 'BCD']);

/**
 * One position at the newest LTP. Mirrors Go `orders.positionRow`: P&L = Go P&L + qty × multiplier × (LTP − Go LTP)
 * (a non-positive live price is ignored; Go already maps multiplier 0 → 1); closed means qty 0.
 */
export function livePosition(row: PositionRow, live?: number): LivePosition {
  const usable = live !== undefined && live > 0 && !SUB_PAISE_EXCHANGES.has(row.exchange);
  const mult = row.multiplier > 0 ? row.multiplier : 1;
  const ltpPaise = usable ? toPaise(live) : row.ltpPaise;
  const pnlPaise = usable ? row.pnlPaise + row.qty * mult * (ltpPaise - row.ltpPaise) : row.pnlPaise;
  return { row, ltp: rupees(ltpPaise), pnl: rupees(pnlPaise), open: !row.closed, ltpPaise, pnlPaise };
}

/**
 * FR-ORD-002 AC1 total P&L: Go's `netTotalPnlPaise` / `dayTotalPnlPaise` (every row, incl. out-of-scope ones) plus
 * each row's live difference, in integer paise.
 */
export function positionsTotalPnl(goTotalPaise: number, rows: LivePosition[]): number {
  let total = goTotalPaise;
  for (const r of rows) total += r.pnlPaise - r.row.pnlPaise;
  return rupees(total);
}

export function positionNetChangePct(row: PositionRow, ltp: number): number | null {
  return row.qty !== 0 ? ratioPct(toPaise(ltp) - row.avgPaise, row.avgPaise) : null;
}

export function orderStatusLabel(status: string): string {
  const s = status.toLowerCase();
  return s.charAt(0).toUpperCase() + s.slice(1);
}

/** Kite statuses after which an order no longer changes (Go's `orders.SectionOf` non-open sections). */
export function isTerminalStatus(status: string): boolean {
  const s = status.toUpperCase();
  return s === 'COMPLETE' || s === 'REJECTED' || s === 'CANCELLED';
}
