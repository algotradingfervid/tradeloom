import { describe, expect, it } from 'vitest';
import { dedupeSorted, mergeBars } from './bars';

const b = (time: number, close: number) => ({ time, open: close, high: close, low: close, close, volume: 1 });

describe('bars', () => {
  it('sorts ascending and keeps the last duplicate', () => {
    expect(dedupeSorted([b(3, 3), b(1, 1), b(3, 30), b(2, 2)])).toEqual([b(1, 1), b(2, 2), b(3, 30)]);
  });
  it('mergeBars lets incoming replace existing times and adds new ones', () => {
    expect(mergeBars([b(1, 1), b(2, 2)], [b(2, 20), b(4, 4)])).toEqual([b(1, 1), b(2, 20), b(4, 4)]);
  });
});
