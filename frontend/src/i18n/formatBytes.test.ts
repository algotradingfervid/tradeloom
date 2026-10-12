import { expect, it } from 'vitest';
import { DASH, formatBytes } from './format';

it('formats sizes in binary units with Indian grouping', () => {
  expect(formatBytes(0)).toBe('0 B');
  expect(formatBytes(1536)).toBe('1.50 KB');
  expect(formatBytes(734_003_200)).toBe('700.0 MB');
  expect(formatBytes(10_737_418_240)).toBe('10.0 GB');
  expect(formatBytes(-1)).toBe(DASH);
  expect(formatBytes(undefined)).toBe(DASH);
});
