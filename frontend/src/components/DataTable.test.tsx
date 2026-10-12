import { fireEvent, render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';
import { DataTable, type Column, type RowAction } from './DataTable';

interface Row { id: string; name: string }
const rows: Row[] = [{ id: '1', name: 'RELIANCE' }, { id: '2', name: 'TCS' }, { id: '3', name: 'INFY' }];
const columns: Column<Row>[] = [{ id: 'name', header: 'Symbol', cell: (r) => r.name, title: (r) => r.name }];

function setup() {
  const onChart = vi.fn();
  const onRemove = vi.fn();
  const actions: RowAction<Row>[] = [
    { id: 'chart', label: 'Open chart', icon: 'positions', onSelect: onChart },
    { id: 'remove', label: 'Remove', icon: 'trash', onSelect: onRemove },
  ];
  render(
    <>
      <button>before</button>
      <DataTable caption="Test rows" columns={columns} rows={rows} rowKey={(r) => r.id} rowLabel={(r) => r.name} rowActions={actions} />
    </>,
  );
  return { onChart, onRemove, user: userEvent.setup() };
}

describe('DataTable — review focus: keyboard-only user (FR-SHELL-014, FR-SHELL-015 AC1)', () => {
  it('Tab enters the active row, arrows move rows, Tab then reaches that row’s actions', async () => {
    const { user, onChart } = setup();
    screen.getByText('before').focus();
    await user.tab();
    expect(screen.getByRole('row', { name: 'RELIANCE' })).toHaveFocus();
    await user.keyboard('{ArrowDown}');
    expect(screen.getByRole('row', { name: 'TCS' })).toHaveFocus();
    await user.tab();
    expect(screen.getByRole('button', { name: 'Open chart TCS' })).toHaveFocus();
    await user.keyboard('{Enter}');
    expect(onChart).toHaveBeenCalledWith(rows[1]);
  });
  it('Home and End jump to the first and last rows', async () => {
    const { user } = setup();
    screen.getByRole('row', { name: 'RELIANCE' }).focus();
    await user.keyboard('{End}');
    expect(screen.getByRole('row', { name: 'INFY' })).toHaveFocus();
    await user.keyboard('{Home}');
    expect(screen.getByRole('row', { name: 'RELIANCE' })).toHaveFocus();
  });
  it('Shift+F10 opens a context menu with the same actions; Escape returns focus to the row', async () => {
    const { user, onRemove } = setup();
    screen.getByRole('row', { name: 'TCS' }).focus();
    await user.keyboard('{Shift>}{F10}{/Shift}');
    const menu = screen.getByRole('menu', { name: 'Actions for TCS' });
    expect(screen.getByRole('menuitem', { name: 'Open chart' })).toHaveFocus();
    await user.keyboard('{Escape}');
    expect(menu).not.toBeInTheDocument();
    expect(screen.getByRole('row', { name: 'TCS' })).toHaveFocus();
    await user.keyboard('{ContextMenu}');
    await user.keyboard('{ArrowDown}{Enter}');
    expect(onRemove).toHaveBeenCalledWith(rows[1]);
  });
  it('right-click opens the same menu', () => {
    setup();
    fireEvent.contextMenu(screen.getByRole('row', { name: 'INFY' }));
    expect(screen.getByRole('menu', { name: 'Actions for INFY' })).toBeInTheDocument();
  });
  it('truncating cells carry the full text in title (long instrument names)', () => {
    setup();
    expect(screen.getByText('RELIANCE')).toHaveAttribute('title', 'RELIANCE');
  });
});

describe('DataTable — long names and empty data (review focus 1)', () => {
  const longName = 'NIFTY INFRASTRUCTURE AND CONSTRUCTION LONG DATED BOND EXCHANGE TRADED FUND GROWTH OPTION';
  it('a 60+ character name renders in a truncating cell whose title holds the full text', () => {
    expect(longName.length).toBeGreaterThanOrEqual(60);
    render(<DataTable caption="Long" columns={columns} rows={[{ id: 'l', name: longName }]} rowKey={(r) => r.id} rowLabel={(r) => r.name} />);
    const cell = screen.getByText(longName);
    expect(cell).toHaveAttribute('title', longName);
    expect(cell).toHaveClass('cell-truncate');
    expect(screen.getByRole('table', { name: 'Long' })).toHaveClass('data-table');
  });
  it('keeps table semantics: caption names the table and headers are column headers', () => {
    setup();
    const table = screen.getByRole('table', { name: 'Test rows' });
    expect(within(table).getByRole('columnheader', { name: 'Symbol' })).toHaveAttribute('scope', 'col');
    expect(within(table).getByRole('columnheader', { name: 'Actions' })).toBeInTheDocument();
  });
  it('an empty rows array renders headers and no body rows, and Tab passes through', async () => {
    const user = userEvent.setup();
    render(
      <>
        <button>before</button>
        <DataTable caption="Empty" columns={columns} rows={[]} rowKey={(r) => r.id} rowLabel={(r) => r.name} />
        <button>after</button>
      </>,
    );
    expect(screen.getAllByRole('row')).toHaveLength(1);
    screen.getByText('before').focus();
    await user.tab();
    expect(screen.getByText('after')).toHaveFocus();
  });
  it('the active row stays tabbable when the rows shrink below its index', async () => {
    const user = userEvent.setup();
    const { rerender } = render(<DataTable caption="Shrink" columns={columns} rows={rows} rowKey={(r) => r.id} rowLabel={(r) => r.name} />);
    screen.getByRole('row', { name: 'RELIANCE' }).focus();
    await user.keyboard('{End}');
    rerender(<DataTable caption="Shrink" columns={columns} rows={[rows[0]]} rowKey={(r) => r.id} rowLabel={(r) => r.name} />);
    expect(screen.getByRole('row', { name: 'RELIANCE' })).toHaveAttribute('tabindex', '0');
  });
});

describe('DataTable — Ruling 84 fix round 1', () => {
  function Live({ list, actions }: { list: Row[]; actions: RowAction<Row>[] }) {
    return <DataTable caption="Live" columns={columns} rows={list} rowKey={(r) => r.id} rowLabel={(r) => r.name} rowActions={actions} />;
  }
  function actionsWith(onChart = vi.fn()): RowAction<Row>[] {
    return [
      { id: 'chart', label: 'Open chart', icon: 'positions', onSelect: onChart },
      { id: 'remove', label: 'Remove', icon: 'trash', onSelect: vi.fn() },
    ];
  }

  it('(I1) a live update removing a row above the open menu keeps the menu on the same instrument', async () => {
    const user = userEvent.setup();
    const onChart = vi.fn();
    const actions = actionsWith(onChart);
    const { rerender } = render(<Live list={rows} actions={actions} />);
    screen.getByRole('row', { name: 'TCS' }).focus();
    await user.keyboard('{Shift>}{F10}{/Shift}');
    rerender(<Live list={[rows[1], rows[2]]} actions={actions} />);
    expect(screen.getByRole('menu', { name: 'Actions for TCS' })).toBeInTheDocument();
    expect(screen.queryByRole('menu', { name: 'Actions for INFY' })).toBeNull();
    expect(screen.getByRole('row', { name: 'TCS' })).toHaveAttribute('tabindex', '0');
    screen.getByRole('menuitem', { name: 'Open chart' }).focus();
    await user.keyboard('{Enter}');
    expect(onChart).toHaveBeenCalledWith(rows[1]);
  });

  it('(I1) the menu closes when its own row disappears, and focus returns to a row', async () => {
    const user = userEvent.setup();
    const actions = actionsWith();
    const { rerender } = render(<Live list={rows} actions={actions} />);
    screen.getByRole('row', { name: 'TCS' }).focus();
    await user.keyboard('{Shift>}{F10}{/Shift}');
    rerender(<Live list={[rows[0], rows[2]]} actions={actions} />);
    expect(screen.queryByRole('menu')).toBeNull();
    rerender(<Live list={rows} actions={actions} />);
    expect(screen.queryByRole('menu')).toBeNull();
    expect(document.activeElement?.getAttribute('role') ?? document.activeElement?.tagName).toBe('TR');
  });

  it('(I1) the active row follows its key when rows above it are removed', async () => {
    const user = userEvent.setup();
    const actions = actionsWith();
    const { rerender } = render(<Live list={rows} actions={actions} />);
    screen.getByRole('row', { name: 'RELIANCE' }).focus();
    await user.keyboard('{ArrowDown}');
    rerender(<Live list={[rows[1], rows[2]]} actions={actions} />);
    expect(screen.getByRole('row', { name: 'TCS' })).toHaveAttribute('tabindex', '0');
    expect(screen.getByRole('row', { name: 'INFY' })).toHaveAttribute('tabindex', '-1');
  });

  it('(M2) Tab in the portalled menu closes it and returns focus to its row', async () => {
    const user = userEvent.setup();
    render(<Live list={rows} actions={actionsWith()} />);
    screen.getByRole('row', { name: 'TCS' }).focus();
    await user.keyboard('{Shift>}{F10}{/Shift}');
    await user.tab();
    expect(screen.queryByRole('menu')).toBeNull();
    expect(screen.getByRole('row', { name: 'TCS' })).toHaveFocus();
  });

  it('(M1) the actions column fits n 32 px buttons, n-1 4 px gaps and 16 px padding', () => {
    const { container } = render(<Live list={rows} actions={actionsWith()} />);
    const cols = container.querySelectorAll('col');
    expect(cols[cols.length - 1]).toHaveStyle({ width: '84px' });
  });

  it('(M2) the row menu renders in a portal and opens upward near the bottom of the window', async () => {
    const user = userEvent.setup();
    vi.spyOn(HTMLElement.prototype, 'getBoundingClientRect').mockReturnValue(
      { top: window.innerHeight - 40, bottom: window.innerHeight - 4, left: 900, right: 1000, width: 100, height: 36, x: 900, y: window.innerHeight - 40, toJSON: () => ({}) } as DOMRect,
    );
    const { container } = render(<Live list={rows} actions={actionsWith()} />);
    screen.getByRole('row', { name: 'INFY' }).focus();
    await user.keyboard('{Shift>}{F10}{/Shift}');
    const menu = screen.getByRole('menu', { name: 'Actions for INFY' });
    expect(container.contains(menu)).toBe(false);
    expect(menu).toHaveAttribute('data-placement', 'up');
  });

  it('(M2) the row menu opens downward when there is room', async () => {
    const user = userEvent.setup();
    render(<Live list={rows} actions={actionsWith()} />);
    screen.getByRole('row', { name: 'RELIANCE' }).focus();
    await user.keyboard('{Shift>}{F10}{/Shift}');
    expect(screen.getByRole('menu', { name: 'Actions for RELIANCE' })).toHaveAttribute('data-placement', 'down');
  });
});
