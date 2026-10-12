export interface ResetOptions {
  /**
   * Also run on sign-out (the bridge's transition to `logged_out` / `no_credentials`, Ruling 83-F M5). Defaults to
   * true so account data (portfolio, profile-derived state) is cleared; stores that are not account data opt out
   * (session, status, settings, app info, ticks; Task 4's toasts and job notices — Ruling 85).
   */
  onLogout?: boolean;
  /**
   * Returns everything the registration resets (store state plus module-level caches). Tests enumerate it through
   * `storeSnapshots()`, e.g. the onboarding leak tests (Ruling 108 M1), so a new store is covered automatically.
   */
  inspect?: () => unknown;
}

export interface Registration { fn: () => void; onLogout: boolean; inspect?: () => unknown }

const resets: Registration[] = [];

export function registerReset(fn: () => void, opts: ResetOptions = {}): void {
  resets.push({ fn, onLogout: opts.onLogout ?? true, inspect: opts.inspect });
}

/** The reset registry (read-only view): one entry per registered store or module cache. */
export function registeredResets(): readonly Readonly<Registration>[] {
  return resets;
}

/** Test helper: what every registration currently holds (`inspect()` of each; `undefined` where none is given). */
export function storeSnapshots(): unknown[] {
  return resets.map((r) => r.inspect?.());
}

/** Test helper: return every registered store to its initial state. */
export function resetAllStores(): void {
  for (const r of resets) r.fn();
}

/** Sign-out: reset the stores that hold account data (every registration without `onLogout: false`). */
export function resetOnLogout(): void {
  for (const r of resets) if (r.onLogout) r.fn();
}
