export interface ResetOptions {
  /**
   * Also run on sign-out (the bridge's transition to `logged_out` / `no_credentials`, Ruling 83-F M5). Defaults to
   * true so account data (portfolio, profile-derived state) is cleared; stores that are not account data opt out
   * (session, status, settings, app info, ticks; Task 4's toasts and job notices — Ruling 85).
   */
  onLogout?: boolean;
}

const resets: { fn: () => void; onLogout: boolean }[] = [];

export function registerReset(fn: () => void, opts: ResetOptions = {}): void {
  resets.push({ fn, onLogout: opts.onLogout ?? true });
}

/** Test helper: return every registered store to its initial state. */
export function resetAllStores(): void {
  for (const r of resets) r.fn();
}

/** Sign-out: reset the stores that hold account data (every registration without `onLogout: false`). */
export function resetOnLogout(): void {
  for (const r of resets) if (r.onLogout) r.fn();
}
