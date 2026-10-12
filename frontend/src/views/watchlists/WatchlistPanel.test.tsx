import { act, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';

vi.mock('../../shortcuts/ShortcutProvider', () => ({ useShortcut: vi.fn() }));
vi.mock('../../market/hooks', () => ({ useSubscription: vi.fn(), usePolled: () => false }));
vi.mock('../../api/instruments', async () => {
  const f = await import('../../instruments/fixtures');
  return { searchInstruments: vi.fn(async () => []), instrumentsByTokens: vi.fn(async (t: number[]) => f.ALL_IN_SCOPE.filter((i) => t.includes(i.token))) };
});

import type { Watchlist } from '../../api/backend';
import { setBackend } from '../../api/current';
import { demoSeed } from '../../api/mock/fixtures';
import { createMockBackend, type MockBackend } from '../../api/mock/index';
import { watchlistItemOf } from '../../api/mock/market';
import { WatchlistPanel } from './WatchlistPanel';
import { useWatchlists } from '../../store/watchlistsStore';
import { useToastStore } from '../../store/toasts';
import { SBIN, TCS } from '../../instruments/fixtures';

let backend: MockBackend;
const lists = (...names: string[]): Watchlist[] =>
  names.map((name, i) => ({ id: `w${i + 1}`, name, items: i === 0 ? [SBIN, TCS].map(watchlistItemOf) : [] }));
const seed = (ls: Watchlist[]) => { backend.state.watchlists = { lists: ls, activeId: ls[0].id }; };
const renderPanel = () => render(<WatchlistPanel onChartItem={vi.fn()} />);

beforeEach(() => {
  backend = createMockBackend(demoSeed());
  setBackend(backend);
  useWatchlists.setState({ lists: [], activeId: null, selectedToken: null, status: 'idle', error: null });
});

describe('WatchlistPanel', () => {
  it('FR-WATCH-001 AC1: fresh install shows one empty "Watchlist 1" with the hint', async () => {
    renderPanel();
    expect(await screen.findByRole('tab', { name: 'Watchlist 1' })).toHaveAttribute('aria-selected', 'true');
    expect(screen.getByText('Search to add instruments')).toBeInTheDocument();
    expect(screen.getByRole('region', { name: 'Watchlist' })).toBeInTheDocument();
  });

  it('FR-WATCH-001 AC2: clicking the second tab shows it and persists it as active in Go', async () => {
    seed(lists('Watchlist 1', 'Watchlist 2', 'Watchlist 3'));
    renderPanel();
    fireEvent.click(await screen.findByRole('tab', { name: 'Watchlist 2' }));
    await waitFor(() => expect(backend.callsTo('watchlists.setActive').at(-1)?.args).toEqual(['w2']));
    expect(backend.state.watchlists.activeId).toBe('w2');
    expect(screen.getByRole('tab', { name: 'Watchlist 2' })).toHaveAttribute('aria-selected', 'true');
    expect(screen.getByText('Search to add instruments')).toBeInTheDocument();
  });

  it('FR-WATCH-002 AC1: "New watchlist +" pre-fills "Watchlist 3"; Enter creates and activates it', async () => {
    seed(lists('Watchlist 1', 'Watchlist 2'));
    renderPanel();
    fireEvent.click(await screen.findByRole('button', { name: 'New watchlist +' }));
    const name = screen.getByRole('textbox', { name: 'New watchlist name' });
    expect(name).toHaveValue('Watchlist 3');
    fireEvent.keyDown(name, { key: 'Enter' });
    expect(await screen.findByRole('tab', { name: 'Watchlist 3' })).toHaveAttribute('aria-selected', 'true');
    expect(backend.callsTo('watchlists.create')[0].args).toEqual(['Watchlist 3']);
  });

  it('FR-WATCH-002 AC3: empty name is refused; Escape cancels', async () => {
    seed(lists('Watchlist 1'));
    renderPanel();
    fireEvent.click(await screen.findByRole('button', { name: 'New watchlist +' }));
    const name = screen.getByRole('textbox', { name: 'New watchlist name' });
    fireEvent.change(name, { target: { value: '  ' } });
    fireEvent.keyDown(name, { key: 'Enter' });
    expect(await screen.findByRole('alert')).toHaveTextContent('Enter a name');
    expect(backend.callsTo('watchlists.create')).toHaveLength(0);
    fireEvent.keyDown(name, { key: 'Escape' });
    expect(screen.queryByRole('textbox', { name: 'New watchlist name' })).toBeNull();
  });

  it('FR-WATCH-002 AC2: renaming (double-click) to an existing name shows the inline error and changes nothing', async () => {
    seed(lists('Watchlist 1', 'Banks'));
    renderPanel();
    fireEvent.doubleClick(await screen.findByRole('tab', { name: 'Banks' }));
    const field = screen.getByRole('textbox', { name: 'Watchlist name' });
    fireEvent.change(field, { target: { value: 'watchlist 1' } });
    fireEvent.keyDown(field, { key: 'Enter' });
    expect(await screen.findByRole('alert')).toHaveTextContent('A watchlist with this name exists');
    expect(backend.callsTo('watchlists.rename')).toHaveLength(0);
  });

  it('context menu "Rename" renames', async () => {
    seed(lists('Watchlist 1'));
    renderPanel();
    fireEvent.contextMenu(await screen.findByRole('tab', { name: 'Watchlist 1' }));
    fireEvent.click(screen.getByRole('menuitem', { name: 'Rename' }));
    const field = screen.getByRole('textbox', { name: 'Watchlist name' });
    fireEvent.change(field, { target: { value: 'Core' } });
    fireEvent.keyDown(field, { key: 'Enter' });
    expect(await screen.findByRole('tab', { name: 'Core' })).toBeInTheDocument();
    expect(backend.callsTo('watchlists.rename')[0].args).toEqual(['w1', 'Core']);
  });

  it('FR-WATCH-003 AC1/AC2: delete asks with name and count; Cancel keeps it; Delete on the only list leaves "Watchlist 1"', async () => {
    seed(lists('Mine'));
    renderPanel();
    fireEvent.click(await screen.findByRole('button', { name: 'Delete watchlist Mine' }));
    expect(screen.getByText('Delete watchlist Mine and its 2 instruments?')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Cancel' }));
    expect(backend.callsTo('watchlists.delete')).toHaveLength(0);
    expect(screen.getByRole('tab', { name: 'Mine' })).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Delete watchlist Mine' }));
    fireEvent.click(screen.getByRole('button', { name: 'Delete' }));
    expect(await screen.findByRole('tab', { name: 'Watchlist 1' })).toHaveAttribute('aria-selected', 'true');
    expect(backend.callsTo('watchlists.delete')[0].args).toEqual(['w1']);
  });
});

describe('WatchlistPanel — tablist keyboard (WAI-ARIA tabs, roving focus)', () => {
  it('only the active tab is tabbable; arrows wrap and activate, Home/End jump', async () => {
    seed(lists('A', 'B', 'C'));
    renderPanel();
    const a = await screen.findByRole('tab', { name: 'A' });
    const tab = (n: string) => screen.getByRole('tab', { name: n });
    expect(a).toHaveAttribute('tabindex', '0');
    expect(tab('B')).toHaveAttribute('tabindex', '-1');
    a.focus();
    fireEvent.keyDown(a, { key: 'ArrowRight' });
    expect(tab('B')).toHaveFocus();
    await waitFor(() => expect(backend.state.watchlists.activeId).toBe('w2'));
    expect(tab('B')).toHaveAttribute('aria-selected', 'true');
    expect(tab('B')).toHaveAttribute('tabindex', '0');
    fireEvent.keyDown(tab('B'), { key: 'End' });
    expect(tab('C')).toHaveFocus();
    fireEvent.keyDown(tab('C'), { key: 'ArrowRight' });
    expect(tab('A')).toHaveFocus();
    fireEvent.keyDown(tab('A'), { key: 'ArrowLeft' });
    expect(tab('C')).toHaveFocus();
    fireEvent.keyDown(tab('C'), { key: 'Home' });
    expect(tab('A')).toHaveFocus();
    await waitFor(() => expect(backend.state.watchlists.activeId).toBe('w1'));
  });

  it('the tabs control the panel of the active list', async () => {
    seed(lists('A', 'B'));
    renderPanel();
    const a = await screen.findByRole('tab', { name: 'A' });
    const panel = screen.getByRole('tabpanel', { name: 'A' });
    expect(a).toHaveAttribute('aria-controls', panel.id);
    expect(within(panel).getByRole('grid', { name: 'A instruments' })).toBeInTheDocument();
  });

  it('F2 renames the focused tab; Escape cancels and returns focus to the tab', async () => {
    seed(lists('A', 'B'));
    renderPanel();
    const b = await screen.findByRole('tab', { name: 'B' });
    fireEvent.keyDown(b, { key: 'F2' });
    const field = screen.getByRole('textbox', { name: 'Watchlist name' });
    expect(field).toHaveFocus();
    expect(field).toHaveValue('B');
    fireEvent.keyDown(field, { key: 'Escape' });
    expect(screen.queryByRole('textbox', { name: 'Watchlist name' })).toBeNull();
    expect(screen.getByRole('tab', { name: 'B' })).toHaveFocus();
  });

  it('the context menu closes on Escape and returns focus to its tab', async () => {
    seed(lists('A'));
    renderPanel();
    fireEvent.contextMenu(await screen.findByRole('tab', { name: 'A' }));
    const item = screen.getByRole('menuitem', { name: 'Rename' });
    expect(item).toHaveFocus();
    fireEvent.keyDown(item, { key: 'Escape' });
    expect(screen.queryByRole('menu')).toBeNull();
    expect(screen.getByRole('tab', { name: 'A' })).toHaveFocus();
  });
});

describe('WatchlistPanel — loading, errors and refusals', () => {
  it('Ruling 102 I3: the mount effect loads only while the store is idle (the event bridge owns loading)', async () => {
    useWatchlists.setState({ lists: lists('Ready'), activeId: 'w1', status: 'ready' });
    renderPanel();
    expect(await screen.findByRole('tab', { name: 'Ready' })).toBeInTheDocument();
    expect(backend.callsTo('watchlists.list')).toHaveLength(0);
  });

  it('a failed load shows the reason and Retry; Retry keeps focus while busy and then shows the lists', async () => {
    seed(lists('Watchlist 1'));
    backend.failNext('watchlists.list', 'Network unavailable');
    renderPanel();
    const alert = await screen.findByRole('alert');
    expect(alert).toHaveTextContent('Could not load watchlists: Network unavailable');
    const retry = within(alert).getByRole('button', { name: 'Retry' });
    retry.focus();
    let release!: () => void;
    const gate = new Promise<void>((r) => { release = r; });
    const list = backend.watchlists.list.bind(backend.watchlists);
    backend.watchlists.list = async () => { await gate; return list(); };
    fireEvent.click(retry);
    expect(retry).toHaveAttribute('aria-disabled', 'true');
    expect(retry).toHaveFocus();
    fireEvent.click(retry); // ignored while busy
    await act(async () => release());
    expect(await screen.findByRole('tab', { name: 'Watchlist 1' })).toBeInTheDocument();
    expect(screen.queryByRole('alert')).toBeNull();
  });

  it("Ruling 87: Go's invalid_name refusal on rename (local rules pass) shows Go's reason", async () => {
    seed(lists('A', 'B'));
    renderPanel();
    fireEvent.doubleClick(await screen.findByRole('tab', { name: 'B' }));
    backend.failNext('watchlists.rename', 'Watchlist names can be up to 40 characters.', 'invalid_name');
    const field = screen.getByRole('textbox', { name: 'Watchlist name' });
    fireEvent.change(field, { target: { value: 'Core' } });
    fireEvent.keyDown(field, { key: 'Enter' });
    expect(await screen.findByRole('alert')).toHaveTextContent('Watchlist names can be up to 40 characters.');
    expect(screen.getByRole('textbox', { name: 'Watchlist name' })).toBeInTheDocument();
  });

  it("Ruling 87: Go's duplicate_name refusal on create shows the validator's text and keeps the field", async () => {
    seed(lists('A'));
    renderPanel();
    fireEvent.click(await screen.findByRole('button', { name: 'New watchlist +' }));
    backend.failNext('watchlists.create', 'duplicate', 'duplicate_name');
    const field = screen.getByRole('textbox', { name: 'New watchlist name' });
    fireEvent.change(field, { target: { value: 'Core' } });
    fireEvent.keyDown(field, { key: 'Enter' });
    expect(await screen.findByRole('alert')).toHaveTextContent('A watchlist with this name exists');
    expect(backend.callsTo('watchlists.create')).toHaveLength(1);
    expect(screen.getByRole('textbox', { name: 'New watchlist name' })).toHaveValue('Core');
  });

  it('a refused tab switch puts the previous tab back and says why', async () => {
    seed(lists('A', 'B'));
    renderPanel();
    backend.failNext('watchlists.setActive', 'That watchlist no longer exists.', 'not_found');
    fireEvent.click(await screen.findByRole('tab', { name: 'B' }));
    await waitFor(() => expect(useToastStore.getState().visible[0]?.message).toMatch(/^Could not switch watchlist/));
    expect(screen.getByRole('tab', { name: 'A' })).toHaveAttribute('aria-selected', 'true');
  });

  it('a one-item list says "1 instrument" in the delete question', async () => {
    seed([{ id: 'w1', name: 'Solo', items: [watchlistItemOf(SBIN)] }]);
    renderPanel();
    fireEvent.click(await screen.findByRole('button', { name: 'Delete watchlist Solo' }));
    expect(screen.getByText('Delete watchlist Solo and its 1 instrument?')).toBeInTheDocument();
  });

  it('G-0: shows no order buttons', async () => {
    seed(lists('A'));
    renderPanel();
    await screen.findByRole('tab', { name: 'A' });
    expect(screen.queryByRole('button', { name: /^(buy|sell|b|s)\b/i })).toBeNull();
  });
});
