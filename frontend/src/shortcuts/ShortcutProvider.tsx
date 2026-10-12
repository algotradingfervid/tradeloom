import { createContext, useContext, useEffect, useLayoutEffect, useMemo, useRef, type ReactNode } from 'react';
import type { Backend } from '../api/backend';
import { useBackend } from '../api/BackendContext';
import { isTypingTarget, resolveAction, scopeStackFor } from './registry';
import { useShortcutStore } from './store';

interface ShortcutContextValue { register(actionId: string, handler: () => void): () => void }

const ShortcutContext = createContext<ShortcutContextValue | null>(null);

/**
 * The registry lives in Go (ARCHITECTURE §8.8); the UI only mirrors it. A `shortcuts` event that arrives while the
 * fetch is in flight is newer, so the fetched list is then dropped.
 */
export async function reloadShortcuts(backend: Backend): Promise<void> {
  const before = useShortcutStore.getState().version;
  const actions = await backend.settings.shortcuts();
  if (useShortcutStore.getState().version === before) useShortcutStore.getState().setActions(actions);
}

export function ShortcutProvider({ children }: { children: ReactNode }) {
  const backend = useBackend();
  const handlers = useRef(new Map<string, () => void>());

  useEffect(() => {
    // On failure keep whatever list is present (empty, or one a shortcuts event delivered).
    reloadShortcuts(backend).catch(() => undefined);
  }, [backend]);

  useEffect(() => {
    const onKeyDown = (e: KeyboardEvent) => {
      if (e.defaultPrevented || e.repeat) return;
      const typing = isTypingTarget(e.target);
      if (typing && !e.ctrlKey && !e.altKey && !e.metaKey && !/^F\d{1,2}$/.test(e.key)) return;
      const action = resolveAction(useShortcutStore.getState().actions, scopeStackFor(document.activeElement), e, (id) => handlers.current.has(id));
      if (!action) return;
      e.preventDefault();
      handlers.current.get(action.id)?.();
    };
    window.addEventListener('keydown', onKeyDown);
    return () => window.removeEventListener('keydown', onKeyDown);
  }, []);

  const value = useMemo<ShortcutContextValue>(
    () => ({
      register: (actionId, handler) => {
        handlers.current.set(actionId, handler);
        return () => {
          if (handlers.current.get(actionId) === handler) handlers.current.delete(actionId);
        };
      },
    }),
    [],
  );

  return <ShortcutContext.Provider value={value}>{children}</ShortcutContext.Provider>;
}

export function useShortcut(actionId: string, handler: () => void): void {
  const ctx = useContext(ShortcutContext);
  if (!ctx) throw new Error('useShortcut must be used inside <ShortcutProvider>');
  const latest = useRef(handler);
  useLayoutEffect(() => {
    latest.current = handler;
  });
  useEffect(() => ctx.register(actionId, () => latest.current()), [ctx, actionId]);
}
