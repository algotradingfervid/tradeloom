import { describe, expect, it } from 'vitest';
import { chartPriceFormatter, precisionForTick } from './format';

describe('chart price format', () => {
  it('derives precision from tick size with a floor of 2', () => {
    expect(precisionForTick(0.05)).toBe(2);
    expect(precisionForTick(0.1)).toBe(2);
    expect(precisionForTick(0.0025)).toBe(4);
    expect(precisionForTick(1)).toBe(2);
  });
  it('uses Indian grouping on the price scale', () => {
    expect(chartPriceFormatter(2)(2252045.5)).toBe('22,52,045.50');
  });
});
