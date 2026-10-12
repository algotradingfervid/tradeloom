# Tradeloom — Requirements

Status: v2.2 — APPROVED by user 2026-10-12 (with ROADMAP.md) — reviews applied, scope set 2026-10-12.
Date: 2026-10-12.

| Version | Date | Change |
|---|---|---|
| v1 (DRAFT) | 2026-10-11 | First complete pass: 293 FRs generated from `docs/research/FEATURES.md` (part files in `requirements/fr/`, now history). |
| v2 | 2026-10-12 | Product named **Tradeloom**. Scope set to equity + futures (options, MCX, currency out of scope → §10). All 49 findings of `REVIEW-1-completeness.md` and all items of `REVIEW-2-live-sweep.md` applied. New module DATA; new FRs for update, logging, shortcuts, backup, calendar, clock, candle store, Kite client, scanner schema, capital used, post-closing orders, live square-off, secret fallback, Telegram alerts, legal pages, risk disclosure, price alerts. Scalper rewritten for equity/futures. §7–§9 regenerated; §10 added. Change log: `CHANGELOG-v2.md`. |

| v2.2 | 2026-10-12 | Indicator source switched from go-talib to the in-house verified TA-Lib v0.8.1 port `tradeloom/ta` (user decision). User approved v2.1 + ROADMAP; closed OQ-09 (Kite rejections only) and OQ-45 (kill switch keeps stops). |
| v2.1 | 2026-10-12 | Added FR module TG (42 FRs, two-way Telegram bot modelled on freqtrade; research `areas/08-freqtrade-telegram.md`); FR-ACCT-014 reduced to an entry point; FR-EQB-038 amended: indicators via TA-Lib port (see v2.2); OQ-45…49 added. |

This file is the single source of truth. The `fr/*.md` part files are v1 history and are not maintained.

## 1. Purpose and scope

Build **Tradeloom**: a local, single-user Linux desktop application (Go + Wails v2, React UI, Zerodha Kite Connect via
`github.com/zerodha/gokiteconnect/v4`) that reproduces the Streak web platform's workflows for **cash equities and futures** on
top of the user's own Kite Connect API key, so the user is not bound by Streak's hosted limits (deployment caps, backtest caps,
lookback windows, scanner caps) and keeps strategies, data and results on their own machine. Binary `tradeloom`; data directory
`~/.local/share/tradeloom`; export files `.tlstrategy.json` / `.tlscanner.json`. "Streak" appears in this document only when
referring to the reference product or the research under `docs/research/`.

### 1.1 In scope (user decision 2026-10-12)

- **Tradable, backtestable and scannable instruments:** NSE and BSE cash equities, and NSE/BSE index and stock **futures**
  (segments `NFO-FUT`, `BFO-FUT`).
- **Indices** (NIFTY 50, NIFTY BANK, SENSEX, BANKEX, …) are usable as chart symbols, as scanner universes/constituent sources
  and as instruments in strategy conditions (signal-only), but are **not tradable**: every B/S control is disabled for them.
- **Every Streak workflow that applies to those instruments** (catalogue `docs/research/FEATURES.md`): shell, home dashboard,
  charting, Technicals and Advance/Decline analytics, watchlists and universes, account/settings, equity strategy builder,
  Discover library (strategies and scanners), My Strategies/My Scanners, backtesting, deployment (notification-based trading),
  scanners, Scalper (adapted to equity/futures, §1.3), orders/positions/holdings.
- **Enhancements beyond Streak (in scope):**
  1. **Paper trading** — simulated fills for deployments (FR-DEP-017).
  2. **Opt-in auto-execution** — orders placed automatically only with a registered static IP, explicit per-session arming
     and mandatory risk limits (FR-DEP-018; NFR-05, C-04, C-05).
  3. **GTT-protected exits** — SL/target mirrored to Kite GTT so exits survive app closure (FR-DEP-009).
  4. **Telegram bot alerts** — the user's own bot token and chat id, stored in the keyring, off by default, per-alert-type
     filter (FR-ACCT-014).
  Smaller enhancements are marked "**Enhancement (beyond Streak)**" in their FR and listed in §7.

### 1.2 Out of scope (deferred — see §10)

- **All options features:** the options strategy builder (OPT module), option legs/strike/expiry selection, payoff graph and
  table, options pricing/Greeks engine, option chain, OI, PCR and ATM IV/Straddle analytics tabs, option universes and baskets,
  CALL/PUT/strike filters, option templates, options deployment, options backtesting, options categories in libraries and lists.
- **MCX (commodities)** and **currency derivatives** (Streak excludes them too).
- A mobile app, server-side execution, public URL sharing, distribution to other traders (SEBI/NSE algo-provider rules —
  unverified, needs legal review before any distribution).

The full text of every FR removed from §6 for these reasons is preserved in §10 so it can be revived without re-research.

### 1.3 Scalper adaptation

The Scalper stays as a fast multi-chart trading workspace for stocks, futures and indices (index as reference only): charts,
chart trading, docked watchlist, positions/orders panel, collapse, quantity presets and opt-in one-click trading, and basket
orders made of equity/futures legs. The CE/PE toggle, option-chain drawer, option templates and payoff are dropped (§10).

### 1.4 Principles

- **Clean-room.** Requirements describe behaviour observed in the UI and documented publicly. No Streak source code, bundles,
  API contracts, branding, logos, illustrations or copy are reused. Names of generic concepts (indicator names, "strategy
  cycle", trigger codes) are industry terms and may be used.
- **Personal use.** One user, their own Kite API key and static IP.
- **Documented parity choices.** Where Streak's rule is documented (e.g. comparator semantics, A06 §1) Tradeloom follows the
  documentation; where a Streak behaviour cannot be reproduced locally, the FR states the closest achievable behaviour and the
  reason (§4) and §7 lists it.

## 2. Users and goals

- Primary user: an individual retail trader on Zerodha who builds rule-based equity and futures strategies, backtests them,
  scans the market, receives signals and (optionally) places orders.
- Goals: (G1) parity with Streak's equity/futures workflows; (G2) no artificial usage caps beyond Kite/SEBI limits; (G3) data and
  strategies stored locally, with backup/restore; (G4) transparent, reproducible backtests with documented assumptions;
  (G5) safe order handling — never place an order the user did not intend.

## 3. Definitions and glossary

### 3.1 Engine and market terms

| Term | Meaning |
|---|---|
| Base timeframe (base TF) | The strategy/scanner chart timeframe on whose candle closes evaluation happens |
| MTF condition | A condition evaluated on a higher timeframe's last closed candle (FR-BT-034) |
| Strategy cycle | Max number of entries per trading day (MIS) per instrument; "–" = unlimited |
| Trigger codes | EN entry, EX exit condition, SL stop loss, TP target, TSL trailing SL, SO square-off, EXP expiry (FR-BT-043) |
| MIS / CNC / NRML | Kite products: intraday, delivery, F&O carry-forward |
| Trading day, session, phase | As defined by the market calendar service (FR-DATA-001): pre-open, continuous (09:15–15:30 on a regular day), closing (15:30–15:40), post-closing (15:40–16:00, equity cash only), special session |
| App clock | System clock + measured offset to exchange time (FR-DATA-004); every schedule uses it |
| Empty row | A condition row with no LHS operand; dropped silently (INFO toast) on Save/Execute/Run Scan |
| Incomplete row | A condition row with an LHS but a missing comparator or RHS, or an invalid parameter; blocks Execute and Run Scan with an ERROR (FR-EQB-008, FR-SCAN-004, FR-SHELL-009) |
| Signal-only instrument | An index: usable in conditions, charts and universes, never orderable |
| Reference machine | See §5 |

### 3.2 UI glossary (one name per concept, used in every FR and in the UI)

| Term | Meaning (not to be called anything else) |
|---|---|
| **Equity Builder** | The strategy builder view (sidebar entry). "Strategy" always means an equity/futures strategy |
| **Scanner builder** | The scanner editing view (sidebar entry "New Scanner") |
| **Execute** | The builder footer button that opens the two-step **execution panel**: step 1 *Choose execution type* (Deploy / Backtest), step 2 the parameters (FR-EQB-032) |
| **Backtest** | The execution type that runs the simulation (FR-BT-001); its results open in the **backtest results view** |
| **Deploy** / **Deployment** | The execution type that takes a strategy live, and the resulting live object (FR-DEP-001). Replaces Streak's "Notification Based Trading" label; also the footer action on library and My Strategies items |
| **Execution mode** | One of **Notify & confirm** (parity with Streak's notification-based trading), **Signal-only**, **Paper**, **Armed auto** (FR-DEP-003) |
| **Live Strategies** | The sidebar entry and view listing deployments (FR-DEP-011) |
| **Deployment instrument (DI)** | One instrument inside a deployment, with its own state (FR-DEP-004) |
| **Order ticket** | The single order entry dialog (FR-ORD-004). Opened in **review mode** when prefilled from a deployment alert (FR-DEP-007). There is no separate "order window", "compact confirmation" or "order review" dialog |
| **Buy/Sell buttons** | Chart overlay buttons (FR-CHART-012) and instrument-row **B / S** buttons (FR-ORD-012); both open the order ticket |
| **Instrument picker** | The shared multi-select dialog "Add instruments" (FR-WATCH-007); its single-select variant is the **symbol search** (FR-CHART-005) |
| **Universe** | A named instrument set for scanners (FR-WATCH-009); **custom basket** = user-defined universe (FR-WATCH-012) |
| **Library** | The bundled, versioned catalogue of ready-made strategies, scanners and suggestions (Discover views); "prebuilt" is not used |
| **Alerts** | Live scanner notifications (FR-SCAN-009); **Scanner Alerts** is the sidebar entry/view (FR-SCAN-011) |
| **Notifications** | The inbox view behind the top-bar bell (FR-DEP-015) |
| **Settings** | The settings view (FR-ACCT-012); **Profile** is the Kite account view (FR-ACCT-001) |
| **Kill switch** | "Stop all trading" (FR-ORD-009) |
| **Status indicator** | Top-bar connection/session/market indicator (FR-SHELL-005) |

## 4. Platform constraints (from `docs/research/areas/07-platform-kite-wails.md`)

| ID | Constraint | Consequence for requirements |
|---|---|---|
| C-01 | Kite Connect paid plan (₹500/month/key) is needed for live + historical data | App requires a Connect key for all data features; a degraded "orders + portfolio only" mode works on the free Personal plan |
| C-02 | Access token expires 06:00 daily; login is manual (request_token → access_token) | Daily login flow in-app (system browser + local redirect); session status visible at all times; arming of auto-execution lapses with the session |
| C-03 | No server-side runtime — app must be running for scanners, signals, deployments | Deployments run only while the app is open; on start the app reconciles missed signals; protective exits can be mirrored to Kite GTT so they survive app closure |
| C-04 | Order placement requires a registered static IP (SEBI 2026) | Order-placing features are gated behind a "trading enabled" setting with an IP check; signal-only and paper modes always available |
| C-05 | ≤10 orders/sec/exchange without algo registration; 400/min, 5000/day, 25 modifications/order | Central order throttle with queue; hard cap below limits |
| C-06 | Market orders need `market_protection` | Order layer always sets market protection |
| C-07 | WebSocket: 3 connections × 3000 instruments | Subscription manager with prioritisation |
| C-08 | Historical API: 3 req/s, per-interval max windows, intraday from ~2015 | Local candle store + resumable incremental backfill with progress UI (FR-DATA-002/003) |
| C-09 | Index constituents: free lists are current-only, scraping NSE is prohibited; shipping copies of provider CSVs is storage in a retrieval system | Universes are user-imported/editable lists; the app ships **empty** lists with a one-click import flow (REVIEW-1 §4 "§9 #14"); survivorship bias documented |
| C-10 | Accurate charges via Kite charges/margins APIs | Backtest charges model uses a local fee table validated against Kite's virtual contract note; live margin checks via basket margin API |
| C-11 | Wails v2 Linux build needs CGO + WebKit2GTK 4.1, built on Linux | CI/Docker build; .deb + AppImage packaging via nfpm/appimagetool |
| C-12 | Charting: Streak uses the TradingView Charting Library (licensed) | Use an openly licensed charting library (e.g. TradingView lightweight-charts, Apache-2.0) and implement drawings/indicators ourselves, or obtain a Charting Library licence — decision OQ-01 |
| C-13 | Wails v2 is single-window and has no system-tray API (A07 §D; tray only in v3 beta) | Full-view chart is an in-app view; "keep running in tray" is UNVERIFIED (FR-DEP-020, OQ-38) |
| C-14 | Kite has no exchange-calendar endpoint and no server clock endpoint | Market calendar is a shipped + user-maintained list (FR-DATA-001); the app clock is derived from tick/HTTP timestamps (FR-DATA-004) |

## 5. Non-functional requirements

| ID | Requirement |
|---|---|
| NFR-01 Platform | Runs on Ubuntu 22.04+/Debian 12+/Fedora 39+ x86_64 (WebKit2GTK 4.1); also builds on macOS for development |
| NFR-02 Packaging | Distributed as .deb and AppImage; single binary `tradeloom` + local data directory `~/.local/share/tradeloom` (layout in FR-DATA-005) |
| NFR-03 Storage | Strategies, scanners, watchlists, settings, backtest results in SQLite; candles in the candle store (FR-DATA-002); schema migrations versioned and run per FR-DATA-005 |
| NFR-04 Security | Kite API secret, access token and Telegram bot token stored in the OS keyring (libsecret) or the encrypted-file fallback of FR-ACCT-013, never in plain files or logs (redaction per FR-SHELL-018); no telemetry; the only outbound hosts are Kite and, when the user enables it, the Telegram Bot API and the optional update feed / IP-echo endpoint |
| NFR-05 Safety | No order is ever sent without (a) trading enabled, (b) explicit user confirmation (in-app, or an authorized single-use Telegram confirmation per FR-TG-020/021) or an explicitly armed auto mode (FR-DEP-018), (c) passing risk checks (qty, price band, margin, throttle); kill switch cancels pending and disarms all deployments |
| NFR-06 Correctness | Indicators verified against reference implementations with golden test vectors; backtest engine rules (fills, SL/TP order, square-off) covered by deterministic tests |
| NFR-07 Performance | Backtest of 50 instruments × 90 days × 15-min completes in < 10 s on the reference machine with cached data; UI stays responsive (engine off the UI thread; input latency < 100 ms) |
| NFR-08 Latency and time integrity | Live signals evaluated < 2 s after candle close (Streak: 8–14 s); all schedules use the app clock (FR-DATA-004); clock skew > 5 s suspends live evaluation |
| NFR-09 Reliability | Websocket auto-reconnect with resubscribe; backfill and deployments resume after crash/restart; all order actions journaled; backup/restore (FR-DATA-005) |
| NFR-10 Observability | Structured local logs with redaction and rotation (FR-SHELL-018), per-deployment event log, order audit trail viewable in-app |
| NFR-11 UX | Desktop-first layout ≥1280 px, keyboard navigation for all hover-only actions (shortcut registry FR-SHELL-019), light/dark theme, Indian number/date formats (₹, lakh/crore, DD-MM-YYYY) |
| NFR-12 Data integrity | Candles stored raw (unadjusted, like Streak) with an option to apply corporate-action adjustment later; every backtest records data range, assumptions, calendar version and app version for reproducibility |
| NFR-13 Limits | No artificial caps on strategies, scanners, deployments or backtests; only Kite/SEBI limits apply, surfaced clearly when hit |
| NFR-14 Offline | Strategy editing, scanners and backtests on cached data work without network; live features show a clear offline state |
| NFR-15 Scope guard | No option, MCX or currency instrument can be charted, added to a watchlist, strategy, scanner, universe or order anywhere; the instrument master (FR-WATCH-008) excludes them at import |

**Reference machine (used by every performance acceptance criterion):** x86_64 laptop, 4 cores / 8 threads at ≥ 2.5 GHz base
clock, 16 GB RAM, NVMe SSD, Ubuntu 24.04, 1920×1080 display, warm OS file cache, no other load. "Reference laptop" in older
text means this machine.

## 6. Functional requirements

265 FRs in scope (SHELL 20 · HOME 8 · CHART 17 · ANLY 8 · WATCH 14 · ACCT 15 · DATA 5 · EQB 42 · DISC 19 · MYS 8 · BT 46 · DEP 21 · SCAN 16 · SCALP 10 · ORD 16); 72 entries out of scope in §10. Every feature ID of `docs/research/FEATURES.md` maps to ≥1 FR in §6 or §10 (§8).

### 6.0a Conventions (all FRs)

- "The system" = Tradeloom (Go + Wails v2, React UI, Kite Connect via `github.com/zerodha/gokiteconnect/v4`). Kite dependencies
  are named by REST endpoint or gokiteconnect method (e.g. `GetHistoricalData`, `PlaceOrder`, `GetBasketMargins`,
  `GetOrderCharges`, `PlaceGTT`, ticker `OnOrderUpdate`); the wrapper method used is a design decision.
- Every FR has the fields **Source features**, **Requirement**, **Acceptance criteria**, **Data/Kite dependency**,
  **Deviation from Streak**, **Priority hint** (Core / Important / Nice-to-have). Every FR traces to ≥1 feature ID.
- All UI copy in quotes is **our own** wording and is illustrative; final copy is set in the UX spec. No Streak branding or text
  is reused. UI terms follow the glossary in §3.2.
- Times are exchange time (IST) on the app clock (FR-DATA-004). Session times are never literals: they come from the market
  calendar (FR-DATA-001); "09:15", "15:30", "15:20", "15:21" etc. in FR text denote the regular-day values of that calendar.
  Dates are shown DD-MM-YYYY (compact) or `09 Oct 2026` (long) per FR-SHELL-016; periods always use the long style.
- Monetary examples use ₹ and are computed at full precision, displayed to 2 dp; acceptance tolerance ±0.01 unless stated.
- "Library" = the app's own bundled, versioned catalogue of ready-made strategies, scanners and suggestions, authored by us from
  generic, well-known trading setups. The FEATURES.md lists are coverage references, not content to copy.
- "**Enhancement (beyond Streak)**" marks behaviour Streak does not have; such FRs or clauses are separable so phasing can
  drop or defer them independently. "Deviation from Streak" records where behaviour intentionally differs (platform constraint
  C-xx, NFR, a §3.2 bug not replicated, or a scope decision) and feeds §7.
- Where FEATURES §6 lists a conflict, the "chosen interpretation" of that table is used and the conflict number is cited.
- Acceptance criteria use Given/When/Then and are written to be automatable (Go unit/integration tests, or UI tests via the
  Wails frontend). Performance criteria are measured on the reference machine (§5).
- Feature IDs (e.g. `BT-026`, `SHELL-007`) refer to `docs/research/FEATURES.md`; FR IDs (`FR-<MODULE>-NNN`) refer to this
  document. No FR in §6 depends on an FR that lives only in §10 (verified by script, `CHANGELOG-v2.md`); where a §6 FR records
  that removed text lives in §10, it cites the entry as `§10 <MODULE>-NNN[-opt]` **without** the `FR-` prefix.

### 6.0b Engine glossary (used by BT and DEP)

| Term | Meaning |
|---|---|
| Signal candle | The completed base-TF candle on whose close a condition evaluated to true |
| Fill candle | The candle immediately after the signal candle; fills occur at its open |
| Position | At most one open position per (strategy, instrument) at a time; direction = strategy side (Buy → long, Sell → short) |
| Capital C | Backtest "Initial Capital", applied per instrument (FR-BT-005); combined figures use C × n (n = instruments) |
| Capital used | Largest notional ever deployed per instrument, Σ over instruments for Combined (FR-BT-046) |
| Realised P&L R | Sum of gross P&L of all closed round trips of that instrument before the current trade |
| Path | Assumed intra-candle price sequence: long O→L→H→C, short O→H→L→C |
| Backtest SO table | Per-instrument-class square-off times used by the simulation (FR-BT-031) |
| Live SO table | Separate, earlier, user-editable square-off times used by deployments (FR-DEP-021) |

---

## FR module SHELL — Global shell

### FR-SHELL-001 Application window layout
- **Source features:** SHELL-001
- **Requirement:** The system shall render every view inside one persistent frame consisting of (a) a top bar, (b) a
  left navigation sidebar showing an icon and a text label per entry, and (c) a content area filling the remaining space.
  The top bar shall contain, left to right: the index ticker strip (FR-SHELL-004), the connection/session/market status
  indicator (FR-SHELL-005), and right-aligned quick links Order Book, Positions, Notifications and the profile avatar
  (FR-SHELL-006, FR-SHELL-010). The active top-bar link shall be visually highlighted. The minimum supported window
  size is 1280×720 px (NFR-11); the frame shall have no horizontal scrollbar at ≥1280 px. Views with a multi-column
  layout (e.g. dashboard right column) shall scroll each column independently of the frame.
- **Acceptance criteria:**
  1. Given any view, when it is displayed, then the top bar, sidebar and content area are all visible and the sidebar
     shows 8 navigation entries (FR-SHELL-002).
  2. Given the window is resized to 1280×720, then no horizontal scrollbar appears on the frame and each of the 3 ticker pills, the status indicator, the Order Book and Positions links, the bell and the avatar is fully
     visible (bounding boxes inside the window, no clipping).
  3. Given the Order Book view is open, then the Order Book link in the top bar is shown in the active style and no other
     top-bar link is.
  4. Given the dashboard, when the right column is scrolled, then the chart in the left column does not move.
- **Data/Kite dependency:** local only (Wails window + React frontend).
- **Deviation from Streak:** Browser routes become in-app views; Streak's ticker hiding below the `lg` breakpoint is
  replaced by FR-SHELL-004 rule. Minimum width 1280 px per NFR-11.
- **Priority hint:** Core

### FR-SHELL-002 Sidebar navigation
- **Source features:** SHELL-001, SHELL-005
- **Requirement:** The sidebar shall contain exactly these entries, in this order, each with an icon, a text label and a
  keyboard-focusable target: Home (dashboard), Scalper, Strategies (library, FR-DISC-001), Equity Builder (new strategy draft,
  FR-EQB-001), Live Strategies (FR-DEP-011), Scanners (library, FR-DISC-010), New Scanner (FR-SCAN-001), Scanner Alerts
  (FR-SCAN-011). The entry for the current view shall be highlighted. Equity Builder always opens a new draft (subject to the
  unsaved-change guard, FR-SHELL-008). The sidebar shall be collapsible to icons only (tooltips show labels) and the collapsed
  state persisted. Each entry has a default navigation shortcut in the shortcut registry (FR-SHELL-019).
- **Acceptance criteria:**
  1. Given the app is open, then the sidebar lists the 8 entries above in that order.
  2. Given the user clicks Equity Builder twice (saving nothing), then each click opens a fresh draft (subject to FR-SHELL-008).
  3. Given focus is on the sidebar, when the user presses Up/Down and Enter, then focus moves between entries and Enter
     navigates.
  4. Given the user collapses the sidebar and restarts the app, then it is still collapsed.
- **Data/Kite dependency:** local only.
- **Deviation from Streak:** Streak's "Options Trader" entry is dropped (options out of scope, §10 SHELL-002-opt). Labels use
  our own wording ("Equity Builder"/"New Scanner"). Collapsible sidebar added for small windows (NFR-11).
- **Priority hint:** Core

### FR-SHELL-003 Attention badges on navigation entries
- **Source features:** SHELL-005
- **Requirement:** The system shall show a red dot badge on **Live Strategies** when at least one deployment event the user
  has not yet viewed exists (signal triggered, order placed/rejected/filled for a deployment, deployment stopped,
  expired or errored), and on **Scanner Alerts** when at least one scanner-alert hit the user has not yet viewed exists.
  Opening the corresponding view marks all events listed in it as seen and clears the badge. The Notifications bell
  (FR-SHELL-006) shall show the count of unread notifications (1–99, "99+" above).
- **Acceptance criteria:**
  1. Given no unseen events, then neither entry shows a badge.
  2. Given a scanner alert fires while the user is on Home, then within 2 s a red dot appears on Scanner Alerts.
  3. Given the badge is shown, when the user opens Scanner Alerts, then the badge disappears and does not reappear after
     restart until a new alert fires.
  4. Given 3 unread notifications, then the bell shows "3"; after opening Notifications it shows no count.
- **Data/Kite dependency:** local event store (deployment engine, scanner engine, Kite order updates via ticker
  `OnOrderUpdate`).
- **Deviation from Streak:** Badge semantics were unobservable in Streak; we define them explicitly. Bell unread count is
  an addition (Streak shows no count).
- **Priority hint:** Important

### FR-SHELL-004 Index ticker strip
- **Source features:** SHELL-002
- **Requirement:** The top bar shall show three ticker pills: NIFTY 50, NIFTY BANK and SENSEX. Each pill shows the display
  name, last traded price (2 decimals, Indian grouping) and absolute and percent change versus the previous session close,
  formatted `+123.45 (+0.55%)` with a green ▲ for ≥0 and red ▼ for <0. Values update on every tick. Clicking (or Enter on)
  a pill loads that index into the dashboard main chart and navigates to Home. When the window is narrower than 1440 px
  the pills shall drop the absolute change and show only LTP and percent; when no data is available they show "—".
- **Acceptance criteria:**
  1. Given the feed is connected, when a tick for NIFTY 50 arrives, then its pill updates within 500 ms.
  2. Given LTP 22,600.00 and previous close 22,500.00, then the pill shows `22,600.00` and `+100.00 (+0.44%)` with ▲ green.
  3. Given the user clicks the SENSEX pill on the Scanners view, then the app navigates to Home and the main chart shows
     SENSEX.
  4. Given no data plan / feed disconnected and no cached quote, then each pill shows "—" (never 0.00).
  5. Given the app restarts outside market hours, then pills show the last cached LTP/change with the status indicator
     showing market closed.
- **Data/Kite dependency:** Kite WebSocket (gokiteconnect `ticker`, `quote` mode; previous close = tick `ohlc.close`);
  fallback `GET /quote/ltp` + `GET /quote/ohlc` at ≤1 req/s. Requires Connect plan (C-01).
- **Deviation from Streak:** Pills are clickable (Streak bug §3.2 #28: they look like links but are inert). Missing data
  shown as "—" (bug #19).
- **Priority hint:** Important

### FR-SHELL-005 Connection, session and market status indicator
- **Source features:** SHELL-003
- **Requirement:** The top bar shall show a status indicator composed of a coloured dot **and** a text label, plus a tooltip
  with details. States of the market-data feed: `Live` (green; ticker connected and subscribed), `Reconnecting` (amber;
  ticker auto-reconnect in progress, tooltip shows attempt count and next retry), `Offline` (red; disconnected, retries
  exhausted or network down), `No data plan` (grey; Kite key lacks Connect permissions, C-01), `Logged out` (grey; no
  valid access token). The tooltip shall additionally show: broker session state and expiry time ("Session valid until
  06:00 tomorrow"), market phase obtained from the market calendar service (FR-DATA-001; phases `Pre-open`, `Open`, `Closing`,
  `Post-closing`, `Closed`, `Holiday`, `Special session`) and the measured clock offset (FR-DATA-004), and time of last tick. When the market is closed or a holiday, a small "Market closed"
  label shall appear next to the dot. Clicking the indicator opens a status panel with a "Reconnect now" button and a
  "Log in" button when the session is missing/expired. Colour transitions take ≤300 ms. Holidays and special sessions come from FR-DATA-001 (never from literals in this FR).
- **Acceptance criteria:**
  1. Given the ticker is connected, then the indicator shows a green dot with label "Live".
  2. Given the network drops, when the ticker reports a disconnect, then within 1 s the indicator shows amber
     "Reconnecting"; when reconnect succeeds it returns to "Live" and all prior subscriptions are restored (NFR-09).
  3. Given the access token passed 06:00 IST, then the indicator shows "Logged out" and the status panel offers "Log in".
  4. Given Sunday or a listed holiday, then the label "Market closed" is shown regardless of feed state.
  5. Given any state, then hovering or focusing the indicator shows a tooltip containing feed state, session expiry and
     market phase (no state is unlabelled).
- **Data/Kite dependency:** gokiteconnect `ticker` callbacks (`OnConnect`, `OnReconnect`, `OnNoReconnect`, `OnClose`,
  `OnError`); session store (C-02); market calendar service (FR-DATA-001); app clock (FR-DATA-004).
- **Deviation from Streak:** Streak shows only a dot with "Live" or an unlabelled amber dot and no market-closed signal
  (bug §3.2 #27). We label every state, add session and market phase (C-02, C-03).
- **Priority hint:** Core

### FR-SHELL-006 Top-bar quick links
- **Source features:** SHELL-004
- **Requirement:** The top bar shall provide, on every view, a link "Order Book" (to the order book view), a link
  "Positions" (to the positions view), a bell icon button (to the Notifications view, a full view, not a dropdown) with
  unread count (FR-SHELL-003), and the profile avatar button (FR-SHELL-010). Each has an accessible name and tooltip.
- **Acceptance criteria:**
  1. Given any view, when the user clicks Order Book, then the order book view opens and the link is highlighted.
  2. Given any view, when the user clicks the bell, then the Notifications view opens in the content area (no popover).
  3. Given keyboard focus on the bell, then the screen-reader name reads "Notifications, N unread".
- **Data/Kite dependency:** none (navigation). Targets ORD-001, ORD-002, DEP-009.
- **Deviation from Streak:** Unread count added (see FR-SHELL-003).
- **Priority hint:** Core

### FR-SHELL-007 Onboarding guided tours
- **Source features:** SHELL-006, BT-029
- **Requirement:** The system shall provide first-visit guided tours composed of popovers anchored to UI regions, each
  with a title, body text (our own wording), a counter "N of M", buttons Back (hidden on step 1), Next (on non-final
  steps), Done (final step) and a close ×. Tours: Discover Scanners (3 steps: library list, scan results, enabling
  alerts), library scanner view (2 steps: universe & conditions, results), Scalper (4 steps: charts & chart trading, watchlist
  drawer, third chart, positions & order book), backtest results view (2 steps: per-instrument table and combined results,
  metrics and trade log). A tour starts automatically the first time its view is opened, is marked seen on Done, × or Escape,
  and never auto-starts again. Each view with a tour exposes "Replay tour" in its help (?) menu; Settings provides "Reset all
  tours". If a step's anchor element is not present, that step is skipped. All tour copy is written fresh (none of Streak's
  tour text was captured or may be reused).
- **Acceptance criteria:**
  1. Given a fresh profile, when the user opens the Scalper view, then step "1 of 4" is shown anchored to the chart area.
  2. Given step 2 of 4, when the user presses Back, then step 1 is shown; Next on step 4 is replaced by Done.
  3. Given the user presses Escape on step 2, then the tour closes and does not reappear on next visit or after restart.
  4. Given "Reset all tours" is used, then each tour auto-starts again on next visit to its view.
  5. Given a backtest results view opened for the first time, then the 2-step tour starts; after Done it never auto-starts again.
- **Data/Kite dependency:** local only (per-tour "seen" flags in SQLite).
- **Deviation from Streak:** Replay/reset added (Streak open question). Tour copy is our own. Escape closes. The Options Builder
  tour is dropped with the options scope (§10 SHELL-007-opt).
- **Priority hint:** Nice-to-have

### FR-SHELL-008 Unsaved-change guard
- **Source features:** SHELL-007
- **Requirement:** When the user has unsaved edits in the Equity Builder **or** Scanner builder and
  attempts to navigate to another view, open another draft, or close the application window, the system shall show an
  in-app modal "Unsaved changes" with buttons "Save", "Discard" and "Stay" (default focus Stay; Escape = Stay). Save runs
  the view's save flow and proceeds only if save succeeds; Discard drops the edits and proceeds; Stay cancels the
  navigation/close. A view is "dirty" from the first edit after load/save until the next successful save. The guard
  shall not block closing if nothing is dirty.
- **Acceptance criteria:**
  1. Given a new scanner draft with one condition row edited, when the user clicks Home, then the modal appears.
  2. Given the modal, when the user chooses Stay, then the view and edits are unchanged.
  3. Given the modal, when Discard is chosen, then navigation proceeds and the draft is not persisted.
  4. Given a dirty strategy, when the user closes the window, then the modal appears and the app stays open until Save
     or Discard is chosen.
  5. Given a builder with no edits since its last save, when the user navigates away, then no modal appears.
- **Data/Kite dependency:** local only (Wails `OnBeforeClose` hook + frontend router guard).
- **Deviation from Streak:** Applies to the scanner builder too (bug §3.2 #15) and to in-app navigation — Streak guards only the browser
  `beforeunload`; leaving a dirty draft through its sidebar discards it silently (REVIEW-2 #28). Replaces the browser
  beforeunload prompt;
  offers Save directly.
- **Priority hint:** Core

### FR-SHELL-009 Toast notifications
- **Source features:** SHELL-008
- **Requirement:** The system shall show transient toasts at top-centre with a coloured left border and icon by level:
  INFO (blue), SUCCESS (green), WARNING (amber), ERROR (red). Each toast has a message, an optional action link, and a
  close ×. Auto-dismiss: INFO/SUCCESS after 5 s, WARNING after 8 s, ERROR stays until dismissed; hovering or focusing a
  toast pauses its timer. At most 3 toasts are visible (newest on top; older ones queue). The toast region is an ARIA
  live region; Alt+T (default binding, FR-SHELL-019) moves keyboard focus to the newest toast. Every error toast that concerns a specific input shall
  also be shown inline at that input. Toast messages shall be plain, specific and our own wording (e.g. "No instruments
  matched the conditions", "Found 5 instruments", "Strategy saved").
- **Acceptance criteria:**
  1. Given a SUCCESS toast, then it disappears after 5 s ±0.5 s unless hovered.
  2. Given an ERROR toast, when 60 s pass, then it is still visible until × is clicked.
  3. Given 4 toasts raised within 1 s, then 3 are visible and the 4th appears when one is dismissed.
  4. Given a scanner run with an incomplete row (§3.1: LHS present, comparator or RHS missing), then an ERROR toast appears
     **and** the incomplete row is outlined with an inline message; a fully empty row produces only an INFO toast
     (FR-SCAN-004).
  5. Given toasts are visible, when Alt+T is pressed, then focus moves to the newest toast.
- **Data/Kite dependency:** local only.
- **Deviation from Streak:** WARNING level and inline duplication of field errors added; errors no longer rely on a
  toast alone (§3.2 note on toast-only failures). Errors persist until dismissed.
- **Priority hint:** Core

### FR-SHELL-010 Profile menu
- **Source features:** SHELL-009
- **Requirement:** Clicking the avatar shall open a menu showing, top to bottom: user name (bold) and Zerodha client ID
  (grey); divider; Profile (→ profile view, FR-ACCT-001); Holdings (→ holdings view); Settings (→ settings view,
  FR-ACCT-012); Help (→ built-in help, FR-ACCT-010); divider; "Dark theme" toggle (FR-SHELL-011); divider; app version
  (from build metadata, e.g. "v1.0.0") right-aligned. The menu closes on outside click, Escape or item activation and
  is fully keyboard operable. When logged out, name/client ID are replaced by "Not logged in" and a "Log in" item.
- **Acceptance criteria:**
  1. Given a logged-in session, when the avatar is clicked, then the menu shows the name and client ID returned by
     `GET /user/profile`.
  2. Given the menu is open, when Escape is pressed, then it closes and focus returns to the avatar.
  3. Given the build version 1.2.3, then the menu shows "v1.2.3".
  4. Given Help is chosen, then the built-in help opens inside the app (no external browser).
- **Data/Kite dependency:** `GET /user/profile` (gokiteconnect `GetUserProfile`), cached locally after login.
- **Deviation from Streak:** Help points to built-in help, not an external site; Settings item added (Streak has no
  settings page; the desktop app needs one for Kite credentials, trading enablement, data storage).
- **Priority hint:** Core

### FR-SHELL-011 Light/dark theme
- **Source features:** SHELL-010
- **Requirement:** The system shall support a light theme (default) and a dark theme, switched by the "Dark theme"
  toggle in the profile menu (and in Settings), applied immediately to all views, dialogs, charts and analytics charts,
  and persisted. An additional Settings option "Follow system theme" (default off) shall track the desktop colour-scheme
  preference when on. All text shall meet WCAG 2.1 AA contrast (≥4.5:1 body text, ≥3:1 large text/icons) in both themes.
- **Acceptance criteria:**
  1. Given a fresh install, then the light theme is active and the toggle is off.
  2. Given the toggle is switched on, then within 300 ms every visible surface including the chart canvas uses the dark
     palette, and after restart the dark theme is still active.
  3. Given either theme, when an automated contrast check is run over the views' text tokens, then every pair passes AA.
- **Data/Kite dependency:** local only (preference in SQLite).
- **Deviation from Streak:** Charts explicitly follow the theme (unknown in Streak); contrast requirement fixes the 404
  contrast bug (§3.2 #26). Follow-system option added.
- **Priority hint:** Important

### FR-SHELL-012 Unknown-view fallback
- **Source features:** SHELL-011
- **Requirement:** When the app is asked to show a view that does not exist (unknown internal route, stale deep link
  from a notification, removed strategy/scanner id), the system shall show a "Page not found" view with a one-line
  explanation, readable contrast (FR-SHELL-011) and a "Go to Home" button; for a missing strategy/scanner/backtest the
  message names the item type ("This strategy no longer exists").
- **Acceptance criteria:**
  1. Given navigation to an unregistered route, then the "Page not found" view appears with a working "Go to Home" button.
  2. Given a notification links to a deleted scanner, when it is clicked, then the message "This scanner no longer
     exists" is shown.
- **Data/Kite dependency:** none.
- **Deviation from Streak:** Contrast bug fixed (§3.2 #26); item-specific messages added.
- **Priority hint:** Nice-to-have

### FR-SHELL-013 Contextual help and tooltips
- **Source features:** SHELL-012
- **Requirement:** Every icon-only button shall have a tooltip and an accessible name (e.g. "Refresh chart data", "Save
  indicators and drawings", "Clear indicators and drawings", "Open chart", "Disable row", "Enable row", "Copy logic",
  "Paste logic after", "Delete watchlist <name>", "Search symbols"). Tooltips appear after 500 ms hover **or** on keyboard
  focus and hide on Escape. Section headers with an (i) icon (e.g. the Advanced conditions header) shall open the matching
  article of the built-in help (FR-ACCT-010) in a side panel. No control may show a "coming soon" placeholder; unfinished
  controls are hidden.
- **Acceptance criteria:**
  1. Given an automated scan of all views, then every button without visible text has a non-empty accessible name.
  2. Given keyboard focus moves to the chart refresh button, then its tooltip is displayed.
  3. Given the user clicks (i) on the Advanced conditions header, then the help panel opens on the advanced-conditions
     article.
- **Data/Kite dependency:** none (bundled help content).
- **Deviation from Streak:** Help is bundled offline, not an external site; "coming soon" placeholders forbidden
  (§3.2 #31).
- **Priority hint:** Important

### FR-SHELL-014 List-page template and empty states
- **Source features:** SHELL-001
- **Requirement:** Account and list views (Order Book, Positions, Holdings, Notifications, Live Strategies, Scanner
  Alerts, My Strategies/My Scanners) shall share one header template: back arrow (returns to previous view), H1 title,
  a one-line description, and right-aligned utility controls Refresh (re-fetch now) and Search (filters visible rows
  client-side, case-insensitive, by symbol/name). Tables have a header row and support keyboard row navigation. Empty
  state: illustration (our own), a one-line message specific to the view (e.g. Holdings: "You have no holdings"), and at
  most one call-to-action button. A loading state (skeleton rows) and an error state (message + Retry) are mandatory.
- **Acceptance criteria:**
  1. Given the Holdings view with no holdings, then the message refers to holdings (not positions) and one CTA is shown.
  2. Given a list with 20 rows, when "TCS" is typed in Search, then only rows whose symbol or name contains "tcs"
     (case-insensitive) remain.
  3. Given the Kite call fails with a network error, then the error state with "Retry" is shown instead of an empty state.
  4. Given the back arrow is clicked, then the previous view is restored.
- **Data/Kite dependency:** none for the template; data per target view.
- **Deviation from Streak:** Holdings empty-state wording bug fixed (§3.2 #25); explicit loading/error states added.
- **Priority hint:** Important

### FR-SHELL-015 Keyboard access and dialog dismissal
- **Source features:** SHELL-001, SHELL-012
- **Requirement:** Every action that Streak reveals only on hover (row chart/remove/B/S actions, study
  legend icons, operand add/remove) shall also be reachable by keyboard: the actions become visible when the row/element
  has keyboard focus, and a context menu (Shift+F10 / Menu key / right-click) lists the same actions. All modal dialogs
  shall close on Escape (equivalent to Cancel), trap focus while open, restore focus on close, and have a visible
  close × or Cancel button.
- **Acceptance criteria:**
  1. Given a watchlist row has keyboard focus, then its chart, remove, B and S actions are visible and reachable by Tab.
  2. Given any modal dialog in the app (indicator parameters, instrument picker, suggestions, copy dialogs, chart
     dialogs), when Escape is pressed, then it closes without applying changes.
  3. Given an automated check of all dialogs, then each has a close × or Cancel button.
- **Data/Kite dependency:** none.
- **Deviation from Streak:** Fixes §3.2 #8, #16, #20 (Escape ignored, missing ×/Cancel); NFR-11 keyboard requirement.
- **Priority hint:** Core

### FR-SHELL-016 Number, currency and date formatting
- **Source features:** SHELL-001, WATCH-008
- **Requirement:** One central formatter shall be used for all displayed values: prices with Indian digit grouping and
  instrument tick precision (2 decimals default, e.g. `22,520.45`); money with `₹` prefix; large quantities/OI
  auto-abbreviated to `L` (lakh, ≥1,00,000) and `Cr` (crore, ≥1,00,00,000) with 2 decimals; percentages with sign and 2
  decimals in parentheses next to a price; dates in two defined styles only — long `09 Oct 2026, 15:15` and compact
  `09-10-2026 15:15:32` (DD-MM-YYYY per NFR-11); 24-hour IST times. Each view uses one style per field consistently.
- **Acceptance criteria:**
  1. Given 13200000 OI, then it renders `1.32Cr`; given 254000, `2.54L`.
  2. Given ₹1234567.8, then it renders `₹12,34,567.80`.
  3. Given the scanner "last scan" timestamp is displayed in the library list and in the editor, then both use the same
     style.
- **Data/Kite dependency:** none.
- **Deviation from Streak:** Fixes inconsistent "Last Scan" formats (§3.2 #14).
- **Priority hint:** Core

### FR-SHELL-017 Application update
- **Source features:** ACCT-005, SHELL-009
- **Requirement:** The system shall check for a newer release at start and once per day **only if** the user turned on
  "Check for updates" in Settings → Updates (default off, NFR-04); the feed URL is shown and editable. When a newer version is
  found, a non-blocking banner "Update available: vX.Y.Z" with release notes and a "Download page" link appears; the app never
  downloads or installs anything itself (packaging is .deb/AppImage, C-11). On the first start of a new version, the migration
  path of FR-DATA-005 runs and a "What changed" page lists app changes and library-version changes (FR-DISC-020). An update
  installed while deployments are running takes effect only at the next start; the quit dialog of FR-DEP-020 applies. The
  profile menu and Settings → About show the running version and the last check time.
- **Acceptance criteria:**
  1. Given "Check for updates" is off, then a network capture during 24 h of use shows no request to any host other than Kite
     (and Telegram if enabled).
  2. Given the check is on and the feed lists v1.3.0 while v1.2.0 runs, then the banner shows "Update available: v1.3.0" with notes.
  3. Given v1.3.0 starts over a v1.2.0 database, then the FR-DATA-005 migration runs first and the "What changed" page is shown
     once.
- **Data/Kite dependency:** optional HTTPS GET of the release feed (user-enabled).
- **Deviation from Streak:** Streak updates itself server-side; a packaged desktop app needs an opt-in check (NFR-02, NFR-04).
- **Priority hint:** Important

### FR-SHELL-018 Application logging and diagnostics
- **Source features:** ACCT-007, ORD-005, ORD-001
- **Requirement:** The system shall write structured JSON logs (`time`, `level`, `component`, `event`, fields) to
  `<data dir>/logs/`, level INFO by default and DEBUG switchable at runtime from Settings → Diagnostics without restart; files
  rotate at 20 MB with 10 files kept. A **redaction filter** applied to every log line removes `Authorization` header values,
  `request_token`, `access_token`, `api_secret`, `checksum`, the Telegram bot token and any value read from the keyring, and is
  unit-tested against every call site that logs HTTP traffic. A crash handler writes a crash report (stack trace, app version,
  schema version, state summary without secrets) to `logs/crash-<timestamp>.json` and offers to view it on the next start.
  "Create support bundle" zips logs, settings (secrets excluded), schema and library versions, calendar version, the last 200
  order-journal rows and recent deployment-log excerpts into `exports/`. "Open logs folder" opens the directory (FR-ACCT-010).
  The clock-offset history (FR-DATA-004) and rate-limiter events (FR-DATA-003) are logged at INFO.
- **Acceptance criteria:**
  1. Given DEBUG HTTP logging during a login and 100 Kite calls, then a search of all log files for the access token, API secret and
     request token returns nothing.
  2. Given a forced panic in a background goroutine, then a crash report exists and the next start shows "The app crashed last
     time — view report".
  3. Given 250 MB of log output, then at most 10 files × 20 MB remain.
  4. Given "Create support bundle", then the archive contains no keyring value (grep test) and opens with the listed contents.
- **Data/Kite dependency:** none.
- **Deviation from Streak:** Streak has no user-visible logs; required by NFR-10 and NFR-04.
- **Priority hint:** Important

### FR-SHELL-019 Global keyboard shortcuts and window state
- **Source features:** SHELL-001, SHELL-012, CHART-008, SCALP-013
- **Requirement:** One **shortcut registry** shall own every keyboard binding in the app with a stable action id, a default
  binding and a scope (global, view, chart, builder). Defaults: navigation Alt+1…Alt+8 to the sidebar entries in order; toasts
  Alt+T; search Ctrl+K; new order Shift+T (chart scope); kill switch Ctrl+Shift+K (global, requires the FR-ORD-009
  confirmation); help F1; full-view chart F11; builder undo/redo Ctrl+Z / Ctrl+Shift+Z; watchlist reorder Alt+↑/↓; chart
  tools Alt+T (trend line), Alt+H, Alt+J, Alt+V, Alt+C (chart scope, FR-CHART-010; the chart-scope Alt+T shadows the toast
  binding only while a chart has focus); Scalper buy/sell/exit-all/cancel-all (unbound by default, FR-SCALP-013). Settings →
  Shortcuts lists all actions, allows remapping with conflict detection (a binding used by another action in an overlapping scope
  is refused with the conflicting action named) and reset to defaults; GTK menu mnemonics are not used, so Alt+letter bindings
  are free. The Help "Keyboard shortcuts" article (FR-ACCT-010) is generated from the registry. **Window state:** size,
  position, maximised state, sidebar collapsed state and the last view are persisted on quit and restored on start (clamped to
  the current display).
- **Acceptance criteria:**
  1. Given the kill switch is remapped to Ctrl+Shift+X, then Ctrl+Shift+K does nothing, Ctrl+Shift+X opens the kill-switch
     confirmation, and the Help article shows the new binding.
  2. Given an attempt to bind Alt+1 to "Buy" in the global scope, then it is refused naming "Go to Home".
  3. Given the app is closed maximised on the Scalper view, then it reopens maximised on the Scalper view.
  4. Given focus on a chart, when Alt+T is pressed, then the trend-line tool activates; given focus elsewhere with toasts visible,
     then Alt+T focuses the newest toast.
- **Data/Kite dependency:** none.
- **Deviation from Streak:** Streak has no shortcut list or remapping (FEATURES §3.3); all bindings are ours.
- **Priority hint:** Important

### FR-SHELL-020 Risk-disclosure acknowledgement per session
- **Source features:** SHELL-013, DEP-001
- **Requirement:** After every successful Kite login (FR-ACCT-006) and before any other view, the system shall show a
  risk-disclosure screen with our own text (educational tool, hypothetical results, user bears trading risk, no automatic
  trading unless explicitly armed), a link to the legal pages (FR-ACCT-015) and one button "I understand". Acknowledgement is
  recorded with the session id in SQLite; it is shown once per login session (a restart with the same still-valid token does not
  repeat it). While unacknowledged, Deploy, order placement and arming are disabled. The text is also reachable from Settings →
  About → "Risk disclosure".
- **Acceptance criteria:**
  1. Given a fresh login, then the screen is shown before Home; after "I understand", Home opens and the screen is not shown again
     until the next login.
  2. Given the screen is shown and the user opens the deploy dialog via a deep link, then Deploy is disabled with "Acknowledge the
     risk disclosure first".
  3. Given a restart at 14:00 with a token issued at 09:00 and already acknowledged, then the screen is not shown.
- **Data/Kite dependency:** session id from FR-ACCT-006.
- **Deviation from Streak:** Streak shows a risk-disclosure acknowledgement once per login session (REVIEW-2 N9, text not
  captured); ours is written fresh and also gates arming (NFR-05).
- **Priority hint:** Important

---

## FR module HOME — Home dashboard

### FR-HOME-001 Dashboard layout
- **Source features:** HOME-001
- **Requirement:** The Home view shall be laid out as a left column (~2/3 width) and a right column (~1/3 width,
  independently scrollable). Left column, top to bottom: a row with the Holdings summary card (FR-HOME-002) and the Live
  Strategy Overview card (FR-HOME-003); the main chart (FR-CHART-002 and following); the analytics panel (FR-ANLY-001).
  Right column, top to bottom: Watchlist panel (FR-WATCH-001), Live Scanners widget (FR-HOME-004), Live Backtest widget
  (FR-HOME-006). On first run the main chart shows NIFTY 50 at the Settings → Chart default timeframe (shipped 1 Hour; OQ-23); thereafter the last charted symbol
  and timeframe are restored (FR-CHART-016). Each card renders its own loading, empty and error state without blocking
  the others.
- **Acceptance criteria:**
  1. Given a fresh install, when Home opens after login, then the chart shows NIFTY 50 at the shipped default timeframe (1 Hour).
  2. Given the holdings request fails, then only the Holdings card shows an error with Retry; the chart and widgets load.
  3. Given a 1280×720 window, then all six areas are reachable (right column scrolls) without horizontal scroll.
- **Data/Kite dependency:** aggregates `GET /portfolio/holdings`, ticker, local deployment/scanner/backtest stores.
- **Deviation from Streak:** none functionally.
- **Priority hint:** Core

### FR-HOME-002 Holdings summary card
- **Source features:** HOME-002
- **Requirement:** The card shall show `Current value` = Σ(quantity × LTP), `Investment` = Σ(quantity × average price) and
  overall `P&L` = Current value − Investment as absolute ₹ and percent of Investment (green ▲ if ≥0, red ▼ if <0), where
  quantity = settled quantity + T1 quantity of each holding. A secondary line shows `Day's change` = Σ(quantity × (LTP −
  previous close)). LTPs update live from the ticker; the holdings list is re-fetched on view open, on Kite order-update
  events for CNC orders, and every 5 minutes. With no holdings the card shows "No holdings" instead of zero values.
  Clicking the card opens the Holdings view.
- **Acceptance criteria:**
  1. Given holdings A (10 @ avg 100, LTP 110, prev close 105) and B (5 @ avg 200, LTP 190, prev close 200), then Current
     value ₹2,050.00, Investment ₹2,000.00, P&L `+₹50.00 (+2.50%)`, Day's change `+₹0.00`.
  2. Given a tick changes A's LTP to 120, then within 1 s Current value shows ₹2,150.00.
  3. Given zero holdings, then "No holdings" is shown and no ₹0.00 values.
  4. Given the card is clicked, then the Holdings view opens.
- **Data/Kite dependency:** `GET /portfolio/holdings` (gokiteconnect `GetHoldings`); ticker `quote` mode for LTP;
  works on the free Personal plan with `GET /quote/ltp` polling (60 s) if no ticker (C-01).
- **Deviation from Streak:** Explicit overall-P&L definition plus a day's-change line; empty state message instead of zeros;
  card navigates to Holdings (Streak: not clickable).
- **Priority hint:** Important

### FR-HOME-003 Live Strategy Overview card
- **Source features:** HOME-003
- **Requirement:** The card shall show three counters from the local deployment store: `Active` (deployments currently
  running or armed, each instrument counted once, consistent with Live Strategies), `Closed` (deployments stopped by the
  user, expired, or completed), and `Total` (= Active + Closed, excluding deleted deployments). Counters update within
  2 s of any deployment state change. Clicking a counter opens Live Strategies filtered to that status.
- **Acceptance criteria:**
  1. Given 2 running and 1 stopped deployment, then the card shows Active 2, Closed 1, Total 3.
  2. Given a deployment expires, then within 2 s Active decrements and Closed increments.
  3. Given "Closed" is clicked, then Live Strategies opens with the Closed filter applied.
- **Data/Kite dependency:** local deployment store (DEP module).
- **Deviation from Streak:** Counters are links; "Closed" defined explicitly (open question in catalogue).
- **Priority hint:** Important

### FR-HOME-004 Live Scanners widget — header, universe and tabs
- **Source features:** HOME-004
- **Requirement:** The widget shall show a header with a status dot (pulsing while the scanner is evaluating live, static
  grey when the market is closed), the title "Live Scanners", a read-only label of the active scanner's timeframe
  (e.g. "1 Day", "5 Minutes"), and a universe dropdown listing the universes of FR-WATCH-009 (same list and order as every
  other universe dropdown). Below it a horizontally scrollable tab strip (« » buttons and mouse-wheel scroll) shows the library
  scanners tagged `featured` (FR-DISC-011), in library order; the user can change the featured set in Settings → Home (any
  library or own scanner). The selected universe is the shared scanner-universe preference (FR-WATCH-011). The selected tab is
  persisted. While the market is open the active scanner is re-evaluated on every close of its timeframe's candle (NFR-08:
  results within 2 s after close); when the market is closed (FR-DATA-001), results are computed once from cached candles of
  the last session and the header shows "As of <timestamp>". The widget subscribes through the subscription manager at the
  **lowest** priority (below deployments, live scanners, charts and watchlists); if the universe does not fit the remaining
  budget (C-07) the widget falls back to polled evaluation (`GET /quote` batches, ≤ 1 req/s) and shows a "polled" hint; it never
  refuses a universe.
- **Acceptance criteria:**
  1. Given the widget, then the universe dropdown lists exactly the entries of FR-WATCH-009 in its order.
  2. Given the user changes the universe to Nifty 100 in the widget, then the Discover Scanners universe is also Nifty 100.
  3. Given the library's featured scanners, then the tab strip shows them in library order and the first tab's timeframe label
     matches its scanner (e.g. an opening-range scanner → "5 Minutes").
  4. Given market open and a 5-minute candle closes at 10:05:00, then the active scanner's results reflect that candle by 10:05:02.
  5. Given Sunday, then results show the last session's hits and "As of 09 Oct 2026, 15:30".
  6. Given 8,900 live subscriptions in use and a 500-instrument universe selected, then the widget evaluates by polling, shows the
     "polled" hint and no activation error.
- **Data/Kite dependency:** local scanner engine over the candle store (FR-DATA-002); universe constituents (FR-WATCH-010);
  subscription manager (C-07); `GET /quote` fallback via FR-DATA-003.
- **Deviation from Streak:** Live evaluation runs only while the app is running (C-03); results are computed locally, so after a
  fresh install the first results appear only after backfill of the universe (progress shown, FR-HOME-005). The featured set is
  ours (tagged in the library), not Streak's first 20 scanner names (RV-020).
- **Priority hint:** Important

### FR-HOME-005 Live Scanners widget — result rows and navigation
- **Source features:** HOME-004
- **Requirement:** Each result row shall show: instrument (symbol + exchange tag, display name per FR-WATCH-013; futures with an F badge), one column "LTP / Change" (LTP and coloured percent change), and B / S buttons (disabled for
  indices). Hovering or focusing a row reveals a chart action that loads the instrument into the dashboard main chart.
  About 6 rows are visible; the list scrolls. Below the list a "See full results" button opens the full scanner view of
  the active tab's scanner (SCAN module). B/S open the order ticket (ORD-004) prefilled with the instrument and side.
  While universe data is being backfilled, the list area shows "Preparing data… n of m instruments" with a progress bar;
  if no instrument matches, "No instruments match right now".
- **Acceptance criteria:**
  1. Given a row for NIFTY OCT FUT, then it shows the F badge and exchange tag NFO-FUT.
  2. Given the chart action of a row is activated, then the main chart switches to that instrument and the analytics tab
     set adapts (a futures contract → Technicals only; an index with constituents → Technicals and Advance/Decline).
  3. Given B is clicked on a stock row, then the order ticket opens prefilled (BUY, that symbol) and no order is sent.
  4. Given "See full results" on the Top Losers tab, then the Top Losers scanner view opens with the same universe.
  5. Given an index row, then B and S are disabled.
- **Data/Kite dependency:** ticker for LTP; instrument master; order ticket (ORD-004).
- **Deviation from Streak:** Chart action keyboard-reachable (FR-SHELL-015); explicit preparing/no-match states; option
  rows dropped with the options scope (§10 HOME-005-opt).
- **Priority hint:** Important

### FR-HOME-006 Live Backtest widget — tabs and metrics
- **Source features:** HOME-005
- **Requirement:** The widget shall show a header (dot + "Live Backtest") and a horizontally scrollable tab strip of the
  library strategies tagged `featured` (FR-DISC-002; shipped set of 10, in library order); the user can change the featured set
  in Settings → Home. For the active tab it shows the **cached summary run** of FR-DISC-007 (same window, parameters and cache —
  the widget never runs its own backtest): `Backtest P&L` (₹, green/red), `Max gain`, `Max loss` (largest single-trade gain/loss),
  `Win vs loss` bar (green fill = wins ÷ trades) with labels `N W` / `N L`, `Trades` (count of round trips), `Capital used`
  (FR-BT-046), all Combined across the strategy's default instruments per FR-BT-012/014, over the window of FR-DISC-007 (default
  last 3 calendar months ending the last completed trading day). "See full results" opens the backtest results view (FR-BT-011)
  of that cached run. If the run is not computed yet, the widget shows FR-HOME-008's preparing state.
- **Acceptance criteria:**
  1. Given a cached run with trades P&L [+100, −40, +60], then Backtest P&L ₹120.00, Max gain ₹100.00, Max loss −₹40.00,
     2 W / 1 L, Trades 3, win bar 66.7 % filled.
  2. Given the active tab changes, then all metrics and the heatmap switch to that strategy within 300 ms (cached result).
  3. Given "See full results", then the backtest results view opens showing the same P&L, the same period and the same
     "Computed <timestamp>" as the Discover detail pane for that strategy.
  4. Given the Discover detail pane recomputes the summary, then the widget shows the new values on its next render without a
     second engine run.
- **Data/Kite dependency:** cached summary runs (FR-DISC-007) from the local backtest engine over the candle store.
- **Deviation from Streak:** Results computed locally; one cache shared with Discover (RV-011); featured set configurable and
  tagged in our library (Streak rotates its own picks, RV-020).
- **Priority hint:** Nice-to-have

### FR-HOME-007 Live Backtest widget — calendar heatmap
- **Source features:** HOME-005
- **Requirement:** Under the metrics the widget shall show one mini calendar per month covered by the window (oldest
  left), columns Mon–Fri, one cell per weekday. Cell colour: green shades for positive daily return, red shades for
  negative, 4 intensity steps each by |return| quartile within the window; grey for days with no trade or exchange
  holidays. Daily return % = that day's P&L ÷ (C × n) × 100 per FR-BT-046 (C = initial capital per instrument, n = instruments
  of the run) — the same definition as FR-BT-015/019. Hover/focus on a cell shows a
  tooltip `13 Jul 2026: −1.48%` (or `13 Jul 2026: no trades`).
- **Acceptance criteria:**
  1. Given a 90-day window from 13 Jul to 10 Oct 2026, then 4 month grids (Jul–Oct) are shown.
  2. Given a day with P&L −₹5,122 and C × n = ₹3,46,000, then the tooltip reads `−1.48%` and the cell is red.
  3. Given an exchange holiday, then the cell is grey and the tooltip says no trades.
- **Data/Kite dependency:** cached summary run (FR-DISC-007); market calendar (FR-DATA-001).
- **Deviation from Streak:** Explicit daily-return definition (unspecified in Streak).
- **Priority hint:** Nice-to-have

### FR-HOME-008 Live Backtest widget — refresh schedule and data preparation
- **Source features:** HOME-005
- **Requirement:** The featured summary runs (FR-DISC-007 cache) shall be recomputed automatically once per trading day (FR-DATA-001) after 16:00 IST (when
  the app is running) or at app start if the stored result is older than the last completed trading day. Missing candle
  data is backfilled first; while backfilling, the widget shows progress ("Preparing data… 37%") and the previous result,
  if any, marked "Stale". Recompute runs in the background engine and must not block the UI (NFR-07).
- **Acceptance criteria:**
  1. Given the app starts on Tuesday with results from last Friday, then a recompute starts and on completion the "As of"
     date equals Monday.
  2. Given backfill is in progress, then the widget shows a percentage that increases monotonically to 100.
  3. Given offline (NFR-14), then the last result is shown with "Stale — offline".
- **Data/Kite dependency:** historical API backfill (C-08), local backtest engine.
- **Deviation from Streak:** Streak refreshes server-side (docs: after 21:00); we refresh locally after 16:00 or on start
  (C-03).
- **Priority hint:** Nice-to-have

---

## FR module CHART — Charting

Library summary (C-12, decision pending): capabilities FR-CHART-001/003/004/006/011/016/017 are `[Either]`;
FR-CHART-008/009/010/013/014/015 and the study-configuration and template parts of FR-CHART-007 are native to the
licensed TradingView Charting Library `[Full-TV]` and otherwise must be built by us `[Open+Own]` (own indicator engine
shared with strategy conditions, own drawing layer, own dialogs, own order-line overlay).

### FR-CHART-001 Chart component and embedding contexts
- **Source features:** CHART-001
- **Requirement:** The system shall provide one reusable chart component used in: the dashboard main chart, each Scalper
  chart (SCALP module), the strategy builder chart pane (EQB-031) and the
  full-view chart (FR-CHART-017). It renders OHLC candlesticks (green up / red down by default) with a volume histogram,
  an optional set of studies in overlay or separate panes, drawings, a header bar (FR-CHART-002), and Buy/Sell overlay
  buttons (FR-CHART-012). Chart type is candlestick only. Each instance has independent symbol, timeframe and layout
  state; the host context may set defaults and limits (e.g. Scalper: max 5 studies per chart). The component sits behind
  an internal adapter interface so that the charting library can be swapped without changing host views.
- **Acceptance criteria:**
  1. Given the dashboard and a Scalper chart are both open on different symbols, then changing the timeframe of one does
     not change the other (except where SCALP rules sync them).
  2. Given a host context sets `maxStudies=5`, when a 6th study is added, then it is refused with the message "This chart
     allows up to 5 indicators".
  3. Given the dark theme, then candles, grid, scales and panes use the dark palette (FR-SHELL-011).
- **Data/Kite dependency:** see FR-CHART-003.
- **Deviation from Streak:** Streak embeds TradingView (and ChartIQ next to conditions, Conflicts §6 #23); we use a
  single own-adapter component with a library chosen per C-12. Library note: `[Either]`.
- **Priority hint:** Core

### FR-CHART-002 Chart header bar
- **Source features:** CHART-001
- **Requirement:** Above the canvas each chart shall show, left to right: a symbol button (search icon, symbol, exchange
  tag such as `INDICES`/`NSE`/`NFO-FUT`, LTP and coloured percent change; opens FR-CHART-005), a refresh icon
  (FR-CHART-006), a timeframe dropdown (FR-CHART-004), a "Save" button and a "Clear" icon (FR-CHART-007), and an
  "Indicators" button (FR-CHART-008). LTP and percent update live.
- **Acceptance criteria:**
  1. Given RELIANCE is charted, then the header shows `RELIANCE NSE <LTP> (<pct>%)` with the percent green or red.
  2. Given each header control is tabbed to, then each has a visible focus ring and a tooltip (FR-SHELL-013).
- **Data/Kite dependency:** ticker for LTP/change; instrument master for exchange tag.
- **Deviation from Streak:** none. Library note: `[Either]` (header is our own React component in both options).
- **Priority hint:** Core

### FR-CHART-003 Chart data loading and live bar updates
- **Source features:** CHART-001, CHART-002
- **Requirement:** On symbol/timeframe change the chart shall request candles from the candle store (FR-DATA-002); missing
  ranges are fetched through the backfill service (FR-DATA-003) at interactive priority. Initial window: 1m → 5 trading days,
  3m/5m → 15, 10m/15m → 30, 30m/1h → 120, 1d → 3 years (≥ 740 daily candles); scrolling left past the loaded range lazily loads
  older data (as far back as Kite provides). During the continuous session (FR-DATA-001) the last candle is built live from
  ticks (OHLC from LTP; volume from the increase in cumulative day volume; indices have zero volume); a new candle starts at each
  interval boundary aligned to the session start, on the app clock (FR-DATA-004); tick-built candles are written to the store as
  incomplete (FR-DATA-002). Candles are raw/unadjusted (NFR-12). Loading shows a spinner; failure shows an inline error with
  Retry; with no data plan the chart shows "Charts need a Kite Connect data plan" (C-01). When offline, cached candles are shown
  with an "Offline — cached data" badge (NFR-14).
- **Acceptance criteria:**
  1. Given a 1 Day chart of RELIANCE with an empty cache, then ≥ 740 daily candles appear within 5 s and the historical API was
     called with windows ≤ 2000 days through FR-DATA-003.
  2. Given a 5 Minute chart during the session, when ticks arrive at 10:03, then the 10:00 candle's high/low/close update and at
     10:05:00 (app clock) a new 10:05 candle starts.
  3. Given the user scrolls left beyond the first loaded candle, then older candles are appended without resetting zoom.
  4. Given a Personal-plan key, then the chart shows the data-plan message and makes no historical calls.
  5. Given the same symbol/timeframe is reopened, then candles render from the store without a historical API call for
     already-cached ranges (coverage map).
- **Data/Kite dependency:** FR-DATA-002 (store), FR-DATA-003 (`GetHistoricalData`), ticker `quote`/`full` mode.
- **Deviation from Streak:** Data stored locally and lazily backfilled; first view of a long history may take seconds
  (3 req/s). Library note: `[Either]`.
- **Priority hint:** Core

### FR-CHART-004 Timeframe selector
- **Source features:** CHART-002
- **Requirement:** The timeframe dropdown shall offer: 1 Minute, 3 Minutes, 5 Minutes, 10 Minutes, 15 Minutes, 30 Minutes,
  1 Hour, 1 Day (Appendix I), mapping 1:1 to Kite intervals minute, 3minute, 5minute, 10minute, 15minute, 30minute,
  60minute, day. Defaults: dashboard and Scalper = the Settings → Chart default timeframe (shipped 1 Hour; OQ-23); builder chart = strategy's base timeframe (default 1 Minute);
  full-view chart = timeframe passed by the caller. Changing the timeframe reloads candles (FR-CHART-003), keeps studies
  and drawings, and notifies dependants (the analytics Technicals tab follows it, FR-ANLY-003). Keyboard: the dropdown
  is operable with arrows/Enter.
- **Acceptance criteria:**
  1. Given the dropdown is opened, then exactly the 8 options above are listed in that order.
  2. Given an RSI study on the chart, when timeframe changes from 1 Hour to 1 Day, then RSI remains and is recomputed on
     daily candles.
  3. Given the dashboard chart switches to 1 Day, then the Technicals tab shows daily pivots/events.
- **Data/Kite dependency:** Kite historical intervals (Appendix I).
- **Deviation from Streak:** none (10 Minutes included per Conflicts §6 #8). Library note: `[Either]`.
- **Priority hint:** Core

### FR-CHART-005 Chart symbol search (single-select)
- **Source features:** CHART-003
- **Requirement:** The symbol button shall open a modal "Search instruments" (single-select variant of FR-WATCH-007) with
  an auto-focused search input (placeholder e.g. "Search, e.g. SBIN, TCS"), quick chips NIFTY 50, NIFTY BANK, SENSEX,
  BANKEX, and a result list with columns Instrument (symbol + exchange tag + an F badge for futures) and Company name. Empty
  query lists NSE equities alphabetically. Search uses the shared instrument search (FR-WATCH-008) across INDICES, NSE, BSE,
  NFO-FUT and BFO-FUT. Up/Down moves the highlight, Enter or click selects: the chart switches symbol, the modal closes and the
  analytics panel reloads for the new symbol. Escape, × or backdrop click closes without change.
- **Acceptance criteria:**
  1. Given the modal opens, then the input has focus and the 4 chips are shown.
  2. Given query "BANKNIFTY", then results include NIFTY BANK (index) and the current BANKNIFTY futures, and no option contract.
  3. Given query "reliance oct fut", then the RELIANCE October future appears with an F badge.
  4. Given a result is selected by Enter, then the chart shows that instrument within 1 s (cached) and the modal is closed.
  5. Given Escape is pressed, then the modal closes and the chart symbol is unchanged.
- **Data/Kite dependency:** instrument master (FR-WATCH-008).
- **Deviation from Streak:** Escape closes (§3.2 #8); BFO futures included so SENSEX/BANKEX futures are chartable; option
  contracts are not searchable (options out of scope, §10 CHART-005-opt). Library note: `[Either]` (own modal).
- **Priority hint:** Core

### FR-CHART-006 Refresh chart data
- **Source features:** CHART-004
- **Requirement:** The refresh icon (tooltip "Refresh chart data") shall re-fetch from Kite the candles of the visible
  range plus the current session for the chart's symbol/timeframe, replace those candles in the local store, and redraw
  without losing studies, drawings or zoom. The icon spins while fetching and is disabled until done.
- **Acceptance criteria:**
  1. Given a cached candle with a wrong close, when refresh is clicked, then the candle shows Kite's current value.
  2. Given refresh is in progress, then a second click has no effect.
  3. Given zoomed to a range, after refresh the same range is still visible.
- **Data/Kite dependency:** historical API via FR-DATA-003; candle store FR-DATA-002 (`source` = manual-refresh).
- **Deviation from Streak:** none. Library note: `[Either]`.
- **Priority hint:** Important

### FR-CHART-007 Save and clear chart layout
- **Source features:** CHART-005
- **Requirement:** "Save" (tooltip "Save indicators and drawings") shall persist the chart's layout: the study list with
  parameters/styles applies to the chart **context** (e.g. dashboard, Scalper chart 1/2/3, full view) for all symbols;
  drawings are stored per context **and** per symbol (they are price/time anchored). "Clear" (tooltip "Clear indicators
  and drawings") shall ask for confirmation ("Remove all indicators and drawings from this chart?" Cancel/Remove) and then
  remove all studies and the current symbol's drawings and persist that. Unsaved layout changes are marked by a dot on
  the Save button; on context close/app quit unsaved layout changes are discarded (studies and drawings revert to the last
  save), matching the explicit-save model.
- **Acceptance criteria:**
  1. Given RSI is added and Save clicked, when the app restarts, then the dashboard chart shows RSI for any symbol.
  2. Given RSI is added without Save, when the app restarts, then RSI is absent and the Save dot was visible before.
  3. Given a trend line drawn on RELIANCE and saved, when the chart switches to TCS, then the line is not shown; switching
     back to RELIANCE shows it.
  4. Given Clear → Cancel, then nothing changes; Clear → Remove removes studies and drawings and survives restart.
- **Data/Kite dependency:** local only (layout JSON in SQLite).
- **Deviation from Streak:** Persistence scope defined explicitly (open in catalogue); Clear confirms. Library note:
  `[Full-TV]` provides layout save/load APIs; `[Open+Own]` requires our own serialisation of studies and drawings.
- **Priority hint:** Important

### FR-CHART-008 Indicators dialog
- **Source features:** CHART-006
- **Requirement:** "Indicators" shall open a modal with a search box and a single flat list of studies (header "Name"),
  containing all 107 studies of Appendix J (52 Week High/Low … Zig Zag). Typing filters the list case-insensitively and
  highlights the matched substring. Clicking/Enter on a row adds that study immediately with default inputs; the dialog
  stays open for multiple adds; Escape/× closes. Oscillator-type studies open in their own pane below price (e.g. RSI 14
  with 30/70 band shading); overlay studies draw on the price pane. Study calculations shall match the reference
  implementations defined for the indicator engine (golden vectors, NFR-06) and, where a study also exists as a strategy
  condition operand, use the same code path.
- **Acceptance criteria:**
  1. Given the dialog, then the list contains exactly the 107 names of Appendix J.
  2. Given query "rsi", then Relative Strength Index, Stochastic RSI and Connors RSI are listed with "RSI" highlighted.
  3. Given RSI is clicked twice, then two RSI panes exist and the dialog is still open.
  4. Given RSI(14) on a golden candle series, then the plotted values equal the golden vector to 1e-6.
  5. Given Escape, then the dialog closes.
- **Data/Kite dependency:** none / local indicator engine.
- **Deviation from Streak:** Escape closes. Library note: `[Full-TV]` ships these studies; `[Open+Own]` requires us to
  implement all 107 studies (large effort; shared with EQB indicator engine where overlapping) and the dialog.
- **Priority hint:** Core (dialog and common studies); Important (complete set of 107)

### FR-CHART-009 Study configuration and pane controls
- **Source features:** CHART-007
- **Requirement:** Each study shall have a legend entry in its pane showing name, inputs and current values, with
  actions (visible on hover and keyboard focus): show/hide (eye), settings (gear), remove (trash), more (⋯: move to
  new pane/existing pane, move up/down, duplicate). Settings opens a dialog titled with the study name with tabs Inputs
  (all numeric/select inputs with defaults, e.g. RSI: Length 14, Smoothing line SMA/EMA/WMA/…, Smoothing length 14),
  Style (colour, line width, line style, band/fill visibility) and Visibility (per-timeframe show/hide); footer
  "Defaults ▾" (Reset to defaults / Save as default), Cancel, OK. Each pane has move-down, collapse and maximise
  controls. The legend can be collapsed (–) and the study list expanded/collapsed (˄). Input validation rejects
  non-numeric, zero/negative lengths and out-of-range values with an inline message.
- **Acceptance criteria:**
  1. Given RSI is configured to Length 21 and OK, then the RSI line recomputes with 21 and the legend shows "RSI 21".
  2. Given Length is set to 0 or -5, then OK is disabled and an inline error "Must be a whole number ≥ 1" is shown.
  3. Given the eye is toggled, then the study is hidden but remains in the legend.
  4. Given "Save as default" for RSI with Length 21, then the next added RSI uses 21.
  5. Given Visibility excludes 1 Day, then switching to 1 Day hides that study.
- **Data/Kite dependency:** none / local.
- **Deviation from Streak:** Invalid inputs rejected (Streak §3.2 #7 accepts invalid periods in builder dialogs; same
  rule applied here). Library note: `[Full-TV]` native; `[Open+Own]` our own legend overlay, dialogs and pane management.
- **Priority hint:** Important

### FR-CHART-010 Drawing toolbar and drawing tools
- **Source features:** CHART-008
- **Requirement:** A collapsible left-edge drawing toolbar (toggle tab `›`, state persisted) shall offer these groups,
  each with a flyout of variants and the last-used variant shown: Cursors (Cross, Dot, Arrow); Lines (Trend Line Alt+T,
  Ray, Info Line, Extended Line, Trend Angle, Horizontal Line Alt+H, Horizontal Ray Alt+J, Vertical Line Alt+V, Cross
  Line Alt+C, Parallel Channel); Fibonacci/Gann (Fib Retracement, Fib Extension, Gann Fan); Patterns (XABCD Pattern,
  Head and Shoulders); Prediction & measurement (Long Position, Short Position, Price Range, Date Range); Shapes (Brush,
  Rectangle, Ellipse, Path); Annotation (Text, Callout, Note); Icons (emoji/icon stamps); plus single tools Measure,
  Zoom In, Magnet mode (snap to OHLC), Stay in drawing mode, Lock all drawings, Hide all drawings, Remove all drawings
  (with confirmation). Drawings are selectable, movable, resizable by handles, deletable (Delete key), and editable
  (colour, width, style, text) via a floating mini-toolbar. Drawings persist only via Save (FR-CHART-007). Shortcuts are the default chart-scope bindings of the shortcut registry (FR-SHELL-019) and use Alt on Linux.
- **Acceptance criteria:**
  1. Given Alt+H then a click on the price pane at 22,500, then a horizontal line at 22,500 is created.
  2. Given Magnet is on, when a trend line endpoint is dropped near a candle high, then it snaps to that high.
  3. Given "Lock all drawings", then drawings cannot be moved until unlocked.
  4. Given "Remove all drawings" → confirm, then no drawings remain on the chart.
  5. Given a Long Position tool is placed with entry/stop/target, then it shows risk/reward ratio and amounts.
  6. Given "Stay in drawing mode" is off, after drawing one line the cursor returns to Cross.
- **Data/Kite dependency:** none / local.
- **Deviation from Streak:** Variant lists beyond those observed are our proposed minimum set (Streak's full TV list was
  not transcribed; see open questions); Alt replaces ⌥. Library note: `[Full-TV]` native (all ~80 TV tools);
  `[Open+Own]` every tool must be built on the library's primitive/plugin API — scope the variant set accordingly.
- **Priority hint:** Important (lines, horizontal/vertical, Fib retracement, text, measure, remove-all); Nice-to-have (rest)

### FR-CHART-011 In-chart readouts, scales and navigation
- **Source features:** CHART-009
- **Requirement:** Every chart shall show: an OHLC legend `O H L C change (pct%)` coloured by candle direction for the
  candle under the crosshair (last candle otherwise); a Volume legend and green/red volume histogram (indices show none);
  a right price scale with Indian grouping and a last-price label coloured by direction; a crosshair with time label
  (`01 Oct '26 09:15`) and price label; a "+" button on the crosshair price label offering "Create order at <price>" (FR-CHART-014, when trading is
  enabled) and "Create price alert at <price>" (FR-ORD-016); a gear (bottom-right) opening chart settings (FR-CHART-015); auto-scale (A)
  and log-scale (L) toggles at the bottom-right of the price scale (always available). Navigation: drag to pan,
  mouse-wheel/pinch to zoom time, drag price scale to zoom price, double-click price scale to reset auto-scale; keyboard:
  Left/Right pan, +/− zoom, Home = jump to latest candle.
- **Acceptance criteria:**
  1. Given the crosshair is over the 09:15 candle of 01 Oct 2026, then the time label reads `01 Oct '26 09:15` and the
     legend shows that candle's OHLC.
  2. Given L is toggled on, then the price scale is logarithmic; A on re-enables auto-scaling.
  3. Given an index chart, then no volume histogram is drawn.
  4. Given the Home key, then the latest candle is scrolled into view.
- **Data/Kite dependency:** none / local.
- **Deviation from Streak:** A/L toggles always visible (Streak showed them only with the panel collapsed). Library note:
  `[Either]` (crosshair/scales native in both; "+" order button `[Full-TV]` native / `[Open+Own]` custom overlay).
- **Priority hint:** Core

### FR-CHART-012 Buy / Sell overlay buttons
- **Source features:** CHART-010
- **Requirement:** At the top-left of the price pane each chart shall show a blue "BUY <best ask>" button, the spread
  between them, and a red "SELL <best bid>" button, updated from market depth; plus, when available, the required margin
  for the default quantity under each button. Clicking opens the order ticket (ORD-004) prefilled with side, instrument,
  order type MARKET, quantity = default quantity (1 share, or 1 lot for F&O, or remembered F&O quantity per FR-ACCT-002);
  it never places an order directly (NFR-05). When no bid/ask is available (market closed, illiquid), the buttons show
  "—" and still open the order ticket. For indices the buttons are hidden. The buttons are hidden when trading is not
  enabled (FR-ACCT-008) and replaced by a small "Signal-only mode" tag. Settings → Chart → "Show Buy/Sell buttons"
  (default on) controls visibility.
- **Acceptance criteria:**
  1. Given best bid 1249.30 / ask 1249.50, then the buttons read `BUY 1249.50`, spread `0.20`, `SELL 1249.30`.
  2. Given BUY is clicked on a NIFTY futures chart, then the order ticket opens with BUY, MARKET, qty = 1 lot (lot size from
     the instrument master) and no order is sent.
  3. Given market closed, then the buttons show "—" (not 0.00).
  4. Given an index chart, then no Buy/Sell buttons are shown.
  5. Given margin for 1 lot is ₹8,908.25, then "₹8,908.25" appears under BUY (refreshed at most every 10 s).
- **Data/Kite dependency:** ticker `full` mode (5-level depth) or `GET /quote`; `POST /margins/orders` (debounced,
  ≤1 req/10 s per chart); order ticket (ORD-004); static IP + trading enablement (C-04, C-05, C-06).
- **Deviation from Streak:** Opens the order ticket instead of a possibly one-click order (NFR-05 safety); "—" instead of
  0.00 (§3.2 #19); hidden in signal-only mode (C-04). Library note: `[Full-TV]` has native buy/sell buttons via its
  broker API; `[Open+Own]` React overlay.
- **Priority hint:** Important

### FR-CHART-013 Chart context menu and Trade submenu
- **Source features:** CHART-011
- **Requirement:** Right-click (or Shift+F10) on the chart shall open a menu: "Copy price <price>", "Paste" (paste a copied
  drawing; Ctrl+V), "Trade ›", "Lock vertical cursor line by time", "Remove <N> indicators", "Hide marks on bars",
  "Settings…". The Trade submenu (only when trading is enabled) offers, for the price under the cursor relative to LTP:
  "Buy/Sell 1 <EXCH>:<SYMBOL> @ <price> limit", "Buy/Sell 1 … @ <price> stop" (SL-M), "Buy/Sell 1 … stop <price> limit"
  (SL), "New order… (Shift+T, FR-SHELL-019)" and "Trading settings…" (opens order defaults in Settings). Each Trade item opens the
  order ticket prefilled (type, trigger/limit price, quantity default) — never places directly. Quantity default is
  1 share / 1 lot / remembered F&O qty.
- **Acceptance criteria:**
  1. Given right-click at price 1,249.30 on RELIANCE with LTP 1,260, then the submenu offers "Buy 1 NSE:RELIANCE @ 1,249.30
     limit" and "Sell 1 NSE:RELIANCE @ 1,249.30 stop".
  2. Given "Copy price", then the clipboard contains `1249.30`.
  3. Given 2 studies, then the menu shows "Remove 2 indicators"; choosing it removes both.
  4. Given trading is not enabled, then "Trade ›" is absent.
  5. Given Shift+T on a focused chart, then the order ticket opens for the charted instrument.
- **Data/Kite dependency:** order ticket → `POST /orders/regular` (LIMIT/SL/SL-M) via ORD module; clipboard (Wails).
- **Deviation from Streak:** Symbol shown as `NSE:RELIANCE` (Kite convention) instead of `NSE_RELIANCE`; always via order
  window (NFR-05). Library note: `[Full-TV]` native trading-terminal menu via broker adapter; `[Open+Own]` our own menu.
- **Priority hint:** Important

### FR-CHART-014 Order and position lines on chart (chart trading)
- **Source features:** CHART-011
- **Requirement:** When trading is enabled and Settings → Chart → "Show orders and positions on chart" is on (default on),
  each chart shall draw a horizontal line for each open order of the charted instrument (label: side, type, qty, price;
  × to cancel) and for an open net position (label: qty, average price, live P&L; × to exit). Dragging an order line
  proposes a modification: on release a confirmation "Modify order to <new price>?" appears; confirm sends a modify via
  the ORD layer (subject to the 25-modifications-per-order limit, C-05). × on an order line asks "Cancel this order?";
  × on a position line opens the order ticket prefilled to exit the full quantity. Lines update from Kite order-update
  events.
- **Acceptance criteria:**
  1. Given an open LIMIT BUY 10 @ 1,240 on RELIANCE, then a line at 1,240 labelled "BUY LMT 10" is drawn.
  2. Given the line is dragged to 1,235 and confirmed, then a modify request with price 1235 is sent once; on reject the
     line returns to 1,240 and an ERROR toast shows Kite's message.
  3. Given an order has been modified 25 times, when it is dragged again, then the drag is refused with "Modification
     limit reached".
  4. Given the order is cancelled elsewhere, then the line disappears within 2 s of the order update.
- **Data/Kite dependency:** `GET /orders`, ticker `OnOrderUpdate`, `PUT/DELETE /orders/:variety/:id`, `GET /portfolio/positions`.
- **Deviation from Streak:** Every modify/cancel is confirmed (NFR-05). Library note: `[Full-TV]` native order/position
  lines; `[Open+Own]` custom overlay primitives.
- **Priority hint:** Nice-to-have

### FR-CHART-015 Chart settings dialog
- **Source features:** CHART-012
- **Requirement:** The gear or "Settings…" shall open a dialog with tabs: **Symbol** — candles: colour bars based on
  previous close (default off), Body (on; up/down colours green/red), Borders (on), Wick (on); data: Precision (Default,
  1, 0.1, 0.01, 0.05 tick); Timezone fixed "(UTC+5:30) Kolkata" (read-only); **Status line** — show symbol, OHLC values,
  bar change, volume, indicator titles, indicator inputs, indicator values (all default on); **Scales and lines** — last
  price label, price line, grid lines (vertical/horizontal), session breaks (default off), scale placement right/left;
  **Canvas** — background colour, grid colour, crosshair style, watermark (off); **Trading** — show Buy/Sell buttons,
  show orders and positions, confirm before modifying by drag (always on, read-only). Footer: "Template ▾" (Save as
  template, Apply template, Reset to defaults), Cancel, OK. Settings apply per chart context and persist on OK.
- **Acceptance criteria:**
  1. Given Body up colour is changed to blue and OK, then up candles render blue and persist after restart.
  2. Given Cancel, then no setting changes.
  3. Given a template saved from the dashboard chart, when applied to Scalper chart 1, then its settings match.
  4. Given "Reset to defaults", then all settings return to the defaults listed above.
- **Data/Kite dependency:** none / local.
- **Deviation from Streak:** Tab contents beyond Symbol were not observed in Streak; contents above are our definition.
  Library note: `[Full-TV]` native dialog; `[Open+Own]` our own dialog mapping to library options.
- **Priority hint:** Nice-to-have

### FR-CHART-016 Chart symbol and timeframe persistence
- **Source features:** CHART-013
- **Requirement:** The dashboard main chart shall persist its last symbol and timeframe locally and restore them at start
  and on returning to Home. Each Scalper chart slot persists its own symbol/timeframe. Builder charts take the strategy's
  instruments/timeframe and do not persist separately.
- **Acceptance criteria:**
  1. Given RELIANCE at 15 Minutes on the dashboard, when the app restarts, then RELIANCE at 15 Minutes is shown.
  2. Given the user navigates Home → Scanners → Home, then the same symbol/timeframe is shown.
  3. Given Scalper chart 2 set to a NIFTY futures contract, after restart chart 2 still shows it (if the contract has not
     expired; otherwise it falls back to the current-month contract of the same underlying with an INFO toast).
- **Data/Kite dependency:** local preference store.
- **Deviation from Streak:** Expired-contract fallback defined. Library note: `[Either]`.
- **Priority hint:** Important

### FR-CHART-017 Full-view chart
- **Source features:** CHART-013
- **Requirement:** The system shall provide a full-view chart view opened with parameters (instrument, timeframe), used by
  the chart action on scanner result rows (SCAN-014) and available from any chart's ⋯ menu ("Open in full view"). It
  shows the full chart component (header with symbol search, LTP/%, timeframe, Save, Clear, Indicators; OHLC readout;
  Buy/Sell; volume) filling the content area, with a back arrow returning to the originating view with its state intact.
  F11 (default binding, FR-SHELL-019) toggles hiding the sidebar/top bar for a distraction-free chart.
- **Acceptance criteria:**
  1. Given a scanner result row for ADANIENT at 1 Day, when its chart action is used, then the full-view chart shows
     ADANIENT NSE at 1 Day.
  2. Given the back arrow, then the scanner results view reappears with the same results and scroll position.
  3. Given F11, then only the chart is visible; F11 again restores the frame.
- **Data/Kite dependency:** as FR-CHART-003.
- **Deviation from Streak:** Streak opens a new browser tab (`/chartiq?seg_sym=…`); Wails v2 supports a single window (C-13), so
  the chart opens as an in-app view with back navigation. Library note: `[Either]`.
- **Priority hint:** Important

---

## FR module ANLY — Analytics panel

The Advance/Decline chart is a simple line chart and is `[Either]` with respect to C-12 (any open chart library
suffices). Streak's Option Chain, OI, PCR and ATM IV/Straddle tabs are out of scope (§10).

### FR-ANLY-001 Analytics tab bar and symbol-dependent tab set
- **Source features:** ANLY-001
- **Requirement:** Under the dashboard main chart (and in the Scalper bottom-left area for the left chart's symbol) the
  system shall show a tab bar (icon + label, active tab filled) whose tab set is derived from the charted instrument via the
  instrument master and universe store: **Technicals** (FR-ANLY-003/004/005) — always; **Advance/Decline** (FR-ANLY-011) — only
  if the instrument is an index with a constituent list in the universe store (FR-WATCH-010). Stocks and futures therefore show
  Technicals only. If the active tab is not in the new set after a symbol change, Technicals becomes active; otherwise the active
  tab is kept.
- **Acceptance criteria:**
  1. Given NIFTY 50 with a Nifty 50 constituent list imported, then two tabs are shown: Technicals and Advance/Decline.
  2. Given RELIANCE or a NIFTY futures contract, then only Technicals is shown.
  3. Given NIFTY 50 without an imported constituent list, then only Technicals is shown.
  4. Given Advance/Decline active on NIFTY 50, when the chart switches to RELIANCE, then Technicals becomes active.
- **Data/Kite dependency:** instrument master (segment, instrument_type); universe store.
- **Deviation from Streak:** Streak's Option Chain, OI, PCR and ATM IV/Straddle tabs are dropped with the options scope (§10
  ANLY-006/007/008/009/012 and ANLY-001-opt). Non-F&O and futures tab sets defined explicitly.
- **Priority hint:** Important

### FR-ANLY-002 Analytics panel collapse/expand
- **Source features:** ANLY-001
- **Requirement:** A "Collapse" button at the right of the tab bar shall hide panel content (tab bar stays docked at the
  bottom with an "Expand" button) and let the chart grow to fill the freed height; "Expand" restores it. Panel height is
  fixed at about 270 px with internal scrolling; the user may drag the divider between chart and panel (min 180 px,
  max 60 % of column) and the height and collapsed state persist per context (dashboard/Scalper). In Scalper, Collapse
  collapses both bottom panels together.
- **Acceptance criteria:**
  1. Given Collapse, then panel content is hidden and the chart height increases by the panel height.
  2. Given the app restarts after collapsing, then the panel is still collapsed.
  3. Given the divider is dragged to 400 px, then the panel is 400 px tall and remains so after restart.
- **Data/Kite dependency:** local only.
- **Deviation from Streak:** Resizable divider added (fixed height in Streak).
- **Priority hint:** Nice-to-have

### FR-ANLY-003 Technicals — sentiment gauge
- **Source features:** ANLY-002
- **Requirement:** The Technicals tab shall show a horizontal red→grey→green gauge with labels `Bearish N`, `Neutral N`,
  `Bullish N` and a pointer positioned at (Bullish − Bearish) ÷ 27 on a −1…+1 scale. N are counts of 27 indicator votes
  computed on the chart's timeframe from the last **closed** candle: 15 trend votes — close vs SMA and EMA of 10, 20, 30,
  50, 100, 200 (12 votes: close > MA → Bullish, < → Bearish, insufficient data → Neutral), Ichimoku base line (9, 26, 52,
  26), VWMA 20, Hull MA 9 (same rule); 12 oscillator votes — RSI 14 (<30 rising → Bullish, >70 falling → Bearish), Stochastic
  %K 14,3,3 (<20 and %K>%D → Bullish, >80 and %K<%D → Bearish), CCI 20 (<−100 rising → Bullish, >100 falling → Bearish),
  ADX 14 (>20 and +DI>−DI rising → Bullish, >20 and −DI>+DI → Bearish), Awesome Oscillator 5,34 (AO > 0 and rising → Bullish, AO < 0 and falling → Bearish),
  Momentum 10 (rising → Bullish, falling → Bearish), MACD 12,26,9 (MACD>signal → Bullish, < → Bearish), Stochastic RSI
  3,3,14,14 (<20 rising → Bullish, >80 falling → Bearish), Williams %R 14 (<−80 rising → Bullish, >−20 falling → Bearish),
  Bull/Bear Power 13 (Bull Power = High − EMA13 > 0 and rising → Bullish; Bear Power = Low − EMA13 < 0 and falling → Bearish), Ultimate Oscillator 7,14,28 (>70 → Bullish, <30 → Bearish), Rate of Change 9
  (>0 rising → Bullish, <0 falling → Bearish); all other cases Neutral. This rule table is normative and is encoded in the golden tests (NFR-06). The gauge shows "Updated <time>" and recomputes within 2 s of each candle close (NFR-08, app clock FR-DATA-004).
- **Acceptance criteria:**
  1. Given any instrument with sufficient history, then Bearish + Neutral + Bullish = 27.
  2. Given a golden candle series and the rule table, then counts equal the golden expected counts.
  3. Given a 1 Hour chart and the 11:15 candle closes, then by 11:15:02 the gauge reflects that candle.
  4. Given a newly listed stock with 15 daily candles, then votes needing 200 candles are Neutral.
- **Data/Kite dependency:** local indicator engine over local candles (historical + ticker).
- **Deviation from Streak:** Streak's vote composition is unpublished; we define our own 27-vote set (same count for
  parity). Latency < 2 s instead of Streak's documented 6–7 min lag.
- **Priority hint:** Important

### FR-ANLY-004 Technicals — pivot ruler and verdict rows
- **Source features:** ANLY-002
- **Requirement:** The Technicals tab shall show a pivot ruler with ticks S3, S2, S1, P, R1, R2, R3 and their values, using
  classic floor pivots computed from the previous completed period's high H, low L and close C: P=(H+L+C)/3, R1=2P−L,
  S1=2P−H, R2=P+(H−L), S2=P−(H−L), R3=H+2(P−L), S3=L−2(H−P). Period by chart timeframe: 1–15 min → previous trading day;
  30 min and 1 Hour → previous week; 1 Day → previous month. Below, three verdict rows: "Short term (SMA 5 / SMA 20)" and
  "Long term (SMA 20 / SMA 50)" → Bullish (green) if fast > slow, Bearish (orange) if fast < slow, "—" if insufficient
  data, with "crossed N candles ago" when a crossover happened within the last 20 candles; "RSI (14)" → value rounded to
  integer and Bullish (≥60), Bearish (≤40) or Neutral, e.g. `51 – Neutral`. Hover/focus on ruler ticks shows the formula
  basis ("Previous week H/L/C").
- **Acceptance criteria:**
  1. Given previous day H=110, L=90, C=100 on a 5-minute chart, then P=100, R1=110, S1=90, R2=120, S2=80, R3=130, S3=70.
  2. Given the chart switches 5 Minutes → 1 Day, then the pivots use the previous month's H/L/C.
  3. Given RSI 37, then the row reads `37 – Bearish`; RSI 51 → `51 – Neutral`.
  4. Given SMA5 crossed above SMA20 three candles ago, then Short term shows Bullish and "crossed 3 candles ago".
- **Data/Kite dependency:** local candles (daily/weekly/monthly aggregates from day candles).
- **Deviation from Streak:** Streak shows S3–R3 without P and does not document the pivot period; we show P and define
  periods. Thresholds defined explicitly.
- **Priority hint:** Important

### FR-ANLY-005 Technicals — recent events table
- **Source features:** ANLY-002
- **Requirement:** The Technicals tab shall list detected pattern/indicator events on the chart timeframe in an `Event |
  Time` table, newest first, showing the 5 most recent with internal scrolling up to 20. Detected events: Heikin Ashi
  bullish/bearish reversal, Bullish/Bearish engulfing, Bullish/Bearish harami, Bullish inside bar, Doji, Three black crows,
  Three white soldiers, Bullish/Bearish CCI crossover (±100), Bullish/Bearish RSI divergence (RSI 14), Bullish/Bearish
  money-flow trend (MFI 14 crossing 50), Volatility expansion (ATR 14 > 1.5 × its 20-period average). Time = start time of
  the candle on which the event completed, format `09 Oct 2026 15:15`. Clicking/Enter on an event scrolls the chart to that
  candle and shows a temporary marker. Each row has a hover/focus tooltip explaining the event in plain language (our own
  one- or two-sentence description per event type, from a bundled text table).
- **Acceptance criteria:**
  1. Given a golden series containing a bullish engulfing at 09 Oct 2026 14:15 (1h), then a row "Bullish engulfing |
     09 Oct 2026 14:15" is listed.
  2. Given 7 events, then the 5 newest are visible and the other 2 reachable by scrolling, newest first.
  3. Given an event row is activated, then the chart centres that candle with a marker.
  4. Given hover or focus on a "Bullish RSI divergence" row, then a tooltip with the bundled explanation for that event type
     appears within 500 ms.
- **Data/Kite dependency:** local pattern engine over local candles.
- **Deviation from Streak:** Event rows navigate the chart (not clickable in Streak); event definitions and tooltip texts are ours (Streak shows an
  explanatory tooltip per event row, REVIEW-2 N1, with its own copy).
- **Priority hint:** Nice-to-have

### FR-ANLY-010 Analytics time-series recorder and backfill
- **Source features:** ANLY-006
- **Requirement:** While the app runs during the continuous session (FR-DATA-001), a background recorder shall store 1-minute
  snapshots for each index that has a constituent list (FR-WATCH-010) and is a "tracked index" (default NIFTY 50, NIFTY BANK,
  SENSEX, plus any such index opened in the analytics panel that day): the index LTP and the advance/decline counts over its
  constituents (LTP vs previous close). Retention default 30 days (configurable in Settings → Data). Backfill for missed periods
  uses the constituents' Kite minute candles through FR-DATA-003 at analytics-recorder priority; user-visible fetches and
  deployments take precedence. The recorder uses the subscription manager at a priority below deployments, live scanners and
  charts (C-07) and must not degrade live signal latency (NFR-08).
- **Acceptance criteria:**
  1. Given the app runs 09:15–15:30, then 375 snapshots per tracked index exist for that day.
  2. Given the app starts at 11:00, then 09:15–10:59 is backfilled from the constituents' minute candles within the rate limit and
     marked as backfilled.
  3. Given retention 30 days, then data older than 30 days is purged on the next start.
  4. Given a deployment signal and recorder backfill compete, then the signal is evaluated within 2 s of candle close.
- **Data/Kite dependency:** ticker; historical minute candles via FR-DATA-003; local time-series table in `app.db`.
- **Deviation from Streak:** Required because Kite offers no breadth history (C-03, C-08); history exists only from first
  install plus what Kite minute candles allow. OI/ATM recording is dropped with the options scope (§10 ANLY-010-opt).
- **Priority hint:** Nice-to-have

### FR-ANLY-011 Advance/Decline tab
- **Source features:** ANLY-006
- **Requirement:** For indices with a constituent list, the tab shall show a two-line chart over the last 2 trading
  sessions at 1-minute resolution: `Advance` (teal) = number of constituents with LTP > previous close, `Decline` (red) =
  number with LTP < previous close (unchanged excluded), y-axis 0…constituent count, "As of <time>" top-right. Tooltip:
  `09 Oct 09:57`, `Advances: 48`, `Declines: 2`. Data from the recorder (FR-ANLY-010), backfilled from constituents'
  minute candles via FR-DATA-003. If the list is stale (older than the user-set threshold, default 90 days) a note "Constituent list
  imported <date>" is shown (survivorship, C-09).
- **Acceptance criteria:**
  1. Given 50 constituents with 48 up, 1 flat, 1 down at 09:57, then Advances 48, Declines 1.
  2. Given NIFTY 50 without an imported list, then the tab is not shown (FR-ANLY-001).
  3. Given a 120-day-old list, then the stale note is visible.
- **Data/Kite dependency:** universe store (user-imported constituents, C-09); ticker; historical minute candles.
- **Deviation from Streak:** Constituents user-imported (C-09); history only as recorded/backfilled.
- **Priority hint:** Nice-to-have

### FR-ANLY-013 Analytics refresh and staleness
- **Source features:** ANLY-001, ANLY-002, ANLY-006
- **Requirement:** Each analytics tab shall show its data time ("As of …") and refresh: Technicals on each candle close of the
  chart timeframe; Advance/Decline every minute during the continuous session (FR-DATA-001). A tab whose data is older
  than 2 minutes during market hours shows a "Delayed" badge; when offline, "Offline — cached" (NFR-14). Data for a hidden
  (inactive) tab is not polled.
- **Acceptance criteria:**
  1. Given the feed disconnects for 3 minutes in market hours, then visible tabs show "Delayed".
  2. Given the Advance/Decline tab is not active, then no A/D-specific quote calls are made (the recorder continues).
- **Data/Kite dependency:** as per tab.
- **Deviation from Streak:** Staleness made visible (Streak refresh cadence unknown); options tabs' cadence → §10 ANLY-013-opt.
- **Priority hint:** Important

---

## FR module WATCH — Watchlists & instrument selection

### FR-WATCH-001 Watchlist panel and multiple lists
- **Source features:** WATCH-001
- **Requirement:** The dashboard right column (and the Scalper watchlist drawer, SCALP-008) shall show a Watchlist panel
  with header "Watchlist", a "New watchlist +" action, a segmented tab control of all watchlists (active tab highlighted;
  horizontal scroll when they overflow), a search input (FR-WATCH-004), a delete icon (FR-WATCH-003), and a table with
  columns `Symbol | LTP / Change | Buy/Sell`. A fresh install has one empty list "Watchlist 1". The number of watchlists is
  unlimited; each list holds any number of instruments (no 50-item cap) subject to the live subscription budget (C-07):
  when the total live subscriptions would exceed the budget, rows beyond it update by polling (`GET /quote/ltp`, every
  5 s) and show a "polled" hint. Rows can be reordered by drag-and-drop and by Alt+Up/Alt+Down (default bindings, FR-SHELL-019); order persists. Lists
  are available as chips in the instrument picker (FR-WATCH-007) and as import sources for strategies and scanners. The
  active tab persists.
- **Acceptance criteria:**
  1. Given a fresh install, then one empty list "Watchlist 1" is shown with an empty-state hint "Search to add instruments".
  2. Given 3 lists, when the second tab is clicked, then its rows are shown and it stays active after restart.
  3. Given a list of 60 instruments, then all 60 are shown with live prices (no cap message).
  4. Given row TCS is moved above SBIN with Alt+Up, then the order persists after restart.
  5. Given total subscriptions exceed the budget, then excess rows show the "polled" hint and still update within 5 s.
- **Data/Kite dependency:** local store (SQLite); ticker `quote` mode; `GET /quote/ltp` fallback.
- **Deviation from Streak:** No 50-item cap (NFR-13; only the Kite subscription limit applies, C-07); reordering added
  (absent in Streak); table sorting stays absent (parity).
- **Priority hint:** Core

### FR-WATCH-002 Create and rename watchlist
- **Source features:** WATCH-001
- **Requirement:** "New watchlist +" shall open an inline name field prefilled "Watchlist N" (N = lowest unused number);
  Enter creates the list and makes it active, Escape cancels. Double-clicking a tab (or its context menu "Rename") allows
  renaming. Names are 1–40 characters, trimmed, unique case-insensitively; violations show an inline error.
- **Acceptance criteria:**
  1. Given lists "Watchlist 1" and "Watchlist 2", when New is used and Enter pressed, then "Watchlist 3" is created and active.
  2. Given a rename to an existing name, then the inline error "A watchlist with this name exists" is shown and nothing changes.
  3. Given an empty name, then Enter is refused.
- **Data/Kite dependency:** local only.
- **Deviation from Streak:** Naming/renaming defined (Streak behaviour unobserved).
- **Priority hint:** Important

### FR-WATCH-003 Delete watchlist
- **Source features:** WATCH-004
- **Requirement:** The delete icon (tooltip "Delete watchlist <name>") shall ask for confirmation ("Delete watchlist
  <name> and its N instruments?" Cancel/Delete) and then delete the active list and activate the adjacent one. Deleting
  the last remaining list leaves a new empty "Watchlist 1". Strategies/scanners that imported instruments from the list
  keep their copied instruments.
- **Acceptance criteria:**
  1. Given Delete → Cancel, then the list remains.
  2. Given Delete → Delete on the only list, then an empty "Watchlist 1" exists.
  3. Given a strategy imported the list earlier, after deletion the strategy still has those instruments.
- **Data/Kite dependency:** local only.
- **Deviation from Streak:** Confirmation and last-list behaviour defined (unknown in Streak).
- **Priority hint:** Important

### FR-WATCH-004 Inline search and add to watchlist
- **Source features:** WATCH-002
- **Requirement:** Typing in the watchlist search input shall show a dropdown of matches from the shared instrument search
  (FR-WATCH-008) with rows `symbol, exchange tag … company name`; each row shows a "+" (on hover, focus and keyboard
  highlight). Clicking a row or "+" or pressing Enter adds the instrument to the end of the active list and keeps the
  dropdown open for further adds; instruments already in the list show a check mark instead of "+" and cannot be added
  twice (an INFO toast "Already in <list>" if attempted). Escape closes the dropdown.
- **Acceptance criteria:**
  1. Given query "sbin" and Enter on the first result, then SBIN NSE is appended to the active list.
  2. Given SBIN is already in the list, then its search row shows a check and adding shows "Already in Watchlist 1".
  3. Given query "nifty oct fut", then the NIFTY current-month future appears and can be added.
- **Data/Kite dependency:** instrument master (local).
- **Deviation from Streak:** Duplicate handling defined; keyboard add.
- **Priority hint:** Core

### FR-WATCH-005 Instrument row
- **Source features:** WATCH-003
- **Requirement:** The standard instrument row (used in watchlists, scanner results and the Live Scanners widget) shall
  show: instrument logo or initial, symbol (display name per FR-WATCH-013), exchange tag, LTP, percent change in
  parentheses coloured green/red, and `B` (blue outline) / `S` (red outline) buttons. Hover or keyboard focus reveals a
  chart action (load into the main chart) and, in watchlists, a remove action. Clicking the symbol loads it into the
  context's main chart and highlights the row (selection persists until another row is chosen). For indices, B/S are
  shown disabled with tooltip "Indices cannot be traded". When trading is not enabled, B/S are hidden. B/S open the order
  window prefilled (ORD-004). Remove deletes the row without confirmation and shows an INFO toast with "Undo" (5 s).
- **Acceptance criteria:**
  1. Given row RELIANCE is clicked, then the dashboard chart shows RELIANCE and the row is highlighted.
  2. Given row NIFTY 50, then B and S are disabled with the tooltip.
  3. Given Remove on TCS then Undo within 5 s, then TCS is restored at its previous position.
  4. Given a stock LTP 1,000 vs previous close 1,010, then change shows `(-0.99%)` in red.
- **Data/Kite dependency:** ticker; instrument master; order ticket.
- **Deviation from Streak:** Undo for removal; keyboard-reachable actions (FR-SHELL-015).
- **Priority hint:** Core

### FR-WATCH-006 Watchlist live price subscription
- **Source features:** WATCH-001, WATCH-003
- **Requirement:** The system shall subscribe ticker `quote` mode for instruments of the visible watchlist rows (all rows
  of the active list plus the instruments on screen elsewhere), unsubscribe instruments no longer needed after 30 s, and
  re-subscribe after reconnect. Watchlist subscriptions have lower priority than deployments and open charts in the
  subscription manager (C-07). Outside market hours, the last price and change vs previous close are shown from the last
  quote.
- **Acceptance criteria:**
  1. Given a list switch from A to B, then within 30 s A-only instruments are unsubscribed and B's are subscribed.
  2. Given a reconnect, then all current watchlist instruments receive ticks again without user action.
  3. Given the budget is full of deployment instruments, then the watchlist falls back to polling (FR-WATCH-001 #5).
- **Data/Kite dependency:** gokiteconnect `ticker` (Subscribe/SetMode/Unsubscribe); subscription manager.
- **Deviation from Streak:** Priority rules due to C-07.
- **Priority hint:** Core

### FR-WATCH-007 Instrument picker dialog (multi-select)
- **Source features:** WATCH-005
- **Requirement:** A shared modal "Add instruments" shall be used by the equity builder ("Add instruments"), scanner custom
  symbols ("Add symbols") and any feature needing multiple instruments. It contains: a search input (placeholder e.g.
  "Search, e.g. SBIN, TCS"); universe chips Nifty 50, Nifty 100, Nifty 200, Nifty 500, Bank Nifty (from the universe store; FR-WATCH-009 names) and
  one chip per watchlist; a left result list with a select-all checkbox and rows (checkbox, logo, symbol, exchange tag,
  company name); a right "Selected (n)" list (or "Selected (n / max)" when the caller passes a maximum) with live price,
  percent change and a remove (−) button per row; footer "Save as watchlist" (prompts for a name, creates a watchlist from
  the selection) and "Done" (returns the selection). Clicking a chip lists that universe's/watchlist's instruments in the
  left list; select-all selects all listed. The caller may restrict segments (e.g. strategies: NSE/BSE cash, indices and NFO/BFO futures; scanners: as per
  universe); option contracts never appear (NFR-15). Duplicate selections are ignored. Escape/× closes without applying; Done applies.
- **Acceptance criteria:**
  1. Given the Nifty 50 chip and select-all, then 50 instruments appear on the right and the counter reads "Selected (50)".
  2. Given a caller with max 100 and 100 selected, when another row is ticked, then it is refused with "Up to 100
     instruments" and the counter shows `100 / 100`.
  3. Given no caller maximum, then 300 instruments can be selected.
  4. Given "Save as watchlist" named "Banks", then a watchlist "Banks" with the selection exists.
  5. Given Escape, then the dialog closes and the caller's selection is unchanged.
- **Data/Kite dependency:** instrument master; universe store; `GET /quote/ltp` (≤1000 instruments/call) for live prices of
  the selection.
- **Deviation from Streak:** Fixed 50-instrument cap replaced by a caller-supplied optional maximum (NFR-13); Escape closes
  (§3.2 #8).
- **Priority hint:** Core

### FR-WATCH-008 Instrument master and search engine
- **Source features:** WATCH-005, WATCH-002, WATCH-008
- **Requirement:** The system shall download the Kite instruments dump for NSE, BSE, NFO and BFO (and indices) through
  FR-DATA-003 once per trading day (FR-DATA-001) at the first start after 08:30 IST, and manually on demand (Settings → Data →
  "Refresh now"); store it locally with its download date; **keep only** indices, cash equities (`EQ` and BSE equivalents) and
  futures (`FUT`); drop option, MCX and currency rows at import (NFR-15); and drop expired contracts. Derived futures metadata
  (underlying → expiries, lot size, tick size, weekly/monthly marker = whether the expiry is the last of its month) comes from the
  dump only; nothing is hard-coded. If a lot size changes between dumps, new orders and quantity derivations use the new lot
  size and saved quantities expressed in lots are recomputed in units. Search shall be local, return results within 100 ms for
  any query on the reference machine, match on tradingsymbol, name and the display name (FR-WATCH-013), be case-insensitive and
  token-based ("reliance oct fut"), and rank: exact symbol > symbol prefix > display-name tokens all matched > name contains;
  indices and cash equities before futures on equal rank; nearest expiry first among futures. BSE cash results shall be shown
  for all BSE equities (no BSE-1000 restriction) but flagged when the same company is on NSE (NSE ranked first). If the download
  fails, the previous dump is used and a WARNING toast states its date.
- **Acceptance criteria:**
  1. Given 08:45 IST on a trading day and a dump dated yesterday, then a new dump is downloaded once; given 08:45 on a holiday, then
     no download is attempted.
  2. Given query "BANKNIFTY", then NIFTY BANK (index) appears in the top 3 and no option contract appears for any query.
  3. Given any query on the full dump, then results return in < 100 ms on the reference machine.
  4. Given the download fails, then search still works on yesterday's dump and a WARNING names its date.
  5. Given a contract that expired yesterday, then it no longer appears in search.
  6. Given a fixture dump where the NIFTY lot size changes from 65 to 75, then the next futures order ticket uses 75 and a saved
     Scalper quantity preset of "2 lots" shows 150 units.
- **Data/Kite dependency:** `GET /instruments` / `GET /instruments/:exchange` (gokiteconnect `GetInstruments`/
  `GetInstrumentsByExchange`) via FR-DATA-003; works on the Personal plan.
- **Deviation from Streak:** BSE not restricted to BSE 1000 (Streak restriction is a product limit, NFR-13). Option contracts are
  excluded by scope (NFR-15). Manual refresh and the lot-size rule were folded in from the former options-master FR (RV-049).
- **Priority hint:** Core

### FR-WATCH-009 Universes (scan-on baskets)
- **Source features:** WATCH-006
- **Requirement:** The system shall offer these **21 built-in universes**, in this order, wherever a universe is selected
  (Discover Scanners, scanner builder "Scan on", Live Scanners widget, instrument-picker chips where applicable): Nifty 50,
  Bank Nifty, Sensex, Bankex, Nifty 100, Nifty 200, Nifty 500, Indices, Futures, NFO Equities, Energy, Communication Services,
  Materials, Consumer Discretionary, Consumer Staples, Industrials, Healthcare, Financials, Information Technology, Utilities,
  Real Estate. **One ordered list rule for every universe dropdown:** the 21 built-ins above, then a separator, then the user's
  custom baskets (FR-WATCH-012) alphabetically, then a separator, then the user's watchlists (FR-WATCH-001) in watchlist order;
  the scanner builder additionally shows "Custom symbols" as the last entry. Membership: index baskets and sectors from the
  universe store (FR-WATCH-010); Indices = all index instruments of the instrument master; Futures = current-month futures of
  all NFO/BFO underlyings (index and stock); NFO Equities = cash equities that have NFO futures. Each universe shows its member
  count and data date in a tooltip, and a warning with member count and budget when it exceeds the live subscription budget
  (C-07) in a live context.
- **Acceptance criteria:**
  1. Given any universe dropdown, then the 21 built-in names are listed in the order above, followed by custom baskets and then
     watchlists, each group behind a separator.
  2. Given custom basket "My banks" and watchlist "Watchlist 1", then both appear in the Discover Scanners, widget and scanner
     builder dropdowns at the same positions.
  3. Given "NFO Equities", then every member has at least one NFO future in the instrument master.
  4. Given "Futures" on 20 Oct 2026, then every member is an October-expiry contract; on the first trading day after the October
     expiry the universe resolves to November contracts automatically.
  5. Given a universe exceeds the live budget when used live, then the user is warned with the member count and budget.
- **Data/Kite dependency:** instrument master; universe store; subscription budget (C-07).
- **Deviation from Streak:** The 8 option universes of Streak's 29 are dropped with the options scope (§10 WATCH-009-opt).
  "Futures" basket supported (Conflicts §6 #11 — UI authoritative). Sector membership comes from the imported constituent files'
  Industry field mapped to the 11 sectors (C-09). One placement rule for custom baskets/watchlists (RV-038).
- **Priority hint:** Core

### FR-WATCH-010 Universe constituent store and import
- **Source features:** WATCH-006
- **Requirement:** The system shall ship a snapshot of constituent lists for Nifty 50, Bank Nifty, Nifty 100, Nifty 200,
  Nifty 500, Sensex and Bankex dated at build time, and a Settings → Universes view where the user can: see each list's
  member count and as-of date; import a replacement from a CSV file the user downloaded (format of the public index
  constituent CSVs: Company Name, Industry, Symbol, Series, ISIN Code; also a generic one-symbol-per-line format); preview
  added/removed symbols before applying; and edit membership manually. Symbols not found in the instrument master are
  reported and skipped. A sector mapping table (Industry → one of the 11 sectors) is editable. The app shall never
  download constituent files automatically. Lists older than a threshold (default 90 days) show a "stale" badge.
  Backtests record the list version used (survivorship note, C-09, NFR-12).
- **Acceptance criteria:**
  1. Given a valid Nifty 50 CSV with 2 changes, when imported, then the preview shows +1/−1 and Apply updates the list.
  2. Given a CSV containing an unknown symbol, then the import report lists it and it is not added.
  3. Given an Industry value "Oil Gas & Consumable Fuels" mapped to Energy, then those constituents appear in the Energy
     universe.
  4. Given network monitoring, then the app makes no request to index-provider websites.
- **Data/Kite dependency:** local files; instrument master for symbol validation.
- **Deviation from Streak:** Streak maintains lists server-side with history; we use user-imported current lists (C-09);
  survivorship bias documented.
- **Priority hint:** Core

### FR-WATCH-011 Shared universe preference
- **Source features:** WATCH-006
- **Requirement:** The selected universe of Discover Scanners and of the Live Scanners widget shall be one shared,
  persisted preference: changing it in either place changes the other immediately. The scanner builder's "Scan on" is
  per scanner and does not affect this preference.
- **Acceptance criteria:**
  1. Given the widget universe is changed to Bankex, then Discover Scanners shows Bankex on next open, and after restart both
     show Bankex.
  2. Given a scanner is saved with "Scan on" Nifty 200, then the shared preference is unchanged.
- **Data/Kite dependency:** local preference.
- **Deviation from Streak:** none.
- **Priority hint:** Important

### FR-WATCH-012 Custom baskets (custom symbols)
- **Source features:** WATCH-007
- **Requirement:** In the scanner builder "Scan on" list, a "Custom symbols" entry shall let the user pick instruments via
  the instrument picker (FR-WATCH-007), showing "Custom symbols (n)" or "No symbols selected" and an "Add symbols" button.
  The user may save the selection as a named custom basket (name 1–40 chars, unique) which then appears in every universe list at the position defined by FR-WATCH-009; baskets are managed (rename, edit, delete with confirmation) in Settings →
  Universes. No limit on the number of baskets or instruments per basket beyond the live budget (C-07).
- **Acceptance criteria:**
  1. Given Custom symbols with 12 instruments saved as "My banks", then "My banks" appears in the Discover Scanners and
     widget universe dropdowns.
  2. Given 11 baskets of 80 instruments each, then all can be created (no 10 × 50 limit).
  3. Given a basket used by a saved scanner is deleted, then the confirmation names the affected scanners and the scanners
     keep a frozen copy of the symbols.
- **Data/Kite dependency:** local store.
- **Deviation from Streak:** No 10-basket × 50-symbol cap and no paid-feature marker (NFR-13); baskets are saved
  independently of scanners (resolves catalogue question).
- **Priority hint:** Important

### FR-WATCH-013 Instrument display naming
- **Source features:** WATCH-008
- **Requirement:** One formatter shall render instrument names everywhere from instrument-master fields: equities `SYMBOL`;
  indices by display name (e.g. `NIFTY 50`); futures `UNDERLYING MON FUT` for the current and next month (e.g. `RELIANCE OCT FUT`)
  and `UNDERLYING DD MON YYYY FUT` when two contracts of the same underlying share a month (never observed on NSE; defensive),
  always with an "F" badge in lists. Exchange-qualified identifiers use Kite's `EXCHANGE:TRADINGSYMBOL` form (e.g. `NSE:RELIANCE`,
  `NFO:NIFTY26OCTFUT`) internally and in deep-link parameters. No superscript or ordinal markers are used (own convention,
  RV-024).
- **Acceptance criteria:**
  1. Given a RELIANCE October 2026 future, then the display is `RELIANCE OCT FUT` with an F badge and the identifier is
     `NFO:RELIANCE26OCTFUT`.
  2. Given NIFTY 50, then the display is `NIFTY 50` with exchange tag `INDICES`.
  3. Given an automated check over every rendered instrument in a test fixture, then every name was produced by the one formatter
     (no literal names in components).
- **Data/Kite dependency:** instrument master (`expiry`, `instrument_type`, `name`, `tradingsymbol`).
- **Deviation from Streak:** Identifier format `NSE:RELIANCE` instead of Streak's `NSE_RELIANCE`; Streak's option naming (ordinal
  day + superscript "w") is not reproduced — options are out of scope and the convention is Streak-specific (§10
  WATCH-013-opt).
- **Priority hint:** Core

### FR-WATCH-014 Consistent exchange tagging
- **Source features:** WATCH-008
- **Requirement:** The exchange tag of an instrument shall always be derived from its own instrument-master record and be
  one of `INDICES`, `NSE`, `BSE`, `NFO-FUT`, `BFO-FUT`. Wherever the same instrument appears (chart search, watchlist, picker, scanner results, order ticket) it shall carry
  the same tag (NSE for NSE-listed F&O stocks).
- **Acceptance criteria:**
  1. Given RELIANCE appears in the chart search, watchlist, instrument picker and scanner results, then all show `NSE`.
  2. Given an automated check over all rendered instrument rows in a test fixture, then no instrument has two different tags.
- **Data/Kite dependency:** instrument master.
- **Deviation from Streak:** Fixes inconsistent BSE/NSE tagging (§3.2 #18).
- **Priority hint:** Important

---

## FR module ACCT — Account, settings, notifications and legal

### FR-ACCT-001 Profile view
- **Source features:** ACCT-001
- **Requirement:** The Profile view (header: back arrow, title "Profile", description "Your Zerodha account details as
  returned by Kite") shall show an identity card (avatar from Kite if provided, otherwise initials; name; client ID with a
  "Zerodha" chip) and read-only fields Name, Email, Client ID, Broker ("Zerodha"), plus Exchanges and Products enabled
  (from the profile response). Below: the preference toggles FR-ACCT-002 and FR-ACCT-003 (Telegram per FR-ACCT-014 lives in Settings) (applied immediately, no Save
  button), session information (logged in at, expires at 06:00 IST), and the Log out button (FR-ACCT-004). A "Refresh"
  re-fetches the profile.
- **Acceptance criteria:**
  1. Given a valid session, then Name, Email and Client ID equal `GET /user/profile` values and fields are not editable.
  2. Given a toggle is changed, then it persists without any Save action.
  3. Given login at 09:02 today, then session info shows "Expires 06:00 tomorrow".
- **Data/Kite dependency:** `GET /user/profile` (gokiteconnect `GetUserProfile`).
- **Deviation from Streak:** Session information and exchanges/products added (C-02).
- **Priority hint:** Important

### FR-ACCT-002 Remember futures order quantity
- **Source features:** ACCT-002
- **Requirement:** A toggle "Remember futures order quantity" (help "Reuse the last quantity for NFO-FUT and BFO-FUT orders"),
  default off. When on, after an order ticket for a futures contract is submitted, its quantity in lots is stored per underlying;
  the next order ticket for any futures contract of the same underlying is prefilled with that number of lots (converted with the
  contract's current lot size, FR-WATCH-008). When off, futures order tickets default to 1 lot. Cash-segment quantity is never
  remembered.
- **Acceptance criteria:**
  1. Given the toggle on and a NIFTY futures order of 3 lots submitted, when another NIFTY futures order ticket opens, then
     quantity = 3 lots.
  2. Given the toggle off, then a NIFTY futures order ticket defaults to 1 lot.
  3. Given the lot size changed in the instrument master, then the prefilled quantity = 3 × new lot size.
  4. Given an equity order of 25 shares, then the next equity order ticket does not default to 25.
- **Data/Kite dependency:** local preference; instrument master lot sizes.
- **Deviation from Streak:** Streak's toggle covers NFO-OPT/BFO-OPT too (options out of scope, §10 ACCT-002-opt). Storage key
  (per underlying, in lots) defined explicitly; Streak semantics unknown.
- **Priority hint:** Nice-to-have

### FR-ACCT-003 Desktop notifications (single definition of the OS channel)
- **Source features:** ACCT-003, DEP-010
- **Requirement:** A toggle "Desktop notifications" (Profile and Settings → Notifications; help "Show system notifications for
  signals, orders, scanner alerts and deployments"), **default off** (parity with Streak's push toggle). When on, the system
  shall raise an OS notification (freedesktop notification service via D-Bus) for the event types ticked in a **per-type filter**
  (all on when the toggle is first enabled): deployment entry signal awaiting confirmation; exit signal; order filled / rejected /
  cancelled; deployment stopped / expired / errored / needs attention; scanner alert hit; session expiry; feed offline > 60 s
  during the continuous session; clock out of sync (FR-DATA-004). An optional notification sound (off by default) can be chosen
  per type group (signals, orders, alerts). Clicking a notification focuses the app and opens the related view (order ticket in
  review mode for a pending signal, Notifications otherwise). A "Send test notification" button verifies delivery. In-app toasts
  and the Notifications view are unaffected by the toggle. If the notification service is unavailable, the toggle shows "Not
  available on this desktop" and is disabled. This FR is the only definition of the OS channel; FR-DEP-016 and FR-SCAN-009 refer
  to it. The Telegram channel is FR-ACCT-014.
- **Acceptance criteria:**
  1. Given the toggle on, when a scanner alert fires while the window is minimised, then an OS notification with the scanner and
     instrument names appears within 2 s.
  2. Given the toggle off, then no OS notification is raised but the in-app notification is recorded.
  3. Given an OS notification for an entry signal is clicked, then the app comes to front with the order ticket in review mode for
     that signal.
  4. Given the per-type filter has "scanner alert hit" unticked, then scanner alerts raise no OS notification while deployment
     signals still do.
  5. Given no notification daemon on the session bus, then the toggle is disabled with the explanation.
- **Data/Kite dependency:** OS notification service via D-Bus; local event bus.
- **Deviation from Streak:** OS notifications replace browser push / mobile push (no server, C-03). Per-type filter and sound are
  additions. Default off (Streak default off, ACCT-003; RV-010).
- **Priority hint:** Important

### FR-ACCT-004 Log out
- **Source features:** ACCT-004
- **Requirement:** "Log out" (outline button with icon) shall ask for confirmation; if deployments are running, the dialog
  states that live deployments will stop receiving data and signals until the next login. On confirm the system invalidates
  the Kite access token, deletes it from the OS keyring, closes ticker connections, pauses all deployments and scanners
  (state preserved for resume after login), and shows the login screen. Local data (strategies, backtests, candles) remains.
  The app also enforces a single running instance per data directory: a second launch focuses the existing window and exits
  (this is the only definition; RV-023).
- **Acceptance criteria:**
  1. Given Log out → confirm, then `DELETE /session/token` is called, the keyring entry is removed and the login screen shows.
  2. Given 2 running deployments, then the dialog mentions them; after logout they are "Paused (logged out)" and resume after
     login.
  3. Given the app is running, when it is launched again, then no second instance starts and the existing window is focused.
- **Data/Kite dependency:** `DELETE /session/token` (gokiteconnect `InvalidateAccessToken`); keyring (NFR-04).
- **Deviation from Streak:** Streak's single-session rule becomes single-instance locally; deployments pause rather than
  run server-side (C-03).
- **Priority hint:** Core

### FR-ACCT-005 Kite API credentials setup
- **Source features:** ACCT-005
- **Requirement:** On first run (and from Settings → Kite connection) the system shall present a setup flow that: explains
  that the user needs their own Kite Connect app (Connect plan for data, C-01) and shows the exact redirect URL to register
  (`http://127.0.0.1:<port>/kite/callback`, port configurable, default fixed); collects API key and API secret; stores the
  secret only in the OS keyring (NFR-04; encrypted-file fallback per FR-ACCT-013), never in files or logs; and verifies the credentials by completing a first login
  (FR-ACCT-006). The API key is shown masked (last 4 characters) in Settings with "Change" and "Remove credentials" actions.
- **Acceptance criteria:**
  1. Given a fresh install, when the app starts, then the setup flow is shown before any other view.
  2. Given the secret is entered, then it is in the keyring and a search of the data directory and logs for the secret
     string finds nothing.
  3. Given Remove credentials → confirm, then key, secret and token are deleted and the setup flow is shown.
- **Data/Kite dependency:** OS keyring (libsecret); Kite developer console (user-side).
- **Deviation from Streak:** Streak needs no user API key; we require the user's own (C-01).
- **Priority hint:** Core

### FR-ACCT-006 Daily Kite login and session lifecycle
- **Source features:** ACCT-005, ACCT-004, ACCT-011
- **Requirement:** "Log in with Kite" shall open the Kite login page in the system browser and start a loopback HTTP
  listener on the configured port; on redirect with `request_token` the app exchanges it for an access token (checksum
  SHA-256 of api_key + request_token + api_secret), stores the token in the keyring with expiry at the next 06:00 IST,
  shows "Logged in — you can close this browser tab" in the browser, and focuses the app. Fallback if the loopback
  redirect cannot be used: the user pastes the full redirect URL or the request_token into a field. The login is always
  manual (no stored passwords/TOTP automation). At 06:00 IST the session is treated as expired: live features stop,
  the status indicator shows "Logged out" (FR-SHELL-005), and a banner "Kite session expired — log in to continue" with a
  Log in button is shown. On start with a still-valid token, no login is needed. After each successful login the risk-disclosure acknowledgement
  (FR-SHELL-020) is shown once before any other view. The 06:00 expiry is evaluated on the app clock (FR-DATA-004). The
  login screen is the app's own view (Kite credentials are entered only in Kite's browser page). Any Kite `TokenException` response
  triggers the same expired state.
- **Acceptance criteria:**
  1. Given valid credentials, when the user completes Kite login in the browser, then within 3 s the app shows Home and the
     status indicator shows "Live" (Connect plan).
  2. Given the loopback listener is blocked, when the user pastes the redirect URL, then login completes.
  3. Given the clock passes 06:00 IST, then within 5 s the expired banner appears and no Kite calls are made until login.
  4. Given the app restarts at 14:00 after a 09:00 login, then no login is requested.
  5. Given a `TokenException` from any endpoint, then the expired state is entered.
- **Data/Kite dependency:** Kite login URL, `POST /session/token` (gokiteconnect `GenerateSession`), keyring; all calls via
  FR-DATA-003.
- **Deviation from Streak:** Daily manual login with own key (C-02); loopback redirect acceptance by Kite is unverified,
  hence the paste fallback.
- **Priority hint:** Core

### FR-ACCT-007 Plan capability detection and degraded mode
- **Source features:** ACCT-005
- **Requirement:** After each login the system shall detect whether the API key has market-data permissions (Connect plan)
  by a minimal historical-data request and ticker connection attempt. Without them the app runs in "Orders and portfolio
  only" mode: charts, scanners, analytics, backtests needing new data, and live deployments show a clear explanation
  ("This needs a Kite Connect data plan") instead of empty content; holdings, positions, orders and profile work; cached
  data remains usable offline (NFR-14). The detected mode is shown in Settings and the status indicator.
- **Acceptance criteria:**
  1. Given a Personal-plan key, then the status shows "No data plan" and the chart shows the explanation, while Holdings
     loads.
  2. Given a Connect-plan key, then all features are enabled.
  3. Given a Personal-plan key and cached candles, then a backtest over the cached range runs.
- **Data/Kite dependency:** historical API, ticker (permission errors), portfolio endpoints.
- **Deviation from Streak:** Streak needs no data plan; our dependency is explicit (C-01).
- **Priority hint:** Core

### FR-ACCT-008 Trading enablement and static IP
- **Source features:** ACCT-005, ACCT-006
- **Requirement:** Settings → Trading shall contain "Enable order placement" (default off) and "Registered static IP(s)"
  (up to 2, as registered with Zerodha). Until enabled, the app is in "signal-only mode": all B/S buttons and trade menus
  are hidden or replaced by a "Signal-only mode" tag and the order layer refuses to send orders. Enabling requires
  confirming a statement that order placement uses the user's registered static IP (C-04) and shows the throttle limits
  in force (C-05). If Kite rejects an order for IP reasons, trading is automatically disabled with an ERROR toast
  explaining why. A global kill switch ("Stop all trading", FR-ORD-009) is reachable from the status panel (NFR-05; behaviour in ORD/DEP).
- **Acceptance criteria:**
  1. Given a fresh install, then trading is disabled and no B/S buttons are shown anywhere.
  2. Given trading is enabled, then B/S buttons appear in watchlists, charts and scanner results.
  3. Given an IP-related rejection from Kite, then trading becomes disabled and the toast names the cause.
- **Data/Kite dependency:** order endpoints (rejection messages); local settings.
- **Deviation from Streak:** Static-IP gate and signal-only default are new (C-04, NFR-05).
- **Priority hint:** Core

### FR-ACCT-009 Limits and usage view
- **Source features:** ACCT-006
- **Requirement:** Settings → Limits shall list the limits that apply to the app and the current usage, read from FR-DATA-003
  and FR-ORD-006 counters: Kite limits — orders per second (10; app cap FR-ORD-006), per minute (400), per day (5,000) with
  today's count; modifications per order (25); quote API 1 req/s and historical API 3 req/s, each with the current queue depth
  per priority class and today's HTTP 429 count; WebSocket subscriptions used / 9,000 (3 × 3,000) and connections used / 3, with
  the breakdown by consumer (deployments, live scanners, charts, watchlists, widget, recorder); per-request historical windows.
  The view states that the app imposes no limits of its own on strategies, scanners, deployments, backtests, watchlist sizes or
  custom baskets (NFR-13), and lists app configuration values that act as budgets (candle-store size budget, recorder retention,
  risk limits of FR-ORD-005) with links to change them. When a Kite limit is hit anywhere, the toast links to this view.
- **Acceptance criteria:**
  1. Given 12 orders placed today, then the view shows "12 / 5,000 orders today".
  2. Given 2,400 instruments subscribed (1,800 deployments, 600 charts), then it shows "2,400 / 9,000" and the breakdown.
  3. Given an HTTP 429 from Kite, then a WARNING toast with a "View limits" link appears and the 429 counter increments.
- **Data/Kite dependency:** local counters from FR-DATA-003, FR-ORD-006 and the subscription manager.
- **Deviation from Streak:** Streak's own caps (100 deployments, 25 live scanners, 50 instruments, backtests/day,
  lookback windows, 10 × 50 baskets) are not applied (NFR-13); only Kite/SEBI limits.
- **Priority hint:** Important

### FR-ACCT-010 Built-in help
- **Source features:** ACCT-007
- **Requirement:** The app shall bundle an offline help centre (our own content) opened from the profile menu, (i) icons
  and view help menus, as an in-app view/side panel with a search box (full-text, results as you type), category cards
  (Dashboard & charts, Scalper, Strategies, Scanners, Backtesting, Deployments & orders, Kite setup, Data & backup, FAQ,
  Advanced conditions), articles with a category tree and "Updated for v<x>" stamp, and a "Keyboard shortcuts" article generated from the shortcut registry (FR-SHELL-019). It shall document backtest assumptions, square-off rules, limits and deviations from Streak behaviour.
  An "Open logs folder" link and "Create support bundle" (FR-SHELL-018) are provided for support (NFR-10).
- **Acceptance criteria:**
  1. Given no network, when help is opened and "square-off" searched, then matching articles are listed.
  2. Given the (i) icon of the Advanced conditions header, then the Advanced conditions article opens.
  3. Given the Keyboard shortcuts article, then it lists every binding in the registry (FR-SHELL-019), including remapped ones.
- **Data/Kite dependency:** none (bundled content).
- **Deviation from Streak:** Offline, bundled, own content instead of an external help site; adds a shortcuts list and
  logs access (absent in Streak).
- **Priority hint:** Important

### FR-ACCT-011 Away-from-desk alerting (closest equivalent to the mobile companion)
- **Source features:** ACCT-008
- **Requirement:** The system shall not provide a mobile app. Alerts that Streak delivers to its mobile app shall be delivered
  as desktop notifications (FR-ACCT-003), recorded in the Notifications view (FR-DEP-015) and, when the user enables it, sent to
  the user's own Telegram bot (FR-ACCT-014). No other data leaves the machine (NFR-04). Alerts received on Telegram are
  informational: acting on a signal still requires the desktop app (NFR-05).
- **Acceptance criteria:**
  1. Given a scanner alert with Telegram off, then it is recorded in Notifications and (if enabled) raised as a desktop
     notification, and network monitoring shows no outbound request other than to Kite.
  2. Given Telegram on, then the same alert additionally produces one Bot API call and nothing else.
- **Data/Kite dependency:** none (Telegram per FR-ACCT-014).
- **Deviation from Streak:** No mobile companion (single-user local desktop scope; C-03, NFR-04); Telegram relay is the
  **Enhancement (beyond Streak)** that replaces it (RV-042).
- **Priority hint:** Nice-to-have

### FR-ACCT-012 Settings view
- **Source features:** ACCT-001, ACCT-005, ACCT-006
- **Requirement:** A Settings view (profile menu → Settings) shall group: **Kite connection** (FR-ACCT-005/006/007); **Trading**
  (FR-ACCT-008 enablement and static IPs, order defaults: default product per segment, default order type, market protection
  FR-ORD-007, risk limits FR-ORD-005, `algo_id` field reserved FR-ORD-005, AMO/iceberg/TTL enablement FR-ORD-004, live square-off
  table FR-DEP-021, "Allow automatic execution" FR-DEP-018, GTT mirroring defaults FR-DEP-009, one-click trading FR-SCALP-011);
  **Appearance** (theme, follow system, sidebar collapsed); **Chart** (default timeframe, show Buy/Sell buttons, show orders on
  chart); **Home** (featured strategies and scanners, FR-HOME-004/006); **Universes & baskets** (FR-WATCH-010/012); **Market
  calendar** (FR-DATA-001); **Data** (data directory path, candle-store size and budget, purge rules, "Clear cached candles",
  backfill status, recorder retention, instrument master date with "Refresh now", **Backup now / Restore / Scheduled backup**
  FR-DATA-005); **Notifications** (FR-ACCT-003 desktop, FR-ACCT-014 Telegram); **Shortcuts** (FR-SHELL-019); **Limits**
  (FR-ACCT-009); **Updates** (FR-SHELL-017); **Diagnostics** (log level, open logs folder, support bundle, FR-SHELL-018); **Tours**
  ("Reset all tours"); **About** (version, build, licences of bundled libraries, legal pages FR-ACCT-015, risk disclosure
  FR-SHELL-020, library versions FR-DISC-020). Every setting has a default, validates input inline and applies immediately.
- **Acceptance criteria:**
  1. Given "Clear cached candles" → confirm, then the candle store size becomes ~0 and charts re-fetch on next open.
  2. Given the live square-off time set to "25:00", then an inline error is shown and the old value is kept.
  3. Given About, then the version equals the profile-menu version and third-party licences are listed.
  4. Given an automated check, then every FR that names a setting finds it in exactly one group above.
- **Data/Kite dependency:** local only (SQLite settings, keyring).
- **Deviation from Streak:** Streak has no settings page (its /settings is a 404); a local desktop app needs one.
- **Priority hint:** Core

### FR-ACCT-013 Secret storage fallback
- **Source features:** ACCT-005
- **Requirement:** If no Secret Service (libsecret / D-Bus `org.freedesktop.secrets`) is available at setup or at start, the
  system shall offer two choices and nothing else: (a) an **encrypted secrets file** `<data dir>/secrets.enc` (XChaCha20-Poly1305
  with a key derived by Argon2id from a user passphrase of ≥ 8 characters; the passphrase is asked once per app start and kept
  only in memory), or (b) refuse to store credentials (the app then runs without a Kite session). Plaintext storage is never
  offered. The same store holds the API secret, access token and Telegram bot token (NFR-04). Settings → Kite connection shows
  which backend is in use; switching back to the keyring when it becomes available migrates the secrets and deletes the file.
- **Acceptance criteria:**
  1. Given no keyring daemon, then the setup flow shows exactly the two choices and no "store in settings" option.
  2. Given the encrypted file is chosen, then a search of the data directory for the API secret string finds nothing, and the file
     does not decrypt with a wrong passphrase.
  3. Given the keyring becomes available and the user switches, then the secrets are in the keyring and `secrets.enc` is gone.
- **Data/Kite dependency:** none.
- **Deviation from Streak:** Not applicable to Streak (no local secrets); required by NFR-04 for minimal desktops (RV-044).
- **Priority hint:** Important

### FR-ACCT-014 Telegram bot — settings entry point (Enhancement beyond Streak)
- **Source features:** ACCT-008, ACCT-003, DEP-010
- **Requirement:** Settings → Notifications shall contain a **Telegram** section that is the entry point to the Telegram bot
  defined in full by **FR module TG (FR-TG-001…FR-TG-042)** — setup/pairing, authorization, two-way commands, confirmations,
  notification catalogue and loudness, rate limiting and offline behaviour. The bot is off by default; its token is stored per
  FR-TG-002 / FR-ACCT-013. This FR adds no rules of its own; module TG is the single source of truth (supersedes the v2
  send-only wording, user decision 2026-10-12: two-way bot modelled on freqtrade).
- **Acceptance criteria:**
  1. Given Settings → Notifications, then a Telegram section is present and opens the FR-TG-003 guided setup.
  2. Given Telegram is disabled, then the section shows "Off" and no request is made to the Telegram API (FR-TG-001).
- **Data/Kite dependency:** none (see module TG).
- **Deviation from Streak:** Streak has no Telegram channel (A06 §3); replaces its mobile push for away-from-desk use (RV-042).
- **Priority hint:** Important

### FR-ACCT-015 Legal text pages (terms of use, privacy notice)
- **Source features:** ACCT-009, ACCT-010, DEP-001, SCAN-009
- **Requirement:** The app shall bundle two in-app read-only pages with **our own text**: "Terms of use" (educational and
  decision-support tool; the user is responsible for every order; no automatic trading without explicit arming; no warranty;
  glossary of the app's own product terms per §3.2) and "Privacy notice" (all data stays on the machine; the only outbound hosts
  are Kite and, if enabled, Telegram, the update feed and the IP-echo endpoint; no telemetry; what the support bundle contains).
  They open as full in-app views with a back arrow from: the "trading terms" link in the deploy dialog (FR-DEP-001) and the
  Set Alerts dialog (FR-SCAN-009), Settings → About, and the risk-disclosure screen (FR-SHELL-020). Each page shows its
  "Updated for v<x>" stamp. No network access is needed; no Streak text is reused.
- **Acceptance criteria:**
  1. Given the deploy dialog, when "trading terms" is activated, then the Terms of use view opens and the back arrow returns to the
     dialog with its state intact.
  2. Given Settings → About, then both pages are reachable and render offline.
  3. Given an automated text comparison, then no sentence of either page matches Streak's `/terms` or `/privacy` pages
     (clean-room check against the research screenshots' transcribed headings).
- **Data/Kite dependency:** none.
- **Deviation from Streak:** Streak's `/terms` and `/privacy` are web routes without the app shell (REVIEW-2 N6/N7); ours are
  in-app views with our own text.
- **Priority hint:** Important

---

## FR module DATA — Market calendar, clock, candle store, Kite client, backup

Cross-cutting services that every other module consumes. They exist because the app runs locally without Streak's servers
(C-03, C-08, C-14). No other FR may hard-code a session time, a candle alignment anchor, a Kite rate limit or a storage path;
it cites the FR here instead (REVIEW-1 RV-003/004/005/013).

### FR-DATA-001 Market calendar and session model
- **Source features:** SHELL-003, BT-020, DEP-004, DEP-005, SCAN-010, ACCT-006
- **Requirement:** One calendar service shall define, per exchange (NSE, BSE, NFO, BFO) and per date: trading day / holiday /
  special session, and the session phases with times. Regular-day phases: **pre-open** (09:00–09:08 order entry, 09:08–09:12
  matching, 09:12–09:15 buffer; NSE/BSE cash only), **continuous** (09:15–15:30), **closing** (15:30–15:40, closing-price
  computation, no orders), **post-closing** (15:40–16:00; equity cash only; MARKET orders at the closing price), **closed**
  otherwise. A special session is a dated entry with its own phases and times (e.g. the annual Muhurat evening session, an
  exchange-announced half-day or extended session). The app ships the current year's NSE/BSE trading-holiday list and known
  special sessions as a versioned data file (`calendar version` recorded by FR-BT-044); Settings → Market calendar lets the user
  **import** an updated list (CSV: `date, exchange, type ∈ {holiday, special, trading}, phase, start, end`), **add/edit/delete**
  entries, and shows "Calendar may be stale" when the list does not cover the next 30 calendar days. Every time-based rule in the
  app — candle alignment and MTF LCM anchor (FR-DATA-002, FR-BT-034), MIS square-off (FR-BT-031, FR-DEP-021), the MIS 15:20/15:21
  cut-offs (FR-DEP-012), scanner runs (FR-SCAN-010), alert expiry (FR-DEP-006), reconciliation (FR-DEP-010), the "Market
  closed" label (FR-SHELL-005), the Live Backtest refresh (FR-HOME-008) and order-placement validity per phase (FR-ORD-015) —
  shall obtain its boundaries from this service, never from literals. On a special session, candles align to that session's
  start and MIS square-off is `session end − N minutes` (N from the live/backtest SO tables). A date absent from the list is a
  trading day if it is a weekday; if, in addition, no NIFTY 50 daily candle exists for it after 16:00, the service marks it
  "inferred non-trading day" and flags it in the status tooltip.
- **Acceptance criteria:**
  1. Given a shipped list and a date marked holiday, then FR-SHELL-005 shows "Holiday", no scanner or deployment evaluation runs,
     and no missed-signal replay (FR-DEP-010) is attempted for that date.
  2. Given a special session 18:15–19:15 configured for date D, then 5-minute candles on D start at 18:15, the last candle is
     19:10–19:15, and the MIS square-off candle is the one containing `19:15 − N`.
  3. Given 15:45 on a trading day and an NSE equity MARKET order, then FR-ORD-015 accepts it as a post-closing order; given an
     NFO futures order at 15:45, then it is refused with "Post-closing session is for equity cash only".
  4. Given an imported CSV whose row 7 has an invalid time, then the import is refused with "Row 7: invalid time" and the current
     calendar is unchanged.
  5. Given a weekday absent from the list and no NIFTY 50 daily candle for it by 16:00, then the day is treated as non-trading and
     the status tooltip reads "Non-trading day (inferred)".
  6. Given the calendar ends on 31 Dec and today is 05 Dec, then Settings shows "Calendar may be stale" and a WARNING toast appears
     once per day.
- **Data/Kite dependency:** none (Kite has no calendar endpoint, C-14); NIFTY 50 daily candles from FR-DATA-002 for inference.
- **Deviation from Streak:** Streak maintains the calendar server-side; the local list is shipped and user-maintained (C-03, C-14).
  Special sessions are modelled explicitly (not documented by Streak).
- **Priority hint:** Core

### FR-DATA-002 Candle store
- **Source features:** CHART-001, CHART-002, BT-006, BT-025, HOME-005, ANLY-006, SCAN-006
- **Requirement:** One local store shall hold candles per (exchange, instrument token, interval) for the eight Kite intervals
  natively (minute, 3minute, 5minute, 10minute, 15minute, 30minute, 60minute, day); week, month and any other interval are
  **resampled on read** (anchored per FR-DATA-001; weeks Mon–Fri, months per calendar month) and never stored. Each candle row:
  start time (exchange time), O, H, L, C, V, OI (nullable; futures only), `source` ∈ {historical, tick-built, manual-refresh},
  `complete` flag. Today's tick-built candles are stored as incomplete and are replaced by historical candles when fetched
  after the session (or by FR-CHART-006 refresh). Writes are idempotent: the same (token, interval, start) upserts. Per (token,
  interval) the store keeps a **coverage map** of fetched ranges so that consumers ask "what is missing between A and B" without
  scanning rows, and two consumers requesting the same range produce one fetch (FR-DATA-003). A checksum per (token, interval,
  calendar month) is maintained for FR-BT-044. Settings → Data shows size per interval and total, lets the user set a size budget
  and purge rules (e.g. "keep 1-minute candles for 2 years, everything else forever"), and "Clear cached candles"; purge never
  removes ranges referenced by a stored backtest run without a confirmation naming the runs. Candles are raw/unadjusted
  (NFR-12). Storage engine (SQLite vs DuckDB/Parquet) is a design decision benchmarked against the ≈470 M-row estimate of A07 §D
  (OQ-41); the store sits behind one Go interface so the engine can change.
- **Acceptance criteria:**
  1. Given the same 200-day window requested by the chart and by a backtest within 1 s, then exactly one historical request is made
     and both consumers receive the candles.
  2. Given a tick-built 10:00 5-minute candle and a later historical fetch for the day, then the stored values equal the historical
     candle and `complete` = true.
  3. Given a purge of 1-minute data older than 2 years while backtest run R references it, then a confirmation lists R and nothing
     is deleted until confirmed.
  4. Given 500 instruments × 2 years × 1-minute loaded, then a query for one instrument-day returns in < 50 ms and the coverage
     query "missing ranges for token T between A and B" returns in < 5 ms (reference machine).
  5. Given daily candles Mon–Fri of one week, then the resampled week candle has O = Monday open, H = max, L = min, C = Friday close,
     V = Σ volume (golden test); given a Muhurat special session, then its candles belong to that calendar date's day candle.
  6. Given an append of one 1-minute candle, then the monthly checksum of that (token, interval, month) changes and no other
     checksum does.
- **Data/Kite dependency:** `GET /instruments/historical/:token/:interval` via FR-DATA-003; ticker-built candles (FR-DEP-006,
  FR-CHART-003); market calendar (FR-DATA-001).
- **Deviation from Streak:** Streak's data lives server-side; the local store is required by C-03/C-08 and makes checksums and
  budgets user-visible.
- **Priority hint:** Core

### FR-DATA-003 Kite client, shared rate limiter and backfill service
- **Source features:** ACCT-005, ACCT-006, BT-006, BT-007, CHART-001, SHELL-003
- **Requirement:** All Kite HTTP calls shall pass through one client with per-class token buckets — quote 1 req/s, historical
  3 req/s, orders per FR-ORD-006, all other endpoints 10 req/s (A07 §A3) — **priority classes** (interactive user action >
  deployment > live scanner > backtest > analytics recorder > library precompute), fair queuing within a class, and visible queue
  depth per class (FR-ACCT-009). Error taxonomy: `TokenException` → session expired state (FR-ACCT-006) and all queues pause;
  `PermissionException` → degraded mode (FR-ACCT-007); HTTP 429 → exponential back-off on that class only (base 1 s, factor 2,
  max 30 s, counter shown in FR-ACCT-009); `NetworkException` → idempotent GETs retried up to 3× with jitter (0.5–1.5 s), order
  POSTs never retried blindly (FR-ORD-006). The `Authorization` header is set by the client only and never logged (FR-SHELL-018).
  The **backfill service** accepts jobs (token, interval, from, to, requester, priority), consults the coverage map
  (FR-DATA-002) and splits the missing ranges into Kite window sizes (minute 60 d; 3/5/10-minute 100 d; 15/30-minute 200 d;
  60-minute 400 d; day 2000 d), adapting when Kite returns a window error; persists job state per window (queued / running /
  done / failed with reason) so a job resumes after restart without refetching done windows; reports progress (windows done /
  total, ETA at the current rate) to every subscriber; de-duplicates overlapping jobs; fetches futures with `oi=1`.
- **Acceptance criteria:**
  1. Given 10 backtest jobs queued and then one interactive chart load, then the chart's requests are served before the next
     backtest request and no 1-second window contains more than 3 historical requests.
  2. Given an HTTP 429 on a historical request, then the next historical request waits ≥ 1 s, quote and order traffic are
     unaffected, and FR-ACCT-009 shows the 429 count.
  3. Given the app is killed during a 40-window backfill with 25 windows done, when it restarts, then only the remaining 15 windows
     are fetched.
  4. Given a `TokenException` from any endpoint, then every queue pauses and the expired banner (FR-ACCT-006) shows within 5 s.
  5. Given two consumers request overlapping ranges of the same token/interval, then the service issues one set of non-overlapping
     window requests.
  6. Given a `NetworkException` on `PlaceOrder`, then no retry is sent before `GetOrders` has been checked for the order's tag
     (FR-ORD-006 AC3).
- **Data/Kite dependency:** every Kite endpoint; `GetHistoricalData(token, interval, from, to, continuous, oi)`.
- **Deviation from Streak:** Streak's server fetches data; the client, limiter and backfill service exist because of C-08 and the
  single API key (C-01).
- **Priority hint:** Core

### FR-DATA-004 Time synchronisation and clock integrity
- **Source features:** DEP-005, DEP-004, SCAN-010, ACCT-004
- **Requirement:** All scheduling shall use an **app clock** = system clock + measured offset to exchange time. The offset is
  estimated continuously from Kite tick `exchange_timestamp` arrival deltas (median over the last 60 s of ticks of the most
  liquid subscribed instrument) and, when no ticks flow, from the `Date` header of Kite HTTP responses (median of the last 5).
  All internal times and displays are IST regardless of the system timezone. Candle finalisation (FR-DEP-006, FR-SCAN-010,
  FR-CHART-003), the 06:00 session expiry (FR-ACCT-006), the MIS cut-offs and square-off (FR-DEP-012, FR-DEP-021), time windows
  (FR-BT-003 live equivalent) and alert expiry use the app clock; the late-tick grace window of FR-DEP-006 is measured from the
  exchange-time boundary. If |offset| > 1 s the status tooltip shows "Clock offset +3.2 s" with a WARNING once; if |offset| > 5 s,
  live evaluation, live scanners and auto-execution are suspended (DIs → SUSPENDED, reason "clock out of sync") until the offset
  returns below 2 s or the user acknowledges the warning. Offset history is logged (FR-SHELL-018).
- **Acceptance criteria:**
  1. Given the system clock set 3 s fast and ticks flowing, then within 60 s the measured offset is −3 s ± 0.5 s and 1-minute
     candles close at exchange time ± 0.5 s.
  2. Given the system timezone is UTC, then every displayed time, log timestamp and schedule is IST and the 06:00 IST expiry fires
     at 00:30 UTC.
  3. Given an offset of 7 s, then all active DIs show SUSPENDED (clock) within 5 s and no order is sent until the offset is < 2 s.
  4. Given no ticks (market closed) and HTTP responses with `Date` headers, then the offset is estimated from them and the status
     tooltip shows its source.
- **Data/Kite dependency:** ticker `exchange_timestamp` / `last_trade_time`; HTTP `Date` header of Kite responses; no extra
  network calls (NFR-04).
- **Deviation from Streak:** Streak runs on synchronised servers; a local app must detect skew itself (NFR-08).
- **Priority hint:** Core

### FR-DATA-005 Backup, restore and data directory
- **Source features:** MYS-001, MYS-005, DISC-017, ACCT-005, BT-008
- **Requirement:** The data directory `~/.local/share/tradeloom` (overridable by `TRADELOOM_DATA_DIR`) shall have the layout:
  `app.db` (SQLite: strategies, scanners, watchlists, universes, settings, backtest runs, deployment and order journals, analytics
  recordings, calendar edits), `candles/` (FR-DATA-002 store), `logs/` (FR-SHELL-018), `exports/` (default export folder),
  `backups/` (automatic pre-migration backups), `library/` (bundled library copies). **Schema migrations:** forward migrations run
  on start inside one transaction after a backup copy of `app.db` is written to `backups/`; the app refuses to open a database
  whose schema version is newer than it supports, naming both versions and the newer app version that wrote it. **Backup now**
  (Settings → Data) writes one archive `tradeloom-backup-YYYYMMDD-HHMMSS.tar.zst` containing `app.db`, calendar and library
  versions, and optionally the candle store (checkbox, size shown); secrets are never included (keyring / encrypted secrets file
  excluded). **Restore** validates the archive (schema version ≤ supported), shows what will be replaced (counts of strategies,
  scanners, runs, deployments), requires typed confirmation, restores into a fresh data directory and keeps the previous one as
  `*.replaced-<timestamp>`. **Scheduled backup** (off by default): daily after 16:00 or on quit, to a user-chosen folder, keeping
  the last N (default 7). Running deployments must be flat for a restore (else refused).
- **Acceptance criteria:**
  1. Given a backup from machine A and a clean install on machine B, when restored, then every strategy, scanner, backtest run,
     deployment and order-journal entry is present, the keyring is empty and the app asks for Kite credentials.
  2. Given `app.db` at schema 3 and a binary supporting schema 2, then the app refuses to start with "Database schema 3 (written by
     v1.4.0) is newer than this version supports (2)".
  3. Given an archive, when it is searched for the API secret, access token and Telegram token strings, then no match exists.
  4. Given a migration from schema 2 to 3 fails midway, then `app.db` is unchanged (transaction rolled back) and the pre-migration
     backup exists in `backups/`.
  5. Given scheduled backup to a folder with N = 7 and 8 existing backups, then the oldest is deleted after the new one is written.
- **Data/Kite dependency:** none.
- **Deviation from Streak:** Streak keeps everything server-side; local storage (G3) requires backup/restore and migration rules
  (NFR-02/03/09).
- **Priority hint:** Important

---

## FR module EQB — Equity strategy builder

### FR-EQB-001 Equity builder workspace layout
- **Source features:** EQB-001
- **Requirement:** The system shall provide an Equity Strategy Builder view, reachable from the sidebar ("Equity Builder") and from
  My Strategies (edit), consisting of: (a) a header with the strategy title (FR-EQB-003) and a "My Strategies" navigation button;
  (b) a left, independently scrolling form column (~48% width at ≥1280 px) containing, in order, an **Entry** card (direction toggle,
  instruments, chart settings, conditions editor with mode switch, suggestions button, quick-setup tiles) and an **Exit** card
  (risk parameters, optional exit conditions); (c) a sticky footer with Save (and Execute for a saved strategy, FR-EQB-032);
  (d) a right column (~52%) with the builder chart pane (FR-EQB-033). A new draft shall be named "Untitled strategy", direction Buy,
  base timeframe 1 minute, one empty Basic entry row, no exit values, and shall be pre-seeded with the user-configurable *default
  instrument set* (Settings; shipped default = four liquid large-cap NSE equities taken from the bundled NIFTY 50 snapshot; may be set
  to empty). Navigating away with unsaved edits triggers the unsaved-change guard (SHELL-007).
- **Acceptance criteria:**
  1. Given a fresh install, when the user opens a new equity strategy, then the title reads "Untitled strategy", direction = Buy, base timeframe = 1 minute, entry has exactly one empty Basic row, and the instrument list equals the configured default set (4 instruments).
  2. Given the default instrument set is configured empty, when a new draft opens, then the instrument list is empty and the chart pane shows an empty-state prompt to add instruments.
  3. Given the window is 1280×800, when exit conditions are expanded, then only the left column scrolls and the footer stays visible.
  4. Given unsaved edits exist, when the user navigates to another view, then the unsaved-change guard dialog appears (SHELL-007).
- **Data/Kite dependency:** Local strategy store (SQLite); historical candles for the chart pane (`GET /instruments/historical`, via local candle store, C-08).
- **Deviation from Streak:** Default instrument set is user-configurable rather than fixed.
- **Priority hint:** Core

### FR-EQB-002 Strategy definition model (versioned JSON + canonical text DSL)
- **Source features:** EQB-006, EQB-016, EQB-022, EQB-023, EQB-024, EQB-027, EQB-028, EQB-034, EQB-038
- **Requirement:** The system shall persist every strategy and (through the ScannerDoc extension of FR-SCAN-016) every scanner in a single,
  documented, versioned JSON model, published as a JSON Schema at `docs/spec/strategy-schema-v1.json` and described in
  `docs/spec/strategy-dsl.md`. The model shall contain at minimum:
  ```
  StrategyDoc  { schema: "tl.strategy", version: "<semver>", id: uuid, kind: "equity",   // other kinds reserved (§10)
                 meta: { name, category, tags[], description, favourite, createdAt, updatedAt,
                         origin: { type: "user"|"library"|"import", libraryId?, libraryVersion? } },
                 equity: EquityBody }
  EquityBody   { direction: "buy"|"sell", instruments: [InstrumentRef],
                 chart: { type: "candlestick", timeframe: TF },
                 entry: ConditionBlock, exit: { risk: Risk, conditions?: ConditionBlock } }
  InstrumentRef{ exchange, tradingsymbol, instrumentToken? }      // token is a cache; re-resolved from the instruments dump;
                                                                 // segments allowed: INDICES, NSE, BSE, NFO-FUT, BFO-FUT (NFR-15)
  TF           = "1m"|"3m"|"5m"|"10m"|"15m"|"30m"|"1h"|"1d"|"1w"|"1M"
  ConditionBlock { mode: "basic"|"advanced", root: Group }
  Group        { node: "group", id, enabled: bool, children: [ { join?: "and"|"or", item: Condition|Group } ] }  // first child has no join
  Condition    { node: "cond", id, enabled: bool, timeframe: TF, lhs: Expr, cmp: Comparator, rhs: Expr }
  Comparator   { op: "eq"|"cross_above"|"cross_below"|"lt"|"lte"|"gt"|"gte"|"up_by"|"down_by", value?: number, unit?: "pct"|"pts" }
  Expr         = { num: number }
               | { ind: <operand id>, params: { <name>: value }, offset: int (≤ 0) }
               | { op: "+"|"-"|"*"|"/", left: Expr, right: Expr }
               | { fn: "period_min"|"period_max"|"min"|"max"|"floor"|"ceil"|"abs", period?: int, args: [Expr] }
               | { paren: Expr }
  Risk         { unit: "pct"|"pts"|"pnl", sl?: number, tp?: number, tsl?: number }
  ```
  Rules: (1) operand ids and parameter names/types/enumerations come from the operand catalogue (FR-EQB-039); (2) `offset` 0 = the
  current (last closed) candle, −N = N candles earlier; positive offsets are invalid; (3) a canonical **text form** shall exist with an
  EBNF grammar in the DSL doc (e.g. `[15m] (High[-2] - Low[-2]) lower than (High[-6] - Low[-6]) and Volume[0] crosses above Volume[-3] * 3.6`),
  produced by one canonical formatter and accepted by one parser, used for clipboard, suggestions, library authoring, display and
  export; (4) `parse(format(doc)) == doc` for every valid doc; (5) the loader shall migrate any older minor/major version it knows to
  the current version and shall refuse a document whose major version is newer than supported, with a message naming both versions;
  (6) unknown fields are preserved on round-trip within the same major version.
- **Acceptance criteria:**
  1. Given the JSON Schema, when every library strategy and scanner shipped with the app is validated, then all validate with zero errors (CI test).
  2. Given 1,000 randomly generated valid docs (property test), when each is formatted to text and parsed back, then the result is deep-equal to the original.
  3. Given a doc with `version` "1.x" saved by an earlier build, when loaded by a build with schema 2.0 and a registered 1→2 migration, then it loads, and re-saving writes version 2.0.
  4. Given a doc with version "9.0.0", when imported, then import fails with a message stating the file's version and the highest supported version, and nothing is written to the store.
  5. Given a condition with `offset: 1`, when validated, then validation fails with error code `OFFSET_POSITIVE` pointing at that operand.
- **Data/Kite dependency:** Local only (SQLite; NFR-03 versioned migrations).
- **Deviation from Streak:** Streak's internal format is not observable; ours is documented, versioned and round-trippable (clean-room design). The options body is deferred (§10 EQB-002-opt).
- **Priority hint:** Core

### FR-EQB-003 Strategy title with inline rename
- **Source features:** EQB-002
- **Requirement:** The header title shall be editable inline (click or F2 → text field; Enter or blur commits; Escape reverts). Names:
  1–80 characters after trimming, unique among the user's strategies (case-insensitive). The description (if any) shows as a
  secondary line under the title. Renaming marks the strategy dirty; it is persisted on Save.
- **Acceptance criteria:**
  1. Given a draft, when the user clicks the title, types "Gap fill test" and presses Enter, then the header shows "Gap fill test" and the dirty indicator is shown; no dialog opens.
  2. Given an existing strategy named "Alpha", when another strategy is renamed to "alpha", then an inline error "A strategy with this name already exists" is shown and the name is not committed.
  3. Given the user is editing the title, when Escape is pressed, then the previous title is restored.
- **Data/Kite dependency:** Local only.
- **Deviation from Streak:** Streak opens the Save dialog on title click (§3.2 #10); we provide inline rename. The Save dialog stays reachable from Save.
- **Priority hint:** Important

### FR-EQB-004 Save Strategy dialog (name, category, tags, description)
- **Source features:** EQB-029, EQB-002
- **Requirement:** On first Save of a new strategy (or via the overflow "Edit details"), the system shall show a modal with: Name
  (required, rules as FR-EQB-003, focused); Category as a two-option segmented control **Equity Intraday** (default) / **Equity Swing**;
  Tags (chip input, Enter or comma adds, × removes, max 5 tags, each 1–30 chars, duplicates ignored case-insensitively, counter "n/5");
  Description (multi-line, max 2,000 chars). Buttons Cancel and Save; Escape and × close without saving; Enter in Name submits.
  Category determines the My Strategies tab and the default Holding Type in Execute → Backtest/Deploy (Intraday → MIS, Swing → CNC; BT-004).
  No quantity, capital, date or time fields appear here (they belong to BT-001/DEP-001). Saving an already-saved strategy from the
  footer saves silently (no dialog) and shows a success toast.
- **Acceptance criteria:**
  1. Given a new draft, when Save is pressed, then the dialog opens with Category = Equity Intraday and empty Name focused.
  2. Given 5 tags entered, when the user types a 6th and presses Enter, then it is not added and the counter shows "5/5".
  3. Given Name is empty, then the dialog's Save button is disabled and an inline hint explains why.
  4. Given the dialog is open, when Escape is pressed, then it closes and nothing is persisted.
  5. Given a saved strategy with Category Equity Swing, when Execute → Backtest opens, then Holding Type defaults to CNC.
- **Data/Kite dependency:** Local only.
- **Deviation from Streak:** Escape closes; validation is visible inline (§3.2 #8, #16).
- **Priority hint:** Core

### FR-EQB-005 Position direction (Buy / Sell)
- **Source features:** EQB-003
- **Requirement:** The Entry card shall have a two-state toggle Buy (long) / Sell (short), default Buy, colour-coded (buy = blue,
  sell = orange-red) and keyboard-operable. Changing it shall not alter conditions or exits. When direction = Sell and Category =
  Equity Swing, the system shall show a persistent warning that delivery (CNC) short positions cannot be carried overnight in the cash
  segment, and Execute shall block Holding Type CNC for such a strategy (MIS allowed).
- **Acceptance criteria:**
  1. Given a strategy with two conditions, when the toggle changes Buy→Sell, then the conditions JSON is byte-identical before and after.
  2. Given Sell + Equity Swing, when Execute → Backtest is opened, then CNC is disabled with an explanatory tooltip and MIS is selected.
  3. Given focus on the toggle, when Space is pressed, then the direction flips.
- **Data/Kite dependency:** Order side BUY/SELL and product MIS for intraday shorts at deploy time (ORD/DEP modules).
- **Deviation from Streak:** Streak's handling of Sell + CNC is unknown; we block it explicitly.
- **Priority hint:** Core

### FR-EQB-006 Instrument selection
- **Source features:** EQB-004
- **Requirement:** The Entry card shall show the selected instruments as an avatar/chip stack (first N visible, then "+k") and an
  "Add instruments" button that opens the shared multi-select instrument picker (WATCH-005). Each chip shall offer remove (×, also via
  keyboard Delete). Allowed segments for equity strategies: NSE/BSE cash equities, indices (signal-only — not orderable), and NFO/BFO
  futures (BT-026). The system shall not impose a fixed instrument cap (NFR-13); the picker shows the running count and shows a
  non-blocking warning when the count would exceed what a single deployment can stream within the WebSocket budget (C-07).
  Adding or removing an instrument updates the chart pane tabs (FR-EQB-033).
- **Acceptance criteria:**
  1. Given 60 instruments are selected in the picker, when confirmed, then all 60 are saved (no cap) and the stack shows the overflow count.
  2. Given an index (NIFTY 50) is added, then its chip carries a "signal only" marker and deploy-time order placement for it is disabled.
  3. Given 3 instruments, when one chip's × is clicked, then 2 remain and its chart tab disappears.
- **Data/Kite dependency:** Instruments dump `GET /instruments` (daily refresh, local cache).
- **Deviation from Streak:** Streak caps at 50 instruments/strategy; we remove the cap and warn on WebSocket capacity (NFR-13, C-07).
- **Priority hint:** Core

### FR-EQB-007 Chart settings (chart type and base timeframe)
- **Source features:** EQB-005
- **Requirement:** The Entry card shall show the chart type as a static label "Candlestick" (not a button) and a base-timeframe dropdown
  with exactly: 1 minute, 3 minutes, 5 minutes, 10 minutes, 15 minutes, 30 minutes, 1 hour, 1 day (default 1 minute; Appendix I).
  Weekly/monthly are not base timeframes (only per-condition, FR-EQB-025). Changing the base timeframe re-renders the chart pane, sets
  the default timeframe for new Advanced rows, updates rows that were "following base" (rows whose timeframe equals the old base and
  were never explicitly changed), and determines the default backtest lookback (BT-006). Candles align to the session start of the market calendar (FR-DATA-001; 09:15 on a regular day).
- **Acceptance criteria:**
  1. Given the dropdown is opened, then it lists exactly the 8 values above in that order, and no 1w/1M entry.
  2. Given two Advanced rows, one following base and one explicitly set to 1d, when the base timeframe changes from 5m to 15m, then the first row becomes 15m and the second stays 1d.
  3. Given base = 1 hour, then the chart pane's candles start at 09:15, 10:15, … 15:15 (last candle partial to 15:30).
- **Data/Kite dependency:** Kite historical intervals minute/3minute/5minute/10minute/15minute/30minute/60minute/day map 1:1 (Appendix I).
- **Deviation from Streak:** Inert "Candlestick" button replaced by a label (§3.2 #9). Heikin-Ashi/Renko not offered (same as Streak).
- **Priority hint:** Core

### FR-EQB-008 Basic condition row
- **Source features:** EQB-006
- **Requirement:** In Basic mode, entry (and exit) conditions shall be a flat list of rows `If|And <LHS operand> <comparator> <RHS operand or number>`
  with a delete action per row and an "Add condition" action. Row 1 is labelled "If", later rows "And" (Basic is AND-only).
  The comparator and RHS controls are disabled until an LHS exists. Picking an indicator into an **empty** row shall auto-fill the
  operand's default condition template (comparator + RHS) from the operand catalogue (FR-EQB-039; Appendix A column "Default
  condition"); picking into a row that already has a comparator/RHS replaces only the LHS. A row with no LHS is **empty**; a row with an LHS but a missing comparator or RHS is **incomplete** (§3.1). Incomplete rows
  are shown with an "incomplete" marker and block Execute (FR-EQB-037); empty rows are dropped on Save/Execute with an INFO
  toast naming how many were ignored.
- **Acceptance criteria:**
  1. Given an empty row, when "Simple Moving Average" is picked as LHS, then the row reads `SMA(close, 9)[0] crosses above SMA(close, 21)[0]` (canonical form of the Appendix A template).
  2. Given a row `Supertrend(7, 3)[0] crosses below Close[0]`, when the LHS is changed to RSI, then the row reads `RSI(14)[0] crosses below Close[0]`.
  3. Given a row with no LHS, then the comparator and RHS controls are disabled and announce "Choose an indicator first" to assistive tech.
  4. Given three rows where row 2 is incomplete, when Execute is pressed, then Execute is blocked and row 2 is highlighted with "Incomplete condition".
  5. Given two complete rows and one empty row, when Execute is pressed, then the empty row is removed, an INFO toast says "1 empty condition ignored" and execution proceeds.
- **Data/Kite dependency:** Local rule engine.
- **Deviation from Streak:** Incomplete rows are flagged and block execution rather than being silently ignored (EQB-006 note on Run Scan).
- **Priority hint:** Core

### FR-EQB-009 Operand picker (Basic) with search
- **Source features:** EQB-007
- **Requirement:** Each Basic operand cell shall open a searchable list of all Basic operands (Appendix A rows 1–71 including Central
  Pivot Range, plus ATR Trailing Stop). Search matches full name, short token and aliases (FR-EQB-039), case-insensitive substring,
  and is fully keyboard-navigable (↑/↓/Enter/Escape). The list ends with a "Browse suggestions…" entry that opens FR-EQB-016 filtered to
  the typed text. The **RHS** list starts with a "Number" entry; typing a numeric literal (integer, decimal, negative) in the RHS search
  and pressing Enter creates a number operand. **Central Pivot Range shall be selectable and valid in Basic mode.**
- **Acceptance criteria:**
  1. Given the LHS list, when "super" is typed, then "Supertrend" is the first result.
  2. Given the LHS list, when "Central Pivot Range" is chosen, then the row is filled with a valid default condition and no error is shown; the strategy evaluates it in backtest.
  3. Given the RHS search, when "-80" is typed and Enter pressed, then the RHS becomes the number −80.
  4. Given the list is open, when Escape is pressed, then it closes and the cell value is unchanged.
- **Data/Kite dependency:** Local operand catalogue.
- **Deviation from Streak:** CPR works in Basic (§3.2 #1 not replicated).
- **Priority hint:** Core

### FR-EQB-010 Operand parameter dialog with validation
- **Source features:** EQB-008, EQB-035
- **Requirement:** Clicking a placed operand (Basic cell or Advanced token name) shall open a modal titled with the operand's display
  name, with one control per catalogue parameter: numeric fields with type (integer/decimal), min, max and step from the catalogue;
  dropdowns for enumerations (e.g. source field, MA type, line/band selector); and an **Offset** field (integer, −500…0, default 0) for
  every operand except those flagged offset-less (Nth Candle, Opening Range, Pivot Points, Day High/Low, Central Pivot Range, Candle
  Time). Labels shall be human-readable (e.g. "Fast period", "Standard deviations", "Acceleration start"). Footer: Cancel, Save; a
  "Remove" action clears the operand (row returns to empty LHS and the comparator disables; RHS kept). Validation is inline and blocking:
  out-of-range, empty or non-integer values show a field message and disable Save; cross-field rules from the catalogue (e.g. MACD fast
  < slow; SAR start ≤ max; Ultimate Oscillator lengths ascending) are enforced. Escape = Cancel; Enter = Save when valid; backdrop click =
  Cancel. Changes apply to the draft only.
- **Acceptance criteria:**
  1. Given the SMA dialog, when Period is cleared, then Save is disabled and "Period is required" is shown.
  2. Given the SMA dialog, when Period = −5 is entered, then "Must be between 1 and 5000" (catalogue bounds) is shown and Save is disabled.
  3. Given the MACD dialog, when fast = 30 and slow = 26, then a cross-field error is shown and Save is disabled.
  4. Given the Opening Range dialog, then no Offset field is present.
  5. Given the dialog is open with changes, when Escape is pressed, then the dialog closes and the operand is unchanged.
- **Data/Kite dependency:** Local operand catalogue.
- **Deviation from Streak:** Streak has no validation, raw identifier labels and ignores Escape (§3.2 #6, #7, #8); we validate and use readable labels.
- **Priority hint:** Core

### FR-EQB-011 Number operand
- **Source features:** EQB-009
- **Requirement:** A number operand shall hold a finite decimal (up to 8 fractional digits, range ±1e12). It is created from the RHS
  "Number" entry, by typing a number in any operand search (Basic RHS, Advanced picker shows "Use number: <n>"), and edited via a small
  "Number" dialog (single field, Cancel/Save/Remove, Escape cancels). Numbers render in the number colour in Advanced mode.
- **Acceptance criteria:**
  1. Given the RHS "Number" entry, when 2.5 is entered and saved, then the RHS shows `2.5` and the JSON contains `{ "num": 2.5 }`.
  2. Given the Number dialog, when "abc" is typed, then Save is disabled with "Enter a number".
  3. Given the Advanced picker, when "60" is typed, then an entry "Use number: 60" appears first and selecting it inserts the constant.
- **Data/Kite dependency:** Local only.
- **Deviation from Streak:** none.
- **Priority hint:** Core

### FR-EQB-012 Comparators and their semantics
- **Source features:** EQB-010, EQB-021
- **Requirement:** The system shall offer 9 comparators (Appendix C) in both modes with identical semantics, evaluated at
  evaluation time t on the row's timeframe, where L and R are the evaluated LHS/RHS values. Semantics follow Streak's documented
  rules (A06 §1: "Crosses above means false on the previous candle and true now. Higher than is the current state only, and counts
  equal as true"), applied symmetrically to the "lower" family:
  | Comparator | Rule |
  |---|---|
  | equal to | \|L − R\| ≤ ε, ε = 1e-9 × max(1, \|L\|, \|R\|) (tolerance under OQ-18) |
  | higher than | L ≥ R (equality counts as true — documented Streak behaviour) |
  | higher than or equal to | L ≥ R (identical to "higher than"; kept because the UI offers both) |
  | lower than | L ≤ R (equality counts as true, by symmetry) |
  | lower than or equal to | L ≤ R (identical to "lower than") |
  | crosses above | not (L[t−1] ≥ R[t−1]) and (L[t] ≥ R[t]), i.e. L[t−1] < R[t−1] and L[t] ≥ R[t] |
  | crosses below | L[t−1] > R[t−1] and L[t] ≤ R[t] |
  | up by v (unit) from | pct: L ≥ R × (1 + v/100); pts: L ≥ R + v |
  | down by v (unit) from | pct: L ≤ R × (1 − v/100); pts: L ≤ R − v |
  If any required value (including the t−1 values for crosses) is undefined (insufficient history, division by zero, missing OI),
  the condition is false. One ordering is used everywhere: equal to, crosses above, crosses below, higher than, higher than or
  equal to, lower than, lower than or equal to, up by, down by. The comparator control is disabled until an LHS exists. The help
  article for comparators states that "higher than" and "higher than or equal to" are equivalent (and likewise "lower").
- **Acceptance criteria:**
  1. Given golden fixture series L=[1,2,3], R=[2,2,2], then crosses above is **true** at t=1 (L[0]=1 < 2 and L[1]=2 ≥ 2) and false
     at t=2 (L[1] ≥ R[1] already); higher than is true at t=1.
  2. Given L=[3,2], R=[2,2], then crosses below is **true** at t=1 (3 > 2 then 2 ≤ 2).
  3. Given `RSI(14)[0] crosses above 30` with RSI = [29.5, 30.0], then true at t=1; with RSI = [30.0, 31.0], then false at t=1.
  4. Given L=101, R=100, up by 1 pct, then true; given L=100.99, then false.
  5. Given L undefined at t−1, then crosses above is false at t regardless of L[t], R[t].
  6. Given the Basic and Advanced comparator lists, then both show the same 9 items in the same order.
  7. Given `Open[0] equal to High[0]` with Open = High = 1,250.00, then true (ε-equality on prices).
- **Data/Kite dependency:** Local rule engine.
- **Deviation from Streak:** none in semantics (documented parity, user decision 2026-10-12 on RV-021); ordering is unified across
  modes (Streak used a different Advance order). The "equal to" tolerance is our choice (OQ-18).
- **Priority hint:** Core

### FR-EQB-013 "Up by" / "Down by" parameters
- **Source features:** EQB-011
- **Requirement:** Choosing "up by"/"down by" shall open a dialog with Value (decimal > 0, required) and Unit (Percent default, Points).
  The comparator cell renders inline, e.g. `up by 1% from` / `down by 12 pts from`. Clicking the cell reopens the dialog. Save is
  enabled only for valid values; Escape cancels; a Remove action resets the comparator to the operand's default comparator.
- **Acceptance criteria:**
  1. Given a row with comparator "up by", when value 1 and unit Percent are saved, then the row renders `Close[0] up by 1% from Close[-1]` and JSON has `{op:"up_by", value:1, unit:"pct"}`.
  2. Given value 0, then Save is disabled with "Must be greater than 0".
  3. Given unit Points and value 5, then evaluation uses L ≥ R + 5 (unit test).
- **Data/Kite dependency:** Local only.
- **Deviation from Streak:** none — Streak offers `%` and `abs` on editable scanners (REVIEW-2 N4); our Points = abs, and
  Remove mirrors Streak's trash icon.
- **Priority hint:** Important

### FR-EQB-014 Quick-setup tiles
- **Source features:** EQB-012
- **Requirement:** In Basic mode, under the condition rows, the system shall show a horizontally scrollable strip of tiles, one per
  Basic operand (all Appendix A operands, our own icons and short labels), with previous/next scroll buttons and keyboard focus.
  Activating a tile fills the first empty row with that operand's default condition, or appends a new row if none is empty. Tiles are
  hidden in Advanced mode.
- **Acceptance criteria:**
  1. Given one empty row, when the RSI tile is activated, then that row reads `RSI(14)[0] crosses above 30`.
  2. Given all rows complete, when the EMA tile is activated, then a new row is appended with the EMA default condition.
  3. Given Advanced mode, then the strip is not rendered.
- **Data/Kite dependency:** Local only.
- **Deviation from Streak:** none (own artwork and labels).
- **Priority hint:** Nice-to-have

### FR-EQB-015 Suggestions button
- **Source features:** EQB-013
- **Requirement:** The Conditions header (Basic and Advanced) shall have a "Suggestions" button that opens the suggestion browser
  (FR-EQB-016), pre-filtered to the operand of the focused row if any. The button label and tooltip shall not imply AI or generative
  assistance.
- **Acceptance criteria:**
  1. Given focus in a row whose LHS is MACD, when Suggestions is pressed, then the browser opens with MACD selected.
  2. Given no focused row, then the browser opens with "All" selected.
- **Data/Kite dependency:** Local only.
- **Deviation from Streak:** Streak's "Assist" implies AI but is static (§3.2 #4); ours is labelled honestly and also available in Advanced mode (inserts the suggestion as an Advanced row).
- **Priority hint:** Important

### FR-EQB-016 Suggestion browser dialog
- **Source features:** EQB-014
- **Requirement:** A modal "Condition suggestions" shall offer: Bullish/Bearish segmented filter (Bullish default); search box
  (filters operands and suggestion text, case-insensitive); A–Z initial-letter strip; category chips from one clean taxonomy (All, Trend
  & moving averages, Momentum & oscillators, Volume, Volatility & bands, Price action & levels); a left list of operands with counts; a
  right list of suggestions (checkbox, canonical condition text, tag chips). **Apply semantics (explicit):** ticking stages a suggestion;
  conditions already present in the block appear pre-ticked; on **Add selected**, staged additions are appended as rows (Basic: AND
  rows; Advanced: rows joined with "and" at the top-level group) and un-ticked pre-existing ones are removed; Cancel, × and Escape discard
  all staging. Suggestion text is produced only by the canonical formatter (FR-EQB-002), so casing/spacing is consistent.
- **Acceptance criteria:**
  1. Given Bullish, when "cross" is searched, then every listed suggestion's text contains "cross" (case-insensitive) and operand counts equal the number of matching suggestions.
  2. Given 2 suggestions ticked, when Cancel is pressed, then the block is unchanged.
  3. Given 2 ticked, when "Add selected" is pressed, then exactly 2 rows are appended in the order ticked.
  4. Given a block already containing `RSI(14)[0] crosses above 30`, when the browser opens on RSI, then that suggestion is pre-ticked; un-ticking it and pressing Add selected removes that row.
  5. Given the dialog open, when Escape is pressed, then it closes without changes.
- **Data/Kite dependency:** Local suggestion library (FR-EQB-017).
- **Deviation from Streak:** One explicit apply behaviour (§3.2 #30), Cancel/×/Escape (§3.2 #8, #20), clean categories and formatted text (§3.2 #5).
- **Priority hint:** Important

### FR-EQB-017 Suggestion library content
- **Source features:** EQB-014, EQB-036
- **Requirement:** The app shall ship a versioned suggestion library (data file validated against the schema) authored by us from
  generic, well-known single-condition setups. Coverage: for every Basic operand in Appendix A (including Central Pivot Range and ATR
  Trailing Stop), at least one Bullish suggestion, and at least one Bearish suggestion wherever a bearish reading is meaningful; total
  ≥ 300 Bullish and ≥ 300 Bearish suggestions (Appendix D scale). Each suggestion: id, operand id (correct operand — never another
  operand's items), bias (bullish/bearish), category (taxonomy of FR-EQB-016), 1–3 tags from a controlled vocabulary (e.g. Scalping,
  Trend following, Long-term, Crossover, Oversold reversal, Overbought reversal, Breakout, Breakdown, Momentum, Mean reversion), and a
  condition expressed in the DSL. Bearish suggestions mirror bullish ones where symmetric (crosses above ↔ crosses below, higher ↔ lower).
- **Acceptance criteria:**
  1. Given the library, when a CI check runs, then every Basic operand id has ≥1 bullish suggestion, every suggestion's condition uses its declared operand on the LHS or RHS, and every condition parses.
  2. Given the library, then counts are ≥300 bullish and ≥300 bearish and no two suggestions have identical canonical text.
  3. Given the tag vocabulary, then every tag used is in the vocabulary (no free-text duplicates).
- **Data/Kite dependency:** Local only.
- **Deviation from Streak:** Own content; data bugs of the reference (Open showing Opening Range items, etc.) are not replicated (§3.2 #5).
- **Priority hint:** Important

### FR-EQB-018 Basic ↔ Advanced mode switch with lossless conversion
- **Source features:** EQB-015
- **Requirement:** The Conditions header shall have a Basic/Advanced switch with a help link. **Basic → Advanced** shall convert every
  row losslessly (same conditions, joined by "and", timeframe preserved) without a confirmation. **Advanced → Basic** shall convert
  losslessly when the block is representable in Basic (single top-level group, only "and" joins, no nested groups, no arithmetic or
  functions, no disabled rows); otherwise the system shall show a dialog listing the non-representable elements with choices "Stay in
  Advanced" (default) and "Switch and clear conditions" (destructive, requires explicit confirmation). Switching modes is undoable
  (Ctrl+Z) within the session. The switch is independent per block (entry, exit).
- **Acceptance criteria:**
  1. Given 3 Basic rows, when switching to Advanced, then 3 Advanced rows with identical lhs/cmp/rhs/timeframe exist joined by "and", and no dialog appeared.
  2. Given Advanced rows all AND-joined with only indicators/numbers, when switching to Basic, then the rows are preserved without a dialog.
  3. Given an Advanced block containing an "or" and `High[0] - Low[0]`, when switching to Basic, then a dialog lists "OR join" and "arithmetic expression"; choosing Stay keeps the block unchanged.
  4. Given a switch just happened, when Ctrl+Z is pressed, then the previous mode and conditions are restored.
- **Data/Kite dependency:** Local only.
- **Deviation from Streak:** Streak wipes conditions on switch (§3.2 #3); we convert.
- **Priority hint:** Core

### FR-EQB-019 Advanced expression row
- **Source features:** EQB-016
- **Requirement:** In Advanced mode each row shall present `[timeframe] [expression] [comparator] [expression]` plus row tools
  (FR-EQB-028). Empty slots show placeholders ("Add expression", "Add comparator"). Tokens are styled by kind (operand name + muted
  parameters, comparator, operator, number) using theme tokens with sufficient contrast in light and dark themes. Each token has
  visible (not hover-only) affordances to remove it and to append an operator/term (FR-EQB-021), and is reachable by keyboard
  (FR-EQB-042). Expressions can contain operands, numbers, arithmetic and functions to arbitrary nesting depth up to 16 levels.
- **Acceptance criteria:**
  1. Given an empty Advanced block, when "Add condition" is pressed, then a row with timeframe = base timeframe and three placeholders appears.
  2. Given a row `High[0] - Low[0] higher than 50`, when the remove action on `Low[0]` is used, then the expression becomes `High[0] -` with a placeholder and the row is marked incomplete.
  3. Given nesting depth 17, when the user tries to insert another function, then the action is disabled with "Maximum nesting depth reached".
- **Data/Kite dependency:** Local expression engine.
- **Deviation from Streak:** Affordances are visible/keyboard-accessible rather than hover-only (NFR-11, §3.1).
- **Priority hint:** Core

### FR-EQB-020 Expression picker (All / Indicators / Math functions)
- **Source features:** EQB-017
- **Requirement:** An expression slot shall open a picker with tabs All / Indicators / Math functions and a search box that also
  accepts a number ("Use number: n"). Indicators = all operands in the catalogue (Appendix A + Appendix B operands), each listed once
  under its display name with aliases searchable (e.g. "T Range", "MOM", "LBB", "UBB", "Aroon-Up"); math functions = Period min, Period
  max, Min, Max, Floor, Ceil, Abs, Brackets. Choosing an operand places a token with catalogue defaults and opens nothing; activating
  the token name opens the parameter dialog (FR-EQB-010). Escape closes the picker.
- **Acceptance criteria:**
  1. Given the picker, when "aroon-up" is searched, then exactly one entry "Aroon Up" is listed (aliases do not create duplicates).
  2. Given the Math functions tab, then exactly the 8 functions listed above appear.
  3. Given the picker, when "2.5" is typed and Enter pressed, then the number 2.5 is inserted.
  4. Given the Indicators tab, then the count equals the operand catalogue size (single source of truth; CI test; reference: Streak's All tab shows 107
     entries = 99 operands including Number + 8 math functions, REVIEW-2 #25).
- **Data/Kite dependency:** Local catalogue.
- **Deviation from Streak:** Duplicate alias entries are collapsed into one entry with searchable aliases (Conflicts §6 #6).
- **Priority hint:** Core

### FR-EQB-021 Arithmetic operators and precedence
- **Source features:** EQB-018
- **Requirement:** After any term, the user can append an operator (+, −, ×, ÷) via an operator chooser (keyboard: typing + - * /),
  which immediately opens the expression picker for the next term. Evaluation follows standard precedence (× ÷ before + −,
  left-associative); Brackets (FR-EQB-022) override. The canonical text shows the effective structure; the formatter inserts brackets
  only where needed to preserve the parsed tree. Division by zero yields undefined (condition false).
- **Acceptance criteria:**
  1. Given `2 + 3 * 4` built left to right, then it evaluates to 14.
  2. Given `(2 + 3) * 4` using Brackets, then it evaluates to 20 and renders with brackets.
  3. Given `Volume[0] / 0`, then the condition evaluates false and the backtest log records one "undefined value" diagnostic per instrument per run (not per candle).
- **Data/Kite dependency:** Local only.
- **Deviation from Streak:** Precedence is specified explicitly (not observable in Streak).
- **Priority hint:** Core

### FR-EQB-022 Math functions
- **Source features:** EQB-019
- **Requirement:** The system shall support: **Period max(n, expr)** / **Period min(n, expr)** = max/min of expr over the last n candles
  of the row timeframe, including the current (n integer 1…5000, default 14); **Max(a, b)** / **Min(a, b)** = larger/smaller of two
  expressions; **Floor(x)**, **Ceil(x)**, **Abs(x)** (one slot each); **Brackets(x)** grouping. Undefined inside the window → undefined.
  Function tokens render with their slots, e.g. `Period max(14, True Range[0])`.
- **Acceptance criteria:**
  1. Given Close = [5,7,6,9,8], when `Period max(3, Close[0])` is evaluated at the last candle, then the value is 9; `Period min(3, Close[0])` = 6.
  2. Given `Abs(Close[0] - Open[0])` with Close 100, Open 103, then 3.
  3. Given `Floor(-2.5)`, then −3; `Ceil(-2.5)` = −2.
  4. Given n = 3 and only 2 candles of history, then Period max is undefined.
- **Data/Kite dependency:** Local only.
- **Deviation from Streak:** none (semantics specified where Streak's were unobserved).
- **Priority hint:** Core

### FR-EQB-023 Inline numeric argument editor
- **Source features:** EQB-020
- **Requirement:** Numeric arguments inside function tokens (e.g. the period of Period max) shall be editable in place via a small
  popover with a number input (spinner, min/max from the function's spec) and OK; Enter = OK, Escape = cancel; invalid values disable OK.
- **Acceptance criteria:**
  1. Given `Period min(14, …)`, when the 14 is activated, 20 entered and Enter pressed, then the token shows 20.
  2. Given 0 is entered, then OK is disabled with "Must be at least 1".
- **Data/Kite dependency:** Local only.
- **Deviation from Streak:** Validation added.
- **Priority hint:** Important

### FR-EQB-024 Advanced comparator picker
- **Source features:** EQB-021
- **Requirement:** The "Add comparator" slot shall open a searchable popover listing the 9 comparators of FR-EQB-012 in the same order
  as Basic; up by/down by then opens FR-EQB-013.
- **Acceptance criteria:**
  1. Given the popover, when "cross" is typed, then only "crosses above" and "crosses below" remain.
  2. Given "down by" is chosen, then the Down by dialog opens.
- **Data/Kite dependency:** Local only.
- **Deviation from Streak:** Same order as Basic (Streak used a different order).
- **Priority hint:** Core

### FR-EQB-025 Per-condition timeframe and multi-timeframe (MTF) evaluation
- **Source features:** EQB-022
- **Requirement:** Each condition carries a timeframe from {1m, 3m, 5m, 10m, 15m, 30m, 1h, 1d, 1w, 1M} (default = base
  timeframe). The chip is disabled for conditions whose operands are all time-only (Candle Time). **Evaluation rules are defined
  once, in FR-BT-034, and apply identically to backtests, live deployments and scanners**; in summary: evaluation instants are
  base-TF candle closes, except that when an intraday condition timeframe is neither a multiple nor a divisor of the base
  timeframe the instants are the closes of LCM(base, those intraday timeframes) anchored at the session start; daily, weekly and
  monthly rows never change the instants and use their last fully closed candle (a week candle is complete only when the next
  week begins). The builder shows an inline note stating the effective evaluation interval when it differs from the base
  timeframe. Warm-up for higher timeframes is loaded automatically (FR-EQB-041); when unavailable, the condition is undefined
  (false) and the backtest report lists a warm-up warning.
- **Acceptance criteria:**
  1. Given base 5m and a 1h condition, when evaluating at 10:20, then the 1h value is from the 09:15–10:15 candle.
  2. Given base 3m and a 5m condition, then the builder shows "Evaluated every 15 minutes".
  3. Given base 1h and a 1d condition, then the builder shows no interval note (instants stay hourly) and the 1d value on
     Wednesday 11:15 is Tuesday's daily candle.
  4. Given a Candle Time-only condition, then its timeframe chip is disabled.
  5. Given EMA(50) on 1M with only 3 years of daily data, then the condition is undefined until 50 monthly candles exist and the
     backtest report shows a warm-up warning.
- **Data/Kite dependency:** candle store with local resampling (FR-DATA-002); scheduler (DEP module) on the app clock.
- **Deviation from Streak:** none intended (parity with documented MTF/LCM rules, A06 §1; Streak bug §3.2 #2 not replicated).
  The rule text lives in FR-BT-034 only (RV-008).
- **Priority hint:** Core

### FR-EQB-026 AND / OR joiners
- **Source features:** EQB-023
- **Requirement:** In Advanced mode, each pair of adjacent items in a group is joined by a toggleable "and"/"or" pill (colour-coded,
  keyboard-toggleable). "Add condition" appends a row joined by "and". Within one group, **AND binds tighter than OR** (A or B and C =
  A or (B and C)); when a group mixes and/or without sub-groups, the editor shows a non-blocking hint rendering the effective
  bracketing. Basic mode is AND-only.
- **Acceptance criteria:**
  1. Given rows A, B, C with joins "or","and" and truth values A=true, B=false, C=false, then the group evaluates true.
  2. Given A=false, B=true, C=false with joins "or","and", then false.
  3. Given a mixed chain, then the hint text shows `A or (B and C)`.
- **Data/Kite dependency:** Local only.
- **Deviation from Streak:** Precedence documented explicitly (Streak's is unknown — open question OQ-15).
- **Priority hint:** Core

### FR-EQB-027 Condition groups (nested brackets)
- **Source features:** EQB-024
- **Requirement:** In Advanced mode, "Add group" inserts a bracketed group (indented, bordered) containing its own rows, joiners,
  "Add condition", "Add group" (nesting) and group tools (enable/disable, copy, paste after, delete). Nesting depth limit 8. An empty
  group is incomplete. A group evaluates as one boolean in its parent chain.
- **Acceptance criteria:**
  1. Given `A and (B or C)` with A=true, B=false, C=true, then true.
  2. Given a group nested 8 deep, then "Add group" inside it is disabled with an explanatory tooltip.
  3. Given a group is deleted, then all its children are removed and the parent joiners are re-normalised (no dangling join).
- **Data/Kite dependency:** Local only.
- **Deviation from Streak:** Explicit depth limit (Streak's unknown).
- **Priority hint:** Core

### FR-EQB-028 Row and group tools
- **Source features:** EQB-025
- **Requirement:** Each Advanced row and group shall have: enable/disable switch (disabled items are greyed, kept, saved, and ignored
  in evaluation — a disabled item is removed from its chain together with its preceding joiner for evaluation purposes); Copy (to the
  logic clipboard); Paste after (inserts clipboard content after this item, joined by "and"); Duplicate; Delete. Tools are always
  visible (or exposed via a focusable "⋯" menu) and keyboard accessible. A block whose items are all disabled is incomplete.
- **Acceptance criteria:**
  1. Given `A and B or C` and B disabled, then evaluation equals `A or C`.
  2. Given row A copied, when Paste after on row C, then a copy of A appears after C with a new id.
  3. Given all rows disabled, then Execute is blocked with "No active entry conditions".
- **Data/Kite dependency:** Local only.
- **Deviation from Streak:** Disabled-item evaluation rule specified.
- **Priority hint:** Important

### FR-EQB-029 Logic clipboard (Copy all / Paste all)
- **Source features:** EQB-026
- **Requirement:** Below each condition block: "Copy all" copies the whole block; "Paste all" replaces the block's contents with the
  clipboard (confirm if the block is non-empty; undoable). The clipboard is app-wide, persists for the app session, and is also written
  to the OS clipboard as canonical DSL text; pasting accepts either the app clipboard or valid DSL text from the OS clipboard.
  Pasting into a Basic block that cannot represent the content follows FR-EQB-018 (offer switch to Advanced). Operands invalid for the
  target (e.g. Open Interest operands pasted into a strategy whose instruments are all cash equities) are flagged after paste.
- **Acceptance criteria:**
  1. Given strategy X's entry copied, when Paste all in strategy Y's exit block, then Y's exit conditions equal X's entry conditions (new ids).
  2. Given DSL text `RSI(14)[0] crosses above 30` in the OS clipboard, when Paste all, then one row with that condition results.
  3. Given invalid text in the OS clipboard, when Paste all, then an error toast "Clipboard does not contain valid conditions" appears and nothing changes.
- **Data/Kite dependency:** Local; OS clipboard via Wails runtime.
- **Deviation from Streak:** Clipboard scope defined (app-wide + OS text).
- **Priority hint:** Important

### FR-EQB-030 Exit risk parameters (Stop loss / Target / Trailing stop)
- **Source features:** EQB-027
- **Requirement:** The Exit card ("Exit — per instrument") shall have three optional numeric inputs Stop loss, Target profit,
  Trailing stop loss and one unit selector applying to all three: Percent (default, suffix "%"), Points ("pts"), Profit/loss in ₹
  ("₹"). Values must be > 0 (percent ≤ 100 for SL/TSL); empty = not used. Invalid input shows an inline message (not silently dropped).
  Engine semantics are defined by BT-021 (backtest) and DEP-006 (live); TSL is a fixed trail (no conditional modification).
- **Acceptance criteria:**
  1. Given unit Percent, when SL = 4 and TP = 6 are entered, then JSON risk = `{unit:"pct", sl:4, tp:6}`.
  2. Given "-5" typed into SL, then the field shows "Must be greater than 0" and Save/Execute are blocked until fixed or cleared.
  3. Given SL = 150 with unit Percent, then "Must be at most 100" is shown.
- **Data/Kite dependency:** None for definition; live protective exits may be mirrored to Kite GTT (C-03; DEP module).
- **Deviation from Streak:** Visible validation instead of silent rejection (§3.2 #7).
- **Priority hint:** Core

### FR-EQB-031 Exit conditions block
- **Source features:** EQB-028
- **Requirement:** "Add exit conditions (optional)" shall add a second, full condition block (mode switch, suggestions, rows,
  quick-setup tiles, groups in Advanced) evaluated with the same engine; a "Remove exit conditions" action (with confirm if non-empty)
  removes it. An exit is generated when the block evaluates true for an open position (trigger code EX, Appendix M). The exit block's
  mode is independent of the entry block's.
- **Acceptance criteria:**
  1. Given an exit block `RSI(14)[0] crosses below 70`, when backtested on a fixture where it fires 3 bars after entry, then the trade exits with trigger EX at the configured fill rule (BT-019).
  2. Given a non-empty exit block, when Remove is chosen, then a confirm appears; Cancel keeps it.
- **Data/Kite dependency:** Local only.
- **Deviation from Streak:** No per-block condition cap (see FR-EQB-036).
- **Priority hint:** Core

### FR-EQB-032 Footer actions: Save and Execute (two-step execution panel)
- **Source features:** EQB-030, EQB-039, BT-001, DEP-001
- **Requirement:** The footer shall show **Save** (new: opens FR-EQB-004; saved: saves in place) and, once saved, **Execute**,
  which opens the **execution panel** as a two-step modal. **Step 1 — "Choose execution type":** two radio cards with icon,
  title and one-line description — **Deploy** ("Get notified when the strategy's conditions are met and act on the signal",
  selected by default) and **Backtest** ("Test the strategy on historical data") — a right-hand preview pane (static
  illustration of our own per type; no animation required), a progress indicator (step 1 of 2), and footer **Cancel** / **Next**.
  **Step 2** is the parameters form of the chosen type (FR-BT-001 or FR-DEP-001) with footer **Back** / **Run backtest** or
  **Back** / **Deploy**; Back returns to step 1 keeping entered values. Escape or × on either step closes the panel without
  running or deploying. Execute is disabled with a reason list when the strategy is incomplete (FR-EQB-037). Starting a backtest
  from a dirty strategy saves it first (success toast), then runs. Deploy on step 2 is disabled until the risk disclosure of the
  session is acknowledged (FR-SHELL-020).
- **Acceptance criteria:**
  1. Given a new draft, then only Save is shown.
  2. Given a saved, complete strategy, when Execute is pressed, then step 1 opens with Deploy pre-selected and Next enabled.
  3. Given Backtest chosen and Next, then the backtest parameters form shows with Back and Run backtest; Back returns to step 1 with
     Backtest still selected.
  4. Given unsaved edits, when a backtest is started, then the strategy's `updatedAt` changes before the backtest record's
     `createdAt`.
  5. Given Escape on step 2, then the panel closes and nothing runs.
- **Data/Kite dependency:** Local; downstream BT/DEP modules.
- **Deviation from Streak:** Step 1 mirrors Streak's execution-type chooser (REVIEW-2 N2) with our own copy and a static preview;
  Deploy replaces the "Notification Based Trading" label (§3.2 glossary). Escape/× close (§3.2 #8).
- **Priority hint:** Core

### FR-EQB-033 Builder chart pane
- **Source features:** EQB-031
- **Requirement:** The right pane shall show the shared chart component (CHART-001) with one tab per selected instrument (scrollable
  tab strip with next/previous buttons and keyboard switching), rendering candles at the strategy's base timeframe, with the chart's
  own indicator and save controls. The chart timeframe follows FR-EQB-007. If candles are not yet cached, the pane shows backfill
  progress (C-08); offline it shows cached data with an offline badge (NFR-14).
- **Acceptance criteria:**
  1. Given 12 instruments, then 12 tabs exist and the strip scrolls; Ctrl+Tab cycles tabs.
  2. Given base = 1 day, then the active chart shows daily candles.
  3. Given network offline and cached data, then the chart renders and shows an "Offline — cached data" badge.
- **Data/Kite dependency:** `GET /instruments/historical/:token/:interval` via local candle store; optional live ticks from WebSocket.
- **Deviation from Streak:** Charting library per C-12.
- **Priority hint:** Important

### FR-EQB-034 Saved strategy header and overflow menu
- **Source features:** EQB-032
- **Requirement:** When a saved own strategy is open, the header shall show title + description, a favourite toggle (FR-EQB-035), the
  "My Strategies" button and an overflow menu with: Edit details (Save dialog), Duplicate (Copy dialog, DISC-008 behaviour), Export
  (FR-DISC-019), Delete (FR-MYS-007). Library strategies open read-only (FR-DISC-008), never in this editable view.
- **Acceptance criteria:**
  1. Given a saved strategy, when the overflow menu opens, then it lists Edit details, Duplicate, Export, Delete in that order.
  2. Given Delete is chosen and confirmed, then the strategy is removed and the app navigates to My Strategies.
- **Data/Kite dependency:** Local only.
- **Deviation from Streak:** Adds Export and Edit details.
- **Priority hint:** Important

### FR-EQB-035 Favourites
- **Source features:** EQB-033
- **Requirement:** Strategies and scanners have a favourite flag toggled by a star control in the builder header, list rows and
  detail panes (all reflect the same stored flag immediately). My Strategies / My Scanners lists sort favourites first within the
  active sort, and offer a "Favourites" filter.
- **Acceptance criteria:**
  1. Given a strategy starred in the builder, when My Strategies is opened, then its row star is filled and it appears above non-favourites.
  2. Given the Favourites filter is on, then only starred items are listed.
- **Data/Kite dependency:** Local only.
- **Deviation from Streak:** Adds favourites-first ordering and filter (Streak had none).
- **Priority hint:** Nice-to-have

### FR-EQB-036 Condition limits
- **Source features:** EQB-034
- **Requirement:** The system shall not cap the number of conditions per block in either mode (NFR-13). Structural rules remain:
  Basic = AND-only flat list; Advanced = AND/OR, groups, arithmetic, functions, weekly/monthly timeframes. For performance, a block
  with more than 50 conditions shows a non-blocking notice that evaluation may be slower.
- **Acceptance criteria:**
  1. Given a Basic entry block, when a 6th and an 11th row are added, then both are accepted and saved.
  2. Given 51 conditions, then a non-blocking performance notice appears.
- **Data/Kite dependency:** Local only.
- **Deviation from Streak:** Streak limits Basic to 5 entry + 5 exit and Advanced to 10 (Appendix N); removed per NFR-13.
- **Priority hint:** Important

### FR-EQB-037 Validation and completeness
- **Source features:** EQB-035
- **Requirement:** The system shall validate continuously and show results inline and in a summary popover on the Execute button.
  **Save** requires only a valid name (incomplete drafts may be saved and are marked "Incomplete" in lists). **Execute** (backtest or
  signals) requires: ≥1 instrument; ≥1 enabled, complete entry condition; no incomplete enabled rows/groups in any block; all parameters
  valid; operands compatible with instruments (e.g. Open Interest operands only on F&O instruments — otherwise error naming the
  instruments); direction/holding compatibility (FR-EQB-005). Each error has a stable code, message and a "go to" action focusing the
  offending element. Errors are never shown only as transient toasts.
- **Acceptance criteria:**
  1. Given a strategy with no instruments, when Execute is hovered/focused, then the summary lists "Add at least one instrument" (code `NO_INSTRUMENTS`).
  2. Given an `Open interest[0]` condition and an NSE cash instrument, then error `OPERAND_SEGMENT_MISMATCH` names that instrument.
  3. Given only a name, when Save is pressed, then the strategy is saved and shown as "Incomplete" in My Strategies.
  4. Given an error's "go to" action, when activated, then focus moves to the offending control.
- **Data/Kite dependency:** Local; instruments dump for segment checks.
- **Deviation from Streak:** Streak's save-time validation is unobserved and dialogs accept invalid values (§3.2 #7); we define a full validation set.
- **Priority hint:** Core

### FR-EQB-038 Indicator engine with golden test vectors
- **Source features:** EQB-036, EQB-037
- **Requirement:** The system shall implement, in Go, a deterministic indicator engine computing **every operand in Appendix A (rows
  1–71, including Central Pivot Range) and the ATR Trailing Stoploss entry, and every operand row of Appendix B** (Ulcer, Choppiness
  Index MA, OI MA, Open interest, Negative/Positive Volume Index, Klinger, True Range MA, Central pivot range, Vortex MA, Trade Volume
  Index, Moving average volume, CCI MA, Stochastic Momentum Index, MA Deviation, Schaff Trend Cycle, Intraday Momentum Index and its MA,
  Center of Gravity, Bollinger %B MA, ATR Trailing Stoploss, VWMA, Elder Ray, Day High Low, VWAP StdDev, PROC MA, TriMA, Candle Time),
  with every parameter, default and enumerated option listed there, plus the math functions of FR-EQB-022. Requirements:
  (1) each operand has a written formula spec in `docs/spec/indicators.md` (formula, warm-up length, session-reset behaviour, source
  reference); (2) the three MA-type enumerations of the catalogue — Appendix A "MA11" (11 types), the Moving Average operand's 9-type
  list (…hull, variable, vidya) and the Schaff Trend Cycle 9-type list (…hull, kaufman, vidya) — are each reproduced exactly
  and map to defined algorithms (simple, exponential, weighted,
  triple-exponential, triangular, double-exponential, Hull, MESA adaptive, variable, Kaufman adaptive, VIDYA) — see OQ-16;
  (3) session-anchored operands (VWAP and its MA/StdDev, Opening Range, Nth Candle, Day High Low, Previous N with candle=day, Pivot
  Points, Central Pivot Range, Intraday Momentum Index) reset/anchor at the session start given by FR-DATA-001 (09:15 on a regular day) using the previous session's
  daily OHLC where applicable; (4) Candle Time returns minute-of-hour (0–59), hour-of-day (0–23) or day-of-week (1 = Monday … 7 =
  Sunday) of the candle's start time; (5) Open interest operands require candles fetched with `oi=1` and are undefined elsewhere;
  (6) **golden test vectors (NFR-06):** a fixture set (≥ 3 instruments, ≥ 1,000 candles each across 1m, 15m and 1d, including gaps, a
  zero-volume candle, a flat run and a limit move) is committed with expected outputs for every operand × every enumerated option
  value (each MA type, each line/band/signal selector, each field) at default and at one non-default parameter set; expected outputs are
  produced by the reference implementation named **per operand** in `docs/spec/indicators.md` with its pinned version
  (default reference: TA-Lib through a pinned Python binding where the indicator exists there; otherwise an independently
  written Python implementation reviewed against the formula spec), with the seeding rule stated per operand (e.g. EMA/RSI
  Wilder vs simple seed — OQ-13); CI fails if any output differs
  by more than max(1e-6 absolute, 1e-6 relative) after the declared warm-up; (7) computation is incremental-capable (append one candle
  without full recompute) and produces identical results to full recompute.
  (8) **Implementation source (user decision 2026-10-12, revised):** every operand that has a TA-Lib v0.8.1 function shall be
  computed with Tradeloom's own pure-Go port, module `tradeloom/ta` (repo `ta/`; 201 functions incl. 61 candlestick patterns,
  each verified against the C library through the cgo oracle — `go test -tags oracle`, 1,824 tests, 0 failures on 2026-10-12;
  runtime builds with `CGO_ENABLED=0`). This covers e.g. SMA/EMA/WMA/DEMA/TEMA/T3/KAMA/TRIMA/MAMA/HMA/ZLEMA/VWMA, RSI, Stochastic,
  Stochastic RSI, SMI, CCI, Williams %R, MFI, CMO, Ultimate Oscillator, Momentum, ROC, ADX, ±DI, Aroon, MACD, Parabolic SAR,
  Supertrend, VWAP, Bollinger Bands, Keltner, Donchian, ATR, True Range, Standard Deviation, OBV, Chaikin A/D, Chaikin Money
  Flow, Elder Force, Vortex, Awesome Oscillator, Coppock, Fractal, NVI/PVI, IMI, Linear Regression/TSF, Median/Typical price,
  Min/Max. Only operands absent from TA-Lib v0.8.1 (e.g. Ichimoku, Pivot Points, CPR, Opening Range, Nth Candle, Previous N,
  Trend Intensity Index, Twiggs MF, Ehler Fisher, Choppiness, VIDYA, ATR Bands, Swing Index, Alligator composition) are
  implemented in package `indicators`, built on `ta` primitives and following `ta/CONVENTIONS.md`; `docs/spec/indicators.md`
  records the source (`ta.<FUNC>` or in-house) per operand. The external `markcheno/go-talib` is NOT used (superseded); no
  AGPL/GPL indicator library shall be linked (e.g. `cinar/indicator` is excluded). Golden vectors for `ta`-backed operands come
  from the C oracle; in-house operands follow clause (6).
- **Acceptance criteria:**
  1. Given the fixture set, when the golden-vector suite runs in CI, then 100% of operand × option cases pass within tolerance, and the suite fails if any Appendix A/B operand lacks a vector (coverage check against the catalogue).
  2. Given SMA(close, 9) on fixture F1 daily, then values match the reference at every index ≥ 8 within tolerance.
  3. Given VWAP on 1m fixture spanning two sessions, then VWAP resets at the first candle of the second session.
  4. Given incremental append of 500 candles one by one, then outputs equal the full-recompute outputs exactly (bitwise for deterministic float order).
  5. Given Candle Time(day of week) on a Wednesday candle, then 3.
  6. Given the operand catalogue, when the build's licence check runs, then every indicator dependency is MIT/BSD/Apache-compatible and each TA-Lib-covered operand calls `tradeloom/ta` (asserted by a registry test).
- **Data/Kite dependency:** OHLCV(+OI) candles from the local store fed by `GET /instruments/historical/:token/:interval?oi=1`.
- **Deviation from Streak:** Values may differ slightly from Streak's where its formulas are undocumented; ours are documented and tested.
- **Priority hint:** Core

### FR-EQB-039 Operand catalogue (single source of truth)
- **Source features:** EQB-036, EQB-037
- **Requirement:** A versioned operand catalogue (data file + Go registry) shall define for each operand: stable id; display name;
  short token used in canonical text; aliases; category (FR-EQB-016 taxonomy); availability (Basic+Advanced, or Advanced-only per
  Appendix B — CPR and ATR Trailing Stop are available in both); parameter schema (name, human label, type, default, min, max, step,
  enumeration values with labels); offset-less flag; required data (OHLCV, OI); segment restrictions; default condition template
  (Appendix A rightmost column, re-expressed in our DSL); warm-up function. UI lists, picker, dialogs, suggestions, validators, formatter,
  parser and engine all read this catalogue.
- **Acceptance criteria:**
  1. Given the catalogue, when a CI check runs, then every Appendix A operand (71) and every Appendix B operand is present with a parameter schema whose defaults **and enumeration values** equal the appendix (all three MA-type lists included).
  2. Given an operand added only to the catalogue file, when the app starts, then it appears in pickers and is formatted/parsed without code changes to UI components (engine function registration excepted).
  3. Given any two operands, then no two share an id, token or alias.
- **Data/Kite dependency:** Local only.
- **Deviation from Streak:** Consistent naming of tokens and labels (§3.2 #6).
- **Priority hint:** Core

### FR-EQB-040 Per-row timeframe in Basic mode
- **Source features:** EQB-038
- **Requirement:** Basic rows carry a timeframe like Advanced rows. The Basic view shows a timeframe column automatically when any row's
  timeframe differs from the base timeframe, and via a "Show timeframes" toggle otherwise; the cell is a normal timeframe dropdown
  (same list as FR-EQB-025) and never converts into an operand. Strategies imported/copied whose logic is not Basic-representable
  (arithmetic, functions, OR, groups) open in Advanced mode.
- **Acceptance criteria:**
  1. Given a copied library strategy whose rows use 15m with base 15m and only indicator operands, when opened, then it opens in Basic and the timeframe column is hidden (all equal base).
  2. Given a Basic row set to 1h with base 5m, then the timeframe column is visible and editing it to 1d updates the row without error.
  3. Given a copied strategy containing `Volume[-3] * 3.6`, when opened, then it opens in Advanced mode.
- **Data/Kite dependency:** Local only.
- **Deviation from Streak:** Streak's chip errors when clicked (§3.2 #2) and Basic displays expression operands; we keep modes clean.
- **Priority hint:** Important

### FR-EQB-041 Condition evaluation engine (shared)
- **Source features:** EQB-006, EQB-016, EQB-022
- **Requirement:** A single evaluation engine shall evaluate condition blocks for backtests, live signals and scanners with identical
  results for identical candles. It shall: evaluate only on closed candles; load warm-up history automatically (each operand declares
  its warm-up; exponential-family operands declare ≥ 4× period for convergence; minimum 100 candles); treat undefined values as
  false conditions; be deterministic (same inputs → same outputs, independent of goroutine scheduling); apply the MTF timing of FR-BT-034; evaluate one instrument ×
  one instant in < 1 ms for a 10-condition block on the reference machine (§5) (supports NFR-07/NFR-08); expose a per-condition trace (values of
  L, R, result) for debugging in backtest trade details and deployment logs.
- **Acceptance criteria:**
  1. Given the same strategy and candle fixture, when evaluated by the backtest path and by the live path (replaying candles as ticks closing candles), then the signal timestamps are identical.
  2. Given 100 runs with GOMAXPROCS varied 1..8, then outputs are identical.
  3. Given a benchmark of 10 conditions × 50 instruments × 1 instant, then p95 latency < 50 ms.
  4. Given a signal in a backtest, when the trade's details are opened, then each condition's L/R values at the signal instant are shown.
- **Data/Kite dependency:** Local candle store; live candles built from WebSocket ticks (DEP module).
- **Deviation from Streak:** Adds condition trace; parity of backtest vs live is a requirement.
- **Priority hint:** Core

### FR-EQB-042 Keyboard access to builder actions
- **Source features:** EQB-016, EQB-019, EQB-025
- **Requirement:** Every builder action that Streak exposes only on hover (operand remove/append, row tools, tile scroll, chart-tab
  scroll) shall be reachable by keyboard: Tab/Shift+Tab move between tokens and controls; Enter opens a token's editor; Delete removes the
  focused token/row (with undo); "+"/"-"/"*"/"/" after a focused token appends an operator; Ctrl+Z/Ctrl+Shift+Z (default bindings, FR-SHELL-019) undo/redo all builder
  edits within the session.
- **Acceptance criteria:**
  1. Given focus on token `Low[0]`, when Delete is pressed, then the token is removed; Ctrl+Z restores it.
  2. Given focus on `High[0]`, when "-" is typed, then the operator is appended and the expression picker opens.
  3. Given an automated accessibility scan of the builder, then no interactive element is reachable only by mouse hover.
- **Data/Kite dependency:** Local only.
- **Deviation from Streak:** Adds keyboard alternatives (NFR-11; §3.1 hover-only affordances).
- **Priority hint:** Important

---

## FR module DISC — Discover strategies & scanners

### FR-DISC-001 Discover Strategies view and disclaimer
- **Source features:** DISC-001
- **Requirement:** The system shall provide a "Discover strategies" master/detail view (sidebar) with: a persistent, non-dismissable
  disclaimer line in our own words stating that library strategies are educational examples, not recommendations, and that results are
  hypothetical; header actions "New strategy" (→ FR-EQB-001) and "My Strategies"; a left list (tabs, search, filter, rows) and a right
  detail pane (FR-DISC-007). The first list item is selected on load (or the last selected item, remembered per session).
- **Acceptance criteria:**
  1. Given the view opens, then the disclaimer is visible without scrolling and has no close control.
  2. Given the view opens for the first time, then the first list row is selected and its detail is shown.
- **Data/Kite dependency:** Local library + local backtest engine.
- **Deviation from Streak:** Own copy.
- **Priority hint:** Important

### FR-DISC-002 Library strategy content
- **Source features:** DISC-001, DISC-005, DISC-007
- **Requirement:** The app shall ship a versioned library of ready-made strategies authored by us from generic, well-known
  setups, each a valid StrategyDoc (FR-EQB-002) with: generic descriptive name (unique), plain-language description, category
  (Equity intraday / Equity swing), complexity (Basic/Advanced, derived from the condition mode), base timeframe, default
  instrument set (a named universe reference, e.g. Nifty 50, resolved from the user's imported lists at run time — no symbols
  are bundled, C-09), entry conditions, exit risk and/or exit conditions, trend tag (Bullish/Bearish), 1–4 style tags from the
  filter vocabulary (FR-DISC-005), a **family** tag from the fixed taxonomy below, and an optional `featured` flag (exactly 10
  shipped, used by FR-HOME-006). **Family taxonomy (coverage checklist, ≥ 1 strategy each):** volatility contraction; volatility
  expansion/burst; Bollinger mean reversion; Bollinger squeeze breakout; VWAP mean reversion; CPR/pivot breakout; pivot
  breakdown; SMA crossover; EMA crossover/reversal; WMA confluence; TEMA momentum shift; MACD momentum/trend shift; RSI
  reversal/breakout; stochastic reversal; stochastic RSI breakdown; CCI recovery; Williams %R with Supertrend; Supertrend with
  RSI; ADX trend strength; Aroon/ADX drive; previous-high break; previous-low break; prev-N close/open comparison; price below
  S1; volume-surge accumulation; volume breakdown with EMA; dual-SMA breakout. Coverage: ≥ 45 strategies across timeframes
  1m/5m/15m/1h/1d and both intraday and swing (Appendix E is the coverage reference; its names are not reproduced).
- **Acceptance criteria:**
  1. Given the library file, when CI runs, then ≥ 45 strategies validate, names are unique, every strategy has a trend tag, ≥ 1
     style tag from the vocabulary and a family tag from the taxonomy, every family has ≥ 1 entry, and exactly 10 are `featured`.
  2. Given each library strategy, when backtested on the golden candle fixture, then it runs without validation errors (smoke test).
  3. Given a text comparison, then no library name equals a name in Appendix E (case-insensitive).
- **Data/Kite dependency:** Local only; universe store for default instruments.
- **Deviation from Streak:** Own content (clean-room); may include Advanced-mode examples (Streak's are all Basic). Options
  strategies are not included (scope; §10 DISC-002-opt). Featured set tagged in data (RV-020).
- **Priority hint:** Important

### FR-DISC-003 Strategy category tabs
- **Source features:** DISC-002
- **Requirement:** Tabs **All · Equity swing · Equity intraday**, each showing its item count. The selected tab is remembered
  for the session.
- **Acceptance criteria:**
  1. Given a library with 30 intraday and 20 swing, then counts read All 50, Equity swing 20, Equity intraday 30.
  2. Given a tab selected, when the user navigates away and back within the session, then the same tab is active.
- **Data/Kite dependency:** Local only.
- **Deviation from Streak:** Counts shown. Option tabs are not offered (scope; §10 DISC-003-opt).
- **Priority hint:** Important

### FR-DISC-004 Strategy search
- **Source features:** DISC-003
- **Requirement:** A search box filters the list live (≤ 100 ms after keystroke) by case-insensitive substring over name, description
  and tags, combined (AND) with the active tab and filters; the first match is auto-selected. Empty result: illustration-free empty
  state "No strategies match" with actions "Clear search" and "New strategy".
- **Acceptance criteria:**
  1. Given "vwap", then only items whose name/description/tags contain "vwap" are listed and the first is selected.
  2. Given "zzzz", then the empty state with both actions is shown and the detail pane is cleared.
- **Data/Kite dependency:** Local only.
- **Deviation from Streak:** Searches description and tags too.
- **Priority hint:** Important

### FR-DISC-005 Strategy filters
- **Source features:** DISC-004
- **Requirement:** A filter button opens a popover with groups **Trend** (Bullish, Bearish) and **Style** (Momentum, Trend following,
  Reversal, Breakout, Breakdown, Trend confirmation, Uptrend, Downtrend, Momentum shift, Crossover, Price action, Mean reversion,
  Volatility). Multi-select; within a group OR, across groups AND. Apply (enabled when selection changed) and Clear. Filters match the
  strategies' tags. When any filter is active, the button shows a count badge. The same component is used in My Strategies (FR-MYS-002).
  Filtering applies to the entire catalogue (not only loaded rows).
- **Acceptance criteria:**
  1. Given Trend = Bullish and Style = {Breakout, Crossover}, then listed items are bullish AND (breakout OR crossover).
  2. Given a filter applied, then the button shows badge "3" for 3 selected values.
  3. Given 200 library items, then filtering returns matches from all 200 (no paging effects).
- **Data/Kite dependency:** Local only.
- **Deviation from Streak:** Active-filter badge (§3.2 #24); full-catalogue filtering (§3.2 #11 analogue).
- **Priority hint:** Important

### FR-DISC-006 Strategy list rows
- **Source features:** DISC-005
- **Requirement:** Each row shows a category icon (intraday vs swing), full name (wrapped, never truncated without tooltip),
  complexity badge (Basic/Advanced), base-timeframe chip, and (if computed) last summary P&L sign indicator. Rows are virtualised for
  smooth scrolling; keyboard ↑/↓ change selection.
- **Acceptance criteria:**
  1. Given a 60-character name, then it is fully readable (wrap or tooltip on focus/hover).
  2. Given focus in the list, when ↓ is pressed, then the next row is selected and the detail pane updates.
- **Data/Kite dependency:** Local only.
- **Deviation from Streak:** Keyboard navigation; no truncation loss (§3.2 #12 analogue).
- **Priority hint:** Important

### FR-DISC-007 Strategy detail pane with summary backtest
- **Source features:** DISC-006
- **Requirement:** The detail pane shows name, description, actions **Copy** (FR-DISC-009) and **View logic** (FR-DISC-008), and a
  "Hypothetical results" section from a **locally computed** summary backtest over the last 3 calendar months (configurable 1–36 months
  in Settings) using the library's default universe (resolved to the user's imported list) and default backtest parameters: period (long style `DD Mon YYYY to DD Mon YYYY`, FR-SHELL-016), "View full
  results" (→ BT-008), instrument chips (Combined + each instrument with LTP and % change), metrics Total P&L, Win/Loss count, Trades,
  Win/Loss streak, Max gain, Max loss, Avg win, Avg loss, Capital used (Appendix M definitions; FR-BT-014, FR-BT-046), weekday summary (avg return, max profit, max
  loss per weekday), monthly performance grid (% return per month, coloured). Results are cached with data range, parameters and app
  version (NFR-12) and show "Computed <timestamp>" with a **Recompute** action. This cached summary run and its window are the single source consumed
  by the Live Backtest widget (FR-HOME-006/007/008). If candles are missing, the pane offers "Download data"
  with backfill progress (C-08) instead of results. Footer: **Copy**, **Deploy** (requires copying first, FR-DISC-009).
- **Acceptance criteria:**
  1. Given cached candles, when an uncomputed item is selected, then a summary backtest runs off the UI thread and results appear; the UI remains interactive (NFR-07).
  2. Given computed results, when re-selected, then cached results show instantly with their computed timestamp.
  3. Given missing candles and offline, then the pane shows "Data not available offline" and no metrics.
  4. Given Total P&L, then it equals the sum of the per-trade P&L of the corresponding full backtest (consistency test).
- **Data/Kite dependency:** Local candle store (historical backfill via `GET /instruments/historical`), local backtest engine; LTP via `GET /quote/ltp` or ticker for chips.
- **Deviation from Streak:** Results computed locally on demand and cached, not served from a server (C-01, C-08); period configurable.
- **Priority hint:** Important

### FR-DISC-008 Read-only library strategy view
- **Source features:** DISC-007
- **Requirement:** "View logic" opens the builder layout in read-only mode: a "Library" badge, description, instruments, chart tabs,
  conditions (Basic or Advanced as authored, with timeframe chips, arithmetic, offsets, joiners), exits; all inputs disabled; a banner
  "Read-only — copy to edit". Footer: **Copy**, **Deploy** (via copy). Users can switch the display between Basic and Advanced
  rendering where representable without any data change and without confirmation.
- **Acceptance criteria:**
  1. Given a library strategy view, then no control modifies the doc (automated: all inputs disabled; doc hash unchanged after interacting).
  2. Given Copy is pressed, then the Copy dialog opens prefilled (FR-DISC-009).
- **Data/Kite dependency:** Local; historical candles for chart tabs.
- **Deviation from Streak:** Display-mode switch without destructive confirm.
- **Priority hint:** Important

### FR-DISC-009 Copy strategy dialog
- **Source features:** DISC-008
- **Requirement:** Copy (from Discover, read-only view, backtest page, My Strategies) opens a modal: Name (required, unique, prefilled
  "<source name> (copy)"), Category (prefilled from source), Tags (prefilled, ≤ 5), Description (prefilled). Buttons Cancel / Copy;
  Escape/× cancel. On Copy, a new user-owned StrategyDoc is created with `origin = {type: "library"|"user", libraryId?, libraryVersion?}`
  and opened in the editable builder. When invoked from "Deploy" on a library item, the button reads "Copy and continue" and
  then opens the deployment dialog (DEP-002).
- **Acceptance criteria:**
  1. Given a library item, when copied as "My VWAP test", then My Strategies contains it with origin.type = "library" and the builder opens it in edit mode.
  2. Given a duplicate name, then Copy is disabled with an inline message.
  3. Given the deploy path, then after copy the deployment dialog opens for the new copy.
- **Data/Kite dependency:** Local only.
- **Deviation from Streak:** Cancel/Escape always available, inline name validation (§3.2 #16).
- **Priority hint:** Core

### FR-DISC-010 Discover Scanners view
- **Source features:** DISC-009
- **Requirement:** A "Discover scanners" master/detail view (sidebar) with header actions "New scanner" (→ SCAN-001) and "My Scanners";
  left card: search, universe selector (FR-DISC-012), timeframe filter (FR-DISC-013), scanner list (FR-DISC-014); right detail pane
  (FR-DISC-015); sticky footer **Copy** and **Set alerts** (FR-DISC-018). The first item (or last selected) is selected on load. Library
  scanners are not run continuously in the background; results are computed on demand and optionally auto-refreshed while visible
  (FR-DISC-015).
- **Acceptance criteria:**
  1. Given the view opens, then the first scanner is selected and a scan for the selected universe starts if no fresh result exists.
  2. Given the view is closed, then no library-scanner computation continues in the background.
- **Data/Kite dependency:** Local scanner engine (SCAN module), local candle store, quotes.
- **Deviation from Streak:** Streak runs library scanners continuously on its servers; locally they run only on demand / while visible (C-03, C-07, C-08).
- **Priority hint:** Important

### FR-DISC-011 Library scanner content
- **Source features:** DISC-012, DISC-009
- **Requirement:** The app shall ship a versioned library of ready-made scanners authored by us, each a valid ScannerDoc
  (FR-SCAN-016) with a unique full name, description, universe reference, timeframe, complexity, tags, a family tag and an optional
  `featured` flag (the first 20 in library order are `featured` by default; FR-HOME-004). **Acceptance checklist — every family
  below must have ≥ 1 scanner implementing the stated generic rule and parameters (own name and description; Appendix F is the
  coverage reference):**
  | Family | Rule the library entry must implement | TF |
  |---|---|---|
  | Day gainers | `Close[0] up by 1 % from Close[-1]` | 1d |
  | Day losers | `Close[0] down by 1 % from Close[-1]` | 1d |
  | Opening-range breakout / breakdown | Close crosses above the 30-minute opening-range high / below its low | 5m |
  | Two-day higher highs / lower lows | HH & HL two sessions running; LH & LL two sessions running | 1d |
  | Short-term MA stack bullish / bearish | Close > SMA21 > SMA50 > SMA100; Close < SMA21 < SMA50 < SMA100 | 5m |
  | Momentum surge / fade | Close ≥ 0.5 % above / below the same candle's open | 5m |
  | Engulfing bullish / bearish | Two-candle engulfing pattern | 1h |
  | Volume breakout / breakdown (15m) | Close > previous close / < previous low, and volume > 20-period average volume | 15m |
  | Volume breakout (5m) | Close crosses above previous high with volume > 2 × 20-period average | 5m |
  | RSI entering bullish territory | RSI(14) crosses above 30 | 5m |
  | Possible downtrend | SMA21 ≤ SMA50 and RSI(14) < 50 | 5m |
  | 52-week high breakout / low breakdown | Close crosses above Period max(252, High[-1]) / below Period min(252, Low[-1]) | 1d |
  | Harami, inverted hammer, hammer, doji, marubozu (bullish/bearish) | Candlestick patterns per `docs/spec/patterns.md` | 15m / 1d |
  | Price near 200 SMA (bullish / bearish) | Close crosses above SMA200 with SMA200 rising / below with SMA200 falling | 1d |
  | RSI overbought / oversold | RSI(14) higher than 70 / lower than 30 | 1d |
  | Open = High / Open = Low | `Open[0] equal to High[0]` / `Open[0] equal to Low[0]` | 1d |
  | Range breakout (15m / 1h) | Close crosses above Period max(20, High[-1]) after range width ≤ 2 × ATR(14) | 15m, 1h |
  | Supertrend bullish / bearish crossover | Close crosses above / below Supertrend(7, 3) | 5m / 15m |
  | Falling RSI | RSI(14) lower than RSI(14)[-1] for 5 consecutive candles | 15m |
  | Narrow CPR | `[15m] Abs(CPR(tc) − CPR(bc)) lower than CPR(pp) × 0.001` | 15m |
  Coverage ≥ 37 scanners; each expressible in the DSL (Advanced where needed).
- **Acceptance criteria:**
  1. Given the library, when CI runs, then ≥ 37 scanners validate, names are unique, each family above has ≥ 1 scanner whose
     canonical DSL matches the stated rule (parameter check), and exactly 20 are `featured`.
  2. Given the golden fixture, then each scanner evaluates without error and pattern scanners match hand-labelled fixture candles
     (e.g. a known engulfing pair).
  3. Given a text comparison, then no library name equals a name in Appendix F (case-insensitive).
- **Data/Kite dependency:** Local only.
- **Deviation from Streak:** Own content; unique, untruncated names (§3.2 #12). Option-universe scanners (e.g. Streak's "Possible
  Downtrend" description mentioning Nifty options) run on equity universes here (scope).
- **Priority hint:** Important

### FR-DISC-012 Universe selector
- **Source features:** DISC-010
- **Requirement:** A dropdown of universes per FR-WATCH-009 (built-ins, custom baskets, watchlists in that order), default
  "Nifty 50". The choice persists across restarts and is shared with the dashboard scanner widget (FR-WATCH-011). Changing it
  relabels row universe pills (e.g. "Futures · Oct 2026" for the resolved futures month) and re-runs the selected scanner.
- **Acceptance criteria:**
  1. Given "Bank Nifty" selected, when the app restarts, then "Bank Nifty" is still selected here and in the dashboard widget.
  2. Given the Futures universe, then pills show the resolved contract month.
- **Data/Kite dependency:** Universe lists (user-importable, C-09); instrument master for Indices/Futures/NFO Equities.
- **Deviation from Streak:** Universe list is data-driven and user-editable (C-09); option universes dropped (§10
  WATCH-009-opt).
- **Priority hint:** Important

### FR-DISC-013 Scanner timeframe filter and search
- **Source features:** DISC-011
- **Requirement:** Segmented filter **All** plus one chip per base timeframe present in the library (from 1m, 3m, 5m, 10m, 15m, 30m, 1h, 1d),
  and a live case-insensitive search over name, description and tags. Filters apply to the entire library. Empty result shows "No
  scanners match" with "Clear filters" and "New scanner"; the detail pane clears.
- **Acceptance criteria:**
  1. Given 10 library scanners on 5m, when the 5m chip is selected, then exactly 10 are listed.
  2. Given "rsi", then all scanners with "rsi" in name/description/tags are listed.
  3. Given no matches, then the empty state shows and the detail pane is empty.
- **Data/Kite dependency:** Local only.
- **Deviation from Streak:** Filter covers all items (§3.2 #11); detail pane does not keep a stale selection.
- **Priority hint:** Important

### FR-DISC-014 Scanner list rows
- **Source features:** DISC-012
- **Requirement:** Rows show a pattern icon, full name (wrapping or tooltip, never ambiguous), universe pill, timeframe pill and
  complexity badge; selected row highlighted; keyboard navigation as FR-DISC-006.
- **Acceptance criteria:**
  1. Given two scanners with similar names, then their full names are distinguishable in the list.
  2. Given ↓ pressed, then the next scanner is selected and its detail loads.
- **Data/Kite dependency:** Local only.
- **Deviation from Streak:** No truncation ambiguity (§3.2 #12).
- **Priority hint:** Important

### FR-DISC-015 Scanner detail pane and results
- **Source features:** DISC-013
- **Requirement:** The detail pane shows name, complexity badge, "View conditions" (→ FR-DISC-017), description, "Results (N)" and "Last scan: DD-MM-YYYY HH:mm:ss" (single format app-wide), a "Scan now"
  action, and an "Auto-refresh while open" toggle (default on during market hours: re-scan at each close of the scanner's timeframe
  while the view is visible). Results use the shared results table (SCAN-007: Symbol with exchange tag, LTP, Change %, Volume, Buy/Sell
  buttons, Chart action; sortable, searchable, CSV export). Buy/Sell open the order ticket (ORD-004), disabled when trading is not enabled
  (C-04) or for indices. Empty state: "No instruments match right now — try another universe" with the universe selector focused.
  Results are computed by the local scanner engine (identical engine to FR-EQB-041).
- **Acceptance criteria:**
  1. Given a scan completes at 15:29:32 on 09-10-2026, then the header reads "Last scan: 09-10-2026 15:29:32".
  2. Given trading not enabled, then Buy/Sell are disabled with a tooltip linking to the trading setting.
  3. Given auto-refresh on and a 5m scanner, then scans occur at each 5-minute candle close while visible and stop when hidden.
  4. Given 0 results, then the empty state is shown with "(0)".
- **Data/Kite dependency:** Local candle store; live candles from ticker; quotes for LTP/Change/Volume (`GET /quote` batches or ticker).
- **Deviation from Streak:** One timestamp format (§3.2 #14); sorting/search/export added; on-demand rather than server-continuous; option filters dropped with the options scope (§10 DISC-016).
- **Priority hint:** Important

### FR-DISC-017 Library scanner view (read-only conditions)
- **Source features:** DISC-015
- **Requirement:** "View conditions" opens the scanner editor (SCAN-001) read-only: header with name and description, "Discover" and
  "My Scanners" links, overflow menu (Copy, Export); left card with universe selector (changeable), chart settings (timeframe locked),
  conditions rendered as authored (Basic/Advanced display toggle, non-destructive), comparator parameters visible but not editable; right
  card with results (FR-DISC-015 table incl. CSV export) and "Scan now" for the chosen universe. Footer: **Set alerts** (→ copy gate) and
  **Copy**. No Save.
- **Acceptance criteria:**
  1. Given the view, when the universe changes and Scan now is pressed, then results for the new universe appear and the library doc is unchanged.
  2. Given the Basic/Advanced toggle, then switching changes rendering only, without a confirmation and without data loss.
  3. Given CSV export, then the file contains Symbol, Exchange, LTP, Change %, Volume for all result rows.
- **Data/Kite dependency:** Local scanner engine.
- **Deviation from Streak:** Display toggle non-destructive; export format defined.
- **Priority hint:** Important

### FR-DISC-018 Copy scanner (gate for alerts on library scanners)
- **Source features:** DISC-016
- **Requirement:** Library scanners cannot be made live directly. **Set alerts** on a library scanner shows an info message ("Make a
  copy to set alerts") and opens a Copy scanner modal: Name (required, unique, prefilled "<name> (copy)"), Tags (prefilled, ≤ 5),
  Description (prefilled); buttons Cancel and Copy (or "Copy and set alerts" in the alert path); Escape/× cancel. After copy, the new
  scanner opens in the editable scanner editor with the alert dialog open (SCAN-009).
- **Acceptance criteria:**
  1. Given Set alerts on a library scanner, then the Copy modal opens with the alert-path button label.
  2. Given Escape, then the modal closes and nothing is created.
  3. Given Copy, then My Scanners contains the copy and the alert dialog is open on it.
- **Data/Kite dependency:** Local only.
- **Deviation from Streak:** Cancel/Escape available (§3.2 #16).
- **Priority hint:** Important

### FR-DISC-019 Sharing by export / import
- **Source features:** DISC-017
- **Requirement:** Users can **Export** a strategy or scanner (overflow menus, My Strategies/My Scanners rows) to a `.tlstrategy.json` /
  `.tlscanner.json` file containing the versioned doc (FR-EQB-002 / FR-SCAN-016) (no credentials, no account data, no backtest results unless "Include results" is
  ticked), and can **copy as text** (canonical DSL). **Import** (My Strategies/My Scanners header, or drag-and-drop a file onto the app)
  validates the file (schema version, catalogue operand ids), shows a preview (name, kind, instruments, conditions), resolves name
  conflicts (rename suggestion), and saves with `origin.type = "import"`. Unknown operands or newer schema versions are rejected with
  reasons.
- **Acceptance criteria:**
  1. Given a strategy exported and imported on a clean install, then the imported doc deep-equals the original except id, timestamps and origin.
  2. Given an exported file, then it contains no access token, API key or client id (security test grep).
  3. Given a file with an unknown operand id, then import is rejected naming the operand.
- **Data/Kite dependency:** Local files only.
- **Deviation from Streak:** No public URL sharing (no server); file/text export replaces it (C-03, personal use §1).
- **Priority hint:** Nice-to-have

### FR-DISC-020 Library versioning and updates
- **Source features:** DISC-001, DISC-009
- **Requirement:** Library content (strategies, scanners, suggestions, templates) is bundled per app release as versioned data files.
  Library items are read-only; user copies are independent and never modified by a library update. Cached summary results of a library
  item are invalidated when its library version changes. An "About library" entry shows each library's version and changelog.
- **Acceptance criteria:**
  1. Given a user copy of library item L v1, when the app updates to L v2, then the copy is unchanged and its `origin.libraryVersion` remains v1.
  2. Given L changes version, then its cached summary backtest is marked stale and recomputed on next view.
- **Data/Kite dependency:** Local only.
- **Deviation from Streak:** Explicit versioning (server-side in Streak).
- **Priority hint:** Nice-to-have

---

## FR module MYS — My Strategies / My Scanners

### FR-MYS-001 My Strategies view
- **Source features:** MYS-001
- **Requirement:** A "My Strategies" master/detail view with header subtitle (own copy), actions **New strategy**, **Import** (FR-DISC-019) and **Discover strategies**; left list (FR-MYS-002/003), right detail (FR-MYS-004). Default sort:
  favourites first, then last modified descending; sort menu offers Name A–Z, Last modified, Created, Last backtest P&L. Empty state:
  "No strategies yet" with **New strategy** and **Browse library**. Lists of ≥ 1,000 strategies scroll smoothly (virtualised).
- **Acceptance criteria:**
  1. Given no strategies, then the empty state with both actions is shown.
  2. Given strategies modified at different times, then the default order is favourites first, then most recently modified.
  3. Given 1,000 strategies (fixture), then scrolling stays at ≥ 50 fps on the reference machine (§5) and search returns in ≤ 100 ms.
- **Data/Kite dependency:** Local store (SQLite).
- **Deviation from Streak:** Sorting and import added; no list caps (NFR-13).
- **Priority hint:** Core

### FR-MYS-002 My Strategies tabs, search and filters
- **Source features:** MYS-002
- **Requirement:** Tabs **All · Equity swing · Equity intraday** with counts; search (name, description, tags); filters popover
  identical to FR-DISC-005 plus **Status** (Complete, Incomplete, Deployed) and **Favourites**. Active filters show a badge.
- **Acceptance criteria:**
  1. Given 3 intraday and 2 swing strategies, then counts read All 5, Equity swing 2, Equity intraday 3.
  2. Given filter Status = Incomplete, then only strategies failing Execute validation (FR-EQB-037) are shown.
- **Data/Kite dependency:** Local only.
- **Deviation from Streak:** Status and favourites filters added. Streak's Option Buying / Option Selling tabs are dropped with the
  options scope (§10 MYS-002-opt).
- **Priority hint:** Important

### FR-MYS-003 My Strategies row and row menu
- **Source features:** MYS-003
- **Requirement:** Each row shows kind/category icon, name, favourite star, complexity badge, base-timeframe chip (full text), status
  markers (Incomplete, Deployed — live deployment count), and an overflow menu (keyboard accessible) with **Edit**, **Duplicate** (Copy
  dialog), **Export**, **Delete** (FR-MYS-007).
- **Acceptance criteria:**
  1. Given a strategy with an active deployment, then the row shows a "Deployed" marker.
  2. Given the menu, then items are Edit, Duplicate, Export, Delete and Edit opens the builder.
- **Data/Kite dependency:** Local; deployment registry (DEP module).
- **Deviation from Streak:** Export and status markers added.
- **Priority hint:** Core

### FR-MYS-004 My Strategies detail pane
- **Source features:** MYS-004
- **Requirement:** The detail pane shows name, description, actions **Duplicate** and **Edit**; "Latest backtest" section with period
  and **View full results** (if a backtest exists) or "No backtest yet" with **Run backtest**; instrument chips (Combined + each with
  LTP/%); the same metrics, weekday summary and monthly grid as FR-DISC-007 from the latest saved backtest; a "Backtest history" link
  listing past runs (date, period, P&L); sticky footer **Run backtest** and **Deploy**.
- **Acceptance criteria:**
  1. Given a strategy without backtests, then "No backtest yet" and Run backtest are shown.
  2. Given 3 past backtests, then history lists 3 runs newest first and the pane shows the newest.
  3. Given a strategy marked Incomplete, then Run backtest and Deploy are disabled with the reason list (FR-EQB-037).
- **Data/Kite dependency:** Local backtest store; quotes for chips.
- **Deviation from Streak:** Backtest history added (local storage, no caps); options detail pane → §10 MYS-004-opt.
- **Priority hint:** Core

### FR-MYS-005 My Scanners view
- **Source features:** MYS-005, MYS-007
- **Requirement:** A "My Scanners" master/detail view with actions **New scanner**, **Import**, **Discover scanners**; left: search,
  working filter popover (Universe, Timeframe, Alert status Live/Paused/Off, Favourites), timeframe chips (All + present base timeframes),
  list; right: detail or "Select a scanner". Empty state "No scanners yet" with **New scanner** and **Browse library**. Default sort
  favourites first, then last modified.
- **Acceptance criteria:**
  1. Given no scanners, then the empty state is shown.
  2. Given filter Alert status = Live, then only scanners with active alerts are listed.
- **Data/Kite dependency:** Local store; live-scanner registry (SCAN module).
- **Deviation from Streak:** Filters implemented (Streak: "coming soon", §3.2 #31); no scanner count cap (NFR-13). Streak's `/scanners` alias route
  (REVIEW-2 N8) needs no equivalent — My Scanners is one view.
- **Priority hint:** Core

### FR-MYS-006 My Scanners row, menu and detail pane
- **Source features:** MYS-006
- **Requirement:** Rows show name, favourite star, universe pill, timeframe pill, and an alert-status indicator (Live / Paused / Off,
  text + colour). Row menu: **Edit**, **Duplicate**, **Export**, **Delete**. Detail pane: name, complexity badge, **Edit conditions**,
  description, results (shared results table; "Last scan" timestamp), footer **Scan again**, and **Set alerts** / **Pause alerts** /
  **Resume alerts** according to status (SCAN-009, SCAN-012).
- **Acceptance criteria:**
  1. Given a live scanner, then its row shows "Live" and the footer shows Pause alerts.
  2. Given Pause alerts pressed, then status becomes Paused and the alert engine stops evaluating it (SCAN-012).
  3. Given Scan again, then results and Last scan update.
- **Data/Kite dependency:** Local scanner engine; ticker for live alerts (SCAN module).
- **Deviation from Streak:** Visible live/paused status and toggle (MYS-006 open question).
- **Priority hint:** Core

### FR-MYS-007 Deletion and referential integrity
- **Source features:** MYS-003, MYS-006
- **Requirement:** Deleting a strategy or scanner requires a confirmation naming the item and listing dependent data (backtests count,
  active deployments/alerts). Deletion is **blocked** while the item has an active deployment or live alerts (message with a link to stop
  them). Deleted items go to a 30-day "Recently deleted" area from which they can be restored with their backtests; after 30 days (or on
  "Delete permanently") they and their backtests are purged. Deployment/alert journals referencing the item are retained (NFR-09/10).
- **Acceptance criteria:**
  1. Given an active deployment, when Delete is chosen, then deletion is blocked with a "Stop deployment first" message.
  2. Given a deleted strategy, when restored from Recently deleted, then it and its backtests reappear unchanged.
  3. Given purge, then its backtests are removed but the order/deployment journals still show entries referencing the strategy name.
- **Data/Kite dependency:** Local store.
- **Deviation from Streak:** Confirmation, blocking and soft delete defined (Streak's delete confirmation unobserved).
- **Priority hint:** Important

### FR-MYS-008 Persistence of user strategies and scanners
- **Source features:** MYS-001, MYS-005
- **Requirement:** User strategies and scanners are stored in the local SQLite database (NFR-03) as StrategyDoc / ScannerDoc JSON (FR-EQB-002, FR-SCAN-016) plus indexed columns
  (name, kind, category, tags, favourite, timestamps, status). Writes are transactional; an autosave draft copy of an open, dirty
  editor is written every 10 s to a recovery table and offered on next launch after a crash (NFR-09). Schema migrations follow FR-DATA-005.
- **Acceptance criteria:**
  1. Given the app is killed during editing, when restarted, then a "Recover unsaved changes?" prompt offers the draft (≤ 10 s old).
  2. Given a write failure (disk full simulated), then the user sees an error and the previous saved version remains intact.
- **Data/Kite dependency:** Local only.
- **Deviation from Streak:** Local storage with crash recovery instead of server storage (§1 G3).
- **Priority hint:** Core

---

## FR module BT — Backtesting

### FR-BT-001 Backtest parameters (execution panel step 2)
- **Source features:** BT-001, EQB-039
- **Requirement:** The system shall present, as step 2 of the execution panel (FR-EQB-032) when Backtest is chosen, and from My
  Strategies "Run backtest" and the results view "Run backtest" (which open the panel directly on this step), a parameters form
  with: Initial Capital (number, default 25000), Start Date, End Date (default today), Entry Start Time and Entry End Time (hh:mm
  with AM/PM, defaults 09:15 AM and 03:30 PM), Holding Type (Intraday MIS / Longterm CNC; futures additionally NRML — see
  FR-BT-004), Strategy Cycle ({–, 1…5}, default "–", disabled when CNC), and footer **Back** / **Run backtest** (Cancel exists
  only on step 1). There is no quantity field (quantity is derived, FR-BT-006). The panel shall close on Escape and × (Streak bug
  §3.2 #8 not replicated).
- **Acceptance criteria:**
  1. Given a saved strategy on a 15-min base TF, when the form opens, then Initial Capital = 25000, End Date = today, Start Date =
     today − 90 calendar days (FR-BT-008 default), times = 09:15 AM / 03:30 PM, Holding = MIS, Cycle = "–".
  2. Given Holding Type = CNC, then the Strategy Cycle control is disabled and its value is ignored by the engine.
  3. Given Start Date > End Date, or Entry Start Time ≥ Entry End Time, or Initial Capital ≤ 0, when Run backtest is pressed, then
     the run is blocked and an inline message names the offending field.
  4. Given the form is open, when Back is pressed, then step 1 shows with Backtest still selected and the entered values are kept
     when returning.
  5. Given the form is open, when Escape is pressed, then the panel closes without running or saving.
- **Data/Kite dependency:** Local strategy store (SQLite); candle store (FR-DATA-002).
- **Deviation from Streak:** Footer Back / Run backtest matches Streak (REVIEW-2 #14); Escape closes the panel and validation is
  inline (Streak behaviour unobserved).
- **Priority hint:** Core

### FR-BT-002 Strategy cycle (max entries per day per instrument)
- **Source features:** BT-002
- **Requirement:** For MIS backtests the engine shall cap the number of entries (EN) per instrument per trading day at the
  selected cycle count N (1–5). "–" shall mean no cap. The counter resets at the first candle of each trading day. An
  entry signal that would exceed the cap is ignored and recorded in the run diagnostics ("cycle limit reached"). For CNC
  (and NRML) backtests no cycle cap applies.
- **Acceptance criteria:**
  1. Given MIS, cycle = 2, and on one day fresh entry signals at 09:30, 10:30 and 11:30 each followed by an exit before the
     next signal, then exactly two EN rows exist for that day (the 09:30 and 10:30 signals) and the 11:30 signal is listed in
     diagnostics as skipped.
  2. Given the same data with cycle = "–", then three EN rows exist.
  3. Given cycle = 1 and an entry on day D, then an entry signal on day D+1 is accepted (counter reset).
  4. Given CNC with any cycle value, then entries are limited only by the one-position rule (FR-BT-024).
- **Data/Kite dependency:** none.
- **Deviation from Streak:** "–" = unlimited is an interpretation (Streak does not document it; see Open Questions OQ-22).
- **Priority hint:** Core

### FR-BT-003 Entry window
- **Source features:** BT-003
- **Requirement:** The engine shall accept a new entry only if the fill candle's open time t satisfies
  Entry Start Time ≤ t ≤ Entry End Time. The entry window shall not force an exit; open positions continue to be managed by
  SL/TP/TSL/EX/SO rules after End Time. (Chosen interpretation; Streak's docs describe the window as "entry" start/stop.)
- **Acceptance criteria:**
  1. Given a 15-min base TF, window 09:30–14:00, and a fresh entry signal on the 09:15 candle (fill at 09:30 open), then the
     entry is taken.
  2. Given the same window and a signal on the 13:45 candle (fill at 14:00 open), then the entry is taken; a signal on the
     14:00 candle (fill 14:15) is skipped and recorded as "outside entry window".
  3. Given a long opened at 10:00 that is still open at 14:00 under MIS, then it remains open until an exit rule or the
     square-off (FR-BT-031) closes it.
- **Data/Kite dependency:** none.
- **Deviation from Streak:** none intended (exact Streak boundary semantics unverified — OQ-24).
- **Priority hint:** Core

### FR-BT-004 Holding type and product mapping
- **Source features:** BT-004
- **Requirement:** The engine shall support Holding Type MIS (intraday; square-off per FR-BT-031; cycles per FR-BT-002) and
  CNC (positions carried overnight; no square-off; open positions marked to market daily per FR-BT-019). For futures
  instruments the "Longterm" choice shall map to product NRML (CNC is not valid for derivatives on Kite).
- **Acceptance criteria:**
  1. Given MIS, then no position is held across a trading-day boundary.
  2. Given CNC and a long opened on day D with no exit until D+3, then one EN row on D and no SO rows exist, and the
     calendar shows MTM day P&L for D…D+3 (FR-BT-019).
  3. Given a futures instrument and Longterm selected, then the run's assumptions record product = NRML.
- **Data/Kite dependency:** Kite products MIS, CNC, NRML (07 §A7).
- **Deviation from Streak:** Futures mapped to NRML explicitly (Streak's handling is undocumented).
- **Priority hint:** Core

### FR-BT-005 Initial capital is per instrument
- **Source features:** BT-005
- **Requirement:** Initial Capital C shall be applied independently to each instrument in the strategy; instruments do not
  share or compete for capital. Combined results (FR-BT-012) aggregate per-instrument results.
- **Acceptance criteria:**
  1. Given 3 instruments and C = 25000, then each instrument's simulation starts with 25000 and the run assumptions record
     "capital per instrument = 25000, total notional = 75000".
- **Data/Kite dependency:** none.
- **Deviation from Streak:** none — per-instrument capital is confirmed on the current platform (Backtest Assumptions, Mar 2026: "for every instrument"; the observed qty 9 and qty 2 from one ₹25,000 reproduce; REVIEW-1 §4).
- **Priority hint:** Core

### FR-BT-006 Quantity derivation
- **Source features:** BT-005
- **Requirement:** For each entry the quantity shall be qty = floor(C ÷ P_fill), where P_fill is the entry fill price
  (fill-candle open, after any configured slippage, FR-BT-036). For futures, qty shall be rounded down to a whole multiple of
  the lot size: qty = floor(C ÷ (P_fill × lot)) × lot. If qty = 0 the entry is skipped with reason "capital below price".
- **Acceptance criteria:**
  1. Given C = 25000 and P_fill = 2651.80, then qty = 9 (25000 / 2651.80 = 9.43).
  2. Given C = 25000 and P_fill = 8835.50, then qty = 2.
  3. Given C = 25000 and P_fill = 31000, then no trade is opened and the skip reason is recorded.
  4. Given a futures contract with lot 65, C = 2,000,000, P_fill = 22,500, then qty = floor(2,000,000 / 1,462,500) × 65 = 65.
- **Data/Kite dependency:** Lot size from the instruments dump (`GetInstruments`).
- **Deviation from Streak:** Lot rounding for futures is an interpretation (Streak undocumented; OQ-25).
- **Priority hint:** Core

### FR-BT-007 Capital gating, no margin or leverage, freeze limits ignored
- **Source features:** BT-005, BT-022
- **Requirement:** An entry shall be taken only if P_fill × qty ≤ C + R, where R is the instrument's cumulative gross
  realised P&L up to the previous closed trade. No margin, leverage or exposure multiplier shall be applied for any product
  (including MIS and futures). Exchange freeze-quantity limits shall not be applied in backtests. Gating uses gross P&L so
  the Trading Charges toggle never changes which trades occur.
- **Acceptance criteria:**
  1. Given C = 25000, a first trade of 9 @ 2651.80 closing with R = −1500, and a next fill at 2600.00, then qty = 9, cost =
     23,400 ≤ 23,500 → entry taken.
  2. Given the same but next fill at 2620.00, then cost = 23,580 > 23,500 → entry skipped with reason "insufficient capital".
  3. Given every entry of an instrument is skipped by gating, then the instrument shows the message of FR-BT-040 case 2.
  4. Given an MIS strategy, then exposure never exceeds C + R (no 5× intraday leverage).
- **Data/Kite dependency:** none.
- **Deviation from Streak:** none.
- **Priority hint:** Core

### FR-BT-008 Date range and lookback (no artificial caps)
- **Source features:** BT-006
- **Requirement:** The user shall be able to choose any Start/End Date for which local candle data exists or can be
  backfilled. Default Start Date shall equal today minus the familiar per-TF window (1 min → 30 days; 3–30 min → 90 days;
  1 hour → 365 days; 1 day → 3 years). The engine shall automatically load warm-up candles before Start Date (enough for the
  longest indicator lookback in the strategy, including MTF rows) so indicators are valid on the first evaluated candle;
  signals on warm-up candles are never traded. If data is missing for the requested range the dialog shall show per
  instrument what is missing and offer to backfill (FR-BT-009) or to run on available data only.
- **Acceptance criteria:**
  1. Given a 1-min strategy and Start Date 2 years ago with local data present, then the run proceeds (no 30-day cap).
  2. Given an EMA(200) on a 15-min base, then ≥ 200 candles before Start Date are loaded and the first evaluable candle at
     Start Date has a defined EMA value.
  3. Given an instrument whose local data starts after Start Date and the user chooses "available data only", then that
     instrument runs from its first available candle and the assumptions record the effective range per instrument.
- **Data/Kite dependency:** Candle store (FR-DATA-002); backfill via FR-DATA-003 (minute data from ~2015).
- **Deviation from Streak:** **Enhancement (beyond Streak):** lookback limited only by local data (NFR-13); Streak caps at
  30 d / 90 d / 365 d / 3 y.
- **Priority hint:** Core

### FR-BT-009 Data backfill for backtests
- **Source features:** BT-006, BT-007
- **Requirement:** Before a run the system shall request the missing candles for every instrument and every timeframe used
  (base and MTF rows, plus warm-up per FR-BT-008) from the backfill service (FR-DATA-003) at backtest priority, and show its
  progress (instruments done / total, windows done / total, ETA) in the run dialog with a Cancel. Week/month and any non-native
  interval are resampled on read by the candle store (FR-DATA-002); nothing is fetched twice for ranges already covered. The run
  starts when all requested ranges are present (or when the user chose "available data only", FR-BT-008).
- **Acceptance criteria:**
  1. Given 50 instruments missing 90 days of 15-minute data, then exactly 50 historical requests are issued by FR-DATA-003 (one
     200-day window each) at ≤ 3/s and progress reaches 50/50.
  2. Given the app is closed mid-backfill, when it restarts and the run is re-requested, then only windows not yet done are fetched
     (FR-DATA-003 AC3).
  3. Given an interactive chart load during the backfill, then the chart's requests are served first (priority) and the backtest
     progress continues afterwards.
- **Data/Kite dependency:** FR-DATA-003 (`GetHistoricalData`), FR-DATA-002.
- **Deviation from Streak:** Data is fetched on demand and cached locally (Streak is server-side).
- **Priority hint:** Core

### FR-BT-010 Run backtest: save, navigate, stream per instrument, cancel
- **Source features:** BT-007
- **Requirement:** Running a backtest shall first save the strategy (confirmation toast), then open the results view for
  that strategy and stream results per instrument as each completes (per-instrument spinner until done). The engine shall run
  off the UI thread (NFR-07), may run instruments in parallel, and shall offer Cancel; a cancelled run keeps completed
  instruments marked "partial run". There is no daily backtest cap.
- **Acceptance criteria:**
  1. Given an unsaved edited strategy, when Run is pressed, then the strategy version is persisted before the run starts and
     the run references that version.
  2. Given 50 instruments × 90 days × 15-min with cached data on the reference machine (§5), then all results are shown in < 10 s
     (NFR-07) and the UI remains responsive (input latency < 100 ms) during the run.
  3. Given Cancel is pressed after 10 of 50 instruments complete, then those 10 are shown and the run is labelled partial.
  4. Given 1,000 runs in a day, then none is refused.
- **Data/Kite dependency:** Local engine; local store.
- **Deviation from Streak:** No daily cap ("Limit Exceeded" not reproduced); Cancel added.
- **Priority hint:** Core

### FR-BT-011 Results page layout and header actions
- **Source features:** BT-008
- **Requirement:** The results view shall show: header with strategy name and backtest period (DD Mon YYYY to DD Mon YYYY);
  header actions — for a prebuilt (read-only) strategy: View, Copy Strategy, Deploy; for an own strategy: Edit, Run Backtest,
  Deploy; left column Combined Results + per-instrument table (FR-BT-012); right pane for the selected instrument (symbol,
  exchange, LTP, % change) with Trading Charges toggle, metrics row, weekday summary, monthly grid, chart with trade markers,
  P&L calendar, trade log, cumulative P&L chart (FR-BT-013…FR-BT-022), and an "Assumptions" panel (FR-BT-044).
- **Acceptance criteria:**
  1. Given an own strategy result, then Edit, Run Backtest and Deploy are present and Copy is absent.
  2. Given a prebuilt strategy result, then View, Copy Strategy and Deploy are present and Edit is absent.
  3. Given selection of a different instrument in the left table, then every right-pane section re-renders for that
     instrument without re-running the engine.
- **Data/Kite dependency:** Local results store; LTP from ticker (optional, shows last close offline).
- **Deviation from Streak:** Assumptions panel added (NFR-12).
- **Priority hint:** Core

### FR-BT-012 Combined results and per-instrument table
- **Source features:** BT-009, BT-008
- **Requirement:** Combined Results shall show total P&L, wins, losses and trades summed over all instruments; the
  per-instrument table shall list instrument and P&L (green > 0, red < 0) and switch the right pane on click. Combined
  metrics are cumulative across all instruments (sum of per-instrument values; combined Max DD per FR-BT-014 on the merged,
  exit-time-ordered trade sequence with capital = C × instrument count). Table rows shall be sortable by P&L
  (Enhancement).
- **Acceptance criteria:**
  1. Given instrument P&Ls of +1,200, −523.79 and +300 with 4/6/2 trades, then Combined P&L = 976.21 and Trades = 12.
  2. Given the Trading Charges toggle is ON, then combined and per-instrument values are net of charges.
- **Data/Kite dependency:** none.
- **Deviation from Streak:** Sorting added (Enhancement).
- **Priority hint:** Core

### FR-BT-013 Trading charges model (both legs, toggle)
- **Source features:** BT-010
- **Requirement:** The engine shall compute, for every executed leg (entry and exit), brokerage, STT/CTT, exchange
  transaction charge, SEBI turnover fee, stamp duty (buy side) and GST, from a versioned local fee table keyed by segment and
  product. Each row of the fee table defines: brokerage rule (percentage of turnover with cap, or flat), STT/CTT rate and side,
  transaction-charge rate, SEBI fee per crore, stamp-duty rate and side, GST rate and base, and rounding rule. The results view
  shall have a Trading Charges toggle, default OFF; ON shows every P&L figure net of both legs' charges. The fee table shall be
  user-editable and validatable: a "Validate against broker" action submits synthetic orders through `GetOrderCharges` (no
  order is placed) and reports per-component differences.
- **Acceptance criteria:**
  1. Given the **test fixture** fee table (illustrative values only, not authoritative): brokerage 0.03 % capped ₹20/leg;
     STT 0.025 % on sell, rounded to ₹1; transaction 0.00297 %; SEBI ₹10/crore; stamp 0.003 % on buy, rounded to ₹1; GST 18 %
     on (brokerage + transaction + SEBI); **rounding rule:** each component half-up to ₹0.01 except STT and stamp duty, which are
     half-up to ₹1, and GST is computed on the rounded components — and a long MIS round trip buy 9 @ 2651.80, sell 9 @ 2600.00, then buy-leg charges =
     10.31 (brokerage 7.16, txn 0.71, SEBI 0.02, stamp 1.00, GST 1.42), sell-leg charges = 15.12 (brokerage 7.02, STT 6.00,
     txn 0.69, SEBI 0.02, GST 1.39), total 25.43; gross P&L −466.20; net P&L −491.63.
  2. Given the toggle is OFF, then the same trade shows −466.20.
  3. Given "Validate against broker" for a sample of synthetic orders, then each component's difference vs the virtual
     contract note is shown and differences > ₹0.05 per leg are flagged.
  4. Given the fee table is edited, then previous results keep the fee-table version they were computed with.
- **Data/Kite dependency:** Local fee table; `GetOrderCharges` (POST /charges/orders) for validation (07 §A9, C-10).
- **Deviation from Streak:** Stamp duty included (Streak does not name it); fee table visible and versioned.
- **Priority hint:** Core

### FR-BT-014 Metric definitions
- **Source features:** BT-011
- **Requirement:** For each instrument and for Combined, the system shall compute over **closed** round trips (gross or net per
  the charges toggle): Total P&L = Σ trade P&L; Trades = number of closed round trips; Wins = trades with P&L > 0; Losses =
  trades with P&L < 0 (P&L = 0 counted as neither, shown as "breakeven" count); Win Streak / Loss Streak = longest run of
  consecutive wins / losses in exit-time order (breakeven breaks a run); Max Gain = largest single-trade P&L; Max Loss =
  smallest (most negative) single-trade P&L; Winning Avg = Σ winning P&L ÷ Wins; Losing Avg = Σ losing P&L ÷ Losses; Max DD
  computed on the equity curve E₀ = C, Eᵢ = C + cumulative P&L after trade i: Max DD = maxᵢ (Peakᵢ − Eᵢ) ÷ Peakᵢ where Peakᵢ =
  max(E₀…Eᵢ), shown as a positive percentage with the ₹ amount (Peak − Trough) alongside; 0 when no drawdown; Capital used per FR-BT-046. Trade P&L: long =
  (exit − entry) × qty; short = (entry − exit) × qty.
- **Acceptance criteria:**
  1. Given C = 25000 and closed trades in order +500, −200, −300, +800, +100, then Total P&L = 900; Trades 5; Wins 3; Losses 2;
     Win Streak 2; Loss Streak 2; Max Gain 800; Max Loss −300; Winning Avg 466.67; Losing Avg −250.00; equity 25000, 25500,
     25300, 25000, 25800, 25900 → Max DD = 500 / 25500 = 1.96 % (₹500).
  2. Given only winning trades, then Max DD = 0.00 %.
  3. Given a short entry 500 × 50 exited at 490, then trade P&L = +500.
  4. Given an open CNC position at the end of the range, then it is excluded from Trades/Wins/Losses and shown as
     "Open position, unrealised P&L ₹x" in the metrics row.
- **Data/Kite dependency:** none.
- **Deviation from Streak:** Max DD is computed on equity (capital + cumulative P&L) and shown as a positive %, because
  Streak's `(Peak − Trough)/Peak` on the raw P&L curve has unclear sign and divides by zero at peak 0 (FEATURES §6-16).
  Breakeven handling is explicit.
- **Priority hint:** Core

### FR-BT-015 Weekday summary
- **Source features:** BT-012
- **Requirement:** The system shall show, per weekday Mon–Fri: Avg Return (mean of day-return % over that weekday's active
  days), Max Profit (largest day P&L ₹) and Max Loss (smallest day P&L ₹). Day P&L = realised P&L of trades exited that day +
  change in unrealised MTM of open CNC/NRML positions (FR-BT-019). Day return % = day P&L ÷ C × 100. Active day = a day with
  any trade activity or an open position.
- **Acceptance criteria:**
  1. Given C = 25000 and Monday day P&Ls of −370 and +250, then Monday Avg Return = (−1.48 % + 1.00 %) / 2 = −0.24 %, Max
     Profit = 250, Max Loss = −370.
  2. Given no activity on any Thursday, then the Thursday row shows "—".
- **Data/Kite dependency:** none.
- **Deviation from Streak:** Exact Streak averaging base unverified (OQ-26); definition made explicit.
- **Priority hint:** Important

### FR-BT-016 Monthly performance grid
- **Source features:** BT-013
- **Requirement:** The system shall show a year × month grid where each month tile shows month return % = Σ day P&L in that
  month ÷ C × 100, coloured green (> 0) or red (< 0) with intensity proportional to |return| relative to the largest
  |month return| in the run; months outside the range are blank.
- **Acceptance criteria:**
  1. Given C = 25000 and July day P&Ls summing to +1,000, then the Jul tile shows +4.00 % in green.
  2. Given the largest |month return| is 8 %, then a 4 % tile has half the maximum colour intensity.
- **Data/Kite dependency:** none.
- **Deviation from Streak:** none.
- **Priority hint:** Important

### FR-BT-017 Chart with trade markers
- **Source features:** BT-014
- **Requirement:** The results view shall show a candle chart of the selected instrument at the base TF over the backtest
  range with a marker on every trade event, labelled with its trigger code (EN, EX, SL, TP, TSL, SO, EXP) and placed on the
  candle and price at which it filled; hovering/focusing a marker shows time, price, qty and P&L. A "See all transactions"
  action scrolls to the trade log. Clicking a trade-log row centres the chart on that trade.
- **Acceptance criteria:**
  1. Given a trade with EN at 09:25 open 1002.50 and SL at 980.00, then two markers appear at those candles/prices labelled
     EN and SL.
  2. Given keyboard focus on a marker, then the same tooltip as hover is shown (NFR-11).
- **Data/Kite dependency:** Local candles; open-licence charting library (C-12).
- **Deviation from Streak:** All seven codes are marked (Streak's chart shows EN, EX, SO); row→chart linking added.
- **Priority hint:** Important

### FR-BT-018 Trade-markers and chart data for MTF/long ranges
- **Source features:** BT-014
- **Requirement:** For ranges exceeding the chart's comfortable candle count, the chart shall lazy-load candles around the
  viewport and still show all markers in range.
- **Acceptance criteria:**
  1. Given 2 years of 1-min data (~180,000 candles), then the chart opens in < 2 s and scrolling to any trade shows its markers.
- **Data/Kite dependency:** Local candle store.
- **Deviation from Streak:** Needed because lookback is uncapped (FR-BT-008).
- **Priority hint:** Nice-to-have

### FR-BT-019 Daily P&L calendar heatmap with CNC mark-to-market
- **Source features:** BT-015
- **Requirement:** The system shall show one mini-grid per month with one cell per trading day coloured by day P&L (green/red,
  grey = no activity or holiday), tooltip "DD Mon, YYYY: ±x.xx %". Open CNC/NRML positions shall be marked to that day's close
  against the previous day's mark (first day: against entry price), so the sum of day P&Ls equals realised P&L once closed.
- **Acceptance criteria:**
  1. Given buy 10 @ 100 on day 1 (close 103), day 2 close 101, exit at 105 on day 3, then day P&Ls are +30, −20, +40 and they
     sum to the realised +50.
  2. Given C = 25000 and a day P&L of −370, then the tooltip reads "13 Jul, 2026: −1.48 %".
- **Data/Kite dependency:** Local daily closes (day candles or last intraday close).
- **Deviation from Streak:** Streak's docs mark open CNC positions "against the buy price" (a cumulative view, A06 §2); we mark against the previous day's close so day P&Ls sum to the realised P&L (daily-delta model; OQ-26, RV-027).
- **Priority hint:** Important

### FR-BT-020 Trade log table
- **Source features:** BT-016
- **Requirement:** The trade log shall list every fill with columns Date & Time, Symbol, **P&L** (the **cumulative running
  P&L** of the instrument after that row, on every row — parity with Streak's observed log), Price, Buy/Sell, Qty (signed:
  + buy, − sell), Trigger code, and **Enhancement (beyond Streak)** columns **Trade P&L** (realised P&L of the round trip,
  shown on the exit row, blank on entry rows), Fill Time and Reason/diagnostic. Date & Time shall be the signal candle's open
  time for EN/EX (fill at the next candle's open), and the open time of the candle in which the event occurred for
  SL/TP/TSL/SO/EXP. The table shall paginate (default 50 rows/page, configurable) with Prev / page numbers / Next; the
  Combined view interleaves instruments in time order.
- **Acceptance criteria:**
  1. Given a 5-min strategy whose condition becomes true on the 09:20–09:25 candle and the 09:25 candle opens at 1002.50, then
     the EN row shows time 09:20, price 1002.50, Buy, +qty, and (Enhancement column) fill time 09:25.
  2. Given round trips with realised P&L −111, −56 and +87 in order (qty 2), then the P&L column on the exit rows reads −111, −167,
     −80 and the Trade P&L column reads −111, −56, +87; entry rows repeat the running total of the previous exit.
  3. Given a short round trip, then the entry row shows Sell with negative qty and the exit row shows Buy with positive qty.
  4. Given 230 rows and 50 per page, then 5 pages exist.
- **Data/Kite dependency:** none.
- **Deviation from Streak:** P&L column semantics match Streak's live app (cumulative running total, REVIEW-2 #17); Trade P&L,
  Fill Time and Reason columns are additions that remove the signal-time ambiguity.
- **Priority hint:** Core

### FR-BT-021 Download trade log
- **Source features:** BT-017
- **Requirement:** The system shall export the selected instrument's trade log to CSV (UTF-8, header row, ISO-8601 timestamps
  plus a DD-MM-YYYY display column, all columns of FR-BT-020, plus strategy name, run ID and gross/net P&L). **Enhancement
  (beyond Streak):** a combined export of all instruments in one CSV, and an export of the run assumptions as JSON.
- **Acceptance criteria:**
  1. Given a run with 12 trades on one instrument, then the CSV has 1 header + 24 data rows (entry + exit per trade) and re-
     importing it into a spreadsheet reproduces Total P&L.
  2. Given "Export all instruments", then one CSV contains rows of every instrument with a Symbol column.
- **Data/Kite dependency:** Local file save dialog (Wails runtime).
- **Deviation from Streak:** Combined export added (Streak does not support it).
- **Priority hint:** Important

### FR-BT-022 Cumulative P&L chart
- **Source features:** BT-018
- **Requirement:** The system shall plot cumulative P&L (gross or net per toggle) against date for the selected instrument and
  for Combined, with the line green above 0 and red below 0.
- **Acceptance criteria:**
  1. Given trades +500, −200, −300 on successive days, then the plotted points are 500, 300, 0 with the 0 point on the axis.
- **Data/Kite dependency:** none.
- **Deviation from Streak:** Combined curve added.
- **Priority hint:** Important

### FR-BT-023 Engine: evaluation on completed candles only
- **Source features:** BT-019
- **Requirement:** All strategy conditions (entry, exit-condition, MTF rows) shall be evaluated only at the close of a
  completed base-TF candle using completed-candle OHLCV values; never on ticks or partial candles. Backtest and live (DEP)
  shall use the same evaluation code.
- **Acceptance criteria:**
  1. Given a golden fixture (candles + strategy + expected signals), then the backtest engine and the live engine fed the same
     candles produce identical signal lists (NFR-06).
- **Data/Kite dependency:** Local candles.
- **Deviation from Streak:** none.
- **Priority hint:** Core

### FR-BT-024 Engine: entry requires a false→true transition; no pyramiding
- **Source features:** BT-019
- **Requirement:** An entry signal occurs at candle k only if the entry condition is true at k and false at k−1, where k−1 is
  the previous base-TF candle in the data (including warm-up candles and the previous session's last candle). At most one
  position per (strategy, instrument) shall exist; entry signals while a position is open are ignored (no averaging or
  pyramiding). After an exit, a new entry needs a fresh transition.
- **Acceptance criteria:**
  1. Given closes evaluate F, T, T, F, T, T on candles c1…c6 and the position from c2 (filled at c3 open) is exited by an EX at
     c4, then the entry signals are c2 and c5 only.
  2. Given the same sequence but the position from c2 stays open through c6, then only one EN exists.
  3. Given a condition already true on the last warm-up candle and still true on the first in-range candle, then no entry
     occurs on that first candle.
- **Data/Kite dependency:** none.
- **Deviation from Streak:** none.
- **Priority hint:** Core

### FR-BT-025 Engine: fills at next candle open
- **Source features:** BT-019
- **Requirement:** Entry (EN) and exit-condition (EX) signals detected at the close of candle k shall fill at the open of candle
  k+1 regardless of volume. If no candle k+1 exists in range (end of data) the signal is dropped and recorded. For MIS, a signal
  whose fill candle is at or after the square-off candle (FR-BT-031) is dropped for entries; for EX it is superseded by SO. For
  CNC/NRML, a signal on the session's last candle fills at the next session's first candle open (overnight gap included).
- **Acceptance criteria:**
  1. Given 5-min candles 09:20 (signal) and 09:25 (O = 1002.50, C = 1010), then the entry fill price is 1002.50, not 1010.
  2. Given a CNC signal on the 15:15 candle of 15-min data and next session open 1,020, then the fill is 1,020 at the next
     session's 09:15 candle.
  3. Given the final candle of the range produces a signal, then no fill occurs and diagnostics list "signal at end of data".
- **Data/Kite dependency:** market calendar (FR-DATA-001) for session boundaries.
- **Deviation from Streak:** none (FEATURES §6-1 chosen interpretation).
- **Priority hint:** Core

### FR-BT-026 Engine: exit-condition (EX) semantics
- **Source features:** BT-019, BT-030
- **Requirement:** While a position is open, the exit-condition block (if defined) shall be evaluated at each completed
  candle close starting with the fill candle's close; when true (level-triggered, no transition needed), the position exits at
  the next candle's open with trigger EX.
- **Acceptance criteria:**
  1. Given a position filled at the open of c3 and an exit condition true at c3's close, then the exit fills at c4's open with
     code EX.
  2. Given the exit condition is already true when the entry fills, then the exit still occurs (level-triggered) at the next open.
- **Data/Kite dependency:** none.
- **Deviation from Streak:** Level vs transition triggering for exits is unverified in Streak (OQ-27).
- **Priority hint:** Core

### FR-BT-027 Engine: SL / TP level computation
- **Source features:** BT-021
- **Requirement:** SL and TP levels shall be computed from the actual entry fill price P and quantity q in the unit selected in
  the strategy (applies to SL, TP and TSL together): Percentage — long SL = P × (1 − s/100), TP = P × (1 + t/100); short
  mirrored. Points — long SL = P − s, TP = P + t. P&L (₹) — long SL = P − s/q, TP = P + t/q. Levels are not rounded to tick size
  in backtests (recorded in assumptions). A zero/blank value disables that exit.
- **Acceptance criteria:**
  1. Given long P = 1000, q = 25, SL 2 %, TP 3 %, then SL = 980.00 and TP = 1030.00.
  2. Given the same with unit Points SL 20, TP 30, then SL = 980, TP = 1030.
  3. Given unit ₹ SL 500, TP 750, then SL = 1000 − 500/25 = 980, TP = 1030.
  4. Given short P = 500, SL 1 %, then SL = 505.00.
- **Data/Kite dependency:** none.
- **Deviation from Streak:** none.
- **Priority hint:** Core

### FR-BT-028 Engine: SL / TP evaluation (same-candle SL-first, evaluation mode)
- **Source features:** BT-021
- **Requirement:** The engine shall support two SL/TP evaluation modes, recorded in the run assumptions:
  (a) **Hybrid (default; FEATURES §6-15 chosen interpretation):** on the fill (entry) candle, SL and TP are checked against that
  candle's high/low; if both are touched, SL is assumed hit first; exit price = the level. On later candles, fixed SL/TP are
  checked on the candle close (long: C ≤ SL → SL; C ≥ TP → TP); exit price = that close. TSL is always path-based (FR-BT-029).
  (b) **Intrabar:** on every candle, the open is first checked for a gap beyond a level (exit at the open), then levels are
  checked along the path (long O→L→H→C, short O→H→L→C), SL before TP when both are touched; exit price = the level.
- **Acceptance criteria:**
  1. (Both modes) Given long fill at 1000 on candle k with SL 980, TP 1030 and k = {O 1000, H 1031, L 979, C 1005}, then exit SL
     at 980 on candle k.
  2. (Hybrid) Given candle k+1 = {O 1001, H 1004, L 975, C 990}, then no exit (close 990 > 980); given k+2 close 978, then SL exit
     at 978.
  3. (Intrabar) Given candle k+1 = {O 1001, H 1004, L 975, C 990}, then SL exit at 980; given k+1 opens at 975 (gap), then SL
     exit at 975.
  4. (Intrabar) Given a short at 500 with SL 505, TP 490 and candle {O 499, H 506, L 489, C 495}, then path O→H hits SL first → exit
     505.
- **Data/Kite dependency:** none.
- **Deviation from Streak:** Streak's own docs conflict (§6-15). Default reproduces the chosen interpretation; Intrabar mode
  is an **Enhancement (beyond Streak)** for users who want stricter realism (OQ-28).
- **Priority hint:** Core

### FR-BT-029 Engine: trailing stop loss along the intra-candle path
- **Source features:** BT-021
- **Requirement:** For a long, the trail reference H* starts at the entry fill price and is updated to each new high reached
  along the path O→L→H→C; trail stop = H* − d where d is the TSL distance (% of H*, points, or ₹/q). At each path point after the
  reference update, if price ≤ trail stop the position exits at the trail-stop price with code TSL (or at the open if the candle
  opens below it). Shorts mirror with trough L*, path O→H→L→C. If a fixed SL also exists the effective stop is the tighter of
  the two; the code is TSL when the trail stop is the binding level, else SL. No conditional or dynamic TSL modification exists.
- **Acceptance criteria:**
  1. Given long fill 100 with TSL 2 points and next candle {O 101, L 99.5, H 104, C 101.5}: O → H* 101, stop 99; L 99.5 > 99 no
     hit; H → H* 104, stop 102; C 101.5 ≤ 102 → exit TSL at 102.
  2. Given short fill 100 with TSL 2 points and next candle {O 99, H 101.5, L 96, C 98.5}: O → L* 99, stop 101; H 101.5 ≥ 101 →
     exit TSL at 101.
  3. Given long fill 100, SL 1 % (99) and TSL 2 points, and price rises to 103 then falls, then the exit is TSL at 101 (trail
     stop 101 > fixed 99).
- **Data/Kite dependency:** none.
- **Deviation from Streak:** none intended; exact Streak trail start reference unverified (OQ-29).
- **Priority hint:** Core

### FR-BT-030 Engine: intra-candle event precedence
- **Source features:** BT-019, BT-020, BT-021
- **Requirement:** For each candle the engine shall process, in order: (1) at open — MIS square-off if this is the square-off
  candle (FR-BT-031); otherwise pending EN/EX fills from the previous close; then futures expiry handling if applicable;
  (2) gap checks of SL/TP/TSL at the open (Intrabar mode, and TSL in all modes); (3) path checks SL/TSL before TP;
  (4) at close — Hybrid close-based SL/TP; then exit-condition evaluation (schedules EX); then entry evaluation (schedules EN,
  subject to one-position, cycle, window, square-off and capital rules). A position exited intra-candle may schedule a new entry
  at that candle's close if a fresh transition occurs.
- **Acceptance criteria:**
  1. Given a pending EX from the previous close and this candle is also the SO candle, then exactly one exit row exists, coded EX
     at this candle's open.
  2. Given an SL hit intrabar on candle k and a fresh entry transition at k's close, then a new EN fills at k+1's open (cycle
     permitting).
- **Data/Kite dependency:** none.
- **Deviation from Streak:** Ordering made explicit (Streak does not document it).
- **Priority hint:** Core

### FR-BT-031 Engine: MIS square-off per instrument class (backtest table)
- **Source features:** BT-020
- **Requirement:** Open MIS positions shall be squared off (code SO) at the open of the base-TF candle that contains the
  square-off time T (candle start ≤ T < candle end), with T by instrument class from the user-editable **backtest SO table**
  (FEATURES §6-2 chosen interpretation): F&O stocks under the closing call-auction list: 15:10; indices (signal-only, no fills):
  15:10; non-F&O equities: 15:25; futures: 15:25; fallback (class unknown): 15:15 — all expressed as offsets from the session end
  of the market calendar (FR-DATA-001), so a special session with end E uses `E − (15:30 − T)`. With a 1-hour base TF on an
  F&O stock this yields the 14:15 candle. New MIS entries whose fill candle starts at or after the SO candle are not taken.
  **MIS with a 1-day base TF is allowed:** the entry fills at the next session's open, SL/TP/TSL are evaluated on that session's
  1-minute candles in Intrabar manner (FR-BT-028 b) and the square-off fills at the open of the 1-minute candle containing T; the
  run requires 1-minute data for the range (backfilled per FR-BT-009) and records "MIS on daily base: intraday evaluation" in the
  assumptions. Instrument class is derived from the instruments dump (equity with an NFO futures underlying → F&O stock) and the
  user-importable call-auction list. The live square-off time is a separate table (FR-DEP-021).
- **Acceptance criteria:**
  1. Given a non-F&O stock on 5-min, then SO occurs at the 15:25 candle's open.
  2. Given an F&O stock in the call-auction list on 5-min, then SO occurs at the 15:10 candle's open.
  3. Given an F&O stock on 1-hour candles (09:15, 10:15 … 14:15, 15:15), then SO occurs at the 14:15 candle's open.
  4. Given an F&O stock on 15-min, then SO occurs at the 15:00 candle's open (contains 15:10).
  5. Given an MIS strategy on a 1-day base TF with a signal on day D, then the entry fills at D+1 09:15 open and, absent SL/TP, the
     SO fills at the open of the D+1 15:25 1-minute candle (non-F&O stock).
  6. Given a special session 18:15–19:15, then a non-F&O stock squares off at the candle containing 19:10.
- **Data/Kite dependency:** Instruments dump (`GetInstruments`, NFO underlyings); user-importable call-auction list (C-09);
  market calendar (FR-DATA-001); 1-minute candles for MIS-on-daily runs.
- **Deviation from Streak:** The "containing candle" rule is an interpretation that reproduces Streak's documented 14:15
  hourly case; Streak's exact rule for 15-min F&O stocks is unverified (OQ-30). MIS on a daily base is allowed (Streak lists both
  without exclusion; RV-028) using intraday data.
- **Priority hint:** Core

### FR-BT-032 Engine: futures contract expiry (EXP)
- **Source features:** BT-030, BT-026
- **Requirement:** A futures position open at the close of the contract's last trading day shall be closed at that candle's
  close with code EXP. Backtests on a specific futures contract shall run only within that contract's listed life; an optional
  "continuous futures" mode (day TF only, Kite `continuous=1`) shall roll by using the stitched series and shall record that
  rolls are not traded.
- **Acceptance criteria:**
  1. Given a NRML long in a futures contract expiring 27 Oct 2026, still open on that day, then an EXP exit at the 15:30 close of
     27 Oct exists.
  2. Given continuous mode with an intraday base TF, then the option is disabled with an explanation.
- **Data/Kite dependency:** `GetHistoricalData(..., continuous=true)` (day only); instruments dump expiry.
- **Deviation from Streak:** Continuous-futures mode is an Enhancement; intraday history of expired contracts is UNVERIFIED (07 §A4; OQ-40).
- **Priority hint:** Important

### FR-BT-033 Engine: open positions at end of range
- **Source features:** BT-015, BT-011
- **Requirement:** Positions still open at End Date shall not be force-closed; they are shown as open with unrealised P&L
  marked to the last close, excluded from closed-trade metrics (FR-BT-014) and included in the calendar/MTM (FR-BT-019).
- **Acceptance criteria:**
  1. Given a CNC long of 10 @ 100 open at End Date with last close 108, then the log shows only an EN row and the summary shows
     "Open: +₹80 unrealised".
- **Data/Kite dependency:** none.
- **Deviation from Streak:** Behaviour made explicit.
- **Priority hint:** Core

### FR-BT-034 Engine: multi-timeframe evaluation (single definition)
- **Source features:** BT-023, EQB-022
- **Requirement:** This FR is the only definition of MTF timing; FR-EQB-025 and the scanner engine cite it. (1) The strategy or
  scanner is evaluated at every base-TF candle close. (2) A condition row on timeframe T uses T's **last fully closed** candle at
  the evaluation instant (never a forming candle). (3) **Intraday rows only:** if an intraday row's TF (≤ 1h) is neither an
  integer multiple nor a divisor of the base TF, evaluation occurs only at instants that are closes of both — LCM(base, all such
  TFs) in minutes, anchored at the session start (09:15 on a regular day, FR-DATA-001). (4) **Daily, weekly and monthly rows
  never change the evaluation instants**; they are evaluated at every base instant using their last closed candle. (5) Intraday
  higher-TF candles are resampled from finer local candles anchored at the session start; day candles are Kite day candles;
  week candles (Mon–Fri) and month candles are resampled from day candles (FR-DATA-002) and are **complete only when the next
  week / month begins** (A06 §1). (6) Week and month may be used only as MTF rows, never as base TF. (7) Indicator warm-up on MTF
  rows uses the same unlimited local history (FR-BT-008).
- **Acceptance criteria:**
  1. Given base 5-min and a 15-min row, at the 10:00 evaluation (5-min candle 09:55–10:00), then the 15-min value used is the
     09:45–10:00 candle; at 09:55 it is the 09:30–09:45 candle.
  2. Given base 3-min and a 5-min row, then evaluations occur only at 09:30, 09:45, 10:00 … (every 15 min).
  3. Given base 1-hour and a 1-day row, then evaluations occur at every hourly close (no LCM) and the daily value at Wednesday 11:15
     is Tuesday's candle.
  4. Given a weekly row evaluated on Friday's last candle, then the previous week's candle is used; on the next Monday's first
     evaluation the just-finished week is used.
  5. Given a 1-hour row, then hourly candles are 09:15, 10:15, … 15:15 (last candle 15:15–15:30).
- **Data/Kite dependency:** candle store with resampling (FR-DATA-002); LCM scheduler on the app clock (FR-DATA-004); market
  calendar (FR-DATA-001).
- **Deviation from Streak:** None in rules (A06 §1 parity, including weekly completion); warm-up not limited by Streak's lookback
  (Enhancement via FR-BT-008). The scope of LCM (intraday only) resolves the three conflicting v1 texts (RV-008).
- **Priority hint:** Core

### FR-BT-035 Engine: no slippage by default
- **Source features:** BT-024
- **Requirement:** By default the engine shall apply zero slippage; fills occur exactly at the prices defined by FR-BT-025,
  FR-BT-028, FR-BT-029, FR-BT-031, FR-BT-032. The assumptions panel shall state "Slippage: none".
- **Acceptance criteria:**
  1. Given default settings, then every fill price equals the rule price exactly.
- **Data/Kite dependency:** none.
- **Deviation from Streak:** none.
- **Priority hint:** Core

### FR-BT-036 Configurable slippage — Enhancement (beyond Streak)
- **Source features:** BT-024
- **Requirement:** The user may set slippage per run as a percentage of price or a number of ticks, applied adversely to every
  fill (buys higher, sells lower), including SL/TP/TSL/SO/EXP exits; quantity derivation (FR-BT-006) uses the slipped entry price.
- **Acceptance criteria:**
  1. Given slippage 0.05 % and a long entry at 1000.00 and exit at 1030.00, then fills are 1000.50 and 1029.485 (displayed 1029.49).
  2. Given slippage 2 ticks with tick 0.05, then a buy at 1000.00 fills at 1000.10.
- **Data/Kite dependency:** Tick size from the instruments dump.
- **Deviation from Streak:** Streak has no slippage control.
- **Priority hint:** Important

### FR-BT-037 Engine: raw, unadjusted candles
- **Source features:** BT-025
- **Requirement:** The engine shall use candles exactly as stored (raw, unadjusted for splits/bonuses/dividends) by default.
  The system shall flag, per instrument in range, overnight gaps larger than a configurable threshold (default 20 %) as
  "possible corporate action" warnings in the results. **Enhancement (beyond Streak):** an optional "apply corporate-action
  adjustment" switch using a user-maintained adjustment table (date, ratio) (NFR-12); the choice is recorded in assumptions.
- **Acceptance criteria:**
  1. Given a 1:2 split (close 2000 → next open 1000) in range, then the default run uses raw prices and shows a warning on that
     date.
  2. Given adjustment enabled with that split in the table, then prices before the split are halved for the run only; stored
     candles are unchanged.
- **Data/Kite dependency:** Local candle store; Kite day-candle adjustment status UNVERIFIED (07 §E).
- **Deviation from Streak:** Warning and optional adjustment added.
- **Priority hint:** Important

### FR-BT-038 Supported segments and instruments
- **Source features:** BT-026
- **Requirement:** Backtests shall support NSE and BSE equities (cash) and NSE/BSE index and stock futures (NFO-FUT, BFO-FUT),
  long and short (intraday short-selling for MIS; CNC short is refused at run time). Indices in a strategy are evaluated for
  signals but never filled (signal-only instruments; their rows show "signal-only" instead of trades). Candlestick charts only.
  **Enhancement (beyond Streak):** any BSE stock with data, not only a BSE 1000 subset.
- **Acceptance criteria:**
  1. Given a Sell strategy with CNC, then the run is refused: "Short delivery positions are not allowed".
  2. Given a strategy containing NIFTY 50 and RELIANCE, then NIFTY 50's result shows signal timestamps and "signal-only (index)",
     while RELIANCE shows trades.
  3. Given a BSE stock outside the BSE 1000 with local data, then it backtests normally.
- **Data/Kite dependency:** `GetHistoricalData` for NSE/BSE/NFO/BFO via FR-DATA-003; instruments dump.
- **Deviation from Streak:** Streak supports equities and futures only — same here; full BSE is an enhancement. Options
  backtesting (a v1 enhancement idea) is out of scope (§10 BT-038-opt).
- **Priority hint:** Core

### FR-BT-039 Options, MCX, currency, AMO, MTF exclusions
- **Source features:** BT-026
- **Requirement:** Options (all NFO/BFO option contracts), MCX, currency and MTF-product backtests are out of scope (§1, §10); the instrument picker for backtests
  shall exclude those segments and say why.
- **Acceptance criteria:**
  1. Given an MCX instrument, then it cannot be added to a strategy that will be backtested and a tooltip explains "segment not supported".
  2. Given an option contract, then it is absent from every picker and search (NFR-15) and a pasted identifier is rejected with "options are not supported".
- **Data/Kite dependency:** Instruments dump segment field.
- **Deviation from Streak:** none for MCX/currency/AMO/MTF (Streak excludes them too); options excluded by scope decision (§1); Streak has no options backtest either.
- **Priority hint:** Nice-to-have

### FR-BT-040 Backtest error and empty-result messages
- **Source features:** BT-027
- **Requirement:** Per instrument the system shall show: (1) "No trades happened" when no entry signal occurred; (2) "No trades
  happened because capital is too low" when signals occurred but every entry was skipped by FR-BT-006/007; (3) "No data for
  range" when candles are missing and the user chose not to backfill; (4) engine errors with the failing condition named.
  An error on one instrument shall not abort others. There is no "limit exceeded" error.
- **Acceptance criteria:**
  1. Given C = 1000 and a stock at 2,600, then message (2) appears for that instrument and others run normally.
  2. Given an invalid indicator parameter (e.g. period 0), then the run is blocked before start with the row identified
     (Streak bug §3.2 #7 not replicated).
- **Data/Kite dependency:** none.
- **Deviation from Streak:** No daily cap; more specific messages.
- **Priority hint:** Core

### FR-BT-041 Current-day data in backtests
- **Source features:** BT-028
- **Requirement:** When End Date is today, the run shall include all of today's candles that are complete at run time
  (from the live candle builder or a fresh historical fetch), and mark the result "includes today up to HH:MM". There is no
  21:00 delay. Re-running later in the day updates the result.
- **Acceptance criteria:**
  1. Given a run at 11:07 on a 5-min base TF, then the last evaluated candle is 11:00–11:05 and the label says "up to 11:05".
- **Data/Kite dependency:** Ticker candle builder or `GetHistoricalData` for today.
- **Deviation from Streak:** Streak refreshes current-day results only after 21:00.
- **Priority hint:** Nice-to-have

### FR-BT-042 Backtest results view onboarding tour
- **Source features:** BT-029
- **Requirement:** The two-step tour of the backtest results view is defined in FR-SHELL-007 (tours list); this FR only records
  that the results view participates in the tour framework and exposes "Replay tour" in its help menu.
- **Acceptance criteria:**
  1. Given FR-SHELL-007 AC5, then it passes on the backtest results view.
- **Data/Kite dependency:** Local settings.
- **Deviation from Streak:** Tour copy written fresh (clean-room); single definition in FR-SHELL-007 (RV-049).
- **Priority hint:** Nice-to-have

### FR-BT-043 Trigger codes
- **Source features:** BT-030
- **Requirement:** Every exit row and marker shall carry exactly one trigger code: EN entry; EX exit condition; SL stop loss;
  TP target; TSL trailing stop loss; SO intraday square-off; EXP contract expiry. A legend explaining codes shall be available
  next to the trade log.
- **Acceptance criteria:**
  1. Given a fixture exercising each exit type, then the log contains each of EN, EX, SL, TP, TSL, SO, EXP at the expected rows.
- **Data/Kite dependency:** none.
- **Deviation from Streak:** none.
- **Priority hint:** Core

### FR-BT-044 Run assumptions and reproducibility record
- **Source features:** BT-008, BT-024, BT-025
- **Requirement:** Every run shall persist: strategy version snapshot, parameters, per-instrument effective data range, data
  checksum of candles used (FR-DATA-002 monthly checksums), fee-table version, slippage, SL/TP evaluation mode, square-off table version, market-calendar version (FR-DATA-001), adjustment setting,
  app version and engine version. Re-running a stored run with the same inputs shall reproduce identical results.
- **Acceptance criteria:**
  1. Given a stored run, when "Re-run with same inputs" is chosen and data are unchanged, then all trades and metrics match exactly.
  2. Given candles changed since (e.g. re-backfilled), then the system warns that the data checksum differs.
- **Data/Kite dependency:** SQLite results store.
- **Deviation from Streak:** **Enhancement (beyond Streak)** (NFR-12).
- **Priority hint:** Important

### FR-BT-045 Backtest run history
- **Source features:** BT-007, BT-008
- **Requirement:** The system shall keep all runs of a strategy, list them (date, parameters summary, total P&L) and let the user
  open or delete any of them; the results view opens the latest by default.
- **Acceptance criteria:**
  1. Given three runs with different capitals, then all three are listed and each opens its own results.
- **Data/Kite dependency:** SQLite.
- **Deviation from Streak:** **Enhancement (beyond Streak)**: Streak shows only the latest result.
- **Priority hint:** Nice-to-have

### FR-BT-046 "Capital used" metric and return denominators
- **Source features:** BT-011, HOME-005
- **Requirement:** **Capital used** per instrument = max over that instrument's trades of (P_fill × qty) — the largest notional
  ever deployed in the run; **Combined** Capital used = Σ over instruments. It is shown in the metrics row (FR-BT-014), the
  Discover summary (FR-DISC-007) and the Live Backtest widget (FR-HOME-006). **Return denominators everywhere** (weekday summary
  FR-BT-015, monthly grid FR-BT-016, calendar heatmap FR-BT-019, widget heatmap FR-HOME-007): day/month return % = P&L ÷ C per
  instrument, or ÷ (C × n) for Combined (n = instruments in the run); Capital used is displayed, never used as a denominator.
- **Acceptance criteria:**
  1. Given trades of notional 23,866.20 and 23,400.00 on one instrument, then Capital used = ₹23,866.20.
  2. Given two instruments with Capital used 23,866.20 and 8,835.50, then Combined Capital used = ₹32,701.70.
  3. Given C = 25,000, n = 2 and a Combined day P&L of −₹370, then the day return is −0.74 %.
- **Data/Kite dependency:** none.
- **Deviation from Streak:** Streak shows "Capital Used" on the Live Backtest card without a definition (Appendix M, INFERRED); ours
  is explicit (RV-011).
- **Priority hint:** Important

---

## FR module DEP — Deployment (notify & confirm, signal-only, paper, armed auto)

### FR-DEP-001 Deploy dialog (execution panel step 2, own strategy)
- **Source features:** DEP-001, EQB-039
- **Requirement:** As step 2 of the execution panel (FR-EQB-032) when Deploy is chosen, and from My Strategies and the results
  view "Deploy" (which open the panel on this step), the system shall show: instrument list with master checkbox and count
  ("n / total"), per-instrument checkbox and Qty (default 1; lots for futures; indices listed as "signal-only", unticked and
  not deployable); Capital (required; used for the capital check in FR-DEP-019 and for P&L % display); Strategy Cycle {–, 1…5}
  (**default 1**, disabled for CNC); Order Type (Market default; Limit as **Enhancement (beyond Streak)** with offset from LTP);
  Holding Type MIS / CNC (NRML for futures); Execution Mode (FR-DEP-003); "Re-arm daily" (MIS only, default on, FR-DEP-012);
  "Protect exits with GTT" (FR-DEP-009, default per Settings); Validity end date (default today + 29 calendar days); and a
  mandatory acknowledgement "I accept the trading terms" whose link opens the in-app Terms of use (FR-ACCT-015). Footer **Back**
  / **Deploy**; Deploy is disabled until the acknowledgement is ticked, all fields are valid and the session's risk disclosure is
  acknowledged (FR-SHELL-020). Each selected instrument becomes one deployment-instrument (DI) under one deployment.
- **Acceptance criteria:**
  1. Given 15 instruments selected of 50, then the header shows "15 / 50" and Deploy creates 1 deployment with 15 DIs in state
     ARMED (FR-DEP-004).
  2. Given the acknowledgement is unticked, then Deploy is disabled; given "trading terms" is activated, then FR-ACCT-015 opens.
  3. Given Qty 0 or a non-lot-multiple for a futures instrument, then Deploy is disabled and the row is highlighted.
  4. Given today = 11 Oct 2026, then default validity = 09 Nov 2026 (T+29) and Strategy Cycle shows "1".
  5. Given Escape, then the panel closes and nothing is deployed; given Back, then step 1 shows with Deploy still selected.
  6. Given NIFTY 50 is among the strategy's instruments, then its row reads "signal-only" and cannot be ticked.
- **Data/Kite dependency:** Local deployment store; instruments dump (lot size).
- **Deviation from Streak:** Footer Back / Deploy and cycle default 1 match Streak (REVIEW-2 #15); mode selector, re-arm, GTT,
  validity date, Limit option and Escape-to-close are additions; Streak's Market-only order type is the default.
- **Priority hint:** Core

### FR-DEP-002 Deploy from a prebuilt strategy (copy wizard)
- **Source features:** DEP-002
- **Requirement:** Deploying a prebuilt (read-only) strategy shall open a two-step wizard: step 1 copies it to the user's
  strategies (Name required, auto-focused; Category pre-selected from the source; Tags pre-filled, max 5 shown as chips;
  Description pre-filled); step 2 is the deploy dialog (FR-DEP-001) for the copy. Name validation shall be an inline visible
  message (not tooltip-only). The wizard shall have Back, Cancel and close (×), and close on Escape.
- **Acceptance criteria:**
  1. Given an empty name on Next, then an inline error "Enter a name" appears below the field and step 2 is not shown.
  2. Given a valid name, then Next saves the copy (visible in My Strategies) and shows step 2.
  3. Given Cancel on step 2, then the copy remains saved and nothing is deployed.
- **Data/Kite dependency:** Local store.
- **Deviation from Streak:** Cancel/×/Escape and inline validation added (Streak bug §3.2 #16 not replicated).
- **Priority hint:** Important

### FR-DEP-003 Execution modes
- **Source features:** DEP-011, DEP-001
- **Requirement:** Each deployment shall run in exactly one mode:
  (1) **Notify & confirm (default; parity with Streak's notification-based trading):** signals raise alerts; an order is placed
  only when the user confirms it in the order ticket (review mode, FR-DEP-007).
  (2) **Signal-only:** alerts only, no order capability; tracks a notional position (as if every entry were taken) so exit alerts
  are meaningful; available without static IP or trading enabled.
  (3) **Paper — Enhancement (beyond Streak):** simulated orders and positions (FR-DEP-017); available without trading enabled.
  (4) **Armed auto — Enhancement (beyond Streak):** orders placed automatically through the order safety pipeline, only under
  FR-DEP-018 gating, and only while the deployment is armed in the current session.
  Mode changes on a live deployment are allowed only when all its DIs are flat. The mode is shown on every DI row and in every
  alert.
- **Acceptance criteria:**
  1. Given trading is not enabled (FR-ORD-013), then modes 1 and 4 are disabled in the dialog with the reason shown; modes 2 and
     3 remain available.
  2. Given a DI is IN_POSITION, then the mode selector is read-only for that deployment.
  3. Given mode 4 and the session expires at 06:00, then the deployment behaves as mode 1 until re-armed (FR-DEP-018).
- **Data/Kite dependency:** none (modes 1/4 depend on the ORD pipeline).
- **Deviation from Streak:** Streak offers notification-based trading only; signal-only, paper and armed-auto are additions
  (paper and auto are explicitly absent in Streak, DEP-011).
- **Priority hint:** Core (1, 2) · Important (3) · Nice-to-have (4)

### FR-DEP-004 Deployment state machine
- **Source features:** DEP-003, DEP-004, DEP-005, DEP-006, DEP-007
- **Requirement:** Each deployment-instrument (DI) shall follow this state machine; the deployment's status is derived
  (ACTIVE if any DI non-terminal, else STOPPED/EXPIRED/COMPLETED). All times are market-calendar phases (FR-DATA-001) on the app
  clock (FR-DATA-004); "15:20", "15:21" and "SO time" denote the regular-day values (FR-DEP-012, FR-DEP-021).

  | State | Meaning |
  |---|---|
  | ARMED | Flat, evaluating entry conditions |
  | ENTRY_PENDING | Entry signal raised, awaiting user confirmation (mode 1) |
  | ENTRY_ORDER_OPEN | Entry order submitted, awaiting fill/rejection |
  | IN_POSITION | Position open (real, paper or notional); exits monitored |
  | EXIT_PENDING | Exit signal raised, awaiting confirmation (mode 1) |
  | EXIT_ORDER_OPEN | Exit order submitted, awaiting fill |
  | CYCLE_DONE | Flat; MIS only — the Strategy Cycle count for the day is exhausted (CNC has no cycle, FR-BT-002) |
  | DAY_DONE | MIS only — after the daily expiry (15:21) for the day; re-arms on the next trading day while validity lasts |
  | SUSPENDED | Overlay: app offline, session expired, feed down, clock out of sync or kill switch; stores `resume_state` |
  | ATTENTION | Needs user decision: rejection, partial fill, position mismatch, unknown order outcome, GTT delete failure |
  | STOPPED / EXPIRED | Terminal (user stop / validity reached, or MIS daily expiry with "re-arm daily" off) |

  | From | Event / guard | To | Action |
  |---|---|---|---|
  | ARMED | entry transition at candle close, within cycle/time rules, mode 1 | ENTRY_PENDING | Alert (FR-DEP-006) |
  | ARMED | same, mode 4 armed | ENTRY_ORDER_OPEN | Submit via ORD pipeline |
  | ARMED | same, mode 2 / 3 | IN_POSITION | Notional / simulated fill (FR-DEP-017) |
  | ENTRY_PENDING | user confirms, pipeline accepts | ENTRY_ORDER_OPEN | Order submitted |
  | ENTRY_PENDING | user ignores, or signal expires (FR-DEP-006) | ARMED | Notification status Ignored/Expired; cycle not consumed |
  | ENTRY_ORDER_OPEN | fully filled | IN_POSITION | Compute SL/TP/TSL from avg fill price; optional GTT (FR-DEP-009); cycle +1 |
  | ENTRY_ORDER_OPEN | rejected / cancelled with 0 fill | ARMED | Alert with reason |
  | ENTRY_ORDER_OPEN | partially filled then closed | ATTENTION | Track filled qty; user chooses keep/exit |
  | ENTRY_ORDER_OPEN | submit outcome unknown (timeout) | ATTENTION | Resolve by order lookup by tag (FR-ORD-008) |
  | IN_POSITION | EX/SL/TP/TSL trigger, or live SO time (FR-DEP-021) after reconciliation shows the position open, mode 1 | EXIT_PENDING | Exit alert |
  | IN_POSITION | same, mode 4 / 3 / 2 | EXIT_ORDER_OPEN / ARMED or CYCLE_DONE | Auto / simulated / notional exit |
  | IN_POSITION | position found closed at broker (reconcile, incl. before SO) | ARMED or CYCLE_DONE (MIS) | Event "closed externally"; cancel GTT |
  | IN_POSITION | GTT triggered and filled | ARMED or CYCLE_DONE (MIS) | Record exit from GTT |
  | EXIT_PENDING | user confirms | EXIT_ORDER_OPEN | Order submitted |
  | EXIT_PENDING | user dismisses | IN_POSITION | Alert stays visible as "exit not taken"; monitoring continues |
  | EXIT_ORDER_OPEN | filled | ARMED, or CYCLE_DONE if MIS cycles exhausted | Cancel GTT; P&L recorded |
  | EXIT_ORDER_OPEN | rejected | ATTENTION | Alert with reason |
  | any non-terminal (MIS) | 15:20 | (no new entries) | Entry evaluation disabled |
  | ARMED / CYCLE_DONE / ENTRY_PENDING (MIS) | 15:21 | DAY_DONE (re-arm on, default) or EXPIRED (re-arm off) | Pending entry alerts expire |
  | DAY_DONE / CYCLE_DONE (MIS) | next trading day's first candle, validity not reached | ARMED | Cycle counter reset |
  | any non-terminal | validity date passed and flat | EXPIRED | — |
  | IN_POSITION | validity date passed | IN_POSITION (no new entries) | Exit alerts continue until flat, then EXPIRED |
  | any non-terminal | app offline / session expired / feed down / clock skew > 5 s / kill switch | SUSPENDED (resume_state saved) | — |
  | SUSPENDED | session valid + feed up + clock ok + not killed | resume_state, after reconciliation (FR-DEP-010) | — |
  | ATTENTION | user resolves | ARMED / IN_POSITION / STOPPED | Per choice |
  | any | user Stop | STOPPED | Optional exit of open position (FR-DEP-013) |

- **Acceptance criteria:**
  1. Given a state-transition test table covering every row above, then each event moves the DI to the listed state and performs
     the listed action, and no other transition is possible (illegal events are logged and ignored).
  2. Given mode 1 and an entry alert ignored, then the DI returns to ARMED and the cycle counter is unchanged.
  3. Given MIS with cycle 1 and a completed round trip, then the DI is CYCLE_DONE until the next trading day; given CNC, then the DI
     never enters CYCLE_DONE.
  4. Given MIS with re-arm daily on (default) at 15:21, then the DI is DAY_DONE and is ARMED again at the next trading day's first
     candle; with re-arm off it is EXPIRED.
- **Data/Kite dependency:** SQLite deployment store; ORD pipeline; ticker; FR-DATA-001/004.
- **Deviation from Streak:** States are explicit and visible; SUSPENDED exists because the app may be closed (C-03). CYCLE_DONE is
  MIS-only (RV-037); DAY_DONE is the parity state for MIS daily expiry within the T+29 validity (RV-009).
- **Priority hint:** Core

### FR-DEP-005 Persistence and restart
- **Source features:** DEP-004, DEP-003
- **Requirement:** Every DI field needed to resume (state, resume_state, cycle count and day, position qty and average price,
  SL/TP levels, trail reference, last evaluated candle time, pending signal, linked order IDs and tags, GTT IDs, mode, re-arm flag, armed state and risk limits (FR-DEP-018), validity)
  shall be persisted transactionally in SQLite **before** any external side effect (write-ahead intent), and each transition shall
  append an event to the deployment log. After a crash or restart, deployments resume from persisted state via reconciliation
  (FR-DEP-010); no order is re-sent without first resolving the previous attempt.
- **Acceptance criteria:**
  1. Given the process is killed immediately after an order intent is journaled but before the HTTP call returns, when the app
     restarts, then the DI is ATTENTION or resolved by looking up the order by its tag, and no duplicate order is placed.
  2. Given 20 active DIs and an app restart, then all 20 reappear in their last states within 5 s of login.
- **Data/Kite dependency:** SQLite; `GetOrders` for tag lookup.
- **Deviation from Streak:** Streak is server-side; local persistence is required by C-03 and NFR-09.
- **Priority hint:** Core

### FR-DEP-006 Signal generation and alert delivery
- **Source features:** DEP-005
- **Requirement:** For active deployments the system shall build base-TF candles from ticks (subscribed with priority
  "deployment", C-07), finalise each candle at its boundary (session-aligned per FR-DATA-001, on the app clock FR-DATA-004) plus
  a 1 s grace for late ticks, evaluate with the shared engine (FR-BT-023/024/034), and raise any signal within 2 s of candle close
  (NFR-08). An alert shall be delivered to the in-app Notifications inbox (FR-DEP-015), as an OS desktop notification when enabled
  (FR-ACCT-003), to Telegram when enabled (FR-ACCT-014) and as an in-app toast with "Review order" and "Ignore" actions. An entry
  alert expires at the next base-TF candle close (configurable) and then becomes "Expired". **Signals after the continuous
  session:** a signal raised on the session's last candle (15:30 close) stays actionable during the post-closing session
  (15:40–16:00) for **equity cash** instruments as a MARKET order at the closing price (FR-ORD-015), with the ticket banner
  "Post-closing session — market order at closing price"; for futures, and after 16:00 for all instruments, the alert is
  informational and its action is disabled with "Next session 09:00 <next trading day>"; AMO is offered only if the FR-ORD-004
  enhancement is enabled.
- **Acceptance criteria:**
  1. Given a 5-min crossover on the 09:20–09:25 candle, then the alert appears by 09:25:02 (measured from candle end on the app
     clock).
  2. Given the alert is not acted on by 09:30 close, then its status is Expired and the DI is ARMED.
  3. Given the same fixture candles, then live signals equal backtest signals.
  4. Given the app is minimised and desktop notifications are on, then an OS notification is shown; clicking it focuses the app on
     the order ticket in review mode.
  5. Given a CNC equity entry signal on the 15:15–15:30 candle, then at 15:45 "Review order" is enabled and the ticket shows the
     post-closing banner; given the same for a futures contract, then the action is disabled with the next-session text.
- **Data/Kite dependency:** Ticker (`ticker.Subscribe`, `SetMode` quote), local candle builder (FR-DATA-002), warm-up via
  FR-DATA-003, market calendar (FR-DATA-001).
- **Deviation from Streak:** Latency target < 2 s (Streak 8–14 s); toast added; channels are local plus optional Telegram (no
  web/mobile push). Post-closing placement matches Streak (A06 §3 Late triggers; RV-006).
- **Priority hint:** Core

### FR-DEP-007 Notify & confirm order flow
- **Source features:** DEP-005, DEP-009
- **Requirement:** Acting on an alert shall open the order ticket (FR-ORD-004) in **review mode** pre-filled with instrument, side, qty, product, order type
  (Market with market protection), the signal price (signal candle close), current LTP, estimated margin and charges, and a
  short condition summary. Confirming sends it through the order safety pipeline (FR-ORD-005); the result updates the
  notification status (Placed / Rejected / Failed) and the DI state.
- **Acceptance criteria:**
  1. Given an entry alert for 5 × RELIANCE MIS Buy, then the ticket shows those values and "Place order" submits exactly one order tagged
     with the deployment's tag.
  2. Given margin is insufficient, then the pipeline blocks placement and the notification shows "Blocked: insufficient margin".
- **Data/Kite dependency:** `GetOrderMargins`, `GetOrderCharges`, `PlaceOrder`.
- **Deviation from Streak:** none in flow; preview of margin/charges added.
- **Priority hint:** Core

### FR-DEP-008 Live SL / TP / TSL monitoring
- **Source features:** DEP-006
- **Requirement:** For IN_POSITION DIs the system shall compute SL/TP/TSL levels from the actual average fill price and filled
  quantity (FR-BT-027 formulas), monitor them on every tick (LTP), and raise an exit trigger (SL/TP/TSL) the first time a level
  is crossed; exit-condition (EX) triggers are evaluated on candle close; SO triggers at the instrument class's **live
  square-off time** (FR-DEP-021), after reconciling the broker position (FR-DEP-010). Partially filled entries are monitored on
  the filled quantity (**Enhancement (beyond Streak)**; Streak does not monitor partial fills).
- **Acceptance criteria:**
  1. Given a long filled at avg 1000.40 with SL 2 %, then SL level = 980.39 and an exit alert fires on the first tick ≤ 980.39.
  2. Given a partial fill of 3 of 5, then levels use qty 3 for ₹-based exits and the DI shows "partial (3/5)".
  3. Given a live MIS long on a non-F&O stock and the live table at 15:10, then the SO alert fires at 15:10:00 ± 1 s on the app
     clock, not at the backtest table's 15:25.
- **Data/Kite dependency:** Ticker LTP; `GetPositions` for average price reconciliation; FR-DEP-021 table.
- **Deviation from Streak:** Partial fills monitored. Combined SL/TP for multi-leg option strategies is out of scope (§10
  DEP-008-opt). Live SO is a separate, earlier table (RV-017).
- **Priority hint:** Core

### FR-DEP-009 Kite GTT mirroring of SL/TP — Enhancement (beyond Streak)
- **Source features:** DEP-006, DEP-004
- **Requirement:** Per deployment (opt-in, requires trading enabled), on entering IN_POSITION the system shall place a Kite GTT:
  two-leg OCO with SL and TP when both exist, single-leg otherwise, with LIMIT orders priced at the trigger ± a configurable
  buffer (default 0.5 %) in the exit direction, product and qty matching the position. When the TSL moves, the GTT SL leg is
  modified no more often than every 60 s and only if the move ≥ 1 tick-buffer; when the position is exited by the app, the GTT is
  deleted; when the GTT triggers, the DI records the exit from order updates. GTT IDs are persisted. The UI shall show per DI
  whether exits are "protected while app is closed" (Live Strategies column and the quit dialog of FR-DEP-020).
- **Acceptance criteria:**
  1. Given a long 10 @ 1000 with SL 980 and TP 1030, then one OCO GTT exists with triggers 980/1030 and sell LIMIT prices
     975.10/1024.85 (0.5 % buffer below each trigger, rounded to tick 0.05).
  2. Given the app exits the position, then the GTT is deleted within 5 s; failure to delete raises ATTENTION.
  3. Given the app is closed and the GTT triggers, then on next start the DI shows the GTT exit and is ARMED/CYCLE_DONE.
  4. Given an MIS position, then the GTT is deleted at the DI's live square-off time (FR-DEP-021).
- **Data/Kite dependency:** `PlaceGTT`, `ModifyGTT`, `DeleteGTT`, `GetGTTs`; static IP requirement for GTT placement UNVERIFIED (OQ-33); GTT modifications limited to one per 60 s per DI.
- **Deviation from Streak:** Streak has no GTT usage; GTT LIMIT orders may not fill on gaps (documented in UI).
- **Priority hint:** Important

### FR-DEP-010 Startup / reconnect reconciliation and missed signals
- **Source features:** DEP-005, DEP-003, DEP-004
- **Requirement:** On app start, after daily login, and after a feed outage > 1 candle, for each non-terminal DI the system shall:
  (1) apply time rules (validity, MIS daily expiry per FR-DEP-012, on the market calendar FR-DATA-001) first; (2) fetch candles from `last_evaluated_candle` to the latest closed candle via
  historical API and replay the engine over the gap; (3) record entry signals in the gap as "Missed" notifications that are
  informational only and never auto-executed; (4) if IN_POSITION and an exit trigger occurred in the gap, raise a high-priority
  "Missed exit" alert (auto-executed only in armed mode with the separate opt-in "auto-execute missed exits", default off);
  (5) reconcile positions (`GetPositions`), orders (`GetOrders` by tag) and GTTs (`GetGTTs`) and resolve mismatches to ATTENTION
  with a description (a position already flat at the broker becomes "closed externally", also before any live SO, FR-DEP-021); (6) then leave SUSPENDED to the resume state.
- **Acceptance criteria:**
  1. Given the app was closed 10:00–11:00 on a 5-min deployment and an entry transition occurred at 10:25, then a "Missed entry
     10:25" notification exists and no order was placed.
  2. Given an open long whose SL level was crossed at 10:40 while closed, then a "Missed exit (SL)" alert is shown on start.
  3. Given the broker shows the position closed (net qty 0), then the DI moves to ARMED with event "closed externally".
  4. Given an MIS DI and the app opened at 16:00, then it is DAY_DONE/EXPIRED and no missed entries are actionable.
- **Data/Kite dependency:** `GetHistoricalData`, `GetPositions`, `GetOrders`, `GetGTTs`.
- **Deviation from Streak:** Needed because the local app is not always running (C-03).
- **Priority hint:** Core

### FR-DEP-011 Live Strategies view
- **Source features:** DEP-003
- **Requirement:** The system shall provide a Live Strategies view with mode tabs (All, Notify & confirm, Signal-only, Paper,
  Armed auto), a table of deployments (parent rows) with expandable instruments (child rows) and columns: Strategy / Instrument,
  Avg Price, LTP, Day Change, TP, SL (absolute levels), P&L, Net Change, Status / Cycle (e.g. "In position · 1/3"), GTT-protected
  indicator, mode badge (Paper / ARMED), Log; row actions (FR-DEP-013); search and status filter; an empty state with a call to action to the strategy
  library. Positions closed outside the app shall show "closed externally" rather than stale P&L.
- **Acceptance criteria:**
  1. Given a deployment of 3 instruments, then 1 parent and 3 children are shown, and parent P&L = Σ children.
  2. Given no deployments, then the empty state with a "Browse strategies" action is shown.
  3. Given a position closed in Kite web, then after reconciliation the child row shows "closed externally" and P&L stops updating.
- **Data/Kite dependency:** Ticker LTP; `GetPositions`; local store.
- **Deviation from Streak:** Extra mode tabs and GTT column; stale-P&L quirk not replicated.
- **Priority hint:** Core

### FR-DEP-012 Lifecycle rules and capacity
- **Source features:** DEP-004
- **Requirement:** The system shall enforce, on the market calendar (FR-DATA-001) and app clock (FR-DATA-004): MIS — no new
  entry signals after 15:20 (regular day; `session end − 10 min` generally); MIS daily expiry at 15:21 (`session end − 9 min`):
  DI → DAY_DONE when "Re-arm daily" is on (**default on** — parity: Streak's MIS deployments expire daily yet remain valid for
  T+29 days, A06 §3; RV-009), else EXPIRED; validity end date (default T+29, editable, no maximum); no artificial cap on
  concurrent deployments (NFR-13) — the only cap is live-data capacity: a deployment that would push total subscriptions above
  9,000 instruments (3 × 3,000) is refused with the reason and the usage breakdown; order quantity above the exchange freeze limit
  is blocked or autosliced per FR-ORD-005. Single-instance behaviour is defined in FR-ACCT-004 (a second launch focuses the
  existing window).
- **Acceptance criteria:**
  1. Given 150 DIs, then all are accepted (no 100 cap) if subscriptions ≤ 9,000.
  2. Given an MIS entry transition at the 15:20 close, then no alert is raised.
  3. Given an MIS deployment with default settings at 15:21 on day D, then its DIs are DAY_DONE and are ARMED again at D+1's first
     candle; the deployment is still listed as ACTIVE.
  4. Given a special session ending 19:15, then the MIS cut-offs are 19:05 and 19:06.
- **Data/Kite dependency:** Ticker capacity (07 §A6); local scheduler on FR-DATA-001/004.
- **Deviation from Streak:** No 100-deployment cap; editable validity; "re-arm daily" exposed as a switch (parity default on).
- **Priority hint:** Core

### FR-DEP-013 Stop, re-execute, edit, delete
- **Source features:** DEP-007
- **Requirement:** Row actions shall include: Stop (deployment or single DI; if IN_POSITION, ask "Stop and exit position" /
  "Stop and keep position (untracked)"); Re-execute (create a new deployment with the same parameters and a fresh validity from a
  STOPPED/EXPIRED one); Edit quantity/validity/re-arm/risk limits (only when flat); View strategy; View log; Delete (only terminal; logs retained
  in the audit store).
- **Acceptance criteria:**
  1. Given Stop and exit on an IN_POSITION DI in mode 1, then the order ticket opens in review mode for the exit; on confirmation the DI ends STOPPED.
  2. Given Re-execute on an EXPIRED deployment, then a new deployment with new validity is ARMED and the old one stays EXPIRED.
- **Data/Kite dependency:** ORD pipeline.
- **Deviation from Streak:** Menu contents defined (Streak's unobserved).
- **Priority hint:** Core

### FR-DEP-014 Deployment log
- **Source features:** DEP-008
- **Requirement:** Each deployment and DI shall have an append-only event log viewable in a side panel: time, event type
  (signal, alert, user action, state transition, order intent/submit/update/fill/reject, GTT action, reconciliation result,
  suspension/resume), details and linked order ID. Exportable to CSV.
- **Acceptance criteria:**
  1. Given a full round trip in mode 1, then the log shows in order: entry signal, alert, confirm, order submitted, filled,
     IN_POSITION, exit trigger, confirm, submitted, filled, ARMED.
- **Data/Kite dependency:** SQLite; order updates via ticker `OnOrderUpdate`.
- **Deviation from Streak:** none (Streak's log contents unobserved).
- **Priority hint:** Important

### FR-DEP-015 Notifications inbox
- **Source features:** DEP-009
- **Requirement:** The system shall provide a notifications view with tabs All / Strategy / Scanner, search (instrument, strategy
  or scanner name), columns Type (Buy/Sell/Scanner), Instrument(s), Strategy/Scanner, Time, Status (Pending, Placed, Rejected,
  Ignored, Expired, Missed, Info) and actions (Review order for Pending, View log). Unread notifications are counted on the top-bar bell (FR-SHELL-003); the view has no sidebar entry.
- **Acceptance criteria:**
  1. Given 3 strategy alerts and 2 scanner alerts, then All shows 5, Strategy 3, Scanner 2.
  2. Given a Pending entry alert, then "Review order" opens FR-DEP-007; after expiry the action is disabled and status Expired.
- **Data/Kite dependency:** Local store.
- **Deviation from Streak:** Unread count on the bell added (absent in Streak, FEATURES §3.3); Missed and Info statuses added.
- **Priority hint:** Core

### FR-DEP-016 Desktop notification channel (reference)
- **Source features:** DEP-010
- **Requirement:** Deployment alerts use the OS notification channel exactly as defined in FR-ACCT-003 (toggle, per-type filter,
  sound, click behaviour); this FR adds nothing.
- **Acceptance criteria:**
  1. Given FR-ACCT-003 AC1–AC5, then they hold for deployment alerts.
- **Data/Kite dependency:** see FR-ACCT-003.
- **Deviation from Streak:** see FR-ACCT-003 (single definition, RV-010).
- **Priority hint:** Core

### FR-DEP-017 Paper trading mode — Enhancement (beyond Streak)
- **Source features:** DEP-011
- **Requirement:** Paper deployments shall use the same engine, state machine, alert flow (mode-3 variant: entries fill
  automatically, no confirmation) and order-pipeline validations (stages 2–3 and 7 of FR-ORD-005; no gate on trading enabled,
  no margin call), with a **simulated executor**: MARKET fills at the first LTP received after the trigger (plus configured
  slippage, default 0.05 %), LIMIT fills when LTP crosses the limit price, SL/SL-M fills at the trigger when LTP crosses it;
  fills are for the full quantity (no partial fills); charges from the fee table (FR-BT-013); a separate **paper positions and
  paper orders book** (own tabs in Live Strategies, Positions and Order Book, FR-ORD-001/002) with its own P&L totals; MIS paper
  positions are squared off at the live SO time (FR-DEP-021); GTT mirroring and auto-execution are not applicable; no Kite order
  endpoint is called. Paper results are labelled "Paper" everywhere (badge on rows, alerts, notifications, Telegram messages) and
  never mixed into real P&L. A paper deployment can be converted to Notify & confirm or Armed auto only when flat (FR-DEP-003).
- **Acceptance criteria:**
  1. Given a paper entry and next LTP 1000.20 with slippage 0.05 %, then the paper fill is 1000.70 and no `PlaceOrder` call occurs
     (mock client).
  2. Given trading is disabled, then paper mode still works end to end (entry, SL exit, SO).
  3. Given a paper LIMIT buy at 995 and LTP sequence 996, 995.5, 994.8, then the fill occurs at 995 on the third tick.
  4. Given a paper position at the live SO time, then a simulated SO exit at the current LTP is recorded with code SO.
  5. Given paper and real positions on the same instrument, then the Positions view shows them under separate tabs and the real
     Total P&L excludes paper.
- **Data/Kite dependency:** Ticker; local store.
- **Deviation from Streak:** Streak has no paper trading (DEP-011).
- **Priority hint:** Important

### FR-DEP-018 Armed auto-execution — Enhancement (beyond Streak)
- **Source features:** DEP-011
- **Requirement:** Armed auto mode shall be available only when **all** hold: (a) trading enabled with static IP confirmed
  (FR-ORD-013; C-04); (b) the user turned on "Allow automatic execution" in Settings → Trading with a typed confirmation
  ("ARM"); (c) the deployment has **mandatory per-deployment risk limits** — max orders/day, max loss/day (₹), max qty per order,
  max open positions — all > 0 and within the global limits of FR-ORD-005; (d) the deployment was **armed in the current
  session** by pressing "Arm" on the deployment row and confirming a summary (instruments, qty, limits, GTT state); arming lapses
  at session expiry (06:00) and after any kill switch, logout, clock-skew suspension (FR-DATA-004) or IP rejection, and must be
  renewed after the daily login and risk-disclosure acknowledgement (FR-SHELL-020). Armed deployments show an "ARMED" badge in
  Live Strategies and the status indicator shows "Auto: N armed". Every auto order passes the full pipeline (FR-ORD-005) and the
  throttle (FR-ORD-006; C-05); stale or missed signals are never auto-executed (except FR-DEP-010 (4) opt-in). Breaching a risk
  limit disarms the deployment (to mode-1 behaviour), raises an ATTENTION alert and a Telegram message when enabled. "Disarm"
  and the kill switch (FR-ORD-009) take effect immediately.
- **Acceptance criteria:**
  1. Given static IP not confirmed or "Allow automatic execution" off, then Armed auto cannot be selected and the reason is shown.
  2. Given yesterday's armed deployment, after today's login, then it is in mode-1 behaviour with a "Re-arm" action until re-armed.
  3. Given max loss/day ₹1,000 and realised −₹1,050, then the deployment disarms, no further auto orders are sent and an ATTENTION
     alert names the limit.
  4. Given 3 armed deployments and the kill switch, then all three disarm within 1 s and are SUSPENDED.
  5. Given a deployment without risk limits, then "Arm" is disabled with "Set risk limits first".
- **Data/Kite dependency:** ORD pipeline; `PlaceOrder` with `tag`.
- **Deviation from Streak:** Streak does not auto-trade (pending regulatory approval, A06 §3); allowed here only under NFR-05
  explicit opt-in, static IP (C-04) and the order-rate cap (C-05).
- **Priority hint:** Nice-to-have

### FR-DEP-019 Deploy-time checks
- **Source features:** DEP-001, DEP-004
- **Requirement:** On Execute the system shall check and report: Kite session valid; risk disclosure acknowledged (FR-SHELL-020); candle warm-up available for each DI
  (backfill via FR-DATA-003 if not); subscriptions capacity; for mode 1/4: basket margin for the first entry of all DIs vs available funds
  (`GetBasketMargins`, `GetUserMargins`) as a warning; Qty × LTP vs Capital as a warning.
- **Acceptance criteria:**
  1. Given required margin 1.2 L and available 1.0 L, then a warning lists the shortfall and Execute still requires the
     acknowledgement.
- **Data/Kite dependency:** `GetBasketMargins`, `GetUserMargins`, `GetHistoricalData`.
- **Deviation from Streak:** Margin preview added (C-10).
- **Priority hint:** Important

### FR-DEP-020 App-availability and session handling for deployments
- **Source features:** DEP-004, DEP-005
- **Requirement:** The system shall show a persistent banner whenever active deployments are not being evaluated (logged out,
  session expired at 06:00, feed down, offline) and offer one-click login. Closing the app with active deployments shall show a
  dialog listing them and whether each open position is GTT-protected, with options "Keep running in tray" (Enhancement — **UNVERIFIED**: Wails v2 has no system-tray API, A07 §D / C-13; the option is
  shown only if a tray integration is available at build time, OQ-38), "Quit" and "Cancel".
- **Acceptance criteria:**
  1. Given the access token expired, then all DIs are SUSPENDED and a banner says "Deployments paused — log in to resume".
  2. Given 2 IN_POSITION DIs without GTT, when quitting, then the dialog lists both as "unprotected".
- **Data/Kite dependency:** Session state (C-02).
- **Deviation from Streak:** Streak runs server-side; this compensates (C-03). Tray option unverified on the chosen stack (RV-043).
- **Priority hint:** Core

### FR-DEP-021 Live square-off timing
- **Source features:** DEP-004, DEP-006, BT-020
- **Requirement:** Live SO triggers shall use a separate, user-editable **live SO table** per instrument class (Settings →
  Trading), defaulting to times earlier than the broker's MIS auto-square-off: 15:10 for all MIS cash equities (F&O and non-F&O
  stocks), 15:10 for futures; expressed as offsets from the session end of the market calendar (FR-DATA-001) so special sessions
  scale. The table is independent of the backtest SO table (FR-BT-031), which is a simulation rule. Before raising an SO trigger
  the DI reconciles the broker position (`GetPositions`); if it is already flat (broker RMS square-off, manual exit on Kite), the
  DI records "closed externally" instead of raising an exit alert; GTTs for that DI are deleted at the same time (FR-DEP-009).
  Zerodha's RMS timings are not in the research sources and must be confirmed (OQ-31); the Settings page shows the warning "Set
  these earlier than your broker's auto-square-off".
- **Acceptance criteria:**
  1. Given a live MIS long on a non-F&O stock and the table at 15:10, then the SO alert fires at 15:10:00 ± 1 s (app clock), not at
     15:25.
  2. Given the broker shows net qty 0 at that time, then no SO alert is raised and the log shows "closed externally".
  3. Given the user edits the cash row to 15:05, then the next SO fires at 15:05 and FR-BT-031's table is unchanged.
  4. Given a paper MIS position (FR-DEP-017), then the simulated SO uses this table.
- **Data/Kite dependency:** `GetPositions`; `DeleteGTT`; FR-DATA-001/004.
- **Deviation from Streak:** Streak does not square off live positions itself (alerts only; broker RMS does); the local table
  gives the user an alert before the broker acts (RV-017).
- **Priority hint:** Core

---

## FR module SCAN — Scanners

### FR-SCAN-001 Scanner builder layout
- **Source features:** SCAN-001
- **Requirement:** The system shall provide a scanner builder view: editable title (default "Untitled scanner", inline rename),
  link to My Scanners, a Conditions card (universe, timeframe, conditions) with sticky footer Save / Run Scan / Set Alerts (Set
  Alerts enabled once saved), and a Results card with match count, last-run timestamp, export, search and expand. Leaving with
  unsaved changes shall prompt per FR-SHELL-008 (Save / Discard / Stay).
- **Acceptance criteria:**
  1. Given a new scanner, then the results card shows an empty state suggesting another universe or timeframe.
  2. Given unsaved edits, when navigating away, then the unsaved-changes prompt appears (Streak bug §3.2 #15 not replicated).
- **Data/Kite dependency:** Local store.
- **Deviation from Streak:** Unsaved-change guard and inline rename added.
- **Priority hint:** Core

### FR-SCAN-002 Universe selector
- **Source features:** SCAN-002
- **Requirement:** "Scan on" shall offer the universes of FR-WATCH-009 in its order (21 built-ins, custom baskets, watchlists,
  then "Custom symbols"), default Nifty 50. Index and sector universes come from user-imported constituent lists (C-09; the app
  ships empty lists with an import prompt); Indices, Futures and NFO Equities are derived from the instruments dump. The selector
  shows the universe's instrument count and, for imported lists, the constituent snapshot date (stale badge per FR-WATCH-010).
  "Custom symbols" opens the instrument picker (FR-WATCH-007) restricted to NSE/BSE cash, indices and futures, and the selection
  can be saved as a custom basket (FR-WATCH-012).
- **Acceptance criteria:**
  1. Given an imported Nifty 500 CSV (columns Company Name, Industry, Symbol, Series, ISIN Code), then the universe has 500
     instruments mapped to NSE tokens, and unmapped symbols are listed.
  2. Given no Nifty 50 list imported, then the Nifty 50 entry shows "0 — import list" and the scan is refused with a link to
     Settings → Universes.
  3. Given Custom Symbols, then the instrument picker opens and the basket can be saved for reuse.
  4. Given the Futures universe, then the member count equals the number of current-month NFO/BFO futures in the instrument master.
- **Data/Kite dependency:** `GetInstruments` (daily dump, FR-WATCH-008); user-imported CSVs.
- **Deviation from Streak:** Constituent lists are user-imported, not shipped or live (C-09; survivorship bias documented).
  Option universes and the CALL/PUT/VOLUME>0/strike-range filters are dropped with the options scope (§10 SCAN-002-opt).
- **Priority hint:** Core

### FR-SCAN-003 Scanner timeframe and chart type
- **Source features:** SCAN-003
- **Requirement:** Base timeframe choices shall be 1, 3, 5, 10, 15, 30 min, 1 hour, 1 day (default 1 min; FEATURES §6-7). Chart
  type is candlestick only and shall be shown as a non-interactive label (not a button). Week/month are allowed only on Advance
  condition rows (MTF).
- **Acceptance criteria:**
  1. Given the timeframe dropdown, then exactly 8 options appear.
- **Data/Kite dependency:** Kite intervals; candle store resampling (FR-DATA-002).
- **Deviation from Streak:** Inert chart-type button not replicated (§3.2 #9).
- **Priority hint:** Core

### FR-SCAN-004 Scanner conditions (shared editor)
- **Source features:** SCAN-004
- **Requirement:** The scanner shall reuse the strategy condition editor (Basic and Advance modes, indicator catalogue,
  comparators, AND/OR, groups, per-row timeframe in Advance, row tools, copy/paste, suggestions/"Assist", prebuilt setups) without
  side or exits. Fully empty rows (no LHS; §3.1) are dropped at Run Scan with an INFO toast naming how many were ignored; incomplete rows
  (LHS present, comparator or RHS missing) block Run Scan with an ERROR toast and inline outline (FR-SHELL-009 AC4), as in
  the strategy builder (FR-EQB-008). No artificial condition-count limit (NFR-13).
- **Acceptance criteria:**
  1. Given 12 Advance rows, then the scanner saves and runs (no 10-row cap).
  2. Given 2 filled and 1 fully empty Basic row, then Run Scan evaluates the 2 filled rows and an INFO toast says "1 empty
     condition ignored"; given 2 filled and 1 incomplete row, then Run Scan is blocked and the row is outlined.
- **Data/Kite dependency:** none.
- **Deviation from Streak:** No 5/10 condition caps. Incomplete rows block (Streak silently ignores them, EQB-006 note); empty rows are ignored for parity (RV-022).
- **Priority hint:** Core

### FR-SCAN-005 Advance-mode validation
- **Source features:** SCAN-005
- **Requirement:** Before running, the system shall block and highlight (inline + error toast) any Advance row with empty
  operands ("Fill all conditions before running"), and shall block with an explicit visible error when no row uses the base
  timeframe ("At least one condition must use the base timeframe (X)") offering to set the base timeframe to a row's timeframe.
- **Acceptance criteria:**
  1. Given every row on "day" and base = 1 min, when Run Scan, then the explicit error appears (not silent) with an action "Use 1 day as base".
  2. Given that action, then the base becomes 1 day and the scan runs.
- **Data/Kite dependency:** none.
- **Deviation from Streak:** Silent no-op (§3.2 #13) replaced by explicit error.
- **Priority hint:** Core

### FR-SCAN-006 Manual scan (Run Scan)
- **Source features:** SCAN-006
- **Requirement:** Run Scan shall evaluate the conditions once over the universe, on saved or unsaved drafts. During market hours
  the base-TF condition uses the **current forming candle** (from the live candle builder for subscribed instruments; otherwise
  from today's candles via historical fetch merged with the latest quote); higher-TF rows use their last closed candle; outside
  market hours the last completed candle is used. The system shall show progress for data loading, then a toast ("N matches" or
  "No matches") and set the result timestamp (DD-MM-YYYY HH:MM:SS) to the evaluation time. Data older than the evaluation candle
  shall be reported per instrument ("stale data: 3 instruments").
- **Acceptance criteria:**
  1. Given a cached Nifty 50 universe on 1-day base outside market hours, then the scan completes in < 2 s.
  2. Given market hours and a 5-min base at 10:07, then the base candle evaluated is 10:05–10:10 forming (values as of 10:07).
  3. Given zero matches, then an info toast "No matches" appears and the table shows the empty state.
- **Data/Kite dependency:** Ticker candle builder; `GetHistoricalData` and `GetQuote` via FR-DATA-003 (3 req/s; ≤500 per call, 1 req/s).
- **Deviation from Streak:** Large uncached universes may take longer than Streak's server-side scan (bounded by 3 req/s);
  progress is shown. Whether Kite historical returns today's partial candle is UNVERIFIED (OQ-35).
- **Priority hint:** Core

### FR-SCAN-007 Scanner results table
- **Source features:** SCAN-007
- **Requirement:** Results shall list Symbol (exchange tag), LTP, Change %, Volume and actions B, S, Chart. Header click cycles
  ascending → descending → unsorted; search filters by symbol substring and updates the count; Export writes CSV (symbol,
  exchange, LTP, change %, volume, timestamp, and the values of each condition's operands); Expand shows results full-width with
  an inline toolbar (universe, timeframe, Run Scan, collapse) and a "Show conditions" link. **Enhancement (beyond Streak):**
  optional columns for indicator values used in conditions. B/S open the order ticket with a technical summary
  (FR-ORD-004). After-hours % change notes that the official close is a 15:00–15:30 VWAP.
- **Acceptance criteria:**
  1. Given 5 results, clicking LTP thrice yields ascending, descending, original order.
  2. Given search "ADA", then only symbols containing ADA remain and the count updates.
  3. Given Export, then the CSV row count = visible results.
- **Data/Kite dependency:** Ticker/quotes for LTP; local CSV export.
- **Deviation from Streak:** Indicator-value columns added.
- **Priority hint:** Core

### FR-SCAN-008 Save scanner
- **Source features:** SCAN-008
- **Requirement:** Save shall open a dialog (Name required, Tags up to 5, Description), persist the scanner as a ScannerDoc (FR-SCAN-016: universe, timeframe,
  conditions, mode, alert settings) to My Scanners, and keep the builder open on the saved scanner. Saving again updates it;
  "Save as copy" creates a new one.
- **Acceptance criteria:**
  1. Given a saved scanner reopened, then universe, timeframe, conditions and alert settings are identical (ScannerDoc round-trip).
- **Data/Kite dependency:** SQLite.
- **Deviation from Streak:** none (Streak dialog unobserved).
- **Priority hint:** Core

### FR-SCAN-009 Set Alerts (take a scanner live)
- **Source features:** SCAN-009
- **Requirement:** Set Alerts shall open a dialog with a prominent note "Alerts trigger at every <timeframe> candle close" (frequency = scanner base timeframe),
  notification channels (in-app always; OS and Telegram per FR-ACCT-003/014 settings), alert content option ("all matches" default / "only new matches since last alert"), optional end date, and a
  required acknowledgement; "Start alerts" activates the live scanner; the dialog closes on Escape, × and Cancel (FR-SHELL-015); the acknowledgement
  links to the in-app Terms of use (FR-ACCT-015). Settings are stored in the ScannerDoc (FR-SCAN-016). For a prebuilt scanner, the user first copies it
  (DISC flow) then activates the copy.
- **Acceptance criteria:**
  1. Given a 1-day scanner, then the dialog states "Alerts trigger at every 1 Day candle close".
  2. Given the acknowledgement unticked, then Start alerts is disabled.
- **Data/Kite dependency:** Local scheduler.
- **Deviation from Streak:** Content option and end date added (Enhancement); no 25-scanner cap; dialog closable by Escape/×/Cancel (Streak's closes only on backdrop click, REVIEW-2 N5); the trigger-frequency note matches Streak.
- **Priority hint:** Core

### FR-SCAN-010 Live scanner engine
- **Source features:** SCAN-010
- **Requirement:** While the app runs and the session is valid, each active scanner shall be evaluated at every base-TF candle
  close during the continuous session (FR-DATA-001; 5-min → 09:20, 09:25, …; boundaries on the app clock of FR-DATA-004), using completed base candles and last closed higher-TF candles per FR-BT-034, and shall
  create one alert per run with the matched symbols (or new matches, per FR-SCAN-009) within 2 s of the candle close for universes
  fully covered by the ticker. On start or after an outage, missed runs of the current day are replayed and stored as "Missed"
  entries (not notified as OS notifications, shown in history).
- **Acceptance criteria:**
  1. Given a 5-min live scanner on Nifty 50, then alerts are created at each 5-min close with match lists equal to a manual scan of
     the same completed candle.
  2. Given the app was closed 10:00–10:30, then on start the 10:05…10:30 runs appear in history marked "Missed".
- **Data/Kite dependency:** Ticker subscriptions (priority "live scanner"), `GetHistoricalData` for replay.
- **Deviation from Streak:** Runs only while the app is open (C-03); faster than Streak.
- **Priority hint:** Core

### FR-SCAN-011 Scanner alerts history view
- **Source features:** SCAN-011
- **Requirement:** The system shall provide a master/detail view: left — searchable, filterable (status: Active / Paused / Ended;
  timeframe) list of the user's scanners with alert status; right — alert history for the selected scanner (time, match count,
  symbols, Missed flag) with click-through to a result snapshot (symbols, LTP at alert time) and B/S/Chart actions; header actions
  Create Scanner and My Scanners.
- **Acceptance criteria:**
  1. Given a scanner with 12 alerts, then 12 rows show newest first and each opens its snapshot.
  2. Given the filter "Paused", then only paused scanners show (filter works; "coming soon" not replicated, §3.2-31).
- **Data/Kite dependency:** Local alert store.
- **Deviation from Streak:** Working filters.
- **Priority hint:** Important

### FR-SCAN-012 Stop / resume alerts
- **Source features:** SCAN-012
- **Requirement:** The user shall be able to Stop (pause) and Resume alerts of a live scanner from My Scanners, the scanner
  builder footer and the alerts view; pausing unsubscribes instruments not needed by other consumers.
- **Acceptance criteria:**
  1. Given Stop, then no further runs occur and status = Paused; Resume restarts at the next candle close.
- **Data/Kite dependency:** Ticker subscription manager.
- **Deviation from Streak:** none.
- **Priority hint:** Core

### FR-SCAN-013 Scanner capacity (no artificial limits)
- **Source features:** SCAN-013
- **Requirement:** The system shall not cap condition counts, live scanner count, custom basket count or basket size. Activation
  of a live scanner shall be refused only if total live subscriptions would exceed 9,000 instruments, with the current usage
  breakdown (deployments, scanners, charts, watchlists) shown.
- **Acceptance criteria:**
  1. Given 30 live scanners within capacity, then all run.
  2. Given usage 8,800 and a new 500-instrument scanner, then activation is refused with the breakdown.
- **Data/Kite dependency:** Ticker limits (07 §A6).
- **Deviation from Streak:** Streak caps 5/10 conditions, 25 live scanners, 10 baskets × 50.
- **Priority hint:** Important

### FR-SCAN-014 Open chart from a result row
- **Source features:** SCAN-014
- **Requirement:** The Chart action on a result row shall open the full-view chart (FR-CHART-017) for the symbol at the scanner timeframe.
- **Acceptance criteria:**
  1. Given a 1-day scanner result ADANIENT, then the chart opens on ADANIENT, 1 day.
- **Data/Kite dependency:** Local candles / `GetHistoricalData`.
- **Deviation from Streak:** In-app full-view chart instead of a browser tab (FR-CHART-017; RV-035).
- **Priority hint:** Important

### FR-SCAN-015 Scanner vs strategy builder scope
- **Source features:** SCAN-015
- **Requirement:** The scanner builder shall: have no side and no exits; use universe selection instead of an instrument list;
  offer the same 8 base timeframes and week/month on Advance rows; share comparators, indicators and suggestions with the
  strategy builder; and add scanner-only features (Run Scan, results sort/search/export/expand). **Enhancement
  (beyond Streak):** "Create strategy from scanner" copies conditions into a new strategy draft.
- **Acceptance criteria:**
  1. Given the scanner builder, then no Buy/Sell toggle or exit card is present.
  2. Given "Create strategy from scanner", then a strategy draft with the same conditions and timeframe opens.
- **Data/Kite dependency:** none.
- **Deviation from Streak:** Scanner→strategy linking added.
- **Priority hint:** Core (scope) · Nice-to-have (enhancement)

### FR-SCAN-016 Scanner document schema
- **Source features:** SCAN-008, SCAN-009, DISC-017
- **Requirement:** The strategy model (FR-EQB-002) is extended with the scanner document, published in the same JSON Schema
  (`docs/spec/strategy-schema-v1.json`) and DSL doc:
  ```
  ScannerDoc  { schema: "tl.scanner", version: "<semver>", id: uuid,
                meta: { name, tags[], description, favourite, createdAt, updatedAt, family?,
                        featured?: bool, origin: { type: "user"|"library"|"import", libraryId?, libraryVersion? } },
                universe: UniverseRef, timeframe: TF (1m..1d only), conditions: ConditionBlock,
                alerts?: { enabled: bool, content: "all"|"new", endDate?: date, channels: { desktop: bool, telegram: bool },
                           frozenSymbols?: [InstrumentRef] } }
  UniverseRef = { builtin: <one of the 21 ids of FR-WATCH-009> } | { basket: id } | { watchlist: id } | { custom: [InstrumentRef] }
  ```
  `frozenSymbols` holds the copy a scanner keeps when its basket is deleted (FR-WATCH-012 AC3). Round-trip (`parse(format(doc))
  == doc`), migration, unknown-field preservation and export rules are those of FR-EQB-002; library scanners validate against
  the schema in CI; export files use `.tlscanner.json` (FR-DISC-019).
- **Acceptance criteria:**
  1. Given FR-EQB-002 AC1–AC5 applied to ScannerDoc fixtures, then they pass.
  2. Given a scanner with alerts enabled, content "new", end date and Telegram on, when exported and imported on a clean install,
     then the imported doc restores all alert settings (alerts remain disabled until the user re-enables them, with a note).
  3. Given a `UniverseRef` naming an unknown built-in id, then validation fails with `UNKNOWN_UNIVERSE`.
- **Data/Kite dependency:** Local only.
- **Deviation from Streak:** Streak's internal format is not observable; ours is documented (RV-016). The option-filter block is
  deferred (§10 SCAN-016-opt).
- **Priority hint:** Core

---

## FR module SCALP — Scalper workspace (equity/futures)

The Scalper is adapted to equity/futures (§1.3): the option-chain drawer, option templates and payoff of Streak's
Scalper are in §10 (SCALP-004/006/007).

### FR-SCALP-001 Scalper layout (equity/futures workspace)
- **Source features:** SCALP-001
- **Requirement:** The system shall provide a Scalper workspace for fast trading of stocks and futures with an index as
  reference: top half — **left chart** = reference instrument (index or stock; default NIFTY 50), **right chart** = the
  current-month future of the left chart's underlying when one exists (default NIFTY current-month future), otherwise the last
  instrument the user chose for that slot; edge tabs Watchlist (left, FR-SCALP-008) and Add/Hide Chart (right, FR-SCALP-003);
  bottom half — left analytics panel (FR-ANLY-001) for the left chart's symbol, right Positions / Open orders / Order book panel
  (FR-SCALP-009) with refresh and collapse (FR-SCALP-010). Each chart is the shared chart component (FR-CHART-001) with chart
  trading (FR-CHART-012/013/014) and the Scalper additions of FR-SCALP-011/013. Any chart slot accepts an index, stock or futures
  contract; option contracts are not selectable (NFR-15). Layout, symbols and timeframes persist locally across restarts
  (FR-CHART-016).
- **Acceptance criteria:**
  1. Given a fresh install, then the left chart shows NIFTY 50 and the right chart shows the NIFTY current-month future (e.g. `NIFTY
     OCT FUT`).
  2. Given the user changes symbols and restarts, then the same symbols reappear; given the right chart's contract expired, then it
     falls back to the new current-month future with an INFO toast.
  3. Given RELIANCE chosen for the left chart, then the right chart offers "Switch to RELIANCE OCT FUT" (FR-SCALP-002).
  4. Given the symbol search in any Scalper chart, then no option contract is listed.
- **Data/Kite dependency:** Ticker; instruments dump (futures expiries, lot sizes); candle store (FR-DATA-002).
- **Deviation from Streak:** Streak's Scalper is an options workspace (underlying + ATM CE/PE charts); ours charts the
  underlying and its futures (user decision 2026-10-12; §10 SCALP-001-opt records the option layout). Local persistence
  (Streak's unclear).
- **Priority hint:** Important

### FR-SCALP-002 Chart headers and underlying ⇄ future flip
- **Source features:** SCALP-002
- **Requirement:** Each Scalper chart shall have the standard chart header (FR-CHART-002: symbol search, LTP with % change,
  refresh, timeframe with 8 options defaulting to the Settings → Chart default, save/clear layout, indicators) plus a **flip**
  control that swaps the charted symbol between the cash/index instrument and its current-month future (and, when a next-month
  future exists, cycles underlying → current month → next month → underlying); the flip is hidden for instruments without
  futures. Changing chart 1's timeframe sets all charts to it; each can then be changed individually.
- **Acceptance criteria:**
  1. Given the right chart shows NIFTY OCT FUT, when flipped, then it shows NIFTY 50; flipped again, NIFTY OCT FUT; again, NIFTY NOV
     FUT (if listed).
  2. Given chart 1 changed to 5 min, then charts 2 and 3 switch to 5 min; changing chart 2 afterwards does not change chart 1.
  3. Given a non-F&O stock in a chart, then no flip control is shown.
- **Data/Kite dependency:** Ticker; instruments dump.
- **Deviation from Streak:** Streak's CE⇄PE flip is replaced by an underlying⇄future flip (scope; §10 SCALP-002-opt). Timeframe
  sync behaviour is parity.
- **Priority hint:** Important

### FR-SCALP-003 Add / hide third chart
- **Source features:** SCALP-003
- **Requirement:** Add Chart shall insert a third chart showing the next-month future of the right chart's underlying (if
  listed; otherwise the right chart's underlying cash/index instrument); Hide removes it. The user may change its symbol freely.
  When the watchlist drawer is open the workspace shows at most 2 charts (FEATURES §6-26) and the UI states this.
- **Acceptance criteria:**
  1. Given Add Chart with the right chart on NIFTY OCT FUT, then the third chart shows NIFTY NOV FUT.
  2. Given 3 charts and the watchlist opened, then the third chart is hidden with a visible note, and restored when the watchlist
     closes.
- **Data/Kite dependency:** Instruments dump.
- **Deviation from Streak:** Streak's third chart is the ATM put (scope; §10 SCALP-003-opt). Layout constraint surfaced in UI
  (§3.2 #32).
- **Priority hint:** Important

### FR-SCALP-005 Basket order panel (equity/futures legs)
- **Source features:** SCALP-005
- **Requirement:** The basket panel shall list legs — any NSE/BSE cash equity or NFO/BFO futures contract, added from the
  watchlist row "Add to basket", the chart header menu or the symbol search — with select checkbox (and select-all), drag handle,
  B/S, instrument, LTP, product (MIS / CNC for cash, MIS / NRML for futures), order type (Market / Limit, with price field for
  Limit), qty (lot-stepped spinner for futures, share spinner for cash), required margin per leg and status; footer shows total
  margin required and margin after execution (basket margin with spread benefit, e.g. for a calendar spread of two futures of the
  same underlying; recalc button) and **Place basket**. Placing shows a confirmation summary, then submits the selected legs
  **sequentially in the shown order** through the order pipeline (FR-ORD-005/006); each leg's status updates live. If a leg is
  rejected, remaining legs are held and the user chooses Continue / Cancel remaining. A **Save basket** action stores the legs as
  a named template (own name) for reuse; no option templates exist.
- **Acceptance criteria:**
  1. Given 4 legs, then 4 orders are placed in displayed order, each respecting the throttle, and statuses show Complete/Rejected.
  2. Given leg 2 rejected, then legs 3–4 are not sent until the user chooses.
  3. Given the qty spinner on NIFTY futures (lot 65), then values step 65, 130, …; on a cash equity they step 1.
  4. Given legs long NIFTY OCT FUT and short NIFTY NOV FUT, then the footer's "after execution" margin reflects the basket API's
     `final` total (spread benefit) in a mocked-API test.
- **Data/Kite dependency:** `GetBasketMargins` (initial/final), `PlaceOrder` (market_protection for Market), ticker order updates.
- **Deviation from Streak:** Streak's basket holds option legs and templates (scope; §10 SCALP-005-opt, SCALP-006);
  equity/futures baskets with saved templates are our adaptation. Rejection hold behaviour defined (Streak's unobserved).
- **Priority hint:** Important

### FR-SCALP-008 Docked watchlist drawer
- **Source features:** SCALP-008
- **Requirement:** The watchlist drawer shall dock beside the charts (not overlay; fixed width 320 px, user-resizable 280–480 px)
  and reuse the standard watchlist component (FR-WATCH-001: lists, create, search, delete, rows with B/S, chart and "Add to
  basket"); selecting a row loads it into the left chart (or the focused chart when one is focused).
- **Acceptance criteria:**
  1. Given the drawer opens, then the chart area narrows by the drawer width (320 px by default), the charts re-render within 300 ms
     and a pan gesture on a chart responds within 100 ms.
  2. Given a row is selected while chart 2 is focused, then chart 2 loads it.
- **Data/Kite dependency:** see watchlist module.
- **Deviation from Streak:** Focused-chart targeting and "Add to basket" added.
- **Priority hint:** Nice-to-have

### FR-SCALP-009 Positions / Open orders / Order book panel
- **Source features:** SCALP-009
- **Requirement:** The docked panel shall show tabs Positions (Product, Symbol, Qty, Avg, LTP, P&L, Net change, Total P&L),
  Open orders (Time, Type, Symbol, Qty, LTP, Price, Status) and Order book (Time, Type, Symbol, Qty, LTP, Avg price, Status),
  live-updated from ticker order updates and LTP, with manual refresh, a Paper sub-tab when paper deployments exist (FR-DEP-017),
  and row actions per FR-ORD-011.
- **Acceptance criteria:**
  1. Given an order filled, then it moves from Open orders to Order book and the position appears within 1 s of the order update.
- **Data/Kite dependency:** `GetPositions`, `GetOrders`, ticker `OnOrderUpdate`.
- **Deviation from Streak:** Paper sub-tab added.
- **Priority hint:** Important

### FR-SCALP-010 Collapse bottom panels
- **Source features:** SCALP-010
- **Requirement:** A collapse control shall collapse both bottom panels to their tab bars so charts take full height; Expand
  restores them; state persists (shared with FR-ANLY-002's Scalper context).
- **Acceptance criteria:**
  1. Given Collapse, then chart height increases to the window height minus header and tab bars.
- **Data/Kite dependency:** none.
- **Deviation from Streak:** none.
- **Priority hint:** Nice-to-have

### FR-SCALP-011 Trade from chart (Scalper delta over the shared chart trading)
- **Source features:** SCALP-011
- **Requirement:** Scalper charts use the shared chart-trading behaviour unchanged: Buy/Sell overlay buttons (FR-CHART-012,
  including the "—" label when no bid/ask is available and the hidden state in signal-only mode), the crosshair "+" and context
  menu (FR-CHART-011/013) and order/position lines with drag-to-modify and × (FR-CHART-014, confirmations per NFR-05). The Scalper
  adds only: (a) **quantity presets** — a strip of up to 5 user-defined quantities (lots for futures, shares for cash) above the
  Buy/Sell buttons; the active preset is the ticket's default quantity (FR-SCALP-013); (b) **Enhancement (beyond Streak)** —
  opt-in **one-click trading**: enabled per session from the Scalper header with a typed confirmation, a max quantity and a max
  orders-per-minute (defaults 1 lot / 10), it skips the order ticket for Buy/Sell/+ actions (the order still passes the full
  pipeline FR-ORD-005 and shows a 3-second undoable toast "Buy 65 NIFTY OCT FUT sent — Cancel order"); it lapses at session
  expiry, kill switch or trading disablement.
- **Acceptance criteria:**
  1. Given a drag of a limit order line from 120 to 118, then FR-CHART-014's confirmation appears, one modify call is sent and the
     counter shows 24 remaining.
  2. Given market closed, then the Buy/Sell buttons show "—" and still open the order ticket (same as the dashboard chart).
  3. Given one-click enabled with max qty 1 lot and preset "2 lots", then Buy opens the order ticket instead (quantity above the
     one-click cap) with a note.
  4. Given one-click enabled, when Buy is pressed, then exactly one `PlaceOrder` call occurs within the throttle and the undo toast
     appears; pressing "Cancel order" sends one cancel.
- **Data/Kite dependency:** Ticker full mode (best bid/ask), `PlaceOrder`, `ModifyOrder`, `CancelOrder`, `GetOrderMargins`.
- **Deviation from Streak:** Behaviour is one shared implementation (RV-012); one-click mode and presets are additions.
- **Priority hint:** Important

### FR-SCALP-012 Indicators on Scalper charts
- **Source features:** SCALP-012
- **Requirement:** Every Scalper chart (including the third) shall support indicators without a fixed count limit; a soft
  warning appears above 10 indicators per chart for performance.
- **Acceptance criteria:**
  1. Given 6 indicators on chart 3, then all render and the chart repaints within 100 ms on a tick.
- **Data/Kite dependency:** none.
- **Deviation from Streak:** Streak allows 5 per chart on charts 1–2 only; removed per NFR-13.
- **Priority hint:** Nice-to-have

### FR-SCALP-013 Scalper additions not in Streak — Enhancement (beyond Streak)
- **Source features:** SCALP-013
- **Requirement:** The parity Scalper has no market depth, hotkey settings or quantity presets. As enhancements the system shall
  offer: (a) a 5-level market depth panel for the focused chart's instrument (ticker full mode), with total bid/ask quantities;
  (b) Scalper actions in the shortcut registry (FR-SHELL-019): buy at market, sell at market, exit all positions of the focused
  symbol, cancel all open orders of the focused symbol — unbound by default, each opening the order ticket (or the one-click
  path when FR-SCALP-011 one-click is on); (c) quantity presets (up to 5, e.g. 1/2/5 lots or 50/100/500 shares), edited inline
  and stored per segment.
- **Acceptance criteria:**
  1. Given depth enabled, then 5 bid/ask levels with quantities update live within 500 ms of a tick.
  2. Given a shortcut bound to "Buy at market", then it opens the same order ticket as the Buy button (never bypasses confirmation
     unless one-click mode is on).
  3. Given presets 1/2/5 lots on a futures chart, then selecting "5" makes the next ticket default to 5 × lot size units.
- **Data/Kite dependency:** Ticker `ModeFull`.
- **Deviation from Streak:** All three are absent in Streak.
- **Priority hint:** Nice-to-have

---

## FR module ORD — Orders, positions, holdings and order safety

### FR-ORD-001 Order book
- **Source features:** ORD-001
- **Requirement:** The order book view shall show sections Open, Executed, Rejected, Cancelled with counts; a Paper tab when paper deployments exist (FR-DEP-017); columns Time, Type
  (Buy/Sell), Instrument, Product, Qty (filled/total), Avg price, Status, Source (manual / deployment name / basket / scalper, from
  the order tag); search (instrument, order ID, source), refresh, and live updates from ticker order updates with a 30 s `GetOrders`
  poll fallback.
- **Acceptance criteria:**
  1. Given an order placed from a deployment, then its Source column shows the deployment name.
  2. Given the ticker is disconnected, then the list still refreshes at least every 30 s.
- **Data/Kite dependency:** `GetOrders`, `GetOrderHistory`, `GetTrades`, ticker `OnOrderUpdate`.
- **Deviation from Streak:** Source column and section counts for all statuses added.
- **Priority hint:** Core

### FR-ORD-002 Positions
- **Source features:** ORD-002
- **Requirement:** The positions view shall show Net and Day tabs (and a Paper tab when paper deployments exist, FR-DEP-017), open and closed positions with Product, Symbol, Qty, Avg, LTP,
  P&L, Net change, a Total P&L footer, live LTP, refresh and search; row actions per FR-ORD-011 including product conversion
  (MIS↔CNC/NRML) with confirmation.
- **Acceptance criteria:**
  1. Given two positions with P&L +120 and −45.50, then Total P&L = 74.50.
  2. Given Convert MIS→CNC for 10 shares, then one `ConvertPosition` call is made after confirmation and the row updates.
- **Data/Kite dependency:** `GetPositions`, `ConvertPosition`, ticker LTP.
- **Deviation from Streak:** Total footer and conversion explicit.
- **Priority hint:** Core

### FR-ORD-003 Holdings
- **Source features:** ORD-003
- **Requirement:** The holdings view shall show Symbol, Qty, Avg, LTP, Invested, Current value, P&L, Net change, Day change, totals
  row, search and refresh; empty state "No holdings".
- **Acceptance criteria:**
  1. Given no holdings, then the empty state reads "No holdings" (§3.2 #25 not replicated).
  2. Given 10 @ 100 and LTP 110, then Invested 1,000, Current 1,100, P&L 100, Net change +10.00 %.
- **Data/Kite dependency:** `GetHoldings`, `GetAuctionInstruments`.
- **Deviation from Streak:** Correct empty-state copy, refresh added.
- **Priority hint:** Core

### FR-ORD-004 Order ticket
- **Source features:** ORD-004
- **Requirement:** All B/S entry points (watchlists, scanner results, alert snapshots, chart buttons/context menu/"+",
  notifications, Scalper shortcuts, basket) shall open one **order ticket** with: side, instrument, exchange, qty (lots for
  futures; remembered per underlying when FR-ACCT-002 is on; default from the active Scalper preset when opened from the
  Scalper), product (CNC / MIS for cash, MIS / NRML for futures), order type (Market / Limit / SL / SL-M), price and trigger
  price as applicable, validity (Day / IOC), required margin and charges preview, the current session phase and its banner
  (FR-ORD-015), and (from scanner results and deployment alerts) a technical summary of the condition values. In **review mode**
  (opened from a deployment alert, FR-DEP-007) the ticket is prefilled and shows the signal price and condition summary; its
  primary button reads "Place order". **Enhancement (beyond Streak):** variety AMO outside market hours, iceberg and TTL
  validity, each enabled in Settings → Trading (off by default). Indices are not orderable (B/S disabled with tooltip). Option
  contracts cannot be entered (NFR-15).
- **Acceptance criteria:**
  1. Given a NIFTY futures contract with lot 65 and qty 100, then the ticket blocks with "quantity must be a multiple of 65".
  2. Given SL type, then trigger price is required; given SL-M, then price is hidden and trigger required.
  3. Given a scanner result B, then the ticket shows the conditions and their current values above the form.
  4. Given 15:45 and an NSE equity, then the ticket shows the post-closing banner and only MARKET is selectable; given AMO disabled
     in Settings at 16:05, then the ticket's primary button is disabled with "Next session 09:00 <date>".
- **Data/Kite dependency:** `GetOrderMargins`, `GetOrderCharges`, instruments dump; `PlaceOrder`.
- **Deviation from Streak:** AMO/iceberg/TTL optional enhancements; one ticket for every entry point (Streak's order ticket fields
  were not fully observed).
- **Priority hint:** Core

### FR-ORD-005 Order safety pipeline
- **Source features:** ORD-004, ORD-006, ORD-007
- **Requirement:** Every order action (place, modify, cancel, exit, basket leg, GTT) from any source shall pass, in order:
  (1) **Gate:** trading enabled (FR-ORD-013), Kite session valid, kill switch not engaged; (2) **Validation:** instrument tradable
  and not expired; qty > 0 and lot multiple; qty ≤ freeze quantity (else block, or autoslice if the user enabled it); price/trigger
  multiples of tick size; Limit price within the day's circuit limits; SL trigger on the correct side of LTP; CNC sell ≤ holdings
  qty; product valid for segment; order-placement phase per FR-DATA-001 / FR-ORD-015 (regular variety in pre-open, continuous and — equity cash MARKET only — post-closing sessions); (3) **Risk limits:** user-configurable max order value,
  max qty per order, max orders per day, max daily loss; a price deviation warning when a Limit price is > X % (default 5 %) from
  LTP; (4) **Margin:** `GetOrderMargins` / `GetBasketMargins` vs `GetUserMargins` available — block if insufficient; (5) **Duplicate
  guard:** an identical order (instrument, side, qty, product, type, price) from a manual source within 3 s requires re-confirmation;
  (6) **Confirmation:** explicit user confirmation, or armed-auto authorisation (FR-DEP-018); (7) **Journal intent** (FR-ORD-008);
  (8) **Throttle** (FR-ORD-006); (9) **Submit** with `market_protection` (FR-ORD-007), a unique `tag` and — when set in Settings → Trading — the `algo_id` value (OQ-37; field reserved, empty by default); (10) **Journal outcome**
  and track updates. A failure at any stage stops the order and shows the stage and reason.
- **Acceptance criteria:**
  1. Given trading disabled, then no Kite order endpoint is called (verified with a mock client) and the UI says why.
  2. Given qty above the freeze limit and autoslice off, then the order is blocked at stage 2.
  3. Given available margin 10,000 and required 12,000, then blocked at stage 4.
  4. Given every stage passes, then exactly one `PlaceOrder` call carries market_protection (if Market/SL-M) and the tag.
- **Data/Kite dependency:** `GetQuote` (circuit limits), `GetOrderMargins`, `GetBasketMargins`, `GetUserMargins`, `GetHoldings`,
  `PlaceOrder`/`ModifyOrder`/`CancelOrder`; optional freeze-quantity table (Kite applies freeze limits server-side and `autoslice=true` splits into ≤ 10 slices; OQ-34).
- **Deviation from Streak:** Explicit, documented pipeline (NFR-05).
- **Priority hint:** Core

### FR-ORD-006 Order throttle and rate limits
- **Source features:** ORD-004, ORD-006
- **Requirement:** A single process-wide order queue shall enforce hard caps below Kite/SEBI limits: ≤ 8 order requests per second
  (configurable 1–9, never ≥ 10; SEBI threshold 10 OPS per exchange), ≤ 380 per minute (limit 400), ≤ 4,800 per day (limit 5,000),
  and ≤ 24 modifications per order (limit 25, warning from 20). Priority: kill-switch cancels > exits > cancels > modifications >
  entries. HTTP 429 or network errors on placement shall never be retried blindly: the order is looked up by tag first. Counters
  persist across restarts within the trading day. When a cap is reached, further orders are refused with the cap named.
- **Acceptance criteria:**
  1. Given 20 basket orders queued, then no 1-second window contains more than 8 requests.
  2. Given 24 modifications on an order, then the 25th is refused with "modification limit reached".
  3. Given a timeout on PlaceOrder, then `GetOrders` is checked for the tag before any retry; if found, no retry occurs.
  4. Given the daily count reaches 4,800, then new entries are refused while exits remain allowed up to the Kite limit.
- **Data/Kite dependency:** Kite rate limits (07 §A3), SEBI threshold (07 §B).
- **Deviation from Streak:** none (Streak server-side throttling not visible).
- **Priority hint:** Core

### FR-ORD-007 Market protection
- **Source features:** ORD-004
- **Requirement:** Every MARKET and SL-M order shall carry `market_protection`, default −1 (automatic), user-configurable to a
  percentage 0 < x ≤ 100; the value 0 shall never be sent. LIMIT and SL orders carry no market protection.
- **Acceptance criteria:**
  1. Given a Market order with default settings, then the request has market_protection = −1.
  2. Given the user enters 0, then the setting is rejected with "must be −1 or between 0 and 100 (exclusive of 0)".
- **Data/Kite dependency:** `OrderParams.MarketProtection` (gokiteconnect v4).
- **Deviation from Streak:** none.
- **Priority hint:** Core

### FR-ORD-008 Order journal and audit trail
- **Source features:** ORD-001, ORD-005, ORD-006
- **Requirement:** The system shall keep an append-only journal of every order action: intent (before sending; source, deployment,
  sanitised parameters), submission, Kite response (order ID or error), every status update, fills, modifications, cancellations,
  and outcome-unknown states. Secrets and access tokens are never written (NFR-04). The journal is viewable and filterable in-app
  and exportable to CSV. Each order has a unique `tag` (≤ 20 alphanumeric) linking Kite orders to journal entries.
- **Acceptance criteria:**
  1. Given a placed and filled order, then the journal shows intent → submitted → OPEN → COMPLETE with timestamps.
  2. Given a search of journal files and logs for the access token string, then no match exists.
  3. Given an unknown outcome, then a reconciliation entry records how it was resolved.
- **Data/Kite dependency:** SQLite; ticker order updates; `GetOrders`.
- **Deviation from Streak:** Audit trail visible to the user (NFR-10).
- **Priority hint:** Core

### FR-ORD-009 Kill switch
- **Source features:** ORD-006
- **Requirement:** An always-visible kill switch (button and the kill-switch binding of FR-SHELL-019, default Ctrl+Shift+K) shall, after one confirmation: clear the order
  queue; cancel all open **entry** orders placed by the app (protective SL/SL-M exit orders and GTTs on open positions are
  kept — OQ-45 decision; option: also cancel them / all open orders in the account); disarm and suspend all deployments
  (SUSPENDED, reason "kill switch"); block new orders until the user re-enables trading. Protective GTTs remain unless the user
  ticks "also delete GTTs". A separate "Exit all positions" action (own confirmation) places exit orders for all open positions.
  All actions are journaled; the operation completes within 5 s for ≤ 30 open orders (throttle permitting).
- **Acceptance criteria:**
  1. Given 3 open app orders and 2 armed deployments, then after the kill switch 3 cancel requests were sent, both deployments are
     SUSPENDED and a new order attempt is blocked at stage 1.
  2. Given the kill switch pressed twice, then the second press is a no-op (idempotent).
- **Data/Kite dependency:** `CancelOrder`, `GetOrders`, `DeleteGTT`.
- **Deviation from Streak:** Streak has no kill switch (required by NFR-05).
- **Priority hint:** Core

### FR-ORD-010 Rejected order reason
- **Source features:** ORD-005
- **Requirement:** Rejected orders shall show a rejection indicator; selecting it shows the broker's `status_message` verbatim plus
  a plain-language hint for common causes (insufficient margin, static IP not registered, freeze limit, circuit limit, market
  protection missing).
- **Acceptance criteria:**
  1. Given a rejected order with a status message, then the message is displayed verbatim in the detail.
- **Data/Kite dependency:** `GetOrders` / order update `status_message`.
- **Deviation from Streak:** Hints added.
- **Priority hint:** Core

### FR-ORD-011 Row actions on orders and positions
- **Source features:** ORD-006
- **Requirement:** Order and position rows (scalper panel and standalone views) shall offer: Buy, Sell, Info (order history and
  trades), Cancel (open orders), Modify (open orders; price/qty/trigger), Add more (positions), Exit (full or partial qty); all via
  the pipeline with confirmation; available via a visible menu button and keyboard, not only on hover.
- **Acceptance criteria:**
  1. Given Exit partial 4 of 10, then a 4-qty opposite order ticket opens prefilled.
  2. Given Modify on an order already modified 24 times, then Modify is disabled with the reason.
- **Data/Kite dependency:** `ModifyOrder`, `CancelOrder`, `GetOrderHistory`, `GetOrderTrades`, `PlaceOrder`.
- **Deviation from Streak:** Same menu on standalone pages (Streak: scalper only, per docs).
- **Priority hint:** Core

### FR-ORD-012 Instrument-row B / S buttons
- **Source features:** ORD-007
- **Requirement:** Every instrument row (watchlist, scanner results, alert snapshots, dashboard widgets) shall have B
  and S buttons that open the order ticket prefilled with that instrument and side; disabled with a tooltip for indices and
  non-tradable instruments; reachable by keyboard.
- **Acceptance criteria:**
  1. Given NIFTY 50 index row, then B/S are disabled with tooltip "Indices cannot be traded".
- **Data/Kite dependency:** see FR-ORD-004.
- **Deviation from Streak:** Keyboard access.
- **Priority hint:** Core

### FR-ORD-013 Trading-enabled gate and static IP
- **Source features:** ORD-004
- **Requirement:** Order capability shall be off by default. Enabling "Trading" requires the user to confirm their registered static
  IP(s) and acknowledge the rules (static IP mandatory, ≤ 10 OPS, personal use). The system shall detect an IP-related rejection
  from Kite and then disable trading and armed modes with an explanation. An optional public-IP check against a user-configured
  endpoint (off by default; no telemetry, NFR-04) compares the current egress IP to the registered list. Signal-only and paper
  modes never require trading to be enabled. Armed auto additionally requires FR-DEP-018.
- **Acceptance criteria:**
  1. Given trading disabled, then every B/S opens the ticket in read-only "preview" with a link to enable trading.
  2. Given a Kite rejection indicating the IP is not allowed, then trading is disabled and all armed deployments disarm.
- **Data/Kite dependency:** Kite order responses; optional user-configured IP echo URL.
- **Deviation from Streak:** Required by C-04 (Streak handles IP server-side).
- **Priority hint:** Core

### FR-ORD-014 Order and position state synchronisation
- **Source features:** ORD-001, ORD-002
- **Requirement:** The system shall maintain local order and position state from ticker order updates, reconcile with `GetOrders`
  and `GetPositions` on connect, every 30 s while orders are open, and on demand; differences are resolved in favour of Kite and
  logged.
- **Acceptance criteria:**
  1. Given an order placed from Kite web, then it appears in the order book within 30 s (or immediately via order updates).
- **Data/Kite dependency:** Ticker `OnOrderUpdate`, `GetOrders`, `GetPositions`.
- **Deviation from Streak:** Postbacks are not used (07 §A10).
- **Priority hint:** Core

### FR-ORD-015 Order placement outside the continuous session
- **Source features:** ORD-004, DEP-005
- **Requirement:** Order placement validity follows the market calendar phases (FR-DATA-001) on the app clock (FR-DATA-004):
  **pre-open order entry** (09:00–09:08 on a regular day): regular LIMIT and MARKET orders for NSE/BSE cash equities are
  accepted with the ticket banner "Pre-open session — order enters the opening auction"; 09:08–09:15: refused ("Pre-open
  matching in progress"); **continuous session**: all varieties; **closing** (15:30–15:40): refused; **post-closing**
  (15:40–16:00): **equity cash MARKET orders only**, at the closing price, banner "Post-closing session — market order at closing
  price"; futures orders are refused in pre-open and post-closing with the reason; any other time: refused with "Next session
  <phase> at <time> on <date>", unless the AMO enhancement of FR-ORD-004 is enabled, in which case the ticket offers variety
  `amo`. Deployment alerts follow FR-DEP-006 (late signals stay actionable in post-closing for equity cash). The pipeline stage 2
  (FR-ORD-005) enforces these rules for every source, including armed auto-execution.
- **Acceptance criteria:**
  1. Given 15:45 and an equity signal from the 15:15–15:30 candle, then "Review order" is enabled and the ticket shows the
     post-closing banner with order type locked to MARKET.
  2. Given 15:45 and a LIMIT equity order, then stage 2 refuses it with "Post-closing session accepts market orders only".
  3. Given 16:05 and AMO disabled, then the action is disabled with "Next session pre-open at 09:00 on <next trading day>".
  4. Given 09:05 and a NIFTY futures order, then it is refused with "Pre-open session is for equity cash only".
  5. Given a special session (FR-DATA-001 AC2), then the phases of that session apply instead of the regular-day times.
- **Data/Kite dependency:** `PlaceOrder` (variety `regular` / `amo`); FR-DATA-001.
- **Deviation from Streak:** Streak allows post-market placement 15:40–16:00 (A06 §3) — same here (RV-006); pre-open entry and
  explicit phase messages are additions; AMO remains an optional enhancement (Streak has none).
- **Priority hint:** Important

### FR-ORD-016 Price alerts via Kite alerts — Nice-to-have
- **Source features:** CHART-009, ORD-007
- **Requirement:** From the chart crosshair "+" menu (FR-CHART-011) and from instrument-row menus (FR-ORD-012) the user can
  create a **Kite price alert** (`simple` type, `LastTradedPrice` `>=` / `<=` a constant) that runs on Zerodha's servers and so
  fires while the app is closed; the Notifications view lists the user's alerts (Settings → Notifications → "Kite price alerts")
  with name, instrument, condition, status and a delete action, and shows triggered alerts from the alert history on start.
  The app never creates `ato` (order-placing) alerts. Kite's cap of 500 active alerts is shown as usage.
- **Acceptance criteria:**
  1. Given "+" at 1,249.30 on RELIANCE → "Create price alert", then `CreateAlert` is called once with `LastTradedPrice >= 1249.30`
     (or `<=` when below LTP) and the alert appears in the list.
  2. Given 500 active alerts, then creation is refused with "Kite allows 500 active alerts".
  3. Given an alert triggered while the app was closed, then on start a Notifications entry "Price alert hit: RELIANCE ≥ 1,249.30"
     exists.
- **Data/Kite dependency:** `CreateAlert`, `GetAlerts`, `DeleteAlerts`, `GetAlertHistory` (A07 §A8b); works on the Personal plan.
- **Deviation from Streak:** Streak's crosshair "+" offers alert or order (CHART-009); the alert is implemented with Kite's
  server-side alerts (RV-033).
- **Priority hint:** Nice-to-have

---

## FR module TG — Telegram bot (Enhancement beyond Streak; design inspired by freqtrade's Telegram RPC, no code reused)

Research: `docs/research/areas/08-freqtrade-telegram.md`. Library: `github.com/go-telegram/bot` (MIT). Freqtrade is GPL-3.0 — only feature design is borrowed.

### FR-TG-001 Telegram integration is optional and off by default
- **Source features:** DEP-009, DEP-010, ACCT-008 (alert channels; Telegram itself is beyond Streak)
- **Requirement:** The app shall include a Telegram integration that is disabled on first install and can be enabled from Settings → Telegram. When disabled, the app shall make no network calls to `api.telegram.org`.
- **Acceptance criteria:**
  1. Given a fresh install, when the app starts, then Telegram is disabled and no request is made to `api.telegram.org`.
  2. Given Telegram is enabled with valid credentials, when the user disables it, then polling stops within 25 s and no further messages are sent.
- **Data/Kite dependency:** None.
- **Deviation from Streak:** Enhancement (beyond Streak) — Streak delivers alerts only via web push and its mobile app; Telegram replaces the mobile channel for a local app.
- **Priority hint:** Core

### FR-TG-002 Secure storage of the bot token
- **Source features:** DEP-009, DEP-010, ACCT-008 (alert channels; Telegram itself is beyond Streak)
- **Requirement:** The bot token shall be stored in the OS secret store (Linux Secret Service / keyring). It shall be stored in no other place: not in plain config files, logs, crash reports or Telegram messages. The UI shall show it masked, with only the last 4 characters visible.
- **Acceptance criteria:**
  1. Given a saved token, when the config directory and logs are searched for the token string, then there are no matches.
  2. Given the secret store is unavailable, when the user tries to save a token, then the app refuses and explains why. It does not fall back to plaintext.
- **Data/Kite dependency:** OS keyring (the Go keyring library is still to be chosen; UNVERIFIED).
- **Deviation from Streak:** Enhancement (beyond Streak) — Streak delivers alerts only via web push and its mobile app; Telegram replaces the mobile channel for a local app.
- **Priority hint:** Core

### FR-TG-003 Guided setup and pairing
- **Source features:** DEP-009, DEP-010, ACCT-008 (alert channels; Telegram itself is beyond Streak)
- **Requirement:** Settings shall offer a guided setup:
  1. Instructions for creating a bot with @BotFather.
  2. Paste and validate the token (call `getMe` and show the bot username).
  3. **Pairing:** the app shows a one-time 6-digit code valid for 10 minutes. The user sends `/pair <code>` to the bot from the private chat or group/topic to be used.
  4. The app records that chat's `chat_id`, `topic_id` and the sender's `user_id` as the first authorized user.
  
  No unauthenticated command shall be answered outside an active pairing window.
- **Acceptance criteria:**
  1. Given pairing is active, when `/pair <correct code>` arrives, then `chat_id`, `topic_id` and `user_id` are saved and a confirmation is sent to that chat.
  2. Given pairing is not active, when `/pair` or any other command arrives from an unknown chat, then nothing is replied.
  3. Given 5 wrong codes, when another attempt arrives, then the pairing window is closed and the desktop shows a warning.
- **Data/Kite dependency:** Telegram `getMe` and `getUpdates`.
- **Deviation from Streak:** Enhancement (beyond Streak) — Streak delivers alerts only via web push and its mobile app; Telegram replaces the mobile channel for a local app.
- **Priority hint:** Core

### FR-TG-004 Authorization of every update
- **Source features:** DEP-009, DEP-010, ACCT-008 (alert channels; Telegram itself is beyond Streak)
- **Requirement:** Every inbound command **and every callback query** shall be processed only if all of these hold:
  - the chat id equals the configured chat;
  - the topic id equals the configured topic, if one is set;
  - the sender's user id is in the authorized-user list.
  
  The list shall always be non-empty once paired. Confirmation callbacks shall additionally be accepted only from the user who requested the action.
- **Acceptance criteria:**
  1. Given a group where user B is a member but not authorized, when B presses a Confirm button that user A requested, then nothing executes and the callback is answered with "Not authorized".
  2. Given a message from another chat, when it is a valid command, then it is ignored.
- **Data/Kite dependency:** Telegram updates.
- **Deviation from Streak:** Enhancement (beyond Streak) — Streak delivers alerts only via web push and its mobile app; Telegram replaces the mobile channel for a local app.
- **Priority hint:** Core

### FR-TG-005 Handling unauthorized messages
- **Source features:** DEP-009, DEP-010, ACCT-008 (alert channels; Telegram itself is beyond Streak)
- **Requirement:** Unauthorized commands shall get no chat reply. Callback queries are the exception: they are answered silently so the client spinner stops. Each rejection shall be written to the security log with chat id, user id and command name, but not the message body. More than 10 rejections in 10 minutes shall raise one in-app warning, and a Telegram `warning` to the authorized chat.
- **Acceptance criteria:**
  1. Given 11 unauthorized commands within 10 minutes, then exactly one in-app warning and one Telegram warning are produced, and none of the commands is executed.
- **Data/Kite dependency:** None.
- **Deviation from Streak:** Enhancement (beyond Streak) — Streak delivers alerts only via web push and its mobile app; Telegram replaces the mobile channel for a local app.
- **Priority hint:** Core

### FR-TG-006 Group and topic use
- **Source features:** DEP-009, DEP-010, ACCT-008 (alert channels; Telegram itself is beyond Streak)
- **Requirement:** The bot may be paired to a private chat (recommended) or to a group, optionally a forum topic. When the target is a group, Settings shall show a warning that all authorized users can act on the account and that every group member can read notifications. Settings shall allow adding and removing authorized user ids.
- **Acceptance criteria:**
  1. Given pairing from a group, when it completes, then the warning is shown and must be acknowledged before saving.
  2. Given a topic is configured, when a notification is sent, then it is posted in that topic.
- **Data/Kite dependency:** None.
- **Deviation from Streak:** Enhancement (beyond Streak) — Streak delivers alerts only via web push and its mobile app; Telegram replaces the mobile channel for a local app.
- **Priority hint:** Important

### FR-TG-007 Connection status and test message
- **Source features:** DEP-009, DEP-010, ACCT-008 (alert channels; Telegram itself is beyond Streak)
- **Requirement:** Settings shall show the Telegram connection state:
  - disabled;
  - connecting;
  - online (with last successful poll time);
  - error (with last error).
  
  A **Send test message** button shall be provided.
- **Acceptance criteria:**
  1. Given an invalid token, when the integration starts, then the status shows "error: unauthorized token" and the app keeps running normally.
  2. Given online, when Send test is clicked, then a test message arrives in the paired chat.
- **Data/Kite dependency:** None.
- **Deviation from Streak:** Enhancement (beyond Streak) — Streak delivers alerts only via web push and its mobile app; Telegram replaces the mobile channel for a local app.
- **Priority hint:** Core

### FR-TG-008 Lifecycle bound to the desktop app (offline behaviour)
- **Source features:** DEP-009, DEP-010, ACCT-008 (alert channels; Telegram itself is beyond Streak)
- **Requirement:** The bot shall run only while the Tradeloom desktop process is running. It shall use long polling (`getUpdates`); webhooks are not used. When the app is closed, suspended, or has no network, the bot is offline: no commands are processed, no notifications are sent, and Telegram keeps undelivered commands for at most 24 h. Settings and docs shall state this clearly, together with the fact that GTT protections at Zerodha remain active while offline. Telegram failures shall never block or stop trading.
- **Acceptance criteria:**
  1. Given the app is quit, when the user sends `/status`, then there is no response. The docs say this is expected.
  2. Given `api.telegram.org` is unreachable, when a signal fires, then the in-app/OS notification still appears and the engine's timing is unaffected.
- **Data/Kite dependency:** None.
- **Deviation from Streak:** Enhancement (beyond Streak) — Streak delivers alerts only via web push and its mobile app; Telegram replaces the mobile channel for a local app.
- **Priority hint:** Core

### FR-TG-009 Discard stale commands on start
- **Source features:** DEP-009, DEP-010, ACCT-008 (alert channels; Telegram itself is beyond Streak)
- **Requirement:** On every (re)connect, the app shall fetch and acknowledge pending updates **without executing them**. It shall then send one message listing the dropped commands (name and time received). Pending callback presses on old confirmation prompts shall also be discarded.
- **Acceptance criteria:**
  1. Given `/exit all` was sent while the app was closed, when the app starts, then no exit is placed and the startup message says "1 command received while offline was ignored: /exit all (08:02)".
- **Data/Kite dependency:** Telegram `getUpdates` offset.
- **Deviation from Streak:** Enhancement (beyond Streak) — Streak delivers alerts only via web push and its mobile app; Telegram replaces the mobile channel for a local app.
- **Priority hint:** Core

### FR-TG-010 Online/offline announcements
- **Source features:** DEP-009, DEP-010, ACCT-008 (alert channels; Telegram itself is beyond Streak)
- **Requirement:** The app shall send:
  - **`app_online`** when Telegram connects. It includes app version; per-mode deployment counts (live/paper); auto-exec armed state; kill-switch state; Kite session state; and dropped-command info (FR-TG-009).
  - **`app_offline`** on graceful quit or system suspend, within a 5 s grace period. It says that signals and alerts stop and GTTs remain.
  
  Crash or power loss cannot be announced; the docs shall say so.
- **Acceptance criteria:**
  1. Given a normal quit, then `app_offline` arrives before the process exits, unless the network is down.
  2. Given a crash, then on next start `app_online` includes "previous session ended unexpectedly at <time>".
- **Data/Kite dependency:** Kite session status.
- **Deviation from Streak:** Enhancement (beyond Streak) — Streak delivers alerts only via web push and its mobile app; Telegram replaces the mobile channel for a local app.
- **Priority hint:** Important

### FR-TG-011 Command menu and /help
- **Source features:** DEP-009, DEP-010, ACCT-008 (alert channels; Telegram itself is beyond Streak)
- **Requirement:** The app shall register its command list with Telegram (`setMyCommands`), so the client shows a menu. `/help` shall list commands grouped as Status, Account, Performance and Control. It shall omit commands disabled by current permissions and mark order-capable commands with "requires confirmation".
- **Acceptance criteria:**
  1. Given `allow_telegram_order_confirm` is off, when `/help` is sent, then signal confirmation is described as disabled.
  2. Given a permission changes, then the command menu is updated within 60 s.
- **Data/Kite dependency:** None.
- **Deviation from Streak:** Enhancement (beyond Streak) — Streak delivers alerts only via web push and its mobile app; Telegram replaces the mobile channel for a local app.
- **Priority hint:** Important

### FR-TG-012 Deployment status commands
- **Source features:** DEP-009, DEP-010, ACCT-008 (alert channels; Telegram itself is beyond Streak)
- **Requirement:** The bot shall provide:
  - `/status`: one block per active deployment with id, strategy, instrument, state, PAPER/LIVE tag, position qty and avg price, LTP, MTM P&L ₹ and %, SL and target, GTT status, time in state.
  - `/status <id…>`: the listed deployments only.
  - `/status table`: a monospaced table of id, instrument, state, side, qty and P&L.
  - `/status <id> detail`: the last evaluated value of each rule condition.
  
  Output longer than 4096 characters shall be split across messages.
- **Acceptance criteria:**
  1. Given 120 deployments, when `/status table` is sent, then the table arrives in at most 3 messages, none exceeding 4096 characters.
  2. Given an unknown id, then the reply is "Deployment <id> not found".
- **Data/Kite dependency:** Engine state, Kite ticks for LTP.
- **Deviation from Streak:** Enhancement (beyond Streak) — Streak delivers alerts only via web push and its mobile app; Telegram replaces the mobile channel for a local app.
- **Priority hint:** Core

### FR-TG-013 Account commands
- **Source features:** DEP-009, DEP-010, ACCT-008 (alert channels; Telegram itself is beyond Streak)
- **Requirement:** The bot shall provide:
  - `/funds`: equity available, used, net from Kite margins, or the paper ledger.
  - `/positions`: Kite net positions, each flagged as Tradeloom-managed or external.
  - `/holdings`: top 10 holdings by value, plus total value and P&L.
  
  Kite errors shall be reported as a short message, e.g. "Kite session expired — log in on the desktop".
- **Acceptance criteria:**
  1. Given the Kite token has expired, when `/positions` is sent, then the reply explains that login is required and no stack trace is shown.
- **Data/Kite dependency:** `GET /user/margins`, `GET /portfolio/positions`, `GET /portfolio/holdings`.
- **Deviation from Streak:** Enhancement (beyond Streak) — Streak delivers alerts only via web push and its mobile app; Telegram replaces the mobile channel for a local app.
- **Priority hint:** Core

### FR-TG-014 Orders and GTT commands
- **Source features:** DEP-009, DEP-010, ACCT-008 (alert channels; Telegram itself is beyond Streak)
- **Requirement:** The bot shall provide:
  - `/orders [id]`: today's orders, all or for one deployment, with status and rejection reason.
  - `/gtt`: active Tradeloom-created GTTs with trigger values.
- **Acceptance criteria:**
  1. Given a rejected order today, then `/orders` shows it with Kite's status message.
- **Data/Kite dependency:** `GET /orders`, `GET /gtt/triggers`.
- **Deviation from Streak:** Enhancement (beyond Streak) — Streak delivers alerts only via web push and its mobile app; Telegram replaces the mobile channel for a local app.
- **Priority hint:** Important

### FR-TG-015 Performance commands
- **Source features:** DEP-009, DEP-010, ACCT-008 (alert channels; Telegram itself is beyond Streak)
- **Requirement:** The bot shall provide these, all from the local trade journal, with `live|paper` and `long|short` filters where meaningful:
  - `/trades [n]` (default 10, max 50)
  - `/profit [n] [live|paper] [long|short]`: realised P&L, count, win rate, profit factor, expectancy, avg holding time, max drawdown, best and worst instrument
  - `/daily [n]` (default 7)
  - `/weekly [n]` (8, weeks start Monday)
  - `/monthly [n]` (6)
  - `/performance` (by instrument)
  - `/bystrategy`
  - `/exits` (by exit reason)
  - `/mix` (strategy × exit reason)
  - `/stats` (exit-reason wins/losses and average holding time of winners vs losers)
  
  Paper and live shall never be mixed unless the user explicitly asks for `all`.
- **Acceptance criteria:**
  1. Given both paper and live trades, when `/profit` is sent, then it shows live only by default and says so in the header.
  2. Given `/daily abc`, then the default (7) is used.
- **Data/Kite dependency:** Local journal (no Kite calls).
- **Deviation from Streak:** Enhancement (beyond Streak) — Streak delivers alerts only via web push and its mobile app; Telegram replaces the mobile channel for a local app.
- **Priority hint:** Important

### FR-TG-016 Engine and session information commands
- **Source features:** DEP-009, DEP-010, ACCT-008 (alert channels; Telegram itself is beyond Streak)
- **Requirement:** The bot shall provide:
  - `/health`: last tick time, ticker WebSocket state, last Kite REST success, notification queue depth, uptime.
  - `/session`: Kite login state, masked user id, token expiry time (06:00 IST), static-IP check result.
  - `/version`.
  - `/settings`: redacted mode, arming, kill switch, Telegram permissions, risk limits.
  - `/logs [n]`: WARN and above, max 50, secrets redacted.
- **Acceptance criteria:**
  1. Given `/settings`, then no API key, API secret, access token or bot token appears in the output.
- **Data/Kite dependency:** `GET /user/profile`, app state.
- **Deviation from Streak:** Enhancement (beyond Streak) — Streak delivers alerts only via web push and its mobile app; Telegram replaces the mobile channel for a local app.
- **Priority hint:** Important

### FR-TG-017 Attention, signals and alerts listing
- **Source features:** DEP-009, DEP-010, ACCT-008 (alert channels; Telegram itself is beyond Streak)
- **Requirement:** The bot shall provide:
  - `/attention`: deployments in ATTENTION, DAY_DONE or EXPIRED, with reasons.
  - `/signals`: pending signals with time left.
  - `/alerts`: active scanner and price alerts with last trigger.
- **Acceptance criteria:**
  1. Given a deployment in ATTENTION, then `/attention` shows it with the reason and the hint "resolve on desktop".
- **Data/Kite dependency:** Engine.
- **Deviation from Streak:** Enhancement (beyond Streak) — Streak delivers alerts only via web push and its mobile app; Telegram replaces the mobile channel for a local app.
- **Priority hint:** Important

### FR-TG-018 Refresh buttons with throttling
- **Source features:** DEP-009, DEP-010, ACCT-008 (alert channels; Telegram itself is beyond Streak)
- **Requirement:** Table-style replies shall carry an inline **Refresh** button that edits the message in place and appends "Updated HH:MM:SS IST". This applies to `/status table`, `/profit`, `/daily`, `/weekly`, `/monthly`, `/performance`, `/funds`, `/positions` and `/count`.
  - Presses shall be throttled to one refresh per message per 3 s. A press inside that window is answered with "Too fast".
  - "Message not modified" errors shall be ignored.
  - The buttons can be turned off in Settings.
- **Acceptance criteria:**
  1. Given two Refresh presses 1 s apart, then only one Kite or engine query is made and the second press shows "Too fast".
- **Data/Kite dependency:** As per the underlying command.
- **Deviation from Streak:** Enhancement (beyond Streak) — Streak delivers alerts only via web push and its mobile app; Telegram replaces the mobile channel for a local app.
- **Priority hint:** Nice-to-have

### FR-TG-019 Custom reply keyboard
- **Source features:** DEP-009, DEP-010, ACCT-008 (alert channels; Telegram itself is beyond Streak)
- **Requirement:** Settings shall allow a custom reply keyboard (rows of command buttons). Only argument-less **read-only** commands, plus `/pause` and `/kill`, may be placed on it. Any other entry is rejected **at save time** with the offending entries named. Unlike freqtrade, the app does not fail at startup.
- **Acceptance criteria:**
  1. Given the user adds `/exit all` to the keyboard, when saving, then validation fails and names that entry.
- **Data/Kite dependency:** None.
- **Deviation from Streak:** Enhancement (beyond Streak) — Streak delivers alerts only via web push and its mobile app; Telegram replaces the mobile channel for a local app.
- **Priority hint:** Nice-to-have

### FR-TG-020 Confirmation framework for risky actions
- **Source features:** DEP-009, DEP-010, ACCT-008 (alert channels; Telegram itself is beyond Streak)
- **Requirement:** Every command of kind **O**, and the global S-commands marked "confirm" in §F2, shall send a confirmation prompt.
  - **Prompt content:** what will happen, listing instruments, qty, side, LTP, estimated P&L or value, and PAPER/LIVE.
  - **Buttons:** **Confirm** and **Cancel**.
  - **Callback data:** an opaque single-use nonce of 64 bytes or less, mapped server-side to the action, the requesting user id and an expiry (default 60 s, configurable 15–300 s).
  - **On Confirm:**
    1. re-check authorization and the requesting user;
    2. re-validate live state: session, kill switch, deployment state, position still open, price drift within tolerance;
    3. execute **once**;
    4. answer the callback;
    5. edit the prompt to show the outcome, with the Kite order id or the error.
  - **Expired, used or cancelled prompts** are edited to say so, and their buttons are removed.
- **Acceptance criteria:**
  1. Given a prompt pressed twice quickly, then exactly one order is placed.
  2. Given a prompt pressed after 61 s, then nothing executes and the message says "expired".
  3. Given the position was already closed by GTT before Confirm, then nothing is sent to Kite and the reply says "already flat".
- **Data/Kite dependency:** Engine; Kite order APIs.
- **Deviation from Streak:** Enhancement (beyond Streak) — Streak delivers alerts only via web push and its mobile app; Telegram replaces the mobile channel for a local app.
- **Priority hint:** Core

### FR-TG-021 Confirm notification-based signals from Telegram
- **Source features:** DEP-009, DEP-010, ACCT-008 (alert channels; Telegram itself is beyond Streak)
- **Requirement:** When `allow_telegram_order_confirm` is on (default **off**), each `signal` notification shall carry **Place** and **Skip** buttons. Place runs the same order path as the in-app confirmation: static IP, ≤10 OPS throttle, `market_protection` for MARKET, product per strategy, then GTT protection after fill. It also checks:
  - the signal is still pending and not expired;
  - LTP drift from the signal price is within the strategy's tolerance (default 0.5 %);
  - sufficient margin;
  - valid Kite session;
  - kill switch off;
  - auto-exec not already handling it.
  
  Confirming from either the desktop or Telegram resolves the signal everywhere. In paper mode, Place creates a simulated order.
- **Acceptance criteria:**
  1. Given the setting is off, then signal messages have no buttons and say "confirm in app".
  2. Given a signal confirmed on the desktop, when Place is pressed in Telegram, then the reply is "already confirmed" and no second order is placed.
  3. Given LTP moved 1 % against a 0.5 % tolerance, then the order is not placed and the user is told the reason.
- **Data/Kite dependency:** `POST /orders/regular`, `GET /user/margins`, GTT APIs, static-IP check.
- **Deviation from Streak:** Enhancement (beyond Streak) — Streak delivers alerts only via web push and its mobile app; Telegram replaces the mobile channel for a local app.
- **Priority hint:** Important

### FR-TG-022 Exit positions remotely
- **Source features:** DEP-009, DEP-010, ACCT-008 (alert channels; Telegram itself is beyond Streak)
- **Requirement:** `/exit <id>` and `/exit all` shall, after confirmation (FR-TG-020):
  1. cancel the deployment's protective GTT and pending SL orders;
  2. place the exit order (MARKET with protection, or LIMIT at LTP ± configurable buffer);
  3. report the fill.
  
  `/exit` with no args shows a picker of open managed positions, and each pick then goes through confirmation. External (non-Tradeloom) positions are never touched. Exits can be disabled in Settings (`allow_telegram_exit`, default on).
- **Acceptance criteria:**
  1. Given 3 managed and 1 external position, when `/exit all` is confirmed, then exactly 3 exit orders are placed, and the reply lists them and notes that the external one was skipped.
  2. Given GTT cancellation fails, then the exit is not placed, the deployment goes to ATTENTION, and an always-on alert is sent.
- **Data/Kite dependency:** `DELETE /gtt/triggers/{id}`, `POST /orders`, `GET /orders`.
- **Deviation from Streak:** Enhancement (beyond Streak) — Streak delivers alerts only via web push and its mobile app; Telegram replaces the mobile channel for a local app.
- **Priority hint:** Core

### FR-TG-023 Cancel a pending entry order
- **Source features:** DEP-009, DEP-010, ACCT-008 (alert channels; Telegram itself is beyond Streak)
- **Requirement:** `/cancel <id>` shall, after confirmation, cancel the deployment's open **entry** order and return the deployment to ARMED or DAY_DONE according to its rules. Cancelling protective exits or GTTs from Telegram shall not be possible.
- **Acceptance criteria:**
  1. Given a deployment in IN_POSITION with a GTT, when `/cancel <id>` is sent, then the reply explains that only pending entry orders can be cancelled remotely.
- **Data/Kite dependency:** `DELETE /orders/regular/{id}`.
- **Deviation from Streak:** Enhancement (beyond Streak) — Streak delivers alerts only via web push and its mobile app; Telegram replaces the mobile channel for a local app.
- **Priority hint:** Important

### FR-TG-024 Pause and resume entries
- **Source features:** DEP-009, DEP-010, ACCT-008 (alert channels; Telegram itself is beyond Streak)
- **Requirement:** - `/pause` (global, confirm) and `/pause <id>` (one deployment) shall stop new entries while open positions stay managed.
  - `/resume [id]` shall be available only if `allow_telegram_resume` is on (default off). It needs confirmation and is refused while the kill switch is engaged.
  - Pause state shall persist across app restarts. This differs from freqtrade.
- **Acceptance criteria:**
  1. Given a global pause, when an entry condition becomes true, then no signal or order is produced, while SL/target exits still execute.
  2. Given the app restarts, then the pause is still in effect.
- **Data/Kite dependency:** Engine.
- **Deviation from Streak:** Enhancement (beyond Streak) — Streak delivers alerts only via web push and its mobile app; Telegram replaces the mobile channel for a local app.
- **Priority hint:** Core

### FR-TG-025 Kill switch from Telegram
- **Source features:** DEP-009, DEP-010, ACCT-008 (alert channels; Telegram itself is beyond Streak)
- **Requirement:** `/kill` shall be available to every authorized user and needs one confirmation. It engages the same kill switch as the desktop:
  - disarm auto-exec;
  - pause all deployments;
  - cancel all open entry orders;
  - then, per the kill-switch setting (default **keep protections**: leave positions under existing GTT/SL — OQ-45 decision), either leave them protected or, only if the user changed the setting, square off all Tradeloom-managed positions.
  
  It then reports a per-action summary. **Disengaging the kill switch shall be desktop-only**, unless `allow_telegram_kill_reset` is enabled (default off).
- **Acceptance criteria:**
  1. Given `/kill` is confirmed, then a `kill_switch` notification lists every cancelled order and exit placed, and `/status` shows all deployments paused.
  2. Given the kill switch is engaged, when `/resume` or `/arm` is sent, then it is refused.
- **Data/Kite dependency:** Kite orders, GTT and positions APIs.
- **Deviation from Streak:** Enhancement (beyond Streak) — Streak delivers alerts only via web push and its mobile app; Telegram replaces the mobile channel for a local app.
- **Priority hint:** Core

### FR-TG-026 Auto-execution arming rules
- **Source features:** DEP-009, DEP-010, ACCT-008 (alert channels; Telegram itself is beyond Streak)
- **Requirement:** - `/disarm` shall always be allowed (with confirmation).
  - `/arm` shall be refused unless `allow_remote_arming` is true in Settings (default false; changing it is desktop-only and needs re-entering the app's confirmation).
  - When allowed, `/arm` additionally requires: static-IP check passing, a valid Kite session, kill switch off, inside market hours. The user must reply with a one-time 4-digit code shown in the prompt within 60 s.
  
  Every arm and disarm shall be audit-logged and notified.
- **Acceptance criteria:**
  1. Given the default settings, when `/arm` is sent, then the reply is "Remote arming is disabled; arm from the desktop" and nothing changes.
  2. Given remote arming is allowed and the static-IP check fails, then arming is refused with the reason.
- **Data/Kite dependency:** Static-IP check, `GET /user/profile`.
- **Deviation from Streak:** Enhancement (beyond Streak) — Streak delivers alerts only via web push and its mobile app; Telegram replaces the mobile channel for a local app.
- **Priority hint:** Core

### FR-TG-027 Paper/live labelling
- **Source features:** DEP-009, DEP-010, ACCT-008 (alert channels; Telegram itself is beyond Streak)
- **Requirement:** Every trade-related message and every confirmation prompt shall carry a prominent `PAPER` or `LIVE` tag. Paper actions shall never call Kite order endpoints.
- **Acceptance criteria:**
  1. Given a paper deployment, when its exit is confirmed from Telegram, then the journal records a simulated fill and no Kite order call is logged.
- **Data/Kite dependency:** None.
- **Deviation from Streak:** Enhancement (beyond Streak) — Streak delivers alerts only via web push and its mobile app; Telegram replaces the mobile channel for a local app.
- **Priority hint:** Core

### FR-TG-028 Notification catalogue and loudness levels
- **Source features:** DEP-009, DEP-010, ACCT-008 (alert channels; Telegram itself is beyond Streak)
- **Requirement:** Each notification type in §F3 shall have its own level: **on** (sound), **silent** (`disable_notification`) or **off**. `exit` and `exit_fill` shall also be configurable per exit reason, with a "default for other reasons" entry. These types are forced on and cannot be set below **on**: `exit_cancel`, `state_attention`, `kill_switch`, `error`, `session` expiry.
  
  | Defaults | Types |
  |---|---|
  | on | `signal`, `entry_fill`, `exit_fill`, `risk_guard*`, `warning`, `session` |
  | silent | `entry`, `exit`, `state_change`, `backtest_done`, `daily_digest` |
  | off | `strategy_msg` |
  | per scanner | `scanner_alert` |
- **Acceptance criteria:**
  1. Given `exit` = {stop_loss: on, default: silent}, when a target exit occurs, then it is sent silently, while a stop-loss exit is sent with sound.
  2. Given `kill_switch` is set to off in a hand-edited config, then the app treats it as on and logs a warning.
- **Data/Kite dependency:** None.
- **Deviation from Streak:** Enhancement (beyond Streak) — Streak delivers alerts only via web push and its mobile app; Telegram replaces the mobile channel for a local app.
- **Priority hint:** Core

### FR-TG-029 Message formatting
- **Source features:** DEP-009, DEP-010, ACCT-008 (alert channels; Telegram itself is beyond Streak)
- **Requirement:** Messages shall use Telegram HTML parse mode, with all dynamic text escaped. Formatting rules:
  - times in IST (`HH:MM:SS`, plus a date when not today);
  - money as ₹ with Indian digit grouping;
  - quantities as integers (lots shown for futures);
  - instruments as `EXCHANGE:TRADINGSYMBOL`;
  - a first line that states the event type and deployment id.
  
  Output over 4096 characters shall be split at line boundaries, and tables shall go in `<pre>` blocks.
- **Acceptance criteria:**
  1. Given a strategy name containing `<` or `&`, then the message renders correctly with no parse error.
  2. Given ₹1234567.5, then it is shown as ₹12,34,567.50.
- **Data/Kite dependency:** None.
- **Deviation from Streak:** Enhancement (beyond Streak) — Streak delivers alerts only via web push and its mobile app; Telegram replaces the mobile channel for a local app.
- **Priority hint:** Core

### FR-TG-030 Outbound rate limiting and retries
- **Source features:** DEP-009, DEP-010, ACCT-008 (alert channels; Telegram itself is beyond Streak)
- **Requirement:** The sender shall:
  - enforce about 1 message/s per private chat and at most 20/min per group, with bursts allowed up to Telegram's tolerance;
  - honour `retry_after` on HTTP 429;
  - retry network errors up to 3 times with back-off (1 s, 2 s, 4 s);
  - drop a message after that and log it.
  
  When the queue backs up, non-critical messages of the same type and deployment (e.g. several `scanner_alert`) shall be **coalesced** into one summary. Critical types (always-on list in FR-TG-028) are never coalesced or dropped before 3 retries.
- **Acceptance criteria:**
  1. Given 50 scanner alerts within 10 s to a group, then no 429 loop occurs and the user receives at most 20 messages that minute, including a coalesced summary.
  2. Given a 429 with `retry_after`=5, then the next send waits at least 5 s.
- **Data/Kite dependency:** None.
- **Deviation from Streak:** Enhancement (beyond Streak) — Streak delivers alerts only via web push and its mobile app; Telegram replaces the mobile channel for a local app.
- **Priority hint:** Core

### FR-TG-031 Non-blocking, decoupled notification pipeline
- **Source features:** DEP-009, DEP-010, ACCT-008 (alert channels; Telegram itself is beyond Streak)
- **Requirement:** - **Typed events.** The engine shall publish typed notification events (Go structs per type) to a notifier bus. Telegram, in-app inbox and OS notifications are independent subscribers.
  - **Telegram goroutine.** The Telegram subscriber shall consume from a **bounded** buffered channel on its own goroutine.
  - **Overflow.** A full channel shall never block the engine. The oldest non-critical message is dropped and counted.
  - **Commands.** Inbound commands shall call a transport-neutral command service, the same one the Wails UI uses, so business rules live in one place.
  - **Shutdown.** The queue gets 5 s to drain.
- **Acceptance criteria:**
  1. Given Telegram is blocked (network hang), when 10,000 ticks and 200 events occur, then engine tick latency is unchanged and the queue-drop counter is visible in `/health` and in the app.
- **Data/Kite dependency:** None.
- **Deviation from Streak:** Enhancement (beyond Streak) — Streak delivers alerts only via web push and its mobile app; Telegram replaces the mobile channel for a local app.
- **Priority hint:** Core

### FR-TG-032 Inbound command rate limiting
- **Source features:** DEP-009, DEP-010, ACCT-008 (alert channels; Telegram itself is beyond Streak)
- **Requirement:** Inbound commands shall be limited to 20 per minute per authorized user. Excess commands get a single "slow down" reply per minute and are not executed. O-kind commands are limited to 5 confirmations per minute.
- **Acceptance criteria:**
  1. Given 25 `/status` commands within a minute, then 20 are answered and one "slow down" message is sent.
- **Data/Kite dependency:** Protects the Kite REST budget.
- **Deviation from Streak:** Enhancement (beyond Streak) — Streak delivers alerts only via web push and its mobile app; Telegram replaces the mobile channel for a local app.
- **Priority hint:** Important

### FR-TG-033 Daily login reminder and session notices
- **Source features:** DEP-009, DEP-010, ACCT-008 (alert channels; Telegram itself is beyond Streak)
- **Requirement:** While the app is running, the bot shall:
  - send a `session` reminder at a configurable time (default 08:45 IST on trading days) if there is no valid Kite session. It states that login must be done on the desktop and that deployments stay idle until then;
  - notify at 06:00 IST that the token has expired, if the app was running overnight;
  - notify on a successful login, and on any `TokenException` during the day.
  
  Telegram shall never offer or accept credentials, TOTP or `request_token`.
- **Acceptance criteria:**
  1. Given no login by 08:45 on a trading day, then one reminder is sent, and another at 09:10 if still not logged in.
  2. Given a holiday per the exchange calendar, then no reminder is sent.
- **Data/Kite dependency:** Kite session state; trading-holiday calendar.
- **Deviation from Streak:** Enhancement (beyond Streak) — Streak delivers alerts only via web push and its mobile app; Telegram replaces the mobile channel for a local app.
- **Priority hint:** Core

### FR-TG-034 Scanner and price alerts to Telegram
- **Source features:** DEP-009, DEP-010, ACCT-008 (alert channels; Telegram itself is beyond Streak)
- **Requirement:** Each scanner alert and price alert shall have a "send to Telegram" toggle (default on once Telegram is enabled). A message shall include scanner name, instrument, timeframe, trigger candle time and price.
  - A repeated trigger for the same scanner and instrument within one candle is sent once.
  - More than N alerts (default 10) from one scan run are sent as one summary message listing instruments.
- **Acceptance criteria:**
  1. Given one scan run matching 25 instruments, then a single summary message lists all 25, chunked if over 4096 characters.
- **Data/Kite dependency:** Scanner engine and ticks.
- **Deviation from Streak:** Enhancement (beyond Streak) — Streak delivers alerts only via web push and its mobile app; Telegram replaces the mobile channel for a local app.
- **Priority hint:** Important

### FR-TG-035 Strategy custom messages
- **Source features:** DEP-009, DEP-010, ACCT-008 (alert channels; Telegram itself is beyond Streak)
- **Requirement:** Strategies may define a "notify" action with a text template. It is delivered as `strategy_msg` (default off), deduplicated per deployment per candle, and limited to 500 characters.
- **Acceptance criteria:**
  1. Given the same notify action firing on every tick within one candle, then only one message is sent for that candle.
- **Data/Kite dependency:** Engine.
- **Deviation from Streak:** Enhancement (beyond Streak) — Streak delivers alerts only via web push and its mobile app; Telegram replaces the mobile channel for a local app.
- **Priority hint:** Nice-to-have

### FR-TG-036 End-of-day digest
- **Source features:** DEP-009, DEP-010, ACCT-008 (alert channels; Telegram itself is beyond Streak)
- **Requirement:** At a configurable time (default 15:45 IST on trading days), the bot shall send `daily_digest`, which `/digest` also returns on demand:
  - realised and unrealised P&L (live and paper separately);
  - trades count and win rate;
  - deployments by state;
  - open carry-forward positions with their GTTs;
  - warnings of the day;
  - a reminder that the token expires at 06:00.
- **Acceptance criteria:**
  1. Given CNC positions held overnight, then the digest lists them with their active GTT triggers.
- **Data/Kite dependency:** Journal, Kite positions and GTT.
- **Deviation from Streak:** Enhancement (beyond Streak) — Streak delivers alerts only via web push and its mobile app; Telegram replaces the mobile channel for a local app.
- **Priority hint:** Nice-to-have

### FR-TG-037 Audit log of remote actions
- **Source features:** DEP-009, DEP-010, ACCT-008 (alert channels; Telegram itself is beyond Streak)
- **Requirement:** Every Telegram-originated S or O action shall be recorded in the app's audit log and shown in the desktop activity view, flagged "via Telegram". This covers the request, the confirmation or expiry, and the execution result. Each record holds timestamp, user id, command, parameters, outcome and Kite order ids.
- **Acceptance criteria:**
  1. Given an exit confirmed from Telegram, then the desktop activity view shows "Exit #12 via Telegram by <user> — order 2510…".
- **Data/Kite dependency:** None.
- **Deviation from Streak:** Enhancement (beyond Streak) — Streak delivers alerts only via web push and its mobile app; Telegram replaces the mobile channel for a local app.
- **Priority hint:** Core

### FR-TG-038 Error handling for commands
- **Source features:** DEP-009, DEP-010, ACCT-008 (alert channels; Telegram itself is beyond Streak)
- **Requirement:** A failing command shall reply with a one-line, user-safe error. It shall not include stack traces or secrets. Full details go to the log. A panic in a handler shall be recovered and must not stop polling or the engine. Validation errors (bad id, bad number) reply with usage help.
- **Acceptance criteria:**
  1. Given a handler panics, then the user receives "Internal error — see app log", polling continues, and the next command works.
- **Data/Kite dependency:** None.
- **Deviation from Streak:** Enhancement (beyond Streak) — Streak delivers alerts only via web push and its mobile app; Telegram replaces the mobile channel for a local app.
- **Priority hint:** Core

### FR-TG-039 Redaction
- **Source features:** DEP-009, DEP-010, ACCT-008 (alert channels; Telegram itself is beyond Streak)
- **Requirement:** No Telegram message shall contain any of the following:
  - API key or secret;
  - access token;
  - bot token;
  - full Kite user id (mask it as `AB***12`);
  - full account balances in group chats when "hide amounts in groups" is on (default on for groups).
- **Acceptance criteria:**
  1. Given a group chat with hide-amounts on, when `/funds` is sent, then amounts show as "•••" with only percentages.
- **Data/Kite dependency:** None.
- **Deviation from Streak:** Enhancement (beyond Streak) — Streak delivers alerts only via web push and its mobile app; Telegram replaces the mobile channel for a local app.
- **Priority hint:** Important

### FR-TG-040 Reconcile command
- **Source features:** DEP-009, DEP-010, ACCT-008 (alert channels; Telegram itself is beyond Streak)
- **Requirement:** `/reconcile <id>` shall re-read the deployment's orders, positions and GTTs from Kite and update local state. Any inconsistency moves the deployment to ATTENTION; the app does not guess. It shall report what changed.
- **Acceptance criteria:**
  1. Given a position manually closed in Kite, when `/reconcile <id>` is sent, then the deployment's position is set to flat, the orphan GTT is reported, and the deployment moves to ATTENTION.
- **Data/Kite dependency:** `GET /orders`, `GET /portfolio/positions`, `GET /gtt/triggers`.
- **Deviation from Streak:** Enhancement (beyond Streak) — Streak delivers alerts only via web push and its mobile app; Telegram replaces the mobile channel for a local app.
- **Priority hint:** Nice-to-have

### FR-TG-041 Go Telegram library
- **Source features:** DEP-009, DEP-010, ACCT-008 (alert channels; Telegram itself is beyond Streak)
- **Requirement:** The integration shall use **`github.com/go-telegram/bot`** (MIT, v1.27.0 or later at implementation time, Bot API 10.3, no transitive dependencies). It is used with long polling, context-based cancellation tied to app shutdown, and callback-query routing. The library version and licence shall be recorded in the third-party notices.
- **Acceptance criteria:**
  1. Given `go mod graph`, then the Telegram integration adds no transitive modules beyond the library itself.
  2. Given app shutdown, then the polling goroutine exits within the context deadline.
- **Data/Kite dependency:** None.
- **Deviation from Streak:** Enhancement (beyond Streak) — Streak delivers alerts only via web push and its mobile app; Telegram replaces the mobile channel for a local app.
- **Priority hint:** Core

### FR-TG-042 No discretionary manual entry from Telegram (v1)
- **Source features:** DEP-009, DEP-010, ACCT-008 (alert channels; Telegram itself is beyond Streak)
- **Requirement:** v1 shall not provide a freqtrade-style "force entry" (buy or short any instrument from chat). New positions may only originate from deployed strategies, via signal confirmation (FR-TG-021) or armed auto-exec.
- **Acceptance criteria:**
  1. Given `/buy INFY`, then the reply is "Not supported — use the desktop".
- **Data/Kite dependency:** None.
- **Deviation from Streak:** Enhancement (beyond Streak) — Streak delivers alerts only via web push and its mobile app; Telegram replaces the mobile channel for a local app.
- **Priority hint:** Nice-to-have


## 7. Deliberate deviations from Streak

Regenerated from the full **Deviation from Streak** field of every §6 FR whose field is not "none". "Capability dropped?" = Y when a Streak capability is not reproduced (the Replacement column names the closest equivalent or the reason); N when behaviour merely differs or is an addition. Options, MCX and currency capabilities are not listed here — they are out of scope as a whole (§1, §10).

| FR | Deviation | Capability dropped? (Y/N) | Replacement |
|---|---|---|---|
| FR-SHELL-001 | Browser routes become in-app views; Streak's ticker hiding below the `lg` breakpoint is replaced by FR-SHELL-004 rule. Minimum width 1280 px per NFR-11. | N | — |
| FR-SHELL-002 | Streak's "Options Trader" entry is dropped (options out of scope, §10 SHELL-002-opt). Labels use our own wording ("Equity Builder"/"New Scanner"). Collapsible sidebar added for small windows (NFR-11). | Y | Options Builder entry dropped (scope); see §10 SHELL-002-opt |
| FR-SHELL-003 | Badge semantics were unobservable in Streak; we define them explicitly. Bell unread count is an addition (Streak shows no count). | N | — |
| FR-SHELL-004 | Pills are clickable (Streak bug §3.2 #28: they look like links but are inert). Missing data shown as "—" (bug #19). | N | — |
| FR-SHELL-005 | Streak shows only a dot with "Live" or an unlabelled amber dot and no market-closed signal (bug §3.2 #27). We label every state, add session and market phase (C-02, C-03). | N | — |
| FR-SHELL-006 | Unread count added (see FR-SHELL-003). | N | — |
| FR-SHELL-007 | Replay/reset added (Streak open question). Tour copy is our own. Escape closes. The Options Builder tour is dropped with the options scope (§10 SHELL-007-opt). | Y | Options Builder tour dropped (scope); §10 SHELL-007-opt |
| FR-SHELL-008 | Applies to the scanner builder too (bug §3.2 #15) and to in-app navigation — Streak guards only the browser `beforeunload`; leaving a dirty draft through its sidebar discards it silently (REVIEW-2 #28). Replaces the browser beforeunload prompt; offers Save directly. | N | — |
| FR-SHELL-009 | WARNING level and inline duplication of field errors added; errors no longer rely on a toast alone (§3.2 note on toast-only failures). Errors persist until dismissed. | N | — |
| FR-SHELL-010 | Help points to built-in help, not an external site; Settings item added (Streak has no settings page; the desktop app needs one for Kite credentials, trading enablement, data storage). | N | — |
| FR-SHELL-011 | Charts explicitly follow the theme (unknown in Streak); contrast requirement fixes the 404 contrast bug (§3.2 #26). Follow-system option added. | N | — |
| FR-SHELL-012 | Contrast bug fixed (§3.2 #26); item-specific messages added. | N | — |
| FR-SHELL-013 | Help is bundled offline, not an external site; "coming soon" placeholders forbidden (§3.2 #31). | N | — |
| FR-SHELL-014 | Holdings empty-state wording bug fixed (§3.2 #25); explicit loading/error states added. | N | — |
| FR-SHELL-015 | Fixes §3.2 #8, #16, #20 (Escape ignored, missing ×/Cancel); NFR-11 keyboard requirement. | N | — |
| FR-SHELL-016 | Fixes inconsistent "Last Scan" formats (§3.2 #14). | N | — |
| FR-SHELL-017 | Streak updates itself server-side; a packaged desktop app needs an opt-in check (NFR-02, NFR-04). | N | — |
| FR-SHELL-018 | Streak has no user-visible logs; required by NFR-10 and NFR-04. | N | — |
| FR-SHELL-019 | Streak has no shortcut list or remapping (FEATURES §3.3); all bindings are ours. | N | — |
| FR-SHELL-020 | Streak shows a risk-disclosure acknowledgement once per login session (REVIEW-2 N9, text not captured); ours is written fresh and also gates arming (NFR-05). | N | — |
| FR-HOME-002 | Explicit overall-P&L definition plus a day's-change line; empty state message instead of zeros; card navigates to Holdings (Streak: not clickable). | N | — |
| FR-HOME-003 | Counters are links; "Closed" defined explicitly (open question in catalogue). | N | — |
| FR-HOME-004 | Live evaluation runs only while the app is running (C-03); results are computed locally, so after a fresh install the first results appear only after backfill of the universe (progress shown, FR-HOME-005). The featured set is ours (tagged in the library), not Streak's first 20 scanner names (RV-020). | N | — |
| FR-HOME-005 | Chart action keyboard-reachable (FR-SHELL-015); explicit preparing/no-match states; option rows dropped with the options scope (§10 HOME-005-opt). | Y | Option rows dropped (scope); §10 HOME-005-opt |
| FR-HOME-006 | Results computed locally; one cache shared with Discover (RV-011); featured set configurable and tagged in our library (Streak rotates its own picks, RV-020). | N | — |
| FR-HOME-007 | Explicit daily-return definition (unspecified in Streak). | N | — |
| FR-HOME-008 | Streak refreshes server-side (docs: after 21:00); we refresh locally after 16:00 or on start (C-03). | N | — |
| FR-CHART-001 | Streak embeds TradingView (and ChartIQ next to conditions, Conflicts §6 #23); we use a single own-adapter component with a library chosen per C-12. Library note: `[Either]`. | N | — |
| FR-CHART-002 | none. Library note: `[Either]` (header is our own React component in both options). | N | — |
| FR-CHART-003 | Data stored locally and lazily backfilled; first view of a long history may take seconds (3 req/s). Library note: `[Either]`. | N | — |
| FR-CHART-004 | none (10 Minutes included per Conflicts §6 #8). Library note: `[Either]`. | N | — |
| FR-CHART-005 | Escape closes (§3.2 #8); BFO futures included so SENSEX/BANKEX futures are chartable; option contracts are not searchable (options out of scope, §10 CHART-005-opt). Library note: `[Either]` (own modal). | Y | Option contracts not searchable (scope); §10 CHART-005-opt |
| FR-CHART-007 | Persistence scope defined explicitly (open in catalogue); Clear confirms. Library note: `[Full-TV]` provides layout save/load APIs; `[Open+Own]` requires our own serialisation of studies and drawings. | N | — |
| FR-CHART-008 | Escape closes. Library note: `[Full-TV]` ships these studies; `[Open+Own]` requires us to implement all 107 studies (large effort; shared with EQB indicator engine where overlapping) and the dialog. | N | — |
| FR-CHART-009 | Invalid inputs rejected (Streak §3.2 #7 accepts invalid periods in builder dialogs; same rule applied here). Library note: `[Full-TV]` native; `[Open+Own]` our own legend overlay, dialogs and pane management. | N | — |
| FR-CHART-010 | Variant lists beyond those observed are our proposed minimum set (Streak's full TV list was not transcribed; see open questions); Alt replaces ⌥. Library note: `[Full-TV]` native (all ~80 TV tools); `[Open+Own]` every tool must be built on the library's primitive/plugin API — scope the variant set accordingly. | N | — |
| FR-CHART-011 | A/L toggles always visible (Streak showed them only with the panel collapsed). Library note: `[Either]` (crosshair/scales native in both; "+" order button `[Full-TV]` native / `[Open+Own]` custom overlay). | N | — |
| FR-CHART-012 | Opens the order ticket instead of a possibly one-click order (NFR-05 safety); "—" instead of 0.00 (§3.2 #19); hidden in signal-only mode (C-04). Library note: `[Full-TV]` has native buy/sell buttons via its broker API; `[Open+Own]` React overlay. | N | — |
| FR-CHART-013 | Symbol shown as `NSE:RELIANCE` (Kite convention) instead of `NSE_RELIANCE`; always via order ticket (NFR-05). Library note: `[Full-TV]` native trading-terminal menu via broker adapter; `[Open+Own]` our own menu. | N | — |
| FR-CHART-014 | Every modify/cancel is confirmed (NFR-05). Library note: `[Full-TV]` native order/position lines; `[Open+Own]` custom overlay primitives. | N | — |
| FR-CHART-015 | Tab contents beyond Symbol were not observed in Streak; contents above are our definition. Library note: `[Full-TV]` native dialog; `[Open+Own]` our own dialog mapping to library options. | N | — |
| FR-CHART-016 | Expired-contract fallback defined. Library note: `[Either]`. | N | — |
| FR-CHART-017 | Streak opens a new browser tab (`/chartiq?seg_sym=…`); Wails v2 supports a single window (C-13), so the chart opens as an in-app view with back navigation. Library note: `[Either]`. | N | In-app full-view chart replaces the browser tab (C-13) |
| FR-ANLY-001 | Streak's Option Chain, OI, PCR and ATM IV/Straddle tabs are dropped with the options scope (§10 ANLY-006/007/008/009/012 and ANLY-001-opt). Non-F&O and futures tab sets defined explicitly. | Y | Option Chain / OI / PCR / IV tabs → §10 |
| FR-ANLY-002 | Resizable divider added (fixed height in Streak). | N | — |
| FR-ANLY-003 | Streak's vote composition is unpublished; we define our own 27-vote set (same count for parity). Latency < 2 s instead of Streak's documented 6–7 min lag. | N | — |
| FR-ANLY-004 | Streak shows S3–R3 without P and does not document the pivot period; we show P and define periods. Thresholds defined explicitly. | N | — |
| FR-ANLY-005 | Event rows navigate the chart (not clickable in Streak); event definitions and tooltip texts are ours (Streak shows an explanatory tooltip per event row, REVIEW-2 N1, with its own copy). | N | — |
| FR-ANLY-010 | Required because Kite offers no breadth history (C-03, C-08); history exists only from first install plus what Kite minute candles allow. OI/ATM recording is dropped with the options scope (§10 ANLY-010-opt). | Y | OI / ATM recording → §10 ANLY-010-opt |
| FR-ANLY-011 | Constituents user-imported (C-09); history only as recorded/backfilled. | N | — |
| FR-ANLY-013 | Staleness made visible (Streak refresh cadence unknown); options tabs' cadence → §10 ANLY-013-opt. | N | — |
| FR-WATCH-001 | No 50-item cap (NFR-13; only the Kite subscription limit applies, C-07); reordering added (absent in Streak); table sorting stays absent (parity). | N | — |
| FR-WATCH-002 | Naming/renaming defined (Streak behaviour unobserved). | N | — |
| FR-WATCH-003 | Confirmation and last-list behaviour defined (unknown in Streak). | N | — |
| FR-WATCH-004 | Duplicate handling defined; keyboard add. | N | — |
| FR-WATCH-005 | Undo for removal; keyboard-reachable actions (FR-SHELL-015). | N | — |
| FR-WATCH-006 | Priority rules due to C-07. | N | — |
| FR-WATCH-007 | Fixed 50-instrument cap replaced by a caller-supplied optional maximum (NFR-13); Escape closes (§3.2 #8). | N | — |
| FR-WATCH-008 | BSE not restricted to BSE 1000 (Streak restriction is a product limit, NFR-13). Option contracts are excluded by scope (NFR-15). Manual refresh and the lot-size rule were folded in from the former options-master FR (RV-049). | N | — |
| FR-WATCH-009 | The 8 option universes of Streak's 29 are dropped with the options scope (§10 WATCH-009-opt). "Futures" basket supported (Conflicts §6 #11 — UI authoritative). Sector membership comes from the imported constituent files' Industry field mapped to the 11 sectors (C-09). One placement rule for custom baskets/watchlists (RV-038). | Y | 8 option universes → §10 WATCH-009-opt |
| FR-WATCH-010 | Streak maintains lists server-side with history; we use user-imported current lists (C-09); survivorship bias documented. | Y | Streak's server-side constituent history → user-imported current lists (C-09) |
| FR-WATCH-012 | No 10-basket × 50-symbol cap and no paid-feature marker (NFR-13); baskets are saved independently of scanners (resolves catalogue question). | N | — |
| FR-WATCH-013 | Identifier format `NSE:RELIANCE` instead of Streak's `NSE_RELIANCE`; Streak's option naming (ordinal day + superscript "w") is not reproduced — options are out of scope and the convention is Streak-specific (§10 WATCH-013-opt). | Y | Option naming → §10 WATCH-013-opt |
| FR-WATCH-014 | Fixes inconsistent BSE/NSE tagging (§3.2 #18). | N | — |
| FR-ACCT-001 | Session information and exchanges/products added (C-02). | N | — |
| FR-ACCT-002 | Streak's toggle covers NFO-OPT/BFO-OPT too (options out of scope, §10 ACCT-002-opt). Storage key (per underlying, in lots) defined explicitly; Streak semantics unknown. | Y | Option quantity memory → §10 ACCT-002-opt |
| FR-ACCT-003 | OS notifications replace browser push / mobile push (no server, C-03). Per-type filter and sound are additions. Default off (Streak default off, ACCT-003; RV-010). | N | — |
| FR-ACCT-004 | Streak's single-session rule becomes single-instance locally; deployments pause rather than run server-side (C-03). | N | — |
| FR-ACCT-005 | Streak needs no user API key; we require the user's own (C-01). | N | User's own Kite key required (C-01) |
| FR-ACCT-006 | Daily manual login with own key (C-02); loopback redirect acceptance by Kite is unverified, hence the paste fallback. | N | — |
| FR-ACCT-007 | Streak needs no data plan; our dependency is explicit (C-01). | N | — |
| FR-ACCT-008 | Static-IP gate and signal-only default are new (C-04, NFR-05). | N | — |
| FR-ACCT-009 | Streak's own caps (100 deployments, 25 live scanners, 50 instruments, backtests/day, lookback windows, 10 × 50 baskets) are not applied (NFR-13); only Kite/SEBI limits. | N | — |
| FR-ACCT-010 | Offline, bundled, own content instead of an external help site; adds a shortcuts list and logs access (absent in Streak). | N | — |
| FR-ACCT-011 | No mobile companion (single-user local desktop scope; C-03, NFR-04); Telegram relay is the **Enhancement (beyond Streak)** that replaces it (RV-042). | Y | Mobile app → desktop notifications (FR-ACCT-003) + Telegram relay (FR-ACCT-014) |
| FR-ACCT-012 | Streak has no settings page (its /settings is a 404); a local desktop app needs one. | N | — |
| FR-ACCT-013 | Not applicable to Streak (no local secrets); required by NFR-04 for minimal desktops (RV-044). | N | — |
| FR-ACCT-014 | Streak has no Telegram channel (A06 §3); a two-way Telegram bot (module TG) replaces its mobile push for away-from-desk use (RV-042). | N | — |
| FR-ACCT-015 | Streak's `/terms` and `/privacy` are web routes without the app shell (REVIEW-2 N6/N7); ours are in-app views with our own text. | N | — |
| FR-DATA-001 | Streak maintains the calendar server-side; the local list is shipped and user-maintained (C-03, C-14). Special sessions are modelled explicitly (not documented by Streak). | N | — |
| FR-DATA-002 | Streak's data lives server-side; the local store is required by C-03/C-08 and makes checksums and budgets user-visible. | N | — |
| FR-DATA-003 | Streak's server fetches data; the client, limiter and backfill service exist because of C-08 and the single API key (C-01). | N | — |
| FR-DATA-004 | Streak runs on synchronised servers; a local app must detect skew itself (NFR-08). | N | — |
| FR-DATA-005 | Streak keeps everything server-side; local storage (G3) requires backup/restore and migration rules (NFR-02/03/09). | N | — |
| FR-EQB-001 | Default instrument set is user-configurable rather than fixed. | N | — |
| FR-EQB-002 | Streak's internal format is not observable; ours is documented, versioned and round-trippable (clean-room design). The options body is deferred (§10 EQB-002-opt). | N | — |
| FR-EQB-003 | Streak opens the Save dialog on title click (§3.2 #10); we provide inline rename. The Save dialog stays reachable from Save. | N | — |
| FR-EQB-004 | Escape closes; validation is visible inline (§3.2 #8, #16). | N | — |
| FR-EQB-005 | Streak's handling of Sell + CNC is unknown; we block it explicitly. | N | — |
| FR-EQB-006 | Streak caps at 50 instruments/strategy; we remove the cap and warn on WebSocket capacity (NFR-13, C-07). | N | — |
| FR-EQB-007 | Inert "Candlestick" button replaced by a label (§3.2 #9). Heikin-Ashi/Renko not offered (same as Streak). | N | — |
| FR-EQB-008 | Incomplete rows are flagged and block execution rather than being silently ignored (EQB-006 note on Run Scan). | N | — |
| FR-EQB-009 | CPR works in Basic (§3.2 #1 not replicated). | N | — |
| FR-EQB-010 | Streak has no validation, raw identifier labels and ignores Escape (§3.2 #6, #7, #8); we validate and use readable labels. | N | — |
| FR-EQB-012 | none in semantics (documented parity, user decision 2026-10-12 on RV-021); ordering is unified across modes (Streak used a different Advance order). The "equal to" tolerance is our choice (OQ-18). | N | — |
| FR-EQB-013 | none — Streak offers `%` and `abs` on editable scanners (REVIEW-2 N4); our Points = abs, and Remove mirrors Streak's trash icon. | N | — |
| FR-EQB-015 | Streak's "Assist" implies AI but is static (§3.2 #4); ours is labelled honestly and also available in Advanced mode (inserts the suggestion as an Advanced row). | N | — |
| FR-EQB-016 | One explicit apply behaviour (§3.2 #30), Cancel/×/Escape (§3.2 #8, #20), clean categories and formatted text (§3.2 #5). | N | — |
| FR-EQB-017 | Own content; data bugs of the reference (Open showing Opening Range items, etc.) are not replicated (§3.2 #5). | N | — |
| FR-EQB-018 | Streak wipes conditions on switch (§3.2 #3); we convert. | N | — |
| FR-EQB-019 | Affordances are visible/keyboard-accessible rather than hover-only (NFR-11, §3.1). | N | — |
| FR-EQB-020 | Duplicate alias entries are collapsed into one entry with searchable aliases (Conflicts §6 #6). | N | — |
| FR-EQB-021 | Precedence is specified explicitly (not observable in Streak). | N | — |
| FR-EQB-023 | Validation added. | N | — |
| FR-EQB-024 | Same order as Basic (Streak used a different order). | N | — |
| FR-EQB-025 | none intended (parity with documented MTF/LCM rules, A06 §1; Streak bug §3.2 #2 not replicated). The rule text lives in FR-BT-034 only (RV-008). | N | — |
| FR-EQB-026 | Precedence documented explicitly (Streak's is unknown — open question OQ-15). | N | — |
| FR-EQB-027 | Explicit depth limit (Streak's unknown). | N | — |
| FR-EQB-028 | Disabled-item evaluation rule specified. | N | — |
| FR-EQB-029 | Clipboard scope defined (app-wide + OS text). | N | — |
| FR-EQB-030 | Visible validation instead of silent rejection (§3.2 #7). | N | — |
| FR-EQB-031 | No per-block condition cap (see FR-EQB-036). | N | — |
| FR-EQB-032 | Step 1 mirrors Streak's execution-type chooser (REVIEW-2 N2) with our own copy and a static preview; Deploy replaces the "Notification Based Trading" label (§3.2 glossary). Escape/× close (§3.2 #8). | N | — |
| FR-EQB-033 | Charting library per C-12. | N | — |
| FR-EQB-034 | Adds Export and Edit details. | N | — |
| FR-EQB-035 | Adds favourites-first ordering and filter (Streak had none). | N | — |
| FR-EQB-036 | Streak limits Basic to 5 entry + 5 exit and Advanced to 10 (Appendix N); removed per NFR-13. | N | — |
| FR-EQB-037 | Streak's save-time validation is unobserved and dialogs accept invalid values (§3.2 #7); we define a full validation set. | N | — |
| FR-EQB-038 | Values may differ slightly from Streak's where its formulas are undocumented; ours are documented and tested. | N | — |
| FR-EQB-039 | Consistent naming of tokens and labels (§3.2 #6). | N | — |
| FR-EQB-040 | Streak's chip errors when clicked (§3.2 #2) and Basic displays expression operands; we keep modes clean. | N | — |
| FR-EQB-041 | Adds condition trace; parity of backtest vs live is a requirement. | N | — |
| FR-EQB-042 | Adds keyboard alternatives (NFR-11; §3.1 hover-only affordances). | N | — |
| FR-DISC-001 | Own copy. | N | — |
| FR-DISC-002 | Own content (clean-room); may include Advanced-mode examples (Streak's are all Basic). Options strategies are not included (scope; §10 DISC-002-opt). Featured set tagged in data (RV-020). | N | No options strategies (Streak's library has none) |
| FR-DISC-003 | Counts shown. Option tabs are not offered (scope; §10 DISC-003-opt). | N | No option tabs (Streak's Discover has none) |
| FR-DISC-004 | Searches description and tags too. | N | — |
| FR-DISC-005 | Active-filter badge (§3.2 #24); full-catalogue filtering (§3.2 #11 analogue). | N | — |
| FR-DISC-006 | Keyboard navigation; no truncation loss (§3.2 #12 analogue). | N | — |
| FR-DISC-007 | Results computed locally on demand and cached, not served from a server (C-01, C-08); period configurable. | N | — |
| FR-DISC-008 | Display-mode switch without destructive confirm. | N | — |
| FR-DISC-009 | Cancel/Escape always available, inline name validation (§3.2 #16). | N | — |
| FR-DISC-010 | Streak runs library scanners continuously on its servers; locally they run only on demand / while visible (C-03, C-07, C-08). | Y | Continuous server-side library scanning → on-demand / while visible (C-03) |
| FR-DISC-011 | Own content; unique, untruncated names (§3.2 #12). Option-universe scanners (e.g. Streak's "Possible Downtrend" description mentioning Nifty options) run on equity universes here (scope). | N | Option-universe scanners run on equity universes |
| FR-DISC-012 | Universe list is data-driven and user-editable (C-09); option universes dropped (§10 WATCH-009-opt). | N | — |
| FR-DISC-013 | Filter covers all items (§3.2 #11); detail pane does not keep a stale selection. | N | — |
| FR-DISC-014 | No truncation ambiguity (§3.2 #12). | N | — |
| FR-DISC-015 | One timestamp format (§3.2 #14); sorting/search/export added; on-demand rather than server-continuous; option filters dropped with the options scope (§10 DISC-016). | N | — |
| FR-DISC-017 | Display toggle non-destructive; export format defined. | N | — |
| FR-DISC-018 | Cancel/Escape available (§3.2 #16). | N | — |
| FR-DISC-019 | No public URL sharing (no server); file/text export replaces it (C-03, personal use §1). | Y | Public URL sharing → file export/import (.tlstrategy.json / .tlscanner.json) |
| FR-DISC-020 | Explicit versioning (server-side in Streak). | N | — |
| FR-MYS-001 | Sorting and import added; no list caps (NFR-13). | N | — |
| FR-MYS-002 | Status and favourites filters added. Streak's Option Buying / Option Selling tabs are dropped with the options scope (§10 MYS-002-opt). | Y | Option buying/selling tabs → §10 MYS-002-opt |
| FR-MYS-003 | Export and status markers added. | N | — |
| FR-MYS-004 | Backtest history added (local storage, no caps); options detail pane → §10 MYS-004-opt. | Y | Options payoff summary → §10 MYS-004-opt |
| FR-MYS-005 | Filters implemented (Streak: "coming soon", §3.2 #31); no scanner count cap (NFR-13). Streak's `/scanners` alias route (REVIEW-2 N8) needs no equivalent — My Scanners is one view. | N | — |
| FR-MYS-006 | Visible live/paused status and toggle (MYS-006 open question). | N | — |
| FR-MYS-007 | Confirmation, blocking and soft delete defined (Streak's delete confirmation unobserved). | N | — |
| FR-MYS-008 | Local storage with crash recovery instead of server storage (§1 G3). | N | — |
| FR-BT-001 | Footer Back / Run backtest matches Streak (REVIEW-2 #14); Escape closes the panel and validation is inline (Streak behaviour unobserved). | N | — |
| FR-BT-002 | "–" = unlimited is an interpretation (Streak does not document it; see Open Questions OQ-22). | N | — |
| FR-BT-003 | none intended (exact Streak boundary semantics unverified — OQ-24). | N | — |
| FR-BT-004 | Futures mapped to NRML explicitly (Streak's handling is undocumented). | N | — |
| FR-BT-005 | none — per-instrument capital is confirmed on the current platform (Backtest Assumptions, Mar 2026: "for every instrument"; the observed qty 9 and qty 2 from one ₹25,000 reproduce; REVIEW-1 §4). | N | — |
| FR-BT-006 | Lot rounding for futures is an interpretation (Streak undocumented; OQ-25). | N | — |
| FR-BT-008 | **Enhancement (beyond Streak):** lookback limited only by local data (NFR-13); Streak caps at 30 d / 90 d / 365 d / 3 y. | N | — |
| FR-BT-009 | Data is fetched on demand and cached locally (Streak is server-side). | N | — |
| FR-BT-010 | No daily cap ("Limit Exceeded" not reproduced); Cancel added. | N | — |
| FR-BT-011 | Assumptions panel added (NFR-12). | N | — |
| FR-BT-012 | Sorting added (Enhancement). | N | — |
| FR-BT-013 | Stamp duty included (Streak does not name it); fee table visible and versioned. | N | — |
| FR-BT-014 | Max DD is computed on equity (capital + cumulative P&L) and shown as a positive %, because Streak's `(Peak − Trough)/Peak` on the raw P&L curve has unclear sign and divides by zero at peak 0 (FEATURES §6-16). Breakeven handling is explicit. | N | — |
| FR-BT-015 | Exact Streak averaging base unverified (OQ-26); definition made explicit. | N | — |
| FR-BT-017 | All seven codes are marked (Streak's chart shows EN, EX, SO); row→chart linking added. | N | — |
| FR-BT-018 | Needed because lookback is uncapped (FR-BT-008). | N | — |
| FR-BT-019 | Streak's docs mark open CNC positions "against the buy price" (a cumulative view, A06 §2); we mark against the previous day's close so day P&Ls sum to the realised P&L (daily-delta model; OQ-26, RV-027). | N | — |
| FR-BT-020 | P&L column semantics match Streak's live app (cumulative running total, REVIEW-2 #17); Trade P&L, Fill Time and Reason columns are additions that remove the signal-time ambiguity. | N | — |
| FR-BT-021 | Combined export added (Streak does not support it). | N | — |
| FR-BT-022 | Combined curve added. | N | — |
| FR-BT-026 | Level vs transition triggering for exits is unverified in Streak (OQ-27). | N | — |
| FR-BT-028 | Streak's own docs conflict (§6-15). Default reproduces the chosen interpretation; Intrabar mode is an **Enhancement (beyond Streak)** for users who want stricter realism (OQ-28). | N | — |
| FR-BT-029 | none intended; exact Streak trail start reference unverified (OQ-29). | N | — |
| FR-BT-030 | Ordering made explicit (Streak does not document it). | N | — |
| FR-BT-031 | The "containing candle" rule is an interpretation that reproduces Streak's documented 14:15 hourly case; Streak's exact rule for 15-min F&O stocks is unverified (OQ-30). MIS on a daily base is allowed (Streak lists both without exclusion; RV-028) using intraday data. | N | — |
| FR-BT-032 | Continuous-futures mode is an Enhancement; intraday history of expired contracts is UNVERIFIED (07 §A4; OQ-40). | N | — |
| FR-BT-033 | Behaviour made explicit. | N | — |
| FR-BT-034 | None in rules (A06 §1 parity, including weekly completion); warm-up not limited by Streak's lookback (Enhancement via FR-BT-008). The scope of LCM (intraday only) resolves the three conflicting v1 texts (RV-008). | N | — |
| FR-BT-036 | Streak has no slippage control. | N | — |
| FR-BT-037 | Warning and optional adjustment added. | N | — |
| FR-BT-038 | Streak supports equities and futures only — same here; full BSE is an enhancement. Options backtesting (a v1 enhancement idea) is out of scope (§10 BT-038-opt). | N | Options backtesting idea → §10 (Streak has none either) |
| FR-BT-039 | none for MCX/currency/AMO/MTF (Streak excludes them too); options excluded by scope decision (§1); Streak has no options backtest either. | N | Same exclusions as Streak plus options (scope) |
| FR-BT-040 | No daily cap; more specific messages. | N | — |
| FR-BT-041 | Streak refreshes current-day results only after 21:00. | N | — |
| FR-BT-042 | Tour copy written fresh (clean-room); single definition in FR-SHELL-007 (RV-049). | N | — |
| FR-BT-044 | **Enhancement (beyond Streak)** (NFR-12). | N | — |
| FR-BT-045 | **Enhancement (beyond Streak)**: Streak shows only the latest result. | N | — |
| FR-BT-046 | Streak shows "Capital Used" on the Live Backtest card without a definition (Appendix M, INFERRED); ours is explicit (RV-011). | N | — |
| FR-DEP-001 | Footer Back / Deploy and cycle default 1 match Streak (REVIEW-2 #15); mode selector, re-arm, GTT, validity date, Limit option and Escape-to-close are additions; Streak's Market-only order type is the default. | N | — |
| FR-DEP-002 | Cancel/×/Escape and inline validation added (Streak bug §3.2 #16 not replicated). | N | — |
| FR-DEP-003 | Streak offers notification-based trading only; signal-only, paper and armed-auto are additions (paper and auto are explicitly absent in Streak, DEP-011). | N | Additional modes; Notify & confirm is the parity default |
| FR-DEP-004 | States are explicit and visible; SUSPENDED exists because the app may be closed (C-03). CYCLE_DONE is MIS-only (RV-037); DAY_DONE is the parity state for MIS daily expiry within the T+29 validity (RV-009). | N | — |
| FR-DEP-005 | Streak is server-side; local persistence is required by C-03 and NFR-09. | N | — |
| FR-DEP-006 | Latency target < 2 s (Streak 8–14 s); toast added; channels are local plus optional Telegram (no web/mobile push). Post-closing placement matches Streak (A06 §3 Late triggers; RV-006). | N | Mobile push → FR-ACCT-011 |
| FR-DEP-008 | Partial fills monitored. Combined SL/TP for multi-leg option strategies is out of scope (§10 DEP-008-opt). Live SO is a separate, earlier table (RV-017). | Y | Combined SL/TP for option legs → §10 DEP-008-opt |
| FR-DEP-009 | Streak has no GTT usage; GTT LIMIT orders may not fill on gaps (documented in UI). | N | — |
| FR-DEP-010 | Needed because the local app is not always running (C-03). | N | — |
| FR-DEP-011 | Extra mode tabs and GTT column; stale-P&L quirk not replicated. | N | — |
| FR-DEP-012 | No 100-deployment cap; editable validity; "re-arm daily" exposed as a switch (parity default on). | N | — |
| FR-DEP-013 | Menu contents defined (Streak's unobserved). | N | — |
| FR-DEP-015 | Unread count on the bell added (absent in Streak, FEATURES §3.3); Missed and Info statuses added. | N | — |
| FR-DEP-016 | see FR-ACCT-003 (single definition, RV-010). | N | Browser/mobile push → OS notifications (FR-ACCT-003) |
| FR-DEP-017 | Streak has no paper trading (DEP-011). | N | — |
| FR-DEP-018 | Streak does not auto-trade (pending regulatory approval, A06 §3); allowed here only under NFR-05 explicit opt-in, static IP (C-04) and the order-rate cap (C-05). | N | — |
| FR-DEP-019 | Margin preview added (C-10). | N | — |
| FR-DEP-020 | Streak runs server-side; this compensates (C-03). Tray option unverified on the chosen stack (RV-043). | N | — |
| FR-DEP-021 | Streak does not square off live positions itself (alerts only; broker RMS does); the local table gives the user an alert before the broker acts (RV-017). | N | Streak has no live square-off; broker RMS acts — local alert added |
| FR-SCAN-001 | Unsaved-change guard and inline rename added. | N | — |
| FR-SCAN-002 | Constituent lists are user-imported, not shipped or live (C-09; survivorship bias documented). Option universes and the CALL/PUT/VOLUME>0/strike-range filters are dropped with the options scope (§10 SCAN-002-opt). | Y | Option universes and CALL/PUT/strike filters → §10 SCAN-002-opt |
| FR-SCAN-003 | Inert chart-type button not replicated (§3.2 #9). | N | — |
| FR-SCAN-004 | No 5/10 condition caps. Incomplete rows block (Streak silently ignores them, EQB-006 note); empty rows are ignored for parity (RV-022). | N | — |
| FR-SCAN-005 | Silent no-op (§3.2 #13) replaced by explicit error. | N | — |
| FR-SCAN-006 | Large uncached universes may take longer than Streak's server-side scan (bounded by 3 req/s); progress is shown. Whether Kite historical returns today's partial candle is UNVERIFIED (OQ-35). | N | — |
| FR-SCAN-007 | Indicator-value columns added. | N | — |
| FR-SCAN-009 | Content option and end date added (Enhancement); no 25-scanner cap; dialog closable by Escape/×/Cancel (Streak's closes only on backdrop click, REVIEW-2 N5); the trigger-frequency note matches Streak. | N | — |
| FR-SCAN-010 | Runs only while the app is open (C-03); faster than Streak. | N | — |
| FR-SCAN-011 | Working filters. | N | — |
| FR-SCAN-013 | Streak caps 5/10 conditions, 25 live scanners, 10 baskets × 50. | N | — |
| FR-SCAN-014 | In-app full-view chart instead of a browser tab (FR-CHART-017; RV-035). | N | Browser tab → in-app full-view chart |
| FR-SCAN-015 | Scanner→strategy linking added. | N | — |
| FR-SCAN-016 | Streak's internal format is not observable; ours is documented (RV-016). The option-filter block is deferred (§10 SCAN-016-opt). | N | — |
| FR-SCALP-001 | Streak's Scalper is an options workspace (underlying + ATM CE/PE charts); ours charts the underlying and its futures (user decision 2026-10-12; §10 SCALP-001-opt records the option layout). Local persistence (Streak's unclear). | Y | ATM CE chart → current-month future; §10 SCALP-001-opt |
| FR-SCALP-002 | Streak's CE⇄PE flip is replaced by an underlying⇄future flip (scope; §10 SCALP-002-opt). Timeframe sync behaviour is parity. | Y | CE⇄PE flip → underlying⇄future flip; §10 SCALP-002-opt |
| FR-SCALP-003 | Streak's third chart is the ATM put (scope; §10 SCALP-003-opt). Layout constraint surfaced in UI (§3.2 #32). | Y | ATM PE third chart → next-month future; §10 SCALP-003-opt |
| FR-SCALP-005 | Streak's basket holds option legs and templates (scope; §10 SCALP-005-opt, SCALP-006); equity/futures baskets with saved templates are our adaptation. Rejection hold behaviour defined (Streak's unobserved). | Y | Option legs/templates → equity/futures legs; §10 SCALP-005-opt, SCALP-006 |
| FR-SCALP-008 | Focused-chart targeting and "Add to basket" added. | N | — |
| FR-SCALP-009 | Paper sub-tab added. | N | — |
| FR-SCALP-011 | Behaviour is one shared implementation (RV-012); one-click mode and presets are additions. | N | — |
| FR-SCALP-012 | Streak allows 5 per chart on charts 1–2 only; removed per NFR-13. | N | — |
| FR-SCALP-013 | All three are absent in Streak. | N | — |
| FR-ORD-001 | Source column and section counts for all statuses added. | N | — |
| FR-ORD-002 | Total footer and conversion explicit. | N | — |
| FR-ORD-003 | Correct empty-state copy, refresh added. | N | — |
| FR-ORD-004 | AMO/iceberg/TTL optional enhancements; one ticket for every entry point (Streak's order ticket fields were not fully observed). | Y | Option contracts not orderable (scope); §10 ORD-004-opt |
| FR-ORD-005 | Explicit, documented pipeline (NFR-05). | N | — |
| FR-ORD-008 | Audit trail visible to the user (NFR-10). | N | — |
| FR-ORD-009 | Streak has no kill switch (required by NFR-05). | N | — |
| FR-ORD-010 | Hints added. | N | — |
| FR-ORD-011 | Same menu on standalone pages (Streak: scalper only, per docs). | N | — |
| FR-ORD-012 | Keyboard access. | N | — |
| FR-ORD-013 | Required by C-04 (Streak handles IP server-side). | N | — |
| FR-ORD-014 | Postbacks are not used (07 §A10). | N | Postbacks → WebSocket order updates |
| FR-ORD-015 | Streak allows post-market placement 15:40–16:00 (A06 §3) — same here (RV-006); pre-open entry and explicit phase messages are additions; AMO remains an optional enhancement (Streak has none). | N | — |
| FR-ORD-016 | Streak's crosshair "+" offers alert or order (CHART-009); the alert is implemented with Kite's server-side alerts (RV-033). | N | — |

FRs marked **Enhancement (beyond Streak)** in §6: FR-ACCT-011, FR-ACCT-014, FR-BT-008, FR-BT-028, FR-BT-036, FR-BT-044, FR-BT-045, FR-DEP-009, FR-DEP-017, FR-DEP-018, FR-SCALP-013.

## 8. Traceability (feature ID → FR IDs)

Every feature ID of `docs/research/FEATURES.md` (228 features after the REVIEW-2 additions) appears below, generated from the **Source features** lines of §6 and §10. Status: In scope = only §6 FRs; Out of scope = only §10 FRs; Partially = both (the feature's options/MCX/currency part is deferred).

| Feature ID | FR IDs (§6) | Out-of-scope FR IDs (§10) | Status |
|---|---|---|---|
| SHELL-001 | FR-SHELL-001, FR-SHELL-002, FR-SHELL-014, FR-SHELL-015, FR-SHELL-016, FR-SHELL-019 | FR-SHELL-002-opt | Partially |
| SHELL-002 | FR-SHELL-004 | — | In scope |
| SHELL-003 | FR-DATA-001, FR-DATA-003, FR-SHELL-005 | — | In scope |
| SHELL-004 | FR-SHELL-006 | — | In scope |
| SHELL-005 | FR-SHELL-002, FR-SHELL-003 | FR-SHELL-002-opt | Partially |
| SHELL-006 | FR-SHELL-007 | FR-SHELL-007-opt | Partially |
| SHELL-007 | FR-SHELL-008 | — | In scope |
| SHELL-008 | FR-SHELL-009 | — | In scope |
| SHELL-009 | FR-SHELL-010, FR-SHELL-017 | — | In scope |
| SHELL-010 | FR-SHELL-011 | — | In scope |
| SHELL-011 | FR-SHELL-012 | — | In scope |
| SHELL-012 | FR-SHELL-013, FR-SHELL-015, FR-SHELL-019 | — | In scope |
| SHELL-013 | FR-SHELL-020 | — | In scope |
| HOME-001 | FR-HOME-001 | — | In scope |
| HOME-002 | FR-HOME-002 | — | In scope |
| HOME-003 | FR-HOME-003 | — | In scope |
| HOME-004 | FR-HOME-004, FR-HOME-005 | FR-HOME-005-opt | Partially |
| HOME-005 | FR-BT-046, FR-DATA-002, FR-HOME-006, FR-HOME-007, FR-HOME-008 | — | In scope |
| CHART-001 | FR-CHART-001, FR-CHART-002, FR-CHART-003, FR-DATA-002, FR-DATA-003 | — | In scope |
| CHART-002 | FR-CHART-003, FR-CHART-004, FR-DATA-002 | — | In scope |
| CHART-003 | FR-CHART-005 | FR-CHART-005-opt | Partially |
| CHART-004 | FR-CHART-006 | — | In scope |
| CHART-005 | FR-CHART-007 | — | In scope |
| CHART-006 | FR-CHART-008 | — | In scope |
| CHART-007 | FR-CHART-009 | — | In scope |
| CHART-008 | FR-CHART-010, FR-SHELL-019 | — | In scope |
| CHART-009 | FR-CHART-011, FR-ORD-016 | — | In scope |
| CHART-010 | FR-CHART-012 | — | In scope |
| CHART-011 | FR-CHART-013, FR-CHART-014 | — | In scope |
| CHART-012 | FR-CHART-015 | — | In scope |
| CHART-013 | FR-CHART-016, FR-CHART-017 | — | In scope |
| ANLY-001 | FR-ANLY-001, FR-ANLY-002, FR-ANLY-013 | FR-ANLY-001-opt | Partially |
| ANLY-002 | FR-ANLY-003, FR-ANLY-004, FR-ANLY-005, FR-ANLY-013 | — | In scope |
| ANLY-003 | — | FR-ANLY-006, FR-ANLY-007, FR-ANLY-013-opt | Out of scope |
| ANLY-004 | — | FR-ANLY-008, FR-ANLY-010-opt | Out of scope |
| ANLY-005 | — | FR-ANLY-009, FR-ANLY-010-opt | Out of scope |
| ANLY-006 | FR-ANLY-010, FR-ANLY-011, FR-ANLY-013, FR-DATA-002 | — | In scope |
| ANLY-007 | — | FR-ANLY-010-opt, FR-ANLY-012, FR-OPT-038 | Out of scope |
| WATCH-001 | FR-WATCH-001, FR-WATCH-002, FR-WATCH-006 | — | In scope |
| WATCH-002 | FR-WATCH-004, FR-WATCH-008 | — | In scope |
| WATCH-003 | FR-WATCH-005, FR-WATCH-006 | — | In scope |
| WATCH-004 | FR-WATCH-003 | — | In scope |
| WATCH-005 | FR-WATCH-007, FR-WATCH-008 | — | In scope |
| WATCH-006 | FR-WATCH-009, FR-WATCH-010, FR-WATCH-011 | FR-WATCH-009-opt | Partially |
| WATCH-007 | FR-WATCH-012 | — | In scope |
| WATCH-008 | FR-SHELL-016, FR-WATCH-008, FR-WATCH-013, FR-WATCH-014 | FR-WATCH-013-opt | Partially |
| EQB-001 | FR-EQB-001 | — | In scope |
| EQB-002 | FR-EQB-003, FR-EQB-004 | — | In scope |
| EQB-003 | FR-EQB-005 | — | In scope |
| EQB-004 | FR-EQB-006 | — | In scope |
| EQB-005 | FR-EQB-007 | — | In scope |
| EQB-006 | FR-EQB-002, FR-EQB-008, FR-EQB-041 | FR-EQB-002-opt | Partially |
| EQB-007 | FR-EQB-009 | — | In scope |
| EQB-008 | FR-EQB-010 | — | In scope |
| EQB-009 | FR-EQB-011 | — | In scope |
| EQB-010 | FR-EQB-012 | — | In scope |
| EQB-011 | FR-EQB-013 | — | In scope |
| EQB-012 | FR-EQB-014 | — | In scope |
| EQB-013 | FR-EQB-015 | — | In scope |
| EQB-014 | FR-EQB-016, FR-EQB-017 | — | In scope |
| EQB-015 | FR-EQB-018 | — | In scope |
| EQB-016 | FR-EQB-002, FR-EQB-019, FR-EQB-041, FR-EQB-042 | — | In scope |
| EQB-017 | FR-EQB-020 | — | In scope |
| EQB-018 | FR-EQB-021 | — | In scope |
| EQB-019 | FR-EQB-022, FR-EQB-042 | — | In scope |
| EQB-020 | FR-EQB-023 | — | In scope |
| EQB-021 | FR-EQB-012, FR-EQB-024 | — | In scope |
| EQB-022 | FR-BT-034, FR-EQB-002, FR-EQB-025, FR-EQB-041 | — | In scope |
| EQB-023 | FR-EQB-002, FR-EQB-026 | — | In scope |
| EQB-024 | FR-EQB-002, FR-EQB-027 | — | In scope |
| EQB-025 | FR-EQB-028, FR-EQB-042 | — | In scope |
| EQB-026 | FR-EQB-029 | — | In scope |
| EQB-027 | FR-EQB-002, FR-EQB-030 | — | In scope |
| EQB-028 | FR-EQB-002, FR-EQB-031 | — | In scope |
| EQB-029 | FR-EQB-004 | — | In scope |
| EQB-030 | FR-EQB-032 | — | In scope |
| EQB-031 | FR-EQB-033 | — | In scope |
| EQB-032 | FR-EQB-034 | — | In scope |
| EQB-033 | FR-EQB-035 | — | In scope |
| EQB-034 | FR-EQB-002, FR-EQB-036 | — | In scope |
| EQB-035 | FR-EQB-010, FR-EQB-037 | — | In scope |
| EQB-036 | FR-EQB-017, FR-EQB-038, FR-EQB-039 | — | In scope |
| EQB-037 | FR-EQB-038, FR-EQB-039 | — | In scope |
| EQB-038 | FR-EQB-002, FR-EQB-040 | — | In scope |
| EQB-039 | FR-BT-001, FR-DEP-001, FR-EQB-032 | — | In scope |
| OPT-001 | — | FR-OPT-001 | Out of scope |
| OPT-002 | — | FR-OPT-002, FR-OPT-003 | Out of scope |
| OPT-003 | — | FR-OPT-004, FR-OPT-036 | Out of scope |
| OPT-004 | — | FR-OPT-005 | Out of scope |
| OPT-005 | — | FR-OPT-006 | Out of scope |
| OPT-006 | — | FR-OPT-002, FR-OPT-007 | Out of scope |
| OPT-007 | — | FR-OPT-002, FR-OPT-008 | Out of scope |
| OPT-008 | — | FR-OPT-002, FR-OPT-009 | Out of scope |
| OPT-009 | — | FR-OPT-010 | Out of scope |
| OPT-010 | — | FR-OPT-011, FR-OPT-036 | Out of scope |
| OPT-011 | — | FR-OPT-002, FR-OPT-012 | Out of scope |
| OPT-012 | — | FR-OPT-013 | Out of scope |
| OPT-013 | — | FR-OPT-014 | Out of scope |
| OPT-014 | — | FR-OPT-015 | Out of scope |
| OPT-015 | — | FR-OPT-016, FR-OPT-017, FR-OPT-020, FR-OPT-036, FR-OPT-038 | Out of scope |
| OPT-016 | — | FR-OPT-002, FR-OPT-018, FR-OPT-020 | Out of scope |
| OPT-017 | — | FR-OPT-002, FR-OPT-019 | Out of scope |
| OPT-018 | — | FR-OPT-021 | Out of scope |
| OPT-019 | — | FR-OPT-002, FR-OPT-022 | Out of scope |
| OPT-020 | — | FR-OPT-002, FR-OPT-023 | Out of scope |
| OPT-021 | — | FR-OPT-002, FR-OPT-024 | Out of scope |
| OPT-022 | — | FR-OPT-025, FR-OPT-038 | Out of scope |
| OPT-023 | — | FR-OPT-026 | Out of scope |
| OPT-024 | — | FR-OPT-027 | Out of scope |
| OPT-025 | — | FR-OPT-028 | Out of scope |
| OPT-026 | — | FR-OPT-029 | Out of scope |
| OPT-027 | — | FR-OPT-030 | Out of scope |
| OPT-028 | — | FR-OPT-031 | Out of scope |
| OPT-029 | — | FR-OPT-032, FR-OPT-037 | Out of scope |
| OPT-030 | — | FR-OPT-033, FR-OPT-037 | Out of scope |
| OPT-031 | — | FR-OPT-034 | Out of scope |
| OPT-032 | — | FR-OPT-035 | Out of scope |
| DISC-001 | FR-DISC-001, FR-DISC-002, FR-DISC-020 | FR-DISC-002-opt | Partially |
| DISC-002 | FR-DISC-003 | FR-DISC-003-opt | Partially |
| DISC-003 | FR-DISC-004 | — | In scope |
| DISC-004 | FR-DISC-005 | — | In scope |
| DISC-005 | FR-DISC-002, FR-DISC-006 | — | In scope |
| DISC-006 | FR-DISC-007 | — | In scope |
| DISC-007 | FR-DISC-002, FR-DISC-008 | FR-DISC-002-opt | Partially |
| DISC-008 | FR-DISC-009 | — | In scope |
| DISC-009 | FR-DISC-010, FR-DISC-011, FR-DISC-020 | — | In scope |
| DISC-010 | FR-DISC-012 | — | In scope |
| DISC-011 | FR-DISC-013 | — | In scope |
| DISC-012 | FR-DISC-011, FR-DISC-014 | — | In scope |
| DISC-013 | FR-DISC-015 | — | In scope |
| DISC-014 | — | FR-DISC-016, FR-SCAN-002-opt | Out of scope |
| DISC-015 | FR-DISC-017 | — | In scope |
| DISC-016 | FR-DISC-018 | — | In scope |
| DISC-017 | FR-DATA-005, FR-DISC-019, FR-SCAN-016 | — | In scope |
| MYS-001 | FR-DATA-005, FR-MYS-001, FR-MYS-008 | — | In scope |
| MYS-002 | FR-MYS-002 | FR-MYS-002-opt | Partially |
| MYS-003 | FR-MYS-003, FR-MYS-007 | — | In scope |
| MYS-004 | FR-MYS-004 | FR-MYS-004-opt | Partially |
| MYS-005 | FR-DATA-005, FR-MYS-005, FR-MYS-008 | — | In scope |
| MYS-006 | FR-MYS-006, FR-MYS-007 | — | In scope |
| MYS-007 | FR-MYS-005 | — | In scope |
| BT-001 | FR-BT-001, FR-EQB-032 | — | In scope |
| BT-002 | FR-BT-002 | — | In scope |
| BT-003 | FR-BT-003 | — | In scope |
| BT-004 | FR-BT-004 | — | In scope |
| BT-005 | FR-BT-005, FR-BT-006, FR-BT-007 | — | In scope |
| BT-006 | FR-BT-008, FR-BT-009, FR-DATA-002, FR-DATA-003 | — | In scope |
| BT-007 | FR-BT-009, FR-BT-010, FR-BT-045, FR-DATA-003 | — | In scope |
| BT-008 | FR-BT-011, FR-BT-012, FR-BT-044, FR-BT-045, FR-DATA-005 | — | In scope |
| BT-009 | FR-BT-012 | — | In scope |
| BT-010 | FR-BT-013 | — | In scope |
| BT-011 | FR-BT-014, FR-BT-033, FR-BT-046 | — | In scope |
| BT-012 | FR-BT-015 | — | In scope |
| BT-013 | FR-BT-016 | — | In scope |
| BT-014 | FR-BT-017, FR-BT-018 | — | In scope |
| BT-015 | FR-BT-019, FR-BT-033 | — | In scope |
| BT-016 | FR-BT-020 | — | In scope |
| BT-017 | FR-BT-021 | — | In scope |
| BT-018 | FR-BT-022 | — | In scope |
| BT-019 | FR-BT-023, FR-BT-024, FR-BT-025, FR-BT-026, FR-BT-030 | — | In scope |
| BT-020 | FR-BT-030, FR-BT-031, FR-DATA-001, FR-DEP-021 | — | In scope |
| BT-021 | FR-BT-027, FR-BT-028, FR-BT-029, FR-BT-030 | — | In scope |
| BT-022 | FR-BT-007 | — | In scope |
| BT-023 | FR-BT-034 | — | In scope |
| BT-024 | FR-BT-035, FR-BT-036, FR-BT-044 | — | In scope |
| BT-025 | FR-BT-037, FR-BT-044, FR-DATA-002 | — | In scope |
| BT-026 | FR-BT-032, FR-BT-038, FR-BT-039 | FR-BT-038-opt | Partially |
| BT-027 | FR-BT-040 | — | In scope |
| BT-028 | FR-BT-041 | — | In scope |
| BT-029 | FR-BT-042, FR-SHELL-007 | — | In scope |
| BT-030 | FR-BT-026, FR-BT-032, FR-BT-043 | — | In scope |
| DEP-001 | FR-ACCT-015, FR-DEP-001, FR-DEP-003, FR-DEP-019, FR-EQB-032, FR-SHELL-020 | FR-OPT-037 | Partially |
| DEP-002 | FR-DEP-002 | — | In scope |
| DEP-003 | FR-DEP-004, FR-DEP-005, FR-DEP-010, FR-DEP-011 | — | In scope |
| DEP-004 | FR-DATA-001, FR-DATA-004, FR-DEP-004, FR-DEP-005, FR-DEP-009, FR-DEP-010, FR-DEP-012, FR-DEP-019, FR-DEP-020, FR-DEP-021 | — | In scope |
| DEP-005 | FR-DATA-001, FR-DATA-004, FR-DEP-004, FR-DEP-006, FR-DEP-007, FR-DEP-010, FR-DEP-020, FR-ORD-015 | — | In scope |
| DEP-006 | FR-DEP-004, FR-DEP-008, FR-DEP-009, FR-DEP-021 | FR-DEP-008-opt | Partially |
| DEP-007 | FR-DEP-004, FR-DEP-013 | — | In scope |
| DEP-008 | FR-DEP-014 | — | In scope |
| DEP-009 | FR-DEP-007, FR-DEP-015 | — | In scope |
| DEP-010 | FR-ACCT-003, FR-ACCT-014, FR-DEP-016 | — | In scope |
| DEP-011 | FR-DEP-003, FR-DEP-017, FR-DEP-018 | — | In scope |
| SCAN-001 | FR-SCAN-001 | — | In scope |
| SCAN-002 | FR-SCAN-002 | FR-SCAN-002-opt | Partially |
| SCAN-003 | FR-SCAN-003 | — | In scope |
| SCAN-004 | FR-SCAN-004 | — | In scope |
| SCAN-005 | FR-SCAN-005 | — | In scope |
| SCAN-006 | FR-DATA-002, FR-SCAN-006 | — | In scope |
| SCAN-007 | FR-SCAN-007 | — | In scope |
| SCAN-008 | FR-SCAN-008, FR-SCAN-016 | FR-SCAN-016-opt | Partially |
| SCAN-009 | FR-ACCT-015, FR-SCAN-009, FR-SCAN-016 | — | In scope |
| SCAN-010 | FR-DATA-001, FR-DATA-004, FR-SCAN-010 | — | In scope |
| SCAN-011 | FR-SCAN-011 | — | In scope |
| SCAN-012 | FR-SCAN-012 | — | In scope |
| SCAN-013 | FR-SCAN-013 | — | In scope |
| SCAN-014 | FR-SCAN-014 | — | In scope |
| SCAN-015 | FR-SCAN-015 | — | In scope |
| SCALP-001 | FR-SCALP-001 | FR-SCALP-001-opt | Partially |
| SCALP-002 | FR-SCALP-002 | FR-SCALP-002-opt | Partially |
| SCALP-003 | FR-SCALP-003 | FR-SCALP-003-opt | Partially |
| SCALP-004 | — | FR-OPT-038, FR-SCALP-004 | Out of scope |
| SCALP-005 | FR-SCALP-005 | FR-SCALP-005-opt | Partially |
| SCALP-006 | — | FR-SCALP-006 | Out of scope |
| SCALP-007 | — | FR-OPT-038, FR-SCALP-007 | Out of scope |
| SCALP-008 | FR-SCALP-008 | — | In scope |
| SCALP-009 | FR-SCALP-009 | — | In scope |
| SCALP-010 | FR-SCALP-010 | — | In scope |
| SCALP-011 | FR-SCALP-011 | — | In scope |
| SCALP-012 | FR-SCALP-012 | — | In scope |
| SCALP-013 | FR-SCALP-013, FR-SHELL-019 | — | In scope |
| ORD-001 | FR-ORD-001, FR-ORD-008, FR-ORD-014, FR-SHELL-018 | — | In scope |
| ORD-002 | FR-ORD-002, FR-ORD-014 | — | In scope |
| ORD-003 | FR-ORD-003 | — | In scope |
| ORD-004 | FR-ORD-004, FR-ORD-005, FR-ORD-006, FR-ORD-007, FR-ORD-013, FR-ORD-015 | FR-ORD-004-opt | Partially |
| ORD-005 | FR-ORD-008, FR-ORD-010, FR-SHELL-018 | — | In scope |
| ORD-006 | FR-ORD-005, FR-ORD-006, FR-ORD-008, FR-ORD-009, FR-ORD-011 | — | In scope |
| ORD-007 | FR-ORD-005, FR-ORD-012, FR-ORD-016 | — | In scope |
| ACCT-001 | FR-ACCT-001, FR-ACCT-012 | FR-ACCT-012-opt | Partially |
| ACCT-002 | FR-ACCT-002 | FR-ACCT-002-opt | Partially |
| ACCT-003 | FR-ACCT-003, FR-ACCT-014 | — | In scope |
| ACCT-004 | FR-ACCT-004, FR-ACCT-006, FR-DATA-004 | — | In scope |
| ACCT-005 | FR-ACCT-005, FR-ACCT-006, FR-ACCT-007, FR-ACCT-008, FR-ACCT-012, FR-ACCT-013, FR-DATA-003, FR-DATA-005, FR-SHELL-017 | — | In scope |
| ACCT-006 | FR-ACCT-008, FR-ACCT-009, FR-ACCT-012, FR-DATA-001, FR-DATA-003 | — | In scope |
| ACCT-007 | FR-ACCT-010, FR-SHELL-018 | — | In scope |
| ACCT-008 | FR-ACCT-011, FR-ACCT-014 | — | In scope |
| ACCT-009 | FR-ACCT-015 | — | In scope |
| ACCT-010 | FR-ACCT-015 | — | In scope |
| ACCT-011 | FR-ACCT-006 | — | In scope |

Summary: 228 feature IDs — In scope 162, Partially 26, Out of scope 40. §6 holds 265 FRs; §10 holds 72 entries (47 whole FRs and 25 "-opt" fragments).

## 9. Open questions

One merged, de-duplicated list (RV-041). Questions answered by REVIEW-1 §4 or by the user decisions of 2026-10-12 are closed and
listed in `CHANGELOG-v2.md`; options-only questions are in §10.0. "Blocks phase?" names the phase that cannot be finalised until
the question is answered: **Phase 1** = data layer, builder, indicators, backtest; **Live** = deployments and orders; **No** =
can be decided later without rework. "Default taken" is what the FRs specify today.

| ID | Question | Owner | Blocks phase? | Default taken |
|---|---|---|---|---|
| OQ-01 | Chart library (C-12): licensed TradingView Charting Library vs an open library plus our own 107 studies, drawing tools, dialogs and order lines (FR-CHART-007/008/009/010/013/014/015). | dev | Phase 1 | Open library (lightweight-charts) + own implementation; variant set per FR-CHART-010 |
| OQ-02 | Drawing-tool variant list: Streak's full TradingView flyouts were not transcribed; is the FR-CHART-010 minimum set sufficient? | user | No | FR-CHART-010 set |
| OQ-03 | Technicals vote composition and pivot periods are ours (FR-ANLY-003/004); acceptable, or should they mimic a known public rule set? | user | No | As specified |
| OQ-04 | Holdings card headline: overall P&L (chosen) vs day P&L; include T1 and collateral quantities? | user | No | Overall; T1 included, collateral excluded |
| OQ-05 | Are soft caps wanted for watchlists (Streak 50), the instrument picker (50) and custom baskets (10 × 50) beyond the budget warnings? | user | No | None (NFR-13) |
| OQ-06 | BSE coverage: all BSE equities (FR-WATCH-008) vs Streak's BSE 1000 subset. | user | No | All |
| OQ-07 | Does Kite accept `http://127.0.0.1:<port>` as the app redirect URL (A07 UNVERIFIED)? | dev (test on first login) | Phase 1 | Loopback primary, paste fallback (FR-ACCT-006) |
| OQ-08 | Market calendar maintenance: is the shipped list + CSV import + manual edit (FR-DATA-001) acceptable, and who records special sessions (e.g. Muhurat) each year? | user | No | Shipped list; user imports/edits |
| OQ-09 | **Closed 2026-10-12 (user):** rely on Kite IP-mismatch rejections only; the IP-echo check (FR-ORD-013) stays available but off by default. | — | — | Decided |
| OQ-10 | Sector universes: mapping of constituent-file "Industry" values to the 11 sectors (FR-WATCH-010) is ours; Streak's source unknown. | user | No | Our editable mapping table |
| OQ-11 | "Indices" and "Futures" universe membership: all indices in the instrument master? current-month futures only? | user | No | All indices; current month |
| OQ-12 | Chart layout persistence: explicit Save (FR-CHART-007) vs autosave. | user | No | Explicit Save |
| OQ-13 | MA-type bindings: "triple" (TEMA vs T3), "variable" vs "vidya"; reference library and version per operand (FR-EQB-038). | dev + user | Phase 1 (golden vectors) | triple = TEMA; variable = Chande VMA; vidya = VIDYA; TA-Lib pinned where available |
| OQ-14 | Pivot Points "continuous = yes" meaning (rolling vs session-anchored). | dev | No | Session-anchored, defined in `indicators.md` |
| OQ-15 | AND-before-OR precedence for flat mixed chains (FR-EQB-026) vs left-to-right. | user | No | AND binds tighter |
| OQ-16 | Exact formulas for less-standard operands (Trend Intensity Index, Twiggs MF, Fractal Chaos Bands, Center of Gravity, Swing Index, Schaff, Intraday Momentum Index, ATR Trailing Stoploss parameters). | dev | Phase 1 (golden vectors) | Chosen per operand in `indicators.md` |
| OQ-17 | Up by / Down by "from" semantics when R is negative (oscillators): percent of \|R\| or signed R? | user | No | Percent of \|R\| |
| OQ-18 | "equal to" tolerance: ε = 1e-9 relative (FR-EQB-012) works for Open = High/Low scanners but oscillator equality effectively never fires; use tick-size tolerance for price operands? | user | No | 1e-9 relative |
| OQ-19 | Summary-backtest window (3 months) and whether library summaries are precomputed after daily backfill (FR-DISC-007). | user | No | 3 months, on demand |
| OQ-20 | Auto-refresh of library scanners while visible (FR-DISC-015): acceptable load on quote/ticker limits for Nifty 500? | dev | No | On while visible |
| OQ-21 | Soft-delete retention (30 days) and restorable backtests (FR-MYS-007). | user | No | 30 days, restorable |
| OQ-22 | Does Strategy Cycle "–" mean unlimited, and what is the maximum cycle count in live mode (FR-BT-002, FR-DEP-001)? | user | No | Unlimited; 1–5 offered |
| OQ-23 | Default chart timeframe: catalogue says 1 Hour (dashboard/Scalper); the REVIEW-2 sweep saw 1 Day (unconfirmed). | user | No | 1 Hour, user setting |
| OQ-24 | Entry window boundaries: test on signal time or fill time, inclusive of End Time (FR-BT-003)? (End Time does not force an exit — answered.) | user | No | Fill time, inclusive |
| OQ-25 | Futures quantity in backtests: lot-rounded or raw floor(C ÷ P) (FR-BT-006)? | user | No | Lot-rounded |
| OQ-26 | Return base for day/month % and inclusion of no-trade days in weekday averages; daily-delta vs cumulative mark of open CNC positions (FR-BT-015/016/019, FR-BT-046). | user | No | C (× n combined); active days only; daily-delta mark |
| OQ-27 | Exit conditions level-triggered or transition-triggered (FR-BT-026)? | user | No | Level-triggered |
| OQ-28 | SL/TP evaluation: Hybrid vs Intrabar (FEATURES §6-15) — verify with a controlled comparison against Streak (FR-BT-028). | dev | No | Hybrid default + Intrabar option |
| OQ-29 | TSL reference start (entry price vs first candle high) and fill price on TSL hit (FR-BT-029). | dev | No | Entry price; fill at level |
| OQ-30 | Backtest square-off when T is not a candle boundary (15:10 on 15-min); source of the call-auction (CAS) stock list (FR-BT-031). | dev | No | Containing-candle open; user-importable list |
| OQ-31 | Zerodha RMS MIS auto-square-off timings per segment (not in the research sources) — the live SO table default of 15:10 (FR-DEP-021) must be earlier than them. | dev | Live | 15:10 cash and futures |
| OQ-32 | Entry alert expiry window in Streak, if any (FR-DEP-006). | dev | No | Next base-TF candle close |
| OQ-33 | Does GTT placement require the registered static IP; cap on active GTTs / GTT modifications (FR-DEP-009)? | dev (test on first GTT) | Live | Treat as requiring trading enabled; ≤ 1 modification / 60 s |
| OQ-34 | Freeze-quantity table: Kite applies freeze limits server-side and `autoslice=true` slices (REVIEW-1 §4); keep an optional local table for pre-flight warnings (FR-ORD-005)? | dev | No | Optional table + autoslice |
| OQ-35 | Does the Kite historical API return today's partial (forming) candle intraday (FR-SCAN-006, FR-BT-041)? | dev | No | Use ticker-built candle; verify |
| OQ-36 | Live scanner alert content: all matches each run or only new matches (FR-SCAN-009/010)? | user | No | All matches; "new only" option |
| OQ-37 | `algo_id` (gokiteconnect v4.4.1+): is it required for API orders after 1 Apr 2026 and what value (FR-ORD-005)? Test on the first live order. | dev | Live | Settings field reserved, empty |
| OQ-38 | System tray on Wails v2: no tray API in v2 (A07 §D) — keep "Keep running in tray" (FR-DEP-020) as UNVERIFIED enhancement or drop? | dev | No | Hidden unless a tray integration is available |
| OQ-39 | Legal status of distributing the app to other traders (algo-provider empanelment, A07 §B) — personal use is clear. | user | No | Personal use only |
| OQ-40 | Expired futures contracts: intraday history availability for backtests on past contracts (FR-BT-032). | dev | No | Live contracts; continuous mode on day TF |
| OQ-41 | Candle store engine: SQLite vs DuckDB/Parquet for the ≈470 M-row estimate (FR-DATA-002, A07 §D). | dev | Phase 1 | SQLite first, benchmark FR-DATA-002 AC4 |
| OQ-42 | Are Kite day candles corporate-action adjusted (A07 §E UNVERIFIED)? Affects FR-BT-037 warnings. | dev | No | Assume raw |
| OQ-43 | Scalper right-chart default = current-month future of the left chart's underlying (FR-SCALP-001) — confirm. | user | No | Yes |
| OQ-44 | ~~Telegram relay send-only?~~ **Closed 2026-10-12:** user chose a two-way bot modelled on freqtrade (module TG); see OQ-45…49. | — | — | Module TG |
| OQ-45 | **Closed 2026-10-12 (user):** kill switch default = cancel pending entries + disarm all deployments, keep GTT/SL protections on open positions; square-off is a separate confirmed action (desktop and /kill). | — | — | Decided |
| OQ-46 | Telegram-confirmed signals are placed from the desktop (static IP) — confirm-from-phone only works while the desktop app is running (FR-TG-008). Acceptable? | user | No | Yes; bot announces offline/online |
| OQ-47 | Does a Telegram confirmation count as 'user confirmation' under the SEBI retail-algo framing (≤10 OPS own key)? | user (legal) | Live | Treated as manual confirmation; auto-exec rules unchanged |
| OQ-48 | Go keyring library for Linux Secret Service (FR-TG-002, NFR-04) and behaviour without a keyring daemon (see FR-ACCT-013). | dev | Phase 1 | Pick a maintained MIT/BSD keyring lib in the architecture spec; FR-ACCT-013 fallback |
| OQ-49 | Telegram: support groups/topics with several authorized users, or private chat only (simplifies FR-TG-004/006)? | user | No | Private chat + optional single group/topic |

Questions blocking **Phase 1**: OQ-01, OQ-07, OQ-13, OQ-16, OQ-41 (5). Blocking **Live**: OQ-09, OQ-31, OQ-33, OQ-37 (4).

## 10. Out of scope (deferred — retained for future)

Everything in this section was removed from §6 by the scope decision of 2026-10-12 (options, MCX, currency) or was drafted by
REVIEW-1 for the options scope. The text is preserved verbatim (with the review corrections noted inline) so it can be revived
without re-research. Entries are of three kinds: **whole FRs** moved from v1 §6 (heading + an "Out of scope (deferred)" line
giving the reason), **"-opt" fragments** = the options-only text removed from an FR that stays in §6 under the same ID, and the
two **deferred FRs** FR-OPT-037/038 drafted by REVIEW-1 §3. No §6 FR references anything here (verified by script). Feature IDs
covered only here are marked "Out of scope" in §8; feature IDs split between §6 and here are "Partially".

### 10.0 Open questions that apply only to this section

| ID | Question | Owner | Default taken |
|---|---|---|---|
| OQ-§10-1 | Expired option contracts: can intraday history be fetched (A07 §A4 UNVERIFIED)? Needed before any options backtest. | dev | live contracts only |
| OQ-§10-2 | Options pricing inputs: risk-free rate (6.5 %) and dividend yield; Black-76 on the synthetic forward vs Black-Scholes on spot (FR-OPT-038). | user | Black-76, r = 6.5 %, q = 0 |
| OQ-§10-3 | Should two underlying blocks default to AND (FR-OPT-007), and should more than 2 underlyings ever be allowed? | user | AND; max 2 |
| OQ-§10-4 | Time-based entry default 09:20 vs Streak's 09:15 (FR-OPT-008). | user | 09:20 |
| OQ-§10-5 | Should the Discover library include options strategies (Streak's has none)? | user | no |
| OQ-§10-6 | Freeze-quantity table for option lot warnings (Kite applies freeze limits server-side; `autoslice=true` splits into ≤ 10 slices). | dev | optional local table |
| OQ-§10-7 | Whole-chain PCR vs the ATM ±20 strike window of FR-ANLY-009 (subscription budget impact). | user | ±20 window |
| OQ-§10-8 | Time-based options exit needs a trigger code: `TE` (FR-OPT-037) vs reusing SO (RV-046). | dev | `TE` |
| OQ-§10-9 | Options deployment semantics not observed in Streak (OPT-029): leg-set DI, partial leg fills (RV-002). | user | FR-OPT-037 draft |


### 10.1 Module SHELL

#### FR-SHELL-002-opt Sidebar entry "Options Builder"
- **Out of scope (deferred):** options out of scope (user decision 2026-10-12).
- **Source features:** SHELL-001, SHELL-005
- **Requirement (removed text):** The sidebar entry "Options Builder" (new options strategy draft) sits between Equity Builder and
  Live Strategies; it opens the shared strategy builder with the type preset to options and always opens a new draft.
- **Acceptance criteria:** Given the user clicks Options Builder twice (saving nothing), then each click opens a fresh options draft
  (subject to the unsaved-change guard).
- **Data/Kite dependency:** local only.
- **Deviation from Streak:** Labels use our own wording.
- **Priority hint:** Core (if revived)

#### FR-SHELL-007-opt Options Builder tour
- **Out of scope (deferred):** options out of scope.
- **Source features:** SHELL-006
- **Requirement (removed text):** Options Builder tour (5 steps: entry trigger underlying/time, execution/strike selection, exit,
  payoff graph, underlying chart); the Scalper tour's first step anchors to the option chain.
- **Acceptance criteria:** Given "Replay tour" on the Options Builder, then the 5-step tour starts at step 1.
- **Data/Kite dependency:** local only.
- **Deviation from Streak:** Tour copy is our own.
- **Priority hint:** Nice-to-have


### 10.2 Module HOME

#### FR-HOME-005-opt Option rows in the Live Scanners widget
- **Out of scope (deferred):** options out of scope.
- **Source features:** HOME-004
- **Requirement (removed text):** Option result rows show a circular C/P badge, the option display name per FR-WATCH-013-opt and
  exchange tag NFO-OPT; charting an option from the widget shows the Technicals tab only.
- **Acceptance criteria:** Given a row for NIFTY 13 OCT (W) 22250 CE, then it shows the C badge and exchange tag NFO-OPT.
- **Data/Kite dependency:** instrument master incl. options.
- **Deviation from Streak:** none.
- **Priority hint:** Important


### 10.3 Module CHART

#### FR-CHART-005-opt Option contracts in the symbol search
- **Out of scope (deferred):** options out of scope.
- **Source features:** CHART-003
- **Requirement (removed text):** Search also covers NFO-OPT and BFO-OPT; result rows show C/P badges for options; query
  "reliance oct 1290 ce" returns the matching NFO option with a C badge.
- **Acceptance criteria:** Given query "BANKNIFTY", then results include current BANKNIFTY options.
- **Data/Kite dependency:** instrument master incl. options.
- **Deviation from Streak:** BFO options included so SENSEX/BANKEX derivatives are chartable.
- **Priority hint:** Core


### 10.4 Module ANLY

#### FR-ANLY-006 Option Chain tab
- **Out of scope (deferred):** Option chain analytics — options out of scope (user decision 2026-10-12).
- **Source features:** ANLY-003
- **Requirement:** The Option Chain tab shall show, for the charted underlying and a selected expiry, rows per strike with
  columns: `Call OI` (lakh, with OI change % to its left), `Call LTP` (with LTP change %), `Strike` (centre), `Put LTP` (with
  change %), `Put OI` (lakh, with OI change %); percentages small and green/red. The ATM strike (nearest to underlying LTP;
  ties → lower strike) is bold; ITM cells are shaded (calls below ATM, puts above). Under each row thin OI bars: call OI
  bar extends left (red), put OI bar right (teal), length ∝ OI relative to the max OI in the expiry. An expiry dropdown
  lists all expiries of the underlying from the instrument master (weekly and monthly, e.g. NIFTY: 13 Oct, 19 Oct, 27 Oct,
  03 Nov, 09 Nov, 23 Nov, 29 Dec as of Appendix K), default nearest. All strikes of the expiry are listed; the view opens
  scrolled to centre ATM. OI change % is versus previous session's closing OI (FR-ANLY-008). Values update live for the
  strikes in the visible window ±10 strikes (selective subscription, C-07) and by `GET /quote` polling (≤1 req/s,
  500 instruments/call) for the rest at most every 60 s. Outside market hours, the last session's values are shown with
  "Market closed — as of <time>". A missing price shows "—".
- **Acceptance criteria:**
  1. Given NIFTY LTP 22,520 and 50-pt strikes, then 22,500 is bold (ATM), calls 22,450 and below are shaded, puts 22,550 and
     above are shaded.
  2. Given the expiry dropdown, then it lists exactly the NIFTY expiries in the instrument master, nearest selected.
  3. Given the user scrolls to a far strike, then its rows start live updates within 2 s and off-screen strikes are
     unsubscribed (subscription count never exceeds the configured chain budget, default 200 instruments).
  4. Given call OI 1,32,00,000, then the cell shows `132.00` (lakh) and the call bar is drawn leftward.
  5. Given Sunday, then the banner "Market closed — as of 09 Oct 2026, 15:30" is shown.
- **Data/Kite dependency:** instrument master (strikes/expiries); ticker `quote` mode (`oi`, `last_price`, change); `GET
  /quote`; previous-day OI baseline (FR-ANLY-008); subscription manager (C-07).
- **Deviation from Streak:** Live updates limited to a visible window (C-07); market-closed banner added; "—" for missing
  prices (§3.2 #19).
- **Priority hint:** Important

#### FR-ANLY-007 Option Chain row trade actions
- **Out of scope (deferred):** Option chain trade actions — options out of scope.
- **Source features:** ANLY-003
- **Requirement:** Hovering or focusing an option-chain row shall reveal `S` `B` buttons next to the strike on the call side
  and `B` `S` on the put side. Each opens the order ticket (ORD-004) — the same component as watchlist B/S — prefilled with
  that option contract, side, quantity = 1 lot (or remembered F&O qty, FR-ACCT-002). Hidden when trading is not enabled.
- **Acceptance criteria:**
  1. Given row 22,500 is focused by keyboard, then the four B/S buttons are visible and Tab-reachable.
  2. Given call-side B is activated, then the order ticket opens for the 22,500 CE of the selected expiry, BUY, 1 lot.
  3. Given signal-only mode, then no B/S buttons appear.
- **Data/Kite dependency:** order ticket (ORD-004).
- **Deviation from Streak:** Keyboard access (FR-SHELL-015); same order ticket as watchlist (resolves catalogue question).
- **Priority hint:** Important

#### FR-ANLY-008 OI tab
- **Out of scope (deferred):** Open-interest analytics — options out of scope.
- **Source features:** ANLY-004
- **Requirement:** The OI tab shall show two side-by-side grouped bar charts, each with its own expiry dropdown (default
  nearest): **Open Interest** — per strike, Call OI (red) and Put OI (teal) bars; **Open Interest Change** — per strike,
  change vs previous session closing OI, bars can be negative (drawn below zero). X-axis covers ATM ±8 strikes by default
  with a range selector (±8, ±15, all). Hover tooltip: `Strike: 22500`, `Put OI: 1.32Cr` or `Put OI change: 1.03Cr`
  (auto-scaled L/Cr). Legend entries toggle series visibility. Previous-session closing OI baseline: the stored last OI of
  the previous session from local recording, else the `oi` of the previous day's day candle from the historical API
  (`oi=1`).
- **Acceptance criteria:**
  1. Given ATM 22,500, then the OI chart shows strikes 22,100–22,900 (±8, 50-pt steps) with paired bars.
  2. Given put OI today 2.35Cr and previous close 1.32Cr at 22,500, then the change bar is +1.03Cr and the tooltip reads
     `Put OI change: 1.03Cr`.
  3. Given OI fell, then the change bar extends below zero.
  4. Given the Call OI legend is clicked, then call bars hide; clicking again shows them.
- **Data/Kite dependency:** ticker/`GET /quote` (`oi`); historical with `oi=1` for baseline; local OI recorder
  (FR-ANLY-010).
- **Deviation from Streak:** Range selector and negative-bar handling defined; baseline defined explicitly.
- **Priority hint:** Important

#### FR-ANLY-009 PCR tab
- **Out of scope (deferred):** Put-call ratio analytics — options out of scope.
- **Source features:** ANLY-005
- **Requirement:** The PCR tab shall show a dual-axis line chart over the last 5 trading sessions at 1-minute resolution:
  `Price` (underlying, blue, left axis) and `PCR` (orange, right axis), where PCR = Σ put OI ÷ Σ call OI over the strikes
  of the nearest expiry within the recorded strike window (default ATM ±20 strikes; configurable). A label top-right shows
  "Expiry <date>" (nearest expiry, no selector). Crosshair tooltip: `07 Oct 14:04`, `Price: 22,569.90`, `PCR: 0.8444`.
  Data source: the local analytics recorder (FR-ANLY-010); gaps where the app was not running are backfilled on demand from
  Kite minute candles with OI for the strikes in the window; segments that cannot be backfilled are left as gaps (no
  interpolation) and marked "No data".
- **Acceptance criteria:**
  1. Given put OI sum 8,444,000 and call OI sum 10,000,000 at 14:04, then PCR at 14:04 = 0.8444.
  2. Given the app was closed on 06 Oct, when the tab opens, then 06 Oct is backfilled (progress shown) or shown as a gap.
  3. Given hover at 07 Oct 14:04, then the tooltip shows the three lines above.
- **Data/Kite dependency:** local recorder + historical minute candles with `oi=1` (3 req/s; ≈2×41 requests per day for
  ±20 strikes).
- **Deviation from Streak:** Streak computes over its server-side history; we record locally and backfill (C-03, C-08);
  PCR over a strike window rather than guaranteed whole chain (documented in the tab's info tooltip).
- **Priority hint:** Nice-to-have

#### FR-ANLY-012 ATM IV/Straddle tab
- **Out of scope (deferred):** ATM IV / straddle analytics (needs the options pricing engine FR-OPT-038) — options out of scope.
- **Source features:** ANLY-007
- **Requirement:** The tab shall show a dual-axis line chart over the last 2 trading sessions at 1-minute resolution:
  `IV` (blue, left axis, %) = mean of the implied volatilities of the ATM CE and ATM PE of the nearest expiry, and
  `Straddle` (red, right axis, ₹) = ATM CE LTP + ATM PE LTP, where ATM is re-determined each minute from the underlying
  LTP. IV is computed with the Black-Scholes (Black-76 for futures-based pricing is a design-spec option) model with
  time to expiry in years to 15:30 IST on expiry day, risk-free rate and dividend yield taken from Settings (defaults set
  in the design spec), solved by a bounded numerical method (precision 0.01 vol-point). Tooltip: `09 Oct 12:38`,
  `IV: 12.78`, `Straddle: 242.80`. "As of <time>" top-right.
- **Acceptance criteria:**
  1. Given ATM CE 120.40 and PE 122.40, then Straddle = 242.80.
  2. Given reference inputs (spot, strike, time, rate, price), then computed IV equals the reference implementation within
     0.01.
  3. Given the underlying moves from 22,510 to 22,560 (ATM 22,500 → 22,550), then from that minute the series uses the
     22,550 strike.
- **Data/Kite dependency:** recorder (ATM CE/PE LTPs), local IV engine (Kite provides no IV).
- **Deviation from Streak:** IV model and parameters are documented and configurable (Streak's undisclosed).
- **Priority hint:** Nice-to-have

#### FR-ANLY-001-opt Options analytics tabs
- **Out of scope (deferred):** options out of scope.
- **Source features:** ANLY-001
- **Requirement (removed text):** Tabs Option Chain, OI, PCR, ATM IV/Straddle appear only if the instrument is an index or stock with
  listed options (NFO/BFO) in the instrument master; futures charts show the tabs of their underlying's option set except
  Advance/Decline; option contracts show Technicals only.
- **Acceptance criteria:** (1) Given NIFTY 50 with a constituent list, then all six tabs are shown. (2) Given RELIANCE (F&O stock),
  then five tabs are shown (no Advance/Decline). (3) Given OI tab active on NIFTY 50, when the chart switches to an option, then
  Technicals becomes active.
- **Data/Kite dependency:** instrument master (options), universe store.
- **Deviation from Streak:** Non-F&O and futures tab sets defined explicitly.
- **Priority hint:** Important

#### FR-ANLY-010-opt Options series in the analytics recorder
- **Out of scope (deferred):** options out of scope.
- **Source features:** ANLY-004, ANLY-005, ANLY-007
- **Requirement (removed text):** The recorder also stores, per tracked underlying, per-strike OI and LTP for the nearest expiry
  within ATM ±20 strikes and the ATM CE/PE LTPs, and each session's closing OI per contract (OI baseline for FR-ANLY-008);
  backfill uses option minute candles with `oi=1` (≈2 × 41 requests per day for ±20 strikes).
- **Acceptance criteria:** Given the app runs 09:15–15:30, then 375 OI snapshots per tracked underlying exist.
- **Data/Kite dependency:** ticker (`oi`), historical `oi=1`.
- **Deviation from Streak:** Required because Kite offers no PCR/IV history.
- **Priority hint:** Important

#### FR-ANLY-013-opt Refresh cadence of options tabs
- **Out of scope (deferred):** options out of scope.
- **Source features:** ANLY-003
- **Requirement (removed text):** Option Chain refreshes live (FR-ANLY-006); OI, PCR and IV tabs refresh every minute during
  market hours; a hidden PCR tab makes no PCR-specific quote calls.
- **Acceptance criteria:** Given the PCR tab is not active, then no PCR-specific quote calls are made (recorder continues).
- **Data/Kite dependency:** as per tab.
- **Deviation from Streak:** Staleness made visible.
- **Priority hint:** Important


### 10.5 Module WATCH

#### FR-WATCH-009-opt Option universes
- **Out of scope (deferred):** options out of scope.
- **Source features:** WATCH-006
- **Requirement (removed text):** Eight further built-in universes, placed after "NFO Equities" in Streak's 29-entry order: Nifty
  Weekly Options, Sensex Weekly Options, Nifty Next Week Options, Sensex Next Week Options, Nifty Monthly Options, Sensex Monthly
  Options, Banknifty Monthly Options, Finnifty Monthly Options = all CE/PE contracts of the named underlying for the current weekly /
  next weekly / current monthly expiry derived from the instrument master, narrowed by the scanner's strike-range filter (default
  ATM ±6, FR-SCAN-002-opt). Option-basket labels in pills may use short forms (e.g. "Nifty current week").
- **Acceptance criteria:** (1) Given today is before the weekly expiry, then "Nifty Weekly Options" contains only that expiry's NIFTY
  CE/PE contracts. (2) Given a weekly expiry has passed, then "Nifty Next Week Options" resolves to the following expiry
  automatically.
- **Data/Kite dependency:** instrument master incl. options; subscription budget (C-07).
- **Deviation from Streak:** none.
- **Priority hint:** Core

#### FR-WATCH-013-opt Option display naming
- **Out of scope (deferred):** options out of scope.
- **Source features:** WATCH-008
- **Requirement (removed text, with the RV-024 correction applied):** One own convention for options: monthly `UNDERLYING MON
  STRIKE CE|PE` (e.g. `BANKNIFTY OCT 55300 CE`); weekly `UNDERLYING DD MON (W) STRIKE CE|PE` (e.g. `NIFTY 13 OCT (W) 22500 CE`);
  dynamic legs `UNDERLYING <rule> CE|PE` (e.g. `NIFTY ATM+2 CE`). Options carry a circular C (calls) or P (puts) badge in result
  lists. A contract is weekly if its expiry is not the last expiry of that underlying in that month. Streak's ordinal/superscript
  form (`13ᵗʰ ʷ`) is not reproduced (clean-room).
- **Acceptance criteria:** Given NIFTY expiry 2026-10-13 strike 22500 CE (a weekly), then the display is `NIFTY 13 OCT (W) 22500 CE`.
- **Data/Kite dependency:** instrument master (`expiry`, `strike`, `instrument_type`).
- **Deviation from Streak:** Own naming convention (RV-024).
- **Priority hint:** Core


### 10.6 Module ACCT

#### FR-ACCT-002-opt Remember option order quantity
- **Out of scope (deferred):** options out of scope.
- **Source features:** ACCT-002
- **Requirement (removed text):** The toggle also covers NFO-OPT and BFO-OPT; quantity in lots is stored per underlying and segment
  type (options/futures); the next option order ticket for the same underlying is prefilled with that number of lots.
- **Acceptance criteria:** Given the toggle on and a NIFTY option order of 3 lots submitted, when another NIFTY option order ticket
  opens, then quantity = 3 lots.
- **Data/Kite dependency:** local preference; instrument master lot sizes.
- **Deviation from Streak:** Storage key defined explicitly.
- **Priority hint:** Nice-to-have

#### FR-ACCT-012-opt Analytics (options) settings group
- **Out of scope (deferred):** options out of scope.
- **Source features:** ACCT-001
- **Requirement (removed text):** Settings group Analytics: risk-free rate, dividend yield (FR-OPT-038), recorder strike window,
  option-chain subscription budget (default 200 instruments).
- **Acceptance criteria:** Given risk-free rate set to "abc", then an inline error is shown and the old value is kept.
- **Data/Kite dependency:** local only.
- **Deviation from Streak:** Streak has no settings page.
- **Priority hint:** Core


### 10.7 Module EQB

#### FR-EQB-002-opt Options body of the strategy model
- **Out of scope (deferred):** options out of scope.
- **Source features:** EQB-006
- **Requirement (removed text):** `StrategyDoc.kind` also admits `"options"` with `options?: OptionsBody` (FR-OPT-002); "every
  strategy (equity and options)" is persisted in the same model.
- **Acceptance criteria:** Given every library option template applied to NIFTY, when saved, then the docs validate against the
  schema.
- **Data/Kite dependency:** local only.
- **Deviation from Streak:** documented, versioned model.
- **Priority hint:** Core


### 10.8 Module DISC

#### FR-DISC-016 Options-universe filters
- **Out of scope (deferred):** Options-universe filters — options out of scope.
- **Source features:** DISC-014
- **Requirement:** For option universes the pane shows Filter by checkboxes **Calls**, **Puts**, **Volume > 0** (all on by default) and a
  **Strike range** dropdown: ATM ± 1 … ATM ± 30 and All strikes (default ATM ± 6). ATM is the strike nearest current spot (FR-OPT-018 rule).
  Result rows show a C/P badge. Filters apply to the scanned set before evaluation (to limit subscriptions, C-07) and to the displayed
  results.
- **Acceptance criteria:**
  1. Given ATM ± 6, then at most 13 strikes × selected sides are scanned.
  2. Given Puts unchecked, then no PE contracts appear.
  3. Given Volume > 0 on, then contracts with zero day volume are excluded.
- **Data/Kite dependency:** Spot LTP; instruments dump; quotes/ticker for option contracts.
- **Deviation from Streak:** none.
- **Priority hint:** Important

#### FR-DISC-002-opt Options strategies in the library
- **Out of scope (deferred):** options out of scope (Streak's Discover has no options strategies either).
- **Source features:** DISC-001, DISC-007
- **Requirement (removed text):** Library strategies may have category Option buying / Option selling; the library may include
  options examples (an enhancement over Streak, whose 45 are all Basic equity).
- **Acceptance criteria:** Given options items exist, then option tabs appear (FR-DISC-003-opt).
- **Data/Kite dependency:** local only.
- **Deviation from Streak:** Streak's Discover has no options strategies; ours could.
- **Priority hint:** Nice-to-have

#### FR-DISC-003-opt Option category tabs
- **Out of scope (deferred):** options out of scope.
- **Source features:** DISC-002
- **Requirement (removed text):** Tabs **Option buying · Option selling** are shown only when the library contains options
  strategies.
- **Acceptance criteria:** Given no options items, then option tabs are hidden.
- **Data/Kite dependency:** local only.
- **Deviation from Streak:** Option tabs conditional (Streak's Discover has none).
- **Priority hint:** Nice-to-have


### 10.9 Module MYS

#### FR-MYS-002-opt Option tabs in My Strategies
- **Out of scope (deferred):** options out of scope.
- **Source features:** MYS-002
- **Requirement (removed text):** Tabs **Option buying · Option selling** with counts; options strategies appear under their
  category per the FR-OPT-035 classification rule.
- **Acceptance criteria:** Given a short strangle saved, then it appears under Option selling and All.
- **Data/Kite dependency:** local only.
- **Deviation from Streak:** Classification rule documented.
- **Priority hint:** Important

#### FR-MYS-004-opt Options strategy detail pane
- **Out of scope (deferred):** options out of scope.
- **Source features:** MYS-004
- **Requirement (removed text):** Options strategies show the payoff summary (max profit/loss, break-evens at last save) instead of
  backtest sections; the footer "Run backtest" is hidden for them (Streak has no options backtest).
- **Acceptance criteria:** Given an options strategy, then no Run backtest action is present.
- **Data/Kite dependency:** quotes at last save.
- **Deviation from Streak:** none.
- **Priority hint:** Core


### 10.10 Module BT

#### FR-BT-038-opt Options backtesting (v1 enhancement idea)
- **Out of scope (deferred):** options out of scope; Streak has no options backtest either (A06 §1, confirmed 3 Sep 2026). Resolves
  RV-001 by removal.
- **Source features:** BT-026
- **Requirement (removed text):** **Enhancement (beyond Streak):** options backtesting for strategies built in the options builder,
  limited to contracts whose historical candles are locally available or fetchable (live contracts; expired-contract intraday
  data is UNVERIFIED, A07 §A4), with the same engine rules applied per leg and combined SL/TP on summed leg P&L.
- **Acceptance criteria:** Given an options strategy whose contracts lack history, then the run reports which legs lack data
  instead of silently producing no trades.
- **Data/Kite dependency:** `GetHistoricalData` for NFO/BFO options; expired-contract history (OQ-§10-1).
- **Deviation from Streak:** Streak supports equities and futures only.
- **Priority hint:** Nice-to-have


### 10.11 Module DEP

#### FR-DEP-008-opt Combined SL/TP for multi-leg option strategies
- **Out of scope (deferred):** options out of scope.
- **Source features:** DEP-006
- **Requirement (removed text):** For options strategies, combined SL/TP on the summed leg P&L raises one exit trigger for all
  legs; on exit, short legs are closed before long hedge legs so a hedge is never removed while its short leg is open.
- **Acceptance criteria:** Given a combined SL of ₹2,000 on a 2-leg strategy, then one exit trigger fires when summed MTM ≤ −2,000.
- **Data/Kite dependency:** ticker LTP of option legs.
- **Deviation from Streak:** none.
- **Priority hint:** Core


### 10.12 Module SCAN

#### FR-SCAN-002-opt Option universes and option filters in "Scan on"
- **Out of scope (deferred):** options out of scope.
- **Source features:** SCAN-002, DISC-014
- **Requirement (removed text):** Option universes (Nifty/Sensex weekly, next-week, monthly; Banknifty/Finnifty monthly) resolve at
  run time to the contracts of the nearest / second-nearest weekly or the current monthly expiry, filtered by strike range around
  ATM (default −6…+6 strikes, options 1…30 and Show All) and by CALL / PUT / VOLUME>0 (all on by default).
- **Acceptance criteria:** Given "Nifty Weekly Options", spot 22,520, strike step 50, range −6…+6, CALL+PUT, then 26 contracts
  (13 strikes × 2) of the nearest weekly expiry are scanned before the VOLUME>0 filter.
- **Data/Kite dependency:** instrument master incl. options; spot LTP for ATM.
- **Deviation from Streak:** none.
- **Priority hint:** Core

#### FR-SCAN-016-opt Option filters in the scanner document
- **Out of scope (deferred):** options out of scope.
- **Source features:** SCAN-008
- **Requirement (removed text):** `ScannerDoc.optionFilters?: { calls: bool, puts: bool, volumeGt0: bool, strikeRange: int(1..30) |
  "all" }` and the eight option universe ids in `UniverseRef.builtin`; import of a `.tlscanner.json` restores option filters.
- **Acceptance criteria:** Given an exported scanner with Puts unchecked and strike range 10, when imported, then both are restored.
- **Data/Kite dependency:** local only.
- **Deviation from Streak:** documented schema.
- **Priority hint:** Core


### 10.13 Module SCALP

#### FR-SCALP-004 Option chain drawer
- **Out of scope (deferred):** Option chain drawer — options out of scope; the Scalper is adapted to equity/futures (FR-SCALP-001).
- **Source features:** SCALP-004
- **Requirement:** The option chain drawer shall show: underlying search, expiry selector (all listed expiries, default nearest),
  OI and Greeks toggles; columns Call OI (lakh, change %), Call LTP (change %), Strike, Put LTP, Put OI, with ITM shading, bold ATM
  and OI bars; with Greeks on, per side Delta, Gamma, Theta, Vega, IV computed locally (Black-Scholes on spot, time to 15:30 of
  expiry day, configurable risk-free rate and dividend yield; IV solved from LTP), validated against golden vectors (NFR-06).
  Row actions (visible on focus/hover and via keyboard): load into the right chart, toggle B or S leg in the basket. Only strikes in
  the visible window are subscribed (C-07).
- **Acceptance criteria:**
  1. Given a reference option (S, K, T, r, σ) from the golden set, then computed Delta/Gamma/Theta/Vega match to 4 decimals.
  2. Given scrolling to far strikes, then subscriptions update and total subscribed instruments for the chain stay ≤ visible rows × 2 + 10.
  3. Given B on a row twice, then the leg is added then removed.
- **Data/Kite dependency:** Ticker (quote mode with OI); instruments dump.
- **Deviation from Streak:** Greeks computed locally — values may differ slightly from Streak's; keyboard access added.
- **Priority hint:** Important

#### FR-SCALP-006 Option strategy templates in the basket
- **Out of scope (deferred):** Option strategy templates in the basket — options out of scope.
- **Source features:** SCALP-006
- **Requirement:** The basket panel shall offer the 34 templates of FEATURES Appendix G with filters All / Bullish / Bearish /
  Neutral; selecting a template fills legs around ATM of the selected expiry with one lot each. If legs exist, the user confirms
  replacement first.
- **Acceptance criteria:**
  1. Given NIFTY spot 22,520 and "Short Iron Condor", then legs are B 22100 PE, S 22300 PE, S 22700 CE, B 22900 CE (Appendix G).
  2. Given existing legs, then a confirmation appears before replacement (§3.2 #23 not replicated).
- **Data/Kite dependency:** Instruments dump; LTP.
- **Deviation from Streak:** Replacement confirmation.
- **Priority hint:** Important

#### FR-SCALP-007 Basket payoff graph and table
- **Out of scope (deferred):** Basket payoff graph and table (options pricing) — options out of scope.
- **Source features:** SCALP-007
- **Requirement:** The basket shall show a payoff graph (x = underlying price range, y = P&L) with "Target date" curve
  (Black-Scholes value at the chosen date with current IVs) and "Expiry" curve (intrinsic), current-price marker, profit/loss
  shading and projected P&L at current price (₹ and % of margin); and a payoff table with Target / On-expiry P&L at configurable
  price steps (default 10 points).
- **Acceptance criteria:**
  1. Given a single long 22500 CE bought at 137.05 × 65, then expiry P&L at 22,400 = −8,908.25 and at 22,700 = (200 − 137.05) × 65 = 4,091.75.
- **Data/Kite dependency:** none (local maths).
- **Deviation from Streak:** none.
- **Priority hint:** Important

#### FR-SCALP-001-opt Options layout of the Scalper
- **Out of scope (deferred):** options out of scope; Scalper adapted to equity/futures (FR-SCALP-001).
- **Source features:** SCALP-001
- **Requirement (removed text):** Right chart = ATM call of the nearest expiry (e.g. spot 22,520, step 50 → 22500 CE); edge tab
  "Option Chain" (FR-SCALP-004); the primary (left) chart accepts only an index or stock.
- **Acceptance criteria:** Given NIFTY spot 22,520 and strike step 50, then the right chart defaults to the nearest-expiry 22500 CE.
- **Data/Kite dependency:** instruments dump (expiries, strikes).
- **Deviation from Streak:** Local persistence.
- **Priority hint:** Important

#### FR-SCALP-002-opt CE⇄PE flip
- **Out of scope (deferred):** options out of scope.
- **Source features:** SCALP-002
- **Requirement (removed text):** Each option chart header has a CE⇄PE flip (same strike and expiry).
- **Acceptance criteria:** Given right chart 22500 CE, when flipped, then it shows 22500 PE of the same expiry.
- **Data/Kite dependency:** instruments dump.
- **Deviation from Streak:** none.
- **Priority hint:** Important

#### FR-SCALP-003-opt Third chart = ATM put
- **Out of scope (deferred):** options out of scope.
- **Source features:** SCALP-003
- **Requirement (removed text):** Add Chart inserts a third chart showing the ATM put of the same expiry as the right chart.
- **Acceptance criteria:** Given Add Chart with ATM 22500, then the third chart shows the 22500 PE.
- **Data/Kite dependency:** instruments dump.
- **Deviation from Streak:** none.
- **Priority hint:** Important

#### FR-SCALP-005-opt Option legs in the basket panel
- **Out of scope (deferred):** options out of scope.
- **Source features:** SCALP-005
- **Requirement (removed text):** Basket legs are option contracts added from the option chain drawer (B/S toggles per row,
  FR-SCALP-004); product MIS/NRML; the user is advised to place hedges (buy legs) first for the margin benefit.
- **Acceptance criteria:** Given B on an option-chain row twice, then the leg is added then removed.
- **Data/Kite dependency:** `GetBasketMargins`.
- **Deviation from Streak:** none.
- **Priority hint:** Important


### 10.14 Module ORD

#### FR-ORD-004-opt Option contracts in the order ticket
- **Out of scope (deferred):** options out of scope.
- **Source features:** ORD-004
- **Requirement (removed text):** Entry points include option chains (dashboard tab, Scalper drawer, options builder drawer);
  quantity in lots for NFO-OPT/BFO-OPT contracts; example: an NFO option with lot 65 and qty 100 is blocked with "quantity must
  be a multiple of 65".
- **Acceptance criteria:** Given call-side B on the 22,500 row, then the ticket opens for the 22,500 CE of the selected expiry, BUY,
  1 lot.
- **Data/Kite dependency:** `PlaceOrder`.
- **Deviation from Streak:** none.
- **Priority hint:** Core


### 10.15 Module OPT

#### FR-OPT-001 Options builder layout and new-draft defaults
- **Out of scope (deferred):** Options strategy builder — all options features are out of scope (user decision 2026-10-12).
- **Source features:** OPT-001
- **Requirement:** The system shall provide an Options Strategy Builder (sidebar "Options strategy") with: header (inline-editable
  title, "My Strategies" button); left scrolling column with cards **Entry**, **Execution** (legs), **Exit**; sticky footer **Save** and
  **Deploy** (FR-OPT-032); right column with Payoff graph / Payoff table tabs, "Option chain" link, legend, target-price
  controls, stats and margins; two collapsible side drawers "Underlying chart" and "Option charts". New-draft defaults: entry mode
  Underlying-based; underlying NIFTY 50; timeframe 1 minute; one empty Basic condition row; one leg = Buy, CE, ATM of the nearest
  expiry, 1 lot (lot size from the instruments dump), included in payoff; payoff computed immediately when quotes are available.
  No Backtest action is offered for options strategies.
- **Acceptance criteria:**
  1. Given online with quotes, when a new options draft opens, then there is one leg B · NIFTY <nearest expiry> <ATM strike> CE · qty = 1 lot and the payoff shows Max Profit "Unlimited" and Max Loss = −(premium × qty).
  2. Given offline, when a new draft opens, then the default leg is created from the cached instruments dump, LTP shows "—" with an offline badge, and payoff shows "Quotes unavailable".
  3. Given the options builder, then no Backtest control exists in footer or Execute.
- **Data/Kite dependency:** Instruments dump (NFO/BFO: expiry, strike, lot_size); `GET /quote` or WebSocket for LTP.
- **Deviation from Streak:** Offline state explicit (NFR-14). Options backtesting absent in both (BT-026).
- **Priority hint:** Core

#### FR-OPT-002 Options strategy model extension
- **Out of scope (deferred):** Options strategy builder — all options features are out of scope (user decision 2026-10-12).
- **Source features:** OPT-002, OPT-006, OPT-007, OPT-008, OPT-011, OPT-016, OPT-017, OPT-019, OPT-020, OPT-021
- **Requirement:** The strategy model (FR-EQB-002) shall define `OptionsBody`:
  ```
  OptionsBody { entry: { mode: "underlying"|"time",
                         underlyings: [ { instrument: InstrumentRef, timeframe: TF(1m..1d), conditions: ConditionBlock } ] (1..2),
                         combine: "and"|"or",                       // how 2 underlying blocks combine; default "and"
                         time?: { at: "HH:MM", weekdays: [1..5] } },
                legs: [ Leg ],
                exit: { combined?: { sl?: number, tp?: number },     // ₹, strategy-level
                        conditions?: { instrument: InstrumentRef, timeframe: TF, conditions: ConditionBlock },
                        time?: { at: "HH:MM", weekdays: [1..5] } } }
  Leg { id, order: int, side: "buy"|"sell", optionType: "CE"|"PE", underlying: InstrumentRef,
        expiry: { date: "YYYY-MM-DD" } | { relative: "week+0"|"week+1"|"month+0"|"month+1" },
        strike: { fixed: number } | { atm: int } | { premium: { op: "eq"|"gt"|"lt", value: number } },
        lots: int ≥ 1, includeInPayoff: bool, risk?: Risk }
  ```
  All legs share one underlying. Validation codes cover: leg count ≥ 1; expiry not in the past; fixed strike exists in the dump;
  time within session (FR-OPT-008/024); conditions complete.
- **Acceptance criteria:**
  1. Given every library template applied to NIFTY, when saved, then the resulting docs validate against the schema.
  2. Given a leg with fixed expiry before today, when Deploy is attempted, then error `EXPIRY_PAST` names the leg.
  3. Given a doc with 3 underlying blocks, then validation fails with `TOO_MANY_UNDERLYINGS`.
- **Data/Kite dependency:** Local; instruments dump for validation.
- **Deviation from Streak:** Explicit combine operator and optional relative expiry (see FR-OPT-007, FR-OPT-020).
- **Priority hint:** Core

#### FR-OPT-003 Entry mode selector
- **Out of scope (deferred):** Options strategy builder — all options features are out of scope (user decision 2026-10-12).
- **Source features:** OPT-002
- **Requirement:** The Entry card shall have a segmented control **Underlying-based** (default) / **Time-based**. Switching swaps the card
  body; legs and exits are untouched; the inactive mode's settings are retained in the draft (restored when switching back) but only the
  active mode is saved and evaluated.
- **Acceptance criteria:**
  1. Given conditions entered, when switching to Time-based and back, then the conditions are still present.
  2. Given Time-based is active at save, then the saved doc has `entry.mode = "time"` and no evaluation of underlying conditions occurs.
- **Data/Kite dependency:** Local only.
- **Deviation from Streak:** Draft retention across switch (Streak behaviour not observed).
- **Priority hint:** Core

#### FR-OPT-004 Underlying picker
- **Out of scope (deferred):** Options strategy builder — all options features are out of scope (user decision 2026-10-12).
- **Source features:** OPT-003
- **Requirement:** A searchable combobox shall list every underlying that has options in NFO/BFO in the current instruments dump
  (indices such as NIFTY 50, NIFTY BANK, NIFTY FIN SERVICE, SENSEX, BANKEX, and any other present, e.g. MIDCPNIFTY; plus all F&O
  stocks), each with a consistent exchange tag derived from the dump (index → "INDEX", stock → its primary cash exchange "NSE"/"BSE").
  The same picker is used in the leg drawer and exit block. The picker is locked (read-only with an explanation) in the leg drawer
  while legs exist (FR-OPT-015).
- **Acceptance criteria:**
  1. Given the instruments dump, then the picker's list equals the distinct option underlyings in the dump (CI fixture test).
  2. Given "bank" typed, then NIFTY BANK and BANKEX appear.
  3. Given the same stock in two pickers, then the exchange tag is identical.
- **Data/Kite dependency:** `GET /instruments` (NFO, BFO, NSE, BSE), cached daily.
- **Deviation from Streak:** Exchange tags consistent (§3.2 #18); underlying list derived from data (not fixed).
- **Priority hint:** Core

#### FR-OPT-005 Underlying timeframe
- **Out of scope (deferred):** Options strategy builder — all options features are out of scope (user decision 2026-10-12).
- **Source features:** OPT-004
- **Requirement:** Each underlying block has a timeframe dropdown with the 8 base timeframes (default 1 minute). Up to two distinct
  timeframes may exist across underlying blocks (one per block); evaluation timing follows FR-OPT-034.
- **Acceptance criteria:**
  1. Given the dropdown, then exactly 1m, 3m, 5m, 10m, 15m, 30m, 1h, 1d are offered.
  2. Given block 1 = 3m and block 2 = 5m, then the builder shows "Evaluated every 15 minutes".
- **Data/Kite dependency:** Kite historical intervals.
- **Deviation from Streak:** none.
- **Priority hint:** Core

#### FR-OPT-006 Underlying conditions block
- **Out of scope (deferred):** Options strategy builder — all options features are out of scope (user decision 2026-10-12).
- **Source features:** OPT-005
- **Requirement:** Each underlying block embeds the full condition editor of the equity builder (FR-EQB-008 … FR-EQB-031 behaviours:
  Basic/Advanced with lossless switch, suggestions, quick-setup tiles, groups, clipboard, timeframes), evaluated on the underlying's
  candles. A "Enter at next evaluation" helper inserts an always-true condition (`Close[0] higher than 0`) for discretionary entry.
- **Acceptance criteria:**
  1. Given the block, then every control of the equity conditions editor is present and behaves per its FR (shared component test).
  2. Given "Enter at next evaluation" is used, then the block contains `Close[0] higher than 0` and evaluates true on every instant with data.
- **Data/Kite dependency:** Historical candles of the underlying (index/stock) via local store.
- **Deviation from Streak:** No 5-condition cap (NFR-13; Appendix N).
- **Priority hint:** Core

#### FR-OPT-007 Second underlying and block combination
- **Out of scope (deferred):** Options strategy builder — all options features are out of scope (user decision 2026-10-12).
- **Source features:** OPT-006
- **Requirement:** "Add underlying" adds a second block (own underlying, timeframe, conditions); each block then has Remove (confirm if
  non-empty). Max 2 blocks; the action is hidden at 2. Between the blocks a visible **all must match (AND)** / **any may match (OR)**
  selector (default AND) defines combination.
- **Acceptance criteria:**
  1. Given 2 blocks with AND, block1 true, block2 false, then no entry.
  2. Given OR, block1 false, block2 true, then entry fires.
  3. Given 2 blocks, then "Add underlying" is not shown.
- **Data/Kite dependency:** Historical candles for both instruments.
- **Deviation from Streak:** Streak has no visible combinator (OPT-006 open question); we add an explicit one defaulting to AND.
- **Priority hint:** Important

#### FR-OPT-008 Time-based entry
- **Out of scope (deferred):** Options strategy builder — all options features are out of scope (user decision 2026-10-12).
- **Source features:** OPT-007
- **Requirement:** In Time-based mode, the card shows hour (01–12), minute (00–59), AM/PM, and weekday chips Mon–Fri (all selected by
  default; ≥1 required). Default time 09:20 AM. The time must lie within 09:15–15:29 IST; otherwise an inline error blocks Save/Deploy.
  Entry fires once per selected weekday that is an exchange trading day (local, user-updatable holiday calendar), at the start of the
  chosen minute, only while the app is running (C-03); if the app was not running at that minute, the entry is recorded as "missed" in
  the deployment log and not placed late.
- **Acceptance criteria:**
  1. Given 9:15 PM, then error "Time must be between 9:15 AM and 3:29 PM" and Save is blocked.
  2. Given all weekday chips deselected, then error "Select at least one weekday".
  3. Given a deployment at 09:20 Mon–Fri and a simulated clock, then exactly one entry signal per trading day at 09:20:00 and none on a holiday.
  4. Given the app started at 09:25, then the 09:20 entry is logged as missed and no order/signal is generated.
- **Data/Kite dependency:** Local scheduler, holiday calendar.
- **Deviation from Streak:** Session-time validation (§3.2 #22); default 09:20 rather than 09:15 (avoids the opening tick); missed-entry handling due to C-03.
- **Priority hint:** Core

#### FR-OPT-009 Execution card — leg table
- **Out of scope (deferred):** Options strategy builder — all options features are out of scope (user decision 2026-10-12).
- **Source features:** OPT-008
- **Requirement:** The Execution card shall list legs in execution order with columns: include-in-payoff checkbox · B/S badge
  (toggleable) · instrument label · lots/quantity · LTP · exit (Add SL/TP) · delete. Instrument labels follow one naming format:
  `<UNDERLYING> <DD MMM> <W|M> <STRIKE> <CE|PE>` for fixed legs and `<UNDERLYING> <DD MMM> <rule> <CE|PE>` for dynamic legs (e.g. `ATM+2`,
  `Premium≈75`) with the currently resolved contract shown beneath. Clicking a label opens the leg drawer at that leg. An "Add legs"
  action opens the drawer. LTP is live; when LTP is unavailable or zero the cell shows "—" with a tooltip giving the reason (no quote,
  no trades today, offline).
- **Acceptance criteria:**
  1. Given a leg whose contract has no trades today, then LTP shows "—" with tooltip "No trades today", not 0.0.
  2. Given a dynamic ATM+1 PE leg, then the label shows `ATM+1` and the resolved contract (e.g. `NIFTY 13 OCT 22550 PE`) beneath.
  3. Given the B badge is clicked, then the leg becomes S and payoff updates.
- **Data/Kite dependency:** Ticker (quote mode) or `GET /quote` for LTP; instruments dump.
- **Deviation from Streak:** No silent 0.0 LTP (§3.2 #19).
- **Priority hint:** Core

#### FR-OPT-010 Include-in-payoff checkbox
- **Out of scope (deferred):** Options strategy builder — all options features are out of scope (user decision 2026-10-12).
- **Source features:** OPT-009
- **Requirement:** Each leg's checkbox (default checked) controls inclusion in payoff graph, table and stats. **Unchecked legs are
  excluded from payoff only; they are still part of the strategy and are traded/signalled.** The column header tooltip states this.
  Premium totals (FR-OPT-013) include all legs.
- **Acceptance criteria:**
  1. Given a straddle with the PE leg unchecked, then Max Profit/Loss equal those of the CE leg alone, and the Premium total still includes both.
  2. Given a deploy signal, then both legs appear in the signal regardless of checkbox state.
- **Data/Kite dependency:** Local only.
- **Deviation from Streak:** Trading behaviour of unchecked legs made explicit (OPT-009 open question).
- **Priority hint:** Important

#### FR-OPT-011 Leg quantity in lots
- **Out of scope (deferred):** Options strategy builder — all options features are out of scope (user decision 2026-10-12).
- **Source features:** OPT-010
- **Requirement:** Quantity is chosen in lots: a dropdown of 1–20 lots (showing both lots and units, e.g. "2 lots · 130") plus a "Custom"
  numeric entry (integer ≥ 1). Lot size comes from the instruments dump for the leg's contract. If units exceed the exchange freeze
  quantity (from a local, user-updatable freeze-limit table), the row shows an info note that orders will be sliced (autoslice) at
  placement.
- **Acceptance criteria:**
  1. Given NIFTY lot size 65, then the dropdown shows 65 … 1,300 units for 1–20 lots.
  2. Given Custom = 40 lots, then the leg saves `lots: 40` and shows the slicing note if 2,600 exceeds the freeze limit.
- **Data/Kite dependency:** Instruments dump `lot_size`; orders use `autoslice=true` (ORD module).
- **Deviation from Streak:** No 20-lot cap (NFR-13).
- **Priority hint:** Core

#### FR-OPT-012 Per-leg SL / TP / TSL
- **Out of scope (deferred):** Options strategy builder — all options features are out of scope (user decision 2026-10-12).
- **Source features:** OPT-011
- **Requirement:** "Add SL/TP" on a leg expands a sub-row with SL, TP, TSL inputs (> 0, optional) and a unit selector Percent (default) /
  Points / ₹ P&L; "Remove SL/TP" collapses and clears it. Semantics are direction-aware on the option premium: for a **buy** leg SL
  triggers when premium ≤ entry × (1 − SL%) (or entry − pts, or leg P&L ≤ −₹), TP when premium ≥ entry × (1 + TP%); for a **sell** leg SL
  triggers when premium ≥ entry × (1 + SL%) and TP when premium ≤ entry × (1 − TP%) (Points/₹ analogous); TSL trails the best favourable
  premium. Percent TP on a sell leg is limited to ≤ 100. In live mode these produce exit signals/alerts per DEP-006.
- **Acceptance criteria:**
  1. Given a sell leg entered at 100 with SL 20%, when premium reaches 120, then an SL exit triggers; at 119.95 it does not.
  2. Given a buy leg entered at 100 with TSL 10 pts, when premium rises to 130 then falls to 120, then TSL triggers at 120.
  3. Given TP = 150% on a sell leg, then the inline error "Must be at most 100% for sell legs" appears.
- **Data/Kite dependency:** Live premiums from ticker during deployment (DEP module).
- **Deviation from Streak:** Sell-leg semantics defined (OPT-011 open question).
- **Priority hint:** Core

#### FR-OPT-013 Premium and total premium readout
- **Out of scope (deferred):** Options strategy builder — all options features are out of scope (user decision 2026-10-12).
- **Source features:** OPT-012
- **Requirement:** The Execution card footer shall show **Net premium per unit** = Σ(sell LTP × lot-ratio) − Σ(buy LTP × lot-ratio) labelled
  "credit" when positive and "debit" when negative, and **Total premium** = Σ over legs of (±LTP × quantity in units) with the same sign
  convention, formatted in ₹. Values update with live LTP. Legs without LTP make the totals show "—" with a tooltip naming the legs.
- **Acceptance criteria:**
  1. Given a short straddle with CE 137.05 and PE 74.70, 1 lot of 65 each, then net premium = 211.75 credit and total = ₹13,763.75 credit.
  2. Given a long call at 137.05 × 65, then net premium 137.05 debit, total ₹8,908.25 debit.
  3. Given a leg without LTP, then totals show "—".
- **Data/Kite dependency:** Quotes.
- **Deviation from Streak:** Sign convention explicit (OPT-012 open question).
- **Priority hint:** Important

#### FR-OPT-014 Clear all, delete leg and empty state
- **Out of scope (deferred):** Options strategy builder — all options features are out of scope (user decision 2026-10-12).
- **Source features:** OPT-013
- **Requirement:** "Clear all" removes all legs after a confirmation; each leg has delete (undoable for 10 s via toast "Undo"). With no
  legs, the payoff pane shows an empty state ("No legs yet — add option legs to see the payoff") with an "Add legs" action; stats show
  "—"; margin needed 0. After Clear all the underlying becomes editable in the drawer.
- **Acceptance criteria:**
  1. Given 4 legs, when Clear all is confirmed, then 0 legs remain and the empty state is shown.
  2. Given a leg deleted, when Undo is pressed within 10 s, then it is restored at its original position.
- **Data/Kite dependency:** Local only.
- **Deviation from Streak:** Confirmation and undo added.
- **Priority hint:** Important

#### FR-OPT-015 Leg-picker drawer
- **Out of scope (deferred):** Options strategy builder — all options features are out of scope (user decision 2026-10-12).
- **Source features:** OPT-014
- **Requirement:** A left slide-over drawer (opened by "Add legs", clicking a leg, or the payoff pane "Option chain" link) shall contain:
  segmented tabs **Option chain** | **Rule-based strike**; the underlying picker (locked while legs exist, with tooltip "Remove all legs to
  change the underlying"); footer "Clear all" and "Done". Escape and Done close the drawer. The drawer footer buttons shall not overlap
  the page footer's Deploy button position (the page footer is hidden or disabled while the drawer is open).
- **Acceptance criteria:**
  1. Given legs exist, then the drawer's underlying field is read-only with the tooltip.
  2. Given the drawer is open, then the page's Deploy button is not clickable (hit-test test at Done's coordinates returns the drawer button).
  3. Given Escape pressed, then the drawer closes and staged legs are kept (drawer edits apply immediately).
- **Data/Kite dependency:** See FR-OPT-016/018/019.
- **Deviation from Streak:** Mis-click hazard removed (§3.2 #17).
- **Priority hint:** Core

#### FR-OPT-016 Drawer — option chain
- **Out of scope (deferred):** Options strategy builder — all options features are out of scope (user decision 2026-10-12).
- **Source features:** OPT-015
- **Requirement:** The Option chain tab shall show: expiry dropdown (all expiries of the underlying from the dump, absolute dates with a
  W/M marker; default nearest); toggles **OI** (on) and **Greeks** (off). Columns with OI on: Call OI (lakh) + % change, Call LTP + %
  change, **Strike** (ATM highlighted), Put LTP + % change, Put OI (lakh) + % change, with proportional OI bars; ITM cells shaded. Greeks
  on adds per side: Delta, Gamma, Theta, Vega, IV (FR-OPT-017), with horizontal scrolling. Each LTP cell has always-visible (or
  focus-revealed) Buy and Sell buttons adding a fixed leg (1 lot); strikes already used show a B/S marker. Strike window: ATM ± 20
  strikes by default with "Show more". Strike step is derived from the dump. Data refreshes live (WebSocket) for the visible window.
- **Acceptance criteria:**
  1. Given NIFTY and the nearest expiry, when the drawer opens, then the strike column is ascending, ATM (strike nearest spot) is highlighted, and 41 rows are shown.
  2. Given Sell on 22500 CE, then a leg S · 22500 CE · 1 lot is added and the chain row shows an S marker.
  3. Given Greeks toggled on, then 5 additional columns per side appear.
  4. Given the visible window changes, then WebSocket subscriptions are updated to the visible contracts only (subscription count ≤ window × 2 + underlying).
- **Data/Kite dependency:** Instruments dump; WebSocket (quote/full mode; OI) within C-07 budget, fallback `GET /quote` (≤ 500 instruments, 1 req/s); previous-day OI for % change from day candles with `oi=1`.
- **Deviation from Streak:** B/S not hover-only (NFR-11).
- **Priority hint:** Core

#### FR-OPT-017 Greeks and implied volatility computation
- **Out of scope (deferred):** Options strategy builder — all options features are out of scope (user decision 2026-10-12).
- **Source features:** OPT-015
- **Requirement:** Because Kite provides no Greeks, the system shall compute IV (from option mid-or-LTP by a bracketed root finder,
  tolerance 1e-6, bounds 0.01%–500%) and Delta, Gamma, Theta (per calendar day), Vega (per 1 vol point) using the Black-76 model on the
  synthetic forward (derived from put-call parity at the ATM strike, falling back to spot × e^{rT}), time to expiry in years to 15:30 IST
  on expiry day (calendar days / 365), and a user-configurable risk-free rate (Settings; default 6.5% p.a.). When IV cannot be solved
  (price below intrinsic, no quote), Greeks show "—".
- **Acceptance criteria:**
  1. Given golden vectors (forward, strike, T, r, price → IV, Δ, Γ, Θ, Vega) generated by an independent reference implementation, then all outputs match within 1e-6 relative.
  2. Given an option price below intrinsic, then IV and Greeks display "—".
  3. Given put-call parity inputs, then the synthetic forward matches the reference within 0.01.
- **Data/Kite dependency:** Quotes for options and underlying.
- **Deviation from Streak:** Model and rate are documented and configurable (Streak's model unknown).
- **Priority hint:** Important

#### FR-OPT-018 Rule-based strike: ATM offset
- **Out of scope (deferred):** Options strategy builder — all options features are out of scope (user decision 2026-10-12).
- **Source features:** OPT-016
- **Requirement:** In Rule-based strike → ATM-based, the user sets Position (Buy/Sell), Option type (CE/PE), Strike rule (ATM−10 … ATM …
  ATM+10, default ATM; custom ±N up to ±50 via entry), Expiry (FR-OPT-020), and adds the rule as a dynamic leg. ATM = the listed strike
  nearest to the underlying spot LTP (tie → lower strike); ATM+N = N strikes above ATM, ATM−N = N strikes below, for both CE and PE.
  Each option label shows an ITM/OTM hint for the chosen type (e.g. "ATM+2 · OTM" for CE, "ATM+2 · ITM" for PE). The resolved "live
  contract" preview updates with spot. **Dynamic legs are re-resolved at each entry signal** (not fixed at deploy).
- **Acceptance criteria:**
  1. Given spot 22,520, strikes every 50, then ATM = 22,500, ATM+2 CE = 22,600 CE labelled OTM, ATM+2 PE = 22,600 PE labelled ITM.
  2. Given spot 22,525 exactly between 22,500 and 22,550, then ATM = 22,500.
  3. Given a deployment and two entry signals on different days with different spots, then each entry resolves its own ATM.
- **Data/Kite dependency:** Underlying LTP (`GET /quote/ltp` or ticker) + instruments dump.
- **Deviation from Streak:** ITM/OTM hints (§3.2 #21); re-resolution rule explicit (OPT-016 open question).
- **Priority hint:** Core

#### FR-OPT-019 Rule-based strike: premium-based
- **Out of scope (deferred):** Options strategy builder — all options features are out of scope (user decision 2026-10-12).
- **Source features:** OPT-017
- **Requirement:** Premium-based rule: operator **Closest to** (default) / **Above** / **Below** and a premium value (> 0; prefilled with
  the ATM premium). Resolution at entry time over strikes of the chosen expiry and type with a valid LTP: Closest to = minimal |LTP − X|
  (tie → strike nearer ATM); Above = minimal LTP among LTP > X; Below = maximal LTP among LTP < X. If no strike qualifies, the entry signal
  is raised with an error "No contract matches the premium rule" and no order is suggested for that leg.
- **Acceptance criteria:**
  1. Given CE LTPs {120, 80, 70, 40} and X = 74.7 Closest, then the 70 strike is chosen (|−4.7| < |5.3|).
  2. Given Above 74.7, then 80; Below 74.7, then 70.
  3. Given Above 500 with max LTP 300, then the "No contract matches" error is logged.
- **Data/Kite dependency:** Chain quotes at entry (`GET /quote`, batches ≤ 500).
- **Deviation from Streak:** none.
- **Priority hint:** Important

#### FR-OPT-020 Expiry selection
- **Out of scope (deferred):** Options strategy builder — all options features are out of scope (user decision 2026-10-12).
- **Source features:** OPT-015, OPT-016
- **Requirement:** Expiry dropdowns shall list all future expiries of the underlying from the instruments dump as absolute dates with a
  weekly/monthly marker (default nearest). For dynamic legs the dropdown additionally offers relative choices: "Current weekly",
  "Next weekly", "Current monthly", "Next monthly", resolved at each entry. A fixed-date leg whose expiry has passed blocks deployment
  (FR-OPT-002).
- **Acceptance criteria:**
  1. Given NIFTY BANK with only monthly expiries in the dump, then only monthly dates are listed and "Current weekly"/"Next weekly" are disabled with a tooltip.
  2. Given a dynamic leg with "Next weekly" and a simulated date, then resolution picks the second-nearest weekly expiry on that date.
- **Data/Kite dependency:** Instruments dump `expiry`.
- **Deviation from Streak:** Relative expiries added (Streak: fixed dates only, §3.3) so long-lived deployments do not stale.
- **Priority hint:** Important

#### FR-OPT-021 Option template library
- **Out of scope (deferred):** Options strategy builder — all options features are out of scope (user decision 2026-10-12).
- **Source features:** OPT-018
- **Requirement:** The Execution card shall show a template carousel with filter tabs All / Bullish / Bearish / Neutral / Volatile; each
  card has our own payoff-shape icon, name and an info tooltip (our own one-line description). Activating a template **asks for
  confirmation when legs exist** ("Replace current legs?") and then replaces the legs with the template's legs resolved around ATM on the
  nearest expiry, 1 lot per ratio unit, as fixed legs (option "Keep as rule-based legs" creates ATM-offset legs instead). Wing width
  W (strike steps) defaults to 4 and is adjustable 1–20 in the confirm dialog. Templates (offsets in units of W strike steps, "ATM" = 0;
  ratio in brackets):
  | Bias | Template | Legs |
  |---|---|---|
  | Bullish | Long call | B CE 0 |
  | Bullish | Short put | S PE 0 |
  | Bullish | Bull call spread | B CE 0; S CE +1 |
  | Bullish | Bull put spread | S PE 0; B PE −1 |
  | Bullish | Call ratio back-spread | S CE 0 [1]; B CE +1 [2] |
  | Bullish | Bull condor | B CE 0; S CE +1; S CE +2; B CE +3 |
  | Bullish | Bull butterfly | B CE 0; S CE +1 [2]; B CE +2 |
  | Bullish | Range forward | S PE −1; B CE +1 |
  | Bullish | Long synthetic future | B CE 0; S PE 0 |
  | Bearish | Long put | B PE 0 |
  | Bearish | Short call | S CE 0 |
  | Bearish | Bear call spread | S CE 0; B CE +1 |
  | Bearish | Bear put spread | B PE 0; S PE −1 |
  | Bearish | Put ratio back-spread | S PE 0 [1]; B PE −1 [2] |
  | Bearish | Bear condor | B PE 0; S PE −1; S PE −2; B PE −3 |
  | Bearish | Bear butterfly | B PE 0; S PE −1 [2]; B PE −2 |
  | Bearish | Risk reversal (bearish) | B PE −1; S CE +1 |
  | Bearish | Short synthetic future | S CE 0; B PE 0 |
  | Neutral | Short straddle | S CE 0; S PE 0 |
  | Neutral | Short iron butterfly | S CE 0; S PE 0; B CE +1; B PE −1 |
  | Neutral | Short strangle | S CE +1; S PE −1 |
  | Neutral | Short iron condor | S PE −1; B PE −2; S CE +1; B CE +2 |
  | Neutral | Double butterfly ("batman") | B CE +1; S CE +2 [2]; B CE +3; B PE −1; S PE −2 [2]; B PE −3 |
  | Neutral | Double condor ("double plateau") | B CE +1; S CE +2; S CE +3; B CE +4; B PE −1; S PE −2; S PE −3; B PE −4 |
  | Neutral | Jade lizard | S PE −1; S CE +1; B CE +2 |
  | Neutral | Reverse jade lizard | S CE +1; S PE −1; B PE −2 |
  | Volatile | Call ratio spread | B CE 0 [1]; S CE +1 [2] |
  | Volatile | Put ratio spread | B PE 0 [1]; S PE −1 [2] |
  | Volatile | Long straddle | B CE 0; B PE 0 |
  | Volatile | Long iron butterfly | B CE 0; B PE 0; S CE +1; S PE −1 |
  | Volatile | Long strangle | B CE +1; B PE −1 |
  | Volatile | Long iron condor | B PE −1; S PE −2; B CE +1; S CE +2 |
  | Volatile | Strap | B CE 0 [2]; B PE 0 [1] |
  | Volatile | Strip | B CE 0 [1]; B PE 0 [2] |
  Leg order = table order, except that the "Hedge first" ordering (FR-OPT-033) is applied automatically when enabled. Templates are
  data (library file), not code.
- **Acceptance criteria:**
  1. Given NIFTY spot 22,520, step 50, W = 4, when Short iron condor is applied, then legs are S 22,300 PE, B 22,100 PE, S 22,700 CE, B 22,900 CE, 1 lot each.
  2. Given existing legs, when a template is activated, then a confirm appears; Cancel leaves legs unchanged.
  3. Given the library, then it contains 34 templates covering every structure of Appendix G, and each validates against the schema.
  4. Given the Volatile tab, then exactly the 8 Volatile templates are shown.
- **Data/Kite dependency:** Local template library; instruments dump; spot quote for ATM.
- **Deviation from Streak:** Confirm before replace (§3.2 #23); Volatile tab added (Streak shows those only under All); wing width adjustable; own names/descriptions.
- **Priority hint:** Important

#### FR-OPT-022 Combined SL / TP (strategy-level ₹)
- **Out of scope (deferred):** Options strategy builder — all options features are out of scope (user decision 2026-10-12).
- **Source features:** OPT-019
- **Requirement:** The Exit card shall have Combined stop loss (₹, > 0, entered as a positive loss amount) and Combined target (₹, > 0).
  Combined MTM = Σ legs (±(LTP − entry premium) × units). Trigger SL when MTM ≤ −SL, TP when MTM ≥ TP; on trigger all open legs are
  exited (signal/alert in live per DEP-006; actual fills may differ). The payoff graph shows optional dashed horizontal lines at −SL and
  +TP (toggle).
- **Acceptance criteria:**
  1. Given SL = 5,000 and MTM path −4,999 → −5,000, then trigger at −5,000.
  2. Given SL entered as −5,000, then inline error "Enter the loss as a positive amount".
  3. Given lines toggled on, then the graph shows lines at y = −5,000 and y = TP.
- **Data/Kite dependency:** Live LTP via ticker during deployment.
- **Deviation from Streak:** Optional SL/TP lines on payoff (Streak draws none); input semantics defined (OPT-019 open question).
- **Priority hint:** Core

#### FR-OPT-023 Exit conditions on an underlying
- **Out of scope (deferred):** Options strategy builder — all options features are out of scope (user decision 2026-10-12).
- **Source features:** OPT-020
- **Requirement:** "Add exit conditions (optional)" adds one underlying block (underlying picker defaulting to the entry underlying,
  timeframe, full condition editor). When it evaluates true while legs are open, all legs are exited (trigger EX). "Remove exit
  conditions" removes it (confirm if non-empty).
- **Acceptance criteria:**
  1. Given an exit block `RSI(14)[0] crosses above 70` on NIFTY 5m, when it fires during an open position, then an EX exit signal for all legs is generated.
  2. Given the block is added, then its underlying defaults to the entry underlying.
- **Data/Kite dependency:** Underlying candles.
- **Deviation from Streak:** Default underlying prefilled (Streak's starts empty).
- **Priority hint:** Core

#### FR-OPT-024 Time-based exit
- **Out of scope (deferred):** Options strategy builder — all options features are out of scope (user decision 2026-10-12).
- **Source features:** OPT-021
- **Requirement:** "Add time-based exit (optional)" adds a time + weekday widget (same control as FR-OPT-008) with default **03:15 PM**,
  Mon–Fri. The exit time must be within 09:15–15:29 and later than a time-based entry time on the same weekday (error otherwise). At the
  exit time on a selected trading day all open legs are exited (trigger SO-style time exit).
- **Acceptance criteria:**
  1. Given entry 09:20 and exit 09:15, then error "Exit time must be after entry time".
  2. Given a new time-based exit, then the default shows 03:15 PM.
  3. Given simulated clock 15:15 on a trading day with open legs, then an exit signal is generated for all legs.
- **Data/Kite dependency:** Local scheduler.
- **Deviation from Streak:** Sensible default and validation (§3.2 #22).
- **Priority hint:** Core

#### FR-OPT-025 Payoff engine and graph
- **Out of scope (deferred):** Options strategy builder — all options features are out of scope (user decision 2026-10-12).
- **Source features:** OPT-022
- **Requirement:** The system shall compute, for included legs: **expiry payoff** P(S) = Σ ±units × (intrinsic(S) − entry premium) where
  entry premium = current LTP for not-yet-entered legs; and **target-date payoff** using Black-76 with each leg's IV (FR-OPT-017) at the
  target date/time (FR-OPT-027). The graph plots P&L (₹) against underlying price over spot ± 10% (zoom/pan allowed), with the expiry
  curve shaded green above zero and red below, the target-date curve in a distinct colour, a vertical marker at current spot ("Spot
  22,520.45") and at target price, a "Projected P&L at target: ₹x (y%)" readout, and a hover/keyboard crosshair tooltip showing price,
  expiry P&L and target-date P&L. Recomputes within 100 ms of any leg change. Legs on different expiries: the expiry curve is evaluated
  at the nearest expiry with later-expiry legs valued by Black-76.
- **Acceptance criteria:**
  1. Given a long 22,500 CE at 137.05 × 65, then P(22,000) = −8,908.25 and P(23,000) = (500 − 137.05) × 65 = 23,591.75.
  2. Given a leg change, then the graph updates in < 100 ms (UI perf test).
  3. Given target date = today, then the target-date curve at spot equals current MTM (0 for un-entered legs) within ₹1.
  4. Given keyboard focus on the graph, when ←/→ are pressed, then the crosshair moves by one strike step and the tooltip updates.
- **Data/Kite dependency:** Quotes; local math.
- **Deviation from Streak:** Target-date curve always computed (Streak shows only the expiry curve on same-week expiries).
- **Priority hint:** Core

#### FR-OPT-026 Payoff table
- **Out of scope (deferred):** Options strategy builder — all options features are out of scope (user decision 2026-10-12).
- **Source features:** OPT-023
- **Requirement:** The Payoff table tab shall list underlying prices (rows every strike step by default, user-selectable step) over the
  graph's range with columns Price · P&L at target date · P&L at expiry, coloured by sign, with the spot row highlighted, and allow CSV
  export.
- **Acceptance criteria:**
  1. Given NIFTY step 50, then consecutive rows differ by 50.
  2. Given export, then the CSV has the same rows and values as the table.
- **Data/Kite dependency:** Local.
- **Deviation from Streak:** Step selectable; CSV export added.
- **Priority hint:** Important

#### FR-OPT-027 Target price and target date
- **Out of scope (deferred):** Options strategy builder — all options features are out of scope (user decision 2026-10-12).
- **Source features:** OPT-024
- **Requirement:** Under the graph: a target-price slider with −/+ stepper (step = strike step), numeric entry, % change from spot, and
  Reset (to spot). A target date-time picker (now … nearest expiry 15:30, default now) and a read-only "Expiry: <weekday, DD-MM-YYYY 3:30 PM>"
  of the nearest leg expiry. Changing either updates the projected P&L readout and the target-date curve.
- **Acceptance criteria:**
  1. Given spot 22,520 and +2 steps, then target = 22,620 and "+0.44%" shown.
  2. Given Reset, then target = spot.
  3. Given target date = expiry 15:30, then the target-date curve equals the expiry curve within ₹1.
- **Data/Kite dependency:** Local.
- **Deviation from Streak:** Target date selectable (Streak: read-only expiry only).
- **Priority hint:** Important

#### FR-OPT-028 Payoff statistics
- **Out of scope (deferred):** Options strategy builder — all options features are out of scope (user decision 2026-10-12).
- **Source features:** OPT-025
- **Requirement:** Show Max profit, Max loss (₹ and % of margin needed; "—" for % when margin unknown), and Break-even point(s). Computed
  analytically on the expiry payoff: evaluate P at S = 0, at every strike, and the slopes beyond the outermost strikes; a positive
  (negative) slope as S → ∞ means unlimited profit (loss); break-evens are the zero crossings found by linear interpolation between
  nodes (rounded to 0.05), listed ascending. No legs → "—".
- **Acceptance criteria:**
  1. Given a long call 22,500 at 137.05, then Max profit = "Unlimited", Max loss = −₹8,908.25, break-even = 22,637.05.
  2. Given a short iron condor with credit c per unit and wing 200, then Max profit = c × units, Max loss = −(200 − c) × units, and two break-evens.
  3. Given a short straddle, then Max loss = "Unlimited".
- **Data/Kite dependency:** Local; margin from FR-OPT-029.
- **Deviation from Streak:** % basis defined (margin needed).
- **Priority hint:** Core

#### FR-OPT-029 Funds and margins
- **Out of scope (deferred):** Options strategy builder — all options features are out of scope (user decision 2026-10-12).
- **Source features:** OPT-026
- **Requirement:** Show **Margin needed** for all legs as a basket (with spread/hedge benefit) and **Margin available** (account equity
  net available). Recomputed (debounced 1 s) on leg changes when online; offline or when the API fails, show "Unavailable" with the
  reason. Margin benefit vs sum of individual legs is shown when > 0.
- **Acceptance criteria:**
  1. Given online and a short iron condor, then Margin needed equals the basket API's `final` total for the four legs (mocked API test).
  2. Given 5 leg edits within 1 s, then exactly one margin request is sent.
  3. Given offline, then both values show "Unavailable (offline)".
- **Data/Kite dependency:** `POST /margins/basket` (consider_positions=false, compact mode), `GET /user/margins`; available on Personal plan (C-01).
- **Deviation from Streak:** none.
- **Priority hint:** Important

#### FR-OPT-030 Underlying chart drawer
- **Out of scope (deferred):** Options strategy builder — all options features are out of scope (user decision 2026-10-12).
- **Source features:** OPT-027
- **Requirement:** A collapsible drawer "Underlying chart" (toggle on the column divider; keyboard shortcut) shows the shared chart
  component (CHART-001) for the entry underlying at the entry timeframe, with its standard controls.
- **Acceptance criteria:**
  1. Given the drawer is opened, then the chart shows the entry underlying at the entry timeframe.
  2. Given the underlying changes, then the drawer chart switches symbol.
- **Data/Kite dependency:** Historical candles; ticker.
- **Deviation from Streak:** Chart library per C-12.
- **Priority hint:** Nice-to-have

#### FR-OPT-031 Option charts drawer (tab per leg)
- **Out of scope (deferred):** Options strategy builder — all options features are out of scope (user decision 2026-10-12).
- **Source features:** OPT-028
- **Requirement:** A collapsible drawer "Option charts" shows the chart component with one tab per leg contract (dynamic legs use their
  currently resolved contract). Where historical data is unavailable (e.g. expired contracts), the tab shows "No historical data for this
  contract".
- **Acceptance criteria:**
  1. Given 4 legs, then 4 tabs labelled with the contracts exist.
  2. Given a contract with no history (mocked empty response), then the message is shown instead of an empty chart.
- **Data/Kite dependency:** `GET /instruments/historical` for option tokens (expired-contract intraday history unverified, Appendix L).
- **Deviation from Streak:** Explicit no-data state.
- **Priority hint:** Nice-to-have

#### FR-OPT-032 Footer: Save and Deploy; options save dialog
- **Out of scope (deferred):** Options strategy builder — all options features are out of scope (user decision 2026-10-12).
- **Source features:** OPT-029
- **Requirement:** Footer actions **Save** and **Deploy** (opens the deployment dialog, DEP-001). Save for a new options strategy
  opens a Save dialog with Name, Category **Option buying** / **Option selling** (pre-selected by the classification rule of FR-OPT-035;
  user may override), Tags (≤ 5), Description. Deploy requires a saved, valid strategy (FR-OPT-002 validation). Unsaved-change guard
  applies (SHELL-007).
- **Acceptance criteria:**
  1. Given a new short straddle, when Save is pressed, then the dialog pre-selects "Option selling".
  2. Given an invalid strategy (no legs), then Deploy is disabled with the reason list.
- **Data/Kite dependency:** Local; DEP module.
- **Deviation from Streak:** Category auto-suggestion explicit.
- **Priority hint:** Core

#### FR-OPT-033 Leg execution order and hedge-first
- **Out of scope (deferred):** Options strategy builder — all options features are out of scope (user decision 2026-10-12).
- **Source features:** OPT-030
- **Requirement:** Legs are executed (signalled/placed) in list order. Users reorder legs by drag handle and by keyboard (Alt+↑/↓). A
  "Hedge first" action (and an optional per-strategy setting applied automatically) reorders legs so all Buy legs precede Sell legs,
  preserving relative order within each group. The card shows the estimated margin benefit of the current order (from FR-OPT-029).
- **Acceptance criteria:**
  1. Given legs [S CE, B CE, S PE, B PE], when Hedge first is used, then order = [B CE, B PE, S CE, S PE].
  2. Given a leg focused, when Alt+↑ is pressed, then it moves up one position.
  3. Given a deploy signal, then the signal lists legs in the saved order.
- **Data/Kite dependency:** Sequential order placement via throttled order queue (C-05; ORD module); `POST /margins/basket`.
- **Deviation from Streak:** Keyboard reorder and automatic hedge-first option.
- **Priority hint:** Important

#### FR-OPT-034 Options multi-timeframe evaluation
- **Out of scope (deferred):** Options strategy builder — all options features are out of scope (user decision 2026-10-12).
- **Source features:** OPT-031
- **Requirement:** Underlying-based entry/exit evaluation shall occur at instants aligned to LCM(timeframes of all underlying blocks
  and their condition timeframes ≤ 1h), measured from 09:15, after the higher-timeframe candle has closed; each block uses its own last
  closed candles per FR-EQB-025. The builder shows the effective interval.
- **Acceptance criteria:**
  1. Given blocks on 3m and 5m, then evaluation instants are 09:30, 09:45, …, and none at 09:18.
  2. Given both blocks on 5m, then instants are every 5 minutes from 09:20.
- **Data/Kite dependency:** Local scheduler; candles.
- **Deviation from Streak:** none.
- **Priority hint:** Core

#### FR-OPT-035 Options strategy classification and surfacing
- **Out of scope (deferred):** Options strategy builder — all options features are out of scope (user decision 2026-10-12).
- **Source features:** OPT-032
- **Requirement:** Saved options strategies appear in My Strategies under **Option buying** or **Option selling** per their category.
  Default classification rule: compute net premium at save time (FR-OPT-013; dynamic legs resolved at that time; if quotes unavailable,
  count legs): net debit → Option buying; net credit → Option selling; zero/unknown → majority of leg sides (tie → Option selling). The
  Discover strategy library may include options strategies (FR-DISC-002) shown under Option tabs; options strategies have no backtest
  (detail pane shows "Backtests are available for equity and futures strategies only").
- **Acceptance criteria:**
  1. Given a bull call spread (net debit), then category = Option buying.
  2. Given a short strangle, then Option selling.
  3. Given an options strategy in My Strategies, then its detail pane shows no Run backtest action and the explanatory text.
- **Data/Kite dependency:** Quotes at save time; local store.
- **Deviation from Streak:** Streak's Discover has no options strategies; ours may (library content decision). Classification rule documented.
- **Priority hint:** Important

#### FR-OPT-036 Option contract master data
- **Out of scope (deferred):** Options strategy builder — all options features are out of scope (user decision 2026-10-12).
- **Source features:** OPT-003, OPT-010, OPT-015
- **Requirement:** The system shall derive all option metadata (underlying → expiries, strikes, strike step, lot size, tradingsymbol,
  instrument token, weekly/monthly marker = whether the expiry is the last expiry of its month) from the daily instruments dump, cached
  locally with its download date, refreshed automatically at first launch after 08:30 IST each trading day and manually on demand. No
  strike step, lot size or expiry is hard-coded.
- **Acceptance criteria:**
  1. Given a fixture dump where NIFTY lot size changes from 65 to 75, when it is loaded, then new legs use 75 and existing saved legs keep `lots` (units recomputed).
  2. Given the dump is older than today's 08:30 and the app is online, when the app starts, then a refresh is triggered and the "data as of" date updates.
  3. Given expiries {13 Oct, 19 Oct, 27 Oct}, then 27 Oct is marked monthly.
- **Data/Kite dependency:** `GET /instruments/NFO`, `GET /instruments/BFO` (gzipped CSV), local cache.
- **Deviation from Streak:** none.
- **Priority hint:** Core

#### FR-OPT-037 Options deployment
- **Out of scope (deferred):** options out of scope; drafted by REVIEW-1 §3 (RV-002) to close the v1 gap and kept for a future options phase.
- **Source features:** OPT-029, DEP-001, OPT-030
- **Requirement:** Deploying an options strategy creates one deployment with one DI per leg set (not per instrument). Dialog
  fields: legs (read-only summary with lots editable per leg), Capital, Holding type NRML (MIS optional for intraday), mode
  (FR-DEP-003), validity, acknowledgement; no instrument list, no Strategy Cycle (one entry per entry signal; Time-based entries
  once per selected day). On an entry signal, dynamic legs are resolved (FR-OPT-018/019/020), the leg set is shown in one order
  review (mode 1) listing legs in execution order with per-leg margin and basket margin; on confirm, legs are submitted
  sequentially in order through the throttle, each tagged with the deployment tag + leg index. State: `ENTRY_ORDER_OPEN` until all
  legs are COMPLETE; if any leg is rejected or partially filled after the others filled, the DI goes to ATTENTION with actions
  "Exit filled legs" (default, pre-filled exit orders, short legs first) / "Keep". Exits (combined SL/TP, per-leg SL/TP/TSL, EX,
  time exit with a new trigger code `TE`, expiry EXP on the leg's expiry day at `session end − N minutes`) exit all open legs,
  shorts before longs. FR-DEP-011 shows the leg set as a parent row with legs as children; TP/SL columns show the combined ₹ levels.
- **Acceptance criteria:** (1) Given a 4-leg iron condor in mode 1, when confirmed, then 4 orders are placed in saved order within
  the throttle and the DI is IN_POSITION only after the 4th COMPLETE. (2) Given leg 3 rejected after legs 1–2 filled, then the DI is
  ATTENTION and "Exit filled legs" pre-fills two opposite orders with the short leg first. (3) Given expiry day 15:15 with open
  legs, then an EXP exit signal for all legs is raised.
- **Data/Kite dependency:** `PlaceOrder` (sequential, tagged), `GetBasketMargins`, ticker.
- **Deviation from Streak:** Streak's options deploy dialog was not observed (OPT-029).
- **Priority hint:** Core (if revived)

#### FR-OPT-038 Options pricing engine
- **Out of scope (deferred):** options out of scope; drafted by REVIEW-1 §3 (RV-007) as the single pricing definition for FR-ANLY-012, FR-OPT-017, FR-OPT-025 and FR-SCALP-004/007.
- **Source features:** OPT-015, OPT-022, ANLY-007, SCALP-004, SCALP-007
- **Requirement:** One engine for IV and Greeks used by every options UI: model Black-76 on a forward derived from put-call parity at
  ATM (fallback spot × e^{(r−q)T}); day count calendar days / 365 to 15:30 IST of expiry; inputs r (default 6.5 % p.a.) and q
  (default 0) from Settings; bracketed solver, tolerance 1e-6, bounds 0.01 %–500 %; units Theta per calendar day, Vega per 1 vol
  point, IV in %; golden vectors (NFR-06); "—" when unsolvable (price below intrinsic, no quote). Payoff table default step 10
  points (parity, RV-034); target-price stepper step 10 (parity).
- **Acceptance criteria:** (1) Given the golden set, then IV/Δ/Γ/Θ/Vega match within 1e-6 relative in every UI (shared code path).
  (2) Given the same contract in the Scalper chain and the options-builder chain, then the displayed Greeks are identical.
- **Data/Kite dependency:** quotes for options and underlying.
- **Deviation from Streak:** Model and parameters documented (Streak's undisclosed).
- **Priority hint:** Important (if revived)
