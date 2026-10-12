import '@testing-library/jest-dom/vitest';
import { afterEach, beforeEach, vi } from 'vitest';
import { cleanup } from '@testing-library/react';
import { FIXED_NOW } from '../api/mock/fixtures';
import { setRunStartedMs } from '../market/quotes';
import { resetAllStores } from '../store/reset';
import { installMatchMedia, resetMatchMedia } from './matchMedia';

// jsdom has no canvas: lightweight-charts throws on its first paint. Every test that mounts the real
// LightweightChartsAdapter (Home renders the dashboard chart, P0-5 Task 10) draws into the fake library instead.
// Adapter tests inject their own fake through AdapterOptions.lib.
vi.mock('../chart/lwc', async () => ({ realLwc: (await import('../chart/testing/fakeLwc')).makeFakeLwc().lib }));

// Guarded for `// @vitest-environment node` files (prodBuild.test.ts), which have no window.
if (typeof window !== 'undefined') installMatchMedia();

// Fixture ticks are stamped FIXED_NOW; this run "started" just before it, so they count as fresh (Ruling 93).
// Tests that exercise staleness set their own run start.
beforeEach(() => setRunStartedMs(FIXED_NOW - 60 * 60_000));

afterEach(() => {
  cleanup();
  resetAllStores();
  if (typeof window !== 'undefined') resetMatchMedia();
});
