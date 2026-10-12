// Mirrors the P0-3 Wails DTOs field-for-field (camelCase JSON tags). Unix-ms fields end in `Ms` (0 = never/unset),
// money fields in `Paise` (integers), ratios in `Bp` (basis points). Types that P0-3 also emits into `events.gen.ts`
// are derived from it (only string enums are narrowed and Go `T[] | null` slices become `T[]`: the Wails adapter
// normalises null to []); the rest are written out here and checked against `wailsjs/go/models` in `wails.ts`.
import type * as G from './events.gen';

export type Ms = number;
export type Rfc3339 = string;

/** RFC3339 → Unix ms; Go's zero time ("0001-01-01T00:00:00Z") and unparsable values → 0. */
export function timeMs(t: Rfc3339): Ms {
  const v = Date.parse(t);
  return Number.isFinite(v) && v > 0 ? v : 0;
}

// --- Session (service.SessionStatus, kite.CredentialsInfo, service.SecretStorageInfo, kite.LoginStart, kite.LogoutResult, kite.Profile)
export type SessionStateName = 'no_credentials' | 'locked' | 'logged_out' | 'logging_in' | 'ready' | 'expired';
export type DataPlan = 'unknown' | 'connect' | 'none';
export type SecretMode = 'keyring' | 'file' | 'choose' | 'none';
export type SecretBackendName = 'keyring' | 'file' | 'none' | 'locked';
export type SecretChoice = 'encrypted_file' | 'no_storage';

export type CredentialsInfo = G.kite_CredentialsInfo;
export interface SecretStorageInfo extends Omit<G.service_SecretStorageInfo, 'mode' | 'backend' | 'choices'> {
  mode: SecretMode;
  backend: SecretBackendName;
  choices: SecretChoice[];
}
/** `Session.SaveCredentials` argument. `apiSecret` is write-only: never put it in a store, state or log. */
export interface CredentialsInput { apiKey: string; apiSecret: string }

export interface SessionStatus extends Omit<G.service_SessionStatus, 'state' | 'dataPlan' | 'credentials' | 'secrets'> {
  state: SessionStateName;
  dataPlan: DataPlan;
  credentials: CredentialsInfo;
  secrets: SecretStorageInfo;
}

export interface LoginStart { loginUrl: string; redirectUrl: string; listenerOk: boolean; listenerError: string }
export interface LogoutResult { remoteInvalidated: boolean; message: string }

export interface Profile {
  userId: string;
  name: string;
  shortName: string;
  email: string;
  broker: string;
  avatarUrl: string;
  initials: string;
  exchanges: string[];
  products: string[];
  orderTypes: string[];
}

// --- Portfolio (orders.* snapshots, P0-3 Task 12). `outOfScope` rows are kept and shown read-only.
export type OrderSection = 'open' | 'executed' | 'rejected' | 'cancelled';
export type OrderSide = 'Buy' | 'Sell';
export type OrderSource = 'tradeloom' | 'external';
export interface OrderRow extends Omit<G.orders_OrderRow, 'side' | 'section' | 'source'> {
  side: OrderSide;
  section: OrderSection;
  source: OrderSource;
}
export type SectionCounts = G.orders_SectionCounts;
export interface BookSnapshot extends Omit<G.orders_BookSnapshot, 'orders'> { orders: OrderRow[] }

/** `multiplier`: Kite multiplier; Go sends 1 when Kite reports 0. */
export type PositionRow = G.orders_PositionRow;
export interface PositionsSnapshot extends Omit<G.orders_PositionsSnapshot, 'net' | 'day'> { net: PositionRow[]; day: PositionRow[] }

export type HoldingRow = G.orders_HoldingRow;
export type HoldingsSummary = G.orders_HoldingsSummary;
export interface HoldingsSnapshot extends Omit<G.orders_HoldingsSnapshot, 'rows'> { rows: HoldingRow[] }

export type RefreshScope = 'orders' | 'positions' | 'holdings' | 'all';

/** `order.update` payload: gokiteconnect `Order` (snake_case JSON). Only the fields the UI reads are typed. */
export type KiteOrder = Pick<
  G.kite_Order,
  | 'order_id' | 'status' | 'status_message' | 'exchange' | 'tradingsymbol' | 'instrument_token' | 'transaction_type'
  | 'product' | 'order_type' | 'quantity' | 'filled_quantity' | 'average_price' | 'tag'
>;

// --- Market data and status (service.TickDTO, service.FeedStateDTO, service.OffsetDTO, service.MarketStatus)
export type TickDTO = G.service_TickDTO;

export type FeedStateName = 'stopped' | 'connecting' | 'live' | 'reconnecting' | 'offline';
/** service.FeedStateDTO: `nextRetryMs` / `lastTickMs` are Unix ms, 0 = none. */
export interface FeedState extends Omit<G.service_FeedStateDTO, 'state'> { state: FeedStateName }

export type ClockSource = 'none' | 'ticks' | 'http-date';
/** service.OffsetDTO: `updatedAtMs` is Unix ms, 0 = never. */
export interface ClockOffset extends Omit<G.service_OffsetDTO, 'source'> { source: ClockSource }

export type MarketPhase = 'pre-open' | 'continuous' | 'closing' | 'post-closing' | 'closed';
export type DayType = 'trading' | 'holiday' | 'special' | 'weekend' | 'inferred-non-trading';
export interface MarketStatus extends Omit<G.service_MarketStatus, 'phase' | 'dayType'> { phase: MarketPhase; dayType: DayType }

export interface StatusSnapshot { feed: FeedState; market: MarketStatus; clock: ClockOffset }

// --- Settings (service.SettingValue, settings.Changed, settings.ShortcutAction)
export interface SettingValue { name: string; valueJson: string }
/** `value` is the decoded JSON value. */
export type SettingsChanged = G.settings_Changed;

export type ShortcutScope = 'global' | 'view' | 'chart' | 'builder';
export interface ShortcutAction { id: string; label: string; scope: ShortcutScope; defaultBinding: string; binding: string }

// --- Data, jobs, calendar, diagnostics, system
export interface DataStats {
  dataDir: string;
  appDbBytes: number;
  candleBytes: number;
  candleBudgetBytes: number;
  instrumentsImportedAtMs: Ms;
  backfillOpenJobs: number;
}

export interface BackupInfo {
  path: string;
  ok: boolean;
  reason: string;
  schema: number;
  supportedSchema: number;
  appVersion: string;
  createdAt: Rfc3339;
  /** Keys from P0-1 `(*store.DB).Counts`: strategies, scanners, backtest_runs, deployments, watchlists, order_journal. */
  counts: Record<string, number>;
  includesCandles: boolean;
  sizeBytes: number;
}

export type JobKind = 'backup' | 'restore' | 'support_bundle';
export interface JobProgress extends Omit<G.service_JobProgress, 'kind'> { kind: JobKind }

export type CalendarExchange = 'NSE' | 'BSE' | 'NFO' | 'BFO';
export type CalendarDayType = 'trading' | 'holiday' | 'special';
export type PhaseKind = 'pre-open' | 'continuous' | 'closing' | 'post-closing' | 'closed';
export interface PhaseDTO { kind: PhaseKind; start: string; end: string } // HH:MM IST
export interface CalendarEntryDTO { exchange: CalendarExchange; date: string; type: CalendarDayType; phases: PhaseDTO[]; source: string }
export interface CalendarEntryInput { exchange: CalendarExchange; date: string; type: CalendarDayType; phases: PhaseDTO[] }
export interface CalendarInfo { version: string; coversUntil: string; stale: boolean; entries: CalendarEntryDTO[] }

/** `path === ''` means no unseen crash report. */
export interface CrashInfo { path: string; atMs: Ms }
export interface Licence { name: string; version: string; licence: string }
export interface AppInfo { version: string; build: string; schemaVersion: number; dataDir: string; licences: Licence[] }

export type Navigate = G.service_Navigate;

// ---- P0-5 market domains (canonical contract §C/§D; JSON of the service.* models). Types that events.gen.ts carries
// are derived from it like the rest of this file; the others are checked against wailsjs/go/models in wails.ts.
/** `alsoOnNse` (FR-WATCH-008): a BSE row whose company is also listed on NSE; Go fills it on Search results only. */
export interface InstrumentDTO {
  token: number;
  exchange: string;
  segment: string;
  tag: string;
  tradingsymbol: string;
  name: string;
  displayName: string;
  instrumentType: string;
  expiryMs: Ms; // 0 = none
  tickSizePaise: number;
  lotSize: number;
  isIndex: boolean;
  isFuture: boolean;
  underlying: string;
  alsoOnNse: boolean;
}
export type CandleDTO = G.service_CandleDTO;
export type CandleEventDTO = G.service_CandleEventDTO;
export interface CandlesRequest { token: number; interval: string; fromMs: Ms; toMs: Ms; tradingDays: number }
export interface RefreshRequest { token: number; interval: string; fromMs: Ms; toMs: Ms }
export interface CandlesResult { candles: CandleDTO[]; complete: boolean; backfillJobId: string; needsDataPlan: boolean; offline: boolean }
export type BackfillState = 'running' | 'done' | 'failed';
/** service.BackfillProgressDTO: `etaMs` is the remaining time in ms. */
export interface BackfillProgress extends Omit<G.service_BackfillProgressDTO, 'state'> { state: BackfillState }
export type QuoteConsumer = 'chart' | 'watchlist' | 'widget';
export interface WatchResult { live: number[]; polled: number[]; snapshot: TickDTO[] }
export type WatchlistItem = G.service_WatchlistItem;
export interface Watchlist extends Omit<G.service_Watchlist, 'items'> { items: WatchlistItem[] }
export interface WatchlistsState extends Omit<G.service_WatchlistsState, 'lists'> { lists: Watchlist[] }
export interface RemovedItem { listId: string; token: number; position: number }

/** The subset of P0-3's `events.gen.ts` this plan subscribes to (P0-5 adds its chart/watchlist events). */
export interface EventMap {
  'session.state': SessionStatus;
  'feed.state': FeedState;
  'clock.offset': ClockOffset;
  'market.status': MarketStatus;
  'tick.batch': TickDTO[];
  'order.update': KiteOrder;
  'orders.changed': BookSnapshot;
  'positions.changed': PositionsSnapshot;
  'holdings.changed': HoldingsSnapshot;
  'job.progress': JobProgress;
  'settings.changed': SettingsChanged;
  'app.navigate': Navigate;
  'candle.forming': CandleEventDTO[];
  'candle.closed': CandleEventDTO;
  'backfill.progress': BackfillProgress[];
  /** Go publishes the whole new state after every watchlist mutation (incl. SetActive) and after a restore. */
  'watchlists.changed': WatchlistsState;
}
export type EventName = keyof EventMap;

// Compile-time guard: every EventMap entry is a generated event whose payload our type satisfies (narrowed mirror).
// Only `order.update` may instead be a subset of the generated payload (KiteOrder types the fields the UI reads).
type Assert<T extends true> = T;
type SubsetEvents = 'order.update';
type Compatible<K extends EventName, Ours, Gen> = [Ours] extends [Gen]
  ? true
  : K extends SubsetEvents ? ([Gen] extends [Ours] ? true : false) : false;
export type EventMapMatchesGenerated = Assert<
  { [K in EventName]: K extends G.EventName ? Compatible<K, EventMap[K], G.EventPayloads[K]> : false }[EventName]
>;

export interface Backend {
  session: {
    status(): Promise<SessionStatus>;
    credentials(): Promise<CredentialsInfo>;
    saveCredentials(input: CredentialsInput): Promise<SessionStatus>;
    removeCredentials(): Promise<SessionStatus>;
    setRedirectPort(port: number): Promise<CredentialsInfo>;
    login(): Promise<LoginStart>;
    completeLogin(pasted: string): Promise<SessionStatus>;
    cancelLogin(): Promise<SessionStatus>;
    logout(): Promise<LogoutResult>;
    ackRiskDisclosure(): Promise<SessionStatus>;
    profile(): Promise<Profile>;
    secretStorage(): Promise<SecretStorageInfo>;
    useEncryptedFile(passphrase: string): Promise<SessionStatus>;
    refuseSecretStorage(): Promise<SessionStatus>;
    unlock(passphrase: string): Promise<SessionStatus>;
    migrateToKeyring(): Promise<SecretStorageInfo>;
    reconnectFeed(): Promise<void>;
  };
  portfolio: {
    orderBook(): Promise<BookSnapshot>;
    positions(): Promise<PositionsSnapshot>;
    holdings(): Promise<HoldingsSnapshot>;
    refresh(scope: RefreshScope): Promise<void>;
  };
  settings: {
    all(): Promise<SettingValue[]>;
    set(name: string, valueJSON: string): Promise<SettingValue>;
    shortcuts(): Promise<ShortcutAction[]>;
    rebind(actionId: string, binding: string): Promise<ShortcutAction>;
    resetShortcuts(): Promise<ShortcutAction[]>;
    getUiState(key: string): Promise<string>;
    setUiState(key: string, value: string): Promise<void>;
  };
  data: {
    stats(): Promise<DataStats>;
    clearCandles(): Promise<void>;
    refreshInstruments(): Promise<void>;
    chooseFolder(): Promise<string>;
    chooseBackupFile(): Promise<string>;
    backupNow(folder: string, includeCandles: boolean): Promise<string>;
    inspectBackup(path: string): Promise<BackupInfo>;
    restore(path: string, confirmation: string): Promise<string>;
  };
  calendar: {
    info(): Promise<CalendarInfo>;
    chooseImportFile(): Promise<string>;
    import(path: string): Promise<CalendarInfo>;
    upsert(entry: CalendarEntryInput): Promise<CalendarInfo>;
    remove(exchange: string, date: string): Promise<CalendarInfo>;
  };
  diagnostics: {
    openLogsFolder(): Promise<void>;
    createSupportBundle(): Promise<string>;
    lastCrash(): Promise<CrashInfo>;
    openCrashReport(): Promise<void>;
    dismissCrash(): Promise<void>;
  };
  system: {
    info(): Promise<AppInfo>;
    status(): Promise<StatusSnapshot>;
  };
  chart: {
    candles(req: CandlesRequest): Promise<CandlesResult>;
    refresh(req: RefreshRequest): Promise<CandlesResult>;
    openLive(token: number, interval: string): Promise<string>;
    closeLive(id: string): Promise<void>;
  };
  instruments: {
    search(q: string, tags: string[], limit: number): Promise<InstrumentDTO[]>;
    byTokens(tokens: number[]): Promise<InstrumentDTO[]>;
    byIdentifier(id: string): Promise<InstrumentDTO>;
  };
  quotes: { watch(consumer: QuoteConsumer, tokens: number[]): Promise<WatchResult> };
  watchlists: {
    list(): Promise<WatchlistsState>;
    create(name: string): Promise<WatchlistsState>;
    rename(id: string, name: string): Promise<WatchlistsState>;
    delete(id: string): Promise<WatchlistsState>;
    setActive(id: string): Promise<WatchlistsState>;
    add(id: string, token: number, position: number): Promise<WatchlistsState>;
    remove(id: string, token: number): Promise<RemovedItem>;
    reorder(id: string, tokens: number[]): Promise<WatchlistsState>;
    reorderLists(ids: string[]): Promise<WatchlistsState>;
  };
  on<K extends EventName>(name: K, cb: (payload: EventMap[K]) => void): () => void;
}
