import { useAppStore } from '../store/app';
import { useSessionStore } from '../store/session';
import { useSettingsStore } from '../store/settings';
import { useStatusStore } from '../store/status';
import { useTicksStore } from '../store/ticks';
import type { Backend } from './backend';

export interface BridgeDeps { navigate: (route: string) => void }

/** Subscribes once to every backend event and dispatches into stores (ARCHITECTURE §8.2). */
export function startEventBridge(backend: Backend, deps: BridgeDeps): () => void {
  const offs = [
    backend.on('session.state', (s) => useSessionStore.getState().setSession(s)),
    backend.on('feed.state', (f) => useStatusStore.getState().setFeed(f)),
    backend.on('market.status', (m) => useStatusStore.getState().setMarket(m)),
    backend.on('clock.offset', (c) => useStatusStore.getState().setClock(c)),
    backend.on('tick.batch', (t) => useTicksStore.getState().applyBatch(t)),
    backend.on('settings.changed', (c) => useSettingsStore.getState().put(c.key, c.value)),
    backend.on('app.navigate', (n) => deps.navigate(n.route)),
  ];
  return () => {
    for (const off of offs) off();
  };
}

export async function bootstrap(backend: Backend): Promise<void> {
  const [session, status, settings, info] = await Promise.allSettled([
    backend.session.status(), backend.system.status(), backend.settings.all(), backend.system.info(),
  ]);
  if (session.status === 'fulfilled') useSessionStore.getState().setSession(session.value);
  if (status.status === 'fulfilled') useStatusStore.getState().setSnapshot(status.value);
  useSettingsStore.getState().hydrate(settings.status === 'fulfilled' ? settings.value : []);
  if (info.status === 'fulfilled') useAppStore.getState().setInfo(info.value);
}
