> **P0-4 Frontend Shell, Account & Data UI Implementation Plan — Task 17.** Read [`../MANIFEST.md`](../MANIFEST.md) first: its Global Constraints,
> Review Focus and Consumes/Produces apply to this task. Other tasks live beside this file in `tasks/`.

<!-- task body (verbatim from the plan) -->
### Task 17: Settings → Market calendar (import, add/edit/remove, stale warning)

**Files:**
- Create: `frontend/src/views/settings/CalendarGroup.tsx`, `frontend/src/app/CalendarStaleCheck.tsx`
- Modify: `frontend/src/views/settings/SettingsView.tsx` (`GROUP_COMPONENTS.calendar`), `frontend/src/app/ShellBanners.tsx`, `frontend/src/i18n/copy.ts`
- Test: `frontend/src/views/settings/CalendarGroup.test.tsx`

**Interfaces:**
- Consumes: Task 2 `Backend.calendar.{info,chooseImportFile,import,upsert,remove}` (each mutation returns the new `CalendarInfo`), `CalendarInfo{version, coversUntil (YYYY-MM-DD), stale, entries}`, `CalendarEntryDTO{exchange, date, type, phases[{kind, start, end}], source}`, `CalendarEntryInput{exchange, date, type, phases}`, `PhaseKind`, `useSettingsStore`, `writeSetting`; Task 5 `DataTable` (row actions = keyboard-reachable Edit/Remove); Task 1 `istDayKey`, `formatDateLong`; P0-2 calendar import validation (all-or-nothing; the refusal reason names the row, e.g. `Row 7: invalid time`) via P0-3 `Calendar.Import`.
- Produces: `CalendarGroup()`; `CalendarStaleCheck()` (once-per-IST-day WARNING toast when `Calendar.Info().stale`, i.e. the calendar does not cover the next 30 days, FR-DATA-001 AC6; day stored in `calendar.stale_toast_day`); `isoToDisplay(iso: string): string` (`2026-10-20` → `20-10-2026`); `isoDayMs(iso: string): number` (an instant inside that IST day, for the IST date formatters).

- [ ] **Step 1: Write the failing test**

Create `frontend/src/views/settings/CalendarGroup.test.tsx`:

```tsx
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
    backend.failNext('calendar.import', 'Row 7: invalid time', 'invalid');
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
});
```

- [ ] **Step 2: Run it to verify it fails**

Run: `cd frontend && npx vitest run src/views/settings/CalendarGroup.test.tsx`
Expected: FAIL — `/settings/calendar` renders "Page not found".

- [ ] **Step 3: Implement the group and the stale check**

Append to `frontend/src/i18n/copy.ts`:

```ts
export const calendarCopy = {
  title: 'Trading days and special sessions',
  version: 'Calendar version',
  coversUntil: 'Covers until',
  stale: 'Calendar may be stale',
  staleHint: 'It does not cover the next 30 days. Import an updated list or add the missing days.',
  staleToast: (until: string) => `Calendar may be stale — it ends on ${until}. Import an updated list in Settings → Market calendar.`,
  open: 'Open',
  import: 'Import CSV…',
  imported: 'Calendar imported',
  importFailed: (reason: string) => `Calendar import refused: ${reason}`,
  add: 'Add entry',
  addTitle: 'Add calendar entry',
  editTitle: 'Edit calendar entry',
  edit: 'Edit',
  remove: 'Remove',
  removeTitle: 'Remove calendar entry?',
  removeMessage: (date: string) => `The entry for ${date} is removed. Weekdays without an entry count as trading days.`,
  save: 'Save',
  cols: { date: 'Date', exchange: 'Exchange', type: 'Type', session: 'Session', source: 'Source' },
  types: { holiday: 'Holiday', special: 'Special session', trading: 'Trading day' },
  // `CalendarEntryDTO.source`: bundled file, Settings dialog, or CSV import (P0-3 `calendar_overrides.source`).
  sources: { shipped: 'Bundled', user: 'Added by you', import: 'Imported' } as Record<string, string>,
  phases: { 'pre-open': 'Pre-open', continuous: 'Continuous trading', closing: 'Closing session', 'post-closing': 'Post-closing', closed: 'Closed' },
  fields: { date: 'Date', exchange: 'Exchange', type: 'Type', phase: 'Session phase', start: 'Start', end: 'End' },
  dateRequired: 'Choose a date',
  timesInvalid: 'Enter a start time before the end time',
} as const;
```

Create `frontend/src/views/settings/CalendarGroup.tsx`:

```tsx
import { useCallback, useEffect, useState } from 'react';
import type { CalendarDayType, CalendarEntryDTO, CalendarEntryInput, CalendarExchange, CalendarInfo, PhaseKind } from '../../api/backend';
import { useBackend } from '../../api/BackendContext';
import { parseBackendError } from '../../api/errors';
import { Button } from '../../components/Button';
import { ConfirmDialog } from '../../components/ConfirmDialog';
import { DataTable, type Column } from '../../components/DataTable';
import { Dialog } from '../../components/Dialog';
import { SelectField } from '../../components/SelectField';
import { TextField } from '../../components/TextField';
import { calendarCopy as copy, componentCopy } from '../../i18n/copy';
import { DASH, formatDateLong } from '../../i18n/format';
import { pushToast } from '../../store/toasts';
import { Fact, SettingSection } from './SettingRow';

export const isoToDisplay = (iso: string): string => iso.split('-').reverse().join('-');

/** `YYYY-MM-DD` (an IST calendar day) → 12:00 IST that day, so the IST date formatters print the same day. */
export function isoDayMs(iso: string): number {
  const [y, m, d] = iso.split('-').map(Number);
  return Date.UTC(y, m - 1, d, 6, 30);
}

const EXCHANGES: { value: CalendarExchange; label: string }[] = ['NSE', 'BSE', 'NFO', 'BFO'].map((x) => ({ value: x as CalendarExchange, label: x }));
const TYPES = (Object.keys(copy.types) as CalendarDayType[]).map((t) => ({ value: t, label: copy.types[t] }));
const PHASES = (Object.keys(copy.phases) as PhaseKind[]).filter((k) => k !== 'closed').map((k) => ({ value: k, label: copy.phases[k] }));

const sessionText = (e: CalendarEntryDTO): string =>
  e.phases.filter((p) => p.kind !== 'closed' && p.start && p.end).map((p) => `${p.start}–${p.end}`).join(', ') || DASH;

const columns: Column<CalendarEntryDTO>[] = [
  { id: 'date', header: copy.cols.date, width: '16%', cell: (e) => isoToDisplay(e.date) },
  { id: 'exchange', header: copy.cols.exchange, width: '12%', cell: (e) => e.exchange },
  { id: 'type', header: copy.cols.type, width: '20%', cell: (e) => copy.types[e.type] },
  { id: 'session', header: copy.cols.session, cell: (e) => sessionText(e), title: (e) => sessionText(e) },
  { id: 'source', header: copy.cols.source, width: '16%', cell: (e) => copy.sources[e.source] ?? e.source },
];

/** One editable phase; other phases of an existing entry are kept as they are. */
interface Draft { exchange: CalendarExchange; date: string; type: CalendarDayType; kind: PhaseKind; start: string; end: string }

function EntryDialog({ initial, onClose, onSaved }: { initial: CalendarEntryDTO | null; onClose: () => void; onSaved: (info: CalendarInfo) => void }) {
  const backend = useBackend();
  const first = initial?.phases.find((p) => p.kind !== 'closed');
  const [draft, setDraft] = useState<Draft>({
    exchange: initial?.exchange ?? 'NSE', date: initial?.date ?? '', type: initial?.type ?? 'holiday',
    kind: first?.kind ?? 'continuous', start: first?.start ?? '', end: first?.end ?? '',
  });
  const [error, setError] = useState<string | null>(null);
  const needsTimes = draft.type !== 'holiday';
  const set = (patch: Partial<Draft>) => setDraft((d) => ({ ...d, ...patch }));

  const save = async () => {
    if (!draft.date) return setError(copy.dateRequired);
    if (needsTimes && (!draft.start || !draft.end || draft.start >= draft.end)) return setError(copy.timesInvalid);
    const others = (initial?.phases ?? []).filter((p) => p.kind !== draft.kind);
    const input: CalendarEntryInput = {
      exchange: draft.exchange, date: draft.date, type: draft.type,
      phases: needsTimes ? [{ kind: draft.kind, start: draft.start, end: draft.end }, ...others] : [],
    };
    try {
      onSaved(await backend.calendar.upsert(input));
    } catch (e) {
      setError(parseBackendError(e).reason);
    }
  };

  return (
    <Dialog
      open
      title={initial ? copy.editTitle : copy.addTitle}
      onClose={onClose}
      footer={<><Button onClick={onClose}>{componentCopy.cancel}</Button><Button variant="primary" onClick={() => void save()}>{copy.save}</Button></>}
    >
      <TextField label={copy.fields.date} type="date" value={draft.date} readOnly={initial !== null} onChange={(e) => set({ date: e.target.value })} />
      <SelectField label={copy.fields.exchange} options={EXCHANGES} value={draft.exchange} disabled={initial !== null} onChange={(e) => set({ exchange: e.target.value as CalendarExchange })} />
      <SelectField label={copy.fields.type} options={TYPES} value={draft.type} onChange={(e) => set({ type: e.target.value as CalendarDayType })} />
      {needsTimes && (
        <>
          <SelectField label={copy.fields.phase} options={PHASES} value={draft.kind} onChange={(e) => set({ kind: e.target.value as PhaseKind })} />
          <TextField label={copy.fields.start} type="time" value={draft.start} onChange={(e) => set({ start: e.target.value })} />
          <TextField label={copy.fields.end} type="time" value={draft.end} onChange={(e) => set({ end: e.target.value })} />
        </>
      )}
      {error && <p className="field__error" role="alert">{error}</p>}
    </Dialog>
  );
}

export function CalendarGroup() {
  const backend = useBackend();
  const [info, setInfo] = useState<CalendarInfo | null>(null);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [importError, setImportError] = useState<string | null>(null);
  const [editing, setEditing] = useState<CalendarEntryDTO | 'new' | null>(null);
  const [removing, setRemoving] = useState<CalendarEntryDTO | null>(null);

  const load = useCallback(async () => {
    try {
      setInfo(await backend.calendar.info());
      setLoadError(null);
    } catch (e) {
      setLoadError(parseBackendError(e).reason);
    }
  }, [backend]);
  useEffect(() => {
    void load();
  }, [load]);

  const importCsv = async () => {
    setImportError(null);
    try {
      const path = await backend.calendar.chooseImportFile();
      if (!path) return;
      setInfo(await backend.calendar.import(path));
      pushToast({ level: 'success', message: copy.imported });
    } catch (e) {
      const reason = parseBackendError(e).reason;
      setImportError(reason);
      pushToast({ level: 'error', message: copy.importFailed(reason) });
    }
  };
  const remove = async (entry: CalendarEntryDTO) => {
    try {
      setInfo(await backend.calendar.remove(entry.exchange, entry.date));
    } catch (e) {
      pushToast({ level: 'error', message: parseBackendError(e).reason });
    } finally {
      setRemoving(null);
    }
  };

  const entries = [...(info?.entries ?? [])].sort((a, b) => a.date.localeCompare(b.date) || a.exchange.localeCompare(b.exchange));

  return (
    <SettingSection title={copy.title}>
      {loadError && <p className="field__error" role="alert">{loadError}</p>}
      {info?.stale && (
        <div className="banner banner--warn" role="status">
          <span><strong>{copy.stale}</strong> — {copy.staleHint}</span>
        </div>
      )}
      <dl className="facts">
        <Fact label={copy.version}>{info?.version ?? DASH}</Fact>
        <Fact label={copy.coversUntil}>{info?.coversUntil ? formatDateLong(isoDayMs(info.coversUntil)) : DASH}</Fact>
      </dl>
      <div className="settings__actions">
        <Button onClick={() => void importCsv()}>{copy.import}</Button>
        <Button onClick={() => setEditing('new')}>{copy.add}</Button>
      </div>
      {importError && <p className="field__error" role="alert">{importError}</p>}
      <DataTable
        caption={copy.title}
        columns={columns}
        rows={entries}
        rowKey={(e) => `${e.exchange}|${e.date}`}
        rowLabel={(e) => `${isoToDisplay(e.date)} ${e.exchange} ${copy.types[e.type]}`}
        rowActions={[
          { id: 'edit', label: copy.edit, icon: 'builder', onSelect: (e) => setEditing(e) },
          { id: 'remove', label: copy.remove, icon: 'trash', onSelect: (e) => setRemoving(e) },
        ]}
      />
      {editing && (
        <EntryDialog
          initial={editing === 'new' ? null : editing}
          onClose={() => setEditing(null)}
          onSaved={(next) => { setEditing(null); setInfo(next); }}
        />
      )}
      <ConfirmDialog
        open={removing !== null}
        title={copy.removeTitle}
        message={removing ? copy.removeMessage(isoToDisplay(removing.date)) : ''}
        confirmLabel={copy.remove}
        tone="danger"
        onConfirm={() => { if (removing) void remove(removing); }}
        onCancel={() => setRemoving(null)}
      />
    </SettingSection>
  );
}
```

Create `frontend/src/app/CalendarStaleCheck.tsx`:

```tsx
import { useEffect, useRef } from 'react';
import { useNavigate } from 'react-router-dom';
import { useBackend } from '../api/BackendContext';
import { calendarCopy } from '../i18n/copy';
import { formatDateLong, istDayKey } from '../i18n/format';
import { useSettingsStore, writeSetting } from '../store/settings';
import { pushToast } from '../store/toasts';
import { isoDayMs } from '../views/settings/CalendarGroup';

export const STALE_TOAST_KEY = 'calendar.stale_toast_day';

/** FR-DATA-001 AC6: one WARNING toast per IST day while `Calendar.Info().stale` (no cover for the next 30 days). */
export function CalendarStaleCheck() {
  const backend = useBackend();
  const navigate = useNavigate();
  const loaded = useSettingsStore((s) => s.loaded);
  const checked = useRef(false);
  useEffect(() => {
    if (!loaded || checked.current) return;
    checked.current = true;
    backend.calendar.info().then((info) => {
      if (!info.stale) return;
      const today = istDayKey(Date.now());
      if (useSettingsStore.getState().values[STALE_TOAST_KEY] === today) return;
      pushToast({
        level: 'warning',
        message: calendarCopy.staleToast(formatDateLong(isoDayMs(info.coversUntil))),
        action: { label: calendarCopy.open, onSelect: () => void navigate('/settings/calendar') },
      });
      void writeSetting(backend, STALE_TOAST_KEY, today);
    }, () => undefined);
  }, [loaded, backend, navigate]);
  return null;
}
```

In `frontend/src/app/ShellBanners.tsx` add `import { CalendarStaleCheck } from './CalendarStaleCheck';` and render `<CalendarStaleCheck />` inside the returned fragment.

In `frontend/src/views/settings/SettingsView.tsx` add `import { CalendarGroup } from './CalendarGroup';` and `calendar: CalendarGroup,` to `GROUP_COMPONENTS`.

- [ ] **Step 4: Run the suite to verify it passes**

Run: `cd frontend && npx vitest run && npm run typecheck`
Expected: PASS (42 test files), `tsc` exits 0. (`coversUntil` `'2026-10-31'` is an IST calendar day; `isoDayMs` turns it into 12:00 IST that day, hence "ends on 31 Oct 2026".)

- [ ] **Step 5: Commit**

```bash
git add frontend/src
git commit -m "feat(frontend): Settings → Market calendar with import, edits and stale warning (P0-4 Task 17)" \
  -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---
