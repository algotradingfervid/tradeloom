// The only module that imports the generated Wails bindings (ARCHITECTURE §8.1).
import * as Calendar from '../../wailsjs/go/api/Calendar';
import * as Data from '../../wailsjs/go/api/Data';
import * as Diagnostics from '../../wailsjs/go/api/Diagnostics';
import * as Portfolio from '../../wailsjs/go/api/Portfolio';
import * as Session from '../../wailsjs/go/api/Session';
import * as Settings from '../../wailsjs/go/api/Settings';
import * as System from '../../wailsjs/go/api/System';
import { kite, orders, service, settings } from '../../wailsjs/go/models';
import { EventsOn } from '../../wailsjs/runtime/runtime';
import type {
  AppInfo, Backend, BackupInfo, BookSnapshot, CalendarEntryDTO, CalendarEntryInput, CalendarInfo, ClockOffset, CrashInfo,
  CredentialsInfo, CredentialsInput, DataStats, EventMap, EventName, FeedState, HoldingRow, HoldingsSnapshot, HoldingsSummary,
  Licence, LoginStart, LogoutResult, MarketStatus, OrderRow, PhaseDTO, PositionRow, PositionsSnapshot, Profile,
  SecretStorageInfo, SectionCounts, SessionStatus, SettingValue, ShortcutAction, StatusSnapshot,
} from './backend';

// The generated functions return the Wails model classes (service.*, kite.*, orders.*, settings.*); their JSON shape
// is the interfaces in backend.ts (checked below), so the adapter narrows the static type and replaces the JSON
// `null` that Go sends for nil slices and maps with [] / {} (Ruling 80).
const cast = <T,>(p: Promise<unknown>): Promise<T> => p as Promise<T>;
const arr = <T,>(v: T[] | null | undefined): T[] => v ?? [];

const normSecrets = (s: SecretStorageInfo): SecretStorageInfo => (s && s.choices == null ? { ...s, choices: [] } : s);
const normSession = (s: SessionStatus): SessionStatus => (s && s.secrets ? { ...s, secrets: normSecrets(s.secrets) } : s);
const normProfile = (p: Profile): Profile => ({ ...p, exchanges: arr(p.exchanges), products: arr(p.products), orderTypes: arr(p.orderTypes) });
const normBook = (b: BookSnapshot): BookSnapshot => ({ ...b, orders: arr(b.orders) });
const normPositions = (p: PositionsSnapshot): PositionsSnapshot => ({ ...p, net: arr(p.net), day: arr(p.day) });
const normHoldings = (h: HoldingsSnapshot): HoldingsSnapshot => ({ ...h, rows: arr(h.rows) });
const normEntry = (e: CalendarEntryDTO): CalendarEntryDTO => ({ ...e, phases: arr(e.phases) });
const normCalendar = (c: CalendarInfo): CalendarInfo => ({ ...c, entries: arr(c.entries).map(normEntry) });
const normBackup = (b: BackupInfo): BackupInfo => ({ ...b, counts: b.counts ?? {} });
const normInfo = (i: AppInfo): AppInfo => ({ ...i, licences: arr(i.licences) });

const EVENT_NORMALISERS: { [K in EventName]?: (p: EventMap[K]) => EventMap[K] } = {
  'session.state': normSession,
  'tick.batch': arr,
  'orders.changed': normBook,
  'positions.changed': normPositions,
  'holdings.changed': normHoldings,
};

export function createWailsBackend(): Backend {
  return {
    session: {
      status: () => cast<SessionStatus>(Session.Status()).then(normSession),
      credentials: () => cast(Session.Credentials()),
      saveCredentials: (input) => cast(Session.SaveCredentials(service.CredentialsInput.createFrom(input))),
      removeCredentials: () => cast<SessionStatus>(Session.RemoveCredentials()).then(normSession),
      setRedirectPort: (port) => cast(Session.SetRedirectPort(port)),
      login: () => cast(Session.Login()),
      completeLogin: (pasted) => cast<SessionStatus>(Session.CompleteLogin(pasted)).then(normSession),
      cancelLogin: () => cast<SessionStatus>(Session.CancelLogin()).then(normSession),
      logout: () => cast(Session.Logout()),
      ackRiskDisclosure: () => cast<SessionStatus>(Session.AckRiskDisclosure()).then(normSession),
      profile: () => cast<Profile>(Session.Profile()).then(normProfile),
      secretStorage: () => cast<SecretStorageInfo>(Session.SecretStorage()).then(normSecrets),
      useEncryptedFile: (passphrase) => cast<SessionStatus>(Session.UseEncryptedFile(passphrase)).then(normSession),
      refuseSecretStorage: () => cast<SessionStatus>(Session.RefuseSecretStorage()).then(normSession),
      unlock: (passphrase) => cast<SessionStatus>(Session.Unlock(passphrase)).then(normSession),
      migrateToKeyring: () => cast<SecretStorageInfo>(Session.MigrateToKeyring()).then(normSecrets),
      reconnectFeed: () => Session.ReconnectFeed(),
    },
    portfolio: {
      orderBook: () => cast<BookSnapshot>(Portfolio.OrderBook()).then(normBook),
      positions: () => cast<PositionsSnapshot>(Portfolio.Positions()).then(normPositions),
      holdings: () => cast<HoldingsSnapshot>(Portfolio.Holdings()).then(normHoldings),
      refresh: (scope) => Portfolio.Refresh(scope),
    },
    settings: {
      all: () => cast<SettingValue[]>(Settings.All()).then(arr),
      set: (name, valueJSON) => cast(Settings.Set(name, valueJSON)),
      shortcuts: () => cast<ShortcutAction[]>(Settings.Shortcuts()).then(arr),
      rebind: (actionId, binding) => cast(Settings.Rebind(actionId, binding)),
      resetShortcuts: () => cast<ShortcutAction[]>(Settings.ResetShortcuts()).then(arr),
      getUiState: (key) => Settings.GetUiState(key),
      setUiState: (key, value) => Settings.SetUiState(key, value),
    },
    data: {
      stats: () => cast(Data.Stats()),
      clearCandles: () => Data.ClearCandles(),
      refreshInstruments: () => Data.RefreshInstruments(),
      chooseFolder: () => Data.ChooseFolder(),
      chooseBackupFile: () => Data.ChooseBackupFile(),
      backupNow: (folder, includeCandles) => Data.BackupNow(folder, includeCandles),
      inspectBackup: (path) => cast<BackupInfo>(Data.InspectBackup(path)).then(normBackup),
      restore: (path, confirmation) => Data.Restore(path, confirmation),
    },
    calendar: {
      info: () => cast<CalendarInfo>(Calendar.Info()).then(normCalendar),
      chooseImportFile: () => Calendar.ChooseImportFile(),
      import: (path) => cast<CalendarInfo>(Calendar.Import(path)).then(normCalendar),
      upsert: (entry) => cast<CalendarInfo>(Calendar.Upsert(service.CalendarEntryInput.createFrom(entry))).then(normCalendar),
      remove: (exchange, date) => cast<CalendarInfo>(Calendar.Remove(exchange, date)).then(normCalendar),
    },
    diagnostics: {
      openLogsFolder: () => Diagnostics.OpenLogsFolder(),
      createSupportBundle: () => Diagnostics.CreateSupportBundle(),
      lastCrash: () => cast(Diagnostics.LastCrash()),
      openCrashReport: () => Diagnostics.OpenCrashReport(),
      dismissCrash: () => Diagnostics.DismissCrash(),
    },
    system: {
      info: () => cast<AppInfo>(System.Info()).then(normInfo),
      status: () => cast(System.Status()),
    },
    on: (name, cb) => {
      const normalise = EVENT_NORMALISERS[name];
      const handler = normalise ? (p: EventMap[typeof name]) => cb(normalise(p)) : cb;
      return EventsOn(name, handler as (...data: unknown[]) => void);
    },
  };
}

// --- Compile-time contract: backend.ts mirrors every generated model field-for-field (same keys, compatible types;
// backend.ts may only narrow Go strings to unions). A field added, removed or retyped in Go fails `tsc`.
type Data<T> = T extends (infer U)[]
  ? Data<U>[]
  : T extends object
    ? { [K in keyof T as T[K] extends (...a: never[]) => unknown ? never : K]: Data<T[K]> }
    : T;
type Mirrors<Ours, Model> = [Ours] extends [Data<Model>]
  ? [Exclude<keyof Ours, keyof Data<Model>> | Exclude<keyof Data<Model>, keyof Ours>] extends [never] ? true : false
  : false;
type Assert<T extends true> = T;
export type ModelContract = [
  Assert<Mirrors<CredentialsInfo, kite.CredentialsInfo>>,
  Assert<Mirrors<LoginStart, kite.LoginStart>>,
  Assert<Mirrors<LogoutResult, kite.LogoutResult>>,
  Assert<Mirrors<Profile, kite.Profile>>,
  Assert<Mirrors<SectionCounts, orders.SectionCounts>>,
  Assert<Mirrors<OrderRow, orders.OrderRow>>,
  Assert<Mirrors<BookSnapshot, orders.BookSnapshot>>,
  Assert<Mirrors<PositionRow, orders.PositionRow>>,
  Assert<Mirrors<PositionsSnapshot, orders.PositionsSnapshot>>,
  Assert<Mirrors<HoldingRow, orders.HoldingRow>>,
  Assert<Mirrors<HoldingsSummary, orders.HoldingsSummary>>,
  Assert<Mirrors<HoldingsSnapshot, orders.HoldingsSnapshot>>,
  Assert<Mirrors<SecretStorageInfo, service.SecretStorageInfo>>,
  Assert<Mirrors<CredentialsInput, service.CredentialsInput>>,
  Assert<Mirrors<SessionStatus, service.SessionStatus>>,
  Assert<Mirrors<FeedState, service.FeedStateDTO>>,
  Assert<Mirrors<ClockOffset, service.OffsetDTO>>,
  Assert<Mirrors<MarketStatus, service.MarketStatus>>,
  Assert<Mirrors<StatusSnapshot, service.StatusSnapshot>>,
  Assert<Mirrors<SettingValue, service.SettingValue>>,
  Assert<Mirrors<ShortcutAction, settings.ShortcutAction>>,
  Assert<Mirrors<DataStats, service.DataStats>>,
  Assert<Mirrors<BackupInfo, service.BackupInfo>>,
  Assert<Mirrors<PhaseDTO, service.PhaseDTO>>,
  Assert<Mirrors<CalendarEntryDTO, service.CalendarEntryDTO>>,
  Assert<Mirrors<CalendarEntryInput, service.CalendarEntryInput>>,
  Assert<Mirrors<CalendarInfo, service.CalendarInfo>>,
  Assert<Mirrors<CrashInfo, service.CrashInfo>>,
  Assert<Mirrors<Licence, service.Licence>>,
  Assert<Mirrors<AppInfo, service.AppInfo>>,
];
