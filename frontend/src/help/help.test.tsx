import { act, render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it } from 'vitest';
import { loggedOutSeed, unackedSeed } from '../api/mock/fixtures';
import { createMockBackend } from '../api/mock/index';
import { isRememberable } from '../app/lastRoute';
import { isRestorableRoute } from '../app/routes';
import { reloadShortcuts } from '../shortcuts/ShortcutProvider';
import { useShortcutStore } from '../shortcuts/store';
import { useToastStore } from '../store/toasts';
import { renderApp } from '../test/render';
import { ARTICLES, HELP_CATEGORIES, shortcutsArticle } from './articles';
import { InfoButton } from './InfoButton';
import { renderArticle } from './renderArticle';
import { searchArticles } from './search';

const toasts = () => useToastStore.getState().visible.map((t) => `${t.level}: ${t.message}`);

describe('FR-ACCT-010 built-in help', () => {
  it('AC1: searching "square-off" lists matching articles, offline', async () => {
    const { user } = renderApp({ route: '/help' });
    await user.type(await screen.findByRole('searchbox', { name: 'Search help' }), 'square-off');
    const results = screen.getByRole('region', { name: 'Search results' });
    expect(within(results).getByRole('link', { name: 'Square-off rules' })).toBeInTheDocument();
  });
  it('shows topic cards only for topics that have articles, and articles carry the "Updated for" stamp', async () => {
    const { user } = renderApp({ route: '/help' });
    const cards = await screen.findByRole('list', { name: 'Help topics' });
    expect(within(cards).getByRole('link', { name: /Kite setup/ })).toBeInTheDocument();
    expect(within(cards).queryByRole('link', { name: /^Scalper/ })).toBeNull();
    await user.click(within(cards).getByRole('link', { name: /Kite setup/ }));
    expect(await screen.findByRole('heading', { name: 'Create your Kite Connect app' })).toBeInTheDocument();
    expect(screen.getByText('Updated for v0.1.0')).toBeInTheDocument();
  });
  // Ruling 83-F I3: Go's real label ("Kill switch"), not the plan's "Stop all trading".
  it('AC3 + FR-SHELL-019 AC1 (help part): the shortcuts article lists every binding, including remapped ones', async () => {
    const { backend } = renderApp({ route: '/help/keyboard-shortcuts' });
    await screen.findByRole('heading', { name: 'Keyboard shortcuts' });
    await backend.settings.rebind('kill.switch', 'Ctrl+Shift+X');
    await act(() => reloadShortcuts(backend));
    expect(screen.getByText('Kill switch — Ctrl+Shift+X (Everywhere)')).toBeInTheDocument();
    const article = screen.getByRole('article');
    expect(within(article).getAllByRole('listitem')).toHaveLength(useShortcutStore.getState().actions.length);
  });
  it('offers "Open logs folder" and "Create support bundle" (FR-SHELL-018 entry points)', async () => {
    const { user, backend } = renderApp({ route: '/help' });
    await user.click(await screen.findByRole('button', { name: 'Open logs folder' }));
    await user.click(screen.getByRole('button', { name: 'Create support bundle' }));
    expect(backend.callsTo('diagnostics.openLogsFolder')).toHaveLength(1);
    expect(backend.callsTo('diagnostics.createSupportBundle')).toHaveLength(1);
  });
  it('an unknown article is "Page not found"', async () => {
    renderApp({ route: '/help/no-such-article' });
    expect(await screen.findByRole('heading', { name: 'Page not found' })).toBeInTheDocument();
  });
  // Ruling 117 I1: signed in and acknowledged, Help keeps the app frame (sidebar, top bar, global shortcuts).
  it('signed in and acknowledged: Help keeps the shell chrome, one main and a single h1', async () => {
    renderApp({ route: '/help/kite-setup' });
    const main = await screen.findByRole('main');
    expect(await within(main).findAllByRole('heading', { level: 1 })).toHaveLength(1);
    expect(screen.getAllByRole('main')).toHaveLength(1);
    expect(screen.getByRole('navigation', { name: 'Main navigation' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Account menu' })).toBeInTheDocument();
  });
  it('signed in: global shortcuts (Alt+1) still work from Help', async () => {
    const { user, router } = renderApp({ route: '/help/kite-setup' });
    await screen.findByRole('heading', { name: 'Create your Kite Connect app' });
    await waitFor(() => expect(useShortcutStore.getState().actions.length).toBeGreaterThan(0));
    await user.keyboard('{Alt>}1{/Alt}');
    await waitFor(() => expect(router.state.location.pathname).toBe('/home'));
  });
  // Ruling 108 M6 is kept: no chrome before acknowledgement or while logged out.
  it('risk not yet acknowledged: Help renders without the shell chrome, in its own main', async () => {
    renderApp({ route: '/help/kite-setup', backend: createMockBackend(unackedSeed()) });
    const main = await screen.findByRole('main');
    expect(await within(main).findByRole('heading', { name: 'Create your Kite Connect app' })).toBeInTheDocument();
    expect(within(main).getAllByRole('heading', { level: 1 })).toHaveLength(1);
    expect(screen.queryByRole('navigation', { name: 'Main navigation' })).toBeNull();
  });
  it('logged out: Help shows no shell chrome, no profile menu and no account data', async () => {
    renderApp({ route: '/help', backend: createMockBackend(loggedOutSeed()) });
    expect(await screen.findByRole('list', { name: 'Help topics' })).toBeInTheDocument();
    expect(screen.queryByRole('navigation', { name: 'Main navigation' })).toBeNull();
    expect(screen.queryByRole('button', { name: 'Account menu' })).toBeNull();
    expect(document.body).not.toHaveTextContent(/Asha|AB1234|asha@example\.com/);
  });
  it('signed in: /help is remembered as the last view (and is a route the restore accepts)', async () => {
    const { backend } = renderApp({ route: '/help/kite-setup' });
    await screen.findByRole('heading', { name: 'Create your Kite Connect app' });
    await waitFor(() => expect(backend.state.settings['ui.last_route']).toBe('"/help/kite-setup"'));
    expect(isRestorableRoute('/help')).toBe(true);
    expect(isRestorableRoute('/help/kite-setup')).toBe(true);
    expect(isRememberable('/legal/terms')).toBe(false);
  });
  // Minor 1 + 2: a new article clears the search and takes focus to its heading.
  it('opening an article from the tree clears the search; the article heading takes focus', async () => {
    const { user } = renderApp({ route: '/help/kite-setup' });
    await user.type(await screen.findByRole('searchbox', { name: 'Search help' }), 'backup');
    await user.click(within(screen.getByRole('navigation', { name: 'Help articles' })).getByRole('link', { name: 'Square-off rules' }));
    const heading = await screen.findByRole('heading', { level: 2, name: 'Square-off rules' });
    expect(screen.getByRole('searchbox', { name: 'Search help' })).toHaveValue('');
    expect(heading).toHaveFocus();
  });
  it('clicking a search result moves focus to the article heading', async () => {
    const { user } = renderApp({ route: '/help' });
    await user.type(await screen.findByRole('searchbox', { name: 'Search help' }), 'square-off');
    await user.click(within(screen.getByRole('region', { name: 'Search results' })).getByRole('link', { name: 'Square-off rules' }));
    expect(await screen.findByRole('heading', { level: 2, name: 'Square-off rules' })).toHaveFocus();
  });
  it('opening Help directly does not move focus to the article', async () => {
    renderApp({ route: '/help/kite-setup' });
    expect(await screen.findByRole('heading', { level: 2, name: 'Create your Kite Connect app' })).not.toHaveFocus();
  });
  // Ruling 117 I2: the Help support buttons use the shared support actions (Ruling 86 claim window, busy guard).
  it('Ruling 86: Create support bundle is busy while running and holds other job failures until it ends', async () => {
    const { user, backend } = renderApp({ route: '/help' });
    let release: (p: string) => void = () => {};
    backend.diagnostics.createSupportBundle = () => new Promise<string>((r) => { release = r; });
    await user.click(await screen.findByRole('button', { name: 'Create support bundle' }));
    const button = screen.getByRole('button', { name: 'Create support bundle' });
    expect(button).toBeDisabled();
    expect(button).toHaveAttribute('aria-busy', 'true');
    backend.emit('job.progress', {
      jobId: 'backup-7', kind: 'backup', stage: 'write', done: 1, total: 4, finished: true,
      error: 'The scheduled backup failed: disk full', resultPath: '',
    });
    expect(toasts()).toEqual([]);
    release('/exports/support-1.zip');
    await waitFor(() => expect(toasts()).toContain('error: The scheduled backup failed: disk full'));
    expect(toasts()).toContain('success: Support bundle saved to /exports/support-1.zip');
    expect(screen.getByRole('button', { name: 'Create support bundle' })).toBeEnabled();
    expect(screen.getByRole('button', { name: 'Create support bundle' })).not.toHaveAttribute('aria-busy');
  });
  it('a search with no match says so once (ListPage live status)', async () => {
    const { user } = renderApp({ route: '/help' });
    await user.type(await screen.findByRole('searchbox', { name: 'Search help' }), 'zzqx');
    expect(screen.getAllByText(/zzqx/)).toHaveLength(1);
    expect(screen.getByRole('status')).toHaveTextContent('No matches for “zzqx”');
  });
});

describe('help content helpers', () => {
  it('HELP_CATEGORIES are the ten FR-ACCT-010 topics and every article uses one of them', () => {
    expect(HELP_CATEGORIES).toHaveLength(10);
    for (const a of ARTICLES) expect(HELP_CATEGORIES).toContain(a.category);
    expect(new Set(ARTICLES.map((a) => a.id)).size).toBe(ARTICLES.length);
  });
  it('searchArticles needs every term, ignores case and ranks title matches first', () => {
    expect(searchArticles(ARTICLES, '   ')).toEqual([]);
    expect(searchArticles(ARTICLES, 'BACKUP restore').map((a) => a.id)).toEqual(['data-backup']);
    expect(searchArticles(ARTICLES, 'kite')[0].title.toLowerCase()).toContain('kite');
    expect(searchArticles(ARTICLES, 'square-off nonexistentterm')).toEqual([]);
  });
  it('renderArticle never injects HTML', () => {
    const { container } = render(<div>{renderArticle('<img src=x onerror=alert(1)>\n\n- **<b>x</b>**')}</div>);
    expect(container.querySelector('img')).toBeNull();
    expect(container.querySelector('b')).toBeNull();
    expect(container.querySelector('strong')).toHaveTextContent('<b>x</b>');
  });
  it('renderArticle headings default to h3 and can start at h2', () => {
    const { container, rerender } = render(<div>{renderArticle('## Part')}</div>);
    expect(container.querySelector('h3')).toHaveTextContent('Part');
    rerender(<div>{renderArticle('## Part', { headingLevel: 2 })}</div>);
    expect(container.querySelector('h2')).toHaveTextContent('Part');
  });
  it('shortcutsArticle marks an unbound action as not set', () => {
    const a = shortcutsArticle([{ id: 'scalper.buy', label: 'Scalper: buy', scope: 'view', defaultBinding: '', binding: '' }]);
    expect(a.body).toContain('- Scalper: buy — not set (Current view)');
  });
});

describe('FR-SHELL-013 AC3 / FR-ACCT-010 AC2: (i) opens the matching article in a side panel', () => {
  it('opens the Advanced conditions article and closes on Escape', async () => {
    const user = userEvent.setup();
    render(<InfoButton articleId="advanced-conditions" label="About advanced conditions" />);
    await user.click(screen.getByRole('button', { name: 'About advanced conditions' }));
    const panel = screen.getByRole('dialog', { name: 'Advanced conditions' });
    expect(panel).toHaveClass('dialog--side');
    await user.keyboard('{Escape}');
    expect(screen.queryByRole('dialog')).toBeNull();
  });
  it('does not move focus until opened, keeps one title heading and returns focus to the (i) button', async () => {
    const user = userEvent.setup();
    render(<><button type="button">Before</button><InfoButton articleId="advanced-conditions" label="About advanced conditions" /></>);
    screen.getByRole('button', { name: 'Before' }).focus();
    expect(screen.getByRole('button', { name: 'Before' })).toHaveFocus();
    const info = screen.getByRole('button', { name: 'About advanced conditions' });
    await user.click(info);
    const panel = screen.getByRole('dialog', { name: 'Advanced conditions' });
    // Minor 3: the article itself takes focus so the body scrolls with the keyboard.
    expect(within(panel).getByRole('article', { name: 'Advanced conditions' })).toHaveFocus();
    expect(within(panel).getAllByRole('heading', { name: 'Advanced conditions' })).toHaveLength(1);
    expect(within(panel).getByRole('heading', { level: 3, name: 'How they are evaluated' })).toBeInTheDocument();
    await user.click(within(panel).getByRole('button', { name: 'Close' }));
    expect(screen.queryByRole('dialog')).toBeNull();
    expect(info).toHaveFocus();
  });
  it('an unknown article id renders nothing', () => {
    const { container } = render(<InfoButton articleId="nope" label="About nothing" />);
    expect(container).toBeEmptyDOMElement();
  });
});
