import { describe, expect, it } from 'vitest';
import { TickMarkType } from 'lightweight-charts';
import { formatCrosshairTime, formatTickMark, fromChartTime, IST_OFFSET_SECONDS, toChartTime } from './time';

// 01 Oct 2026 09:15 IST == 03:45 UTC
const OCT1_0915_IST = Date.UTC(2026, 9, 1, 3, 45);

describe('IST time axis', () => {
  it('shifts epoch ms to IST wall-clock seconds and back', () => {
    expect(IST_OFFSET_SECONDS).toBe(19800);
    expect(toChartTime(OCT1_0915_IST)).toBe(OCT1_0915_IST / 1000 + 19800);
    expect(fromChartTime(toChartTime(OCT1_0915_IST))).toBe(OCT1_0915_IST);
  });
  it("FR-CHART-011 AC1: crosshair label reads 01 Oct '26 09:15", () => {
    expect(formatCrosshairTime(toChartTime(OCT1_0915_IST), true)).toBe("01 Oct '26 09:15");
    expect(formatCrosshairTime(toChartTime(OCT1_0915_IST), false)).toBe("01 Oct '26");
  });
  it('tick marks are IST', () => {
    const t = toChartTime(OCT1_0915_IST);
    expect(formatTickMark(t, TickMarkType.Year)).toBe('2026');
    expect(formatTickMark(t, TickMarkType.Month)).toBe('Oct');
    expect(formatTickMark(t, TickMarkType.DayOfMonth)).toBe('01');
    expect(formatTickMark(t, TickMarkType.Time)).toBe('09:15');
    expect(formatTickMark(t, TickMarkType.TimeWithSeconds)).toBe('09:15:00');
  });
});
