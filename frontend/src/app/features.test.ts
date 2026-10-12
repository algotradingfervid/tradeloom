import { describe, expect, it } from 'vitest';
import { BUILD_PHASE, NAV_ENTRIES, isEnabled, visibleNavEntries } from './features';

describe('phase-gated features (ROADMAP §0)', () => {
  it('defaults to phase 0', () => {
    expect(BUILD_PHASE).toBe(0);
  });
  it('P0 sidebar shows only Home', () => {
    expect(visibleNavEntries(0).map((e) => e.label)).toEqual(['Home']);
  });
  it('FR-SHELL-002 AC1 (run in P6): eight entries in order', () => {
    expect(visibleNavEntries(6).map((e) => e.label)).toEqual([
      'Home', 'Scalper', 'Strategies', 'Equity Builder', 'Live Strategies', 'Scanners', 'New Scanner', 'Scanner Alerts',
    ]);
  });
  it('binds Alt+1…Alt+8 to the entries in order', () => {
    expect(NAV_ENTRIES.map((e) => e.shortcutId)).toEqual(['nav.1', 'nav.2', 'nav.3', 'nav.4', 'nav.5', 'nav.6', 'nav.7', 'nav.8']);
  });
  it('no order buttons and no bell in P0 builds', () => {
    expect(isEnabled('orderButtons', 0)).toBe(false);
    expect(isEnabled('notificationsBell', 0)).toBe(false);
    expect(isEnabled('orderButtons', 4)).toBe(true);
    expect(isEnabled('notificationsBell', 3)).toBe(true);
  });
});
