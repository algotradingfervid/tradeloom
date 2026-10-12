import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { Watchlist, WatchlistsState } from '../api/backend';
import { setBackend } from '../api/current';
import { bootstrap, startEventBridge } from '../api/events';
import { demoSeed, readySession } from '../api/mock/fixtures';
import { createMockBackend, type MockBackend } from '../api/mock/index';
import { watchlistItemOf } from '../api/mock/market';
import { activeList, tokensOf, useWatchlists } from './watchlistsStore';
import { resetOnLogout } from './reset';
import { useSessionStore } from './session';
import { INFY, RELIANCE, SBIN, TCS } from '../instruments/fixtures';
import type { Instrument } from '../instruments/types';

let backend: MockBackend;
const wl = (id: string, name: string, insts: Instrument[] = []): Watchlist => ({ id, name, items: insts.map(watchlistItemOf) });
const seed = (lists: Watchlist[], activeId = lists[0]?.id ?? '') => {
  backend.state.watchlists = JSON.parse(JSON.stringify({ lists, activeId }));
  useWatchlists.setState({ lists, activeId, selectedToken: null, status: 'ready', error: null });
};
const state = () => useWatchlists.getState();
const refusal = (code: string) => new Error(JSON.stringify({ code, stage: 'watchlists', reason: code }));
function deferred<T>() {
  let resolve!: (v: T) => void;
  const promise = new Promise<T>((r) => { resolve = r; });
  return { promise, resolve };
}

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

  it('FR-WATCH-002 AC1: create makes the new list active (Go\'s Create activates it; Ruling 87)', async () => {
    seed([wl('a', 'Watchlist 1'), wl('b', 'Watchlist 2')]);
    expect(await state().create('Watchlist 3')).toBeNull();
    expect(backend.callsTo('watchlists.create')[0].args).toEqual(['Watchlist 3']);
    const created = activeList(state())!;
    expect(created.name).toBe('Watchlist 3');
    expect(state().activeId).toBe(created.id);
    expect(backend.state.watchlists.activeId).toBe(created.id);
    expect(backend.callsTo('watchlists.setActive')).toHaveLength(0);
  });

  it('a seeded create is Create(name) then Add(id, token, -1) per token; without activation it calls setActive(previous)', async () => {
    seed([wl('a', 'Watchlist 1')]);
    expect(await state().create('Banks', [SBIN.token, RELIANCE.token], false)).toBeNull();
    const banks = state().lists.find((l) => l.name === 'Banks')!;
    expect(backend.callsTo('watchlists.add').map((c) => c.args)).toEqual([[banks.id, SBIN.token, -1], [banks.id, RELIANCE.token, -1]]);
    expect(banks.items.map((i) => i.symbol)).toEqual(['SBIN', 'RELIANCE']);
    expect(backend.callsTo('watchlists.setActive').map((c) => c.args)).toEqual([['a']]);
    expect(state().activeId).toBe('a');
    expect(backend.state.watchlists.activeId).toBe('a');
  });

  it('FR-WATCH-002 AC2/AC3: duplicate or empty names are refused before calling Go; Go-side duplicate_name/invalid_name map to the same text', async () => {
    seed([wl('a', 'Watchlist 1'), wl('b', 'Banks')]);
    expect(await state().rename('b', 'watchlist 1')).toBe('A watchlist with this name exists');
    expect(await state().create('')).toBe('Enter a name');
    expect(backend.callsTo('watchlists.rename')).toHaveLength(0);
    expect(backend.callsTo('watchlists.create')).toHaveLength(0);
    backend.failNext('watchlists.create', 'dup', 'duplicate_name');
    expect(await state().create('Other')).toBe('A watchlist with this name exists');
    expect(state().lists).toHaveLength(2);
    backend.failNext('watchlists.rename', 'Watchlist names can be up to 40 characters.', 'invalid_name');
    expect(await state().rename('b', 'Fine locally')).toBe('Watchlist names can be up to 40 characters.');
    // Go's real check on the same rune-counted rule reaches the same text as the local validator.
    backend.state.watchlists.lists.push(wl('c', 'Taken'));
    expect(await state().rename('b', 'TAKEN')).toBe('A watchlist with this name exists');
    expect(state().lists.find((l) => l.id === 'b')!.name).toBe('Banks');
  });

  it('FR-WATCH-003 AC2: deleting the only list leaves a fresh empty "Watchlist 1" active', async () => {
    seed([wl('a', 'Mine', [SBIN])]);
    await state().deleteList('a');
    expect(state().lists.map((l) => [l.name, l.items.length])).toEqual([['Watchlist 1', 0]]);
    expect(state().activeId).toBe(state().lists[0].id);
  });

  it('FR-WATCH-003: deleting the active middle list activates the one at the same index', async () => {
    seed([wl('a', 'A'), wl('b', 'B'), wl('c', 'C')], 'b');
    await state().deleteList('b');
    expect(state().activeId).toBe('c');
    expect(backend.state.watchlists.activeId).toBe('c');
  });

  it('FR-WATCH-003: deleting a list that is not active keeps the active list', async () => {
    seed([wl('a', 'A'), wl('b', 'B'), wl('c', 'C')], 'c');
    await state().deleteList('a');
    expect(state().activeId).toBe('c');
    expect(backend.state.watchlists.activeId).toBe('c');
    expect(backend.callsTo('watchlists.setActive')).toHaveLength(0);
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

  it('add of a token Go does not know rejects with unknown_instrument', async () => {
    seed([wl('a', 'Watchlist 1')]);
    await expect(state().addItem(424242)).rejects.toMatchObject({ code: 'unknown_instrument' });
    expect(state().lists[0].items).toEqual([]);
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

  it('stale_order: reloads and retries the move once against Go\'s current list (Ruling 87)', async () => {
    seed([wl('a', 'Watchlist 1', [SBIN, TCS])]);
    backend.state.watchlists.lists[0].items.push(watchlistItemOf(RELIANCE)); // changed elsewhere
    await state().move(TCS.token, -1);
    expect(backend.callsTo('watchlists.reorder').map((c) => c.args)).toEqual([
      ['a', [TCS.token, SBIN.token]],
      ['a', [TCS.token, SBIN.token, RELIANCE.token]],
    ]);
    expect(backend.callsTo('watchlists.list')).toHaveLength(1);
    expect(tokensOf(state().lists[0])).toEqual([TCS.token, SBIN.token, RELIANCE.token]);
  });

  it('stale_order twice: retries only once, then shows Go\'s order and rejects', async () => {
    seed([wl('a', 'Watchlist 1', [SBIN, TCS])]);
    let attempts = 0;
    backend.watchlists.reorder = async () => {
      attempts++;
      throw refusal('stale_order');
    };
    await expect(state().move(TCS.token, -1)).rejects.toMatchObject({ code: 'stale_order' });
    expect(attempts).toBe(2);
    expect(tokensOf(state().lists[0])).toEqual([SBIN.token, TCS.token]);
  });
});

describe('watchlists.changed (Ruling 87)', () => {
  it('applies Go\'s published state, including a SetActive made elsewhere', () => {
    seed([wl('a', 'A'), wl('b', 'B')]);
    const stop = startEventBridge(backend, { navigate: () => {} });
    const next: WatchlistsState = { lists: [wl('a', 'A', [SBIN]), wl('b', 'B')], activeId: 'b' };
    backend.emit('watchlists.changed', next);
    expect(state().activeId).toBe('b');
    expect(tokensOf(state().lists[0])).toEqual([SBIN.token]);
    stop();
    expect(backend.listenerCount('watchlists.changed')).toBe(0);
  });

  it('every mutation through the mock publishes the state the store ends with', async () => {
    seed([wl('a', 'A'), wl('b', 'B')]);
    const stop = startEventBridge(backend, { navigate: () => {} });
    await state().setActive('b');
    await state().addItem(TCS.token);
    expect(state().activeId).toBe('b');
    expect(tokensOf(activeList(state())!)).toEqual([TCS.token]);
    stop();
  });

  it('is ignored until the store has been loaded (after a logout reset it does not repopulate)', () => {
    resetOnLogout();
    backend.emit('watchlists.changed', { lists: [wl('a', 'A')], activeId: 'a' });
    const stop = startEventBridge(backend, { navigate: () => {} });
    backend.emit('watchlists.changed', { lists: [wl('a', 'A')], activeId: 'a' });
    expect(state()).toMatchObject({ lists: [], activeId: null, status: 'idle' });
    stop();
  });

  it('an event that arrives while load() is in flight wins over the older load response', async () => {
    const stop = startEventBridge(backend, { navigate: () => {} });
    const d = deferred<WatchlistsState>();
    backend.watchlists.list = () => d.promise;
    const p = state().load();
    backend.emit('watchlists.changed', { lists: [wl('n', 'Newer')], activeId: 'n' });
    d.resolve({ lists: [wl('o', 'Older')], activeId: 'o' });
    await p;
    expect(state()).toMatchObject({ activeId: 'n', status: 'ready' });
    expect(state().lists.map((l) => l.name)).toEqual(['Newer']);
    stop();
  });
});

describe('late responses after logout or a user change (Rulings 90/96)', () => {
  it('logout resets the store (watchlists are account data)', () => {
    seed([wl('a', 'A', [SBIN])]);
    useWatchlists.setState({ selectedToken: SBIN.token });
    resetOnLogout();
    expect(state()).toMatchObject({ lists: [], activeId: null, selectedToken: null, status: 'idle', error: null });
  });

  it('a load that resolves after logout applies nothing', async () => {
    const d = deferred<WatchlistsState>();
    backend.watchlists.list = () => d.promise;
    const p = state().load();
    resetOnLogout();
    d.resolve({ lists: [wl('a', 'A', [SBIN])], activeId: 'a' });
    await p;
    expect(state()).toMatchObject({ lists: [], activeId: null, status: 'idle' });
  });

  it('a load that resolves after a login as another user applies nothing', async () => {
    useSessionStore.getState().setSession(readySession({ userId: 'AB1234' }));
    const d = deferred<WatchlistsState>();
    backend.watchlists.list = () => d.promise;
    const p = state().load();
    useSessionStore.getState().setSession(readySession({ userId: 'ZX9876' }));
    d.resolve({ lists: [wl('a', 'A', [SBIN])], activeId: 'a' });
    await p;
    expect(state().lists).toEqual([]);
  });

  it('a load that fails after logout records no error', async () => {
    const d = deferred<WatchlistsState>();
    backend.watchlists.list = () => d.promise.then(() => { throw refusal('internal'); });
    const p = state().load();
    resetOnLogout();
    d.resolve({ lists: [], activeId: '' });
    await p;
    expect(state()).toMatchObject({ status: 'idle', error: null });
  });

  it('a create whose response arrives after logout neither repopulates nor continues with Add', async () => {
    seed([wl('a', 'A')]);
    const d = deferred<WatchlistsState>();
    backend.watchlists.create = () => d.promise;
    const p = state().create('Banks', [SBIN.token], false);
    resetOnLogout();
    d.resolve({ lists: [wl('a', 'A'), wl('n', 'Banks')], activeId: 'n' });
    await p;
    expect(state().lists).toEqual([]);
    expect(backend.callsTo('watchlists.add')).toHaveLength(0);
    expect(backend.callsTo('watchlists.setActive')).toHaveLength(0);
  });

  it('a reorder response that arrives after logout applies nothing', async () => {
    seed([wl('a', 'A', [SBIN, TCS])]);
    const d = deferred<WatchlistsState>();
    backend.watchlists.reorder = () => d.promise;
    const p = state().move(TCS.token, -1);
    resetOnLogout();
    d.resolve({ lists: [wl('a', 'A', [TCS, SBIN])], activeId: 'a' });
    await p;
    expect(state().lists).toEqual([]);
  });
});

describe('fix round 1 (Ruling 102)', () => {
  const goItems = (insts: Instrument[]) => { backend.state.watchlists.lists[0].items = insts.map(watchlistItemOf); };

  it('I1: a session that merely expires mid-create does not cancel it: every Add runs and the previous list is restored', async () => {
    useSessionStore.getState().setSession(readySession({ userId: 'AB1234' }));
    seed([wl('a', 'A')]);
    const realCreate = backend.watchlists.create;
    const d = deferred<void>();
    backend.watchlists.create = async (name) => { await d.promise; return realCreate(name); };
    const p = state().create('Banks', [SBIN.token, TCS.token], false);
    useSessionStore.getState().setSession(readySession({ state: 'expired', userId: 'AB1234' }));
    d.resolve();
    expect(await p).toBeNull();
    const banks = state().lists.find((l) => l.name === 'Banks')!;
    expect(backend.callsTo('watchlists.add').map((c) => c.args)).toEqual([[banks.id, SBIN.token, -1], [banks.id, TCS.token, -1]]);
    expect(backend.callsTo('watchlists.setActive').map((c) => c.args)).toEqual([['a']]);
    expect(state().activeId).toBe('a');
  });

  it('I1: an expired session does not stop the stale_order reload-and-retry', async () => {
    useSessionStore.getState().setSession(readySession({ userId: 'AB1234' }));
    seed([wl('a', 'A', [SBIN, TCS])]);
    goItems([SBIN, TCS, RELIANCE]);
    const realReorder = backend.watchlists.reorder;
    backend.watchlists.reorder = async (id, tokens) => {
      useSessionStore.getState().setSession(readySession({ state: 'expired', userId: 'AB1234' }));
      return realReorder(id, tokens);
    };
    await state().move(TCS.token, -1);
    expect(backend.callsTo('watchlists.reorder')).toHaveLength(2);
    expect(tokensOf(state().lists[0])).toEqual([TCS.token, SBIN.token, RELIANCE.token]);
  });

  it('I2: calls that fail after a reset resolve quietly in every method', async () => {
    const rejectAfterReset = () => {
      const d = deferred<void>();
      return { d, fn: async () => { await d.promise; throw refusal('internal'); } };
    };
    const cases: [string, keyof MockBackend['watchlists'], () => Promise<unknown>, unknown][] = [
      ['addItem', 'add', () => state().addItem(TCS.token), 'duplicate'],
      ['restoreItem', 'add', () => state().restoreItem({ listId: 'a', token: TCS.token, position: 0 }), undefined],
      ['removeItem', 'remove', () => state().removeItem(SBIN.token), null],
      ['setActive', 'setActive', () => state().setActive('b'), undefined],
      ['deleteList', 'delete', () => state().deleteList('b'), undefined],
      ['create', 'create', () => state().create('Banks'), null],
      ['rename', 'rename', () => state().rename('b', 'Banks'), null],
      ['move', 'reorder', () => state().move(SBIN.token, 1), undefined],
    ];
    for (const [name, method, run, quietValue] of cases) {
      seed([wl('a', 'A', [SBIN, RELIANCE]), wl('b', 'B')]);
      const { d, fn } = rejectAfterReset();
      (backend.watchlists as unknown as Record<string, unknown>)[method] = fn;
      const p = run();
      resetOnLogout();
      d.resolve();
      await expect(p, name).resolves.toEqual(quietValue);
      expect(state().lists, name).toEqual([]);
      backend = createMockBackend(demoSeed());
      setBackend(backend);
    }
  });

  it('m1: move() retried after stale_order recomputes from the token\'s index in the reloaded list', async () => {
    seed([wl('a', 'A', [SBIN, TCS])]);
    goItems([INFY, SBIN, TCS]);
    await state().move(TCS.token, -1);
    expect(backend.callsTo('watchlists.reorder').at(-1)!.args).toEqual(['a', [INFY.token, TCS.token, SBIN.token]]);
    expect(tokensOf(state().lists[0])).toEqual([INFY.token, TCS.token, SBIN.token]);
  });

  it('m1: move() down that lands past the end of the reloaded list is a no-op and shows Go\'s order', async () => {
    seed([wl('a', 'A', [SBIN, TCS])]);
    goItems([INFY, TCS, SBIN]);
    await state().move(SBIN.token, 1);
    expect(backend.callsTo('watchlists.reorder')).toHaveLength(1);
    expect(tokensOf(state().lists[0])).toEqual([INFY.token, TCS.token, SBIN.token]);
  });

  it('m1: moveTo() retried after stale_order keeps the row next to its drop neighbour', async () => {
    seed([wl('a', 'A', [SBIN, TCS, RELIANCE])]);
    goItems([INFY, SBIN, TCS, RELIANCE]);
    await state().moveTo(RELIANCE.token, 0); // dropped above SBIN
    expect(tokensOf(state().lists[0])).toEqual([INFY.token, RELIANCE.token, SBIN.token, TCS.token]);

    seed([wl('a', 'A', [SBIN, TCS, RELIANCE])]);
    goItems([SBIN, INFY, TCS, RELIANCE]);
    await state().moveTo(SBIN.token, 2); // dropped below RELIANCE
    expect(tokensOf(state().lists[0])).toEqual([INFY.token, TCS.token, RELIANCE.token, SBIN.token]);
  });

  it('m2: a failed reload during the retry path does not hide the original error', async () => {
    seed([wl('a', 'A', [SBIN, TCS])]);
    backend.failNext('watchlists.reorder', 'disk full', 'internal');
    backend.failNext('watchlists.list', 'offline', 'offline');
    await expect(state().move(TCS.token, -1)).rejects.toMatchObject({ code: 'internal' });
  });

  it('m3: a seeded create whose only failure is setActive(previous) reports success and shows Go\'s state', async () => {
    seed([wl('a', 'A')]);
    backend.failNext('watchlists.setActive', 'busy', 'internal');
    expect(await state().create('Banks', [SBIN.token], false)).toBeNull();
    const banks = state().lists.find((l) => l.name === 'Banks')!;
    expect(banks.items.map((i) => i.token)).toEqual([SBIN.token]);
    expect(state().activeId).toBe(backend.state.watchlists.activeId);
  });
});

describe('the event bridge owns load() (Ruling 102 I3)', () => {
  it('session.state → ready while idle loads the watchlists once', async () => {
    backend.state.watchlists = { lists: [wl('a', 'Mine', [SBIN])], activeId: 'a' };
    const stop = startEventBridge(backend, { navigate: () => {} });
    backend.emit('session.state', readySession());
    await vi.waitFor(() => expect(state().status).toBe('ready'));
    expect(state().lists.map((l) => l.name)).toEqual(['Mine']);
    backend.emit('session.state', readySession());
    await Promise.resolve();
    expect(backend.callsTo('watchlists.list')).toHaveLength(1);
    stop();
  });

  it('a non-ready session.state does not load', async () => {
    const stop = startEventBridge(backend, { navigate: () => {} });
    backend.emit('session.state', readySession({ state: 'logged_out', userId: '' }));
    await Promise.resolve();
    expect(backend.callsTo('watchlists.list')).toHaveLength(0);
    expect(state().status).toBe('idle');
    stop();
  });

  it('bootstrap loads them when the session is already ready', async () => {
    backend.state.watchlists = { lists: [wl('a', 'Mine')], activeId: 'a' };
    await bootstrap(backend);
    await vi.waitFor(() => expect(state().status).toBe('ready'));
    expect(backend.callsTo('watchlists.list')).toHaveLength(1);
  });

  it('bootstrap does not load them for a signed-out session', async () => {
    backend.state.session = readySession({ state: 'no_credentials', userId: '' });
    await bootstrap(backend);
    await Promise.resolve();
    expect(backend.callsTo('watchlists.list')).toHaveLength(0);
  });
});
