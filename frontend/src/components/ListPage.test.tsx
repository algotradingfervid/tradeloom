import { act, render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { useState } from 'react';
import { createMemoryRouter, Outlet, RouterProvider } from 'react-router-dom';
import { describe, expect, it, vi } from 'vitest';
import { useTrackHistoryDepth } from '../store/nav';
import { DataTable } from './DataTable';
import { filterRows } from './filterRows';
import { ListBody } from './ListBody';
import { ListPage } from './ListPage';

interface Item { id: string; symbol: string; name: string }
const items: Item[] = [
  ...Array.from({ length: 18 }, (_, i) => ({ id: `s${i}`, symbol: `SYM${i}`, name: `Company ${i}` })),
  { id: 'tcs', symbol: 'TCS', name: 'Tata Consultancy' },
  { id: 'x', symbol: 'NETX', name: 'Abtcs Networks' },
];

function SearchHost() {
  const [q, setQ] = useState('');
  const visible = filterRows(items, q, (r) => [r.symbol, r.name]);
  return (
    <ListPage title="Holdings" description="Your holdings" search={{ value: q, onChange: setQ }}>
      <DataTable caption="Holdings" columns={[{ id: 's', header: 'Symbol', cell: (r) => r.symbol }]} rows={visible} rowKey={(r) => r.id} rowLabel={(r) => r.symbol} />
    </ListPage>
  );
}

function Tracker() {
  useTrackHistoryDepth();
  return <Outlet />;
}

describe('FR-SHELL-014 list template', () => {
  it('AC2: typing "TCS" keeps only rows whose symbol or name contains "tcs"', async () => {
    const user = userEvent.setup();
    render(<RouterProvider router={createMemoryRouter([{ path: '/', element: <SearchHost /> }])} />);
    expect(screen.getAllByRole('row')).toHaveLength(21);
    await user.type(screen.getByRole('searchbox', { name: 'Search holdings' }), 'TCS');
    const body = screen.getAllByRole('rowgroup')[1];
    expect(within(body).getAllByRole('row').map((r) => r.textContent)).toEqual(['TCS', 'NETX']);
  });
  it('AC3: a failed load shows the error state with Retry instead of the empty state', async () => {
    const onRetry = vi.fn();
    const user = userEvent.setup();
    render(<ListBody status="error" error="Network unavailable" onRetry={onRetry} empty={{ message: 'You have no holdings' }} loadingLabel="Loading holdings" />);
    expect(screen.getByRole('alert')).toHaveTextContent('Network unavailable');
    expect(screen.queryByText('You have no holdings')).toBeNull();
    await user.click(screen.getByRole('button', { name: 'Retry' }));
    expect(onRetry).toHaveBeenCalledTimes(1);
  });
  it('AC1: the empty state shows the view-specific message and at most one call to action', () => {
    render(<ListBody status="empty" empty={{ message: 'No holdings', action: { label: 'Refresh', onSelect: vi.fn() } }} loadingLabel="Loading holdings" />);
    expect(screen.getByText('No holdings')).toBeInTheDocument();
    expect(screen.getAllByRole('button')).toHaveLength(1);
  });
  it('shows skeleton rows while loading', () => {
    render(<ListBody status="loading" empty={{ message: 'No holdings' }} loadingLabel="Loading holdings" />);
    expect(screen.getByRole('status', { name: 'Loading holdings' })).toBeInTheDocument();
  });
  it('AC4: the back arrow restores the previous view, and goes Home when there is none', async () => {
    const user = userEvent.setup();
    const router = createMemoryRouter(
      [{
        element: <Tracker />,
        children: [
          { path: '/a', element: <ListPage title="Orders" description="d"><p>orders body</p></ListPage> },
          { path: '/b', element: <ListPage title="Positions" description="d"><p>positions body</p></ListPage> },
          { path: '/home', element: <p>home body</p> },
        ],
      }],
      { initialEntries: ['/a'] },
    );
    render(<RouterProvider router={router} />);
    await act(() => router.navigate('/b'));
    await user.click(screen.getByRole('button', { name: 'Back' }));
    expect(screen.getByRole('heading', { name: 'Orders' })).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'Back' }));
    expect(screen.getByText('home body')).toBeInTheDocument();
  });
});

describe('ListPage refresh (Ruling 22: no stranded focus on a disabled control)', () => {
  it('Refresh keeps focus while refreshing and ignores repeat activation', async () => {
    const onRefresh = vi.fn();
    const user = userEvent.setup();
    function Host() {
      const [busy, setBusy] = useState(false);
      return (
        <ListPage title="Holdings" description="d" refreshing={busy} onRefresh={() => { onRefresh(); setBusy(true); }}>
          <p>body</p>
        </ListPage>
      );
    }
    render(<RouterProvider router={createMemoryRouter([{ path: '/', element: <Host /> }])} />);
    const btn = screen.getByRole('button', { name: 'Refresh holdings' });
    await user.click(btn);
    expect(btn).toHaveFocus();
    expect(btn).toHaveAttribute('aria-disabled', 'true');
    expect(btn).toHaveAttribute('aria-busy', 'true');
    await user.click(btn);
    await user.keyboard('{Enter}');
    await user.keyboard(' ');
    expect(btn).toHaveFocus();
    expect(onRefresh).toHaveBeenCalledTimes(1);
  });
});

describe('ListPage / ListBody — Ruling 84 fix round 1', () => {
  it('(M4) an error without a message falls back to generic copy', () => {
    render(<ListBody status="error" error={null} onRetry={vi.fn()} empty={{ message: 'No holdings' }} loadingLabel="Loading holdings" />);
    expect(screen.getByRole('alert')).toHaveTextContent('Something went wrong while loading. Try again.');
  });
  it('(M5) a search that hides every row shows "no matches" in a polite live region', async () => {
    const user = userEvent.setup();
    function Host() {
      const [q, setQ] = useState('');
      const visible = filterRows(items, q, (r) => [r.symbol, r.name]);
      return (
        <ListPage title="Holdings" description="d" search={{ value: q, onChange: setQ, matches: visible.length }}>
          <DataTable caption="Holdings" columns={[{ id: 's', header: 'Symbol', cell: (r) => r.symbol }]} rows={visible} rowKey={(r) => r.id} rowLabel={(r) => r.symbol} />
        </ListPage>
      );
    }
    render(<RouterProvider router={createMemoryRouter([{ path: '/', element: <Host /> }])} />);
    const region = screen.getByRole('status');
    expect(region).toHaveAttribute('aria-live', 'polite');
    expect(region).toBeEmptyDOMElement();
    await user.type(screen.getByRole('searchbox', { name: 'Search holdings' }), 'zzz');
    expect(region).toHaveTextContent('No matches for “zzz”');
    await user.clear(screen.getByRole('searchbox', { name: 'Search holdings' }));
    expect(region).toBeEmptyDOMElement();
  });
});
