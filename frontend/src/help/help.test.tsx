import { act, render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it } from 'vitest';
import { reloadShortcuts } from '../shortcuts/ShortcutProvider';
import { useShortcutStore } from '../shortcuts/store';
import { renderApp } from '../test/render';
import { ARTICLES, HELP_CATEGORIES, shortcutsArticle } from './articles';
import { InfoButton } from './InfoButton';
import { renderArticle } from './renderArticle';
import { searchArticles } from './search';

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
  // Ruling 108 M6: help sits in rootChildren, outside the shell chrome, inside its own main landmark.
  it('renders outside the shell, in a main landmark, with a single h1', async () => {
    renderApp({ route: '/help/kite-setup' });
    const main = await screen.findByRole('main');
    expect(within(main).getAllByRole('heading', { level: 1 })).toHaveLength(1);
    expect(screen.queryByRole('navigation', { name: 'Main navigation' })).toBeNull();
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
    expect(panel).toContainElement(document.activeElement as HTMLElement);
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
