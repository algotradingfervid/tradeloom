import type { Backend } from './backend';
import { createMockBackend, type MockBackend } from './mock/index';
import { seedByName } from './mock/fixtures';
import { createWailsBackend } from './wails';

declare global {
  interface Window { __tradeloomMock?: MockBackend }
}

/** Real Wails bindings, or the in-memory mock under `vite --mode mock` (Playwright, `npm run dev:mock`). */
export function defaultBackend(): Backend {
  if (import.meta.env.VITE_BACKEND !== 'mock') return createWailsBackend();
  const params = new URLSearchParams(window.location.search);
  const mock = createMockBackend(seedByName(params.get('seed') ?? 'demo'), {
    persistKey: params.has('persist') ? 'tradeloom.mock.settings' : undefined,
    autoJobs: true,
  });
  // `?fail=system.info,settings.all`: those calls fail until __tradeloomMock.clearFailures() (e2e of the boot notice).
  for (const method of params.get('fail')?.split(',').filter(Boolean) ?? []) mock.failAlways(method, `${method} unavailable (mock)`);
  window.__tradeloomMock = mock;
  return mock;
}
