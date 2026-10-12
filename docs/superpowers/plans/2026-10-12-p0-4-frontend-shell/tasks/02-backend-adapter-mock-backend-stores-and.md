> **P0-4 Frontend Shell, Account & Data UI Implementation Plan — Task 2.** Read [`../MANIFEST.md`](../MANIFEST.md) first: its Global Constraints,
> Review Focus and Consumes/Produces apply to this task. Other tasks live beside this file in `tasks/`.

<!-- task body (verbatim from the plan) -->
### Task 2: Backend adapter, mock backend, stores and event bridge

**Files:**
- Create: `frontend/src/api/backend.ts`, `frontend/src/api/errors.ts`, `frontend/src/api/wails.ts`, `frontend/src/api/BackendContext.tsx`, `frontend/src/api/events.ts`, `frontend/src/api/index.ts`, `frontend/src/api/mock/index.ts`, `frontend/src/api/mock/fixtures.ts`
- Create: `frontend/src/store/reset.ts`, `frontend/src/store/session.ts`, `frontend/src/store/status.ts`, `frontend/src/store/ticks.ts`, `frontend/src/store/settings.ts`, `frontend/src/store/app.ts`
- Create: `frontend/src/test/render.tsx`
- Modify: `frontend/src/test/setup.ts`, `frontend/src/app/App.tsx`, `frontend/src/app/RootLayout.tsx`, `frontend/src/app/ShellLayout.tsx`, `frontend/src/main.tsx`, `frontend/src/i18n/copy.ts`, `frontend/src/app/AppShell.test.tsx`
- Test: `frontend/src/api/errors.test.ts`, `frontend/src/api/wails.test.ts`, `frontend/src/api/mock/mock.test.ts`, `frontend/src/api/events.test.ts`, `frontend/src/store/settings.test.tsx`, `frontend/src/store/ticks.test.ts`, `frontend/src/app/persistence.test.tsx`

**Interfaces:**
- Consumes: Task 1 `ThemeProvider`, `createAppRouter`, `AppRouter`, `ShellLayout`, `Sidebar`; P0-3 generated `frontend/wailsjs/go/api/{Session,Portfolio,Settings,Data,Calendar,Diagnostics,System}` and `frontend/wailsjs/runtime/runtime` `EventsOn` (tables in "Consumes (from P0-3)"); `frontend/wailsjs/go/models` `service.CredentialsInput.createFrom` and `service.CalendarEntryInput.createFrom` (Wails v2.16 `typescriptify` emits `static createFrom(source)` on every model class).
- Produces:
  - `src/api/backend.ts`: the canonical DTOs `SessionStatus`, `CredentialsInfo`, `SecretStorageInfo`, `CredentialsInput`, `LoginStart`, `LogoutResult`, `Profile`, `OrderRow`, `SectionCounts`, `BookSnapshot`, `PositionRow`, `PositionsSnapshot`, `HoldingRow`, `HoldingsSummary`, `HoldingsSnapshot`, `KiteOrder`, `TickDTO`, `FeedState`, `ClockOffset`, `MarketStatus`, `StatusSnapshot`, `SettingValue`, `SettingsChanged`, `ShortcutAction`, `DataStats`, `BackupInfo`, `JobProgress`, `CalendarInfo`, `CalendarEntryDTO`, `PhaseDTO`, `CalendarEntryInput`, `CrashInfo`, `AppInfo`, `Licence`, `Navigate`; helper `timeMs(rfc3339): Ms`; `interface Backend`, `interface EventMap`, `type EventName = keyof EventMap`.
  - `src/api/errors.ts`: `interface BackendError { code: string; stage: string; reason: string }`, `parseBackendError(e: unknown): BackendError`.
  - `src/api/BackendContext.tsx`: `BackendProvider({ backend, children })`, `useBackend(): Backend`.
  - `src/api/mock/index.ts`: `interface MockState`, `interface MockBackend extends Backend { state; calls; emit(name, payload); failNext(method, reason, code?); listenerCount(name); callsTo(method); simulateBrowserLogin() }`, `createMockBackend(seed: MockState, opts?: { persistKey?: string; autoJobs?: boolean; jobDelayMs?: number }): MockBackend`.
  - `src/api/mock/fixtures.ts`: `FIXED_NOW`, `DAY_MS`, `ZERO_TIME`, `iso(ms)`, `next0600Ist(now)`, `readySession(overrides)`, `tickFixture(token, ltp, close?)`, `kiteOrder(overrides)`, `bookOf(orders, asOfMs?)`, `positionsOf(net, day, asOfMs?)`, `holdingsOf(rows, asOfMs?)`, fixtures `profileFixture`, `holdingA`, `holdingB`, `positionOpen`, `positionClosed`, `ordersFixture`, `shortcutFixture`, `LONG_NAME`, seeds `demoSeed`, `freshSeed`, `noKeyringSeed`, `lockedSeed`, `loggedOutSeed`, `unackedSeed`, `personalSeed`, `emptySeed`, `longNamesSeed`, `seedByName(name)`.
  - `src/api/events.ts`: `startEventBridge(backend, { navigate }): () => void`, `bootstrap(backend): Promise<void>`.
  - `src/api/index.ts`: `defaultBackend(): Backend` (mock when `VITE_BACKEND=mock`, exposed as `window.__tradeloomMock`, seed from `?seed=<name>`).
  - Stores: `useSessionStore` (`session: SessionStatus | null`, `profile`, `setSession`, `setProfile`), `useStatusStore` (`feed`, `market`, `clock`, `setFeed`, `setMarket`, `setClock`, `setSnapshot`), `useTicksStore` (`latest: Record<number, TickDTO>`, `applyBatch`) + `useLtp(token)` (rupees `ltp`), `useSettingsStore` (`values`, `loaded`, `writesSuspended`, `hydrate(SettingValue[])`, `put`, `remove`, `setSuspended`) + `useSetting<T>(key, fallback: primitive)`, `writeSetting(backend, key, value)`, `reloadSettings(backend)`, `suspendSettingsWrites()`, `resumeSettingsWrites()`, `useAppStore` (`info`, `setInfo`), `registerReset(fn)`, `resetAllStores()`.
  - `src/test/render.tsx`: `renderApp({ route?, backend? }) → RTL result + { backend: MockBackend; router: AppRouter; user }`.
  - `App({ backend, router? })`.

- [ ] **Step 1: Write the failing tests**

Create `frontend/src/api/errors.test.ts`:

```ts
import { describe, expect, it } from 'vitest';
import { parseBackendError } from './errors';

describe('parseBackendError (ARCHITECTURE §8.4 error JSON)', () => {
  it('reads code, stage and reason from the JSON form', () => {
    expect(parseBackendError('{"code":"conflict","stage":"shortcuts","reason":"Go to Home"}'))
      .toEqual({ code: 'conflict', stage: 'shortcuts', reason: 'Go to Home' });
  });
  it('accepts Error instances and plain strings', () => {
    expect(parseBackendError(new Error('network down')).reason).toBe('network down');
    expect(parseBackendError('timeout')).toEqual({ code: 'error', stage: '', reason: 'timeout' });
  });
  it('falls back to a generic message for empty input', () => {
    expect(parseBackendError(undefined).reason).toBe('Something went wrong');
  });
});
```

Create `frontend/src/api/wails.test.ts`:

```ts
import { afterEach, describe, expect, it, vi } from 'vitest';
import { createWailsBackend } from './wails';

type WailsWindow = Window & { go?: unknown; runtime?: unknown };

afterEach(() => {
  delete (window as WailsWindow).go;
  delete (window as WailsWindow).runtime;
});

describe('Wails adapter wiring (generated bindings call window.go.api.<Struct>.<Method>)', () => {
  it('routes calls and arguments to the bound Go methods', async () => {
    const Status = vi.fn().mockResolvedValue({ state: 'ready' });
    const SaveCredentials = vi.fn().mockResolvedValue({ state: 'logged_out' });
    const Set = vi.fn().mockResolvedValue({ name: 'appearance.theme', valueJson: '"dark"' });
    const OrderBook = vi.fn().mockResolvedValue({ orders: [], counts: { open: 0, executed: 0, rejected: 0, cancelled: 0 }, asOfMs: 0, stale: false });
    const Remove = vi.fn().mockResolvedValue({ version: '2026.1', coversUntil: '2026-12-31', stale: false, entries: [] });
    (window as WailsWindow).go = {
      api: { Session: { Status, SaveCredentials }, Settings: { Set }, Portfolio: { OrderBook }, Calendar: { Remove } },
    };
    const backend = createWailsBackend();
    await expect(backend.session.status()).resolves.toEqual({ state: 'ready' });
    await backend.session.saveCredentials({ apiKey: 'kiteapikey9f2c', apiSecret: 's3cret' });
    expect(SaveCredentials).toHaveBeenCalledWith(expect.objectContaining({ apiKey: 'kiteapikey9f2c', apiSecret: 's3cret' }));
    await backend.settings.set('appearance.theme', '"dark"');
    expect(Set).toHaveBeenCalledWith('appearance.theme', '"dark"');
    await expect(backend.portfolio.orderBook()).resolves.toMatchObject({ orders: [] });
    await backend.calendar.remove('NSE', '2026-10-20');
    expect(Remove).toHaveBeenCalledWith('NSE', '2026-10-20');
  });
  it('subscribes through runtime.EventsOnMultiple and returns its unsubscribe', () => {
    const off = vi.fn();
    const EventsOnMultiple = vi.fn(() => off);
    (window as WailsWindow).runtime = { EventsOnMultiple };
    const cb = vi.fn();
    const unsubscribe = createWailsBackend().on('feed.state', cb);
    expect(EventsOnMultiple).toHaveBeenCalledWith('feed.state', cb, -1);
    unsubscribe();
    expect(off).toHaveBeenCalledTimes(1);
  });
});
```

Create `frontend/src/api/mock/mock.test.ts`:

```ts
import { describe, expect, it, vi } from 'vitest';
import { parseBackendError } from '../errors';
import { demoSeed, freshSeed, personalSeed, seedByName, ZERO_TIME } from './fixtures';
import { createMockBackend } from './index';

describe('mock backend', () => {
  it('records calls and returns copies of its state', async () => {
    const m = createMockBackend(demoSeed());
    const h = await m.portfolio.holdings();
    h.rows[0].qty = 999;
    expect(m.state.holdings.rows[0].qty).not.toBe(999);
    expect(m.callsTo('portfolio.holdings')).toHaveLength(1);
  });
  it('fails the next call of a method once', async () => {
    const m = createMockBackend(demoSeed());
    m.failNext('portfolio.orderBook', 'network down');
    await expect(m.portfolio.orderBook()).rejects.toThrow('network down');
    await expect(m.portfolio.orderBook()).resolves.toMatchObject({ counts: { open: 1, executed: 1, rejected: 1, cancelled: 1 } });
  });
  it('emits events to subscribers and supports unsubscribe', () => {
    const m = createMockBackend(demoSeed());
    const cb = vi.fn();
    const off = m.on('feed.state', cb);
    const feed = { state: 'offline' as const, attempt: 0, nextRetry: ZERO_TIME, lastTick: ZERO_TIME, connections: 0, tokens: 0 };
    m.emit('feed.state', feed);
    off();
    m.emit('feed.state', { ...feed, state: 'live' });
    expect(cb).toHaveBeenCalledTimes(1);
    expect(m.listenerCount('feed.state')).toBe(0);
  });
  it('refuses a conflicting rebind naming the other action (FR-SHELL-019 AC2 contract)', async () => {
    const m = createMockBackend(demoSeed());
    const err = await m.settings.rebind('kill.switch', 'Alt+1').catch((e: unknown) => parseBackendError(e));
    expect(err).toEqual({ code: 'conflict', stage: 'shortcuts', reason: 'Go to Home' });
  });
  it('hides kite.* keys from Settings.All and refuses to set them', async () => {
    const m = createMockBackend(demoSeed());
    m.state.settings['kite.redirect_port'] = '7420';
    expect((await m.settings.all()).map((v) => v.name)).not.toContain('kite.redirect_port');
    const err = await m.settings.set('kite.redirect_port', '8080').catch((e: unknown) => parseBackendError(e));
    expect(err).toMatchObject({ code: 'use_credentials_flow' });
  });
  it('moves through setup → paste login → ready (risk disclosure still required)', async () => {
    const m = createMockBackend(freshSeed());
    await m.session.saveCredentials({ apiKey: 'kiteapikey9f2c', apiSecret: 'secret-value' });
    expect(m.state.session.state).toBe('logged_out');
    expect(m.state.session.credentials.apiKeyMasked).toBe('••••9f2c');
    const status = await m.session.completeLogin('http://127.0.0.1:7420/kite/callback?request_token=abc123XYZ&status=success');
    expect(status).toMatchObject({ state: 'ready', riskAckRequired: true });
  });
  it('the personal seed is a ready session without a data plan', () => {
    expect(personalSeed().session).toMatchObject({ state: 'ready', dataPlan: 'none' });
    expect(seedByName('personal').session.dataPlan).toBe('none');
  });
});
```

Create `frontend/src/api/events.test.ts`:

```ts
import { describe, expect, it, vi } from 'vitest';
import { useAppStore } from '../store/app';
import { useSessionStore } from '../store/session';
import { useSettingsStore } from '../store/settings';
import { useStatusStore } from '../store/status';
import { useTicksStore } from '../store/ticks';
import { bootstrap, startEventBridge } from './events';
import { demoSeed, FIXED_NOW, iso, readySession, tickFixture, ZERO_TIME } from './mock/fixtures';
import { createMockBackend } from './mock/index';

describe('event bridge', () => {
  it('dispatches each backend event into its store and unsubscribes on stop', () => {
    const backend = createMockBackend(demoSeed());
    const navigate = vi.fn();
    const stop = startEventBridge(backend, { navigate });
    backend.emit('session.state', readySession({ state: 'expired' }));
    backend.emit('feed.state', { state: 'reconnecting', attempt: 2, nextRetry: iso(FIXED_NOW + 5000), lastTick: ZERO_TIME, connections: 0, tokens: 3 });
    backend.emit('market.status', { exchange: 'NSE', phase: 'closed', dayType: 'holiday', label: 'Holiday', tooltip: 'Exchange holiday', atMs: FIXED_NOW });
    backend.emit('clock.offset', { millis: 3200, source: 'ticks', samples: 12, warn: true, skewed: false, updatedAt: iso(FIXED_NOW) });
    backend.emit('tick.batch', [tickFixture(1, 10, 9)]);
    backend.emit('settings.changed', { key: 'appearance.theme', value: 'dark' });
    backend.emit('app.navigate', { route: '/holdings' });
    expect(useSessionStore.getState().session?.state).toBe('expired');
    expect(useStatusStore.getState().feed).toMatchObject({ state: 'reconnecting', attempt: 2 });
    expect(useStatusStore.getState().market?.label).toBe('Holiday');
    expect(useStatusStore.getState().clock?.millis).toBe(3200);
    expect(useTicksStore.getState().latest[1].ltp).toBe(10);
    expect(useSettingsStore.getState().values['appearance.theme']).toBe('dark');
    expect(navigate).toHaveBeenCalledWith('/holdings');
    stop();
    for (const name of ['session.state', 'feed.state', 'market.status', 'clock.offset', 'tick.batch', 'settings.changed', 'app.navigate'] as const) {
      expect(backend.listenerCount(name)).toBe(0);
    }
  });
  it('bootstrap loads session, status, settings and app info', async () => {
    const seed = demoSeed();
    seed.settings = { 'appearance.theme': '"dark"' };
    await bootstrap(createMockBackend(seed));
    expect(useSessionStore.getState().session?.state).toBe('ready');
    expect(useStatusStore.getState().feed.state).toBe('live');
    expect(useSettingsStore.getState()).toMatchObject({ loaded: true, values: { 'appearance.theme': 'dark' } });
    expect(useAppStore.getState().info?.version).toBe('0.1.0');
  });
});
```

Create `frontend/src/store/ticks.test.ts`:

```ts
import { expect, it } from 'vitest';
import { tickFixture } from '../api/mock/fixtures';
import { useTicksStore } from './ticks';

it('applyBatch replaces only the tokens in the batch (unchanged rows keep identity)', () => {
  useTicksStore.getState().applyBatch([tickFixture(1, 10, 9), tickFixture(2, 20, 19)]);
  const before = useTicksStore.getState().latest;
  useTicksStore.getState().applyBatch([tickFixture(2, 21, 19)]);
  const after = useTicksStore.getState().latest;
  expect(after[1]).toBe(before[1]);
  expect(after[2].ltp).toBe(21);
});
```

Create `frontend/src/store/settings.test.tsx`:

```tsx
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { useState } from 'react';
import { describe, expect, it } from 'vitest';
import { BackendProvider } from '../api/BackendContext';
import { demoSeed } from '../api/mock/fixtures';
import { createMockBackend } from '../api/mock/index';
import { suspendSettingsWrites, useSetting } from './settings';

function Harness() {
  const [keep, setKeep] = useSetting<number>('backup.schedule.keep', 7);
  const [result, setResult] = useState('');
  return (
    <>
      <output data-testid="value">{String(keep)}</output>
      <button onClick={async () => setResult((await setKeep(0))?.reason ?? 'ok')}>bad</button>
      <button onClick={async () => setResult((await setKeep(30))?.reason ?? 'ok')}>good</button>
      <output data-testid="result">{result}</output>
    </>
  );
}

function setup() {
  const backend = createMockBackend(demoSeed());
  render(<BackendProvider backend={backend}><Harness /></BackendProvider>);
  return { backend, user: userEvent.setup() };
}

describe('useSetting', () => {
  it('commits a valid value to the backend', async () => {
    const { backend, user } = setup();
    await user.click(screen.getByText('good'));
    expect(await screen.findByTestId('result')).toHaveTextContent('ok');
    expect(screen.getByTestId('value')).toHaveTextContent('30');
    expect(backend.state.settings['backup.schedule.keep']).toBe('30');
  });
  it('keeps the old value and returns the reason when the backend refuses (FR-ACCT-012 AC2 pattern)', async () => {
    const { user } = setup();
    await user.click(screen.getByText('bad'));
    expect(await screen.findByText('Keep between 1 and 365 backups')).toBeInTheDocument();
    expect(screen.getByTestId('value')).toHaveTextContent('7');
  });
  it('does not call the backend while writes are suspended', async () => {
    const { backend, user } = setup();
    suspendSettingsWrites();
    await user.click(screen.getByText('good'));
    expect(await screen.findByText('A restore is in progress — settings changes are paused')).toBeInTheDocument();
    expect(backend.callsTo('settings.set')).toHaveLength(0);
  });
});
```

Create `frontend/src/app/persistence.test.tsx`:

```tsx
import { screen, waitFor } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { demoSeed } from '../api/mock/fixtures';
import { createMockBackend } from '../api/mock/index';
import { resetAllStores } from '../store/reset';
import { renderApp } from '../test/render';

describe('settings-backed shell state', () => {
  it('FR-SHELL-002 AC4: the collapsed sidebar survives a restart', async () => {
    const backend = createMockBackend(demoSeed());
    const first = renderApp({ backend });
    await first.user.click(await screen.findByRole('button', { name: 'Collapse sidebar' }));
    expect(backend.state.settings['appearance.sidebar_collapsed']).toBe('true');
    first.unmount();
    resetAllStores();
    renderApp({ backend });
    expect(await screen.findByRole('button', { name: 'Expand sidebar' })).toBeInTheDocument();
  });
  it('applies the persisted dark theme on start', async () => {
    const seed = demoSeed();
    seed.settings = { 'appearance.theme': '"dark"' };
    renderApp({ backend: createMockBackend(seed) });
    await waitFor(() => expect(document.documentElement.dataset.theme).toBe('dark'));
  });
});
```

Overwrite `frontend/src/app/AppShell.test.tsx` (the shell now needs a backend):

```tsx
import { render, screen, within } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { renderApp } from '../test/render';
import { HomeLayout } from './HomeLayout';

describe('FR-SHELL-001 frame', () => {
  it('AC1 (P0 form): top bar, sidebar and content are visible; the sidebar holds the P0 entries', async () => {
    renderApp({ route: '/home' });
    expect(await screen.findByRole('banner')).toBeInTheDocument();
    const nav = screen.getByRole('navigation', { name: 'Main navigation' });
    expect(within(nav).getAllByRole('link').map((l) => l.textContent)).toEqual(['Home']);
    expect(screen.getByRole('main')).toBeInTheDocument();
  });
  it('HomeLayout renders two column scroll containers', () => {
    render(<HomeLayout topRow={<p>cards</p>} main={<p>chart</p>} right={<p>lists</p>} />);
    expect(screen.getByTestId('home-left')).toHaveTextContent('cardschart');
    expect(screen.getByTestId('home-right')).toHaveTextContent('lists');
  });
});
```

- [ ] **Step 2: Run them to verify they fail**

Run: `cd frontend && npx vitest run src/api src/store src/app/persistence.test.tsx src/app/AppShell.test.tsx`
Expected: FAIL — `Failed to resolve import "./errors"`, `"./wails"`, `"./mock/fixtures"`, `"./events"`, `"./settings"`, `"./ticks"`, `"../test/render"`.

- [ ] **Step 3: Implement the backend contract, adapters and mock**

Create `frontend/src/api/backend.ts`:

```ts
// Mirrors the canonical P0-3 Wails DTOs field-for-field (camelCase JSON tags). Unix-ms fields end in `Ms`,
// money fields in `Paise` (integers), ratios in `Bp` (basis points). P0-2 payloads keep `time.Time` as RFC3339 strings.
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

export interface CredentialsInfo { configured: boolean; apiKeyMasked: string; port: number; redirectUrl: string; secretBackend: string }
export interface SecretStorageInfo { mode: SecretMode; backend: SecretBackendName; keyringAvailable: boolean; fileExists: boolean; choices: SecretChoice[] }
export interface CredentialsInput { apiKey: string; apiSecret: string }

export interface SessionStatus {
  state: SessionStateName;
  sessionId: string;
  userId: string;
  loggedInAtMs: Ms;
  expiresAtMs: Ms;
  dataPlan: DataPlan;
  offline: boolean;
  riskAckRequired: boolean;
  errorCode: string;
  errorMessage: string;
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

// --- Portfolio (orders.* snapshots, P0-3 Task 12)
export type OrderSection = 'open' | 'executed' | 'rejected' | 'cancelled';
export interface OrderRow {
  orderId: string;
  timeMs: Ms;
  side: 'Buy' | 'Sell';
  exchange: string;
  symbol: string;
  token: number;
  product: string;
  orderType: string;
  variety: string;
  qty: number;
  filledQty: number;
  pricePaise: number;
  triggerPaise: number;
  avgPricePaise: number;
  status: string;
  statusMessage: string;
  section: OrderSection;
  tag: string;
  source: 'tradeloom' | 'external';
}
export interface SectionCounts { open: number; executed: number; rejected: number; cancelled: number }
export interface BookSnapshot { orders: OrderRow[]; counts: SectionCounts; asOfMs: Ms; stale: boolean }

export interface PositionRow {
  key: string;
  exchange: string;
  symbol: string;
  token: number;
  product: string;
  qty: number;
  avgPaise: number;
  ltpPaise: number;
  pnlPaise: number;
  netChangeBp: number;
  multiplier: number; // Kite multiplier; Go sends 1 when Kite reports 0
  closed: boolean;
}
export interface PositionsSnapshot { net: PositionRow[]; day: PositionRow[]; netTotalPnlPaise: number; dayTotalPnlPaise: number; asOfMs: Ms; stale: boolean }

export interface HoldingRow {
  exchange: string;
  symbol: string;
  token: number;
  qty: number;
  avgPaise: number;
  ltpPaise: number;
  prevClosePaise: number;
  investedPaise: number;
  currentPaise: number;
  pnlPaise: number;
  netChangeBp: number;
  dayChangePaise: number;
  dayChangeBp: number;
}
export interface HoldingsSummary { empty: boolean; currentValuePaise: number; investmentPaise: number; pnlPaise: number; pnlBp: number; dayChangePaise: number }
export interface HoldingsSnapshot { rows: HoldingRow[]; summary: HoldingsSummary; asOfMs: Ms; stale: boolean }

export type RefreshScope = 'orders' | 'positions' | 'holdings' | 'all';

/** `order.update` payload: gokiteconnect `Order` (snake_case JSON). Only the fields the UI reads are typed. */
export interface KiteOrder {
  order_id: string;
  status: string;
  status_message: string;
  exchange: string;
  tradingsymbol: string;
  instrument_token: number;
  transaction_type: string;
  product: string;
  order_type: string;
  quantity: number;
  filled_quantity: number;
  average_price: number;
  tag: string;
}

// --- Market data and status (service.TickDTO, ticker.FeedState, clock.Offset, service.MarketStatus)
export interface TickDTO { token: number; ltp: number; close: number; change: number; changePct: number; volume: number; tsMs: Ms; polled: boolean }

export type FeedStateName = 'stopped' | 'connecting' | 'live' | 'reconnecting' | 'offline';
export interface FeedState { state: FeedStateName; attempt: number; nextRetry: Rfc3339; lastTick: Rfc3339; connections: number; tokens: number }

export type ClockSource = 'none' | 'ticks' | 'http-date';
export interface ClockOffset { millis: number; source: ClockSource; samples: number; warn: boolean; skewed: boolean; updatedAt: Rfc3339 }

export type MarketPhase = 'pre-open' | 'continuous' | 'closing' | 'post-closing' | 'closed';
export type DayType = 'trading' | 'holiday' | 'special' | 'weekend' | 'inferred-non-trading';
export interface MarketStatus { exchange: string; phase: MarketPhase; dayType: DayType; label: string; tooltip: string; atMs: Ms }

export interface StatusSnapshot { feed: FeedState; market: MarketStatus; clock: ClockOffset }

// --- Settings (service.SettingValue, settings.Changed, settings.ShortcutAction)
export interface SettingValue { name: string; valueJson: string }
export interface SettingsChanged { key: string; value: unknown }

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

export interface JobProgress {
  jobId: string;
  kind: 'backup' | 'restore' | 'support_bundle';
  stage: string;
  done: number;
  total: number;
  finished: boolean;
  error: string;
  resultPath: string;
}

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

export interface Navigate { route: string }

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
}
export type EventName = keyof EventMap;

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
  on<K extends EventName>(name: K, cb: (payload: EventMap[K]) => void): () => void;
}
```

Create `frontend/src/api/errors.ts`:

```ts
export interface BackendError { code: string; stage: string; reason: string }

const GENERIC = 'Something went wrong';

export function parseBackendError(e: unknown): BackendError {
  const raw = e instanceof Error ? e.message : typeof e === 'string' ? e : '';
  try {
    const j: unknown = JSON.parse(raw);
    if (j && typeof j === 'object' && typeof (j as { reason?: unknown }).reason === 'string') {
      const o = j as { code?: unknown; stage?: unknown; reason: string };
      return { code: String(o.code ?? 'error'), stage: String(o.stage ?? ''), reason: o.reason };
    }
  } catch {
    // not JSON: plain Go error text
  }
  return { code: 'error', stage: '', reason: raw || GENERIC };
}
```

Create `frontend/src/api/wails.ts`:

```ts
// The only module that imports the generated Wails bindings (ARCHITECTURE §8.1).
import * as Calendar from '../../wailsjs/go/api/Calendar';
import * as Data from '../../wailsjs/go/api/Data';
import * as Diagnostics from '../../wailsjs/go/api/Diagnostics';
import * as Portfolio from '../../wailsjs/go/api/Portfolio';
import * as Session from '../../wailsjs/go/api/Session';
import * as Settings from '../../wailsjs/go/api/Settings';
import * as System from '../../wailsjs/go/api/System';
import { service } from '../../wailsjs/go/models';
import { EventsOn } from '../../wailsjs/runtime/runtime';
import type { Backend } from './backend';

// The generated functions return the Wails model classes (service.*, kite.*, orders.*, settings.*); their JSON shape
// is exactly the interfaces in backend.ts, so the adapter only narrows the static type.
const cast = <T,>(p: Promise<unknown>): Promise<T> => p as Promise<T>;

export function createWailsBackend(): Backend {
  return {
    session: {
      status: () => cast(Session.Status()),
      credentials: () => cast(Session.Credentials()),
      saveCredentials: (input) => cast(Session.SaveCredentials(service.CredentialsInput.createFrom(input))),
      removeCredentials: () => cast(Session.RemoveCredentials()),
      setRedirectPort: (port) => cast(Session.SetRedirectPort(port)),
      login: () => cast(Session.Login()),
      completeLogin: (pasted) => cast(Session.CompleteLogin(pasted)),
      cancelLogin: () => cast(Session.CancelLogin()),
      logout: () => cast(Session.Logout()),
      ackRiskDisclosure: () => cast(Session.AckRiskDisclosure()),
      profile: () => cast(Session.Profile()),
      secretStorage: () => cast(Session.SecretStorage()),
      useEncryptedFile: (passphrase) => cast(Session.UseEncryptedFile(passphrase)),
      refuseSecretStorage: () => cast(Session.RefuseSecretStorage()),
      unlock: (passphrase) => cast(Session.Unlock(passphrase)),
      migrateToKeyring: () => cast(Session.MigrateToKeyring()),
      reconnectFeed: () => Session.ReconnectFeed(),
    },
    portfolio: {
      orderBook: () => cast(Portfolio.OrderBook()),
      positions: () => cast(Portfolio.Positions()),
      holdings: () => cast(Portfolio.Holdings()),
      refresh: (scope) => Portfolio.Refresh(scope),
    },
    settings: {
      all: () => cast(Settings.All()),
      set: (name, valueJSON) => cast(Settings.Set(name, valueJSON)),
      shortcuts: () => cast(Settings.Shortcuts()),
      rebind: (actionId, binding) => cast(Settings.Rebind(actionId, binding)),
      resetShortcuts: () => cast(Settings.ResetShortcuts()),
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
      inspectBackup: (path) => cast(Data.InspectBackup(path)),
      restore: (path, confirmation) => Data.Restore(path, confirmation),
    },
    calendar: {
      info: () => cast(Calendar.Info()),
      chooseImportFile: () => Calendar.ChooseImportFile(),
      import: (path) => cast(Calendar.Import(path)),
      upsert: (entry) => cast(Calendar.Upsert(service.CalendarEntryInput.createFrom(entry))),
      remove: (exchange, date) => cast(Calendar.Remove(exchange, date)),
    },
    diagnostics: {
      openLogsFolder: () => Diagnostics.OpenLogsFolder(),
      createSupportBundle: () => Diagnostics.CreateSupportBundle(),
      lastCrash: () => cast(Diagnostics.LastCrash()),
      openCrashReport: () => Diagnostics.OpenCrashReport(),
      dismissCrash: () => Diagnostics.DismissCrash(),
    },
    system: {
      info: () => cast(System.Info()),
      status: () => cast(System.Status()),
    },
    on: (name, cb) => EventsOn(name, cb as (...data: unknown[]) => void),
  };
}
```

Create `frontend/src/api/BackendContext.tsx`:

```tsx
import { createContext, useContext, type ReactNode } from 'react';
import type { Backend } from './backend';

const BackendContext = createContext<Backend | null>(null);

export function BackendProvider({ backend, children }: { backend: Backend; children: ReactNode }) {
  return <BackendContext.Provider value={backend}>{children}</BackendContext.Provider>;
}

export function useBackend(): Backend {
  const backend = useContext(BackendContext);
  if (!backend) throw new Error('useBackend must be used inside <BackendProvider>');
  return backend;
}
```

Create `frontend/src/api/mock/fixtures.ts`:

```ts
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
  investedPaise: 100_000, currentPaise: 110_000, pnlPaise: 10_000, netChangeBp: 1000, dayChangePaise: 5_000, dayChangeBp: 476,
};
export const holdingB: HoldingRow = {
  exchange: 'NSE', symbol: 'TCS', token: 2953217, qty: 5, avgPaise: 20_000, ltpPaise: 19_000, prevClosePaise: 20_000,
  investedPaise: 100_000, currentPaise: 95_000, pnlPaise: -5_000, netChangeBp: -500, dayChangePaise: -5_000, dayChangeBp: -500,
};

// FR-ORD-002 AC1: P&L +120 and -45.50 → total 74.50.
export const positionOpen: PositionRow = {
  key: 'net|NSE|INFY|MIS', exchange: 'NSE', symbol: 'INFY', token: 408065, product: 'MIS',
  qty: 10, avgPaise: 10_000, ltpPaise: 11_200, pnlPaise: 12_000, netChangeBp: 1200, multiplier: 1, closed: false,
};
export const positionClosed: PositionRow = {
  key: 'net|NSE|TATAMOTORS|MIS', exchange: 'NSE', symbol: 'TATAMOTORS', token: 884737, product: 'MIS',
  qty: 0, avgPaise: 10_455, ltpPaise: 10_100, pnlPaise: -4_550, netChangeBp: -340, multiplier: 1, closed: true,
};

const order = (o: Pick<OrderRow, 'orderId' | 'timeMs' | 'side' | 'symbol' | 'token' | 'product' | 'qty' | 'filledQty' | 'avgPricePaise' | 'status' | 'section'> & Partial<OrderRow>): OrderRow => ({
  exchange: 'NSE', orderType: 'LIMIT', variety: 'regular', pricePaise: 0, triggerPaise: 0, statusMessage: '', tag: '', source: 'external', ...o,
});

export const ordersFixture: OrderRow[] = [
  order({ orderId: '261012000000001', timeMs: FIXED_NOW - 600_000, side: 'Buy', symbol: 'RELIANCE', token: 738561, product: 'CNC', qty: 10, filledQty: 0, pricePaise: 10_900, avgPricePaise: 0, status: 'OPEN', section: 'open' }),
  order({ orderId: '261012000000002', timeMs: FIXED_NOW - 1_200_000, side: 'Sell', symbol: 'INFY', token: 408065, product: 'MIS', qty: 5, filledQty: 5, avgPricePaise: 151_025, status: 'COMPLETE', section: 'executed', orderType: 'MARKET', tag: 'TLd7', source: 'tradeloom' }),
  order({ orderId: '261012000000003', timeMs: FIXED_NOW - 1_800_000, side: 'Buy', symbol: 'TCS', token: 2953217, product: 'CNC', qty: 1, filledQty: 0, avgPricePaise: 0, status: 'REJECTED', statusMessage: 'Insufficient funds', section: 'rejected' }),
  order({ orderId: '261012000000004', timeMs: FIXED_NOW - 2_400_000, side: 'Buy', symbol: 'HDFCBANK', token: 341249, product: 'CNC', qty: 2, filledQty: 0, avgPricePaise: 0, status: 'CANCELLED', section: 'cancelled' }),
];

/** Same rules as Go `orders.BuildBook`: counts per section, newest first. */
export function bookOf(orders: OrderRow[], asOfMs = FIXED_NOW - 30_000): BookSnapshot {
  const counts = { open: 0, executed: 0, rejected: 0, cancelled: 0 };
  for (const o of orders) counts[o.section] += 1;
  return { orders: [...orders].sort((a, b) => b.timeMs - a.timeMs), counts, asOfMs, stale: false };
}

/** Same totals as Go `orders.BuildPositions`. */
export function positionsOf(net: PositionRow[], day: PositionRow[], asOfMs = FIXED_NOW - 30_000): PositionsSnapshot {
  const total = (rows: PositionRow[]) => rows.reduce((s, r) => s + r.pnlPaise, 0);
  return { net, day, netTotalPnlPaise: total(net), dayTotalPnlPaise: total(day), asOfMs, stale: false };
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

// Mirrors the Go registry defaults (contract §E / A8); test data only — the app always reads Settings.Shortcuts().
export const shortcutFixture: ShortcutAction[] = [
  ...NAV_TARGETS.map((label, i): ShortcutAction => ({ id: `nav.${i + 1}`, label: `Go to ${label}`, scope: 'global', defaultBinding: `Alt+${i + 1}`, binding: `Alt+${i + 1}` })),
  { id: 'toasts.focus', label: 'Focus newest notification', scope: 'global', defaultBinding: 'Alt+T', binding: 'Alt+T' },
  { id: 'search.open', label: 'Search symbols', scope: 'global', defaultBinding: 'Ctrl+K', binding: 'Ctrl+K' },
  { id: 'help.open', label: 'Open help', scope: 'global', defaultBinding: 'F1', binding: 'F1' },
  { id: 'kill.switch', label: 'Stop all trading', scope: 'global', defaultBinding: 'Ctrl+Shift+K', binding: 'Ctrl+Shift+K' },
  { id: 'chart.fullView', label: 'Full-view chart', scope: 'global', defaultBinding: 'F11', binding: 'F11' },
  { id: 'chart.trendLine', label: 'Trend line tool', scope: 'chart', defaultBinding: 'Alt+T', binding: 'Alt+T' },
  { id: 'order.new', label: 'New order', scope: 'chart', defaultBinding: 'Shift+T', binding: 'Shift+T' },
  { id: 'builder.undo', label: 'Undo', scope: 'builder', defaultBinding: 'Ctrl+Z', binding: 'Ctrl+Z' },
  { id: 'builder.redo', label: 'Redo', scope: 'builder', defaultBinding: 'Ctrl+Shift+Z', binding: 'Ctrl+Shift+Z' },
  { id: 'watchlist.moveUp', label: 'Move watchlist row up', scope: 'view', defaultBinding: 'Alt+ArrowUp', binding: 'Alt+ArrowUp' },
  { id: 'watchlist.moveDown', label: 'Move watchlist row down', scope: 'view', defaultBinding: 'Alt+ArrowDown', binding: 'Alt+ArrowDown' },
];

const statusFixture: StatusSnapshot = {
  feed: { state: 'live', attempt: 0, nextRetry: ZERO_TIME, lastTick: iso(FIXED_NOW - 1000), connections: 1, tokens: 3 },
  market: { exchange: 'NSE', phase: 'continuous', dayType: 'trading', label: 'Open', tooltip: 'Normal market 09:15–15:30 IST', atMs: FIXED_NOW },
  clock: { millis: 120, source: 'ticks', samples: 40, warn: false, skewed: false, updatedAt: iso(FIXED_NOW) },
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
  s.status.feed = { ...s.status.feed, state: 'stopped', connections: 0, tokens: 0, lastTick: ZERO_TIME };
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
```

Create `frontend/src/api/mock/index.ts`:

```ts
import type {
  AppInfo, Backend, BackupInfo, BookSnapshot, CalendarEntryInput, CalendarInfo, CrashInfo, CredentialsInput, DataStats,
  EventMap, EventName, HoldingsSnapshot, JobProgress, PositionsSnapshot, Profile, RefreshScope, SessionStatus, ShortcutAction,
  ShortcutScope, StatusSnapshot,
} from '../backend';
import { next0600Ist } from './fixtures';

export interface MockState {
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
  calendar: CalendarInfo;
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

const TIMEFRAMES = ['minute', '3minute', '5minute', '10minute', '15minute', '30minute', '60minute', 'day'];

// The Go validators of contract §E that the UI can trip.
const VALIDATORS: Record<string, (v: unknown) => string | null> = {
  'backup.schedule.keep': (v) => (Number.isInteger(v) && (v as number) >= 1 && (v as number) <= 365 ? null : 'Keep between 1 and 365 backups'),
  'log.level': (v) => (v === 'info' || v === 'debug' ? null : 'Log level must be info or debug'),
  'chart.default_timeframe': (v) => (typeof v === 'string' && TIMEFRAMES.includes(v) ? null : 'Unknown timeframe'),
};

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
    if (state.session.dataPlan !== 'none') setFeed({ state: 'live', attempt: 0, lastTick: new Date(now).toISOString() });
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
        const key = input.apiKey.trim();
        if (!key || !input.apiSecret.trim()) throw refusal('invalid_credentials', 'session', 'Enter both the API key and the API secret');
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
        if (!Number.isInteger(port) || port < 1024 || port > 65535) throw refusal('invalid_port', 'session', 'Port must be a whole number between 1024 and 65535');
        const credentials = { ...state.session.credentials, port, redirectUrl: `http://127.0.0.1:${port}/kite/callback` };
        setSession({ credentials });
        return copy(credentials);
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
      completeLogin: (pasted: string) => {
        const v = pasted.trim();
        if (/request_token=[A-Za-z0-9]+/.test(v) || /^[A-Za-z0-9]{6,}$/.test(v)) return loginSucceeded();
        throw refusal('bad_redirect', 'login', "That isn't a Kite redirect URL or request token");
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
      refuseSecretStorage: () => setSession({ secrets: { ...state.session.secrets, mode: 'none', backend: 'none' } }),
      unlock: (passphrase: string) => {
        if (passphrase !== state.encPassphrase) throw refusal('bad_passphrase', 'secrets', 'Wrong passphrase');
        return setSession({
          state: state.session.credentials.configured ? 'logged_out' : 'no_credentials',
          secrets: { ...state.session.secrets, backend: 'file' },
        });
      },
      migrateToKeyring: () => {
        if (!state.session.secrets.keyringAvailable) throw refusal('keyring_unavailable', 'secrets', 'The system keyring is not available');
        const secrets = { mode: 'keyring' as const, backend: 'keyring' as const, keyringAvailable: true, fileExists: false, choices: [] };
        setSession({ secrets, credentials: { ...state.session.credentials, secretBackend: 'keyring' } });
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
        const clash = state.shortcuts.find((a) => a.id !== actionId && a.binding === binding && overlaps(a.scope, target.scope));
        if (clash) throw refusal('conflict', 'shortcuts', clash.label);
        target.binding = binding;
        return copy(target);
      },
      resetShortcuts: () => {
        for (const a of state.shortcuts) a.binding = a.defaultBinding;
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
      info: () => copy(state.calendar),
      chooseImportFile: () => state.fileChoice,
      import: (_path: string) => copy(state.calendar),
      upsert: (entry: CalendarEntryInput) => {
        const others = state.calendar.entries.filter((e) => !(e.exchange === entry.exchange && e.date === entry.date));
        state.calendar.entries = [...others, { ...entry, source: 'user' }].sort((a, b) => a.date.localeCompare(b.date));
        return copy(state.calendar);
      },
      remove: (exchange: string, date: string) => {
        state.calendar.entries = state.calendar.entries.filter((e) => !(e.exchange === exchange && e.date === date));
        return copy(state.calendar);
      },
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
  };
  return backend;
}
```

Create `frontend/src/api/index.ts`:

```ts
import type { Backend } from './backend';
import { createMockBackend, type MockBackend } from './mock/index';
import { seedByName } from './mock/fixtures';
import { createWailsBackend } from './wails';

declare global {
  interface Window { __tradeloomMock?: MockBackend }
}

/** Real Wails bindings, or the in-memory mock under `vite --mode mock` (Playwright, `npm run dev:mock`). */
export function defaultBackend(): Backend {
  if (import.meta.env.VITE_BACKEND !== 'mock') return createWailsBackend();
  const params = new URLSearchParams(window.location.search);
  const mock = createMockBackend(seedByName(params.get('seed') ?? 'demo'), {
    persistKey: params.has('persist') ? 'tradeloom.mock.settings' : undefined,
    autoJobs: true,
  });
  window.__tradeloomMock = mock;
  return mock;
}
```

- [ ] **Step 4: Implement the stores and the event bridge**

Append to `frontend/src/i18n/copy.ts`:

```ts
export const settingsCopy = {
  restoreInProgress: 'A restore is in progress — settings changes are paused',
} as const;
```

Create `frontend/src/store/reset.ts`:

```ts
const resets: (() => void)[] = [];

export function registerReset(fn: () => void): void {
  resets.push(fn);
}

/** Test helper: return every registered store to its initial state. */
export function resetAllStores(): void {
  for (const reset of resets) reset();
}
```

Create `frontend/src/store/session.ts`:

```ts
import { create } from 'zustand';
import type { Profile, SessionStatus } from '../api/backend';
import { registerReset } from './reset';

interface SessionStore {
  session: SessionStatus | null;
  profile: Profile | null;
  setSession(s: SessionStatus): void;
  setProfile(p: Profile | null): void;
}

const initial = { session: null, profile: null };

export const useSessionStore = create<SessionStore>()((set) => ({
  ...initial,
  setSession: (session) =>
    set((st) => ({
      session,
      profile: st.profile && session.state === 'ready' && st.profile.userId === session.userId ? st.profile : null,
    })),
  setProfile: (profile) => set({ profile }),
}));

registerReset(() => useSessionStore.setState(initial));
```

Create `frontend/src/store/status.ts`:

```ts
import { create } from 'zustand';
import type { ClockOffset, FeedState, MarketStatus, StatusSnapshot } from '../api/backend';
import { registerReset } from './reset';

interface StatusStore {
  feed: FeedState;
  market: MarketStatus | null;
  clock: ClockOffset | null;
  setFeed(f: FeedState): void;
  setMarket(m: MarketStatus): void;
  setClock(c: ClockOffset): void;
  setSnapshot(s: StatusSnapshot): void;
}

const ZERO = '0001-01-01T00:00:00Z';
const initial = {
  feed: { state: 'stopped', attempt: 0, nextRetry: ZERO, lastTick: ZERO, connections: 0, tokens: 0 } as FeedState,
  market: null,
  clock: null,
};

const sameFeed = (a: FeedState, b: FeedState) =>
  a.state === b.state && a.attempt === b.attempt && a.nextRetry === b.nextRetry && a.lastTick === b.lastTick
  && a.connections === b.connections && a.tokens === b.tokens;

const sameMarket = (a: MarketStatus | null, b: MarketStatus) =>
  a !== null && a.phase === b.phase && a.dayType === b.dayType && a.label === b.label && a.tooltip === b.tooltip;

export const useStatusStore = create<StatusStore>()((set) => ({
  ...initial,
  // Returning the same state object skips the update, so a storm of identical events re-renders nothing.
  setFeed: (feed) => set((st) => (sameFeed(st.feed, feed) ? st : { feed })),
  setMarket: (market) => set((st) => (sameMarket(st.market, market) ? st : { market })),
  setClock: (clock) => set({ clock }),
  setSnapshot: (s) => set({ feed: s.feed, market: s.market, clock: s.clock }),
}));

registerReset(() => useStatusStore.setState(initial));
```

Create `frontend/src/store/ticks.ts`:

```ts
import { create } from 'zustand';
import type { TickDTO } from '../api/backend';
import { registerReset } from './reset';

interface TicksStore {
  latest: Record<number, TickDTO>;
  applyBatch(ticks: TickDTO[]): void;
}

export const useTicksStore = create<TicksStore>()((set) => ({
  latest: {},
  applyBatch: (ticks) =>
    set((st) => {
      if (ticks.length === 0) return st;
      const latest = { ...st.latest };
      for (const t of ticks) latest[t.token] = t;
      return { latest };
    }),
}));

/** Latest LTP (rupees) for one token; re-renders only when that token's tick changes. */
export function useLtp(token: number | undefined): number | undefined {
  return useTicksStore((s) => (token === undefined ? undefined : s.latest[token]?.ltp));
}

registerReset(() => useTicksStore.setState({ latest: {} }));
```

Create `frontend/src/store/app.ts`:

```ts
import { create } from 'zustand';
import type { AppInfo } from '../api/backend';
import { registerReset } from './reset';

interface AppStore { info: AppInfo | null; setInfo(i: AppInfo): void }

export const useAppStore = create<AppStore>()((set) => ({ info: null, setInfo: (info) => set({ info }) }));

registerReset(() => useAppStore.setState({ info: null }));
```

Create `frontend/src/store/settings.ts`:

```ts
import { useCallback } from 'react';
import { create } from 'zustand';
import type { Backend, SettingValue } from '../api/backend';
import { useBackend } from '../api/BackendContext';
import { parseBackendError, type BackendError } from '../api/errors';
import { settingsCopy } from '../i18n/copy';
import { registerReset } from './reset';

interface SettingsStore {
  /** Decoded values by key (Go sends JSON in `SettingValue.valueJson`). */
  values: Record<string, unknown>;
  loaded: boolean;
  writesSuspended: boolean;
  hydrate(all: SettingValue[]): void;
  put(key: string, value: unknown): void;
  remove(key: string): void;
  setSuspended(v: boolean): void;
}

function parse(raw: string): unknown {
  try {
    return JSON.parse(raw);
  } catch {
    return raw;
  }
}

const initial = { values: {} as Record<string, unknown>, loaded: false, writesSuspended: false };

export const useSettingsStore = create<SettingsStore>()((set) => ({
  ...initial,
  hydrate: (all) => set({ values: Object.fromEntries(all.map((v) => [v.name, parse(v.valueJson)])), loaded: true }),
  put: (key, value) => set((st) => ({ values: { ...st.values, [key]: value } })),
  remove: (key) =>
    set((st) => {
      const values = { ...st.values };
      delete values[key];
      return { values };
    }),
  setSuspended: (writesSuspended) => set({ writesSuspended }),
}));

registerReset(() => useSettingsStore.setState(initial));

/** Optimistic write; on refusal the previous value is restored and the reason returned (inline error). */
export async function writeSetting(backend: Backend, key: string, value: unknown): Promise<BackendError | null> {
  const st = useSettingsStore.getState();
  if (st.writesSuspended) return { code: 'suspended', stage: 'settings', reason: settingsCopy.restoreInProgress };
  const had = key in st.values;
  const previous = st.values[key];
  st.put(key, value);
  try {
    await backend.settings.set(key, JSON.stringify(value));
    return null;
  } catch (e) {
    const now = useSettingsStore.getState();
    if (had) now.put(key, previous);
    else now.remove(key);
    return parseBackendError(e);
  }
}

export async function reloadSettings(backend: Backend): Promise<void> {
  useSettingsStore.getState().hydrate(await backend.settings.all());
}

export const suspendSettingsWrites = (): void => useSettingsStore.getState().setSuspended(true);
export const resumeSettingsWrites = (): void => useSettingsStore.getState().setSuspended(false);

/** `fallback` must be a primitive (it is returned from a store selector). */
export function useSetting<T extends string | number | boolean>(key: string, fallback: T): [T, (v: T) => Promise<BackendError | null>] {
  const backend = useBackend();
  const value = useSettingsStore((s) => (key in s.values ? (s.values[key] as T) : fallback));
  const set = useCallback((v: T) => writeSetting(backend, key, v), [backend, key]);
  return [value, set];
}
```

Create `frontend/src/api/events.ts` (later tasks add lines to the `offs` array):

```ts
import { useAppStore } from '../store/app';
import { useSessionStore } from '../store/session';
import { useSettingsStore } from '../store/settings';
import { useStatusStore } from '../store/status';
import { useTicksStore } from '../store/ticks';
import type { Backend } from './backend';

export interface BridgeDeps { navigate: (route: string) => void }

/** Subscribes once to every backend event and dispatches into stores (ARCHITECTURE §8.2). */
export function startEventBridge(backend: Backend, deps: BridgeDeps): () => void {
  const offs = [
    backend.on('session.state', (s) => useSessionStore.getState().setSession(s)),
    backend.on('feed.state', (f) => useStatusStore.getState().setFeed(f)),
    backend.on('market.status', (m) => useStatusStore.getState().setMarket(m)),
    backend.on('clock.offset', (c) => useStatusStore.getState().setClock(c)),
    backend.on('tick.batch', (t) => useTicksStore.getState().applyBatch(t)),
    backend.on('settings.changed', (c) => useSettingsStore.getState().put(c.key, c.value)),
    backend.on('app.navigate', (n) => deps.navigate(n.route)),
  ];
  return () => {
    for (const off of offs) off();
  };
}

export async function bootstrap(backend: Backend): Promise<void> {
  const [session, status, settings, info] = await Promise.allSettled([
    backend.session.status(), backend.system.status(), backend.settings.all(), backend.system.info(),
  ]);
  if (session.status === 'fulfilled') useSessionStore.getState().setSession(session.value);
  if (status.status === 'fulfilled') useStatusStore.getState().setSnapshot(status.value);
  useSettingsStore.getState().hydrate(settings.status === 'fulfilled' ? settings.value : []);
  if (info.status === 'fulfilled') useAppStore.getState().setInfo(info.value);
}
```

- [ ] **Step 5: Wire the app to the backend**

Overwrite `frontend/src/test/setup.ts`:

```ts
import '@testing-library/jest-dom/vitest';
import { afterEach } from 'vitest';
import { cleanup } from '@testing-library/react';
import { resetAllStores } from '../store/reset';
import { installMatchMedia, resetMatchMedia } from './matchMedia';

installMatchMedia();

afterEach(() => {
  cleanup();
  resetAllStores();
  resetMatchMedia();
});
```

Create `frontend/src/test/render.tsx`:

```tsx
import { render } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { demoSeed } from '../api/mock/fixtures';
import { createMockBackend, type MockBackend } from '../api/mock/index';
import { App } from '../app/App';
import { createAppRouter } from '../app/router';

export interface RenderAppOptions { route?: string; backend?: MockBackend }

export function renderApp({ route = '/home', backend = createMockBackend(demoSeed()) }: RenderAppOptions = {}) {
  const router = createAppRouter([route]);
  const user = userEvent.setup();
  const view = render(<App backend={backend} router={router} />);
  return { ...view, backend, router, user };
}
```

Overwrite `frontend/src/app/App.tsx`:

```tsx
import { useState, type ReactNode } from 'react';
import { RouterProvider } from 'react-router-dom';
import type { Backend } from '../api/backend';
import { BackendProvider } from '../api/BackendContext';
import type { ThemeName } from '../design/tokens';
import { useSetting } from '../store/settings';
import { createAppRouter, type AppRouter } from './router';
import { ThemeProvider } from './theme';

export interface AppProps { backend: Backend; router?: AppRouter }

function ConnectedTheme({ children }: { children: ReactNode }) {
  const [mode] = useSetting<ThemeName>('appearance.theme', 'light');
  const [followSystem] = useSetting<boolean>('appearance.follow_system', false);
  return <ThemeProvider mode={mode} followSystem={followSystem}>{children}</ThemeProvider>;
}

export function App({ backend, router }: AppProps) {
  const [r] = useState(() => router ?? createAppRouter());
  return (
    <BackendProvider backend={backend}>
      <ConnectedTheme>
        <RouterProvider router={r} />
      </ConnectedTheme>
    </BackendProvider>
  );
}
```

Overwrite `frontend/src/app/RootLayout.tsx`:

```tsx
import { useEffect, useLayoutEffect, useRef } from 'react';
import { Outlet, useNavigate } from 'react-router-dom';
import { useBackend } from '../api/BackendContext';
import { bootstrap, startEventBridge } from '../api/events';

export function RootLayout() {
  const backend = useBackend();
  const navigate = useNavigate();
  const navRef = useRef(navigate);
  useLayoutEffect(() => {
    navRef.current = navigate;
  });
  useEffect(() => {
    const stop = startEventBridge(backend, { navigate: (route) => void navRef.current(route) });
    void bootstrap(backend);
    return stop;
  }, [backend]);
  return <Outlet />;
}
```

Overwrite `frontend/src/app/ShellLayout.tsx`:

```tsx
import { Outlet } from 'react-router-dom';
import { useSetting } from '../store/settings';
import { AppShell } from './AppShell';
import { visibleNavEntries } from './features';
import { Sidebar } from './Sidebar';
import { TopBar } from './TopBar';

export function ShellLayout() {
  const [collapsed, setCollapsed] = useSetting<boolean>('appearance.sidebar_collapsed', false);
  return (
    <AppShell
      topBar={<TopBar ticker={null} />}
      sidebar={<Sidebar entries={visibleNavEntries()} collapsed={collapsed} onToggleCollapsed={() => void setCollapsed(!collapsed)} />}
    >
      <Outlet />
    </AppShell>
  );
}
```

Overwrite `frontend/src/main.tsx`:

```tsx
import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import './design/tokens.css';
import './design/base.css';
import { defaultBackend } from './api';
import { App } from './app/App';
import { applyTheme } from './app/theme';

applyTheme('light');
const root = document.getElementById('root');
if (!root) throw new Error('index.html must contain <div id="root">');
createRoot(root).render(
  <StrictMode>
    <App backend={defaultBackend()} />
  </StrictMode>,
);
```

- [ ] **Step 6: Run the suite and the type check**

Run: `cd frontend && npx vitest run && npm run typecheck`
Expected: PASS (14 test files); `tsc` exits 0 (requires P0-3's committed `frontend/wailsjs`).

- [ ] **Step 7: Commit**

```bash
git add frontend/src
git commit -m "feat(frontend): typed backend adapter, mock backend, stores and event bridge (P0-4 Task 2)" \
  -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---
