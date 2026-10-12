import { screen, waitFor, within } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { freshSeed, unackedSeed } from '../../api/mock/fixtures';
import { createMockBackend } from '../../api/mock/index';
import { renderApp } from '../../test/render';

describe('FR-ACCT-015 legal pages', () => {
  it('AC2: Terms of use and Privacy notice render offline with their stamp', async () => {
    renderApp({ route: '/legal/terms' });
    expect(await screen.findByRole('heading', { level: 1, name: 'Terms of use' })).toBeInTheDocument();
    expect(screen.getByText('Updated for v0.1.0')).toBeInTheDocument();
    expect(screen.getByText(/Kill switch:/)).toBeInTheDocument();
  });
  it('the privacy notice names the only outbound hosts', async () => {
    renderApp({ route: '/legal/privacy' });
    expect(await screen.findByRole('heading', { level: 1, name: 'Privacy notice' })).toBeInTheDocument();
    expect(screen.getByText(/api\.kite\.trade and ws\.kite\.trade/)).toBeInTheDocument();
    expect(screen.getByText(/no usage statistics/)).toBeInTheDocument();
  });
  it('AC2 (navigation): reachable from Settings → About; the back arrow returns there', async () => {
    const { user } = renderApp({ route: '/settings/about' });
    await user.click(await screen.findByRole('link', { name: 'Privacy notice' }));
    await screen.findByRole('heading', { level: 1, name: 'Privacy notice' });
    await user.click(screen.getByRole('button', { name: 'Back' }));
    expect(await screen.findByRole('heading', { level: 1, name: 'About' })).toBeInTheDocument();
  });
  it('the risk disclosure is reachable as a page (FR-SHELL-020 "also reachable from Settings → About")', async () => {
    renderApp({ route: '/legal/risk' });
    expect(await screen.findByRole('heading', { level: 1, name: 'Risk disclosure' })).toBeInTheDocument();
  });
  it('is readable before Kite is set up (linked from the risk screen)', async () => {
    renderApp({ route: '/legal/terms', backend: createMockBackend(freshSeed()) });
    expect(await screen.findByRole('heading', { level: 1, name: 'Terms of use' })).toBeInTheDocument();
  });
  it('an unknown legal page is "Page not found"', async () => {
    renderApp({ route: '/legal/cookies' });
    expect(await screen.findByRole('heading', { name: 'Page not found' })).toBeInTheDocument();
  });
  it('a prototype key is not a legal page', async () => {
    renderApp({ route: '/legal/constructor' });
    expect(await screen.findByRole('heading', { name: 'Page not found' })).toBeInTheDocument();
  });
  // Ruling 108 M6: the risk screen's "Terms of use" link works before acknowledgement, without the shell chrome,
  // and Back returns to the risk screen.
  it('opens from the risk screen before acknowledgement without the shell, and Back returns to it', async () => {
    const { user, router } = renderApp({ route: '/', backend: createMockBackend(unackedSeed()) });
    await screen.findByRole('heading', { name: 'Before you start' });
    await user.click(screen.getByRole('link', { name: 'Terms of use' }));
    expect(await screen.findByRole('heading', { level: 1, name: 'Terms of use' })).toBeInTheDocument();
    expect(router.state.location.pathname).toBe('/legal/terms');
    expect(screen.queryByRole('navigation', { name: 'Main navigation' })).toBeNull();
    await user.click(screen.getByRole('button', { name: 'Back' }));
    await waitFor(() => expect(router.state.location.pathname).toBe('/risk'));
    expect(await screen.findByRole('heading', { name: 'Before you start' })).toBeInTheDocument();
  });
  it('section headings sit directly under the page h1 (h2, no skipped level)', async () => {
    renderApp({ route: '/legal/privacy' });
    const main = await screen.findByRole('main');
    await within(main).findByRole('heading', { level: 1, name: 'Privacy notice' });
    expect(within(main).getByRole('heading', { level: 2, name: 'No telemetry' })).toBeInTheDocument();
    expect(within(main).queryAllByRole('heading', { level: 3 })).toHaveLength(0);
  });
});
