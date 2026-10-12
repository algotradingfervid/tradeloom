import { describe, expect, it } from 'vitest';
import {
  DASH, formatChange, formatCompact, formatDate, formatDateLong, formatDateTimeCompact, formatDateTimeLong,
  formatExpiry, formatMoney, formatNumber, formatPercent, formatPrice, formatSigned, formatSignedMoney, formatTime,
  istDayKey, toneOf,
} from './format';

// 09 Oct 2026 15:15:32 IST
const T = Date.UTC(2026, 9, 9, 9, 45, 32);

describe('FR-SHELL-016 numbers', () => {
  it('AC1 abbreviates OI to crore and lakh', () => {
    expect(formatCompact(13_200_000)).toBe('1.32Cr');
    expect(formatCompact(254_000)).toBe('2.54L');
    expect(formatCompact(99_999)).toBe('99,999');
  });
  it('AC2 formats money with ₹ and Indian grouping', () => {
    expect(formatMoney(1234567.8)).toBe('₹12,34,567.80');
  });
  it('formats prices at tick precision', () => {
    expect(formatPrice(22520.45)).toBe('22,520.45');
    expect(formatPrice(1.5, 4)).toBe('1.5000');
  });
  it('formats signed change with percent in parentheses (FR-SHELL-004 AC2 style)', () => {
    expect(formatChange(100, 0.4444)).toBe('+100.00 (+0.44%)');
    expect(formatSigned(-12.3)).toBe('-12.30');
    expect(formatPercent(10)).toBe('+10.00%');
  });
  it('classifies tone', () => {
    expect(toneOf(1)).toBe('up');
    expect(toneOf(-0.01)).toBe('down');
    expect(toneOf(-0.001)).toBe('flat'); // displays as 0.00, so no red
    expect(toneOf(0)).toBe('flat');
    expect(toneOf(null)).toBe('flat');
  });
});

describe('FR-SHELL-016 dates (IST regardless of OS zone)', () => {
  it('renders the long and compact styles', () => {
    expect(formatDateTimeLong(T)).toBe('09 Oct 2026, 15:15');
    expect(formatDateTimeCompact(T)).toBe('09-10-2026 15:15:32');
    expect(formatDate(T)).toBe('09-10-2026');
    expect(formatDateLong(T)).toBe('09 Oct 2026');
    expect(formatTime(T)).toBe('15:15');
    expect(formatTime(T, true)).toBe('15:15:32');
  });
  it('uses our own month table (ICU en-IN would print "Sept")', () => {
    expect(formatDateLong(Date.UTC(2026, 8, 9, 6, 0))).toBe('09 Sep 2026');
  });
  it('rolls the date at IST midnight and never prints 24:00', () => {
    expect(formatDateTimeCompact(Date.UTC(2026, 9, 9, 18, 30, 0))).toBe('10-10-2026 00:00:00');
    expect(istDayKey(Date.UTC(2026, 9, 9, 18, 30, 0))).toBe('2026-10-10');
  });
  it('describes session expiry relative to the app clock (FR-ACCT-001 AC3)', () => {
    const loginAt0902 = Date.UTC(2026, 9, 9, 3, 32);
    const nextSix = Date.UTC(2026, 9, 10, 0, 30);
    expect(formatExpiry(nextSix, loginAt0902)).toBe('06:00 tomorrow');
    expect(formatExpiry(nextSix, Date.UTC(2026, 9, 9, 20, 0))).toBe('06:00 today');
    expect(formatExpiry(Date.UTC(2026, 9, 12, 0, 30), loginAt0902)).toBe('12 Oct 2026, 06:00');
  });
});

describe('review focus: very large, negative and non-finite values', () => {
  it('keeps Indian grouping and one leading sign for large and negative values', () => {
    expect(formatMoney(-1234567890.5)).toBe('-₹1,23,45,67,890.50');
    expect(formatNumber(1e15)).toBe('1,00,00,00,00,00,00,000.00');
    expect(formatSignedMoney(-45.5)).toBe('-₹45.50');
    expect(formatSignedMoney(50)).toBe('+₹50.00');
    expect(formatCompact(-13_200_000)).toBe('-1.32Cr');
    expect(formatPercent(-2.5)).toBe('-2.50%');
  });
  it('rolls lakh into crore and units into lakh at rounding boundaries', () => {
    expect(formatCompact(9_999_999.999)).toBe('1.00Cr');
    expect(formatCompact(99_999.996)).toBe('1.00L');
  });
  it('never renders negative zero', () => {
    expect(formatNumber(-0.001)).toBe('0.00');
    expect(formatSignedMoney(-0.004)).toBe('+₹0.00');
    expect(formatMoney(-0)).toBe('₹0.00');
  });
  it('renders an em dash for missing or non-finite values, never 0.00', () => {
    for (const v of [NaN, Infinity, -Infinity, null, undefined]) {
      expect(formatNumber(v)).toBe(DASH);
      expect(formatMoney(v)).toBe(DASH);
      expect(formatSignedMoney(v)).toBe(DASH);
      expect(formatPercent(v)).toBe(DASH);
      expect(formatCompact(v)).toBe(DASH);
      expect(formatChange(v, 1)).toBe(DASH);
    }
    expect(formatDate(0)).toBe(DASH);
    expect(formatDateTimeLong(NaN)).toBe(DASH);
  });
});
