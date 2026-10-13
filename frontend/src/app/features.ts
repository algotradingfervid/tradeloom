import type { IconName } from '../components/Icon';
import { navCopy } from '../i18n/copy';

export type Phase = 0 | 1 | 2 | 3 | 4 | 5 | 6;

function toPhase(raw: string | undefined): Phase {
  const n = Number(raw ?? 0);
  return (Number.isInteger(n) && n >= 0 && n <= 6 ? n : 0) as Phase;
}

export const BUILD_PHASE: Phase = toPhase(import.meta.env.VITE_PHASE);

export type NavId = 'home' | 'scalper' | 'strategies' | 'builder' | 'live' | 'scanners' | 'newScanner' | 'scannerAlerts';

export interface NavEntry { id: NavId; label: string; path: string; icon: IconName; phase: Phase; shortcutId: string }

// FR-SHELL-002 order; phase per ROADMAP §7 GATE row for FR-SHELL-002.
export const NAV_ENTRIES: readonly NavEntry[] = [
  { id: 'home', label: navCopy.home, path: '/home', icon: 'home', phase: 0, shortcutId: 'nav.1' },
  { id: 'scalper', label: navCopy.scalper, path: '/scalper', icon: 'scalper', phase: 6, shortcutId: 'nav.2' },
  { id: 'strategies', label: navCopy.strategies, path: '/strategies', icon: 'strategies', phase: 2, shortcutId: 'nav.3' },
  { id: 'builder', label: navCopy.builder, path: '/builder', icon: 'builder', phase: 1, shortcutId: 'nav.4' },
  { id: 'live', label: navCopy.live, path: '/live', icon: 'live', phase: 4, shortcutId: 'nav.5' },
  { id: 'scanners', label: navCopy.scanners, path: '/scanners', icon: 'scanners', phase: 3, shortcutId: 'nav.6' },
  { id: 'newScanner', label: navCopy.newScanner, path: '/scanner', icon: 'newScanner', phase: 3, shortcutId: 'nav.7' },
  { id: 'scannerAlerts', label: navCopy.scannerAlerts, path: '/scanner-alerts', icon: 'alerts', phase: 3, shortcutId: 'nav.8' },
];

export function visibleNavEntries(phase: Phase = BUILD_PHASE): NavEntry[] {
  return NAV_ENTRIES.filter((e) => phase >= e.phase);
}

export type Feature = 'notificationsBell' | 'orderButtons' | 'liveStrategyCard' | 'profileToggles';

const FEATURE_PHASE: Record<Feature, Phase> = {
  notificationsBell: 3, // FR-SHELL-006 → FR-SHELL-003 GATE
  orderButtons: 4, // FR-CHART-012 / FR-ORD-012 GATE; trading disabled before P4
  liveStrategyCard: 4, // FR-HOME-003
  profileToggles: 3, // FR-ACCT-001 → FR-ACCT-003 (P3), FR-ACCT-002 (P4)
};

export function isEnabled(feature: Feature, phase: Phase = BUILD_PHASE): boolean {
  return phase >= FEATURE_PHASE[feature];
}
