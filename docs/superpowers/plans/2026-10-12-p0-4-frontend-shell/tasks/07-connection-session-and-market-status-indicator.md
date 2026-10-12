> **P0-4 Frontend Shell, Account & Data UI Implementation Plan — Task 7.** Read [`../MANIFEST.md`](../MANIFEST.md) first: its Global Constraints,
> Review Focus and Consumes/Produces apply to this task. Other tasks live beside this file in `tasks/`.

<!-- task body (verbatim from the plan) -->
### Task 7: Connection, session and market status indicator

**Files:**
- Create: `frontend/src/app/describeStatus.ts`, `frontend/src/app/useNow.ts`, `frontend/src/app/StatusIndicator.tsx`
- Modify: `frontend/src/app/ShellLayout.tsx` (`status` slot), `frontend/src/app/shell.css`, `frontend/src/i18n/copy.ts`
- Test: `frontend/src/app/describeStatus.test.ts`, `frontend/src/app/StatusIndicator.test.tsx`

**Interfaces:**
- Consumes: Task 2 `useStatusStore`, `useSessionStore`, `FeedState` (`stopped|connecting|live|reconnecting|offline`, RFC3339 `nextRetry`/`lastTick`), `MarketStatus` (`{exchange, phase, dayType, label, tooltip, atMs}`), `ClockOffset` (`{millis, source, warn, …}`), `SessionStatus` (`state`, `dataPlan`, `expiresAtMs`), `timeMs`, `Backend.session.reconnectFeed`; Task 3 `Tooltip`, `Dialog`, `Button`; Task 4 `pushToast`; Task 1 `formatTime`, `formatExpiry`, `formatSigned`, `DASH`.
- Produces: `type IndicatorState = FeedStateName | 'no_data_plan' | 'logged_out'`; `type IndicatorTone = 'up' | 'warn' | 'down' | 'flat'`; `interface IndicatorView { state: IndicatorState; label: string; tone: IndicatorTone; marketClosed: boolean; lines: string[]; canLogIn: boolean; canReconnect: boolean }`; `describeStatus(input: { feed; session; market; clock; now }): IndicatorView`; `useNow(intervalMs?: number): number` (app-clock ms = `Date.now() + clock.millis`); `StatusIndicator()`. "Logged out" and "No data plan" are derived from `session.state` / `session.dataPlan` (contract §D), never from `feed.state`.

- [ ] **Step 1: Write the failing tests**

Create `frontend/src/app/describeStatus.test.ts`:

```ts
import { describe, expect, it } from 'vitest';
import type { ClockOffset, FeedState, MarketStatus } from '../api/backend';
import { FIXED_NOW, iso, readySession, ZERO_TIME } from '../api/mock/fixtures';
import { describeStatus } from './describeStatus';

const open: MarketStatus = { exchange: 'NSE', phase: 'continuous', dayType: 'trading', label: 'Open', tooltip: '', atMs: FIXED_NOW };
const base = {
  feed: { state: 'live', attempt: 0, nextRetry: ZERO_TIME, lastTick: iso(FIXED_NOW - 1000), connections: 1, tokens: 3 } as FeedState,
  session: readySession(),
  market: open,
  clock: { millis: 120, source: 'ticks', samples: 40, warn: false, skewed: false, updatedAt: iso(FIXED_NOW) } as ClockOffset,
  now: FIXED_NOW,
};

describe('FR-SHELL-005 status model', () => {
  it('AC1: connected feed → green "Live"', () => {
    expect(describeStatus(base)).toMatchObject({ label: 'Live', tone: 'up', marketClosed: false, canLogIn: false, canReconnect: true });
  });
  it('AC2: reconnecting → amber, with attempt count and next retry in the details', () => {
    const v = describeStatus({ ...base, feed: { ...base.feed, state: 'reconnecting', attempt: 3, nextRetry: iso(Date.UTC(2026, 9, 12, 5, 0, 30)) } });
    expect(v).toMatchObject({ label: 'Reconnecting', tone: 'warn' });
    expect(v.lines[0]).toBe('Market data: reconnecting (attempt 3, next retry at 10:30:30)');
  });
  it('connecting → amber "Connecting"; stopped → grey "Not connected"', () => {
    expect(describeStatus({ ...base, feed: { ...base.feed, state: 'connecting' } })).toMatchObject({ label: 'Connecting', tone: 'warn' });
    expect(describeStatus({ ...base, feed: { ...base.feed, state: 'stopped' } })).toMatchObject({ label: 'Not connected', tone: 'flat' });
  });
  it('offline → red "Offline"', () => {
    expect(describeStatus({ ...base, feed: { ...base.feed, state: 'offline' } })).toMatchObject({ label: 'Offline', tone: 'down' });
  });
  it('AC3: an expired session shows "Logged out" and offers Log in, whatever feed.state says', () => {
    const v = describeStatus({ ...base, session: readySession({ state: 'expired' }) });
    expect(v).toMatchObject({ state: 'logged_out', label: 'Logged out', tone: 'flat', canLogIn: true, canReconnect: false });
    expect(v.lines[1]).toBe('Kite session expired — log in to continue');
  });
  it('FR-ACCT-007 AC1: a Personal-plan key (dataPlan "none") shows grey "No data plan"', () => {
    expect(describeStatus({ ...base, session: readySession({ dataPlan: 'none' }) })).toMatchObject({ label: 'No data plan', tone: 'flat', canReconnect: false });
  });
  it('AC4: a closed phase (holiday, weekend, after hours) shows "Market closed" whatever the feed state', () => {
    const holiday: MarketStatus = { ...open, phase: 'closed', dayType: 'holiday', label: 'Holiday' };
    expect(describeStatus({ ...base, market: holiday })).toMatchObject({ label: 'Live', marketClosed: true });
    expect(describeStatus({ ...base, feed: { ...base.feed, state: 'offline' }, market: { ...open, phase: 'closed', label: 'Closed' } }).marketClosed).toBe(true);
  });
  it('AC5: details name feed state, session expiry, market phase, clock offset and last tick', () => {
    expect(describeStatus(base).lines).toEqual([
      'Market data: live',
      'Session valid until 06:00 tomorrow',
      'Market: Open',
      'Clock offset +0.1 s (from ticks)',
      'Last tick: 10:29:59',
    ]);
  });
  it('FR-DATA-004 AC4: the offset shows its sign and source', () => {
    expect(describeStatus({ ...base, clock: { ...base.clock, millis: 3200, source: 'http-date' } }).lines[3]).toBe('Clock offset +3.2 s (from Kite server time)');
  });
  it('FR-DATA-001 AC5: an inferred non-trading day label passes through', () => {
    const v = describeStatus({ ...base, market: { ...open, phase: 'closed', dayType: 'inferred-non-trading', label: 'Non-trading day (inferred)' } });
    expect(v.lines[2]).toBe('Market: Non-trading day (inferred)');
    expect(v.marketClosed).toBe(true);
  });
});
```

Create `frontend/src/app/StatusIndicator.test.tsx`:

```tsx
import { act, screen, within } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import type { FeedState } from '../api/backend';
import { FIXED_NOW, iso, readySession, ZERO_TIME } from '../api/mock/fixtures';
import { renderApp } from '../test/render';

const indicator = (name: RegExp) => screen.findByRole('button', { name });
const feed = (state: FeedState['state'], attempt = 0): FeedState =>
  ({ state, attempt, nextRetry: state === 'reconnecting' ? iso(FIXED_NOW + attempt) : ZERO_TIME, lastTick: iso(FIXED_NOW), connections: 1, tokens: 3 });

describe('FR-SHELL-005 status indicator', () => {
  it('AC1 + AC2: Live, then Reconnecting on disconnect, then Live again', async () => {
    const { backend } = renderApp();
    expect(await indicator(/^Market data status: Live$/)).toHaveClass('status--up');
    act(() => backend.emit('feed.state', feed('reconnecting', 1)));
    expect(await indicator(/^Market data status: Reconnecting$/)).toHaveClass('status--warn');
    act(() => backend.emit('feed.state', feed('live')));
    expect(await indicator(/^Market data status: Live$/)).toBeInTheDocument();
  });
  it('AC4: shows "Market closed" next to the dot on a holiday', async () => {
    const { backend } = renderApp();
    await indicator(/Live/);
    act(() => backend.emit('market.status', { exchange: 'NSE', phase: 'closed', dayType: 'holiday', label: 'Holiday', tooltip: '', atMs: FIXED_NOW }));
    const button = await indicator(/market closed/);
    expect(within(button).getByText('Market closed')).toBeInTheDocument();
  });
  it('AC5: focusing the indicator shows feed state, session expiry and market phase', async () => {
    renderApp();
    const button = await indicator(/Live/);
    act(() => button.focus());
    const tip = screen.getByRole('tooltip');
    expect(tip).toHaveTextContent('Market data: live');
    expect(tip).toHaveTextContent('Session valid until');
    expect(tip).toHaveTextContent('Market: Open');
  });
  it('the panel offers Reconnect now; AC3: after expiry it offers Log in', async () => {
    const { backend, user, router } = renderApp();
    await user.click(await indicator(/Live/));
    const dialog = screen.getByRole('dialog', { name: 'Connection status' });
    await user.click(within(dialog).getByRole('button', { name: 'Reconnect now' }));
    expect(backend.callsTo('session.reconnectFeed')).toHaveLength(1);
    act(() => backend.emit('session.state', readySession({ state: 'expired' })));
    await user.click(within(dialog).getByRole('button', { name: 'Log in' }));
    expect(router.state.location.pathname).toBe('/login');
  });
  it('FR-DATA-004: warns once when the clock offset exceeds 1 s', async () => {
    const { backend } = renderApp();
    await indicator(/Live/);
    act(() => {
      backend.emit('clock.offset', { millis: 3200, source: 'ticks', samples: 12, warn: true, skewed: false, updatedAt: iso(FIXED_NOW) });
      backend.emit('clock.offset', { millis: 3300, source: 'ticks', samples: 13, warn: true, skewed: false, updatedAt: iso(FIXED_NOW) });
    });
    expect(screen.getAllByText(/differs from exchange time by \+3\.2 s/)).toHaveLength(1);
  });
  it('review focus: an event storm and a session expiry while the panel is open keep it open and focused', async () => {
    const { backend, user } = renderApp();
    await user.click(await indicator(/Live/));
    const dialog = screen.getByRole('dialog', { name: 'Connection status' });
    expect(within(dialog).getByRole('button', { name: 'Reconnect now' })).toHaveFocus();
    act(() => {
      for (let i = 0; i < 500; i++) backend.emit('feed.state', feed(i % 2 ? 'live' : 'reconnecting', i));
      backend.emit('session.state', readySession({ state: 'expired' }));
    });
    expect(screen.getByRole('dialog', { name: 'Connection status' })).toBe(dialog);
    expect(dialog).toContainElement(document.activeElement as HTMLElement);
    expect(within(dialog).getByRole('button', { name: 'Log in' })).toBeInTheDocument();
    expect(within(dialog).getByRole('button', { name: 'Reconnect now' })).toHaveAttribute('aria-disabled', 'true');
    expect(screen.getByRole('button', { name: /^Market data status: Logged out/ })).toBeInTheDocument();
    expect(backend.listenerCount('feed.state')).toBe(1);
    expect(backend.listenerCount('session.state')).toBe(1);
  });
});
```

- [ ] **Step 2: Run them to verify they fail**

Run: `cd frontend && npx vitest run src/app/describeStatus.test.ts src/app/StatusIndicator.test.tsx`
Expected: FAIL — `Failed to resolve import "./describeStatus"`; the component tests time out finding the indicator.

- [ ] **Step 3: Implement the status model and indicator**

Append to `frontend/src/i18n/copy.ts`:

```ts
export const statusCopy = {
  label: {
    live: 'Live', connecting: 'Connecting', reconnecting: 'Reconnecting', offline: 'Offline', stopped: 'Not connected',
    no_data_plan: 'No data plan', logged_out: 'Logged out',
  },
  button: (label: string, closed: boolean) => `Market data status: ${label}${closed ? ', market closed' : ''}`,
  marketClosed: 'Market closed',
  panelTitle: 'Connection status',
  reconnect: 'Reconnect now',
  reconnecting: 'Reconnecting to the market data feed',
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
```

Create `frontend/src/app/useNow.ts`:

```ts
import { useEffect, useState } from 'react';
import { useStatusStore } from '../store/status';

/** App-clock "now" (system time + measured exchange offset `clock.offset.millis`, FR-DATA-004), refreshed every `intervalMs`. */
export function useNow(intervalMs = 30_000): number {
  const offset = useStatusStore((s) => s.clock?.millis ?? 0);
  const [now, setNow] = useState(() => Date.now());
  useEffect(() => {
    const id = window.setInterval(() => setNow(Date.now()), intervalMs);
    return () => window.clearInterval(id);
  }, [intervalMs]);
  return now + offset;
}
```

Create `frontend/src/app/describeStatus.ts`:

```ts
import { timeMs, type ClockOffset, type FeedState, type FeedStateName, type MarketStatus, type SessionStatus } from '../api/backend';
import { DASH, formatExpiry, formatSigned, formatTime } from '../i18n/format';
import { statusCopy } from '../i18n/copy';

export type IndicatorState = FeedStateName | 'no_data_plan' | 'logged_out';
export type IndicatorTone = 'up' | 'warn' | 'down' | 'flat';

export interface IndicatorView {
  state: IndicatorState;
  label: string;
  tone: IndicatorTone;
  marketClosed: boolean;
  lines: string[];
  canLogIn: boolean;
  canReconnect: boolean;
}

export interface StatusInput { feed: FeedState; session: SessionStatus | null; market: MarketStatus | null; clock: ClockOffset | null; now: number }

const TONE: Record<IndicatorState, IndicatorTone> = {
  live: 'up', connecting: 'warn', reconnecting: 'warn', offline: 'down', stopped: 'flat', no_data_plan: 'flat', logged_out: 'flat',
};

function feedLine(state: IndicatorState, feed: FeedState): string {
  if (state === 'reconnecting') {
    const next = timeMs(feed.nextRetry);
    return statusCopy.feed.reconnecting(feed.attempt, next > 0 ? formatTime(next, true) : DASH);
  }
  return statusCopy.feed[state];
}

function sessionLine(session: SessionStatus | null, now: number): string {
  if (session?.state === 'ready') return statusCopy.sessionValid(formatExpiry(session.expiresAtMs, now));
  if (session?.state === 'expired') return statusCopy.sessionExpired;
  return statusCopy.notLoggedIn;
}

function clockLine(clock: ClockOffset | null): string {
  if (!clock || clock.source === 'none') return statusCopy.clockUnmeasured;
  return statusCopy.clock(formatSigned(clock.millis / 1000, 1), statusCopy.clockSource[clock.source]);
}

/** Session state and data plan decide "Logged out" / "No data plan"; only a ready Connect session shows the feed state. */
function indicatorState(feed: FeedState, session: SessionStatus | null): IndicatorState {
  if (!session || session.state !== 'ready') return 'logged_out';
  if (session.dataPlan === 'none') return 'no_data_plan';
  return feed.state;
}

export function describeStatus({ feed, session, market, clock, now }: StatusInput): IndicatorView {
  const state = indicatorState(feed, session);
  const lastTick = timeMs(feed.lastTick);
  return {
    state,
    label: statusCopy.label[state],
    tone: TONE[state],
    marketClosed: market?.phase === 'closed',
    lines: [
      feedLine(state, feed),
      sessionLine(session, now),
      statusCopy.market(market?.label ?? DASH),
      clockLine(clock),
      statusCopy.lastTick(lastTick > 0 ? formatTime(lastTick, true) : DASH),
    ],
    canLogIn: session?.state === 'logged_out' || session?.state === 'expired',
    canReconnect: session?.state === 'ready' && state !== 'no_data_plan',
  };
}
```

Create `frontend/src/app/StatusIndicator.tsx`:

```tsx
import { useEffect, useRef, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import type { ClockOffset } from '../api/backend';
import { useBackend } from '../api/BackendContext';
import { parseBackendError } from '../api/errors';
import { Button } from '../components/Button';
import { Dialog } from '../components/Dialog';
import { Tooltip } from '../components/Tooltip';
import { statusCopy } from '../i18n/copy';
import { formatSigned } from '../i18n/format';
import { useSessionStore } from '../store/session';
import { useStatusStore } from '../store/status';
import { pushToast } from '../store/toasts';
import { describeStatus } from './describeStatus';
import { useNow } from './useNow';

/** P0-2 sets `warn` when |offset| > 1 s; the toast is shown once per app start. */
function useClockWarning(clock: ClockOffset | null): void {
  const warned = useRef(false);
  useEffect(() => {
    if (!clock || warned.current || !clock.warn) return;
    warned.current = true;
    pushToast({ level: 'warning', message: statusCopy.clockWarning(formatSigned(clock.millis / 1000, 1)) });
  }, [clock]);
}

export function StatusIndicator() {
  const backend = useBackend();
  const navigate = useNavigate();
  const feed = useStatusStore((s) => s.feed);
  const market = useStatusStore((s) => s.market);
  const clock = useStatusStore((s) => s.clock);
  const session = useSessionStore((s) => s.session);
  const now = useNow();
  const view = describeStatus({ feed, session, market, clock, now });
  const [open, setOpen] = useState(false);
  useClockWarning(clock);

  const reconnect = () => {
    if (!view.canReconnect) return;
    backend.session.reconnectFeed().then(
      () => pushToast({ level: 'info', message: statusCopy.reconnecting }),
      (e: unknown) => pushToast({ level: 'error', message: parseBackendError(e).reason }),
    );
  };

  return (
    <>
      <Tooltip label={view.lines.join('\n')}>
        <button
          type="button"
          className={`status status--${view.tone}`}
          aria-label={statusCopy.button(view.label, view.marketClosed)}
          onClick={() => setOpen(true)}
        >
          <span className="status__dot" aria-hidden="true" />
          <span className="status__label">{view.label}</span>
          {view.marketClosed && <span className="status__closed">{statusCopy.marketClosed}</span>}
        </button>
      </Tooltip>
      <Dialog
        open={open}
        title={statusCopy.panelTitle}
        size="sm"
        onClose={() => setOpen(false)}
        footer={
          <>
            {/* aria-disabled keeps the button focusable so focus survives state changes (review focus #3) */}
            <Button onClick={reconnect} aria-disabled={!view.canReconnect}>{statusCopy.reconnect}</Button>
            {view.canLogIn && (
              <Button variant="primary" onClick={() => { setOpen(false); void navigate('/login'); }}>{statusCopy.logIn}</Button>
            )}
          </>
        }
      >
        <ul className="status-panel">
          {view.lines.map((line) => <li key={line}>{line}</li>)}
        </ul>
      </Dialog>
    </>
  );
}
```

Append to `frontend/src/app/shell.css`:

```css
.status {
  display: inline-flex; align-items: center; gap: var(--space-2); padding: var(--space-1) var(--space-3);
  border: 1px solid var(--border); border-radius: var(--radius-pill); background: var(--bg); cursor: pointer; white-space: nowrap;
}
.status__dot { width: 10px; height: 10px; border-radius: 50%; background: var(--flat); transition: background-color var(--motion-colour); }
.status--up .status__dot { background: var(--up); }
.status--warn .status__dot { background: var(--warn); }
.status--down .status__dot { background: var(--down); }
.status__closed { color: var(--text-muted); font-size: var(--fs-xs); }
.status-panel { margin: 0; padding-left: var(--space-4); display: flex; flex-direction: column; gap: var(--space-1); }
[aria-disabled="true"] { opacity: 0.55; cursor: not-allowed; }
```

In `frontend/src/app/ShellLayout.tsx` add `import { StatusIndicator } from './StatusIndicator';` and change the top bar to `topBar={<TopBar ticker={null} status={<StatusIndicator />} />}`.

- [ ] **Step 4: Run the suite to verify it passes**

Run: `cd frontend && npx vitest run && npm run typecheck`
Expected: PASS (27 test files), `tsc` exits 0.

- [ ] **Step 5: Commit**

```bash
git add frontend/src
git commit -m "feat(frontend): labelled feed/session/market status indicator with details panel (P0-4 Task 7)" \
  -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---
