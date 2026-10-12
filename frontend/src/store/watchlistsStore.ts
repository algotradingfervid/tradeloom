import { create } from 'zustand';
import type { RemovedItem, Watchlist, WatchlistsState } from '../api/backend';
import { BindingError } from '../api/call';
import { watchlistsApi } from '../api/watchlists';
import { validateWatchlistName } from '../views/watchlists/names';
import { registerReset } from './reset';
import { useSessionStore } from './session';

export type { RemovedItem, Watchlist };
type Status = 'idle' | 'loading' | 'ready' | 'error';

interface WatchlistsStore {
  lists: Watchlist[];
  activeId: string | null;
  selectedToken: number | null;
  status: Status;
  error: string | null;
  load(): Promise<void>;
  setActive(id: string): Promise<void>;
  select(token: number | null): void;
  create(name: string, tokens?: readonly number[], activate?: boolean): Promise<string | null>;
  rename(id: string, name: string): Promise<string | null>;
  deleteList(id: string): Promise<void>;
  /** 'cancelled': the call finished after a reset (sign-out or user change); the caller shows nothing. */
  addItem(token: number): Promise<'added' | 'duplicate' | 'cancelled'>;
  addItems(tokens: readonly number[]): Promise<number>;
  removeItem(token: number): Promise<RemovedItem | null>;
  restoreItem(r: RemovedItem): Promise<void>;
  move(token: number, delta: -1 | 1): Promise<void>;
  moveTo(token: number, index: number): Promise<void>;
  /** Applies Go's `watchlists.changed` (published after every mutation, incl. SetActive; Ruling 87). */
  applyChanged(st: WatchlistsState): void;
}

export const activeList = (s: Pick<WatchlistsStore, 'lists' | 'activeId'>): Watchlist | undefined =>
  s.lists.find((l) => l.id === s.activeId) ?? s.lists[0];

export const tokensOf = (l: Watchlist): number[] => l.items.map((i) => i.token);

const DUPLICATE = 'A watchlist with this name exists';
const isCode = (e: unknown, code: string): boolean => e instanceof BindingError && e.code === code;
const message = (e: unknown): string => (e instanceof Error ? e.message : String(e));
/** Ruling 87 refusal codes: duplicate_name and invalid_name show the same text as the local validator. */
const nameError = (e: unknown, name: string): string => {
  if (isCode(e, 'duplicate_name')) return DUPLICATE;
  if (isCode(e, 'invalid_name')) return validateWatchlistName(name, []) ?? message(e);
  return message(e);
};
const patch = (lists: Watchlist[], id: string, f: (l: Watchlist) => Watchlist) => lists.map((l) => (l.id === id ? f(l) : l));
const resolveActive = (st: WatchlistsState): string | null =>
  st.lists.some((l) => l.id === st.activeId) ? st.activeId : (st.lists[0]?.id ?? null);

/**
 * Late-response guard (P0-4 Rulings 90/96, 102 I1): the logout reset bumps `epoch`; every call captures the epoch and
 * the last-known user before its await and drops its result when either changed. Watchlists are install-wide and Go
 * does not gate them on the session, so a session that merely expires (ready(A) → expired(A)) cancels nothing; only a
 * reset (logout) or a different user does. A dropped call resolves quietly (no rethrow, no error text; Ruling 102 I2). `eventSeq` counts applied
 * `watchlists.changed` events: a response is not applied over an event that arrived while it was in flight, because
 * Go published that event after (or for) the same mutation and it is at least as new.
 */
let epoch = 0;
let eventSeq = 0;
/** The last non-empty session userId, whatever the session state (cleared by the reset). */
let lastUserId: string | null = null;
const noteUser = (userId: string | undefined) => {
  if (userId) lastUserId = userId;
};
noteUser(useSessionStore.getState().session?.userId);
useSessionStore.subscribe((s) => noteUser(s.session?.userId));

const initial = { lists: [] as Watchlist[], activeId: null, selectedToken: null, status: 'idle' as Status, error: null };

export const useWatchlists = create<WatchlistsStore>()((set, get) => {
  const ticket = () => {
    const e = epoch;
    const user = lastUserId;
    const seq = eventSeq;
    // A ticket taken before any user was known (null) survives the first sign-in (null → A; Ruling 104 m2).
    const live = () => e === epoch && (user === null || user === lastUserId);
    return {
      live,
      wasReset: () => e !== epoch,
      superseded: () => seq !== eventSeq,
      apply: (st: WatchlistsState) => {
        if (live() && seq === eventSeq) set({ lists: st.lists, activeId: resolveActive(st) });
      },
    };
  };

  /** Re-reads Go's state without the loading status (after a refused mutation). */
  const refetch = async () => {
    const t = ticket();
    t.apply(await watchlistsApi.list());
  };

  /**
   * `target(list)` gives the row's wanted index in `list`; it is asked again against Go's reloaded list on the
   * stale_order retry (Ruling 102 m1), so the move keeps its meaning when rows changed elsewhere.
   */
  const reorderTo = async (token: number, target: (l: Watchlist) => number, retried: boolean): Promise<void> => {
    const list = activeList(get());
    if (!list) return;
    const from = list.items.findIndex((i) => i.token === token);
    const to = Math.max(0, Math.min(target(list), list.items.length - 1));
    if (from < 0 || from === to) return;
    const items = [...list.items];
    const [moved] = items.splice(from, 1);
    items.splice(to, 0, moved);
    set((s) => ({ lists: patch(s.lists, list.id, (l) => ({ ...l, items })) }));
    const t = ticket();
    try {
      t.apply(await watchlistsApi.reorder(list.id, items.map((i) => i.token)));
    } catch (e) {
      if (!t.live()) return;
      // A failed reload must not hide the original error (Ruling 102 m2).
      await refetch().catch(() => undefined);
      if (!t.live()) return;
      // Ruling 87: stale_order means Go's list changed under us — reload and retry once against it.
      if (isCode(e, 'stale_order') && !retried) return reorderTo(token, target, true);
      throw e;
    }
  };

  return {
    ...initial,

    load: async () => {
      set({ status: 'loading', error: null });
      const t = ticket();
      // Dropped by a user change without a reset: back to idle so the bridge (or a view) can load again.
      const dropped = () => {
        if (!t.wasReset() && get().status === 'loading') set({ status: 'idle' });
      };
      try {
        const st = await watchlistsApi.list();
        if (!t.live()) return dropped();
        t.apply(st);
        set({ status: 'ready' });
      } catch (e) {
        if (!t.live()) return dropped();
        set(t.superseded() ? { status: 'ready' } : { status: 'error', error: message(e) });
      }
    },

    setActive: async (id) => {
      const previous = get().activeId;
      set({ activeId: id });
      const t = ticket();
      try {
        t.apply(await watchlistsApi.setActive(id));
      } catch (e) {
        if (!t.live()) return;
        if (get().activeId === id) set({ activeId: previous });
        throw e;
      }
    },

    select: (token) => set({ selectedToken: token }),

    create: async (name, tokens = [], activate = true) => {
      const err = validateWatchlistName(name, get().lists.map((l) => l.name));
      if (err) return err;
      const wanted = name.trim();
      const previous = get().activeId;
      const t = ticket();
      let createdId: string | null = null;
      try {
        // Go's Create activates the new list (Ruling 87), so its activeId names the list just created.
        let st = await watchlistsApi.create(wanted);
        if (!t.live()) return null;
        const created = st.lists.find((l) => l.id === st.activeId) ?? st.lists.find((l) => l.name.toLowerCase() === wanted.toLowerCase());
        if (!created) return 'The watchlist could not be created';
        createdId = created.id;
        for (const token of tokens) {
          try {
            st = await watchlistsApi.add(created.id, token, -1);
          } catch (e) {
            if (!isCode(e, 'already_in_list')) throw e;
          }
          if (!t.live()) return null;
        }
        // "Save as watchlist" without activation puts the previous list back (Ruling 87).
        if (!activate && previous && previous !== created.id && st.lists.some((l) => l.id === previous)) {
          try {
            st = await watchlistsApi.setActive(previous);
          } catch {
            // The list exists with its items; only the switch back failed. Show Go's state, report success (m3).
            if (t.live()) await refetch().catch(() => undefined);
            return null;
          }
        }
        t.apply(st);
        return null;
      } catch (e) {
        if (!t.live()) return null;
        if (createdId !== null) {
          if (!activate && previous) await watchlistsApi.setActive(previous).catch(() => undefined);
          await refetch().catch(() => undefined);
        }
        return nameError(e, wanted);
      }
    },

    rename: async (id, name) => {
      const err = validateWatchlistName(name, get().lists.filter((l) => l.id !== id).map((l) => l.name));
      if (err) return err;
      const t = ticket();
      try {
        t.apply(await watchlistsApi.rename(id, name.trim()));
        return null;
      } catch (e) {
        if (!t.live()) return null;
        return nameError(e, name);
      }
    },

    // Go picks the next active list (the one at the same index; a fresh "Watchlist 1" when none is left).
    deleteList: async (id) => {
      const t = ticket();
      try {
        t.apply(await watchlistsApi.delete(id));
      } catch (e) {
        if (t.live()) throw e;
      }
    },

    addItem: async (token) => {
      const list = activeList(get());
      if (!list) throw new Error('No watchlist');
      if (list.items.some((i) => i.token === token)) return 'duplicate';
      const t = ticket();
      try {
        t.apply(await watchlistsApi.add(list.id, token, -1));
        return t.live() ? 'added' : 'cancelled';
      } catch (e) {
        if (!t.live()) return 'cancelled';
        if (isCode(e, 'already_in_list')) return 'duplicate';
        throw e;
      }
    },

    addItems: async (tokens) => {
      const t = ticket();
      let added = 0;
      for (const token of tokens) {
        if (!t.live()) break; // a reset between tokens ends the batch quietly (Ruling 104 I1)
        if ((await get().addItem(token)) === 'added') added++;
      }
      return added;
    },

    removeItem: async (token) => {
      const list = activeList(get());
      const position = list ? list.items.findIndex((i) => i.token === token) : -1;
      if (!list || position < 0) return null;
      const item = list.items[position];
      set((s) => ({
        lists: patch(s.lists, list.id, (l) => ({ ...l, items: l.items.filter((i) => i.token !== token) })),
        selectedToken: s.selectedToken === token ? null : s.selectedToken,
      }));
      const t = ticket();
      try {
        const removed = await watchlistsApi.remove(list.id, token);
        return t.live() ? removed : null; // no Undo after a sign-out (Ruling 104 m3)
      } catch (e) {
        if (!t.live()) return null;
        set((s) => ({
          lists: patch(s.lists, list.id, (l) => {
            if (l.items.some((i) => i.token === token)) return l;
            const items = [...l.items];
            items.splice(Math.min(position, items.length), 0, item);
            return { ...l, items };
          }),
        }));
        throw e;
      }
    },

    restoreItem: async (r) => {
      const list = get().lists.find((l) => l.id === r.listId);
      if (!list || list.items.some((i) => i.token === r.token)) return;
      const t = ticket();
      try {
        t.apply(await watchlistsApi.add(r.listId, r.token, r.position));
      } catch (e) {
        if (t.live()) throw e;
      }
    },

    // One step from wherever the row is now, also in Go's reloaded list on a retry.
    move: (token, delta) => reorderTo(token, (l) => l.items.findIndex((i) => i.token === token) + delta, false),

    // A drop: on a retry the row goes next to the neighbour it was dropped beside, else to the clamped index.
    moveTo: (token, index) => {
      const list = activeList(get());
      if (!list) return Promise.resolve();
      const order = tokensOf(list).filter((t) => t !== token);
      const at = Math.max(0, Math.min(index, order.length));
      const above = at > 0 ? order[at - 1] : null;
      const below = at < order.length ? order[at] : null;
      let first = true;
      return reorderTo(token, (l) => {
        if (first) {
          first = false;
          return index;
        }
        const rest = tokensOf(l).filter((t) => t !== token);
        if (above !== null && rest.includes(above)) return rest.indexOf(above) + 1;
        if (below !== null && rest.includes(below)) return rest.indexOf(below);
        return index;
      }, false);
    },

    applyChanged: (st) => {
      // Before the first load (or after the logout reset) there is nothing to update: load() reads Go's state.
      if (get().status === 'idle') return;
      eventSeq++;
      set({ lists: st.lists, activeId: resolveActive(st), status: 'ready', error: null });
    },
  };
});

// Watchlists are account data: cleared on sign-out (default onLogout, Ruling 83-F M5), and in-flight calls dropped.
registerReset(() => {
  epoch++;
  lastUserId = null;
  useWatchlists.setState(initial);
}, { inspect: () => useWatchlists.getState() });
