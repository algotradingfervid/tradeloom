import { screen, waitFor, within } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { demoSeed } from '../../api/mock/fixtures';
import type { CalendarDayType } from '../../api/backend';
import { createMockBackend, type MockState } from '../../api/mock/index';
import { istDayKey } from '../../i18n/format';
import { resetAllStores } from '../../store/reset';
import { renderApp } from '../../test/render';

const staleSeed = (): MockState => {
  const s = demoSeed();
  s.calendar = { ...s.calendar, stale: true, coversUntil: '2026-10-31' };
  return s;
};

describe('FR-DATA-001 Settings → Market calendar', () => {
  it('lists entries with DD-MM-YYYY dates, type and session window', async () => {
    renderApp({ route: '/settings/calendar' });
    const holiday = await screen.findByRole('row', { name: /^20-10-2026/ });
    expect(holiday).toHaveTextContent('Holiday');
    const special = screen.getByRole('row', { name: /^08-11-2026/ });
    expect(special).toHaveTextContent('Special session');
    expect(special).toHaveTextContent('18:15–19:15');
    expect(screen.getByText('31 Dec 2026')).toBeInTheDocument();
  });
  it('AC6: shows "Calendar may be stale" and warns once per day', async () => {
    const backend = createMockBackend(staleSeed());
    const first = renderApp({ route: '/settings/calendar', backend });
    expect(await screen.findByText('Calendar may be stale')).toBeInTheDocument();
    expect(await screen.findByText(/Calendar may be stale — it ends on 31 Oct 2026/)).toBeInTheDocument();
    await waitFor(() => expect(backend.state.settings['calendar.stale_toast_day']).toBe(JSON.stringify(istDayKey(Date.now()))));
    first.unmount();
    resetAllStores();
    const before = backend.callsTo('calendar.info').length;
    renderApp({ route: '/home', backend });
    await screen.findByRole('button', { name: 'Account menu' });
    await waitFor(() => expect(backend.callsTo('calendar.info').length).toBeGreaterThan(before));
    expect(screen.queryByText(/Calendar may be stale — it ends/)).toBeNull();
  });
  it('AC4: a refused import is explained inline and as a toast; the table is unchanged', async () => {
    const backend = createMockBackend(demoSeed());
    backend.failNext('calendar.import', 'Row 7: invalid time', 'invalid_file');
    const { user } = renderApp({ route: '/settings/calendar', backend });
    await screen.findByRole('row', { name: /^20-10-2026/ });
    await user.click(screen.getByRole('button', { name: 'Import CSV…' }));
    expect(await screen.findAllByText(/Row 7: invalid time/)).toHaveLength(2);
    expect(screen.getAllByRole('row', { name: /^\d\d-\d\d-\d{4}/ })).toHaveLength(2);
  });
  it('adds a holiday through the entry dialog', async () => {
    const { user, backend } = renderApp({ route: '/settings/calendar' });
    await user.click(await screen.findByRole('button', { name: 'Add entry' }));
    const dialog = screen.getByRole('dialog', { name: 'Add calendar entry' });
    await user.type(within(dialog).getByLabelText('Date'), '2026-12-25');
    await user.click(within(dialog).getByRole('button', { name: 'Save' }));
    expect(backend.callsTo('calendar.upsert')[0].args[0]).toEqual({ exchange: 'NSE', date: '2026-12-25', type: 'holiday', phases: [] });
    expect(await screen.findByRole('row', { name: /^25-12-2026/ })).toHaveTextContent('Added by you');
  });
  it('a special session needs a start before its end', async () => {
    const { user, backend } = renderApp({ route: '/settings/calendar' });
    await user.click(await screen.findByRole('button', { name: 'Add entry' }));
    const dialog = screen.getByRole('dialog', { name: 'Add calendar entry' });
    await user.type(within(dialog).getByLabelText('Date'), '2026-11-08');
    await user.selectOptions(within(dialog).getByLabelText('Type'), 'special');
    await user.click(within(dialog).getByRole('button', { name: 'Save' }));
    expect(within(dialog).getByRole('alert')).toHaveTextContent('Enter a start time before the end time');
    expect(backend.callsTo('calendar.upsert')).toHaveLength(0);
  });
  it('saves a special session as one continuous phase', async () => {
    const { user, backend } = renderApp({ route: '/settings/calendar' });
    await user.click(await screen.findByRole('button', { name: 'Add entry' }));
    const dialog = screen.getByRole('dialog', { name: 'Add calendar entry' });
    await user.type(within(dialog).getByLabelText('Date'), '2026-11-09');
    await user.selectOptions(within(dialog).getByLabelText('Type'), 'special');
    await user.type(within(dialog).getByLabelText('Start'), '18:00');
    await user.type(within(dialog).getByLabelText('End'), '19:00');
    await user.click(within(dialog).getByRole('button', { name: 'Save' }));
    expect(backend.callsTo('calendar.upsert')[0].args[0]).toEqual({
      exchange: 'NSE', date: '2026-11-09', type: 'special', phases: [{ kind: 'continuous', start: '18:00', end: '19:00' }],
    });
  });
  it('removes an entry from the keyboard via the row actions', async () => {
    const { user, backend } = renderApp({ route: '/settings/calendar' });
    const row = await screen.findByRole('row', { name: /^20-10-2026/ });
    row.focus();
    await user.keyboard('{Shift>}{F10}{/Shift}');
    await user.click(screen.getByRole('menuitem', { name: 'Remove' }));
    await user.click(within(screen.getByRole('dialog', { name: 'Remove calendar entry?' })).getByRole('button', { name: 'Remove' }));
    expect(backend.callsTo('calendar.remove')[0].args).toEqual(['NSE', '2026-10-20']);
    await waitFor(() => expect(screen.queryByRole('row', { name: /^20-10-2026/ })).toBeNull());
  });

  describe('Go calendar facade facts (P0-3 Task 31, Ruling 77)', () => {
    it('refuses a session without a continuous phase before calling Go', async () => {
      const { user, backend } = renderApp({ route: '/settings/calendar' });
      await user.click(await screen.findByRole('button', { name: 'Add entry' }));
      const dialog = screen.getByRole('dialog', { name: 'Add calendar entry' });
      await user.type(within(dialog).getByLabelText('Date'), '2026-11-09');
      await user.selectOptions(within(dialog).getByLabelText('Type'), 'special');
      await user.selectOptions(within(dialog).getByLabelText('Session phase'), 'pre-open');
      await user.type(within(dialog).getByLabelText('Start'), '18:00');
      await user.type(within(dialog).getByLabelText('End'), '18:08');
      await user.click(within(dialog).getByRole('button', { name: 'Save' }));
      expect(within(dialog).getByRole('alert')).toHaveTextContent('A session needs a continuous trading phase');
      expect(backend.callsTo('calendar.upsert')).toHaveLength(0);
    });
    it('a trading day without times is saved with no phases (regular session)', async () => {
      const { user, backend } = renderApp({ route: '/settings/calendar' });
      await user.click(await screen.findByRole('button', { name: 'Add entry' }));
      const dialog = screen.getByRole('dialog', { name: 'Add calendar entry' });
      await user.type(within(dialog).getByLabelText('Date'), '2026-11-14');
      await user.selectOptions(within(dialog).getByLabelText('Type'), 'trading');
      await user.click(within(dialog).getByRole('button', { name: 'Save' }));
      expect(backend.callsTo('calendar.upsert')[0].args[0]).toEqual({ exchange: 'NSE', date: '2026-11-14', type: 'trading', phases: [] });
    });
    it('editing the first phase into another kind keeps the other phases without duplicating the old one', async () => {
      const seed = demoSeed();
      seed.calendar.entries = [{
        exchange: 'NSE', date: '2026-11-08', type: 'special', source: 'shipped',
        phases: [{ kind: 'pre-open', start: '18:00', end: '18:08' }, { kind: 'continuous', start: '18:15', end: '19:15' }],
      }];
      const { user, backend } = renderApp({ route: '/settings/calendar', backend: createMockBackend(seed) });
      const row = await screen.findByRole('row', { name: /^08-11-2026/ });
      row.focus();
      await user.keyboard('{Shift>}{F10}{/Shift}');
      await user.click(screen.getByRole('menuitem', { name: 'Edit' }));
      const dialog = screen.getByRole('dialog', { name: 'Edit calendar entry' });
      await user.selectOptions(within(dialog).getByLabelText('Session phase'), 'closing');
      await user.click(within(dialog).getByRole('button', { name: 'Save' }));
      // pre-open (the edited phase) becomes closing; continuous is kept once.
      expect(backend.callsTo('calendar.upsert')[0].args[0]).toEqual({
        exchange: 'NSE', date: '2026-11-08', type: 'special',
        phases: [{ kind: 'closing', start: '18:00', end: '18:08' }, { kind: 'continuous', start: '18:15', end: '19:15' }],
      });
    });
    it('shows an inferred non-trading day with its own label and source', async () => {
      const seed = demoSeed();
      seed.calendar.entries = [
        ...seed.calendar.entries,
        { exchange: 'NSE', date: '2026-10-27', type: 'inferred-non-trading' as CalendarDayType, phases: [], source: 'inferred' },
      ];
      renderApp({ route: '/settings/calendar', backend: createMockBackend(seed) });
      const row = await screen.findByRole('row', { name: /^27-10-2026/ });
      expect(row).toHaveTextContent('Non-trading day (inferred)');
      expect(row).toHaveTextContent('Inferred');
    });
    it('a refused remove shows Go\'s reason verbatim and re-reads the calendar', async () => {
      const backend = createMockBackend(demoSeed());
      backend.failNext('calendar.remove', 'No override for NSE on 2026-10-20.', 'not_found');
      const { user } = renderApp({ route: '/settings/calendar', backend });
      const row = await screen.findByRole('row', { name: /^20-10-2026/ });
      const reads = backend.callsTo('calendar.info').length;
      row.focus();
      await user.keyboard('{Shift>}{F10}{/Shift}');
      await user.click(screen.getByRole('menuitem', { name: 'Remove' }));
      await user.click(within(screen.getByRole('dialog', { name: 'Remove calendar entry?' })).getByRole('button', { name: 'Remove' }));
      expect(await screen.findByText('No override for NSE on 2026-10-20.')).toBeInTheDocument();
      await waitFor(() => expect(backend.callsTo('calendar.info').length).toBeGreaterThan(reads));
    });
    it('a successful import replaces the table, toasts, and never shows the chosen path', async () => {
      const backend = createMockBackend(demoSeed());
      const { user } = renderApp({ route: '/settings/calendar', backend });
      await screen.findByRole('row', { name: /^20-10-2026/ });
      backend.state.calendar.entries.push({ exchange: 'BSE', date: '2026-12-25', type: 'holiday', phases: [], source: 'import' });
      await user.click(screen.getByRole('button', { name: 'Import CSV…' }));
      expect(await screen.findByText('Calendar imported')).toBeInTheDocument();
      expect(backend.callsTo('calendar.import')[0].args).toEqual([backend.state.fileChoice]);
      expect(await screen.findByRole('row', { name: /^25-12-2026 BSE/ })).toHaveTextContent('Imported');
      expect(document.body.textContent).not.toContain(backend.state.fileChoice);
    });
    it('a cancelled file choice imports nothing', async () => {
      const backend = createMockBackend(demoSeed());
      backend.state.fileChoice = '';
      const { user } = renderApp({ route: '/settings/calendar', backend });
      await user.click(await screen.findByRole('button', { name: 'Import CSV…' }));
      await waitFor(() => expect(backend.callsTo('calendar.chooseImportFile')).toHaveLength(1));
      expect(backend.callsTo('calendar.import')).toHaveLength(0);
    });
  });
});
