import type { RemovedItem, WatchlistsState } from './backend';
import { call } from './call';
import { currentBackend } from './current';

const w = () => currentBackend().watchlists;

/** Every mutation resolves to Go's new WatchlistsState (Remove resolves to the RemovedItem used by Undo). */
export const watchlistsApi = {
  list: (): Promise<WatchlistsState> => call(w().list()),
  create: (name: string): Promise<WatchlistsState> => call(w().create(name)),
  rename: (id: string, name: string): Promise<WatchlistsState> => call(w().rename(id, name)),
  delete: (id: string): Promise<WatchlistsState> => call(w().delete(id)),
  setActive: (id: string): Promise<WatchlistsState> => call(w().setActive(id)),
  /** position −1 appends. */
  add: (id: string, token: number, position = -1): Promise<WatchlistsState> => call(w().add(id, token, position)),
  remove: (id: string, token: number): Promise<RemovedItem> => call(w().remove(id, token)),
  reorder: (id: string, tokens: readonly number[]): Promise<WatchlistsState> => call(w().reorder(id, [...tokens])),
  reorderLists: (ids: readonly string[]): Promise<WatchlistsState> => call(w().reorderLists([...ids])),
};
