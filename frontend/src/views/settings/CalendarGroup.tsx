import { useCallback, useEffect, useId, useState, type ReactNode } from 'react';
import type { CalendarDayType, CalendarEntryDTO, CalendarEntryInput, CalendarExchange, CalendarInfo, PhaseDTO, PhaseKind } from '../../api/backend';
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

// MERGE(P0-4 Task 14): replace these two with `SettingSection` and `Fact` from './SettingRow' (same markup and
// class names); Task 14's settings.css styles `.settings__section`, `.facts`, `.fact`, `.settings__actions` and
// `.settings__notice--warn`. They are local only because Task 14 is not on this branch.
function SettingSection({ title, children }: { title: string; children: ReactNode }) {
  const id = useId();
  return (
    <section className="settings__section" aria-labelledby={id}>
      <h2 id={id}>{title}</h2>
      {children}
    </section>
  );
}

function Fact({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div className="fact">
      <dt>{label}</dt>
      <dd>{children}</dd>
    </div>
  );
}

export const isoToDisplay = (iso: string): string => iso.split('-').reverse().join('-');

/** `YYYY-MM-DD` (an IST calendar day) → 12:00 IST that day, so the IST date formatters print the same day. */
export function isoDayMs(iso: string): number {
  const [y, m, d] = iso.split('-').map(Number);
  return Date.UTC(y, m - 1, d, 6, 30);
}

const EXCHANGES: { value: CalendarExchange; label: string }[] = ['NSE', 'BSE', 'NFO', 'BFO'].map((x) => ({ value: x as CalendarExchange, label: x }));
const TYPES = (Object.keys(copy.types) as CalendarDayType[]).map((t) => ({ value: t, label: copy.types[t] }));
const PHASES = (Object.keys(copy.phases) as PhaseKind[]).filter((k) => k !== 'closed').map((k) => ({ value: k, label: copy.phases[k] }));
const isDialogType = (t: string): t is CalendarDayType => t in copy.types;

// Go lists inferred non-trading days too (type `inferred-non-trading`), so labels fall back to the raw value.
const typeLabel = (t: string): string => copy.typeLabels[t] ?? t;

const sessionText = (e: CalendarEntryDTO): string =>
  e.phases.filter((p) => p.kind !== 'closed' && p.start && p.end).map((p) => `${p.start}–${p.end}`).join(', ') || DASH;

const columns: Column<CalendarEntryDTO>[] = [
  { id: 'date', header: copy.cols.date, width: '16%', cell: (e) => isoToDisplay(e.date) },
  { id: 'exchange', header: copy.cols.exchange, width: '12%', cell: (e) => e.exchange },
  { id: 'type', header: copy.cols.type, width: '20%', cell: (e) => typeLabel(e.type) },
  { id: 'session', header: copy.cols.session, cell: (e) => sessionText(e), title: (e) => sessionText(e) },
  { id: 'source', header: copy.cols.source, width: '16%', cell: (e) => copy.sources[e.source] ?? e.source },
];

/** One editable phase; other phases of an existing entry are kept as they are. */
interface Draft { exchange: CalendarExchange; date: string; type: CalendarDayType; kind: PhaseKind; start: string; end: string }

function EntryDialog({ initial, onClose, onSaved }: { initial: CalendarEntryDTO | null; onClose: () => void; onSaved: (info: CalendarInfo) => void }) {
  const backend = useBackend();
  const first = initial?.phases.find((p) => p.kind !== 'closed');
  const [draft, setDraft] = useState<Draft>({
    exchange: initial?.exchange ?? 'NSE', date: initial?.date ?? '',
    type: initial && isDialogType(initial.type) ? initial.type : 'holiday',
    kind: first?.kind ?? 'continuous', start: first?.start ?? '', end: first?.end ?? '',
  });
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const showTimes = draft.type !== 'holiday';
  // A trading day without times keeps the regular session (Go: no phases = regular phases).
  const needsTimes = draft.type === 'special' || (draft.type === 'trading' && (draft.start !== '' || draft.end !== ''));
  const set = (patch: Partial<Draft>) => setDraft((d) => ({ ...d, ...patch }));

  const save = async () => {
    if (busy) return;
    if (!draft.date) return setError(copy.dateRequired);
    if (needsTimes && (!draft.start || !draft.end || draft.start >= draft.end)) return setError(copy.timesInvalid);
    // The edited phase replaces the one it was opened with; the entry's other phases are kept.
    const others = (initial?.phases ?? []).filter((p) => p !== first && p.kind !== draft.kind && p.kind !== 'closed');
    const phases: PhaseDTO[] = needsTimes ? [{ kind: draft.kind, start: draft.start, end: draft.end }, ...others] : [];
    // Go refuses trading/special days whose phases lack a continuous phase (P0-3 Ruling 77).
    if (phases.length > 0 && !phases.some((p) => p.kind === 'continuous')) return setError(copy.continuousRequired);
    const input: CalendarEntryInput = { exchange: draft.exchange, date: draft.date, type: draft.type, phases };
    setBusy(true);
    try {
      onSaved(await backend.calendar.upsert(input));
    } catch (e) {
      setError(parseBackendError(e).reason);
      setBusy(false);
    }
  };

  return (
    <Dialog
      open
      title={initial ? copy.editTitle : copy.addTitle}
      onClose={onClose}
      dismissible={!busy}
      footer={<><Button onClick={onClose} disabled={busy}>{componentCopy.cancel}</Button><Button variant="primary" onClick={() => void save()} disabled={busy}>{copy.save}</Button></>}
    >
      <TextField label={copy.fields.date} type="date" value={draft.date} readOnly={initial !== null} onChange={(e) => set({ date: e.target.value })} />
      <SelectField label={copy.fields.exchange} options={EXCHANGES} value={draft.exchange} disabled={initial !== null} onChange={(e) => set({ exchange: e.target.value as CalendarExchange })} />
      <SelectField label={copy.fields.type} options={TYPES} value={draft.type} onChange={(e) => set({ type: e.target.value as CalendarDayType })} />
      {showTimes && (
        <>
          <SelectField label={copy.fields.phase} options={PHASES} value={draft.kind} onChange={(e) => set({ kind: e.target.value as PhaseKind })} />
          <TextField label={copy.fields.start} type="time" value={draft.start} hint={draft.type === 'trading' ? copy.tradingTimesHint : undefined} onChange={(e) => set({ start: e.target.value })} />
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
  const [importing, setImporting] = useState(false);
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

  // The chosen path goes only to Go; it is never shown (Go's refusal reasons carry no path either).
  const importCsv = async () => {
    if (importing) return;
    setImportError(null);
    setImporting(true);
    try {
      const path = await backend.calendar.chooseImportFile();
      if (!path) return;
      setInfo(await backend.calendar.import(path));
      pushToast({ level: 'success', message: copy.imported });
    } catch (e) {
      // All-or-nothing in Go: a refused file changed nothing, so the table stays as it is.
      const reason = parseBackendError(e).reason;
      setImportError(reason);
      pushToast({ level: 'error', message: copy.importFailed(reason) });
    } finally {
      setImporting(false);
    }
  };
  const remove = async (entry: CalendarEntryDTO) => {
    try {
      setInfo(await backend.calendar.remove(entry.exchange, entry.date));
    } catch (e) {
      pushToast({ level: 'error', message: parseBackendError(e).reason });
      // A rejected binding drops Go's CalendarInfo; re-read so the table matches Go (e.g. not_found).
      void load();
    } finally {
      setRemoving(null);
    }
  };

  const entries = [...(info?.entries ?? [])].sort((a, b) => a.date.localeCompare(b.date) || a.exchange.localeCompare(b.exchange));

  return (
    <SettingSection title={copy.title}>
      {loadError && <p className="field__error" role="alert">{loadError}</p>}
      {info?.stale && (
        <div className="settings__notice settings__notice--warn" role="status">
          <span><strong>{copy.stale}</strong> — {copy.staleHint}</span>
        </div>
      )}
      <dl className="facts">
        <Fact label={copy.version}>{info?.version || DASH}</Fact>
        <Fact label={copy.coversUntil}>{info?.coversUntil ? formatDateLong(isoDayMs(info.coversUntil)) : DASH}</Fact>
      </dl>
      <div className="settings__actions">
        <Button onClick={() => void importCsv()} disabled={importing}>{copy.import}</Button>
        <Button onClick={() => setEditing('new')}>{copy.add}</Button>
      </div>
      <p className="muted">{copy.importHint}</p>
      {importError && <p className="field__error" role="alert">{importError}</p>}
      {info && entries.length === 0 ? (
        <p className="muted">{copy.empty}</p>
      ) : (
        <DataTable
          caption={copy.title}
          columns={columns}
          rows={entries}
          rowKey={(e) => `${e.exchange}|${e.date}`}
          rowLabel={(e) => `${isoToDisplay(e.date)} ${e.exchange} ${typeLabel(e.type)}`}
          rowActions={[
            { id: 'edit', label: copy.edit, icon: 'builder', onSelect: (e) => setEditing(e) },
            { id: 'remove', label: copy.remove, icon: 'trash', onSelect: (e) => setRemoving(e) },
          ]}
        />
      )}
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
