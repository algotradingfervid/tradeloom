import type {
  AppInfo, Backend, BackupInfo, BookSnapshot, CalendarEntryInput, CalendarInfo, CrashInfo, CredentialsInput, DataStats,
  EventMap, EventName, HoldingsSnapshot, JobProgress, PositionsSnapshot, Profile, RefreshScope, SessionStatus, ShortcutAction,
  ShortcutScope, StatusSnapshot,
} from '../backend';
import { CalendarRefusal, checkEntry, mergeOverrides, parseCalendarCsv } from './calendar';
import { next0600Ist } from './fixtures';
import { createMarketDomains, type MarketMockState } from './market';

export interface MockState extends MarketMockState {
  session: SessionStatus;
  status: StatusSnapshot;
  profile: Profile;
  book: BookSnapshot;
  positions: PositionsSnapshot;
  holdings: HoldingsSnapshot;
  /** name → JSON-encoded value, as `Settings.All` reports it. */
  settings: Record<string, string>;
  uiState: Record<string, string>;
  shortcuts: ShortcutAction[];
  stats: DataStats;
  backup: BackupInfo;
  /** Go lists overrides only (user, import, inferred) — never the bundled days. */
  calendar: CalendarInfo;
  /** Contents of the CSV `Calendar.Import` reads (whatever path was chosen); undefined = file not found. */
  calendarCsv?: string;
  info: AppInfo;
  crash: CrashInfo;
  folderChoice: string;
  fileChoice: string;
  encPassphrase: string;
  /** false simulates a busy redirect port: `Login()` reports `listenerOk: false`. */
  loginListenerOk: boolean;
}

export interface MockOptions { persistKey?: string; autoJobs?: boolean; jobDelayMs?: number }
export interface MockCall { method: string; args: unknown[] }

export interface MockBackend extends Backend {
  readonly state: MockState;
  readonly calls: MockCall[];
  emit<K extends EventName>(name: K, payload: EventMap[K]): void;
  failNext(method: string, reason: string, code?: string): void;
  listenerCount(name: EventName): number;
  callsTo(method: string): MockCall[];
  /** The browser redirect reaching the loopback listener (FR-ACCT-006 AC1). */
  simulateBrowserLogin(): void;
}

const copy = <T,>(v: T): T => JSON.parse(JSON.stringify(v)) as T;

function refusal(code: string, stage: string, reason: string): Error {
  return new Error(JSON.stringify({ code, stage, reason }));
}

/** Go kite requestTokenRe (internal/kite/redirect.go). */
const REQUEST_TOKEN = /^[A-Za-z0-9]{8,128}$/;

/** Runs a calendar operation, turning a `CalendarRefusal` into Go's `{code, stage: "calendar", reason}` rejection. */
function goLike<T>(fn: () => T): T {
  try {
    return fn();
  } catch (e) {
    if (e instanceof CalendarRefusal) throw refusal(e.code, 'calendar', e.reason);
    throw e;
  }
}

const TIMEFRAMES = ['minute', '3minute', '5minute', '10minute', '15minute', '30minute', '60minute', 'day'];

// The Go validators of contract §E that the UI can trip.
const VALIDATORS: Record<string, (v: unknown) => string | null> = {
  'backup.schedule.keep': (v) => (Number.isInteger(v) && (v as number) >= 1 && (v as number) <= 365 ? null : 'Keep between 1 and 365 backups'),
  'log.level': (v) => (v === 'info' || v === 'debug' ? null : 'Log level must be info or debug'),
  'chart.default_timeframe': (v) => (typeof v === 'string' && TIMEFRAMES.includes(v) ? null : 'Unknown timeframe'),
};

const CRED_RE = /^[A-Za-z0-9]{6,64}$/;
const normalizeCredential = (v: string): string => v.trim().replace(/^["'`]+|["'`]+$/g, '').trim();

/** Go kite.CodeLoginBusy (internal/kite/session_types.go). */
const LOGIN_BUSY = 'login_in_progress';
const INVALID_PORT = 'Choose a port between 1024 and 65535. Then set the redirect URL of your app in the Kite developer console to http://127.0.0.1:<port>/kite/callback with that port.';

const overlaps = (a: ShortcutScope, b: ShortcutScope) => a === b || a === 'global' || b === 'global';

type Fns = Record<string, (...args: never[]) => unknown>;
type Wrapped<T extends Fns> = { [K in keyof T]: (...a: Parameters<T[K]>) => Promise<Awaited<ReturnType<T[K]>>> };

export function createMockBackend(seed: MockState, opts: MockOptions = {}): MockBackend {
  const state = copy(seed);
  const calls: MockCall[] = [];
  const failures = new Map<string, Error>();
  const listeners = new Map<EventName, Set<(p: unknown) => void>>();
  let jobSeq = 0;
  let loginSeq = 1;

  if (opts.persistKey) {
    try {
      const stored = window.localStorage.getItem(opts.persistKey);
      if (stored) state.settings = { ...state.settings, ...(JSON.parse(stored) as Record<string, string>) };
    } catch {
      // storage unavailable: start from the seed
    }
  }
  const persist = () => {
    if (!opts.persistKey) return;
    try {
      window.localStorage.setItem(opts.persistKey, JSON.stringify(state.settings));
    } catch {
      // ignore
    }
  };

  const emit = <K extends EventName>(name: K, payload: EventMap[K]) => {
    for (const cb of [...(listeners.get(name) ?? [])]) cb(copy(payload));
  };
  const setSession = (patch: Partial<SessionStatus>): SessionStatus => {
    state.session = { ...state.session, ...patch };
    emit('session.state', state.session);
    return copy(state.session);
  };
  const setFeed = (patch: Partial<StatusSnapshot['feed']>) => {
    state.status.feed = { ...state.status.feed, ...patch };
    emit('feed.state', state.status.feed);
  };

  const wrap = <T extends Fns>(domain: string, impl: T): Wrapped<T> => {
    const out: Record<string, (...a: unknown[]) => Promise<unknown>> = {};
    for (const name of Object.keys(impl)) {
      out[name] = async (...args: unknown[]) => {
        const method = `${domain}.${name}`;
        calls.push({ method, args });
        const failure = failures.get(method);
        if (failure) {
          failures.delete(method);
          throw failure;
        }
        return (impl[name] as (...a: unknown[]) => unknown)(...args);
      };
    }
    return out as Wrapped<T>;
  };

  const loginSucceeded = (): SessionStatus => {
    const now = Date.now();
    loginSeq += 1;
    const next = setSession({
      state: 'ready', sessionId: `sess-${loginSeq}`, riskAckRequired: true, loggedInAtMs: now, expiresAtMs: next0600Ist(now),
      userId: state.profile.userId, errorCode: '', errorMessage: '',
    });
    if (state.session.dataPlan !== 'none') setFeed({ state: 'live', attempt: 0, lastTickMs: now });
    return next;
  };

  const finishJob = (jobId: string, kind: JobProgress['kind'], stage: string, resultPath: string) => {
    if (!opts.autoJobs) return;
    window.setTimeout(() => emit('job.progress', { jobId, kind, stage, done: 1, total: 1, finished: true, error: '', resultPath }), opts.jobDelayMs ?? 50);
  };

  const publishPortfolio = (scope: RefreshScope) => {
    const asOfMs = Date.now();
    if (scope === 'orders' || scope === 'all') emit('orders.changed', (state.book = { ...state.book, asOfMs, stale: false }));
    if (scope === 'positions' || scope === 'all') emit('positions.changed', (state.positions = { ...state.positions, asOfMs, stale: false }));
    if (scope === 'holdings' || scope === 'all') emit('holdings.changed', (state.holdings = { ...state.holdings, asOfMs, stale: false }));
  };

  const market = createMarketDomains(state, emit);
  const backend: MockBackend = {
    state,
    calls,
    emit,
    simulateBrowserLogin: () => void loginSucceeded(),
    failNext: (method, reason, code = 'error') => {
      failures.set(method, code === 'error' ? new Error(reason) : refusal(code, method.split('.')[0], reason));
    },
    listenerCount: (name) => listeners.get(name)?.size ?? 0,
    callsTo: (method) => calls.filter((c) => c.method === method),
    on: (name, cb) => {
      const set = listeners.get(name) ?? new Set();
      const fn = cb as (p: unknown) => void;
      set.add(fn);
      listeners.set(name, set);
      return () => {
        set.delete(fn);
      };
    },
    session: wrap('session', {
      status: () => copy(state.session),
      credentials: () => copy(state.session.credentials),
      saveCredentials: (input: CredentialsInput) => {
        // As Go kite.NormalizeCredential + ValidateAPIKey / ValidateAPISecret (internal/kite/credentials.go).
        const key = normalizeCredential(input.apiKey);
        const secret = normalizeCredential(input.apiSecret);
        if (!CRED_RE.test(key)) throw refusal('invalid_credentials', 'session', 'The API key should be 6–64 letters and digits. Copy it from your app in the Kite developer console.');
        if (!CRED_RE.test(secret)) throw refusal('invalid_credentials', 'session', 'The API secret should be 6–64 letters and digits. Copy it from your app in the Kite developer console.');
        if (key === secret) {
          throw refusal('invalid_credentials', 'session', 'The API key and the API secret are identical. Paste each from its own field in the Kite developer console.');
        }
        // As Go kite.Session.SaveCredentials: refused while a login runs or a session is live.
        if (state.session.state === 'logging_in') throw refusal(LOGIN_BUSY, 'session', 'Finish or cancel the current login before changing credentials.');
        if (state.session.state === 'ready') throw refusal('already_logged_in', 'session', 'Log out before changing the Kite credentials.');
        if (state.session.secrets.mode === 'choose' || state.session.secrets.mode === 'none') {
          throw refusal('secret_store', 'session', 'Choose where to store the secret first');
        }
        return setSession({
          state: 'logged_out',
          credentials: { ...state.session.credentials, configured: true, apiKeyMasked: '••••' + key.slice(-4), secretBackend: state.session.secrets.backend },
        });
      },
      removeCredentials: () => setSession({ state: 'no_credentials', riskAckRequired: false, credentials: { ...state.session.credentials, configured: false, apiKeyMasked: '' } }),
      setRedirectPort: (port: number) => {
        // As Go kite.Session.SetRedirectPort (texts verbatim); Go publishes no session.state for a port change.
        if (!Number.isInteger(port) || port < 1024 || port > 65535) throw refusal('invalid_port', 'session', INVALID_PORT);
        const redirectUrl = `http://127.0.0.1:${port}/kite/callback`;
        if (state.session.state === 'logging_in') {
          throw refusal(LOGIN_BUSY, 'session', `Finish or cancel the current login before changing the port. Then set the redirect URL of your app in the Kite developer console to ${redirectUrl}.`);
        }
        state.session.credentials = { ...state.session.credentials, port, redirectUrl };
        return copy(state.session.credentials);
      },
      login: () => {
        const ok = state.loginListenerOk;
        setSession({ state: 'logging_in' });
        return {
          loginUrl: 'https://kite.zerodha.com/connect/login?v=3&api_key=kiteapikey9f2c',
          redirectUrl: state.session.credentials.redirectUrl,
          listenerOk: ok,
          listenerError: ok ? '' : `Port ${state.session.credentials.port} is already in use by another program`,
        };
      },
      // As Go kite.Session.CompleteLogin → ParseRedirect (internal/kite/redirect.go): same accept rule and texts; a
      // parse refusal publishes no session.state. The refusal never echoes the pasted text.
      completeLogin: (pasted: string) => {
        const refuse = (reason: string) => refusal('bad_redirect', 'login', reason);
        if (pasted.length > 4096) throw refuse("That text is too long to be a login address. Copy only the address from the browser's address bar.");
        const v = pasted.trim().replace(/^["'<>]+|["'<>]+$/g, '');
        if (!v) throw refuse('Paste the full address from the browser after logging in, or just the request_token.');
        if (REQUEST_TOKEN.test(v)) return loginSucceeded();
        const scheme = /^[A-Za-z][A-Za-z0-9+.-]*:\/\//.exec(v)?.[0].toLowerCase();
        if (scheme && scheme !== 'http://' && scheme !== 'https://') throw refuse('That address is not a web address. Copy the whole address bar after logging in.');
        const query = new URLSearchParams(v.includes('?') ? v.slice(v.indexOf('?') + 1).split('#')[0] : v.split('#')[0]);
        const status = query.get('status') ?? '';
        if (status !== '' && status !== 'success') {
          throw refusal('login_cancelled', 'login', 'Kite reports the login was not completed. Click "Log in with Kite" to try again.');
        }
        if (!REQUEST_TOKEN.test((query.get('request_token') ?? '').trim())) {
          throw refuse("No request_token found in the pasted text. After logging in, copy the whole address from the browser's address bar.");
        }
        return loginSucceeded();
      },
      cancelLogin: () => (state.session.state === 'logging_in' ? setSession({ state: 'logged_out' }) : copy(state.session)),
      logout: () => {
        setSession({ state: 'logged_out', riskAckRequired: false, sessionId: '' });
        setFeed({ state: 'stopped', attempt: 0, connections: 0, tokens: 0 });
        return { remoteInvalidated: true, message: '' };
      },
      ackRiskDisclosure: () => setSession({ riskAckRequired: false }),
      profile: () => copy(state.profile),
      secretStorage: () => copy(state.session.secrets),
      useEncryptedFile: (passphrase: string) => {
        if ([...passphrase].length < 8) throw refusal('passphrase_too_short', 'secrets', 'Use at least 8 characters');
        state.encPassphrase = passphrase;
        return setSession({
          secrets: { mode: 'file', backend: 'file', keyringAvailable: state.session.secrets.keyringAvailable, fileExists: true, choices: [] },
          credentials: { ...state.session.credentials, secretBackend: 'file' },
        });
      },
      // As Go service.RefuseSecretStorage: mode "none" with no choices, and no session.state is published.
      refuseSecretStorage: () => {
        state.session = { ...state.session, secrets: { ...state.session.secrets, mode: 'none', backend: 'none', choices: [] } };
        return copy(state.session);
      },
      unlock: (passphrase: string) => {
        if (passphrase !== state.encPassphrase) throw refusal('bad_passphrase', 'secrets', 'Unlock failed: wrong passphrase or damaged file. Check the passphrase and try again.'); // Go msgBadPassphrase
        return setSession({
          state: state.session.credentials.configured ? 'logged_out' : 'no_credentials',
          secrets: { ...state.session.secrets, backend: 'file' },
        });
      },
      migrateToKeyring: () => {
        // As Go service.MigrateToKeyring: no session.state is published; the caller re-reads Credentials().
        if (!state.session.secrets.keyringAvailable) throw refusal('keyring_unavailable', 'secrets', 'No system keyring is available. Unlock or start it, then try again.');
        const secrets = { mode: 'keyring' as const, backend: 'keyring' as const, keyringAvailable: true, fileExists: false, choices: [] };
        state.session = { ...state.session, secrets, credentials: { ...state.session.credentials, secretBackend: 'keyring' } };
        return copy(secrets);
      },
      reconnectFeed: () => undefined,
    }),
    portfolio: wrap('portfolio', {
      orderBook: () => copy(state.book),
      positions: () => copy(state.positions),
      holdings: () => copy(state.holdings),
      refresh: (scope: RefreshScope) => publishPortfolio(scope),
    }),
    settings: wrap('settings', {
      all: () => Object.entries(state.settings).filter(([name]) => !name.startsWith('kite.')).map(([name, valueJson]) => ({ name, valueJson })),
      set: (name: string, valueJSON: string) => {
        if (name.startsWith('kite.')) throw refusal('use_credentials_flow', 'settings', 'Change Kite settings in Settings → Kite connection');
        const value: unknown = JSON.parse(valueJSON);
        const reason = VALIDATORS[name]?.(value);
        if (reason) throw refusal('invalid', 'settings', reason);
        state.settings[name] = valueJSON;
        persist();
        emit('settings.changed', { key: name, value });
        return { name, valueJson: valueJSON };
      },
      shortcuts: () => copy(state.shortcuts),
      rebind: (actionId: string, binding: string) => {
        const target = state.shortcuts.find((a) => a.id === actionId);
        if (!target) throw refusal('invalid', 'shortcuts', `Unknown action ${actionId}`);
        // As in Go: an empty binding (unbound action) never conflicts.
        const clash = binding === '' ? undefined : state.shortcuts.find((a) => a.id !== actionId && a.binding === binding && overlaps(a.scope, target.scope));
        if (clash) throw refusal('conflict', 'shortcuts', clash.label);
        target.binding = binding;
        // As in Go (service.publishShortcuts): the full list on settings.changed key "shortcuts".
        emit('settings.changed', { key: 'shortcuts', value: state.shortcuts });
        return copy(target);
      },
      resetShortcuts: () => {
        for (const a of state.shortcuts) a.binding = a.defaultBinding;
        emit('settings.changed', { key: 'shortcuts', value: state.shortcuts });
        return copy(state.shortcuts);
      },
      getUiState: (key: string) => state.uiState[key] ?? '',
      setUiState: (key: string, value: string) => {
        state.uiState[key] = value;
      },
    }),
    data: wrap('data', {
      stats: () => copy(state.stats),
      clearCandles: () => {
        state.stats.candleBytes = 0;
      },
      refreshInstruments: () => {
        state.stats.instrumentsImportedAtMs = Date.now();
      },
      chooseFolder: () => state.folderChoice,
      chooseBackupFile: () => state.fileChoice,
      backupNow: (folder: string, _includeCandles: boolean) => {
        const id = `job-${++jobSeq}`;
        finishJob(id, 'backup', 'done', `${folder}/tradeloom-backup-20261012-103000.tar.zst`);
        return id;
      },
      inspectBackup: (path: string) => ({ ...copy(state.backup), path }),
      restore: (_path: string, confirmation: string) => {
        if (confirmation !== 'RESTORE') throw refusal('not_confirmed', 'restore', 'Type RESTORE to confirm');
        const id = `job-${++jobSeq}`;
        finishJob(id, 'restore', 'restart', '');
        return id;
      },
    }),
    calendar: wrap('calendar', {
      // Go-like (Ruling 100 M4): overrides only, Go's refusal codes and reasons (./calendar.ts).
      info: () => copy(state.calendar),
      chooseImportFile: () => state.fileChoice,
      import: (path: string) => goLike(() => {
        if (!path || state.calendarCsv === undefined) throw new CalendarRefusal('not_found', 'The calendar file was not found.');
        mergeOverrides(state.calendar, parseCalendarCsv(state.calendarCsv));
        return copy(state.calendar);
      }),
      upsert: (entry: CalendarEntryInput) => goLike(() => {
        mergeOverrides(state.calendar, [checkEntry(entry)]);
        return copy(state.calendar);
      }),
      remove: (exchange: string, date: string) => goLike(() => {
        if (!state.calendar.entries.some((e) => e.exchange === exchange && e.date === date)) {
          throw new CalendarRefusal('not_found', `No override for ${exchange} on ${date}.`);
        }
        state.calendar.entries = state.calendar.entries.filter((e) => !(e.exchange === exchange && e.date === date));
        return copy(state.calendar);
      }),
    }),
    diagnostics: wrap('diagnostics', {
      openLogsFolder: () => undefined,
      createSupportBundle: () => '/home/asha/.local/share/tradeloom/exports/support-20261012-103000.zip',
      lastCrash: () => copy(state.crash),
      openCrashReport: () => undefined,
      dismissCrash: () => {
        state.crash = { path: '', atMs: 0 };
      },
    }),
    system: wrap('system', {
      info: () => copy(state.info),
      status: () => copy(state.status),
    }),
    chart: wrap('chart', market.chart),
    instruments: wrap('instruments', market.instruments),
    quotes: wrap('quotes', market.quotes),
    watchlists: wrap('watchlists', market.watchlists),
  };
  return backend;
}
