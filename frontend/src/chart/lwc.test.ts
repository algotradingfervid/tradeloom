import { describe, expect, it } from 'vitest';
import { realLwc } from './lwc';
import { FakeChart } from './testing/fakeLwc';

describe('lwc seam under vitest (test/setup.ts)', () => {
  it('realLwc is the fake library: jsdom has no canvas', () => {
    expect(realLwc.createChart(document.createElement('div'))).toBeInstanceOf(FakeChart);
  });
});
