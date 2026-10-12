import { describe, expect, it } from 'vitest';
import { parseBackendError } from '../errors';
import { demoSeed } from './fixtures';
import { createMockBackend } from './index';

const refusalOf = async (p: Promise<unknown>) => {
  try {
    await p;
  } catch (e) {
    return parseBackendError(e);
  }
  throw new Error('expected a refusal');
};

// Ruling 100 M4: the mock calendar behaves like Go's façade (internal/service/calendar.go, internal/calendar/csv.go).
describe('mock Calendar façade mirrors Go', () => {
  it('Info lists overrides only (Go never lists bundled days)', async () => {
    const info = await createMockBackend(demoSeed()).calendar.info();
    expect(info.entries.length).toBeGreaterThan(0);
    expect(info.entries.every((e) => e.source !== 'shipped')).toBe(true);
  });
  it('Remove refuses not_found for an entry that is not an override', async () => {
    const b = createMockBackend(demoSeed());
    expect(await refusalOf(b.calendar.remove('NSE', '2026-10-21'))).toMatchObject({ code: 'not_found', reason: 'No override for NSE on 2026-10-21.' });
  });
  it('Upsert validates like Go parseEntry (invalid_entry)', async () => {
    const b = createMockBackend(demoSeed());
    expect(await refusalOf(b.calendar.upsert({ exchange: 'NSE', date: '2026-11-09', type: 'special', phases: [] })))
      .toMatchObject({ code: 'invalid_entry', reason: 'A special session needs a continuous phase.' });
    expect(await refusalOf(b.calendar.upsert({ exchange: 'NSE', date: '2026-11-09', type: 'holiday', phases: [{ kind: 'continuous', start: '09:15', end: '15:30' }] })))
      .toMatchObject({ code: 'invalid_entry', reason: 'A holiday has no session phases.' });
    expect(await refusalOf(b.calendar.upsert({
      exchange: 'NSE', date: '2026-11-09', type: 'special',
      phases: [{ kind: 'continuous', start: '18:00', end: '19:00' }, { kind: 'continuous', start: '19:00', end: '19:30' }],
    }))).toMatchObject({ code: 'invalid_entry', reason: 'Each phase kind appears at most once.' });
    expect(await refusalOf(b.calendar.upsert({
      exchange: 'NSE', date: '2026-11-09', type: 'special',
      phases: [{ kind: 'continuous', start: '18:00', end: '19:00' }, { kind: 'closing', start: '18:30', end: '19:30' }],
    }))).toMatchObject({ code: 'invalid_entry', reason: 'Phases must not overlap.' });
  });
  it('Import is all-or-nothing and names the row (invalid_file)', async () => {
    const seed = demoSeed();
    seed.calendarCsv = 'date,exchange,type,phase,start,end\n2026-12-25,NSE,holiday,,,\n2026-12-26,NSE,special,continuous,18:00,1x:00\n';
    const b = createMockBackend(seed);
    const before = await b.calendar.info();
    expect(await refusalOf(b.calendar.import('/home/asha/cal.csv'))).toMatchObject({ code: 'invalid_file', reason: 'Row 2: invalid time' });
    expect(await b.calendar.info()).toEqual(before);
  });
  it('Import merges valid rows as "import" overrides', async () => {
    const seed = demoSeed();
    seed.calendarCsv = 'date,exchange,type,phase,start,end\n# comment\n2026-12-25,BSE,holiday,,,\n';
    const info = await createMockBackend(seed).calendar.import('/home/asha/cal.csv');
    expect(info.entries).toContainEqual({ exchange: 'BSE', date: '2026-12-25', type: 'holiday', phases: [], source: 'import' });
  });
  it('Import of a missing file refuses not_found', async () => {
    const seed = demoSeed();
    delete seed.calendarCsv;
    expect(await refusalOf(createMockBackend(seed).calendar.import('/nope.csv'))).toMatchObject({ code: 'not_found' });
  });
});
