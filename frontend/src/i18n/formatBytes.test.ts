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

it('fix round 1: unit boundaries and the TB range', () => {
  expect(formatBytes(1023)).toBe('1,023 B');
  expect(formatBytes(1024)).toBe('1.00 KB');
  expect(formatBytes(1.5 * 1024 ** 4)).toBe('1.50 TB');
  expect(formatBytes(1024 ** 5)).toBe('1,024.0 TB');
});
