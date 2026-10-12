import type { ClockOffset, FeedState, FeedStateName, MarketStatus, SessionStatus } from '../api/backend';
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

// service.FeedStateDTO carries Unix ms (0 = none), Ruling 80.
function feedLine(state: IndicatorState, feed: FeedState): string {
  if (state === 'reconnecting') {
    return statusCopy.feed.reconnecting(feed.attempt, feed.nextRetryMs > 0 ? formatTime(feed.nextRetryMs, true) : DASH);
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
      statusCopy.lastTick(feed.lastTickMs > 0 ? formatTime(feed.lastTickMs, true) : DASH),
    ],
    canLogIn: session?.state === 'logged_out' || session?.state === 'expired',
    // Go's Reconnect only wakes connections that went offline; during the reconnecting backoff it cannot shorten the wait.
    canReconnect: session?.state === 'ready' && state !== 'no_data_plan' && state !== 'reconnecting',
  };
}
