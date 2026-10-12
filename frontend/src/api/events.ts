import { useAppStore, type BootSlice } from '../store/app';
import { resetOnLogout } from '../store/reset';
import { notifyUnclaimedJob } from '../store/jobNotices';
import { usePortfolioStore } from '../store/portfolio';
import { useSessionStore } from '../store/session';
import { markSettingsBaseline, useSettingsStore } from '../store/settings';
import { useStatusStore } from '../store/status';
import { useTicksStore } from '../store/ticks';
import { useWatchlists } from '../store/watchlistsStore';
import { applyShortcutsEvent, SHORTCUTS_CHANGE_KEY } from '../shortcuts/store';
import type { Backend, SessionStateName } from './backend';
import { parseBackendError } from './errors';

export interface BridgeDeps { navigate: (route: string) => void }

/**
 * Bootstrap slices that also arrive as events. An event seen after a bootstrap started is newer than that
 * bootstrap's fetch, so bootstrap skips it (Ruling 83-F I1). Only these slices are tracked (Ruling 85); settings
 * keys are tracked per key by the settings store itself (`markSettingsBaseline`).
 */
type SeenSlice = 'session' | 'feed' | 'market' | 'clock';
const seenBy = new WeakMap<Backend, Set<SeenSlice>>();

const SIGNED_OUT: ReadonlySet<SessionStateName> = new Set(['logged_out', 'no_credentials']);

/**
 * The bridge owns the watchlists load (Ruling 102 I3): when the session is ready and the store is still idle (first
 * start, after the logout reset, or after a user change). The idle check keeps a view's mount-time fallback from
 * loading twice. The load goes through the typed helpers, which read the backend <MarketDataBridge> registers with
 * `setBackend` (Ruling 95 M5); the bridge does not register it.
 */
function loadWatchlistsIfIdle(): void {
  if (useWatchlists.getState().status !== 'idle') return;
  void useWatchlists.getState().load();
}

/** Subscribes once to every backend event and dispatches into stores (ARCHITECTURE §8.2). */
export function startEventBridge(backend: Backend, deps: BridgeDeps): () => void {
  const seen = new Set<SeenSlice>();
  seenBy.set(backend, seen);
  /**
   * The last non-empty session userId (Ruling 98). Go allows expired(A) → (logging_in →) ready(B) and a paste login
   * from expired without passing through logged_out / no_credentials, so a change of user also clears account data.
   * `null` = nothing seen by this bridge yet: fall back to the user bootstrap put in the session store. Empty after
   * a sign-out, whose own reset already cleared everything, so the next login does not reset again.
   */
  let lastUserId: string | null = null;
  const offs = [
    backend.on('session.state', (s) => {
      seen.add('session');
      const current = useSessionStore.getState().session;
      const previous = current?.state;
      const previousUser = lastUserId ?? (current && !SIGNED_OUT.has(current.state) ? current.userId : '');
      // A different user: clear the previous user's account data before their session becomes visible.
      if (s.userId !== '' && previousUser !== '' && s.userId !== previousUser) resetOnLogout();
      useSessionStore.getState().setSession(s);
      // Sign-out clears account data once per transition (M5); stores registered with onLogout: false keep theirs.
      if (previous !== undefined && !SIGNED_OUT.has(previous) && SIGNED_OUT.has(s.state)) resetOnLogout();
      lastUserId = SIGNED_OUT.has(s.state) ? '' : s.userId !== '' ? s.userId : previousUser;
      if (s.state === 'ready') loadWatchlistsIfIdle();
    }),
    backend.on('feed.state', (f) => {
      seen.add('feed');
      useStatusStore.getState().setFeed(f);
    }),
    backend.on('market.status', (m) => {
      seen.add('market');
      useStatusStore.getState().setMarket(m);
    }),
    backend.on('clock.offset', (c) => {
      seen.add('clock');
      useStatusStore.getState().setClock(c);
    }),
    backend.on('tick.batch', (t) => useTicksStore.getState().applyBatch(t)),
    // Go publishes the shortcut list under settings.changed key "shortcuts"; it is not a setting.
    backend.on('settings.changed', (c) =>
      c.key === SHORTCUTS_CHANGE_KEY ? applyShortcutsEvent(c.value) : useSettingsStore.getState().put(c.key, c.value)),
    backend.on('app.navigate', (n) => deps.navigate(n.route)),
    backend.on('job.progress', (p) => notifyUnclaimedJob(p)),
    backend.on('order.update', (o) => usePortfolioStore.getState().applyOrderUpdate(o)),
    backend.on('orders.changed', (b) => usePortfolioStore.getState().applySnapshot('orders', b)),
    backend.on('positions.changed', (p) => usePortfolioStore.getState().applySnapshot('positions', p)),
    backend.on('holdings.changed', (h) => usePortfolioStore.getState().applySnapshot('holdings', h)),
    backend.on('watchlists.changed', (w) => useWatchlists.getState().applyChanged(w)),
  ];
  return () => {
    for (const off of offs) off();
    if (seenBy.get(backend) === seen) seenBy.delete(backend);
  };
}

export interface BootstrapOptions { signal?: AbortSignal }
export interface BootstrapResult { failed: BootSlice[]; aborted: boolean }

/**
 * Loads session, status, settings and app info. Slices that received an event while the calls were in flight keep
 * the event's (newer) value; failed slices are recorded in `useAppStore.bootError` (settings stay `loaded: false`)
 * so the root layout can offer a retry; an aborted bootstrap (unmount) applies nothing.
 */
export async function bootstrap(backend: Backend, opts: BootstrapOptions = {}): Promise<BootstrapResult> {
  const seen = seenBy.get(backend) ?? new Set<SeenSlice>();
  seen.clear();
  markSettingsBaseline();
  const [session, status, settings, info] = await Promise.allSettled([
    backend.session.status(), backend.system.status(), backend.settings.all(), backend.system.info(),
  ]);
  if (opts.signal?.aborted) return { failed: [], aborted: true };

  if (session.status === 'fulfilled' && !seen.has('session')) useSessionStore.getState().setSession(session.value);
  if (status.status === 'fulfilled') {
    const st = useStatusStore.getState();
    if (!seen.has('feed')) st.setFeed(status.value.feed);
    if (!seen.has('market')) st.setMarket(status.value.market);
    if (!seen.has('clock')) st.setClock(status.value.clock);
  }
  if (settings.status === 'fulfilled') useSettingsStore.getState().hydrate(settings.value);
  if (info.status === 'fulfilled') useAppStore.getState().setInfo(info.value);
  if (useSessionStore.getState().session?.state === 'ready') loadWatchlistsIfIdle();

  const slices: [BootSlice, PromiseSettledResult<unknown>][] = [['session', session], ['status', status], ['settings', settings], ['info', info]];
  const rejected = slices.filter((s): s is [BootSlice, PromiseRejectedResult] => s[1].status === 'rejected');
  // Only this bootstrap's slices: the shortcut provider reports the `shortcuts` slice itself (Ruling 94 I3).
  useAppStore.getState().reportBoot(
    Object.fromEntries(slices.map(([slice, r]) => [slice, r.status === 'rejected' ? parseBackendError(r.reason).reason : null])),
  );
  return { failed: rejected.map(([slice]) => slice), aborted: false };
}
