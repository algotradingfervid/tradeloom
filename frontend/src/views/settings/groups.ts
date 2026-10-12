import type { Phase } from '../../app/features';
import { settingsViewCopy as copy } from '../../i18n/copy';

export type SettingsGroupId =
  | 'kite' | 'trading' | 'appearance' | 'chart' | 'home' | 'universes' | 'calendar' | 'data'
  | 'notifications' | 'shortcuts' | 'limits' | 'updates' | 'diagnostics' | 'tours' | 'about';

export interface SettingsGroupDef { id: SettingsGroupId; label: string; phase: Phase }

// FR-ACCT-012 order; phases per ROADMAP §7 GATE row for FR-ACCT-012.
export const SETTINGS_GROUPS: readonly SettingsGroupDef[] = [
  { id: 'kite', label: copy.groups.kite, phase: 0 },
  { id: 'trading', label: copy.groups.trading, phase: 4 },
  { id: 'appearance', label: copy.groups.appearance, phase: 0 },
  { id: 'chart', label: copy.groups.chart, phase: 0 },
  { id: 'home', label: copy.groups.home, phase: 2 },
  { id: 'universes', label: copy.groups.universes, phase: 1 },
  { id: 'calendar', label: copy.groups.calendar, phase: 0 },
  { id: 'data', label: copy.groups.data, phase: 0 },
  { id: 'notifications', label: copy.groups.notifications, phase: 3 },
  { id: 'shortcuts', label: copy.groups.shortcuts, phase: 0 },
  { id: 'limits', label: copy.groups.limits, phase: 4 },
  { id: 'updates', label: copy.groups.updates, phase: 6 },
  { id: 'diagnostics', label: copy.groups.diagnostics, phase: 0 },
  { id: 'tours', label: copy.groups.tours, phase: 6 },
  { id: 'about', label: copy.groups.about, phase: 0 },
];
