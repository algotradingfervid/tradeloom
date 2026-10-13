import { describe, expect, it } from 'vitest';
import { parseBackendError } from '../errors';
import { demoSeed } from './fixtures';
import { createMockBackend } from './index';

// Expected codes and reasons were produced by the Go code itself (internal/settings SetJSON/ValidateBinding/Conflicts,
// wrapped as internal/service SetSetting/Rebind do), so the mock refusals stay byte-identical to Go (Ruling 87).

const refusalOf = (p: Promise<unknown>) => p.then(() => null, (e: unknown) => parseBackendError(e));

describe('mock settings.set mirrors Go service.SetSetting', () => {
  it.each([
    ['nope', '1', 'unknown_setting', 'Unknown setting nope.'],
    ['calendar.covers_until', '""', 'unknown_setting', 'Unknown setting calendar.covers_until.'],
    ['log.level', '{', 'invalid_setting', 'The value is not valid JSON.'],
    ['log.level', '"trace"', 'invalid_setting', 'settings: log.level: "trace" is not one of [info debug]'],
    ['log.level', 'null', 'invalid_setting', 'settings: log.level: "" is not one of [info debug]'],
    ['log.level', '5', 'invalid_setting', 'settings: log.level: json: cannot unmarshal number into Go value of type string'],
    ['backup.schedule.keep', '0', 'invalid_setting', 'settings: backup.schedule.keep: 0 is outside 1…365'],
    ['backup.schedule.keep', '1.5', 'invalid_setting', 'settings: backup.schedule.keep: json: cannot unmarshal number 1.5 into Go value of type int'],
    ['backup.schedule.keep', '"7"', 'invalid_setting', 'settings: backup.schedule.keep: json: cannot unmarshal string into Go value of type int'],
    ['backup.schedule.enabled', '[]', 'invalid_setting', 'settings: backup.schedule.enabled: json: cannot unmarshal array into Go value of type bool'],
    ['backup.schedule.folder', '"rel/dir"', 'invalid_setting', 'settings: backup.schedule.folder: must be an absolute path'],
    ['appearance.theme', '"a\\"b\\u0001é\\u00a0"', 'invalid_setting', 'settings: appearance.theme: "a\\"b\\x01é\\u00a0" is not one of [light dark]'],
    ['chart.default_timeframe', '"2minute"', 'invalid_setting', 'settings: chart.default_timeframe: "2minute" is not one of [minute 3minute 5minute 10minute 15minute 30minute 60minute day]'],
    ['ui.last_route', '"//x"', 'invalid_setting', 'settings: ui.last_route: must not start with //'],
    ['ui.last_route', '"/a/../b"', 'invalid_setting', 'settings: ui.last_route: must not contain .. segments'],
    ['calendar.stale_toast_day', '"2026-02-30"', 'invalid_setting', 'settings: calendar.stale_toast_day: must be a date formatted YYYY-MM-DD'],
  ])('%s = %s → %s', async (name, value, code, reason) => {
    const m = createMockBackend(demoSeed());
    expect(await refusalOf(m.settings.set(name, value))).toEqual({ code, stage: 'settings', reason });
    expect(m.state.settings[name]).toBeUndefined();
  });
  it.each([
    ['backup.schedule.keep', ' 30 ', '30'],
    ['backup.schedule.enabled', 'null', 'false'],
    ['backup.schedule.folder', '"/a<b>&c"', '"/a\\u003cb\\u003e\\u0026c"'],
    ['calendar.stale_toast_day', '"2024-02-29"', '"2024-02-29"'],
  ])('stores %s = %s as Go json.Marshal does (%s)', async (name, value, stored) => {
    const m = createMockBackend(demoSeed());
    expect(await m.settings.set(name, value)).toEqual({ name, valueJson: stored });
    expect(m.state.settings[name]).toBe(stored);
  });
});

describe('mock settings.rebind mirrors Go service.Rebind', () => {
  it.each([
    ['nope.x', 'Ctrl+1', 'unknown_action', 'Unknown shortcut action.'],
    ['kill.switch', 'x'.repeat(65), 'invalid', 'Shortcut action or binding is too long.'],
    ['kill.switch', '', 'invalid', 'binding is empty'],
    ['kill.switch', 'Ctrl++', 'invalid', 'binding "Ctrl++" has an empty part'],
    ['kill.switch', 'Ctrl+Shift', 'invalid', 'binding "Ctrl+Shift": "Shift" is a modifier, not a key'],
    ['kill.switch', 'K+Ctrl', 'invalid', 'binding "K+Ctrl": "K" must be the last part'],
    ['kill.switch', 'Ctrl+NumpadAdd', 'invalid', 'binding "Ctrl+NumpadAdd": "NumpadAdd" is not a supported key'],
    ['kill.switch', 'Ctrl+F01', 'invalid', 'binding "Ctrl+F01": "F01" is not a supported key'],
    ['kill.switch', 'Ctrl+c', 'invalid', 'Ctrl+C is reserved and cannot be used'],
    ['kill.switch', 'Shift+K', 'invalid', 'Shift+K needs Ctrl, Alt or Meta in the global scope'],
    ['scalper.buy', 'B', 'invalid', 'B needs Ctrl, Alt or Meta in the view scope'],
    ['kill.switch', 'Ctrl+K', 'conflict', 'Open search'],
    ['scalper.buy', 'Alt+ArrowUp', 'conflict', 'Move item up'],
  ])('%s ← %s → %s', async (id, binding, code, reason) => {
    const m = createMockBackend(demoSeed());
    expect(await refusalOf(m.settings.rebind(id, binding))).toEqual({ code, stage: 'shortcuts', reason });
  });
  it.each([
    ['kill.switch', 'shift+ctrl+x', 'Ctrl+Shift+X'],
    ['kill.switch', 'meta+space', 'Meta+Space'],
    ['kill.switch', 'F13', 'F13'],
    ['chart.trendLine', 'T', 'T'],
    ['chart.trendLine', 'Alt+T', 'Alt+T'], // shipped overlap with toasts.focus is not a conflict
  ])('%s ← %s stores the canonical %s', async (id, binding, canon) => {
    const m = createMockBackend(demoSeed());
    expect(await m.settings.rebind(id, binding)).toMatchObject({ id, binding: canon });
    expect(m.state.shortcuts.find((a) => a.id === id)?.binding).toBe(canon);
  });
});
