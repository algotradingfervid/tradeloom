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
} as const;

export const chartSettingsCopy = {
  title: 'Chart defaults',
  timeframe: 'Default chart timeframe',
  hint: 'Used the first time an instrument is charted; afterwards each chart remembers its own timeframe.',
} as const;

export const aboutCopy = {
  app: 'Tradeloom',
  version: 'Version',
  versionValue: (v: string) => `v${v}`,
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
