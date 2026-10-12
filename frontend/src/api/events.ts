import { useAppStore, type BootSlice } from '../store/app';
import { resetOnLogout } from '../store/reset';
import { notifyUnclaimedJob } from '../store/jobNotices';
import { useSessionStore } from '../store/session';
import { markSettingsBaseline, useSettingsStore } from '../store/settings';
import { useStatusStore } from '../store/status';
import { useTicksStore } from '../store/ticks';
import { useWatchlists } from '../store/watchlistsStore';
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

/** Subscribes once to every backend event and dispatches into stores (ARCHITECTURE §8.2). */
export function startEventBridge(backend: Backend, deps: BridgeDeps): () => void {
  const seen = new Set<SeenSlice>();
  seenBy.set(backend, seen);
  const offs = [
    backend.on('session.state', (s) => {
      seen.add('session');
      const previous = useSessionStore.getState().session?.state;
      useSessionStore.getState().setSession(s);
      // Sign-out clears account data once per transition (M5); stores registered with onLogout: false keep theirs.
      if (previous !== undefined && !SIGNED_OUT.has(previous) && SIGNED_OUT.has(s.state)) resetOnLogout();
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
    backend.on('settings.changed', (c) => useSettingsStore.getState().put(c.key, c.value)),
    backend.on('app.navigate', (n) => deps.navigate(n.route)),
    backend.on('job.progress', (p) => notifyUnclaimedJob(p)),
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

  const slices: [BootSlice, PromiseSettledResult<unknown>][] = [['session', session], ['status', status], ['settings', settings], ['info', info]];
  const rejected = slices.filter((s): s is [BootSlice, PromiseRejectedResult] => s[1].status === 'rejected');
  useAppStore.getState().setBootError(
    rejected.length === 0 ? null : { failed: rejected.map(([slice]) => slice), reason: parseBackendError(rejected[0][1].reason).reason },
  );
  return { failed: rejected.map(([slice]) => slice), aborted: false };
}
