import type { CalendarEntryDTO, CalendarEntryInput, CalendarExchange, CalendarInfo, PhaseDTO, PhaseKind } from '../backend';

/**
 * The mock Calendar façade's Go-like checks (Ruling 100 M4). They mirror, in a reduced form, Go's
 * `service.parseEntry` (invalid_entry, same reason strings) and `calendar.ParseFile` (invalid_file, "Row N: msg",
 * data rows counted from 1 without header or # comments). Not mirrored: the date window, file size/row limits and
 * the "# covers:" range.
 */

const EXCHANGES: readonly string[] = ['NSE', 'BSE', 'NFO', 'BFO'];
const KINDS: readonly string[] = ['pre-open', 'continuous', 'closing', 'post-closing'];
const HEADER = 'date,exchange,type,phase,start,end';

export class CalendarRefusal extends Error {
  constructor(readonly code: string, readonly reason: string) {
    super(reason);
  }
}

const validDate = (s: string): boolean => {
  if (!/^\d{4}-\d{2}-\d{2}$/.test(s)) return false;
  const [y, m, d] = s.split('-').map(Number);
  const t = new Date(Date.UTC(y, m - 1, d));
  return t.getUTCFullYear() === y && t.getUTCMonth() === m - 1 && t.getUTCDate() === d;
};

/** Minutes since midnight, or null (Go `ParseHM`: exactly HH:MM). */
const hm = (s: string): number | null => {
  if (!/^\d{2}:\d{2}$/.test(s)) return null;
  const h = Number(s.slice(0, 2));
  const m = Number(s.slice(3));
  return h > 23 || m > 59 ? null : h * 60 + m;
};

const sortPhases = (ps: PhaseDTO[]): PhaseDTO[] => [...ps].sort((a, b) => a.start.localeCompare(b.start));

/** Go `parseEntry` without the date window: returns the stored entry (source `user`) or throws invalid_entry. */
export function checkEntry(input: CalendarEntryInput): CalendarEntryDTO {
  const bad = (reason: string): never => {
    throw new CalendarRefusal('invalid_entry', reason);
  };
  if (!EXCHANGES.includes(input.exchange)) bad('Exchange must be NSE, BSE, NFO or BFO.');
  if (!validDate(input.date)) bad('Dates are YYYY-MM-DD.');
  const ty = input.type as string;
  if (ty !== 'holiday' && ty !== 'special' && ty !== 'trading') bad('Type must be holiday, special or trading.');
  if (ty === 'holiday' && input.phases.length > 0) bad('A holiday has no session phases.');
  if (input.phases.length > 4) bad('A day has at most four phases.');
  const seen = new Set<string>();
  const spans: [number, number][] = [];
  for (const p of input.phases) {
    if (p.kind === ('' as PhaseKind)) bad('Every phase needs a kind.');
    if (!KINDS.includes(p.kind)) bad('Phase kind must be pre-open, continuous, closing or post-closing.');
    if (seen.has(p.kind)) bad('Each phase kind appears at most once.');
    seen.add(p.kind);
    const start = hm(p.start);
    const end = hm(p.end);
    if (start === null || end === null || end <= start) return bad('Phase times are HH:MM and end after start.');
    if (spans.some(([s, e]) => start < e && s < end)) bad('Phases must not overlap.');
    spans.push([start, end]);
  }
  if (ty === 'special' && !seen.has('continuous')) bad('A special session needs a continuous phase.');
  if (ty === 'trading' && input.phases.length > 0 && !seen.has('continuous')) bad('A trading day with phases needs a continuous phase.');
  return { exchange: input.exchange, date: input.date, type: input.type, phases: sortPhases(input.phases), source: 'user' };
}

/** Go `calendar.ParseFile` basics: all rows or a refusal naming the first bad row. */
export function parseCalendarCsv(text: string): CalendarEntryDTO[] {
  const bad = (reason: string): never => {
    throw new CalendarRefusal('invalid_file', reason);
  };
  const lines = text.split('\n').map((l) => l.trim()).filter((l) => l !== '' && !l.startsWith('#'));
  if (lines.length === 0) bad(`missing header "${HEADER}"`);
  if (lines[0].split(',').map((f) => f.trim()).join(',') !== HEADER) bad(`header must be "${HEADER}"`);
  const byKey = new Map<string, { entry: CalendarEntryDTO; row: number; spans: [number, number][] }>();
  lines.slice(1).forEach((line, i) => {
    const row = i + 1;
    const rowBad = (msg: string): never => bad(`Row ${row}: ${msg}`);
    const rec = line.split(',').map((f) => f.trim());
    if (rec.length !== 6) rowBad('expected 6 fields');
    const [date, exchange, type, phase, start, end] = rec;
    if (!validDate(date)) rowBad('invalid date');
    if (!EXCHANGES.includes(exchange)) rowBad('invalid exchange');
    const key = `${exchange}|${date}`;
    const prev = byKey.get(key);
    if (type === 'holiday') {
      if (phase || start || end) rowBad('holiday rows take no phase or times');
      if (prev) rowBad(`conflicts with row ${prev.row}`);
      byKey.set(key, { entry: { exchange: exchange as CalendarExchange, date, type: 'holiday', phases: [], source: 'import' }, row, spans: [] });
      return;
    }
    if (type !== 'special' && type !== 'trading') return rowBad('invalid type');
    if (prev && prev.entry.type !== type) rowBad(`conflicts with row ${prev.row}`);
    const cur = prev ?? { entry: { exchange: exchange as CalendarExchange, date, type, phases: [], source: 'import' }, row, spans: [] };
    byKey.set(key, cur);
    if (type === 'trading' && !phase && !start && !end) return;
    if (!KINDS.includes(phase)) rowBad('invalid phase');
    const s = hm(start);
    const e = hm(end);
    if (s === null || e === null) return rowBad('invalid time');
    if (s >= e) rowBad('start must be before end');
    if (cur.spans.some(([a, b]) => s < b && a < e)) rowBad('overlaps another phase');
    cur.spans.push([s, e]);
    cur.entry.phases.push({ kind: phase as PhaseKind, start, end });
  });
  const out: CalendarEntryDTO[] = [];
  for (const { entry, row } of byKey.values()) {
    entry.phases = sortPhases(entry.phases);
    const hasContinuous = entry.phases.some((p) => p.kind === 'continuous');
    if (entry.type === 'special' && !hasContinuous) bad(`Row ${row}: special session needs a continuous phase`);
    if (entry.type === 'trading' && entry.phases.length > 0 && !hasContinuous) bad(`Row ${row}: trading day with phases needs a continuous phase`);
    out.push(entry);
  }
  return out;
}

/** Go `MergeOverrides`: an entry replaces the override with the same exchange and date. Sorted by date, then exchange. */
export function mergeOverrides(info: CalendarInfo, add: CalendarEntryDTO[]): void {
  const keys = new Set(add.map((e) => `${e.exchange}|${e.date}`));
  info.entries = [...info.entries.filter((e) => !keys.has(`${e.exchange}|${e.date}`)), ...add]
    .sort((a, b) => a.date.localeCompare(b.date) || a.exchange.localeCompare(b.exchange));
}
