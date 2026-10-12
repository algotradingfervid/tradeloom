# REVIEW-2: Live sweep of the Streak web app against FEATURES.md

- **Date:** 2026-10-11, Sunday, market closed. Data shown is from the last session.
- **Method:** Claude in Chrome, in one dedicated tab (now closed), against the user's logged-in session. The sweep was read-only: no Save, Copy, Run Backtest, Execute, Deploy, Get Alerts, Buy/Sell, Delete, Logout, settings toggle or terms checkbox was clicked.
- **Approved research items:** `zz-research-copy` and `zz-research-scanner` were only viewed. Dialogs were opened and then closed with Cancel, Back or a backdrop click.
- **Baseline:** `docs/research/FEATURES.md` (222 features).
- **Personal data:** none is recorded here.

## (a) Routes visited and hrefs discovered

**Routes visited:** `/` (stays on `/`, no redirect), `/dashboard`, `/markets`, `/discover-strategies`, `/my-strategies`,
`/strategy?defaultType=equity`, `/strategy?defaultType=options&isNew=true`, `/strategy/equity-zz-research-copy`,
`/backtest/equity-zz-research-copy`, `/live-trades`, `/discover-scanners`, `/my-scanners`, `/scanner`,
`/scanner/zz-research-scanner`, `/live-alerts`, `/notifications`, `/profile`, `/holdings`, `/positions`, `/order-book`,
`/chartiq?seg_sym=NSE_SBIN&timeframe=day`, and these newly found routes: **`/terms`**, **`/privacy`**, **`/scanners`**.

**Every `a[href]` collected across the pages:**
- In-app: `/`, `/order-book`, `/positions`, `/notifications`, `/markets`, `/discover-strategies`, `/strategy?defaultType=…`, `/strategy?defaultType=…&isNew=…`, `/live-trades`, `/discover-scanners`, `/scanner`, `/live-alerts`, `/my-strategies`, `/my-scanners`, `/holdings`.
- External: `https://help.streak.tech/`.
- From the deploy dialog: `/terms` (`target=_blank`).

**Route probe (same-origin `fetch`, HTTP status):**
- **200:** `/privacy`, `/chartiq`, `/scanners`, `/login`.
- **404:** `/privacy-policy`, `/disclaimer`, `/risk-disclosure`, `/terms-and-conditions`, `/settings`, `/pricing`, `/watchlist`, `/backtest`, `/strategies`, `/alerts`, `/faq`, `/support`, `/about`, `/logout`, `/onboarding`, `/referral`, `/basket`, `/options`, `/option-chain`, `/screener`, `/portfolio`, `/funds`, `/orders`, `/gtt`, `/deployments`, `/paper-trading`, `/tour`, `/help`, `/shortcuts`, `/changelog`, `/release-notes`, `/whats-new`, `/policy`, `/refund`, `/contact`, `/cookie-policy`.

`/login` was not opened in the browser. Opening it could disturb the session.

**Not Streak UI:** the `Open chat` / `Dismiss` buttons found during enumeration belong to the Claude extension's own overlay (`#claude-static-indicator-container`).

## (b) New items not in the catalogue

| # | Route | Element / behaviour | Description | Suggested module |
|---|---|---|---|---|
| N1 | `/dashboard`, `/markets` (Analytics → Technicals) | Hover tooltip on each **Event** row | Each row in the `Event \| Time` list is a `<button>`. Hovering shows a dark tooltip that explains the event in plain English, for example "Bullish RSI Divergence is a technical analysis concept where the price makes a lower low, but the RSI forms a higher low…". The CCI row's tooltip describes it as a scanner over Nifty 100 on the daily timeframe. Clicking a row does nothing. ANLY-002 says only "No tooltips on gauge/pivots; rows not clickable". | ANLY (extend ANLY-002) |
| N2 | `/strategy/<own-slug>` → Execute | **"Step 1: Choose your execution type"** wizard step | Execute opens a two-step modal. Step 1 offers two radio cards with icons: **Notification Based Trading** ("Get notified when your Strategy met your condition", selected by default) and **Backtest** ("Backtest your Strategy with Historical Data."). A right-hand pane plays an animated preview for the selected type. The footer has a progress bar, **Cancel** and **Next**. Step 2 is the parameters form (BT-001 or DEP-001). The catalogue never describes step 1. | BT / DEP (new feature, or extend EQB-030) |
| N3 | Execute → step 2 (both modes) | **Back** button and the **trading terms** link to `/terms` | Step 2's footer is **Back** + **Run Backtest** or **Execute**; there is no Cancel. In the NBT step, "I accept the **trading terms**" links to `/terms` in a new tab. The NBT step defaults Strategy Cycle to **1 Cycle**, while the Backtest step defaults to "-". | DEP / BT (correct BT-001 and DEP-001) |
| N4 | `/scanner/<own-slug>` (editable), comparator cell | **Up By dialog: `type` offers `%` and `abs`**, plus a **delete (trash) icon** in the dialog header | On an editable scanner the `type` dropdown is enabled and offers **% and abs**, so absolute-point moves exist. An orange trash icon sits top-right of the "Up By" dialog. EQB-011 lists only `%` (disabled) and Cancel/Save. | EQB (EQB-011) / SCAN |
| N5 | `/scanner/<own-slug>` → Set Alerts | Extra content in the **Get Scanner Alert!** dialog | The dialog also has an animated bell illustration and a warning line with an icon: "This Scanner Alert will get trigger every 1 Day." It has no × and no Cancel, and Escape does not close it; only a backdrop click does. SCAN-009 has the main text and the checkbox, but not these parts. | SCAN (SCAN-009) |
| N6 | `/terms` | **Terms & Conditions page** (in-app route, no shell or sidebar) | Long legal page: "TERMS & CONDITIONS", a boxed "IMPORTANT REGULATORY CLARIFICATION" (no auto-trading; decision-support only), numbered sections, and a glossary of product terms (Strategy, Live Strategy, Notification-based Trading, Equity Trader, Options Trader, Scalper Mode, Live Scanners, Discover…, Live Backtest, Payoff Graph…). It is reached from the deploy dialog's "trading terms" link and presumably from the scanner-alert dialog too. | ACCT (new: legal pages) |
| N7 | `/privacy` | **Privacy Policy page** (in-app route, no shell) | "PRIVACY POLICY" page with numbered sections (privacy commitment, information collected, broker integration data, device data…). No visible link to it was found in the app UI; it was found by probing. | ACCT (new: legal pages) |
| N8 | `/scanners` | **Alias route** that renders My Scanners | `/scanners` returns 200 and renders the My Scanners master/detail. localStorage holds `SCANNERS_ACTIVE_TAB=my`, which suggests one scanners route that remembers a my/discover tab. Not in the module map. | MYS / DISC |
| N9 | global (after login) | **Risk-disclosure acknowledgement, once per session** (not rendered during the sweep) | localStorage has a key `riskDisclosureSeen:<session-token>=1`. The app bundle defines the constant `RISK_DISCLOSURE_PREFIX:'riskDisclosureSeen'`, alongside `PRODUCT_TOUR`. So a risk-disclosure modal or interstitial is shown and acknowledged once per login session. It could not be displayed without a new login. **No screenshot.** | SHELL / ACCT (new) |
| N10 | `/login` | **Login route** | Returns 200. Not opened, to avoid disturbing the session. The catalogue has no login screen; ACCT-005 only says "login via Kite credentials". **No screenshot.** | ACCT |

**Screenshots** (in `docs/research/screenshots/06-review-sweep/`):

N1 – Technicals event tooltip:
![](../research/screenshots/06-review-sweep/01-technicals-event-tooltip.png)

N2 – Execute step 1, execution-type chooser:
![](../research/screenshots/06-review-sweep/02-execute-step1-execution-type.png)

N3 – NBT step 2 with Back, the trading-terms link and the 1 Cycle default:
![](../research/screenshots/06-review-sweep/03-execute-step2-nbt-back-terms-link.png)

N4 – Up By dialog with the `%`/`abs` type and the trash icon:
![](../research/screenshots/06-review-sweep/04-upby-dialog-type-abs-and-trash.png)

N5 – Get Scanner Alert! dialog with the trigger note:
![](../research/screenshots/06-review-sweep/05-scanner-alert-dialog-trigger-note.png)

N6 – `/terms`:
![](../research/screenshots/06-review-sweep/06-terms-page.jpg)

N7 – `/privacy`:
![](../research/screenshots/06-review-sweep/07-privacy-page.jpg)

N8 – `/scanners` alias:
![](../research/screenshots/06-review-sweep/08-scanners-alias-my-scanners.jpg)

**Code-only hints, not counted as UI items:**
- The obfuscated `_app` bundle has a deployment-mode map `{live, auto, paper}`.
- It also has a timeframe map that includes `2min` and `4hour`, beyond the 8 or 10 timeframes the UI shows.

Neither appears in the UI. They are worth a note only because they agree with DEP-011, where these modes once existed.

## (c) Claim verification

| # | Catalogue claim (ID) | Result | Evidence or correction |
|---|---|---|---|
| 1 | 9 comparators in Basic order: Equal to … Down by (EQB-010, App. C) | CONFIRMED | Equity builder, RSI row: the order matches exactly. |
| 2 | RSI picked into an empty row gives `RSI(14,0) crosses above 30` (App. A) | CONFIRMED | Observed. |
| 3 | Chart Settings timeframes are 8 options, default 1 Minute (EQB-005) | CONFIRMED | |
| 4 | A new equity draft is pre-seeded with SBIN, HDFCBANK, ADANIENT, APOLLOHOSP (EQB-001) | CONFIRMED | |
| 5 | Assist opens "Add Prebuild Suggestions" with the A–Z strip, search, Bullish toggle and "Indicators (71)" (EQB-013/014) | CONFIRMED | SMA has "Prebuild Suggestions (11)". |
| 6 | The saved own strategy's ⋮ menu has Delete, Copy; the footer has Save / Execute (EQB-032) | CONFIRMED | |
| 7 | My Strategies tabs are All, Equity Swing, Equity Intraday, Option Buying, Option Selling (MYS-002) | CONFIRMED | |
| 8 | My Strategies and My Scanners row ⋮ menus have Delete, Edit, Copy (MYS-003/006) | CONFIRMED | |
| 9 | Discover Strategies has 45 strategies and the tabs All / Equity Swing / Equity Intraday (DISC-002) | CONFIRMED | 45 rows after scrolling. |
| 10 | The filter popover has Trend: Bullish, Bearish; Apply is disabled until a box is ticked (DISC-004) | CONFIRMED | |
| 11 | The universe dropdown has 29 options in the listed order (App. H) | CONFIRMED | Read programmatically; exact match. |
| 12 | There are 37 prebuilt scanners (DISC-012) | CONFIRMED | 37 rows after scrolling. |
| 13 | Backtest dialog defaults: 25000; 13/07/2026–11/10/2026; 9:15 AM–3:30 PM; Intraday MIS; Strategy Cycle {–, 1–5 Cycles} (BT-001/002) | CONFIRMED | |
| 14 | The Backtest dialog buttons are **Cancel / Run Backtest** (BT-001) | **WRONG** | Step 2 shows **Back / Run Backtest**. Cancel is on step 1, the execution-type chooser (N2). |
| 15 | NBT dialog: Instruments (15/50), * Capital, Qty 1 per instrument, Strategy Cycle, Order Type Market, Holding Type, "I accept the trading terms", **Cancel / Execute** (DEP-001) | **WRONG (partly)** | The fields are right. The buttons are **Back / Execute**. Strategy Cycle defaults to **1 Cycle**. "trading terms" is a link to `/terms`. |
| 16 | Trade-log columns are Date & Time, Symbol, P&L, Price, Buy/Sell, Qty (signed), Trigger, with pagination (BT-016) | CONFIRMED | Here 7 pages. |
| 17 | "P&L on an exit row = realised P&L of that round trip; entry rows carry the running value" (BT-016) | **WRONG** | The P&L column is the **cumulative running P&L** on every row. Example (APOLLOHOSP, qty 2): SO rows read −111 → −111 → −167 → −80. Each step equals the previous total plus (exit − entry) × 2, for instance −111 + (8901 − 8929) × 2 = −167. |
| 18 | Trigger codes EN and SO are observed (BT-030) | CONFIRMED | SL/TP/TSL/EX/EXP were not seen on page 1. |
| 19 | Own backtest page actions are Edit, Run Backtest, NBT, with no Copy; Trading Charges toggle; 9 metric tiles including Max DD (BT-008/011) | CONFIRMED | |
| 20 | Options Auto Strike, ATM Based: 21 options from ATM 10 to ATM −10 (OPT-016) | CONFIRMED | 21 values read from the DOM. |
| 21 | Options new-draft defaults: NIFTY 50, 1 Minute, B NIFTY 13th OCT 22500 CE, qty 65, Premium 137.05, Total 8908.25, Max Loss −8908.25, Break Even 22637.05 (OPT-001) | CONFIRMED | |
| 22 | Time Based entry defaults to 9 : 15 AM with Mon–Fri all selected (OPT-007) | CONFIRMED | |
| 23 | Scalper option-chain drawer: Expiry dropdown, OI on, Greeks off, the listed columns, row hover actions (SCALP-004) | CONFIRMED | |
| 24 | The ƒx Indicators dialog has 107 studies (CHART-006) | CONFIRMED | 107 unique items in the iframe list. |
| 25 | Advance "Select Expression" has about 105 entries, i.e. 97 + 8 math functions (SCAN-004, App. B) | **WRONG (minor)** | The All tab has **107** entries: 99 operands including "Number", plus 8 math functions (Period min … Brackets). |
| 26 | The Advance per-row timeframe chip offers 1min…hour, day, week, month (10) (App. I) | CONFIRMED | |
| 27 | The Up By `type` dropdown shows only `%` (EQB-011) | **WRONG** | Editable scanners offer **`%` and `abs`**, and the dialog has a delete icon (N4). |
| 28 | The leave-site guard protects strategy builders with unsaved edits (SHELL-007) | **WRONG (partly)** | Only the native `beforeunload` is guarded. Leaving a dirty equity or options draft through the **sidebar or the "My Strategies" button** (client-side navigation) gave **no prompt**, and the draft was discarded. |
| 29 | The dashboard and Scalper chart default to **1 Hour** (CHART-002, App. I) | CANNOT VERIFY, likely changed | The dashboard and both Scalper charts showed **1 Day**. localStorage `chart_preferences={}`, so this is either a new default or a server-side preference. |
| 30 | Technicals event rows are not clickable and the panel has no tooltips (ANLY-002) | WRONG (partly) | The rows are not actionable, but each row has an explanatory hover tooltip (N1). |
| 31 | Profile menu: Profile, Holdings, Help, Dark Theme toggle, Version V1.0.0 (SHELL-009) | CONFIRMED | |
| 32 | Profile page: 4 disabled fields, 2 toggles, Logout, no Save (ACCT-001) | CONFIRMED | Checked structurally only; values were not recorded. |
| 33 | Order Book, Positions and Holdings headers, columns and empty states, including the "No positions found" copy bug on Holdings (ORD-001/002/003) | CONFIRMED | |
| 34 | Notifications page tabs are All / Notification Based / Scanner, with the listed columns (DEP-009) | CONFIRMED | |
| 35 | Live Strategies has a single NBT tab, the listed columns and its empty state (DEP-003) | CONFIRMED | |
| 36 | Watchlist row hover shows chart and trash icons; B/S are disabled for indices (WATCH-003) | CONFIRMED | |
| 37 | An option symbol shows only the Technicals tab; an index shows all 6 tabs (ANLY-001) | CONFIRMED | Dashboard with the option symbol; Scalper with NIFTY 50. |
| 38 | The Holdings card is not a link (HOME-002) | CONFIRMED | Quirk: the "Holdings" title has `cursor:pointer` and a click handler, but clicking does nothing. |
| 39 | Expanded scanner results show a Show Conditions link, universe and timeframe dropdowns, Run Scan and collapse (SCAN-007) | CONFIRMED | |
| 40 | There is no in-app keyboard-shortcut list (§3.3) | CONFIRMED | "?" opens nothing. Alt+T does nothing visible when no toasts are shown. |

**Totals:** 40 claims checked: 32 confirmed, **7 wrong or partly wrong** (#14, 15, 17, 25, 27, 28, 30), 1 cannot verify (#29).

## (d) Not checked

- **Risk-disclosure modal (N9) and `/login` (N10):** these need a fresh login session.
- **Narrow-window or responsive behaviour:** the resize tool was not loaded, so this was not tested.
- **Order window (B/S), Place Basket Order, Trading Charges toggle, Save dialogs, Create Watchlist, Copy flows:** not opened, because each is a forbidden or state-changing action.
- **Live behaviour:** the market was closed (Sunday), so live statuses, alert delivery and LTP streaming could not be observed.
- **Trade-log codes SL / TP / TSL / EX / EXP and download format:** not present on the pages viewed, and the download was not run.
- **Which 2 extra Advance operands make 107 versus the catalogue's 105:** the list was captured, but not reconciled name by name.
- **Right-click menus:** inside the TradingView iframe they were not re-tested (already covered by CHART-011). Watchlist rows have no custom context menu.
- **Hover tooltips on every icon button:** only a sample was checked (chart header, filter icon, Strike Selection label).
