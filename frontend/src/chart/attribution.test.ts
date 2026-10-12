import { describe, expect, it } from 'vitest';
import { CHART_ATTRIBUTION } from './attribution';

describe('CHART_ATTRIBUTION', () => {
  it('carries the upstream NOTICE text (v5.2.1) in plain ASCII', () => {
    expect(CHART_ATTRIBUTION.notice).toBe('Copyright (c) 2025 TradingView, Inc. https://www.tradingview.com/');
    expect(CHART_ATTRIBUTION.notice).toMatch(/^[\x20-\x7E]+$/);
    expect(CHART_ATTRIBUTION.licence).toBe('Apache-2.0');
    expect(CHART_ATTRIBUTION.url).toBe('https://www.tradingview.com/');
  });
});
