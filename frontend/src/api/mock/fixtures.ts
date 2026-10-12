import type {
  AppInfo, BackupInfo, BookSnapshot, CalendarInfo, CredentialsInfo, DataStats, HoldingRow, HoldingsSnapshot, KiteOrder,
  OrderRow, PositionRow, PositionsSnapshot, Profile, SecretStorageInfo, SessionStatus, ShortcutAction, StatusSnapshot, TickDTO,
} from '../backend';
import type { MockState } from './index';

export const DAY_MS = 86_400_000;
const IST_OFFSET_MS = 19_800_000;
/** 12 Oct 2026 10:30 IST — reference "now" for fixtures. */
export const FIXED_NOW = Date.UTC(2026, 9, 12, 5, 0, 0);
/** Go's zero `time.Time` as Wails serialises it. */
export const ZERO_TIME = '0001-01-01T00:00:00Z';
export const iso = (ms: number): string => new Date(ms).toISOString();

export function next0600Ist(now: number): number {
  const istMidnight = Math.floor((now + IST_OFFSET_MS) / DAY_MS) * DAY_MS - IST_OFFSET_MS;
  const six = istMidnight + 6 * 3_600_000;
  return six > now ? six : six + DAY_MS;
}

const REDIRECT_URL = 'http://127.0.0.1:7420/kite/callback';
const keyringSecrets: SecretStorageInfo = { mode: 'keyring', backend: 'keyring', keyringAvailable: true, fileExists: false, choices: [] };
const configured: CredentialsInfo = { configured: true, apiKeyMasked: '••••9f2c', port: 7420, redirectUrl: REDIRECT_URL, secretBackend: 'keyring' };

export function readySession(overrides: Partial<SessionStatus> = {}): SessionStatus {
  return {
    state: 'ready', sessionId: 'sess-1', userId: 'AB1234',
    loggedInAtMs: Date.UTC(2026, 9, 12, 3, 32), expiresAtMs: next0600Ist(FIXED_NOW), dataPlan: 'connect', offline: false,
    riskAckRequired: false, errorCode: '', errorMessage: '',
    credentials: { ...configured }, secrets: { ...keyringSecrets }, ...overrides,
  };
}

export function tickFixture(token: number, ltp: number, close = ltp): TickDTO {
  const change = ltp - close;
  return { token, ltp, close, change, changePct: close > 0 ? (change / close) * 100 : 0, volume: 0, tsMs: FIXED_NOW, polled: false };
}

export const profileFixture: Profile = {
  userId: 'AB1234', name: 'Asha Rao', shortName: 'Asha', email: 'asha@example.com', broker: 'Zerodha', avatarUrl: '', initials: 'AR',
  exchanges: ['NSE', 'BSE', 'NFO', 'BFO'], products: ['CNC', 'MIS', 'NRML'], orderTypes: ['MARKET', 'LIMIT', 'SL', 'SL-M'],
};

// FR-HOME-002 AC1: A 10 @ 100 (LTP 110, prev close 105); B 5 @ 200 (LTP 190, prev close 200) — Go already sums B's 3 settled + 2 T1.
export const holdingA: HoldingRow = {
  exchange: 'NSE', symbol: 'RELIANCE', token: 738561, qty: 10, avgPaise: 10_000, ltpPaise: 11_000, prevClosePaise: 10_500,
  investedPaise: 100_000, currentPaise: 110_000, pnlPaise: 10_000, netChangeBp: 1000, dayChangePaise: 5_000, dayChangeBp: 476, outOfScope: false,
};
export const holdingB: HoldingRow = {
  exchange: 'NSE', symbol: 'TCS', token: 2953217, qty: 5, avgPaise: 20_000, ltpPaise: 19_000, prevClosePaise: 20_000,
  investedPaise: 100_000, currentPaise: 95_000, pnlPaise: -5_000, netChangeBp: -500, dayChangePaise: -5_000, dayChangeBp: -500, outOfScope: false,
};

// FR-ORD-002 AC1: P&L +120 and -45.50 → total 74.50.
export const positionOpen: PositionRow = {
  key: 'net|NSE|INFY|MIS', exchange: 'NSE', symbol: 'INFY', token: 408065, product: 'MIS',
  qty: 10, avgPaise: 10_000, ltpPaise: 11_200, pnlPaise: 12_000, netChangeBp: 1200, multiplier: 1, closed: false, outOfScope: false,
};
export const positionClosed: PositionRow = {
  key: 'net|NSE|TATAMOTORS|MIS', exchange: 'NSE', symbol: 'TATAMOTORS', token: 884737, product: 'MIS',
  qty: 0, avgPaise: 10_455, ltpPaise: 10_100, pnlPaise: -4_550, netChangeBp: -340, multiplier: 1, closed: true, outOfScope: false,
};

const order = (o: Pick<OrderRow, 'orderId' | 'timeMs' | 'side' | 'symbol' | 'token' | 'product' | 'qty' | 'filledQty' | 'avgPricePaise' | 'status' | 'section'> & Partial<OrderRow>): OrderRow => ({
  exchange: 'NSE', orderType: 'LIMIT', variety: 'regular', pricePaise: 0, triggerPaise: 0, statusMessage: '', tag: '', source: 'external', outOfScope: false, ...o,
});

export const ordersFixture: OrderRow[] = [
  order({ orderId: '261012000000001', timeMs: FIXED_NOW - 600_000, side: 'Buy', symbol: 'RELIANCE', token: 738561, product: 'CNC', qty: 10, filledQty: 0, pricePaise: 10_900, avgPricePaise: 0, status: 'OPEN', section: 'open' }),
  order({ orderId: '261012000000002', timeMs: FIXED_NOW - 1_200_000, side: 'Sell', symbol: 'INFY', token: 408065, product: 'MIS', qty: 5, filledQty: 5, avgPricePaise: 151_025, status: 'COMPLETE', section: 'executed', orderType: 'MARKET', tag: 'TLd7', source: 'tradeloom' }),
  order({ orderId: '261012000000003', timeMs: FIXED_NOW - 1_800_000, side: 'Buy', symbol: 'TCS', token: 2953217, product: 'CNC', qty: 1, filledQty: 0, avgPricePaise: 0, status: 'REJECTED', statusMessage: 'Insufficient funds', section: 'rejected' }),
  order({ orderId: '261012000000004', timeMs: FIXED_NOW - 2_400_000, side: 'Buy', symbol: 'HDFCBANK', token: 341249, product: 'CNC', qty: 2, filledQty: 0, avgPricePaise: 0, status: 'CANCELLED', section: 'cancelled' }),
];

const outOfScopeCount = (rows: { outOfScope: boolean }[]) => rows.filter((r) => r.outOfScope).length;

/** Same rules as Go `orders.BuildBook`: counts per section, newest first, out-of-scope rows counted. */
export function bookOf(orders: OrderRow[], asOfMs = FIXED_NOW - 30_000): BookSnapshot {
  const counts = { open: 0, executed: 0, rejected: 0, cancelled: 0 };
  for (const o of orders) counts[o.section] += 1;
  return { orders: [...orders].sort((a, b) => b.timeMs - a.timeMs), counts, outOfScopeCount: outOfScopeCount(orders), asOfMs, stale: false };
}

/** Same totals as Go `orders.BuildPositions`. */
export function positionsOf(net: PositionRow[], day: PositionRow[], asOfMs = FIXED_NOW - 30_000): PositionsSnapshot {
  const total = (rows: PositionRow[]) => rows.reduce((s, r) => s + r.pnlPaise, 0);
  return {
    net, day, netTotalPnlPaise: total(net), dayTotalPnlPaise: total(day),
    outOfScopeCount: outOfScopeCount(net) + outOfScopeCount(day), asOfMs, stale: false,
  };
}

/** Same summary as Go `orders.Summarize` (FR-HOME-002 card definitions). */
export function holdingsOf(rows: HoldingRow[], asOfMs = FIXED_NOW - 30_000): HoldingsSnapshot {
  const current = rows.reduce((s, r) => s + r.currentPaise, 0);
  const invested = rows.reduce((s, r) => s + r.investedPaise, 0);
  const pnl = current - invested;
  return {
    rows,
    summary: {
      empty: rows.length === 0, currentValuePaise: current, investmentPaise: invested, pnlPaise: pnl,
      pnlBp: invested === 0 ? 0 : Math.round((pnl * 10_000) / invested), dayChangePaise: rows.reduce((s, r) => s + r.dayChangePaise, 0),
      outOfScopeCount: outOfScopeCount(rows),
    },
    asOfMs,
    stale: false,
  };
}

/** An `order.update` payload (gokiteconnect JSON); defaults describe order 1 (RELIANCE, CNC, open). */
export function kiteOrder(overrides: Partial<KiteOrder> = {}): KiteOrder {
  return {
    order_id: '261012000000001', status: 'OPEN', status_message: '', exchange: 'NSE', tradingsymbol: 'RELIANCE',
    instrument_token: 738561, transaction_type: 'BUY', product: 'CNC', order_type: 'LIMIT', quantity: 10, filled_quantity: 0,
    average_price: 0, tag: '', ...overrides,
  };
}

const NAV_TARGETS = ['Home', 'Scalper', 'Strategies', 'Equity Builder', 'Live Strategies', 'Scanners', 'New Scanner', 'Scanner Alerts'];

// Mirrors the Go registry exactly (internal/settings/shortcuts.go DefaultShortcuts: ids, labels, scopes, defaults and
// order; scalper.* ship unbound). src/api/mock/shortcuts.test.ts compares it with the golden file that
// internal/settings TestDefaultShortcuts_Golden keeps in sync with Go. Test data only — the app always reads
// Settings.Shortcuts().
export const shortcutFixture: ShortcutAction[] = [
  ...NAV_TARGETS.map((label, i): ShortcutAction => ({ id: `nav.${i + 1}`, label: `Go to ${label}`, scope: 'global', defaultBinding: `Alt+${i + 1}`, binding: `Alt+${i + 1}` })),
  { id: 'toasts.focus', label: 'Focus toasts', scope: 'global', defaultBinding: 'Alt+T', binding: 'Alt+T' },
  { id: 'search.open', label: 'Open search', scope: 'global', defaultBinding: 'Ctrl+K', binding: 'Ctrl+K' },
  { id: 'help.open', label: 'Open help', scope: 'global', defaultBinding: 'F1', binding: 'F1' },
  { id: 'kill.switch', label: 'Kill switch', scope: 'global', defaultBinding: 'Ctrl+Shift+K', binding: 'Ctrl+Shift+K' },
  { id: 'chart.fullView', label: 'Full-view chart', scope: 'global', defaultBinding: 'F11', binding: 'F11' },
  { id: 'chart.trendLine', label: 'Draw trend line', scope: 'chart', defaultBinding: 'Alt+T', binding: 'Alt+T' },
  { id: 'chart.horizontalLine', label: 'Draw horizontal line', scope: 'chart', defaultBinding: 'Alt+H', binding: 'Alt+H' },
  { id: 'chart.horizontalRay', label: 'Draw horizontal ray', scope: 'chart', defaultBinding: 'Alt+J', binding: 'Alt+J' },
  { id: 'chart.verticalLine', label: 'Draw vertical line', scope: 'chart', defaultBinding: 'Alt+V', binding: 'Alt+V' },
  { id: 'chart.crossLine', label: 'Draw cross line', scope: 'chart', defaultBinding: 'Alt+C', binding: 'Alt+C' },
  { id: 'order.new', label: 'New order', scope: 'chart', defaultBinding: 'Shift+T', binding: 'Shift+T' },
  { id: 'scalper.buy', label: 'Scalper: buy', scope: 'view', defaultBinding: '', binding: '' },
  { id: 'scalper.sell', label: 'Scalper: sell', scope: 'view', defaultBinding: '', binding: '' },
  { id: 'scalper.exitAll', label: 'Scalper: exit all', scope: 'view', defaultBinding: '', binding: '' },
  { id: 'scalper.cancelAll', label: 'Scalper: cancel all', scope: 'view', defaultBinding: '', binding: '' },
  { id: 'builder.undo', label: 'Undo', scope: 'builder', defaultBinding: 'Ctrl+Z', binding: 'Ctrl+Z' },
  { id: 'builder.redo', label: 'Redo', scope: 'builder', defaultBinding: 'Ctrl+Shift+Z', binding: 'Ctrl+Shift+Z' },
  { id: 'watchlist.moveUp', label: 'Move item up', scope: 'view', defaultBinding: 'Alt+ArrowUp', binding: 'Alt+ArrowUp' },
  { id: 'watchlist.moveDown', label: 'Move item down', scope: 'view', defaultBinding: 'Alt+ArrowDown', binding: 'Alt+ArrowDown' },
];

const statusFixture: StatusSnapshot = {
  feed: { state: 'live', attempt: 0, nextRetryMs: 0, lastTickMs: FIXED_NOW - 1000, connections: 1, tokens: 3 },
  market: { exchange: 'NSE', phase: 'continuous', dayType: 'trading', label: 'Open', tooltip: 'Normal market 09:15–15:30 IST', atMs: FIXED_NOW },
  clock: { millis: 120, source: 'ticks', samples: 40, warn: false, skewed: false, updatedAtMs: FIXED_NOW },
};

const statsFixture: DataStats = {
  dataDir: '/home/asha/.local/share/tradeloom', appDbBytes: 2_400_000, candleBytes: 734_003_200,
  candleBudgetBytes: 10_737_418_240, instrumentsImportedAtMs: FIXED_NOW - 7_200_000, backfillOpenJobs: 0,
};

const backupFixture: BackupInfo = {
  path: '/home/asha/Backups/tradeloom-backup-20261011-170000.tar.zst', ok: true, reason: '', schema: 1,
  supportedSchema: 1, appVersion: '0.1.0', createdAt: iso(FIXED_NOW - DAY_MS),
  counts: { strategies: 0, scanners: 0, backtest_runs: 0, deployments: 0, watchlists: 3, order_journal: 0 },
  includesCandles: false, sizeBytes: 1_200_000,
};

const calendarFixture: CalendarInfo = {
  version: '2026.1', coversUntil: '2026-12-31', stale: false,
  entries: [
    { exchange: 'NSE', date: '2026-10-20', type: 'holiday', phases: [], source: 'shipped' },
    { exchange: 'NSE', date: '2026-11-08', type: 'special', phases: [{ kind: 'continuous', start: '18:15', end: '19:15' }], source: 'shipped' },
  ],
};

const infoFixture: AppInfo = {
  version: '0.1.0', build: 'go1.25.14 webkit2_41,noorders', schemaVersion: 1, dataDir: '/home/asha/.local/share/tradeloom',
  licences: [
    { name: 'react', version: '19.3.0', licence: 'MIT' },
    { name: 'lightweight-charts', version: '5.2.1', licence: 'Apache-2.0' },
  ],
};

const deepCopy = <T,>(v: T): T => JSON.parse(JSON.stringify(v)) as T;

function base(): MockState {
  return deepCopy<MockState>({
    session: readySession(), status: statusFixture, profile: profileFixture,
    book: bookOf(ordersFixture), positions: positionsOf([positionOpen, positionClosed], [
      { ...positionOpen, key: 'day|NSE|INFY|MIS' }, { ...positionClosed, key: 'day|NSE|TATAMOTORS|MIS' },
    ]),
    holdings: holdingsOf([holdingA, holdingB]), settings: {}, uiState: {}, shortcuts: shortcutFixture, stats: statsFixture,
    backup: backupFixture, calendar: calendarFixture, info: infoFixture, crash: { path: '', atMs: 0 },
    folderChoice: '/home/asha/Backups', fileChoice: backupFixture.path, encPassphrase: '', loginListenerOk: true,
  });
}

export const demoSeed = (): MockState => base();

export function freshSeed(): MockState {
  const s = base();
  s.session = readySession({
    state: 'no_credentials', sessionId: '', userId: '', loggedInAtMs: 0, expiresAtMs: 0,
    credentials: { ...configured, configured: false, apiKeyMasked: '' },
  });
  s.status.feed = { ...s.status.feed, state: 'stopped', connections: 0, tokens: 0, lastTickMs: 0 };
  return s;
}

/** FR-ACCT-013: no keyring daemon and no secrets file → Go offers exactly two choices. */
export function noKeyringSeed(): MockState {
  const s = freshSeed();
  s.session.secrets = { mode: 'choose', backend: 'none', keyringAvailable: false, fileExists: false, choices: ['encrypted_file', 'no_storage'] };
  s.session.credentials.secretBackend = 'none';
  return s;
}

export function lockedSeed(): MockState {
  const s = freshSeed();
  s.session.state = 'locked';
  s.session.secrets = { mode: 'file', backend: 'locked', keyringAvailable: false, fileExists: true, choices: [] };
  s.session.credentials = { ...configured, secretBackend: 'file' };
  s.encPassphrase = 'correct horse';
  return s;
}

export function loggedOutSeed(): MockState {
  const s = freshSeed();
  s.session.state = 'logged_out';
  s.session.credentials = { ...configured };
  return s;
}

export function unackedSeed(): MockState {
  const s = base();
  s.session.riskAckRequired = true;
  return s;
}

/** A Kite Connect "Personal" key: logged in, portfolio works, no market-data plan (FR-ACCT-007). */
export function personalSeed(): MockState {
  const s = base();
  s.session.dataPlan = 'none';
  s.status.feed = { ...s.status.feed, state: 'stopped', connections: 0, tokens: 0 };
  return s;
}

export function emptySeed(): MockState {
  const s = base();
  s.holdings = holdingsOf([]);
  s.positions = positionsOf([], []);
  s.book = bookOf([]);
  return s;
}

export const LONG_NAME = 'BHARATIYA INTERNATIONAL INFRASTRUCTURE AND ENGINEERING SERVICES LIMITED';

export function longNamesSeed(): MockState {
  const s = base();
  s.holdings = holdingsOf(s.holdings.rows.map((h) => ({ ...h, symbol: LONG_NAME })));
  s.positions = positionsOf(s.positions.net.map((p) => ({ ...p, symbol: LONG_NAME })), s.positions.day.map((p) => ({ ...p, symbol: LONG_NAME })));
  s.book = bookOf(s.book.orders.map((o) => ({ ...o, symbol: LONG_NAME })));
  return s;
}

const SEEDS: Record<string, () => MockState> = {
  demo: demoSeed, fresh: freshSeed, nokeyring: noKeyringSeed, locked: lockedSeed, loggedout: loggedOutSeed,
  unacked: unackedSeed, personal: personalSeed, empty: emptySeed, longnames: longNamesSeed,
};

export function seedByName(name: string): MockState {
  return (SEEDS[name] ?? demoSeed)();
}
