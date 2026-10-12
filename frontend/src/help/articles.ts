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
