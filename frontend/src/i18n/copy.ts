// All UI strings (ARCHITECTURE §8.1). Our own wording; never reuse third-party product text.

export const navCopy = {
  home: 'Home',
  scalper: 'Scalper',
  strategies: 'Strategies',
  builder: 'Equity Builder',
  live: 'Live Strategies',
  scanners: 'Scanners',
  newScanner: 'New Scanner',
  scannerAlerts: 'Scanner Alerts',
} as const;

export const shellCopy = {
  sidebarLabel: 'Main navigation',
  collapseSidebar: 'Collapse sidebar',
  expandSidebar: 'Expand sidebar',
  quickLinks: 'Quick links',
} as const;

export const homeCopy = {
  title: 'Home',
  rightColumn: 'Watchlists and widgets',
  chartArea: 'Main chart',
  watchlistArea: 'Watchlists',
} as const;

export const componentCopy = {
  close: 'Close',
  cancel: 'Cancel',
  typeToConfirm: (phrase: string) => `Type ${phrase} to confirm`,
} as const;

export const settingsCopy = {
  restoreInProgress: 'A restore is in progress — settings changes are paused',
} as const;

export const toastCopy = {
  region: 'Notifications',
  dismiss: 'Dismiss notification',
} as const;
