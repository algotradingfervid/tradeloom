import '@testing-library/jest-dom/vitest';
import { afterEach } from 'vitest';
import { cleanup } from '@testing-library/react';
import { resetAllStores } from '../store/reset';
import { installMatchMedia, resetMatchMedia } from './matchMedia';

installMatchMedia();

afterEach(() => {
  cleanup();
  resetAllStores();
  resetMatchMedia();
});
