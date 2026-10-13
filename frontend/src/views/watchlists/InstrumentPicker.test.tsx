import { cleanup, act, fireEvent, render, screen, within } from '@testing-library/react';
import { useState } from 'react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

const h = vi.hoisted(() => ({ extra: [] as unknown[] }));
vi.mock('../../market/hooks', () => ({ useSubscription: vi.fn(), usePolled: () => false }));
vi.mock('../../api/instruments', async () => {
  const f = await import('../../instruments/fixtures');
  const all = () => [...f.ALL_IN_SCOPE, ...(h.extra as typeof f.ALL_IN_SCOPE)];
  return {
    searchInstruments: vi.fn(async (q: string, tags: string[]) => all().filter((i) => tags.includes(i.tag) && i.tradingsymbol.toLowerCase().includes(q.toLowerCase()))),
    instrumentsByTokens: vi.fn(async (t: number[]) => all().filter((i) => t.includes(i.token))),
  };
});

import { instrumentsByTokens, searchInstruments } from '../../api/instruments';
import type { Watchlist } from '../../api/backend';
import { setBackend } from '../../api/current';
import { demoSeed } from '../../api/mock/fixtures';
import { createMockBackend, type MockBackend } from '../../api/mock/index';
import { watchlistItemOf } from '../../api/mock/market';
import { SEARCH_DEBOUNCE_MS } from '../../instruments/useInstrumentSearch';
import { useSubscription } from '../../market/hooks';
import * as copyModule from '../../i18n/copy';
import { InstrumentPicker } from './InstrumentPicker';
import { useInstruments } from '../../store/instrumentsStore';
import { resetOnLogout } from '../../store/reset';
import { useToastStore } from '../../store/toasts';
import { useWatchlists } from '../../store/watchlistsStore';
import { INFY, NIFTY_OPTION, RELIANCE, RELIANCE_OCT_FUT, SBIN, TCS } from '../../instruments/fixtures';
import type { Instrument } from '../../instruments/types';

let backend: MockBackend;
const many = (n: number, base = 50_000): Instrument[] =>
  Array.from({ length: n }, (_, i) => ({ ...SBIN, token: base + i, tradingsymbol: `PK${i}`, displayName: `PK${i}` }));
const wl = (id: string, name: string, insts: Instrument[] = []): Watchlist => ({ id, name, items: insts.map(watchlistItemOf) });
const seedLists = (lists: Watchlist[]) => {
  backend.state.watchlists = JSON.parse(JSON.stringify({ lists, activeId: lists[0].id }));
  useWatchlists.setState({ lists, activeId: lists[0].id, selectedToken: null, status: 'ready', error: null });
};
const setup = (props: Partial<Parameters<typeof InstrumentPicker>[0]> = {}) => {
  const onClose = vi.fn(); const onDone = vi.fn();
  const view = render(<InstrumentPicker open onClose={onClose} onDone={onDone} {...props} />);
  return { onClose, onDone, view };
};
/** Runs due timers and the promise callbacks they start, inside act. */
const flush = (ms = SEARCH_DEBOUNCE_MS) => act(async () => { await vi.advanceTimersByTimeAsync(ms); });
const searchBox = () => screen.getByRole('textbox', { name: 'Search instruments' }) as HTMLInputElement;
const type = async (value: string) => {
  fireEvent.change(searchBox(), { target: { value } });
  await flush();
};
const heading = (name: string) => screen.getByRole('heading', { name });
function deferred<T>() {
  let resolve!: (v: T) => void;
  const promise = new Promise<T>((res) => { resolve = res; });
  return { promise, resolve };
}

beforeEach(() => {
  vi.useFakeTimers();
  vi.clearAllMocks();
  h.extra = [];
  backend = createMockBackend(demoSeed());
  setBackend(backend);
  useInstruments.setState({ byToken: {}, byId: {} });
  seedLists([wl('w1', 'Watchlist 1')]);
});
afterEach(() => {
  cleanup(); // unmount before unregistering the backend (Task 19)
  setBackend(null);
  vi.useRealTimers();
});

describe('InstrumentPicker', () => {
  it('opens with focus in the search box; an empty box shows "Type to search" and sends no search', async () => {
    setup();
    expect(screen.getByRole('dialog', { name: 'Add instruments' })).toBeInTheDocument();
    expect(searchBox()).toHaveFocus();
    expect(screen.getByText('Type to search')).toBeInTheDocument();
    await flush(200);
    expect(searchInstruments).not.toHaveBeenCalled();
  });

  it('searches with limit 50 over the caller tags (default: all five)', async () => {
    setup();
    await type('sbin');
    expect(searchInstruments).toHaveBeenCalledWith('sbin', ['INDICES', 'NSE', 'BSE', 'NFO-FUT', 'BFO-FUT'], 50);
    expect(screen.getByRole('checkbox', { name: 'Select SBIN' })).not.toBeChecked();
    expect(screen.queryByText('Type to search')).toBeNull();
  });

  it('AC3: without a caller maximum, 300 instruments can be selected (watchlist chip + select all)', async () => {
    h.extra = many(300);
    seedLists([wl('w1', 'Watchlist 1'), wl('big', 'Big', many(300))]);
    setup();
    fireEvent.click(screen.getByRole('button', { name: 'Big' }));
    await flush(0);
    expect(screen.getAllByRole('checkbox', { name: /^Select PK/ })).toHaveLength(300);
    fireEvent.click(screen.getByRole('checkbox', { name: 'Select all' }));
    expect(heading('Selected (300)')).toBeInTheDocument();
  });

  it('AC2: with max 100 and 100 selected, another tick is refused with "Up to 100 instruments"', async () => {
    h.extra = many(100);
    setup({ max: 100, initial: many(100) });
    expect(heading('Selected (100 / 100)')).toBeInTheDocument();
    await type('reliance');
    fireEvent.click(screen.getAllByRole('checkbox', { name: 'Select RELIANCE' })[0]); // NSE and BSE listings share the name
    expect(screen.getByRole('alert')).toHaveTextContent('Up to 100 instruments');
    expect(heading('Selected (100 / 100)')).toBeInTheDocument();
  });

  it('caller tag restriction hides other tags; duplicates are ignored; the remove button removes', async () => {
    seedLists([wl('w1', 'Watchlist 1', [RELIANCE, SBIN, RELIANCE_OCT_FUT])]);
    setup({ segments: ['NSE', 'INDICES'], initial: [RELIANCE] });
    await type('reliance');
    expect(searchInstruments).toHaveBeenCalledWith('reliance', ['NSE', 'INDICES'], 50);
    expect(screen.getByRole('checkbox', { name: 'Select RELIANCE' })).toBeChecked(); // only the NSE listing: BSE and NFO-FUT filtered
    expect(screen.queryByText(RELIANCE_OCT_FUT.displayName)).toBeNull();
    fireEvent.click(screen.getByRole('button', { name: 'Watchlist 1' }));
    await flush(0);
    expect(screen.getByRole('checkbox', { name: 'Select SBIN' })).toBeInTheDocument();
    expect(screen.queryByText(RELIANCE_OCT_FUT.displayName)).toBeNull(); // the list's NFO-FUT item is outside the caller tags
    fireEvent.click(screen.getByRole('checkbox', { name: 'Select all' }));
    expect(heading('Selected (2)')).toBeInTheDocument();
    fireEvent.click(within(screen.getByRole('list', { name: 'Selected instruments' })).getByRole('button', { name: 'Remove RELIANCE' }));
    expect(heading('Selected (1)')).toBeInTheDocument();
    expect(screen.getByRole('checkbox', { name: 'Select RELIANCE' })).not.toBeChecked();
  });

  it('a watchlist chip loads full instruments with ensureInstruments and sends no search while it is the source', async () => {
    seedLists([wl('w1', 'Watchlist 1', [SBIN, TCS])]);
    setup();
    await type('rel');
    vi.mocked(searchInstruments).mockClear();
    fireEvent.click(screen.getByRole('button', { name: 'Watchlist 1' }));
    expect(screen.getByRole('button', { name: 'Watchlist 1' })).toHaveAttribute('aria-pressed', 'true');
    expect(searchBox()).toHaveValue('');
    await flush(500);
    expect(instrumentsByTokens).toHaveBeenCalledWith([SBIN.token, TCS.token]);
    expect(searchInstruments).not.toHaveBeenCalled();
    expect(screen.getByRole('checkbox', { name: 'Select SBIN' })).toBeInTheDocument();
    expect(screen.getByRole('checkbox', { name: 'Select TCS' })).toBeInTheDocument();
    expect(screen.queryByText('Type to search')).toBeNull();
    await type('tc'); // typing makes the search the source again
    expect(screen.getByRole('button', { name: 'Watchlist 1' })).toHaveAttribute('aria-pressed', 'false');
    expect(searchInstruments).toHaveBeenCalledWith('tc', expect.anything(), 50);
  });

  it('NFR-15: an out-of-scope row from search or from a list is never offered, selected or cached', async () => {
    h.extra = [NIFTY_OPTION];
    seedLists([wl('w1', 'Watchlist 1', [SBIN, NIFTY_OPTION])]);
    vi.mocked(searchInstruments).mockResolvedValueOnce([NIFTY_OPTION, SBIN]);
    setup({ initial: [NIFTY_OPTION, TCS] });
    expect(heading('Selected (1)')).toBeInTheDocument(); // the option in `initial` is dropped
    await type('nifty');
    expect(screen.queryByText(NIFTY_OPTION.displayName)).toBeNull();
    fireEvent.click(screen.getByRole('checkbox', { name: 'Select all' }));
    fireEvent.click(screen.getByRole('button', { name: 'Watchlist 1' }));
    await flush(0);
    expect(screen.queryByText(NIFTY_OPTION.displayName)).toBeNull();
    expect(screen.getAllByRole('checkbox', { name: /^Select / }).map((c) => c.getAttribute('aria-label'))).toEqual(['Select all', 'Select SBIN']);
    expect(useInstruments.getState().byToken[NIFTY_OPTION.token]).toBeUndefined();
  });

  it('a selected search row is cached; nothing is cached for rows only listed', async () => {
    setup();
    await type('s');
    expect(useInstruments.getState().byToken[SBIN.token]).toBeUndefined();
    fireEvent.click(screen.getByRole('checkbox', { name: 'Select SBIN' }));
    expect(useInstruments.getState().byToken[SBIN.token]).toEqual(SBIN);
    expect(useInstruments.getState().byToken[TCS.token]).toBeUndefined();
  });

  it('reset (sign-out) during a search: the late answer is dropped, query and source are cleared', async () => {
    const d = deferred<Instrument[]>();
    vi.mocked(searchInstruments).mockReturnValueOnce(d.promise);
    setup();
    await type('sbin');
    act(() => resetOnLogout());
    expect(searchBox()).toHaveValue('');
    expect(screen.getByText('Type to search')).toBeInTheDocument();
    await act(async () => { d.resolve([SBIN]); await Promise.resolve(); });
    await flush(500);
    expect(screen.queryByRole('checkbox', { name: 'Select SBIN' })).toBeNull();
    expect(useInstruments.getState().byToken[SBIN.token]).toBeUndefined();
  });

  it('reset during a watchlist chip load: no rows appear and the source goes back to search', async () => {
    seedLists([wl('w1', 'Watchlist 1', [SBIN])]);
    const d = deferred<Instrument[]>();
    vi.mocked(instrumentsByTokens).mockReturnValueOnce(d.promise);
    setup();
    fireEvent.click(screen.getByRole('button', { name: 'Watchlist 1' }));
    act(() => resetOnLogout());
    useWatchlists.setState({ lists: [wl('w1', 'Watchlist 1', [SBIN])], activeId: 'w1', status: 'ready' }); // the next session's lists
    await act(async () => { d.resolve([SBIN]); await Promise.resolve(); });
    await flush(500);
    expect(screen.queryByRole('checkbox', { name: 'Select SBIN' })).toBeNull();
    expect(screen.getByRole('button', { name: 'Watchlist 1' })).toHaveAttribute('aria-pressed', 'false');
    expect(screen.getByText('Type to search')).toBeInTheDocument();
  });

  it('keyboard: ArrowDown from the box enters the multi-select grid; arrows move, Space toggles, ArrowUp at the top returns', async () => {
    setup();
    await type('s');
    const grid = screen.getByRole('grid', { name: 'Instruments' });
    expect(grid).toHaveAttribute('aria-multiselectable', 'true');
    fireEvent.keyDown(searchBox(), { key: 'ArrowDown' });
    const boxes = within(grid).getAllByRole('checkbox');
    expect(boxes[0]).toHaveFocus();
    expect(boxes.filter((b) => b.tabIndex === 0)).toHaveLength(1); // roving tabindex
    fireEvent.keyDown(boxes[0], { key: 'ArrowDown' });
    expect(boxes[1]).toHaveFocus();
    fireEvent.keyDown(boxes[1], { key: 'End' });
    expect(boxes.at(-1)).toHaveFocus();
    fireEvent.keyDown(boxes.at(-1)!, { key: 'Home' });
    expect(boxes[0]).toHaveFocus();
    fireEvent.click(boxes[0]); // Space on a native checkbox
    expect(within(grid).getAllByRole('row')[0]).toHaveAttribute('aria-selected', 'true');
    expect(heading('Selected (1)')).toBeInTheDocument();
    fireEvent.keyDown(boxes[0], { key: 'ArrowUp' });
    expect(searchBox()).toHaveFocus();
  });

  it('AC4: "Save as watchlist" named "Banks" creates it from the selection without switching the active list', async () => {
    setup({ initial: [SBIN, RELIANCE] });
    fireEvent.click(screen.getByRole('button', { name: 'Save as watchlist' }));
    const name = screen.getByRole('textbox', { name: 'Watchlist name' });
    expect(name).toHaveValue('Watchlist 2');
    expect(name).toHaveFocus();
    fireEvent.change(name, { target: { value: 'Banks' } });
    fireEvent.keyDown(name, { key: 'Enter' });
    await flush(0);
    expect(useToastStore.getState().visible[0]).toMatchObject({ level: 'info', message: 'Watchlist Banks created' });
    expect(backend.callsTo('watchlists.create')[0].args).toEqual(['Banks']);
    const banks = useWatchlists.getState().lists.find((l) => l.name === 'Banks')!;
    expect(backend.callsTo('watchlists.add').map((c) => c.args)).toEqual([[banks.id, SBIN.token, -1], [banks.id, RELIANCE.token, -1]]);
    expect(useWatchlists.getState().activeId).toBe('w1');
    expect(screen.queryByRole('textbox', { name: 'Watchlist name' })).toBeNull();
    expect(screen.getByRole('button', { name: 'Save as watchlist' })).toHaveFocus();
  });

  it('save as watchlist: a name error is shown on the field and nothing is created; Escape there only cancels naming', async () => {
    const { onClose } = setup({ initial: [SBIN] });
    fireEvent.click(screen.getByRole('button', { name: 'Save as watchlist' }));
    const name = screen.getByRole('textbox', { name: 'Watchlist name' });
    fireEvent.change(name, { target: { value: 'watchlist 1' } });
    fireEvent.keyDown(name, { key: 'Enter' });
    await flush(0);
    expect(screen.getByRole('alert')).toHaveTextContent('A watchlist with this name exists');
    expect(name).toHaveAttribute('aria-invalid', 'true');
    expect(backend.callsTo('watchlists.create')).toHaveLength(0);
    fireEvent.keyDown(name, { key: 'Escape' });
    expect(screen.queryByRole('textbox', { name: 'Watchlist name' })).toBeNull();
    expect(onClose).not.toHaveBeenCalled();
    expect(screen.getByRole('button', { name: 'Save as watchlist' })).toHaveFocus();
  });

  it('Done returns the selection', () => {
    const { onDone } = setup({ initial: [SBIN] });
    fireEvent.click(screen.getByRole('button', { name: 'Done' }));
    expect(onDone).toHaveBeenCalledWith([SBIN]);
  });

  it('AC5: Escape closes once, without applying', () => {
    const { onClose, onDone } = setup({ initial: [SBIN] });
    fireEvent.keyDown(searchBox(), { key: 'Escape' });
    expect(onClose).toHaveBeenCalledTimes(1);
    expect(onDone).not.toHaveBeenCalled();
  });

  it('focus returns to the opener when the picker closes; each opening starts afresh', async () => {
    function Host() {
      const [open, setOpen] = useState(false);
      return (
        <>
          <button type="button" onClick={() => setOpen(true)}>Open picker</button>
          <InstrumentPicker open={open} onClose={() => setOpen(false)} onDone={() => setOpen(false)} />
        </>
      );
    }
    render(<Host />);
    const opener = screen.getByRole('button', { name: 'Open picker' });
    opener.focus();
    fireEvent.click(opener);
    await type('s');
    fireEvent.click(screen.getByRole('checkbox', { name: 'Select SBIN' }));
    fireEvent.keyDown(searchBox(), { key: 'Escape' });
    expect(screen.queryByRole('dialog')).toBeNull();
    expect(opener).toHaveFocus();
    fireEvent.click(opener);
    expect(searchBox()).toHaveValue('');
    expect(heading('Selected (0)')).toBeInTheDocument();
  });

  it('subscribes the selected tokens as consumer widget', async () => {
    setup({ initial: [SBIN] });
    expect(vi.mocked(useSubscription)).toHaveBeenLastCalledWith('instrument-picker', 'widget', [SBIN.token]);
    await type('tcs');
    fireEvent.click(screen.getByRole('checkbox', { name: 'Select TCS' }));
    expect(vi.mocked(useSubscription)).toHaveBeenLastCalledWith('instrument-picker', 'widget', [SBIN.token, TCS.token]);
  });

  it('G-0: no order actions', () => {
    setup({ initial: [SBIN] });
    expect(screen.queryByRole('button', { name: /^(buy|sell|b|s)$/i })).toBeNull();
  });
});

describe('InstrumentPicker fix round 1 (Ruling 118)', () => {
  it('I1: a reset clears the selection in place (picker stays open); the widget set empties; Done returns []', async () => {
    const { onDone, onClose } = setup({ initial: [SBIN] });
    await type('tcs');
    fireEvent.click(screen.getByRole('checkbox', { name: 'Select TCS' }));
    fireEvent.click(screen.getByRole('button', { name: 'Save as watchlist' }));
    expect(heading('Selected (2)')).toBeInTheDocument();
    act(() => resetOnLogout());
    expect(screen.getByRole('dialog')).toBeInTheDocument();
    expect(onClose).not.toHaveBeenCalled();
    expect(heading('Selected (0)')).toBeInTheDocument();
    expect(screen.queryByRole('textbox', { name: 'Watchlist name' })).toBeNull();
    expect(vi.mocked(useSubscription)).toHaveBeenLastCalledWith('instrument-picker', 'widget', []);
    fireEvent.click(screen.getByRole('button', { name: 'Done' }));
    expect(onDone).toHaveBeenCalledWith([]);
  });

  it('I1: a reset also clears the limit message', async () => {
    setup({ max: 1, initial: [SBIN] });
    await type('tcs');
    fireEvent.click(screen.getByRole('checkbox', { name: 'Select TCS' }));
    expect(screen.getByRole('alert')).toHaveTextContent('Up to 1 instruments');
    act(() => resetOnLogout());
    expect(screen.queryByRole('alert')).toBeNull();
  });

  it('I2: a save dropped by a reset shows no toast and no error, even when a list of that name shows up later', async () => {
    let release!: () => void;
    const gate = new Promise<void>((r) => { release = r; });
    const create = backend.watchlists.create;
    backend.watchlists.create = async (name: string) => { await gate; return create(name); };
    setup({ initial: [SBIN] });
    fireEvent.click(screen.getByRole('button', { name: 'Save as watchlist' }));
    const name = screen.getByRole('textbox', { name: 'Watchlist name' });
    fireEvent.change(name, { target: { value: 'Banks' } });
    fireEvent.keyDown(name, { key: 'Enter' });
    await flush(0);
    act(() => resetOnLogout());
    // The next session's load already lists a "Banks" (Go created it before the Adds were dropped).
    act(() => useWatchlists.setState({ lists: [wl('w1', 'Watchlist 1'), wl('w9', 'Banks')], activeId: 'w1', status: 'ready' }));
    await act(async () => { release(); await vi.advanceTimersByTimeAsync(0); });
    expect(useToastStore.getState().visible).toHaveLength(0);
    expect(screen.queryByRole('alert')).toBeNull();
  });

  it('I2: a list of the same name that already existed is not reported as created when Go refuses', async () => {
    seedLists([wl('w1', 'Watchlist 1'), wl('w2', 'Banks ')]); // trailing space: the local check folds it, Go is the backstop
    useWatchlists.setState({ lists: [wl('w1', 'Watchlist 1')] }); // the store has not seen w2 yet
    backend.failNext('watchlists.create', 'A watchlist with this name exists', 'duplicate_name');
    setup({ initial: [SBIN] });
    fireEvent.click(screen.getByRole('button', { name: 'Save as watchlist' }));
    const name = screen.getByRole('textbox', { name: 'Watchlist name' });
    fireEvent.change(name, { target: { value: 'Banks' } });
    fireEvent.keyDown(name, { key: 'Enter' });
    await flush(0);
    expect(screen.getByRole('alert')).toHaveTextContent('A watchlist with this name exists'); // Go's duplicate_name
    expect(name).toHaveAttribute('aria-invalid', 'true');
    expect(useToastStore.getState().visible).toHaveLength(0);
  });

  it('I3: the strings come from i18n/copy.ts', () => {
    const c = (copyModule as Record<string, unknown>).instrumentPickerCopy as { title: string; typeToSearch: string } | undefined;
    expect(c).toBeDefined();
    setup();
    expect(screen.getByRole('dialog', { name: c!.title })).toBeInTheDocument();
    expect(screen.getByText(c!.typeToSearch)).toBeInTheDocument();
  });

  it('double-save guard: Enter twice while the save runs creates one list', async () => {
    let release!: () => void;
    const gate = new Promise<void>((r) => { release = r; });
    const create = backend.watchlists.create;
    backend.watchlists.create = async (name: string) => { await gate; return create(name); };
    setup({ initial: [SBIN] });
    fireEvent.click(screen.getByRole('button', { name: 'Save as watchlist' }));
    const name = screen.getByRole('textbox', { name: 'Watchlist name' });
    fireEvent.change(name, { target: { value: 'Banks' } });
    fireEvent.keyDown(name, { key: 'Enter' });
    fireEvent.keyDown(name, { key: 'Enter' });
    fireEvent.click(screen.getByRole('button', { name: 'Save' }));
    await act(async () => { release(); await vi.advanceTimersByTimeAsync(0); });
    expect(useWatchlists.getState().lists.filter((l) => l.name === 'Banks')).toHaveLength(1);
    expect(useToastStore.getState().visible.filter((t) => t.message === 'Watchlist Banks created')).toHaveLength(1);
  });

  it('minor 1: empty segments mean all tags, for the search and for a chip', async () => {
    seedLists([wl('w1', 'Watchlist 1', [SBIN, RELIANCE_OCT_FUT])]);
    setup({ segments: [] });
    await type('sbin');
    expect(searchInstruments).toHaveBeenCalledWith('sbin', ['INDICES', 'NSE', 'BSE', 'NFO-FUT', 'BFO-FUT'], 50);
    fireEvent.click(screen.getByRole('button', { name: 'Watchlist 1' }));
    await flush(0);
    expect(screen.getByRole('checkbox', { name: 'Select SBIN' })).toBeInTheDocument();
    expect(screen.getByRole('checkbox', { name: `Select ${RELIANCE_OCT_FUT.displayName}` })).toBeInTheDocument();
  });

  it('minor 2: initial is deduped by token and clamped to max', () => {
    setup({ max: 2, initial: [SBIN, SBIN, TCS, INFY] });
    expect(heading('Selected (2 / 2)')).toBeInTheDocument();
    const names = within(screen.getByRole('list', { name: 'Selected instruments' })).getAllByRole('button').map((b) => b.getAttribute('aria-label'));
    expect(names).toEqual(['Remove SBIN', 'Remove TCS']);
  });

  it('minor 3: a failed chip load shows the error; clicking the chip again retries', async () => {
    seedLists([wl('w1', 'Watchlist 1', [SBIN])]);
    vi.mocked(instrumentsByTokens).mockRejectedValueOnce(new Error('Instruments unavailable'));
    setup();
    fireEvent.click(screen.getByRole('button', { name: 'Watchlist 1' }));
    await flush(0);
    expect(screen.getByRole('alert')).toHaveTextContent('Instruments unavailable');
    expect(screen.queryByRole('checkbox', { name: 'Select SBIN' })).toBeNull();
    fireEvent.click(screen.getByRole('button', { name: 'Watchlist 1' }));
    await flush(0);
    expect(instrumentsByTokens).toHaveBeenCalledTimes(2);
    expect(screen.getByRole('checkbox', { name: 'Select SBIN' })).toBeInTheDocument();
    expect(screen.queryByRole('alert')).toBeNull();
  });

  it('minor 4: a repeated limit refusal is announced again (a new alert element)', async () => {
    setup({ max: 1, initial: [SBIN] });
    await type('tcs');
    fireEvent.click(screen.getByRole('checkbox', { name: 'Select TCS' }));
    const first = screen.getByRole('alert');
    fireEvent.click(screen.getByRole('checkbox', { name: 'Select TCS' }));
    const second = screen.getByRole('alert');
    expect(second).toHaveTextContent('Up to 1 instruments');
    expect(second).not.toBe(first);
  });

  it('select all is cut short at max, in row order, with the limit message', async () => {
    seedLists([wl('w1', 'Watchlist 1', [SBIN, TCS, INFY])]);
    setup({ max: 2 });
    fireEvent.click(screen.getByRole('button', { name: 'Watchlist 1' }));
    await flush(0);
    fireEvent.click(screen.getByRole('checkbox', { name: 'Select all' }));
    expect(heading('Selected (2 / 2)')).toBeInTheDocument();
    expect(screen.getByRole('checkbox', { name: 'Select SBIN' })).toBeChecked();
    expect(screen.getByRole('checkbox', { name: 'Select TCS' })).toBeChecked();
    expect(screen.getByRole('checkbox', { name: 'Select INFY' })).not.toBeChecked();
    expect(screen.getByRole('alert')).toHaveTextContent('Up to 2 instruments');
  });

  it('keyboard: Space and Enter pressed on a row toggle it', async () => {
    seedLists([wl('w1', 'Watchlist 1', [SBIN, TCS])]);
    setup();
    fireEvent.click(screen.getByRole('button', { name: 'Watchlist 1' }));
    await flush(0);
    fireEvent.keyDown(searchBox(), { key: 'ArrowDown' });
    const sbin = screen.getByRole('checkbox', { name: 'Select SBIN' });
    expect(sbin).toHaveFocus();
    fireEvent.keyDown(sbin, { key: ' ' });
    expect(sbin).toBeChecked();
    expect(heading('Selected (1)')).toBeInTheDocument();
    fireEvent.keyDown(sbin, { key: 'Enter' });
    expect(sbin).not.toBeChecked();
    fireEvent.keyDown(sbin, { key: 'ArrowDown' });
    const tcs = screen.getByRole('checkbox', { name: 'Select TCS' });
    fireEvent.keyDown(tcs, { key: 'Enter' });
    expect(tcs).toBeChecked();
    expect(heading('Selected (1)')).toBeInTheDocument();
  });

  it('focus after removing: the next remove button, else the previous, else the search box', () => {
    setup({ initial: [SBIN, TCS, INFY] });
    const pane = () => within(screen.getByRole('list', { name: 'Selected instruments' }));
    fireEvent.click(pane().getByRole('button', { name: 'Remove TCS' }));
    expect(pane().getByRole('button', { name: 'Remove INFY' })).toHaveFocus();
    fireEvent.click(pane().getByRole('button', { name: 'Remove INFY' }));
    expect(pane().getByRole('button', { name: 'Remove SBIN' })).toHaveFocus();
    fireEvent.click(pane().getByRole('button', { name: 'Remove SBIN' }));
    expect(searchBox()).toHaveFocus();
  });
});
