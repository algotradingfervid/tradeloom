> **P0-4 Frontend Shell, Account & Data UI Implementation Plan — Task 18.** Read [`../MANIFEST.md`](../MANIFEST.md) first: its Global Constraints,
> Review Focus and Consumes/Produces apply to this task. Other tasks live beside this file in `tasks/`.

<!-- task body (verbatim from the plan) -->
### Task 18: Built-in help centre, contextual help panel and legal pages

**Files:**
- Create: `frontend/src/help/articles.ts`, `frontend/src/help/renderArticle.tsx`, `frontend/src/help/search.ts`, `frontend/src/help/HelpView.tsx`, `frontend/src/help/InfoButton.tsx`, `frontend/src/help/help.css`, `frontend/src/views/legal/LegalView.tsx`
- Modify: `frontend/src/legal/content.ts` (terms, privacy, `LEGAL_PAGES`), `frontend/src/components/Dialog.tsx` (`size: 'side'`), `frontend/src/components/components.css`, `frontend/src/app/routes.tsx` (`help/:article?`, `legal/:page`), `frontend/src/i18n/copy.ts`
- Test: `frontend/src/help/help.test.tsx`, `frontend/src/views/legal/LegalView.test.tsx`, `frontend/src/legal/clean-room.test.ts`

**Interfaces:**
- Consumes: Task 6 `useShortcutStore`, `formatBinding`; Task 15 `shortcutsCopy.scopes`; Task 5 `ListPage`, `NotFoundView`; Task 3 `Dialog`, `IconButton`, `Button`; Task 9 `riskDisclosure`, `LegalPage`; Task 2 `Backend.diagnostics.*`.
- Produces:
  - `articles.ts`: `HELP_CATEGORIES` (the ten FR-ACCT-010 categories), `type HelpCategory`, `interface HelpArticle { id; title; category; updatedFor; body }`, `ARTICLES`, `shortcutsArticle(actions: ShortcutAction[]): HelpArticle` (generated from the registry, FR-SHELL-019).
  - `renderArticle(body: string): ReactNode[]` (paragraphs, `## ` headings, `- ` lists, `**bold**`; no HTML injection).
  - `searchArticles(articles, query): HelpArticle[]` (all terms must match title or body, case-insensitive).
  - `HelpView()` at `/help/:article?`; `InfoButton({ articleId: string; label: string })` — the (i) control that opens an article in a side panel (FR-SHELL-013 AC3; placed on the Advanced conditions header by P1).
  - `legal/content.ts`: `termsOfUse`, `privacyNotice`, `LEGAL_PAGES: Record<string, LegalPage>`; `LegalView()` at `/legal/:page` (`terms`, `privacy`, `risk`).
  - `Dialog` gains `size="side"`.

- [ ] **Step 1: Write the failing tests**

Create `frontend/src/help/help.test.tsx`:

```tsx
import { act, render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it } from 'vitest';
import { reloadShortcuts } from '../shortcuts/ShortcutProvider';
import { useShortcutStore } from '../shortcuts/store';
import { renderApp } from '../test/render';
import { InfoButton } from './InfoButton';

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
  it('AC3 + FR-SHELL-019 AC1 (help part): the shortcuts article lists every binding, including remapped ones', async () => {
    const { backend } = renderApp({ route: '/help/keyboard-shortcuts' });
    await screen.findByRole('heading', { name: 'Keyboard shortcuts' });
    await backend.settings.rebind('kill.switch', 'Ctrl+Shift+X');
    await act(() => reloadShortcuts(backend));
    expect(screen.getByText('Stop all trading — Ctrl+Shift+X (Everywhere)')).toBeInTheDocument();
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
});
```

Create `frontend/src/views/legal/LegalView.test.tsx`:

```tsx
import { screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { freshSeed } from '../../api/mock/fixtures';
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
});
```

Create `frontend/src/legal/clean-room.test.ts`:

```ts
import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import { describe, expect, it } from 'vitest';
import { ARTICLES } from '../help/articles';
import { LEGAL_PAGES } from './content';

// FR-ACCT-015 AC3 + global "own copy only": headings and labels transcribed from the research screenshots
// (docs/research/FEATURES.md SHELL/ORD/ACCT blocks) must not appear in our text.
const FORBIDDEN = [
  'streak', 'terms & conditions', 'important regulatory clarification', 'privacy policy', 'notification-based trading',
  'equity trader', 'options trader', 'scalper mode', 'advance create', 'still need help', 'search for help topics',
  'browse help topics', 'remember f&o order quantity', 'push notifications', 'orderbook', 'no positions found',
  'fill all the conditions before running scan', 'strategy saved successfully', 'copy the scanner in order to set alerts',
];

const corpus = [
  readFileSync(join(process.cwd(), 'src/i18n/copy.ts'), 'utf8'),
  ...Object.values(LEGAL_PAGES).flatMap((p) => [p.title, ...p.paragraphs]),
  ...ARTICLES.flatMap((a) => [a.title, a.body]),
].join('\n').toLowerCase();

describe('clean-room text check', () => {
  it.each(FORBIDDEN)('does not contain "%s"', (phrase) => {
    expect(corpus.includes(phrase)).toBe(false);
  });
});
```

- [ ] **Step 2: Run them to verify they fail**

Run: `cd frontend && npx vitest run src/help src/views/legal src/legal`
Expected: FAIL — `Failed to resolve import "./InfoButton"`, `"../help/articles"`; `LEGAL_PAGES` is not exported.

- [ ] **Step 3: Implement help content, rendering, search, views and legal pages**

Append to `frontend/src/i18n/copy.ts`:

```ts
export const helpCopy = {
  title: 'Help',
  description: 'Guides and answers that work offline.',
  searchPlaceholder: 'Search help articles',
  results: 'Search results',
  noResults: (q: string) => `No articles match "${q}"`,
  topics: 'Help topics',
  tree: 'Help articles',
  articles: (n: number) => (n === 1 ? '1 article' : `${n} articles`),
  updatedFor: (v: string) => `Updated for v${v}`,
  support: 'Support',
  supportHint: 'If something goes wrong, the logs folder and a support bundle help explain what happened.',
  shortcutsTitle: 'Keyboard shortcuts',
  shortcutsIntro: 'Every shortcut can be changed in Settings → Shortcuts. This list always shows the current keys.',
  unbound: 'not set',
} as const;

export const legalCopy = { updatedFor: (v: string) => `Updated for v${v}` } as const;
```

Create `frontend/src/help/articles.ts`:

```ts
import type { ShortcutAction } from '../api/backend';
import { helpCopy, shortcutsCopy } from '../i18n/copy';
import { formatBinding } from '../shortcuts/registry';

export const HELP_CATEGORIES = [
  'Dashboard & charts', 'Scalper', 'Strategies', 'Scanners', 'Backtesting',
  'Deployments & orders', 'Kite setup', 'Data & backup', 'FAQ', 'Advanced conditions',
] as const;
export type HelpCategory = (typeof HELP_CATEGORIES)[number];

export interface HelpArticle { id: string; title: string; category: HelpCategory; updatedFor: string; body: string }

const V = '0.1.0';

// Our own help text. Topics without articles (Scalper, Strategies, Scanners) are hidden until their phase adds content.
export const ARTICLES: HelpArticle[] = [
  {
    id: 'home-dashboard', title: 'The Home dashboard', category: 'Dashboard & charts', updatedFor: V,
    body: `Home brings your portfolio and the market into one view.

- The **Holdings** card shows current value, investment, overall P&L and today's change. Select it to open the Holdings view.
- The main chart starts on NIFTY 50 and remembers the last instrument and timeframe you used.
- The right column scrolls on its own, so the chart stays in place.
- The status indicator in the top bar shows whether live prices are flowing, when your Kite session ends and whether the market is open.`,
  },
  {
    id: 'kite-setup', title: 'Create your Kite Connect app', category: 'Kite setup', updatedFor: V,
    body: `Tradeloom connects to Zerodha through **your own** Kite Connect app. Nothing passes through any other server.

## Steps

- Sign in to the Kite developer console and create a new app.
- Set its redirect URL to the address shown on the setup screen, for example http://127.0.0.1:7420/kite/callback.
- Enter the API key and API secret on Tradeloom's setup screen.
- Choose **Log in with Kite**; your browser opens the Kite login page.

## Plans

The paid Connect plan gives live prices, charts and historical data. With the free Personal plan Tradeloom runs in "orders and portfolio only" mode: holdings, positions and the order book work, and charts explain that a data plan is needed.

## Where the secret is kept

The API secret and the daily access token are stored in your system keyring. On desktops without a keyring you can use an encrypted file protected by a passphrase instead.`,
  },
  {
    id: 'daily-login', title: 'Logging in each day', category: 'Kite setup', updatedFor: V,
    body: `Kite access tokens end every morning at 06:00 exchange time. After that the status indicator shows **Logged out** and a banner offers **Log in**.

- Your password and TOTP are typed only on the Kite page in your browser.
- If the browser cannot return to Tradeloom, copy the address of the final page (it contains request_token=) and paste it under **Login did not come back to Tradeloom?**
- After each login Tradeloom shows its risk disclosure once.`,
  },
  {
    id: 'data-backup', title: 'Backups, restore and the data folder', category: 'Data & backup', updatedFor: V,
    body: `Tradeloom keeps everything in one folder, by default ~/.local/share/tradeloom: the app database, the candle store, logs and exports.

## Back up now

Settings → Data → **Back up now** writes one archive named tradeloom-backup-YYYYMMDD-HHMMSS.tar.zst. Include the candle store if charts should open without downloading history again. Secrets are never included.

## Restore

**Restore from a backup** shows what the archive contains and asks you to type RESTORE. Your current folder is kept beside the restored one, and Tradeloom restarts.

## Scheduled backup

Turn on **Scheduled daily backup**, choose a folder and how many backups to keep. The oldest archive is deleted after a new one is written.`,
  },
  {
    id: 'square-off', title: 'Square-off rules', category: 'Backtesting', updatedFor: V,
    body: `Intraday (MIS) positions are closed automatically near the end of the session — the square-off. Tradeloom keeps two separate square-off tables: one for backtests and an earlier, editable one for live deployments.

- Square-off times come from the market calendar, so a special session uses its own end time.
- In a backtest, a square-off exit is recorded with the trigger code SO.
- Delivery (CNC) and carry-forward (NRML) positions are never squared off.`,
  },
  {
    id: 'kite-limits', title: 'Kite limits Tradeloom respects', category: 'FAQ', updatedFor: V,
    body: `Kite sets limits on every API key. Tradeloom shares them across all features so that one feature cannot starve another.

- Historical data: 3 requests per second. Long histories download in the background with progress.
- Quotes: 1 request per second.
- Live prices: up to 3 connections of 3,000 instruments each.`,
  },
  {
    id: 'behaviour-notes', title: 'Behaviour notes', category: 'FAQ', updatedFor: V,
    body: `A few deliberate choices in how Tradeloom behaves:

- Error messages stay on screen until you close them; information messages close after 5 seconds.
- Everything you can reach with the mouse also works from the keyboard. Press Shift+F10 on a table row to see its actions.
- A missing price shows a dash (—), never 0.00.
- The Holdings card shows overall profit and loss and includes shares bought recently that have not settled yet.
- Times are always Indian Standard Time, whatever your computer's time zone.`,
  },
  {
    id: 'portfolio-views', title: 'Holdings, positions and orders', category: 'Deployments & orders', updatedFor: V,
    body: `In this version Holdings, Positions and the Order Book are read-only views of your Kite account.

- Prices update live while the market data feed is connected.
- The order book is re-read at least every 30 seconds if live updates stop.
- The Source column shows where an order came from: placed manually, or by a named deployment.`,
  },
  {
    id: 'advanced-conditions', title: 'Advanced conditions', category: 'Advanced conditions', updatedFor: V,
    body: `Advanced conditions let one row of the strategy or scanner builder compare indicators across timeframes and combine comparisons.

## How they are evaluated

- Conditions are checked when a candle closes.
- A condition on a higher timeframe uses that timeframe's last closed candle.`,
  },
];

export function shortcutsArticle(actions: ShortcutAction[]): HelpArticle {
  const lines = actions.map((a) => `- ${a.label} — ${a.binding ? formatBinding(a.binding) : helpCopy.unbound} (${shortcutsCopy.scopes[a.scope]})`);
  return {
    id: 'keyboard-shortcuts', title: helpCopy.shortcutsTitle, category: 'FAQ', updatedFor: V,
    body: [helpCopy.shortcutsIntro, lines.join('\n')].filter(Boolean).join('\n\n'),
  };
}
```

Create `frontend/src/help/renderArticle.tsx`:

```tsx
import type { ReactNode } from 'react';

function inline(text: string): ReactNode[] {
  return text.split(/(\*\*[^*]+\*\*)/g).map((part, i) =>
    part.startsWith('**') && part.endsWith('**') && part.length > 4 ? <strong key={i}>{part.slice(2, -2)}</strong> : part,
  );
}

/** Minimal, injection-free renderer: blank-line blocks, "## " headings, "- " lists, **bold**. */
export function renderArticle(body: string): ReactNode[] {
  return body
    .trim()
    .split(/\n{2,}/)
    .filter((block) => block.trim() !== '')
    .map((block, i) => {
      if (block.startsWith('## ')) return <h3 key={i}>{block.slice(3)}</h3>;
      const lines = block.split('\n');
      if (lines.every((l) => l.startsWith('- '))) {
        return <ul key={i}>{lines.map((l, j) => <li key={j}>{inline(l.slice(2))}</li>)}</ul>;
      }
      return <p key={i}>{inline(block.replace(/\n/g, ' '))}</p>;
    });
}
```

Create `frontend/src/help/search.ts`:

```ts
import type { HelpArticle } from './articles';

export function searchArticles(articles: HelpArticle[], query: string): HelpArticle[] {
  const terms = query.toLowerCase().split(/\s+/).filter(Boolean);
  if (terms.length === 0) return [];
  const matches = articles.filter((a) => {
    const text = `${a.title}\n${a.body}`.toLowerCase();
    return terms.every((t) => text.includes(t));
  });
  const inTitle = (a: HelpArticle) => terms.every((t) => a.title.toLowerCase().includes(t));
  return matches.sort((a, b) => Number(inTitle(b)) - Number(inTitle(a)));
}
```

Create `frontend/src/help/help.css`:

```css
.help { display: grid; grid-template-columns: 240px minmax(0, 1fr); gap: var(--space-6); }
.help__tree h2 { margin: var(--space-3) 0 var(--space-1); font-size: var(--fs-xs); text-transform: uppercase; letter-spacing: 0.04em; color: var(--text-muted); }
.help__tree ul, .help__cards, .help__results { margin: 0; padding: 0; list-style: none; display: flex; flex-direction: column; gap: var(--space-1); }
.help__tree a { display: block; padding: var(--space-1) var(--space-2); border-radius: var(--radius-1); color: var(--text); text-decoration: none; }
.help__tree a[aria-current="page"] { background: var(--surface-2); color: var(--accent); }
.help__cards { display: grid; grid-template-columns: repeat(auto-fill, minmax(200px, 1fr)); gap: var(--space-3); }
.help__card { display: flex; flex-direction: column; gap: var(--space-1); color: var(--text); text-decoration: none; }
.help__card:hover { background: var(--surface-2); }
.help article { display: flex; flex-direction: column; gap: var(--space-3); max-width: 760px; }
.help__support { display: flex; flex-direction: column; gap: var(--space-2); margin-top: var(--space-6); }
.legal { display: flex; flex-direction: column; gap: var(--space-3); max-width: 760px; }
```

Create `frontend/src/help/HelpView.tsx`:

```tsx
import { useMemo, useState } from 'react';
import { Link, NavLink, useParams } from 'react-router-dom';
import { useBackend } from '../api/BackendContext';
import { parseBackendError } from '../api/errors';
import { Button } from '../components/Button';
import { ListPage } from '../components/ListPage';
import { diagnosticsCopy, helpCopy as copy } from '../i18n/copy';
import { useShortcutStore } from '../shortcuts/store';
import { pushToast } from '../store/toasts';
import { NotFoundView } from '../views/NotFoundView';
import { ARTICLES, HELP_CATEGORIES, shortcutsArticle, type HelpArticle } from './articles';
import { renderArticle } from './renderArticle';
import { searchArticles } from './search';
import './help.css';

export function ArticleBody({ article }: { article: HelpArticle }) {
  return (
    <article aria-labelledby={`help-${article.id}`}>
      <p className="muted">{article.category}</p>
      <h2 id={`help-${article.id}`}>{article.title}</h2>
      <p className="muted">{copy.updatedFor(article.updatedFor)}</p>
      {renderArticle(article.body)}
    </article>
  );
}

function SupportLinks() {
  const backend = useBackend();
  const fail = (e: unknown) => pushToast({ level: 'error', message: parseBackendError(e).reason });
  const bundle = () =>
    backend.diagnostics.createSupportBundle().then((path) => pushToast({ level: 'success', message: diagnosticsCopy.bundleSaved(path) }), fail);
  return (
    <section className="help__support" aria-labelledby="help-support">
      <h2 id="help-support">{copy.support}</h2>
      <p className="muted">{copy.supportHint}</p>
      <div className="settings__actions">
        <Button onClick={() => void backend.diagnostics.openLogsFolder().catch(fail)}>{diagnosticsCopy.openLogs}</Button>
        <Button onClick={() => void bundle()}>{diagnosticsCopy.bundle}</Button>
      </div>
    </section>
  );
}

export function HelpView() {
  const { article: articleId } = useParams();
  const actions = useShortcutStore((s) => s.actions);
  const all = useMemo(() => [...ARTICLES, shortcutsArticle(actions)], [actions]);
  const [query, setQuery] = useState('');
  const categories = HELP_CATEGORIES.filter((c) => all.some((a) => a.category === c));
  const current = articleId ? all.find((a) => a.id === articleId) : undefined;
  if (articleId && !current) return <NotFoundView />;
  const results = searchArticles(all, query);

  return (
    <ListPage title={copy.title} description={copy.description} search={{ value: query, onChange: setQuery, placeholder: copy.searchPlaceholder }}>
      <div className="help">
        <nav className="help__tree" aria-label={copy.tree}>
          {categories.map((c) => (
            <div key={c}>
              <h2>{c}</h2>
              <ul>
                {all.filter((a) => a.category === c).map((a) => (
                  <li key={a.id}><NavLink to={`/help/${a.id}`}>{a.title}</NavLink></li>
                ))}
              </ul>
            </div>
          ))}
        </nav>
        <div>
          {query.trim() ? (
            <section aria-label={copy.results}>
              {results.length === 0 ? (
                <p>{copy.noResults(query.trim())}</p>
              ) : (
                <ul className="help__results">
                  {results.map((a) => (
                    <li key={a.id}><Link to={`/help/${a.id}`} onClick={() => setQuery('')}>{a.title}</Link> <span className="muted">— {a.category}</span></li>
                  ))}
                </ul>
              )}
            </section>
          ) : current ? (
            <ArticleBody article={current} />
          ) : (
            <>
              <ul className="help__cards" aria-label={copy.topics}>
                {categories.map((c) => {
                  const inCategory = all.filter((a) => a.category === c);
                  return (
                    <li key={c}>
                      <Link to={`/help/${inCategory[0].id}`} className="card help__card">
                        <strong>{c}</strong>
                        <span className="muted">{copy.articles(inCategory.length)}</span>
                      </Link>
                    </li>
                  );
                })}
              </ul>
              <SupportLinks />
            </>
          )}
        </div>
      </div>
    </ListPage>
  );
}
```

Create `frontend/src/help/InfoButton.tsx`:

```tsx
import { useState } from 'react';
import { Dialog } from '../components/Dialog';
import { IconButton } from '../components/IconButton';
import { ARTICLES } from './articles';
import { ArticleBody } from './HelpView';

/** The (i) control on section headers (FR-SHELL-013): opens the matching article in a side panel. */
export function InfoButton({ articleId, label }: { articleId: string; label: string }) {
  const [open, setOpen] = useState(false);
  const article = ARTICLES.find((a) => a.id === articleId);
  if (!article) return null;
  return (
    <>
      <IconButton label={label} icon="info" onClick={() => setOpen(true)} />
      <Dialog open={open} title={article.title} size="side" onClose={() => setOpen(false)}>
        <ArticleBody article={article} />
      </Dialog>
    </>
  );
}
```

In `frontend/src/components/Dialog.tsx` change the `size` prop type to `size?: 'sm' | 'md' | 'lg' | 'side';` and append to `frontend/src/components/components.css`:

```css
.dialog--side { width: 480px; height: 100vh; max-height: 100vh; margin-left: auto; border-radius: 0; align-self: stretch; }
```

Append to `frontend/src/legal/content.ts`:

```ts
export const termsOfUse: LegalPage = {
  title: 'Terms of use',
  updatedFor: '0.1.0',
  paragraphs: [
    'Tradeloom is a desktop application for studying the Indian equity and futures markets, testing trading ideas on historical data and following your own Zerodha Kite account. It is an educational and decision-support tool.',
    '## Your responsibility',
    'You decide every trade. Any order sent from your Kite account, including one you confirm inside Tradeloom, is your own instruction to your broker, and you bear its outcome.',
    '## No automatic trading unless you arm it',
    'Tradeloom does not send orders by itself. Automatic execution exists only where you explicitly arm it for a deployment, within limits you set, and arming ends when your Kite session ends.',
    '## No warranty',
    'Tradeloom is provided as is. Prices, indicators, backtests and alerts can be late, incomplete or wrong because of data, network or software faults. Check important information in Kite before acting on it.',
    '## Your Kite account',
    'You need your own Kite Connect app and must follow the terms of your broker and the exchanges, including the rules for trading through an API.',
    '## Words used in the app',
    [
      '- Equity Builder: where you write a strategy.',
      '- Scanner builder: where you write a scanner.',
      '- Execute: the builder action that starts a backtest or a deployment.',
      '- Backtest: a simulation of a strategy on past candles.',
      '- Deployment: a strategy running live in one execution mode — Notify & confirm, Signal-only, Paper or Armed auto.',
      '- Order ticket: the single dialog in which every order is reviewed before it is sent.',
      '- Universe: a named list of instruments for scanners.',
      '- Library: the bundled collection of ready-made strategies and scanners.',
      '- Kill switch: "Stop all trading" — cancels pending orders and disarms every deployment.',
      '- Status indicator: the top-bar signal for market data, Kite session and market phase.',
    ].join('\n'),
  ],
};

export const privacyNotice: LegalPage = {
  title: 'Privacy notice',
  updatedFor: '0.1.0',
  paragraphs: [
    'Tradeloom runs entirely on your computer. Your strategies, scanners, watchlists, settings, backtests and candle data are stored in your data folder and are not sent anywhere.',
    '## Network connections',
    'Tradeloom connects only to:',
    [
      '- Zerodha Kite (api.kite.trade and ws.kite.trade) for your account, prices and history; you log in on the Kite website in your own browser.',
      '- The Telegram Bot API, only if you turn on the Telegram bot.',
      '- The update feed you configure, only if you turn on update checks.',
      '- The IP-check address you configure, only if you turn on the static-IP check.',
    ].join('\n'),
    '## No telemetry',
    'Tradeloom collects no usage statistics, sends no crash reports and contains no analytics.',
    '## Secrets',
    'Your Kite API secret, daily access token and any Telegram bot token are kept in your system keyring or in an encrypted file protected by your passphrase. They never appear in logs, backups or support bundles.',
    '## Support bundles',
    'A support bundle is created only when you ask for one. It contains log files, settings without secrets, schema, library and calendar versions, the last 200 order-journal rows and recent deployment log excerpts. You decide whether to share it.',
  ],
};

export const LEGAL_PAGES: Record<string, LegalPage> = { terms: termsOfUse, privacy: privacyNotice, risk: riskDisclosure };
```

Create `frontend/src/views/legal/LegalView.tsx`:

```tsx
import { useParams } from 'react-router-dom';
import { ListPage } from '../../components/ListPage';
import { renderArticle } from '../../help/renderArticle';
import { legalCopy } from '../../i18n/copy';
import { LEGAL_PAGES } from '../../legal/content';
import { NotFoundView } from '../NotFoundView';
import '../../help/help.css';

export function LegalView() {
  const { page } = useParams();
  const content = page ? LEGAL_PAGES[page] : undefined;
  if (!content) return <NotFoundView />;
  return (
    <ListPage title={content.title} description={legalCopy.updatedFor(content.updatedFor)}>
      <article className="legal">{renderArticle(content.paragraphs.join('\n\n'))}</article>
    </ListPage>
  );
}
```

In `frontend/src/app/routes.tsx` add `import { HelpView } from '../help/HelpView';` and `import { LegalView } from '../views/legal/LegalView';`, and append to `shellChildren`:

```tsx
  { path: 'help/:article?', element: <HelpView /> },
  { path: 'legal/:page', element: <LegalView /> },
```

- [ ] **Step 4: Run the suite to verify it passes**

Run: `cd frontend && npx vitest run && npm run typecheck`
Expected: PASS (45 test files), `tsc` exits 0.

- [ ] **Step 5: Commit**

```bash
git add frontend/src
git commit -m "feat(frontend): offline help centre, contextual help panel and legal pages (P0-4 Task 18)" \
  -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---
