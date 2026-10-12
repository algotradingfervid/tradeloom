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

export const bootCopy = {
  failed: 'Some app data could not be loaded',
  retry: 'Retry',
} as const;

export const listCopy = {
  back: 'Back',
  refresh: (title: string) => `Refresh ${title.toLowerCase()}`,
  search: (title: string) => `Search ${title.toLowerCase()}`,
  searchPlaceholder: 'Search by symbol or name',
  retry: 'Retry',
  genericError: 'Something went wrong while loading. Try again.',
  noMatches: (query: string) => `No matches for “${query}”`,
  actions: 'Actions',
  rowActions: (row: string) => `Actions for ${row}`,
  rowAction: (action: string, row: string) => `${action} ${row}`,
} as const;

export const notFoundCopy = {
  page: 'Page not found',
  strategy: 'This strategy no longer exists',
  scanner: 'This scanner no longer exists',
  backtest: 'This backtest no longer exists',
  explanation: 'The view you asked for is not part of this version of Tradeloom, or the item was removed.',
  goHome: 'Go to Home',
} as const;

export const toastCopy = {
  region: 'Notifications',
  dismiss: 'Dismiss notification',
} as const;

export const topBarCopy = {
  orders: 'Order Book',
  ordersTip: 'Open the order book',
  positions: 'Positions',
  positionsTip: 'Open positions',
} as const;

export const profileMenuCopy = {
  menu: 'Account menu',
  notLoggedIn: 'Not logged in',
  logIn: 'Log in',
  profile: 'Profile',
  holdings: 'Holdings',
  settings: 'Settings',
  help: 'Help',
  darkTheme: 'Dark theme',
  version: (v: string) => `v${v}`,
  loadFailed: (reason: string) => `Your Kite profile could not be loaded: ${reason}`,
  profilePending: 'Loading profile…',
  profileUnavailable: 'Profile not loaded',
  retry: 'Retry loading profile',
  themeFailed: (reason: string) => `The theme could not be saved: ${reason}`,
} as const;
