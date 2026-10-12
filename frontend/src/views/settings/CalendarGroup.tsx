import { useCallback, useEffect, useId, useState, type ReactNode } from 'react';
import type {
  CalendarDayType, CalendarEntryDTO, CalendarEntryInput, CalendarExchange, CalendarInfo, CalendarListedDayType, PhaseDTO, PhaseKind,
} from '../../api/backend';
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
// One label table (copy.dayTypes); the dialog offers the types a user can save.
const DIALOG_TYPES: readonly CalendarDayType[] = ['holiday', 'special', 'trading'];
const TYPES = DIALOG_TYPES.map((t) => ({ value: t, label: copy.dayTypes[t] }));
const PHASES = (Object.keys(copy.phases) as PhaseKind[]).filter((k) => k !== 'closed').map((k) => ({ value: k, label: copy.phases[k] }));
const isDialogType = (t: CalendarListedDayType): t is CalendarDayType => (DIALOG_TYPES as readonly string[]).includes(t);
const typeLabel = (t: CalendarListedDayType): string => copy.dayTypes[t];
const phaseText = (p: PhaseDTO): string => `${copy.phases[p.kind]} ${p.start}–${p.end}`;

const sessionText = (e: CalendarEntryDTO): string =>
  e.phases.filter((p) => p.kind !== 'closed' && p.start && p.end).map((p) => `${p.start}–${p.end}`).join(', ') || DASH;

const columns: Column<CalendarEntryDTO>[] = [
  { id: 'date', header: copy.cols.date, width: '16%', cell: (e) => isoToDisplay(e.date) },
  { id: 'exchange', header: copy.cols.exchange, width: '12%', cell: (e) => e.exchange },
  { id: 'type', header: copy.cols.type, width: '20%', cell: (e) => typeLabel(e.type) },
  { id: 'session', header: copy.cols.session, cell: (e) => sessionText(e), title: (e) => sessionText(e) },
  { id: 'source', header: copy.cols.source, width: '16%', cell: (e) => copy.sources[e.source] ?? e.source },
];

/** Which phase the dialog edits: an index into the day's phases, or a phase to add. */
type Target = number | 'new';
interface PhaseDraft { kind: PhaseKind; start: string; end: string }
interface FieldErrors { date?: string; kind?: string; times?: string }

const draftFor = (phases: PhaseDTO[], target: Target): PhaseDraft => {
  if (target !== 'new') return { ...phases[target] };
  const used = new Set(phases.map((p) => p.kind));
  const kind = used.has('continuous') ? (PHASES.find((o) => !used.has(o.value))?.value ?? 'continuous') : 'continuous';
  return { kind, start: '', end: '' };
};

function EntryDialog({ initial, onClose, onSaved }: { initial: CalendarEntryDTO | null; onClose: () => void; onSaved: (info: CalendarInfo) => void }) {
  const backend = useBackend();
  const [phases] = useState<PhaseDTO[]>(() => (initial?.phases ?? []).filter((p) => p.kind !== 'closed'));
  const [exchange, setExchange] = useState<CalendarExchange>(initial?.exchange ?? 'NSE');
  const [date, setDate] = useState(initial?.date ?? '');
  // An inferred non-trading day is saved as a user holiday (copy.inferredHint).
  const [type, setType] = useState<CalendarDayType>(initial && isDialogType(initial.type) ? initial.type : 'holiday');
  const [target, setTarget] = useState<Target>(phases.length > 0 ? 0 : 'new');
  const [draft, setDraft] = useState<PhaseDraft>(() => draftFor(phases, phases.length > 0 ? 0 : 'new'));
  const [errors, setErrors] = useState<FieldErrors>({});
  const [refusal, setRefusal] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const showPhases = type !== 'holiday';
  const set = (patch: Partial<PhaseDraft>) => setDraft((d) => ({ ...d, ...patch }));
  const chooseTarget = (t: Target) => {
    setTarget(t);
    setDraft(draftFor(phases, t));
    setErrors({});
  };

  /** The phases to send, or field errors. Other phases are never dropped or merged (Ruling 100 I1). */
  const build = (): { phases: PhaseDTO[] } | { errors: FieldErrors } => {
    const errs: FieldErrors = {};
    if (!date) errs.date = copy.dateRequired;
    if (type === 'holiday') return Object.keys(errs).length ? { errors: errs } : { phases: [] };
    const rest = target === 'new' ? phases : phases.filter((_, i) => i !== target);
    // A trading day without times keeps the regular session (Go: no phases = regular phases).
    const include = target !== 'new' || draft.start !== '' || draft.end !== '' || (type === 'special' && rest.length === 0);
    let out = rest;
    if (include) {
      if (!draft.start || !draft.end || draft.start >= draft.end) errs.times = copy.timesInvalid;
      if (rest.some((p) => p.kind === draft.kind)) errs.kind = copy.duplicateKind;
      out = target === 'new' ? [...rest, draft] : phases.map((p, i) => (i === target ? draft : p));
    }
    // Go refuses trading/special days whose phases lack a continuous phase (P0-3 Ruling 77).
    if (!errs.kind && out.length > 0 && !out.some((p) => p.kind === 'continuous')) errs.kind = copy.continuousRequired;
    return Object.keys(errs).length ? { errors: errs } : { phases: out.map((p) => ({ kind: p.kind, start: p.start, end: p.end })) };
  };

  const save = async () => {
    if (busy) return;
    setRefusal(null);
    const built = build();
    if ('errors' in built) return setErrors(built.errors);
    setErrors({});
    const input: CalendarEntryInput = { exchange, date, type, phases: built.phases };
    setBusy(true);
    try {
      onSaved(await backend.calendar.upsert(input));
    } catch (e) {
      setRefusal(parseBackendError(e).reason); // Go's reason, verbatim
      setBusy(false);
    }
  };

  const targets = [...phases.map((p, i) => ({ value: String(i), label: phaseText(p) })), { value: 'new', label: copy.newPhase }];
  return (
    <Dialog
      open
      title={initial ? copy.editTitle : copy.addTitle}
      onClose={onClose}
      dismissible={!busy}
      footer={<><Button onClick={onClose} disabled={busy}>{componentCopy.cancel}</Button><Button variant="primary" onClick={() => void save()} disabled={busy}>{copy.save}</Button></>}
    >
      {initial?.type === 'inferred-non-trading' && <p className="muted">{copy.inferredHint}</p>}
      <TextField label={copy.fields.date} type="date" value={date} readOnly={initial !== null} error={errors.date} onChange={(e) => setDate(e.target.value)} />
      <SelectField label={copy.fields.exchange} options={EXCHANGES} value={exchange} disabled={initial !== null} onChange={(e) => setExchange(e.target.value as CalendarExchange)} />
      <SelectField label={copy.fields.type} options={TYPES} value={type} onChange={(e) => { setType(e.target.value as CalendarDayType); setErrors({}); }} />
      {showPhases && (
        <>
          {phases.length > 0 && (
            <>
              <ul className="calendar__phases" aria-label={copy.phaseList}>
                {phases.map((p) => <li key={p.kind}>{phaseText(p)}</li>)}
              </ul>
              <SelectField label={copy.fields.target} options={targets} value={String(target)} onChange={(e) => chooseTarget(e.target.value === 'new' ? 'new' : Number(e.target.value))} />
            </>
          )}
          <SelectField label={copy.fields.phase} options={PHASES} value={draft.kind} error={errors.kind} onChange={(e) => set({ kind: e.target.value as PhaseKind })} />
          <TextField label={copy.fields.start} type="time" value={draft.start} hint={type === 'trading' ? copy.tradingTimesHint : undefined} onChange={(e) => set({ start: e.target.value })} />
          <TextField label={copy.fields.end} type="time" value={draft.end} error={errors.times} onChange={(e) => set({ end: e.target.value })} />
        </>
      )}
      {refusal && <p className="field__error" role="alert">{refusal}</p>}
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
  const [removeBusy, setRemoveBusy] = useState(false);

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
    if (removeBusy) return;
    setRemoveBusy(true);
    try {
      setInfo(await backend.calendar.remove(entry.exchange, entry.date));
    } catch (e) {
      pushToast({ level: 'error', message: parseBackendError(e).reason });
      // A rejected binding drops Go's CalendarInfo; re-read so the table matches Go (e.g. not_found).
      void load();
    } finally {
      setRemoveBusy(false);
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
          caption={copy.tableCaption}
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
        busy={removeBusy}
        onConfirm={() => { if (removing) void remove(removing); }}
        onCancel={() => setRemoving(null)}
      />
    </SettingSection>
  );
}
