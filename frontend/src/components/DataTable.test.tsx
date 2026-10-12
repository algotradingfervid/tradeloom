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
