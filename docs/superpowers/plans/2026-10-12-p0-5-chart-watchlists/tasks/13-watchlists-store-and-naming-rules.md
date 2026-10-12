> **P0-5 Chart, Watchlists & Ticker UI Implementation Plan — Task 13.** Read [`../MANIFEST.md`](../MANIFEST.md) first: its Global Constraints,
> Review Focus and Consumes/Produces apply to this task. Other tasks live beside this file in `tasks/`.

<!-- task body (verbatim from the plan) -->
### Task 13: Watchlists store and naming rules

The store mirrors Go's canonical `WatchlistsState` (`lists[{ id, name, items[{ token, exchange, symbol, displayName, name, isIndex, missing }] }]`, `activeId`). Every mutation applies the state Go returns, so the UI never drifts from `app.db`. Items carry their display data, so rows need no instrument lookup.

**Files:**
- Create: `frontend/src/views/watchlists/names.ts`, `frontend/src/store/watchlistsStore.ts`
- Test: `frontend/src/views/watchlists/names.test.ts`, `frontend/src/store/watchlistsStore.test.ts`

**Interfaces:**
- Consumes: Task 2 `watchlistsApi`, `BindingError`, `setBackend`, `Watchlist`, `WatchlistItem`, `RemovedItem` (backend types), mock `watchlistItemOf`; P0-4 `createMockBackend`, `demoSeed`, `MockBackend.failNext/callsTo`.
- Produces:
  - `nextWatchlistName(existing: readonly string[]): string`, `validateWatchlistName(name: string, others: readonly string[]): string | null`, `WATCHLIST_NAME_MAX = 40`
  - `useWatchlists` (zustand) with state `{ lists: Watchlist[]; activeId: string | null; selectedToken: number | null; status: 'idle'|'loading'|'ready'|'error'; error: string | null }` and actions `load()`, `setActive(id)`, `select(token|null)`, `create(name, tokens?, activate?): Promise<string|null>` (returns an error message or null; seeded create = `Create(name)` then `Add(id, token, -1)` per token), `rename(id, name): Promise<string|null>`, `deleteList(id)`, `addItem(token): Promise<'added'|'duplicate'>`, `addItems(tokens): Promise<number>`, `removeItem(token): Promise<RemovedItem|null>`, `restoreItem(r)`, `move(token, -1|1)`, `moveTo(token, index)`
  - `activeList(s): Watchlist | undefined`, `tokensOf(list): number[]`

- [ ] **Step 1: Write the failing tests**

`frontend/src/views/watchlists/names.test.ts`:

```ts
import { describe, expect, it } from 'vitest';
import { nextWatchlistName, validateWatchlistName } from './names';

describe('watchlist names (FR-WATCH-002)', () => {
  it('AC1: lowest unused "Watchlist N"', () => {
    expect(nextWatchlistName(['Watchlist 1', 'Watchlist 2'])).toBe('Watchlist 3');
    expect(nextWatchlistName(['Watchlist 1', 'watchlist 3'])).toBe('Watchlist 2');
    expect(nextWatchlistName([])).toBe('Watchlist 1');
  });
  it('AC2/AC3: trimmed, 1–40 chars, unique case-insensitively', () => {
    expect(validateWatchlistName('   ', [])).toBe('Enter a name');
    expect(validateWatchlistName('x'.repeat(41), [])).toBe('Use 40 characters or fewer');
    expect(validateWatchlistName(' watchlist 1 ', ['Watchlist 1'])).toBe('A watchlist with this name exists');
    expect(validateWatchlistName('Banks', ['Watchlist 1'])).toBeNull();
  });
});
```

`frontend/src/store/watchlistsStore.test.ts` (runs against P0-4's mock backend with this plan's watchlist domain):

```ts
import { beforeEach, describe, expect, it } from 'vitest';
import type { Watchlist } from '../api/backend';
import { setBackend } from '../api/current';
import { demoSeed } from '../api/mock/fixtures';
import { createMockBackend, type MockBackend } from '../api/mock/index';
import { watchlistItemOf } from '../api/mock/market';
import { activeList, tokensOf, useWatchlists } from './watchlistsStore';
import { RELIANCE, SBIN, TCS } from '../instruments/fixtures';
import type { Instrument } from '../instruments/types';

let backend: MockBackend;
const wl = (id: string, name: string, insts: Instrument[] = []): Watchlist => ({ id, name, items: insts.map(watchlistItemOf) });
const seed = (lists: Watchlist[], activeId = lists[0]?.id ?? '') => {
  backend.state.watchlists = JSON.parse(JSON.stringify({ lists, activeId }));
  useWatchlists.setState({ lists, activeId, selectedToken: null, status: 'ready', error: null });
};
const state = () => useWatchlists.getState();

beforeEach(() => {
  backend = createMockBackend(demoSeed());
  setBackend(backend);
  useWatchlists.setState({ lists: [], activeId: null, selectedToken: null, status: 'idle', error: null });
});

describe('watchlistsStore', () => {
  it('load uses Go\'s activeId, falling back to the first list', async () => {
    backend.state.watchlists = { lists: [wl('a', 'Watchlist 1'), wl('b', 'Watchlist 2')], activeId: 'b' };
    await state().load();
    expect(state()).toMatchObject({ activeId: 'b', status: 'ready' });
    backend.state.watchlists.activeId = 'gone';
    await state().load();
    expect(state().activeId).toBe('a');
  });

  it('FR-WATCH-002 AC1: create makes the new list active (Create, then SetActive)', async () => {
    seed([wl('a', 'Watchlist 1'), wl('b', 'Watchlist 2')]);
    expect(await state().create('Watchlist 3')).toBeNull();
    expect(backend.callsTo('watchlists.create')[0].args).toEqual(['Watchlist 3']);
    const created = activeList(state())!;
    expect(created.name).toBe('Watchlist 3');
    expect(backend.callsTo('watchlists.setActive').at(-1)!.args).toEqual([created.id]);
    expect(backend.state.watchlists.activeId).toBe(created.id);
  });

  it('a seeded create is Create(name) followed by Add(id, token, -1) per token, without switching the active list', async () => {
    seed([wl('a', 'Watchlist 1')]);
    expect(await state().create('Banks', [SBIN.token, RELIANCE.token], false)).toBeNull();
    const banks = state().lists.find((l) => l.name === 'Banks')!;
    expect(backend.callsTo('watchlists.add').map((c) => c.args)).toEqual([[banks.id, SBIN.token, -1], [banks.id, RELIANCE.token, -1]]);
    expect(banks.items.map((i) => i.symbol)).toEqual(['SBIN', 'RELIANCE']);
    expect(state().activeId).toBe('a');
  });

  it('FR-WATCH-002 AC2/AC3: duplicate or empty names are refused before calling Go; Go-side name_exists maps to the same text', async () => {
    seed([wl('a', 'Watchlist 1'), wl('b', 'Banks')]);
    expect(await state().rename('b', 'watchlist 1')).toBe('A watchlist with this name exists');
    expect(await state().create('')).toBe('Enter a name');
    expect(backend.callsTo('watchlists.rename')).toHaveLength(0);
    expect(backend.callsTo('watchlists.create')).toHaveLength(0);
    backend.failNext('watchlists.create', 'dup', 'name_exists');
    expect(await state().create('Other')).toBe('A watchlist with this name exists');
    expect(state().lists).toHaveLength(2);
  });

  it('FR-WATCH-003 AC2: deleting the only list leaves a fresh empty "Watchlist 1" active', async () => {
    seed([wl('a', 'Mine', [SBIN])]);
    await state().deleteList('a');
    expect(state().lists.map((l) => [l.name, l.items.length])).toEqual([['Watchlist 1', 0]]);
    expect(state().activeId).toBe(state().lists[0].id);
  });

  it('FR-WATCH-003: deleting the active middle list activates the adjacent one', async () => {
    seed([wl('a', 'A'), wl('b', 'B'), wl('c', 'C')], 'b');
    await state().deleteList('b');
    expect(state().activeId).toBe('c');
    expect(backend.state.watchlists.activeId).toBe('c');
  });

  it('FR-WATCH-004: add appends at the end; duplicates (local or Go already_in_list) report "duplicate"', async () => {
    seed([wl('a', 'Watchlist 1', [SBIN])]);
    expect(await state().addItem(SBIN.token)).toBe('duplicate');
    expect(await state().addItem(TCS.token)).toBe('added');
    expect(tokensOf(state().lists[0])).toEqual([SBIN.token, TCS.token]);
    expect(state().lists[0].items[1]).toMatchObject({ symbol: 'TCS', exchange: 'NSE', missing: false });
    backend.state.watchlists.lists[0].items.push(watchlistItemOf(RELIANCE)); // Go already has it, the UI does not yet
    expect(await state().addItem(RELIANCE.token)).toBe('duplicate');
  });

  it('FR-WATCH-005 AC3: remove then restore puts TCS back at its position', async () => {
    seed([wl('a', 'Watchlist 1', [SBIN, TCS, RELIANCE])]);
    const r = (await state().removeItem(TCS.token))!;
    expect(r).toEqual({ listId: 'a', token: TCS.token, position: 1 });
    expect(tokensOf(state().lists[0])).toEqual([SBIN.token, RELIANCE.token]);
    await state().restoreItem(r);
    expect(backend.callsTo('watchlists.add').at(-1)!.args).toEqual(['a', TCS.token, 1]);
    expect(tokensOf(state().lists[0])).toEqual([SBIN.token, TCS.token, RELIANCE.token]);
  });

  it('FR-WATCH-001 AC4: move TCS above SBIN persists the new order', async () => {
    seed([wl('a', 'Watchlist 1', [SBIN, TCS])]);
    await state().move(TCS.token, -1);
    expect(backend.callsTo('watchlists.reorder')[0].args).toEqual(['a', [TCS.token, SBIN.token]]);
    await state().move(TCS.token, -1); // already first: no-op
    expect(backend.callsTo('watchlists.reorder')).toHaveLength(1);
    await state().moveTo(TCS.token, 99);
    expect(tokensOf(state().lists[0])).toEqual([SBIN.token, TCS.token]);
  });
});
```

- [ ] **Step 2: Run the tests to check they fail**

Run: `cd frontend && npx vitest run src/views/watchlists/names.test.ts src/store/watchlistsStore.test.ts`
Expected: FAIL with `Failed to resolve import "./names"` / `"./watchlistsStore"`.

- [ ] **Step 3: Implement**

`frontend/src/views/watchlists/names.ts`:

```ts
export const WATCHLIST_NAME_MAX = 40;

export function nextWatchlistName(existing: readonly string[]): string {
  const used = new Set(existing.map((n) => n.trim().toLowerCase()));
  for (let n = 1; ; n++) {
    const candidate = `Watchlist ${n}`;
    if (!used.has(candidate.toLowerCase())) return candidate;
  }
}

export function validateWatchlistName(name: string, others: readonly string[]): string | null {
  const t = name.trim();
  if (t.length === 0) return 'Enter a name';
  if (t.length > WATCHLIST_NAME_MAX) return `Use ${WATCHLIST_NAME_MAX} characters or fewer`;
  if (others.some((o) => o.trim().toLowerCase() === t.toLowerCase())) return 'A watchlist with this name exists';
  return null;
}
```

`frontend/src/store/watchlistsStore.ts`:

```ts
import { create } from 'zustand';
import type { RemovedItem, Watchlist } from '../api/backend';
import { BindingError } from '../api/call';
import { watchlistsApi } from '../api/watchlists';
import { validateWatchlistName } from '../views/watchlists/names';

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
  addItem(token: number): Promise<'added' | 'duplicate'>;
  addItems(tokens: readonly number[]): Promise<number>;
  removeItem(token: number): Promise<RemovedItem | null>;
  restoreItem(r: RemovedItem): Promise<void>;
  move(token: number, delta: -1 | 1): Promise<void>;
  moveTo(token: number, index: number): Promise<void>;
}

export const activeList = (s: Pick<WatchlistsStore, 'lists' | 'activeId'>): Watchlist | undefined =>
  s.lists.find((l) => l.id === s.activeId) ?? s.lists[0];

export const tokensOf = (l: Watchlist): number[] => l.items.map((i) => i.token);

const isCode = (e: unknown, code: string): boolean => e instanceof BindingError && e.code === code;
const nameError = (e: unknown): string =>
  isCode(e, 'name_exists') ? 'A watchlist with this name exists' : e instanceof Error ? e.message : String(e);
const patch = (lists: Watchlist[], id: string, f: (l: Watchlist) => Watchlist) => lists.map((l) => (l.id === id ? f(l) : l));

export const useWatchlists = create<WatchlistsStore>()((set, get) => ({
  lists: [],
  activeId: null,
  selectedToken: null,
  status: 'idle',
  error: null,

  load: async () => {
    set({ status: 'loading', error: null });
    try {
      const st = await watchlistsApi.list();
      const activeId = st.lists.some((l) => l.id === st.activeId) ? st.activeId : st.lists[0]?.id ?? null;
      set({ lists: st.lists, activeId, status: 'ready' });
    } catch (e) {
      set({ status: 'error', error: e instanceof Error ? e.message : String(e) });
    }
  },

  setActive: async (id) => {
    set({ activeId: id });
    const st = await watchlistsApi.setActive(id);
    set({ lists: st.lists });
  },

  select: (token) => set({ selectedToken: token }),

  create: async (name, tokens = [], activate = true) => {
    const err = validateWatchlistName(name, get().lists.map((l) => l.name));
    if (err) return err;
    const wanted = name.trim();
    try {
      let st = await watchlistsApi.create(wanted);
      const created = st.lists.find((l) => l.name.toLowerCase() === wanted.toLowerCase());
      if (!created) return 'The watchlist could not be created';
      for (const t of tokens) {
        try {
          st = await watchlistsApi.add(created.id, t, -1);
        } catch (e) {
          if (!isCode(e, 'already_in_list')) throw e;
        }
      }
      set({ lists: st.lists });
      if (activate) await get().setActive(created.id);
      return null;
    } catch (e) {
      return nameError(e);
    }
  },

  rename: async (id, name) => {
    const err = validateWatchlistName(name, get().lists.filter((l) => l.id !== id).map((l) => l.name));
    if (err) return err;
    try {
      const st = await watchlistsApi.rename(id, name.trim());
      set({ lists: st.lists });
      return null;
    } catch (e) {
      return nameError(e);
    }
  },

  deleteList: async (id) => {
    const idx = get().lists.findIndex((l) => l.id === id);
    const st = await watchlistsApi.delete(id);
    const next = st.lists[Math.min(Math.max(idx, 0), st.lists.length - 1)];
    set({ lists: st.lists, activeId: st.activeId || (next?.id ?? null) });
    if (next && next.id !== st.activeId) await get().setActive(next.id);
  },

  addItem: async (token) => {
    const list = activeList(get());
    if (!list) throw new Error('No watchlist');
    if (list.items.some((i) => i.token === token)) return 'duplicate';
    try {
      const st = await watchlistsApi.add(list.id, token, -1);
      set({ lists: st.lists });
      return 'added';
    } catch (e) {
      if (isCode(e, 'already_in_list')) return 'duplicate';
      throw e;
    }
  },

  addItems: async (tokens) => {
    let added = 0;
    for (const t of tokens) if ((await get().addItem(t)) === 'added') added++;
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
    try {
      return await watchlistsApi.remove(list.id, token);
    } catch (e) {
      set((s) => ({
        lists: patch(s.lists, list.id, (l) => {
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
    const st = await watchlistsApi.add(r.listId, r.token, r.position);
    set({ lists: st.lists });
  },

  move: async (token, delta) => {
    const list = activeList(get());
    if (!list) return;
    await get().moveTo(token, tokensOf(list).indexOf(token) + delta);
  },

  moveTo: async (token, index) => {
    const list = activeList(get());
    if (!list) return;
    const from = list.items.findIndex((i) => i.token === token);
    const to = Math.max(0, Math.min(index, list.items.length - 1));
    if (from < 0 || from === to) return;
    const items = [...list.items];
    const [moved] = items.splice(from, 1);
    items.splice(to, 0, moved);
    set((s) => ({ lists: patch(s.lists, list.id, (l) => ({ ...l, items })) }));
    const st = await watchlistsApi.reorder(list.id, items.map((i) => i.token));
    set({ lists: st.lists });
  },
}));
```

- [ ] **Step 4: Run the tests to check they pass**

Run: `cd frontend && npx vitest run src/views/watchlists/names.test.ts src/store/watchlistsStore.test.ts`
Expected: PASS (2 + 9 tests).

- [ ] **Step 5: Commit**

```bash
git add frontend/src/views/watchlists/names.ts frontend/src/views/watchlists/names.test.ts frontend/src/store/watchlistsStore.ts frontend/src/store/watchlistsStore.test.ts
git commit -m "$(cat <<'EOF'
feat(watchlists): store over Go's WatchlistsState with create/rename/delete, add/remove/undo, reorder; naming rules

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
)"
```

---