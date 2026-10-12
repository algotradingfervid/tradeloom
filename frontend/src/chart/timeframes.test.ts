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
