import '@testing-library/jest-dom/vitest';
import { afterEach } from 'vitest';
import { cleanup } from '@testing-library/react';
import { resetAllStores } from '../store/reset';
import { installMatchMedia, resetMatchMedia } from './matchMedia';

// Guarded for `// @vitest-environment node` files (prodBuild.test.ts), which have no window.
if (typeof window !== 'undefined') installMatchMedia();

afterEach(() => {
  cleanup();
  resetAllStores();
  if (typeof window !== 'undefined') resetMatchMedia();
});
