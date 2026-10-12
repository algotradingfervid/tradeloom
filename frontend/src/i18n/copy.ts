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

export const holdingsCopy = {
  title: 'Holdings',
  description: 'Long-term equity holdings in your Kite account, valued at live prices.',
  empty: 'No holdings',
  refresh: 'Refresh',
  loading: 'Loading holdings',
  total: 'Total',
  outOfScope: 'View only',
  outOfScopeHint: 'Outside Tradeloom’s equity scope: shown for completeness and counted in the totals.',
  cols: { symbol: 'Symbol', qty: 'Qty', avg: 'Avg', ltp: 'LTP', invested: 'Invested', current: 'Current value', pnl: 'P&L', net: 'Net change', day: 'Day change' },
} as const;

export const ordersCopy = {
  updated: (symbol: string, status: string) => `Order ${symbol}: ${status}`,
} as const;

export const statusCopy = {
  label: {
    live: 'Live', connecting: 'Connecting', reconnecting: 'Reconnecting', offline: 'Offline', stopped: 'Not connected',
    no_data_plan: 'No data plan', logged_out: 'Logged out',
  },
  button: (label: string, closed: boolean) => `Market data status: ${label}${closed ? ', market closed' : ''}`,
  marketClosed: 'Market closed',
  panelTitle: 'Connection status',
  reconnect: 'Reconnect now',
  reconnectRequested: 'Reconnect requested',
  reconnectWaiting: 'The feed is already retrying on its own; Reconnect now cannot shorten the wait.',
  logIn: 'Log in',
  feed: {
    live: 'Market data: live',
    connecting: 'Market data: connecting',
    reconnecting: (attempt: number, next: string) => `Market data: reconnecting (attempt ${attempt}, next retry at ${next})`,
    offline: 'Market data: offline — retries exhausted or no network',
    stopped: 'Market data: not connected',
    no_data_plan: 'Market data: this needs a Kite Connect data plan',
    logged_out: 'Market data: not connected',
  },
  sessionValid: (when: string) => `Session valid until ${when}`,
  sessionExpired: 'Kite session expired — log in to continue',
  notLoggedIn: 'Not logged in to Kite',
  market: (label: string) => `Market: ${label}`,
  clock: (offset: string, source: string) => `Clock offset ${offset} s (${source})`,
  clockUnmeasured: 'Clock offset: not measured yet',
  clockSource: { ticks: 'from ticks', 'http-date': 'from Kite server time', none: 'not measured' },
  lastTick: (t: string) => `Last tick: ${t}`,
  clockWarning: (offset: string) => `Your computer clock differs from exchange time by ${offset} s. Tradeloom uses the corrected time.`,
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

export const calendarCopy = {
  title: 'Trading days and special sessions',
  version: 'Calendar version',
  coversUntil: 'Covers until',
  stale: 'Calendar may be stale',
  staleHint: 'It does not cover the next 30 days. Import an updated list or add the missing days.',
  staleToast: (until: string) => `Calendar may be stale — it ends on ${until}. Import an updated list in Settings → Market calendar.`,
  open: 'Open',
  import: 'Import CSV…',
  importHint: 'A CSV file of up to 1 MiB and 5,000 rows. Every row is checked first; if one is invalid, nothing is imported.',
  imported: 'Calendar imported',
  importFailed: (reason: string) => `Calendar import refused: ${reason}`,
  add: 'Add entry',
  addTitle: 'Add calendar entry',
  editTitle: 'Edit calendar entry',
  edit: 'Edit',
  remove: 'Remove',
  removeTitle: 'Remove calendar entry?',
  removeMessage: (date: string) => `The entry for ${date} is removed. Weekdays without an entry count as trading days.`,
  save: 'Save',
  empty: 'No holidays or special sessions are listed.',
  cols: { date: 'Date', exchange: 'Exchange', type: 'Type', session: 'Session', source: 'Source' },
  types: { holiday: 'Holiday', special: 'Special session', trading: 'Trading day' },
  // Labels for every `CalendarEntryDTO.type` Go can list (an inferred day is not offered in the dialog).
  typeLabels: { holiday: 'Holiday', special: 'Special session', trading: 'Trading day', 'inferred-non-trading': 'Non-trading day (inferred)' } as Record<string, string>,
  // `CalendarEntryDTO.source`: bundled file, Settings dialog, CSV import or inference (P0-2 `calendar.Entry.Source`).
  sources: { shipped: 'Bundled', user: 'Added by you', import: 'Imported', inferred: 'Inferred' } as Record<string, string>,
  phases: { 'pre-open': 'Pre-open', continuous: 'Continuous trading', closing: 'Closing session', 'post-closing': 'Post-closing', closed: 'Closed' },
  fields: { date: 'Date', exchange: 'Exchange', type: 'Type', phase: 'Session phase', start: 'Start', end: 'End' },
  tradingTimesHint: 'Leave Start and End empty for the regular session times.',
  dateRequired: 'Choose a date',
  timesInvalid: 'Enter a start time before the end time',
  continuousRequired: 'A session needs a continuous trading phase',
} as const;
