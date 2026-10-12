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
} as const;

export const riskCopy = { heading: 'Before you start', terms: 'Terms of use', privacy: 'Privacy notice', accept: 'I understand' } as const;

export const bannerCopy = { expired: 'Kite session expired — log in to continue', logIn: 'Log in' } as const;

export const logoutCopy = {
  // Fallback when Go reports remoteInvalidated: false without a message (Go normally sends msgNotInvalidated).
  notInvalidated: 'Logged out. Kite could not be reached; the old Kite session ends by itself at 06:00 IST.',
} as const;
