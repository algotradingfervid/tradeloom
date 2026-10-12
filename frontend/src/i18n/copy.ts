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

export const positionsCopy = {
  title: 'Positions',
  description: 'Open and closed positions for today, with live P&L.',
  tabs: 'Position type',
  net: 'Net',
  day: 'Day',
  empty: 'No positions',
  loading: 'Loading positions',
  totalPnl: 'Total P&L',
  // P0-3 Ruling 38: same flag and hint as Holdings.
  outOfScope: holdingsCopy.outOfScope,
  outOfScopeHint: holdingsCopy.outOfScopeHint,
  cols: { product: 'Product', symbol: 'Symbol', qty: 'Qty', avg: 'Avg', ltp: 'LTP', pnl: 'P&L', net: 'Net change' },
} as const;

export const ordersCopy = {
  title: 'Order Book',
  description: 'Every order in your Kite account today: open, executed, rejected and cancelled.',
  searchPlaceholder: 'Search by instrument, order ID or source',
  empty: 'No orders today',
  loading: 'Loading orders',
  sections: { open: 'Open', executed: 'Executed', rejected: 'Rejected', cancelled: 'Cancelled' },
  sectionEmpty: { open: 'No open orders', executed: 'No executed orders', rejected: 'No rejected orders', cancelled: 'No cancelled orders' },
  // Go `orders.OrderRow.source`: "tradeloom" for orders tagged TL…, "external" for orders placed in Kite itself.
  sources: { tradeloom: 'Tradeloom', external: 'Kite' },
  outOfScope: holdingsCopy.outOfScope,
  outOfScopeHint: holdingsCopy.outOfScopeHint,
  cols: { time: 'Time', type: 'Type', instrument: 'Instrument', product: 'Product', qty: 'Qty', avg: 'Avg price', status: 'Status', source: 'Source' },
  updated: (symbol: string, status: string) => `Order ${symbol}: ${status}`,
} as const;

/** A background re-read (polling) failed: the view keeps what it shows (Task 10 M8, Ruling 103). */
export const portfolioCopy = {
  staleNotice: (reason: string) => `Showing the last loaded data. The latest read failed: ${reason}`,
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

// --- P0-4 Task 9: session gate, onboarding (setup / login / risk disclosure), shell banners, logout note.
export const gateCopy = { starting: 'Starting Tradeloom…' } as const;

export const setupCopy = {
  brand: 'Tradeloom',
  title: 'Connect your Kite account',
  intro: 'Tradeloom talks to Zerodha Kite through your own Kite Connect app. Charts, live prices and backtests need the paid Connect plan; holdings, positions and orders also work on the free Personal plan.',
  step1: 'In the Kite developer console, create an app and register this redirect URL:',
  redirectLabel: 'Redirect URL',
  redirectHint: 'You can change the port later in Settings → Kite connection.',
  copy: 'Copy',
  copied: 'Redirect URL copied',
  copyUnavailable: 'Copy is not available here — select the URL and copy it',
  step2: "Then enter the app's API key and API secret. The secret is kept in your system keyring, never in files or logs.",
  // Deviation (Task 9): the encrypted-file backend (FR-ACCT-013) must not be described as the keyring.
  step2File: "Then enter the app's API key and API secret. The secret is kept in your encrypted secrets file, never in logs.",
  apiKey: 'API key',
  apiSecret: 'API secret',
  apiKeyRequired: 'Enter the API key',
  secretRequired: 'Enter the API secret',
  save: 'Save and continue',
  helpLink: 'How to create a Kite Connect app',
  storeTitle: 'Choose where to keep your Kite secret',
  noKeyring: 'No system keyring was found on this computer. Choose one of these two options.',
  storeLegend: 'Secret storage',
  encrypted_file: 'Encrypted file protected by a passphrase',
  encfileHint: 'The secret is encrypted on disk. You enter the passphrase once each time Tradeloom starts.',
  no_storage: "Don't store credentials",
  refuseHint: 'Tradeloom keeps no Kite credentials and therefore cannot connect to Kite.',
  refusedNotice: 'Credentials are not stored, so Tradeloom cannot connect to Kite. Choose the encrypted file to continue, or close the app.',
  passphrase: 'Passphrase',
  passphraseRepeat: 'Repeat passphrase',
  passphraseShort: 'Use at least 8 characters',
  passphraseMismatch: 'The passphrases do not match',
  continue: 'Continue',
  unlockTitle: 'Unlock your Kite credentials',
  unlockIntro: 'Enter the passphrase that protects your encrypted secrets file.',
  unlock: 'Unlock',
} as const;

export const loginCopy = {
  title: 'Log in to Kite',
  intro: 'Kite sessions end every morning, so you log in once a day. Your password and TOTP are entered only on the Kite page in your browser.',
  expired: 'Your Kite session has ended. Log in to continue.',
  button: 'Log in with Kite',
  waiting: 'Waiting for the Kite login to finish in your browser…',
  cancel: 'Cancel login',
  pasteToggle: 'Login did not come back to Tradeloom?',
  pasteLabel: 'Redirect URL or request token',
  pasteHint: 'After logging in, copy the full address from the browser tab (it contains request_token=) and paste it here.',
  pasteRequired: 'Paste the redirect URL or the request token',
  pasteSubmit: 'Complete login',
  // Carry-forward (Ruling 101): redirect-URL guidance with Go's RedirectURL(port).
  redirectGuidance: (url: string) =>
    `After you log in, Kite sends the browser back to ${url}. It must match the redirect URL registered for your app in the Kite developer console.`,
  settingsLink: 'Change the login port or the Kite credentials',
} as const;

export const riskCopy = { heading: 'Before you start', terms: 'Terms of use', privacy: 'Privacy notice', accept: 'I understand' } as const;

export const bannerCopy = { expired: 'Kite session expired — log in to continue', logIn: 'Log in' } as const;

export const logoutCopy = {
  // Fallback when Go reports remoteInvalidated: false without a message (Go normally sends msgNotInvalidated).
  notInvalidated: 'Logged out. Kite could not be reached; the old Kite session ends by itself at 06:00 IST.',
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
  removeMessage: (date: string) => `The entry for ${date} is removed. The bundled calendar applies to that day again.`,
  save: 'Save',
  empty: 'No holidays or special sessions are listed.',
  cols: { date: 'Date', exchange: 'Exchange', type: 'Type', session: 'Session', source: 'Source' },
  tableCaption: 'Calendar entries',
  // One label per `CalendarEntryDTO.type` Go lists; the dialog offers all but the inferred type.
  dayTypes: { holiday: 'Holiday', special: 'Special session', trading: 'Trading day', 'inferred-non-trading': 'Non-trading day (inferred)' },
  // `CalendarEntryDTO.source` of an override: Settings dialog, CSV import or inference (Go lists overrides only).
  sources: { user: 'Added by you', import: 'Imported', inferred: 'Inferred' } as Record<string, string>,
  phases: { 'pre-open': 'Pre-open', continuous: 'Continuous trading', closing: 'Closing session', 'post-closing': 'Post-closing', closed: 'Closed' },
  fields: { date: 'Date', exchange: 'Exchange', type: 'Type', target: 'Phase to edit', phase: 'Session phase', start: 'Start', end: 'End' },
  phaseList: 'Phases of this day',
  newPhase: 'New phase',
  inferredHint: 'This day was marked closed because no market data arrived for it. Saving it makes it a holiday added by you.',
  tradingTimesHint: 'Leave Start and End empty for the regular session times.',
  dateRequired: 'Choose a date',
  timesInvalid: 'Enter a start time before the end time',
  continuousRequired: 'A session needs a continuous trading phase',
  // Go's own wording (internal/service/calendar.go parseEntry).
  duplicateKind: 'Each phase kind appears at most once.',
} as const;

export const holdingsCardCopy = {
  title: 'Holdings',
  loading: 'Loading holdings summary',
  error: (reason: string) => `Holdings could not be loaded: ${reason}`,
  staleNotice: (reason: string) => `Showing the last loaded figures. The latest read failed: ${reason}`,
  empty: 'No holdings',
  open: 'Open the Holdings view',
  currentValue: 'Current value',
  investment: 'Investment',
  pnl: 'P&L',
  dayChange: "Day's change",
} as const;

// --- Settings (P0-4 Task 14)
export const settingsViewCopy = {
  nav: 'Settings groups',
  groups: {
    kite: 'Kite connection', trading: 'Trading', appearance: 'Appearance', chart: 'Chart', home: 'Home',
    universes: 'Universes & baskets', calendar: 'Market calendar', data: 'Data', notifications: 'Notifications',
    shortcuts: 'Shortcuts', limits: 'Limits', updates: 'Updates', diagnostics: 'Diagnostics', tours: 'Tours', about: 'About',
  },
  wholeNumber: 'Enter a whole number',
} as const;

export const kiteSettingsCopy = {
  credentials: 'Credentials',
  apiKey: 'API key',
  apiSecret: 'API secret',
  dataPlan: 'Data plan',
  plan: { connect: 'Kite Connect (market data available)', none: 'Orders and portfolio only (no data plan)', unknown: 'Checked after the next login' },
  secretStorage: 'Secret storage',
  // Keys are P0-1 secrets backends as `CredentialsInfo.secretBackend` / `SecretStorageInfo.backend` report them.
  backend: { keyring: 'System keyring', file: 'Encrypted file (secrets.enc)', locked: 'Encrypted file (locked)', none: 'Not stored' } as Record<string, string>,
  change: 'Change',
  changeTitle: 'Change Kite credentials',
  changeConfirmTitle: 'Change Kite credentials?',
  changeConfirmMessage: 'Changing the API key logs you out of Tradeloom; you will log in to Kite again.',
  changeConfirm: 'Log out and continue',
  changeLoginBusy: 'Finish or cancel the current login before changing credentials.',
  // Same wording as Go's refusals (internal/kite/credentials.go, session.go), checked here before logging out.
  invalidKey: 'The API key should be 6–64 letters and digits. Copy it from your app in the Kite developer console.',
  invalidSecret: 'The API secret should be 6–64 letters and digits. Copy it from your app in the Kite developer console.',
  identical: 'The API key and the API secret are identical. Paste each from its own field in the Kite developer console.',
  loggedOutKept: 'You are logged out; your previous credentials are still saved.',
  save: 'Save',
  saved: 'Kite credentials saved. Log in again to use them.',
  remove: 'Remove credentials',
  removeTitle: 'Remove Kite credentials?',
  removeMessage: "The API key, the API secret and today's access token are deleted from this computer. You will need to set them up again before using Kite.",
  migrate: 'Move secrets to the system keyring',
  migrated: 'Secrets moved to the system keyring; the encrypted file was deleted.',
  fileNotRemovedTitle: 'Old secrets file not removed',
  fileNotRemoved: 'Your secrets are now in the system keyring, but the old encrypted file (secrets.enc) is still on disk.',
  fileNotRemovedRetry: 'Try removing it again',
  leftoverFileInfo: 'An old encrypted secrets file (secrets.enc) is still on disk. Tradeloom uses the system keyring and does not read it.',
  loadFailed: 'The Kite connection details could not be loaded.',
  loadFailedRetry: 'Retry loading Kite connection',
  login: 'Login',
  redirectUrl: 'Redirect URL',
  port: 'Login callback port',
  portHint: 'Register the matching redirect URL in the Kite developer console after changing this.',
  portChangedTitle: 'Redirect URL changed',
  portChanged: (url: string) => `Update the redirect URL of your app in the Kite developer console to ${url}, or the next login will fail.`,
} as const;

export const appearanceCopy = {
  title: 'Theme and layout',
  darkTheme: 'Dark theme',
  followSystem: 'Follow system theme',
  followSystemHint: 'Use the light or dark preference of your desktop.',
  followingHint: 'Controlled by your desktop while "Follow system theme" is on.',
  sidebarCollapsed: 'Collapse the sidebar to icons',
  saveFailed: (reason: string) => `The setting could not be saved: ${reason}`,
} as const;

export const chartSettingsCopy = {
  title: 'Chart defaults',
  timeframe: 'Default chart timeframe',
  hint: 'Used the first time an instrument is charted; afterwards each chart remembers its own timeframe.',
} as const;

export const aboutCopy = {
  app: 'Tradeloom',
  version: 'Version',
  build: 'Build',
  schema: 'Database schema',
  legal: 'Legal',
  terms: 'Terms of use',
  privacy: 'Privacy notice',
  risk: 'Risk disclosure',
  licences: 'Third-party licences',
  component: 'Component',
  licenceVersion: 'Version',
  licence: 'Licence',
} as const;

export const dataCopy = {
  storage: 'Storage',
  dataDir: 'Data folder',
  appDb: 'App database',
  candles: 'Candle store',
  candleUsage: (used: string, budget: string) => `${used} of ${budget}`,
  instruments: 'Instrument list updated',
  refreshInstruments: 'Refresh instrument list now',
  instrumentsRefreshed: 'Instrument list refreshed',
  backfill: 'History downloads waiting',
  backfillOpen: (n: number) => (n === 0 ? 'None' : n === 1 ? '1 job' : `${n} jobs`),
  clearCandles: 'Clear cached candles',
  clearTitle: 'Clear cached candles?',
  clearMessage: 'All downloaded candles are deleted. Charts download their data again the next time they open. Watchlists, settings and other data are not affected.',
  cleared: 'Cached candles cleared',
  backup: 'Backup',
  includeCandles: (size: string) => `Include the candle store (${size})`,
  backupNow: 'Back up now',
  backingUp: 'Backing up…',
  backupStage: (done: number, total: number) => `Backing up… (${done} of ${total})`,
  backupSaved: (path: string) => `Backup saved to ${path}`,
  backupFailed: (reason: string) => `Backup failed: ${reason}`,
  scheduled: 'Scheduled daily backup',
  scheduledHint: 'Runs once a day after 16:00 IST, or when you quit after 16:00 if it has not run yet.',
  folder: 'Backup folder',
  noFolder: 'Not chosen — the backups folder inside the data folder',
  chooseFolder: 'Choose folder…',
  keep: 'Backups to keep',
  keepRange: 'Keep between 1 and 365 backups',
  restore: 'Restore',
  restoreIntro: 'Replace all Tradeloom data with the contents of a backup archive. Kite credentials are never part of a backup.',
  restoreButton: 'Restore from a backup…',
  restoreTitle: 'Restore this backup?',
  restoreConfirm: 'Restore',
  restoreFrom: (when: string, version: string) => `Backup made on ${when} by Tradeloom v${version}. It replaces:`,
  count: (label: string, n: number) => `${label}: ${n}`,
  // Keys of the P0-1 backup manifest `counts`.
  counts: { strategies: 'Strategies', scanners: 'Scanners', backtest_runs: 'Backtest runs', deployments: 'Deployments', watchlists: 'Watchlists', order_journal: 'Order journal entries' },
  withCandles: 'The archive includes the candle store.',
  withoutCandles: 'The archive does not include candles; charts will download history again.',
  restoreWarning: 'Your current data folder is not deleted: it is renamed to end in “.replaced-” and the date and time, and the restored data takes its place. Earlier backups in the default backups folder inside the data folder move there with it. Tradeloom restarts when the restore finishes; settings you are still editing are discarded.',
  restoreSecrets: 'If your Kite credentials are kept in the encrypted secrets file (secrets.enc), that file stays in the “.replaced-” folder and you will be asked for your Kite API key and secret again. On a different computer you will be asked for them too.',
  cannotRestore: 'This backup cannot be restored',
  restoring: 'Restoring backup',
  restoreStage: (done: number, total: number) => `Restoring… (${done} of ${total})`,
  restarting: 'Restore complete. Tradeloom is restarting…',
  restartingAfterFailure: 'Tradeloom is restarting…',
  restoreFailed: (reason: string) => `Restore failed: ${reason}`,
  restoreEnded: 'The restore stopped without restarting Tradeloom.',
} as const;

export const shortcutsCopy = {
  title: 'Keyboard shortcuts',
  intro: 'Every shortcut can be changed. A key combination can be used only once within overlapping areas of the app.',
  action: 'Action',
  scope: 'Where',
  binding: 'Shortcut',
  scopes: { global: 'Everywhere', view: 'Current view', chart: 'Chart', builder: 'Builder' },
  unbound: 'Not set',
  change: 'Change',
  changeFor: (label: string) => `Change shortcut for ${label}`,
  dialogTitle: (label: string) => `Change shortcut: ${label}`,
  capture: 'New shortcut',
  pressKeys: 'Press the new key combination',
  // Keys Go's ParseBinding accepts (internal/settings/shortcuts.go normaliseKey); Task 6 carry-forward.
  unsupportedKey: 'This key cannot be used in a shortcut. Use a letter, a digit, F1–F24, an arrow or navigation key, Space, or one of ` - = [ ] \\ ; \' , . /',
  save: 'Save',
  saved: 'Shortcut saved',
  conflict: (label: string) => `Already used by ${label}`,
  reset: 'Reset to defaults',
  resetTitle: 'Reset to defaults',
  resetMessage: 'All shortcuts return to their original keys.',
  resetDone: 'Shortcuts reset to defaults',
} as const;

export const diagnosticsCopy = {
  title: 'Logs and support',
  level: 'Log detail',
  // Values are the Go `log.level` setting (P0-1): lower-case "info" | "debug".
  levels: [{ value: 'info', label: 'Normal (info)' }, { value: 'debug', label: 'Detailed (debug)' }],
  levelHint: 'Changes apply immediately. Secrets are always removed from logs.',
  openLogs: 'Open logs folder',
  bundle: 'Create support bundle',
  bundleHint: 'The bundle contains logs, settings without secrets, version information, the market calendar version, the last 200 order-journal rows and recent deployment log excerpts.',
  bundleSaved: (path: string) => `Support bundle saved to ${path}`,
} as const;

export const crashCopy = {
  message: (when: string) => `The app crashed last time (${when}).`,
  view: 'View report',
  dismiss: 'Dismiss crash notice',
} as const;

export const instrumentCopy = {
  future: 'Future',
  alsoOnNse: 'Also on NSE',
  alsoOnNseHint: 'This company is also listed on NSE',
  staleQuote: 'Last known price, not live yet',
  // P0-3 Ruling 38: the same "View only" flag and hint as the portfolio views.
  outOfScope: holdingsCopy.outOfScope,
  outOfScopeHint: holdingsCopy.outOfScopeHint,
  // InstrumentRow (P0-5 Task 14; reused by P3 scanners): row actions and the polled/stale text hints.
  showOnChart: (name: string) => `Show ${name} on chart`,
  remove: (name: string) => `Remove ${name}`,
  polled: 'polled',
  polledHint: 'Live subscription limit reached — updates every 5 s',
  stale: 'last known',
} as const;

export const chartHeaderCopy = {
  controls: 'Chart controls',
  back: 'Back',
  changeSymbol: 'Change symbol',
  searchInstruments: 'Search instruments',
  refresh: 'Refresh chart data',
  timeframe: 'Timeframe',
  more: 'More chart actions',
  openFullView: 'Open in full view',
} as const;

/** Index ticker strip in the top bar (FR-SHELL-004). The direction is spoken as a word, never by colour alone. */
export const tickerCopy = {
  strip: 'Index ticker',
  show: (name: string) => `Show ${name} on the Home chart`,
  noPrice: 'no price yet',
  up: 'up',
  down: 'down',
  flat: 'unchanged',
} as const;

export const profileCopy = {
  title: 'Profile',
  description: 'Your Zerodha account details as returned by Kite.',
  loading: 'Loading profile',
  fields: { name: 'Name', email: 'Email', clientId: 'Client ID', broker: 'Broker', exchanges: 'Exchanges', products: 'Products', orderTypes: 'Order types' },
  session: 'Session',
  loggedInAt: (t: string) => `Logged in at ${t}`,
  expires: (t: string) => `Expires ${t}`,
  logOut: 'Log out',
  confirmTitle: 'Log out of Kite?',
  confirmMessage: 'Live prices and the order book stop updating until you log in again. Your watchlists, settings and cached candles stay on this computer.',
  // Task 13 adaptations: a failed load (Ruling 90 M1) and an expired session are said in text, never an endless skeleton.
  loadFailed: 'Your profile could not be loaded from Kite.',
  retry: 'Retry',
  expired: 'Your Kite session has ended. Log in again to see your profile.',
  logIn: 'Log in',
} as const;

/** Chart symbol search (FR-CHART-005, P0-5 Task 9). */
export const symbolSearchCopy = {
  title: 'Search instruments',
  inputLabel: 'Search instruments',
  placeholder: 'Search, e.g. SBIN, TCS',
  quickChips: 'Quick picks',
  results: 'Results',
  colInstrument: 'Instrument',
  colCompany: 'Company name',
} as const;

/** Shared by every instrument search box (useInstrumentSearch): the polite result-count announcement. */
export const instrumentSearchCopy = {
  noMatch: 'No instruments match',
  count: (n: number) => (n === 1 ? '1 result' : `${n} results`),
} as const;

/** Watchlist inline search-and-add (FR-WATCH-004, P0-5 Task 15). Count and no-match text: instrumentSearchCopy. */
export const watchlistSearchCopy = {
  label: 'Search instruments to add',
  placeholder: 'Search to add, e.g. SBIN',
  results: 'Search results',
  alreadyIn: (list: string) => `Already in ${list}`,
  addTo: (list: string) => `Add to ${list}`,
  added: (symbol: string, list: string) => `Added ${symbol} to ${list}`,
  searchFailed: 'Search failed. Try again.',
  addFailed: (symbol: string, reason: string) => `Could not add ${symbol}: ${reason}`,
} as const;

/** P0-5 Task 14: watchlist rows (FR-WATCH-001/005/006). */
export const watchlistRowsCopy = {
  empty: 'Search to add instruments',
  symbolColumn: 'Symbol',
  priceColumn: 'LTP / Change',
  actionsColumn: 'Actions',
  tableLabel: (list: string) => `${list} instruments`,
  removed: (name: string, list: string) => `Removed ${name} from ${list}`,
  undo: 'Undo',
  missing: 'Instrument no longer available',
  removeMissing: (row: number) => `Remove unavailable instrument in row ${row}`,
  missingName: 'unavailable instrument',
  removeFailed: (reason: string) => `Could not remove the instrument: ${reason}`,
  undoFailed: (reason: string) => `Could not restore the instrument: ${reason}`,
  moveFailed: (reason: string) => `Could not reorder the watchlist: ${reason}`,
  moved: (name: string, position: number, count: number) => `${name} moved to ${position} of ${count}`,
  readOnlyMisuse: 'WatchlistRows: the list is not the active watchlist; rendering it read-only (store actions act on the active list)',
} as const;

/** Shared multi-select instrument picker (FR-WATCH-007, P0-5 Task 17). Count and no-match text: instrumentSearchCopy. */
export const instrumentPickerCopy = {
  title: 'Add instruments',
  search: 'Search instruments',
  placeholder: 'Search, e.g. SBIN, TCS',
  typeToSearch: 'Type to search',
  lists: 'Lists',
  results: 'Instruments',
  selectAll: 'Select all',
  select: (name: string) => `Select ${name}`,
  remove: (name: string) => `Remove ${name}`,
  selected: (n: number, max?: number) => (max === undefined ? `Selected (${n})` : `Selected (${n} / ${max})`),
  selectedList: 'Selected instruments',
  noneSelected: 'Nothing selected yet',
  limit: (max: number) => `Up to ${max} instruments`,
  emptyList: 'This list has no instruments to offer here',
  saveAs: 'Save as watchlist',
  name: 'Watchlist name',
  save: 'Save',
  cancel: 'Cancel',
  created: (name: string) => `Watchlist ${name} created`,
  done: 'Done',
} as const;
