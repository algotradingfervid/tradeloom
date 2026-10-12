import { act, screen, waitFor, within } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import type { FeedState } from '../api/backend';
import { createMockBackend } from '../api/mock/index';
import { demoSeed, FIXED_NOW, readySession } from '../api/mock/fixtures';
import { renderApp } from '../test/render';

const indicator = (name: RegExp) => screen.findByRole('button', { name });
const feed = (state: FeedState['state'], attempt = 0): FeedState =>
  ({ state, attempt, nextRetryMs: state === 'reconnecting' ? FIXED_NOW + attempt : 0, lastTickMs: FIXED_NOW, connections: 1, tokens: 3 });

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
  it('carry-forward: a refused reconnect shows the refusal and never claims success', async () => {
    const { backend, user } = renderApp();
    backend.failNext('session.reconnectFeed', 'The market feed is not available in this build.', 'unavailable');
    await user.click(await indicator(/Live/));
    const dialog = screen.getByRole('dialog', { name: 'Connection status' });
    await user.click(within(dialog).getByRole('button', { name: 'Reconnect now' }));
    expect(await screen.findByText('The market feed is not available in this build.')).toBeInTheDocument();
    expect(screen.queryByText(/Reconnect requested/)).not.toBeInTheDocument();
  });
  it('carry-forward: while reconnecting, Reconnect now is unavailable and the panel says it cannot shorten the wait', async () => {
    const { backend, user } = renderApp();
    await user.click(await indicator(/Live/));
    act(() => backend.emit('feed.state', feed('reconnecting', 2)));
    const dialog = screen.getByRole('dialog', { name: 'Connection status' });
    const button = within(dialog).getByRole('button', { name: 'Reconnect now' });
    expect(button).toHaveAttribute('aria-disabled', 'true');
    expect(dialog).toHaveTextContent('already retrying');
    await user.click(button);
    expect(backend.callsTo('session.reconnectFeed')).toHaveLength(0);
  });
  it('FR-DATA-004: warns once when the clock offset exceeds 1 s', async () => {
    const { backend } = renderApp();
    await indicator(/Live/);
    act(() => {
      backend.emit('clock.offset', { millis: 3200, source: 'ticks', samples: 12, warn: true, skewed: false, updatedAtMs: FIXED_NOW });
      backend.emit('clock.offset', { millis: 3300, source: 'ticks', samples: 13, warn: true, skewed: false, updatedAtMs: FIXED_NOW });
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

describe('boot-failure notice (carry-forward: AppShell banner slot)', () => {
  it('renders inside the shell content area, not above the 100vh shell, and Retry still reloads', async () => {
    const backend = createMockBackend(demoSeed());
    backend.failNext('settings.all', 'database is locked');
    const { user } = renderApp({ backend });
    const alert = await screen.findByRole('alert');
    expect(document.getElementById('main-content')).toContainElement(alert);
    await user.click(within(alert).getByRole('button', { name: 'Retry' }));
    await waitFor(() => expect(screen.queryByRole('alert')).not.toBeInTheDocument());
    expect(backend.callsTo('settings.all')).toHaveLength(2);
  });
});
