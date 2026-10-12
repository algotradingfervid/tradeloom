import { screen, waitFor, within } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { demoSeed } from '../../api/mock/fixtures';
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
  it('is the Market calendar group of the Settings view (Task 14 shell)', async () => {
    renderApp({ route: '/settings/calendar' });
    expect(await screen.findByRole('heading', { level: 1, name: 'Market calendar' })).toBeInTheDocument();
    const nav = screen.getByRole('navigation', { name: 'Settings groups' });
    expect(within(nav).getByRole('link', { name: 'Market calendar' })).toHaveAttribute('aria-current', 'page');
    expect(screen.getByRole('heading', { level: 2, name: 'Trading days and special sessions' })).toBeInTheDocument();
  });
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
        { exchange: 'NSE', date: '2026-10-27', type: 'inferred-non-trading', phases: [], source: 'inferred' },
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
      const seed = demoSeed();
      seed.calendarCsv = 'date,exchange,type,phase,start,end\n2026-12-25,BSE,holiday,,,\n';
      const backend = createMockBackend(seed);
      const { user } = renderApp({ route: '/settings/calendar', backend });
      await screen.findByRole('row', { name: /^20-10-2026/ });
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

  describe('fix round 1 (Ruling 100)', () => {
    const twoPhaseSeed = () => {
      const seed = demoSeed();
      seed.calendar.entries = [{
        exchange: 'NSE', date: '2026-11-08', type: 'special', source: 'import',
        phases: [{ kind: 'pre-open', start: '18:00', end: '18:08' }, { kind: 'continuous', start: '18:15', end: '19:15' }],
      }];
      return createMockBackend(seed);
    };
    const openEdit = async (user: ReturnType<typeof renderApp>['user'], name: RegExp, dialogName = 'Edit calendar entry') => {
      const row = await screen.findByRole('row', { name });
      row.focus();
      await user.keyboard('{Shift>}{F10}{/Shift}');
      await user.click(screen.getByRole('menuitem', { name: 'Edit' }));
      return screen.getByRole('dialog', { name: dialogName });
    };

    it('I1: switching a phase to a kind another phase already has is refused inline; no phase is dropped', async () => {
      const backend = twoPhaseSeed();
      const { user } = renderApp({ route: '/settings/calendar', backend });
      const dialog = await openEdit(user, /^08-11-2026/);
      await user.selectOptions(within(dialog).getByLabelText('Session phase'), 'continuous');
      await user.click(within(dialog).getByRole('button', { name: 'Save' }));
      expect(within(dialog).getByLabelText('Session phase')).toHaveAccessibleDescription('Each phase kind appears at most once.');
      expect(backend.callsTo('calendar.upsert')).toHaveLength(0);
    });
    it('I2: the remove confirmation says the bundled calendar applies again', async () => {
      const { user } = renderApp({ route: '/settings/calendar' });
      const row = await screen.findByRole('row', { name: /^20-10-2026/ });
      row.focus();
      await user.keyboard('{Shift>}{F10}{/Shift}');
      await user.click(screen.getByRole('menuitem', { name: 'Remove' }));
      const dialog = screen.getByRole('dialog', { name: 'Remove calendar entry?' });
      expect(dialog).toHaveTextContent('The entry for 20-10-2026 is removed. The bundled calendar applies to that day again.');
    });
    it('M1: field errors are attached to their fields; Go refusals are a form-level alert', async () => {
      const backend = createMockBackend(demoSeed());
      const { user } = renderApp({ route: '/settings/calendar', backend });
      await user.click(await screen.findByRole('button', { name: 'Add entry' }));
      const dialog = screen.getByRole('dialog', { name: 'Add calendar entry' });
      await user.click(within(dialog).getByRole('button', { name: 'Save' }));
      const date = within(dialog).getByLabelText('Date');
      expect(date).toHaveAttribute('aria-invalid', 'true');
      expect(date).toHaveAccessibleDescription('Choose a date');
      await user.type(date, '2026-11-09');
      await user.selectOptions(within(dialog).getByLabelText('Type'), 'special');
      await user.type(within(dialog).getByLabelText('Start'), '19:00');
      await user.type(within(dialog).getByLabelText('End'), '18:00');
      await user.click(within(dialog).getByRole('button', { name: 'Save' }));
      expect(within(dialog).getByLabelText('End')).toHaveAccessibleDescription('Enter a start time before the end time');
      expect(date).not.toHaveAttribute('aria-invalid');
      backend.failNext('calendar.upsert', 'The date must be between 2000-01-01 and 2028-12-31.', 'invalid_entry');
      await user.clear(within(dialog).getByLabelText('Start'));
      await user.type(within(dialog).getByLabelText('Start'), '17:00');
      await user.click(within(dialog).getByRole('button', { name: 'Save' }));
      const alert = await within(dialog).findByText('The date must be between 2000-01-01 and 2028-12-31.');
      expect(alert).toHaveAttribute('role', 'alert');
      expect(within(dialog).getByLabelText('End')).not.toHaveAttribute('aria-invalid');
    });
    it('M2: the dialog lists every phase and edits the one the user chooses', async () => {
      const backend = twoPhaseSeed();
      const { user } = renderApp({ route: '/settings/calendar', backend });
      const dialog = await openEdit(user, /^08-11-2026/);
      const list = within(dialog).getByRole('list', { name: 'Phases of this day' });
      expect(within(list).getAllByRole('listitem').map((li) => li.textContent)).toEqual(['Pre-open 18:00–18:08', 'Continuous trading 18:15–19:15']);
      await user.selectOptions(within(dialog).getByLabelText('Phase to edit'), '1');
      expect(within(dialog).getByLabelText('Start')).toHaveValue('18:15');
      await user.clear(within(dialog).getByLabelText('End'));
      await user.type(within(dialog).getByLabelText('End'), '19:30');
      await user.click(within(dialog).getByRole('button', { name: 'Save' }));
      expect(backend.callsTo('calendar.upsert')[0].args[0]).toEqual({
        exchange: 'NSE', date: '2026-11-08', type: 'special',
        phases: [{ kind: 'pre-open', start: '18:00', end: '18:08' }, { kind: 'continuous', start: '18:15', end: '19:30' }],
      });
    });
    it('M2: a new phase can be added to an existing day', async () => {
      const backend = twoPhaseSeed();
      const { user } = renderApp({ route: '/settings/calendar', backend });
      const dialog = await openEdit(user, /^08-11-2026/);
      await user.selectOptions(within(dialog).getByLabelText('Phase to edit'), 'new');
      await user.selectOptions(within(dialog).getByLabelText('Session phase'), 'closing');
      await user.type(within(dialog).getByLabelText('Start'), '19:15');
      await user.type(within(dialog).getByLabelText('End'), '19:25');
      await user.click(within(dialog).getByRole('button', { name: 'Save' }));
      expect((backend.callsTo('calendar.upsert')[0].args[0] as { phases: unknown }).phases).toEqual([
        { kind: 'pre-open', start: '18:00', end: '18:08' }, { kind: 'continuous', start: '18:15', end: '19:15' },
        { kind: 'closing', start: '19:15', end: '19:25' },
      ]);
    });
    it('M5: the remove confirmation is busy while Go works and sends one request', async () => {
      const backend = createMockBackend(demoSeed());
      const real = backend.calendar.remove;
      let release = () => {};
      const gate = new Promise<void>((r) => { release = r; });
      let sent = 0;
      backend.calendar.remove = (exchange, date) => { sent++; return gate.then(() => real(exchange, date)); };
      const { user } = renderApp({ route: '/settings/calendar', backend });
      const row = await screen.findByRole('row', { name: /^20-10-2026/ });
      row.focus();
      await user.keyboard('{Shift>}{F10}{/Shift}');
      await user.click(screen.getByRole('menuitem', { name: 'Remove' }));
      const dialog = screen.getByRole('dialog', { name: 'Remove calendar entry?' });
      const confirm = within(dialog).getByRole('button', { name: 'Remove' });
      await user.click(confirm);
      await user.click(confirm);
      await user.keyboard('{Escape}');
      expect(confirm).toBeDisabled();
      expect(screen.getByRole('dialog', { name: 'Remove calendar entry?' })).toBeInTheDocument();
      release();
      await waitFor(() => expect(screen.queryByRole('row', { name: /^20-10-2026/ })).toBeNull());
      expect(sent).toBe(1);
    });
    it('M7: the table caption differs from the section heading', async () => {
      renderApp({ route: '/settings/calendar' });
      expect(await screen.findByRole('table', { name: 'Calendar entries' })).toBeInTheDocument();
    });
    it('M7: editing an inferred day explains it becomes a holiday you added', async () => {
      const seed = demoSeed();
      seed.calendar.entries = [{ exchange: 'NSE', date: '2026-10-27', type: 'inferred-non-trading', phases: [], source: 'inferred' }];
      const backend = createMockBackend(seed);
      const { user } = renderApp({ route: '/settings/calendar', backend });
      const dialog = await openEdit(user, /^27-10-2026/);
      expect(dialog).toHaveTextContent('This day was marked closed because no market data arrived for it. Saving it makes it a holiday added by you.');
      expect(within(dialog).getByLabelText('Type')).toHaveValue('holiday');
    });
  });
});
