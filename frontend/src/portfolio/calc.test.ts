import { describe, expect, it } from 'vitest';
import { holdingA, holdingB, holdingsOf, positionClosed, positionOpen, tickFixture } from '../api/mock/fixtures';
import { holdingRow, isTerminalStatus, livePosition, liveLtp, orderStatusLabel, positionNetChangePct, positionPnl, positionsTotalPnl, rupees, summarizeHoldings, toPaise } from './calc';

describe('portfolio arithmetic (paise from Go → rupees in the view)', () => {
  it('converts paise to rupees', () => {
    expect(rupees(151_025)).toBe(1510.25);
    expect(toPaise(1510.25)).toBe(151_025);
  });
  it('Ruling 96 M2: toPaise rounds half away from zero, like Go orders.Paise', () => {
    expect(toPaise(0.125)).toBe(13);
    expect(toPaise(-0.125)).toBe(-13);
    expect(Object.is(toPaise(-0.001), 0)).toBe(true);
  });
  it('FR-ORD-003 AC2: 10 @ 100 with LTP 110 → invested 1,000, current 1,100, P&L 100, net change +10 %', () => {
    expect(holdingRow(holdingA)).toMatchObject({ ltp: 110, invested: 1000, current: 1100, pnl: 100, netChangePct: 10, dayChange: 50 });
  });
  it('FR-HOME-002 AC1: totals start from the Go summary (quantity = settled + T1, summed by Go)', () => {
    const snap = holdingsOf([holdingA, holdingB]);
    const s = summarizeHoldings(snap.summary, snap.rows.map((r) => holdingRow(r)));
    expect(s).toEqual({ empty: false, currentValue: 2050, investment: 2000, pnl: 50, pnlPct: 2.5, dayChange: 0 });
  });
  it('FR-HOME-002 AC2: a live LTP replaces the snapshot price in rows and totals', () => {
    const snap = holdingsOf([holdingA, holdingB]);
    expect(holdingRow(holdingA, 120)).toMatchObject({ ltp: 120, current: 1200, pnl: 200, dayChange: 150 });
    expect(summarizeHoldings(snap.summary, [holdingRow(holdingA, 120), holdingRow(holdingB)]).currentValue).toBe(2150);
  });
  it('totals stay exact in paise across many fractional live prices (no float drift)', () => {
    const rows = Array.from({ length: 50 }, (_, i) => ({ ...holdingA, token: i + 1, qty: 3, avgPaise: 10, ltpPaise: 10, prevClosePaise: 10, investedPaise: 30, currentPaise: 30, pnlPaise: 0, dayChangePaise: 0 }));
    const snap = holdingsOf(rows);
    const s = summarizeHoldings(snap.summary, rows.map((r) => holdingRow(r, 0.1 + 0.2)));
    // 50 × 3 × 30 paise = 4,500 paise; summing 0.9 fifty times in floats would not land on 45 exactly.
    expect(s.currentValue).toBe(45);
    expect(s.dayChange).toBe(30);
  });
  it('mirrors Go: no previous close → no day change; a non-positive live LTP is ignored', () => {
    const noClose = { ...holdingA, prevClosePaise: 0, dayChangePaise: 0, dayChangeBp: 0 };
    expect(holdingRow(noClose, 120)).toMatchObject({ dayChange: 0, dayChangePct: null });
    expect(holdingRow(holdingA, 0)).toMatchObject({ ltp: 110 });
  });
  it('no investment → no percentage (never divide by zero)', () => {
    const empty = holdingsOf([]);
    expect(summarizeHoldings(empty.summary, [])).toMatchObject({ empty: true, pnlPct: null });
  });
  it('FR-ORD-002 AC1: position P&L comes from Go; a live LTP adds qty × (LTP − snapshot LTP)', () => {
    expect(positionPnl(positionOpen)).toBe(120);
    expect(positionPnl(positionClosed)).toBe(-45.5);
    expect(positionPnl(positionOpen, 113)).toBe(130);
    expect(positionNetChangePct(positionOpen, 112)).toBeCloseTo(12, 10);
    expect(positionNetChangePct(positionClosed, 101)).toBeNull();
  });
  it('labels Kite order statuses and knows which are final', () => {
    expect(orderStatusLabel('TRIGGER PENDING')).toBe('Trigger pending');
    expect(isTerminalStatus('COMPLETE')).toBe(true);
    expect(isTerminalStatus('OPEN')).toBe(false);
  });
  it('carry-forward: a quote older than the current session is stale and never used as the live LTP', () => {
    const tick = tickFixture(holdingA.token, 120);
    expect(liveLtp(tick, tick.tsMs - 1)).toBe(120);
    expect(liveLtp(tick, tick.tsMs + 1)).toBeUndefined();
    expect(liveLtp(tick, 0)).toBeUndefined();
    expect(liveLtp(undefined, 1)).toBeUndefined();
  });
});

describe('positions at live LTP (P0-4 Task 11)', () => {
  it('livePosition mirrors Go orders.positionRow: P&L moves by qty × multiplier × (LTP − Go LTP), in paise', () => {
    const r = livePosition(positionOpen, 113);
    expect([r.ltpPaise, r.pnlPaise, r.ltp, r.pnl, r.open]).toEqual([11_300, 13_000, 113, 130, true]);
    expect(livePosition({ ...positionOpen, multiplier: 50 }, 113).pnlPaise).toBe(12_000 + 10 * 50 * 100);
    expect(livePosition(positionClosed).pnlPaise).toBe(-4_550);
    expect(livePosition(positionClosed).open).toBe(false);
    expect(livePosition(positionOpen, 0).pnlPaise).toBe(12_000);
  });
  it('a CDS/BCD row keeps Go’s snapshot (4-decimal prices; Ruling 96 M1)', () => {
    const cds = { ...positionOpen, exchange: 'CDS', outOfScope: true };
    expect(livePosition(cds, 113).pnlPaise).toBe(12_000);
    expect(livePosition({ ...cds, exchange: 'BCD' }, 113).ltpPaise).toBe(11_200);
  });
  it('positionsTotalPnl starts from Go’s total and adds each row’s live difference in integer paise', () => {
    const rows = [livePosition(positionOpen, 113), livePosition(positionClosed)];
    expect(positionsTotalPnl(7_450, rows)).toBe(84.5);
    expect(positionsTotalPnl(7_450, [livePosition(positionOpen), livePosition(positionClosed)])).toBe(74.5);
  });
});
