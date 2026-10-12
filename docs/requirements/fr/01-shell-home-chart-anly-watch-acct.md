# Functional Requirements — Part 01: SHELL, HOME, CHART, ANLY, WATCH, ACCT

Status: DRAFT · Date: 2026-10-11 · Input: `docs/research/FEATURES.md` (modules SHELL, HOME, CHART, ANLY, WATCH, ACCT;
appendices H, I, J, K, L, N; §3 cross-cutting; §6 conflicts) · Governing document: `docs/requirements/REQUIREMENTS.md`
§1–5 (constraints C-01..C-12, NFR-01..14).

Conventions
- "The system" = the Streak Desktop application (Go + Wails v2 backend, React frontend, gokiteconnect v4).
- All times are IST (Asia/Kolkata). "Trading day" follows the local exchange calendar (see FR-SHELL-005).
- "Order window" = the order ticket specified in the ORD module (ORD-004). No FR in this file sends an order directly;
  every trade entry point opens the order window, which enforces NFR-05.
- "Instrument master" = the locally cached Kite instruments dump (FR-WATCH-008).
- UI copy in quotes is **our** proposed wording, not Streak's.
- Charting (C-12): each chart FR carries a **Library note**: `[Full-TV]` = provided natively by the licensed TradingView
  Charting Library; `[Open+Own]` = with an open library (e.g. lightweight-charts, Apache-2.0) the capability must be built
  by us (plugins/primitives, own dialogs, own indicator engine). `[Either]` = available in both with little work.
- Priority hint is a hint only; phasing is decided later.

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
     shows 9 navigation entries (FR-SHELL-002).
  2. Given the window is resized to 1280×720, then no horizontal scrollbar appears on the frame and all top-bar controls
     remain visible.
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
  keyboard-focusable target: Home (dashboard), Scalper, Strategies (discover/library), Equity Builder (new equity strategy
  draft), Options Builder (new options strategy draft), Live Strategies, Scanners (discover/library), New Scanner,
  Scanner Alerts. The entry for the current view shall be highlighted. Equity Builder and Options Builder shall open the
  shared strategy builder view with the type preset to equity or options respectively; Options Builder always opens a
  new draft. The sidebar shall be collapsible to icons only (tooltips show labels) and the collapsed state persisted.
- **Acceptance criteria:**
  1. Given the app is open, then the sidebar lists the 9 entries above in that order.
  2. Given the user clicks Options Builder twice (saving nothing), then each click opens a fresh options draft (subject
     to the unsaved-change guard, FR-SHELL-008).
  3. Given focus is on the sidebar, when the user presses Up/Down and Enter, then focus moves between entries and Enter
     navigates.
  4. Given the user collapses the sidebar and restarts the app, then it is still collapsed.
- **Data/Kite dependency:** local only.
- **Deviation from Streak:** Labels use our own wording ("Equity Builder"/"Options Builder"/"New Scanner" instead of the
  Streak labels). Collapsible sidebar added for small windows (NFR-11).
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
  06:00 tomorrow"), market phase computed from the local exchange calendar (`Pre-open` 09:00–09:15, `Open` 09:15–15:30,
  `Closed` otherwise, `Holiday`), and time of last tick. When the market is closed or a holiday, a small "Market closed"
  label shall appear next to the dot. Clicking the indicator opens a status panel with a "Reconnect now" button and a
  "Log in" button when the session is missing/expired. Colour transitions take ≤300 ms. The exchange holiday calendar is
  a local, user-editable list shipped with the current year's holidays.
- **Acceptance criteria:**
  1. Given the ticker is connected, then the indicator shows a green dot with label "Live".
  2. Given the network drops, when the ticker reports a disconnect, then within 1 s the indicator shows amber
     "Reconnecting"; when reconnect succeeds it returns to "Live" and all prior subscriptions are restored (NFR-09).
  3. Given the access token passed 06:00 IST, then the indicator shows "Logged out" and the status panel offers "Log in".
  4. Given Sunday or a listed holiday, then the label "Market closed" is shown regardless of feed state.
  5. Given any state, then hovering or focusing the indicator shows a tooltip containing feed state, session expiry and
     market phase (no state is unlabelled).
- **Data/Kite dependency:** gokiteconnect `ticker` callbacks (`OnConnect`, `OnReconnect`, `OnNoReconnect`, `OnClose`,
  `OnError`); session store (C-02); local holiday calendar.
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
- **Source features:** SHELL-006
- **Requirement:** The system shall provide first-visit guided tours composed of popovers anchored to UI regions, each
  with a title, body text (our own wording), a counter "N of M", buttons Back (hidden on step 1), Next (on non-final
  steps), Done (final step) and a close ×. Tours: Discover Scanners (3 steps: prebuilt list, scan results, enabling
  alerts), prebuilt scanner view (2 steps: universe & conditions, results), Options Builder (5 steps: entry trigger
  underlying/time, execution/strike selection, exit, payoff graph, underlying chart), Scalper (4 steps: option chain,
  watchlist, third chart, positions & order book), full backtest results (2 steps: summary metrics, trade log). A tour
  starts automatically the first time its view is opened, is marked seen on Done, × or Escape, and never auto-starts
  again. Each view with a tour exposes "Replay tour" in its help (?) menu; Settings provides "Reset all tours".
  If a step's anchor element is not present, that step is skipped.
- **Acceptance criteria:**
  1. Given a fresh profile, when the user opens the Scalper view, then step "1 of 4" is shown anchored to the option chain.
  2. Given step 2 of 4, when the user presses Back, then step 1 is shown; Next on step 4 is replaced by Done.
  3. Given the user presses Escape on step 2, then the tour closes and does not reappear on next visit or after restart.
  4. Given "Reset all tours" is used, then each tour auto-starts again on next visit to its view.
  5. Given "Replay tour" on the Options Builder, then the 5-step tour starts at step 1.
- **Data/Kite dependency:** local only (per-tour "seen" flags in SQLite).
- **Deviation from Streak:** Replay/reset added (Streak open question). Tour copy is our own. Escape closes.
- **Priority hint:** Nice-to-have

### FR-SHELL-008 Unsaved-change guard
- **Source features:** SHELL-007
- **Requirement:** When the user has unsaved edits in the Equity Builder, Options Builder **or** Scanner builder and
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
- **Deviation from Streak:** Applies to the scanner builder too (bug §3.2 #15). Replaces the browser beforeunload prompt;
  offers Save directly.
- **Priority hint:** Core

### FR-SHELL-009 Toast notifications
- **Source features:** SHELL-008
- **Requirement:** The system shall show transient toasts at top-centre with a coloured left border and icon by level:
  INFO (blue), SUCCESS (green), WARNING (amber), ERROR (red). Each toast has a message, an optional action link, and a
  close ×. Auto-dismiss: INFO/SUCCESS after 5 s, WARNING after 8 s, ERROR stays until dismissed; hovering or focusing a
  toast pauses its timer. At most 3 toasts are visible (newest on top; older ones queue). The toast region is an ARIA
  live region; Alt+T moves keyboard focus to the newest toast. Every error toast that concerns a specific input shall
  also be shown inline at that input. Toast messages shall be plain, specific and our own wording (e.g. "No instruments
  matched the conditions", "Found 5 instruments", "Strategy saved").
- **Acceptance criteria:**
  1. Given a SUCCESS toast, then it disappears after 5 s ±0.5 s unless hovered.
  2. Given an ERROR toast, when 60 s pass, then it is still visible until × is clicked.
  3. Given 4 toasts raised within 1 s, then 3 are visible and the 4th appears when one is dismissed.
  4. Given a scanner run with an incomplete row, then an ERROR toast appears **and** the incomplete row is outlined with
     an inline message.
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
- **Requirement:** Every action that Streak reveals only on hover (row chart/remove/B/S actions, option-chain B/S, study
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

---

## FR module HOME — Home dashboard

### FR-HOME-001 Dashboard layout
- **Source features:** HOME-001
- **Requirement:** The Home view shall be laid out as a left column (~2/3 width) and a right column (~1/3 width,
  independently scrollable). Left column, top to bottom: a row with the Holdings summary card (FR-HOME-002) and the Live
  Strategy Overview card (FR-HOME-003); the main chart (FR-CHART-002 and following); the analytics panel (FR-ANLY-001).
  Right column, top to bottom: Watchlist panel (FR-WATCH-001), Live Scanners widget (FR-HOME-004), Live Backtest widget
  (FR-HOME-006). On first run the main chart shows NIFTY 50 at the 1 Hour timeframe; thereafter the last charted symbol
  and timeframe are restored (FR-CHART-016). Each card renders its own loading, empty and error state without blocking
  the others.
- **Acceptance criteria:**
  1. Given a fresh install, when Home opens after login, then the chart shows NIFTY 50 at 1 Hour.
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
  (e.g. "1 Day", "5 Minutes"), and a universe dropdown listing the universes of FR-WATCH-009 (29 entries, Appendix H
  order). Below it a horizontally scrollable tab strip (« » buttons and mouse-wheel scroll) shows the featured prebuilt
  scanners in this order: Top Gainers, Top Losers, Opening Range Breakout, Opening Range Breakdown, Higher high for 2
  days, Lower Low for 2 days, Short Term Bullish, Short Term Bear, Momentum Surge, Momentum Fade, Bullish Engulfing,
  Bearish Engulfing, Volume Breakout, Volume Breakdown, RSI Entering Bullish Territory, Possible Downtrend, 52 Week High
  Breakout, Bullish Harami, Inverted Hammer, Hammer (the first 20 of the prebuilt scanner library, Appendix F). The
  selected universe is the shared scanner-universe preference (FR-WATCH-011). The selected tab is persisted. While the
  market is open the active scanner is re-evaluated on every close of its timeframe's candle (NFR-08: results within 2 s
  after close); when the market is closed, results are computed once from cached candles of the last session and the
  header shows "As of <timestamp>".
- **Acceptance criteria:**
  1. Given the widget, then the universe dropdown lists exactly 29 universes in Appendix H order.
  2. Given the user changes the universe to Nifty 100 in the widget, then the Discover Scanners universe is also Nifty 100.
  3. Given the Opening Range Breakout tab is active, then the timeframe label reads "5 Minutes".
  4. Given market open and a 5-minute candle closes at 10:05:00, then the ORB results reflect that candle by 10:05:02.
  5. Given Sunday, then results show the last session's hits and "As of 09 Oct 2026, 15:30".
- **Data/Kite dependency:** local scanner engine over local candle store (historical backfill + ticker aggregation);
  universe constituents (FR-WATCH-010); option universes need selective subscriptions within the 9,000-instrument cap
  (C-07).
- **Deviation from Streak:** Live evaluation runs only while the app is running (C-03); results are computed locally,
  so after a fresh install the first results appear only after backfill of the universe (progress shown, FR-HOME-005).
- **Priority hint:** Important

### FR-HOME-005 Live Scanners widget — result rows and navigation
- **Source features:** HOME-004
- **Requirement:** Each result row shall show: instrument (symbol + exchange tag; options with a C/P badge and display
  name per FR-WATCH-013), one column "LTP / Change" (LTP and coloured percent change), and B / S buttons (disabled for
  indices). Hovering or focusing a row reveals a chart action that loads the instrument into the dashboard main chart.
  About 6 rows are visible; the list scrolls. Below the list a "See full results" button opens the full scanner view of
  the active tab's scanner (SCAN module). B/S open the order window (ORD-004) prefilled with the instrument and side.
  While universe data is being backfilled, the list area shows "Preparing data… n of m instruments" with a progress bar;
  if no instrument matches, "No instruments match right now".
- **Acceptance criteria:**
  1. Given a row for NIFTY 13 OCT 22250 CE, then it shows the C badge and exchange tag NFO-OPT.
  2. Given the chart action of a row is activated, then the main chart switches to that instrument and the analytics tab
     set adapts (option → Technicals only).
  3. Given B is clicked on a stock row, then the order window opens prefilled (BUY, that symbol) and no order is sent.
  4. Given "See full results" on the Top Losers tab, then the Top Losers scanner view opens with the same universe.
  5. Given an index row, then B and S are disabled.
- **Data/Kite dependency:** ticker for LTP; instrument master; order window (ORD-004).
- **Deviation from Streak:** Chart action keyboard-reachable (FR-SHELL-015); explicit preparing/no-match states.
- **Priority hint:** Important

### FR-HOME-006 Live Backtest widget — tabs and metrics
- **Source features:** HOME-005
- **Requirement:** The widget shall show a header (dot + "Live Backtest") and a horizontally scrollable tab strip of
  featured prebuilt strategies (default set: the 10 featured entries of the prebuilt strategy library, Appendix E —
  Bollinger Band Mean Reversion, Falling Momentum, Volatility Contraction, Volatility Expansion, Strong Breakout
  Continuation, Vwap Mean Reversion, Volatility burst, TEMA Momentum Shift, RSI Trend Momentum Breakout, Volatility
  Expansion Trend Breakout); the user can change the featured set in Settings. For the active tab it shows: `Backtest
  P&L` (₹, green/red), `Max gain`, `Max loss` (largest single-trade gain/loss), `Win vs loss` bar (green fill = wins ÷
  trades) with labels `N W` / `N L`, `Trades` (count of round trips), `Capital used`, aggregated across all instruments of
  the strategy, over the window "last 90 calendar days ending the last completed trading day". Metric definitions follow
  Appendix M. A "See full results" button opens the full backtest results view (BT-008) for that run.
- **Acceptance criteria:**
  1. Given a run with trades P&L [+100, −40, +60], then Backtest P&L ₹120.00, Max gain ₹100.00, Max loss −₹40.00,
     2 W / 1 L, Trades 3, win bar 66.7 % filled.
  2. Given the active tab changes, then all metrics and the heatmap switch to that strategy within 300 ms (cached result).
  3. Given "See full results", then the backtest results view opens showing the same P&L.
- **Data/Kite dependency:** local backtest engine (BT) over the local candle store (historical API, 3 req/s, C-08).
- **Deviation from Streak:** Results computed locally; aggregation across instruments defined explicitly; featured set
  configurable (Streak rotates its own picks).
- **Priority hint:** Nice-to-have

### FR-HOME-007 Live Backtest widget — calendar heatmap
- **Source features:** HOME-005
- **Requirement:** Under the metrics the widget shall show one mini calendar per month covered by the window (oldest
  left), columns Mon–Fri, one cell per weekday. Cell colour: green shades for positive daily return, red shades for
  negative, 4 intensity steps each by |return| quartile within the window; grey for days with no trade or exchange
  holidays. Daily return % = that day's realised P&L ÷ capital used of the run × 100. Hover/focus on a cell shows a
  tooltip `13 Jul 2026: −1.48%` (or `13 Jul 2026: no trades`).
- **Acceptance criteria:**
  1. Given a 90-day window from 13 Jul to 10 Oct 2026, then 4 month grids (Jul–Oct) are shown.
  2. Given a day with P&L −₹5,122 and capital used ₹3,46,000, then the tooltip reads `−1.48%` and the cell is red.
  3. Given an exchange holiday, then the cell is grey and the tooltip says no trades.
- **Data/Kite dependency:** local backtest results; local holiday calendar.
- **Deviation from Streak:** Explicit daily-return definition (unspecified in Streak).
- **Priority hint:** Nice-to-have

### FR-HOME-008 Live Backtest widget — refresh schedule and data preparation
- **Source features:** HOME-005
- **Requirement:** The featured backtests shall be recomputed automatically once per trading day after 16:00 IST (when
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
  chart (SCALP module), the strategy builder chart pane (EQB-031), the options builder chart drawers (OPT module) and the
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
  tag such as `INDICES`/`NSE`/`NFO-OPT`, LTP and coloured percent change; opens FR-CHART-005), a refresh icon
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
- **Requirement:** On symbol/timeframe change the chart shall load candles from the local candle store, fetching any
  missing range from the Kite historical API (respecting per-interval max window per request: minute 60 days, 3/5/10
  minute 100, 15/30 minute 200, 60 minute 400, day 2000; ≤3 requests/s via the shared rate limiter). Initial window:
  1m → 5 trading days, 3m/5m → 15, 10m/15m → 30, 30m/1h → 120, 1d → 3 years; scrolling left past the loaded range
  lazily loads older data (as far back as Kite provides). During market hours the last candle is built live from
  ticks (OHLC from LTP; volume from the increase in cumulative day volume; indices have zero volume); a new candle starts
  at each interval boundary aligned to 09:15 IST. Candles are raw/unadjusted (NFR-12). Loading shows a spinner; failure
  shows an inline error with Retry; with no data plan the chart shows "Charts need a Kite Connect data plan" (C-01).
  When offline, cached candles are shown with an "Offline — cached data" badge (NFR-14).
- **Acceptance criteria:**
  1. Given a 1 Day chart of RELIANCE with an empty cache, then about 3 years of daily candles appear and the historical
     API was called with windows ≤2000 days.
  2. Given a 5 Minute chart during market hours, when ticks arrive at 10:03, then the 10:00 candle's high/low/close update
     and at 10:05:00 a new 10:05 candle starts.
  3. Given the user scrolls left beyond the first loaded candle, then older candles are appended without resetting zoom.
  4. Given a Personal-plan key, then the chart shows the data-plan message and makes no historical calls.
  5. Given the same symbol/timeframe is reopened, then candles render from the local store without a historical API call
     for already-cached ranges.
- **Data/Kite dependency:** `GET /instruments/historical/:token/:interval` (gokiteconnect `GetHistoricalData`), ticker
  `quote`/`full` mode, local candle store (NFR-03), rate limiter (C-08).
- **Deviation from Streak:** Data stored locally and lazily backfilled; first view of a long history may take seconds
  (3 req/s). Library note: `[Either]`.
- **Priority hint:** Core

### FR-CHART-004 Timeframe selector
- **Source features:** CHART-002
- **Requirement:** The timeframe dropdown shall offer: 1 Minute, 3 Minutes, 5 Minutes, 10 Minutes, 15 Minutes, 30 Minutes,
  1 Hour, 1 Day (Appendix I), mapping 1:1 to Kite intervals minute, 3minute, 5minute, 10minute, 15minute, 30minute,
  60minute, day. Defaults: dashboard and Scalper 1 Hour; builder chart = strategy's base timeframe (default 1 Minute);
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
  BANKEX, and a result list with columns Instrument (symbol + exchange tag + segment icon: C/P badges for options, F for
  futures) and Company name. Empty query lists NSE equities alphabetically. Search uses the shared instrument search
  (FR-WATCH-008) across INDICES, NSE, BSE, NFO-FUT, NFO-OPT, BFO-FUT, BFO-OPT. Up/Down moves the highlight, Enter or
  click selects: the chart switches symbol, the modal closes and the analytics panel reloads for the new symbol. Escape,
  × or backdrop click closes without change.
- **Acceptance criteria:**
  1. Given the modal opens, then the input has focus and the 4 chips are shown.
  2. Given query "BANKNIFTY", then results include NIFTY BANK (index) and current BANKNIFTY futures and options.
  3. Given query "reliance oct 1290 ce", then the matching NFO option appears with a C badge.
  4. Given a result is selected by Enter, then the chart shows that instrument within 1 s (cached) and the modal is closed.
  5. Given Escape is pressed, then the modal closes and the chart symbol is unchanged.
- **Data/Kite dependency:** instrument master (FR-WATCH-008).
- **Deviation from Streak:** Escape closes (§3.2 #8); BFO segments included so SENSEX/BANKEX derivatives are chartable.
  Library note: `[Either]` (own modal).
- **Priority hint:** Core

### FR-CHART-006 Refresh chart data
- **Source features:** CHART-004
- **Requirement:** The refresh icon (tooltip "Refresh chart data") shall re-fetch from Kite the candles of the visible
  range plus the current session for the chart's symbol/timeframe, replace those candles in the local store, and redraw
  without losing studies, drawings or zoom. The icon spins while fetching and is disabled until done. Also present in
  options builder chart drawers.
- **Acceptance criteria:**
  1. Given a cached candle with a wrong close, when refresh is clicked, then the candle shows Kite's current value.
  2. Given refresh is in progress, then a second click has no effect.
  3. Given zoomed to a range, after refresh the same range is still visible.
- **Data/Kite dependency:** historical API (3 req/s).
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
  (colour, width, style, text) via a floating mini-toolbar. Drawings persist only via Save (FR-CHART-007). Shortcuts use
  Alt on Linux.
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
  (`01 Oct '26 09:15`) and price label; a "+" button on the crosshair price label opening "Create order at <price>"
  (FR-CHART-014) when trading is enabled; a gear (bottom-right) opening chart settings (FR-CHART-015); auto-scale (A)
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
  for the default quantity under each button. Clicking opens the order window (ORD-004) prefilled with side, instrument,
  order type MARKET, quantity = default quantity (1 share, or 1 lot for F&O, or remembered F&O quantity per FR-ACCT-002);
  it never places an order directly (NFR-05). When no bid/ask is available (market closed, illiquid), the buttons show
  "—" and still open the order window. For indices the buttons are hidden. The buttons are hidden when trading is not
  enabled (FR-ACCT-008) and replaced by a small "Signal-only mode" tag. Settings → Chart → "Show Buy/Sell buttons"
  (default on) controls visibility.
- **Acceptance criteria:**
  1. Given best bid 1249.30 / ask 1249.50, then the buttons read `BUY 1249.50`, spread `0.20`, `SELL 1249.30`.
  2. Given BUY is clicked on a NIFTY option, then the order window opens with BUY, MARKET, qty = 1 lot (65 for NIFTY as of
     Appendix K) and no order is sent.
  3. Given market closed, then the buttons show "—" (not 0.00).
  4. Given an index chart, then no Buy/Sell buttons are shown.
  5. Given margin for 1 lot is ₹8,908.25, then "₹8,908.25" appears under BUY (refreshed at most every 10 s).
- **Data/Kite dependency:** ticker `full` mode (5-level depth) or `GET /quote`; `POST /margins/orders` (debounced,
  ≤1 req/10 s per chart); order window (ORD-004); static IP + trading enablement (C-04, C-05, C-06).
- **Deviation from Streak:** Opens the order window instead of a possibly one-click order (NFR-05 safety); "—" instead of
  0.00 (§3.2 #19); hidden in signal-only mode (C-04). Library note: `[Full-TV]` has native buy/sell buttons via its
  broker API; `[Open+Own]` React overlay.
- **Priority hint:** Important

### FR-CHART-013 Chart context menu and Trade submenu
- **Source features:** CHART-011
- **Requirement:** Right-click (or Shift+F10) on the chart shall open a menu: "Copy price <price>", "Paste" (paste a copied
  drawing; Ctrl+V), "Trade ›", "Lock vertical cursor line by time", "Remove <N> indicators", "Hide marks on bars",
  "Settings…". The Trade submenu (only when trading is enabled) offers, for the price under the cursor relative to LTP:
  "Buy/Sell 1 <EXCH>:<SYMBOL> @ <price> limit", "Buy/Sell 1 … @ <price> stop" (SL-M), "Buy/Sell 1 … stop <price> limit"
  (SL), "New order… (Shift+T)" and "Trading settings…" (opens order defaults in Settings). Each Trade item opens the
  order window prefilled (type, trigger/limit price, quantity default) — never places directly. Quantity default is
  1 share / 1 lot / remembered F&O qty.
- **Acceptance criteria:**
  1. Given right-click at price 1,249.30 on RELIANCE with LTP 1,260, then the submenu offers "Buy 1 NSE:RELIANCE @ 1,249.30
     limit" and "Sell 1 NSE:RELIANCE @ 1,249.30 stop".
  2. Given "Copy price", then the clipboard contains `1249.30`.
  3. Given 2 studies, then the menu shows "Remove 2 indicators"; choosing it removes both.
  4. Given trading is not enabled, then "Trade ›" is absent.
  5. Given Shift+T on a focused chart, then the order window opens for the charted instrument.
- **Data/Kite dependency:** order window → `POST /orders/regular` (LIMIT/SL/SL-M) via ORD module; clipboard (Wails).
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
  × on a position line opens the order window prefilled to exit the full quantity. Lines update from Kite order-update
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
  3. Given Scalper chart 2 set to a NIFTY option, after restart chart 2 still shows it (if the contract has not expired;
     otherwise it falls back to the underlying with an INFO toast).
- **Data/Kite dependency:** local preference store.
- **Deviation from Streak:** Expired-contract fallback defined. Library note: `[Either]`.
- **Priority hint:** Important

### FR-CHART-017 Full-view chart
- **Source features:** CHART-013
- **Requirement:** The system shall provide a full-view chart view opened with parameters (instrument, timeframe), used by
  the chart action on scanner result rows (SCAN-014) and available from any chart's ⋯ menu ("Open in full view"). It
  shows the full chart component (header with symbol search, LTP/%, timeframe, Save, Clear, Indicators; OHLC readout;
  Buy/Sell; volume) filling the content area, with a back arrow returning to the originating view with its state intact.
  F11 toggles hiding the sidebar/top bar for a distraction-free chart.
- **Acceptance criteria:**
  1. Given a scanner result row for ADANIENT at 1 Day, when its chart action is used, then the full-view chart shows
     ADANIENT NSE at 1 Day.
  2. Given the back arrow, then the scanner results view reappears with the same results and scroll position.
  3. Given F11, then only the chart is visible; F11 again restores the frame.
- **Data/Kite dependency:** as FR-CHART-003.
- **Deviation from Streak:** Streak opens a new browser tab (`/chartiq?seg_sym=…`); Wails v2 supports a single window, so
  the chart opens as an in-app view with back navigation. Library note: `[Either]`.
- **Priority hint:** Important

---
## FR module ANLY — Analytics panel

Analytics charts (OI bars, PCR, A/D, IV/Straddle) are simple bar/line charts and are `[Either]` with respect to C-12
(any open chart library suffices; they do not need the TradingView Charting Library).

### FR-ANLY-001 Analytics tab bar and symbol-dependent tab set
- **Source features:** ANLY-001
- **Requirement:** Under the dashboard main chart (and in the Scalper bottom-left area for the left chart's symbol) the
  system shall show a tab bar with up to six tabs (icon + label, active tab filled): Technicals, Option Chain, OI, PCR,
  Advance/Decline, ATM IV/Straddle. The tab set is derived from the charted instrument via the instrument master and
  universe store: **Technicals** — always; **Option Chain, OI, PCR, ATM IV/Straddle** — only if the instrument is an index
  or stock with listed options (NFO/BFO) in the instrument master; **Advance/Decline** — only if the instrument is an index
  with a constituent list in the universe store (FR-WATCH-010). Futures charts show the tabs of their underlying's option
  set except Advance/Decline; option contracts show Technicals only. If the active tab is not in the new set after a
  symbol change, Technicals becomes active; otherwise the active tab is kept.
- **Acceptance criteria:**
  1. Given NIFTY 50 with a Nifty 50 constituent list imported, then all six tabs are shown.
  2. Given RELIANCE (F&O stock), then five tabs are shown (no Advance/Decline).
  3. Given a non-F&O stock, then only Technicals is shown.
  4. Given an option contract, then only Technicals is shown.
  5. Given OI tab active on NIFTY 50, when the chart switches to an option, then Technicals becomes active.
- **Data/Kite dependency:** instrument master (segment, underlying `name`, instrument_type); universe store.
- **Deviation from Streak:** Non-F&O and futures tab sets defined explicitly (inferred/unknown in Streak).
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
  ADX 14 (>20 and +DI>−DI rising → Bullish, >20 and −DI>+DI → Bearish), Awesome Oscillator (saucer/zero-cross rules),
  Momentum 10 (rising → Bullish, falling → Bearish), MACD 12,26,9 (MACD>signal → Bullish, < → Bearish), Stochastic RSI
  3,3,14,14 (<20 rising → Bullish, >80 falling → Bearish), Williams %R 14 (<−80 rising → Bullish, >−20 falling → Bearish),
  Bull/Bear Power 13 (bull/bear power rules), Ultimate Oscillator 7,14,28 (>70 → Bullish, <30 → Bearish), Rate of Change 9
  (>0 rising → Bullish, <0 falling → Bearish); all other cases Neutral. The exact rule table shall be fixed in the design
  spec with golden tests. The gauge shows "Updated <time>" and recomputes within 2 s of each candle close (NFR-08).
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
  candle and shows a temporary marker.
- **Acceptance criteria:**
  1. Given a golden series containing a bullish engulfing at 09 Oct 2026 14:15 (1h), then a row "Bullish engulfing |
     09 Oct 2026 14:15" is listed.
  2. Given 7 events, then the 5 newest are visible and the other 2 reachable by scrolling, newest first.
  3. Given an event row is activated, then the chart centres that candle with a marker.
- **Data/Kite dependency:** local pattern engine over local candles.
- **Deviation from Streak:** Event rows navigate the chart (not clickable in Streak); event definitions are ours.
- **Priority hint:** Nice-to-have

### FR-ANLY-006 Option Chain tab
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

### FR-ANLY-007 Option Chain row trade actions
- **Source features:** ANLY-003
- **Requirement:** Hovering or focusing an option-chain row shall reveal `S` `B` buttons next to the strike on the call side
  and `B` `S` on the put side. Each opens the order window (ORD-004) — the same component as watchlist B/S — prefilled with
  that option contract, side, quantity = 1 lot (or remembered F&O qty, FR-ACCT-002). Hidden when trading is not enabled.
- **Acceptance criteria:**
  1. Given row 22,500 is focused by keyboard, then the four B/S buttons are visible and Tab-reachable.
  2. Given call-side B is activated, then the order window opens for the 22,500 CE of the selected expiry, BUY, 1 lot.
  3. Given signal-only mode, then no B/S buttons appear.
- **Data/Kite dependency:** order window (ORD-004).
- **Deviation from Streak:** Keyboard access (FR-SHELL-015); same order window as watchlist (resolves catalogue question).
- **Priority hint:** Important

### FR-ANLY-008 OI tab
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

### FR-ANLY-009 PCR tab
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

### FR-ANLY-010 Analytics time-series recorder and backfill
- **Source features:** ANLY-004, ANLY-005, ANLY-006, ANLY-007
- **Requirement:** While the app runs during market hours, a background recorder shall store 1-minute snapshots for each
  "tracked underlying" (default NIFTY 50, NIFTY BANK, SENSEX, plus any underlying opened in the analytics panel that day):
  underlying LTP; per-strike OI and LTP for the nearest expiry within ATM ±20 strikes; ATM CE/PE LTP; and, for indices with
  constituent lists, advance/decline counts. It shall also store each session's closing OI per contract (OI baseline).
  Retention default 30 days (configurable). Backfill for missed periods uses Kite historical minute candles (`oi=1`) through
  the shared rate limiter at background priority; user-visible fetches take precedence. The recorder must stay within the
  subscription budget (C-07) and must not degrade live signal latency (NFR-08).
- **Acceptance criteria:**
  1. Given the app runs 09:15–15:30, then 375 snapshots per tracked underlying exist for that day.
  2. Given the app starts at 11:00, then 09:15–10:59 is backfilled from historical data within the rate limit and marked as
     backfilled.
  3. Given retention 30 days, then data older than 30 days is purged on the next start.
  4. Given a deployment signal and backfill compete, then the signal is evaluated within 2 s of candle close.
- **Data/Kite dependency:** ticker, historical API, local time-series store.
- **Deviation from Streak:** Required because Kite offers no PCR/IV/breadth history (C-03, C-08); history exists only from
  first install plus what Kite minute candles allow.
- **Priority hint:** Important

### FR-ANLY-011 Advance/Decline tab
- **Source features:** ANLY-006
- **Requirement:** For indices with a constituent list, the tab shall show a two-line chart over the last 2 trading
  sessions at 1-minute resolution: `Advance` (teal) = number of constituents with LTP > previous close, `Decline` (red) =
  number with LTP < previous close (unchanged excluded), y-axis 0…constituent count, "As of <time>" top-right. Tooltip:
  `09 Oct 09:57`, `Advances: 48`, `Declines: 2`. Data from the recorder (FR-ANLY-010), backfilled from constituents'
  minute candles. If the list is stale (older than the user-set threshold, default 90 days) a note "Constituent list
  imported <date>" is shown (survivorship, C-09).
- **Acceptance criteria:**
  1. Given 50 constituents with 48 up, 1 flat, 1 down at 09:57, then Advances 48, Declines 1.
  2. Given NIFTY 50 without an imported list, then the tab is not shown (FR-ANLY-001).
  3. Given a 120-day-old list, then the stale note is visible.
- **Data/Kite dependency:** universe store (user-imported constituents, C-09); ticker; historical minute candles.
- **Deviation from Streak:** Constituents user-imported (C-09); history only as recorded/backfilled.
- **Priority hint:** Nice-to-have

### FR-ANLY-012 ATM IV/Straddle tab
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

### FR-ANLY-013 Analytics refresh and staleness
- **Source features:** ANLY-001, ANLY-002, ANLY-003
- **Requirement:** Each analytics tab shall show its data time ("As of …") and refresh: Technicals on each candle close of the
  chart timeframe; Option Chain live (FR-ANLY-006); OI/PCR/A-D/IV every minute during market hours. A tab whose data is older
  than 2 minutes during market hours shows a "Delayed" badge; when offline, "Offline — cached" (NFR-14). Data for a hidden
  (inactive) tab is not polled.
- **Acceptance criteria:**
  1. Given the feed disconnects for 3 minutes in market hours, then visible tabs show "Delayed".
  2. Given the PCR tab is not active, then no PCR-specific quote calls are made (recorder continues).
- **Data/Kite dependency:** as per tab.
- **Deviation from Streak:** Staleness made visible (Streak refresh cadence unknown).
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
  5 s) and show a "polled" hint. Rows can be reordered by drag-and-drop and by Alt+Up/Alt+Down; order persists. Lists
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
  3. Given query "nifty 22500 ce", then NFO options appear and can be added.
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
- **Data/Kite dependency:** ticker; instrument master; order window.
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
  "Search, e.g. SBIN, TCS"); universe chips NIFTY50, NIFTY100, NIFTY200, NIFTY500, BANKNIFTY (from the universe store) and
  one chip per watchlist; a left result list with a select-all checkbox and rows (checkbox, logo, symbol, exchange tag,
  company name); a right "Selected (n)" list (or "Selected (n / max)" when the caller passes a maximum) with live price,
  percent change and a remove (−) button per row; footer "Save as watchlist" (prompts for a name, creates a watchlist from
  the selection) and "Done" (returns the selection). Clicking a chip lists that universe's/watchlist's instruments in the
  left list; select-all selects all listed. The caller may restrict segments (e.g. equity strategies: NSE and BSE cash;
  scanners: as per universe). Duplicate selections are ignored. Escape/× closes without applying; Done applies.
- **Acceptance criteria:**
  1. Given the NIFTY50 chip and select-all, then 50 instruments appear on the right and the counter reads "Selected (50)".
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
- **Requirement:** The system shall download the Kite instruments dump for NSE, BSE, NFO, BFO (and indices) once per trading
  day after 08:30 IST (or at first start of the day), store it locally with a version date, and drop expired contracts.
  Search shall be local, return results within 100 ms for any query, match on tradingsymbol, name and a generated display
  name (FR-WATCH-013), be case-insensitive and token-based ("reliance oct 1290 ce"), and rank: exact symbol > symbol prefix
  > display-name tokens all matched > name contains; indices and cash equities before derivatives on equal rank; nearest
  expiry first among derivatives. BSE cash results shall be shown for all BSE equities (no BSE-1000 restriction) but
  flagged when the same company is on NSE (NSE ranked first). If the download fails, the previous dump is used and a
  WARNING toast states its date.
- **Acceptance criteria:**
  1. Given 08:45 IST on a trading day and a dump dated yesterday, then a new dump is downloaded once.
  2. Given query "BANKNIFTY", then NIFTY BANK (index) appears in the top 3.
  3. Given any query on the full dump, then results return in <100 ms on the reference laptop.
  4. Given the download fails, then search still works on yesterday's dump and a WARNING names its date.
  5. Given a contract that expired yesterday, then it no longer appears in search.
- **Data/Kite dependency:** `GET /instruments` / `GET /instruments/:exchange` (gokiteconnect `GetInstruments`/
  `GetInstrumentsByExchange`), works on the Personal plan.
- **Deviation from Streak:** BSE not restricted to BSE 1000 (Streak restriction is a product limit, NFR-13).
- **Priority hint:** Core

### FR-WATCH-009 Universes (scan-on baskets)
- **Source features:** WATCH-006
- **Requirement:** The system shall offer these 29 universes, in this order, wherever a universe is selected (Discover
  Scanners, scanner builder "Scan on", Live Scanners widget): Nifty 50, Bank Nifty, Sensex, Bankex, Nifty 100, Nifty 200,
  Nifty 500, Indices, Futures, NFO Equities, Nifty Weekly Options, Sensex Weekly Options, Nifty Next Week Options, Sensex
  Next Week Options, Nifty Monthly Options, Sensex Monthly Options, Banknifty Monthly Options, Finnifty Monthly Options,
  Energy, Communication Services, Materials, Consumer Discretionary, Consumer Staples, Industrials, Healthcare,
  Information Technology, Financials, Utilities, Real Estate. The scanner builder additionally lists custom baskets
  (FR-WATCH-012) and watchlists after "Futures". Membership: index baskets and sectors from the universe store
  (FR-WATCH-010); Indices = all index instruments of the instrument master; Futures = current-month futures of all NFO/BFO
  underlyings; NFO Equities = cash equities that have NFO derivatives; option baskets = all CE/PE contracts of the named
  underlying for the current weekly / next weekly / current monthly expiry derived from the instrument master, narrowed by
  the scanner's strike-range filter (default ATM ±6, SCAN module). Option-basket labels in pills may use short forms
  (e.g. "Nifty current week"). Each universe shows its member count and data date in a tooltip.
- **Acceptance criteria:**
  1. Given any universe dropdown, then the 29 names are listed in the order above.
  2. Given today is before the weekly expiry, then "Nifty Weekly Options" contains only that expiry's NIFTY CE/PE contracts.
  3. Given a weekly expiry has passed, then "Nifty Next Week Options" resolves to the following expiry automatically.
  4. Given "NFO Equities", then every member has at least one NFO future in the instrument master.
  5. Given a universe exceeds the live budget when used live, then the user is warned with the member count and budget.
- **Data/Kite dependency:** instrument master; universe store; subscription budget (C-07).
- **Deviation from Streak:** "Futures" basket supported (Conflicts §6 #11 — UI authoritative). Sector membership comes
  from the imported constituent files' Industry field mapped to the 11 sectors (C-09).
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
  The user may save the selection as a named custom basket (name 1–40 chars, unique) which then appears in every universe
  list after the built-in universes; baskets are managed (rename, edit, delete with confirmation) in Settings →
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
- **Requirement:** One formatter shall render instrument names everywhere from instrument-master fields: equities/indices
  `SYMBOL` (indices by display name, e.g. NIFTY 50); futures `UNDERLYING MON FUT` (e.g. `RELIANCE OCT FUT`) with an "F"
  badge; monthly options `UNDERLYING MON STRIKE CE|PE` (e.g. `BANKNIFTY OCT 55300 CE`); weekly options `UNDERLYING DDth MON
  STRIKE CE|PE` with the ordinal day and a superscript "w" marker (e.g. `NIFTY 13ᵗʰ ʷ OCT 22500 CE`); dynamic option legs
  (OPT module) `UNDERLYING DD MON LTP@<price> CE|PE`. Options carry a circular C (calls) or P (puts) badge in result lists.
  A contract is weekly if its expiry is not the last expiry of that underlying in that month. Exchange-qualified identifiers
  use Kite's `EXCHANGE:TRADINGSYMBOL` form (e.g. `NSE:RELIANCE`) internally and in route/deep-link parameters.
- **Acceptance criteria:**
  1. Given tradingsymbol for NIFTY expiry 2026-10-13 strike 22500 CE (a weekly), then the display is `NIFTY 13ᵗʰ ʷ OCT 22500 CE`.
  2. Given the BANKNIFTY 2026-10-27 55300 CE (monthly), then the display is `BANKNIFTY OCT 55300 CE`.
  3. Given a RELIANCE October future, then the display is `RELIANCE OCT FUT` with an F badge.
- **Data/Kite dependency:** instrument master (`expiry`, `strike`, `instrument_type`, `name`).
- **Deviation from Streak:** Identifier format `NSE:RELIANCE` instead of Streak's `NSE_RELIANCE`.
- **Priority hint:** Core

### FR-WATCH-014 Consistent exchange tagging
- **Source features:** WATCH-008
- **Requirement:** The exchange tag of an instrument shall always be derived from its own instrument-master record and be
  one of `INDICES`, `NSE`, `BSE`, `NFO-FUT`, `NFO-OPT`, `BFO-FUT`, `BFO-OPT`. Where a picker lists underlyings (e.g. options
  underlying picker) the same instrument shall carry the same tag as in every other picker (NSE for NSE-listed F&O stocks).
- **Acceptance criteria:**
  1. Given RELIANCE appears in the chart search, watchlist, instrument picker and options underlying picker, then all show
     `NSE`.
  2. Given an automated check over all rendered instrument rows in a test fixture, then no instrument has two different tags.
- **Data/Kite dependency:** instrument master.
- **Deviation from Streak:** Fixes inconsistent BSE/NSE tagging (§3.2 #18).
- **Priority hint:** Important

---

## FR module ACCT — Account & settings

### FR-ACCT-001 Profile view
- **Source features:** ACCT-001
- **Requirement:** The Profile view (header: back arrow, title "Profile", description "Your Zerodha account details as
  returned by Kite") shall show an identity card (avatar from Kite if provided, otherwise initials; name; client ID with a
  "Zerodha" chip) and read-only fields Name, Email, Client ID, Broker ("Zerodha"), plus Exchanges and Products enabled
  (from the profile response). Below: the preference toggles FR-ACCT-002 and FR-ACCT-003 (applied immediately, no Save
  button), session information (logged in at, expires at 06:00 IST), and the Log out button (FR-ACCT-004). A "Refresh"
  re-fetches the profile.
- **Acceptance criteria:**
  1. Given a valid session, then Name, Email and Client ID equal `GET /user/profile` values and fields are not editable.
  2. Given a toggle is changed, then it persists without any Save action.
  3. Given login at 09:02 today, then session info shows "Expires 06:00 tomorrow".
- **Data/Kite dependency:** `GET /user/profile` (gokiteconnect `GetUserProfile`).
- **Deviation from Streak:** Session information and exchanges/products added (C-02).
- **Priority hint:** Important

### FR-ACCT-002 Remember F&O order quantity
- **Source features:** ACCT-002
- **Requirement:** A toggle "Remember F&O order quantity" (help "Reuse the last quantity for NFO-OPT, NFO-FUT, BFO-OPT and
  BFO-FUT orders"), default off. When on, after an order window for an F&O contract is submitted, its quantity in lots is
  stored per underlying and segment type (options/futures); the next order window for any contract of the same underlying
  and type is prefilled with that number of lots (converted with the contract's lot size). When off, F&O order windows
  default to 1 lot. Cash-segment quantity is never remembered.
- **Acceptance criteria:**
  1. Given the toggle on and a NIFTY option order of 3 lots submitted, when another NIFTY option order window opens, then
     quantity = 3 lots.
  2. Given the toggle off, then a NIFTY option order window defaults to 1 lot.
  3. Given the lot size changed in the instrument master, then the prefilled quantity = 3 × new lot size.
  4. Given an equity order of 25 shares, then the next equity order window does not default to 25.
- **Data/Kite dependency:** local preference; instrument master lot sizes.
- **Deviation from Streak:** Storage key (per underlying + type, in lots) defined explicitly; Streak semantics unknown.
- **Priority hint:** Nice-to-have

### FR-ACCT-003 Desktop notifications toggle
- **Source features:** ACCT-003
- **Requirement:** A toggle "Desktop notifications" (help "Show system notifications for trades, scanner alerts and
  deployments"), default off. When on, the system shall raise an OS notification (freedesktop notification service) for:
  deployment signal awaiting confirmation, order filled/rejected/cancelled, deployment stopped/expired/errored, scanner
  alert hit, session expiry, and feed offline >60 s during market hours. Clicking a notification focuses the app and opens
  the related view. A "Send test notification" button verifies delivery. In-app toasts and the Notifications view are
  unaffected by the toggle. If the notification service is unavailable, the toggle shows "Not available on this desktop".
- **Acceptance criteria:**
  1. Given the toggle on, when a scanner alert fires while the window is minimised, then an OS notification with the
     scanner and instrument names appears within 2 s.
  2. Given the toggle off, then no OS notification is raised but the in-app notification is recorded.
  3. Given an OS notification for a signal is clicked, then the app comes to front on the related Notifications entry.
- **Data/Kite dependency:** OS notification service via D-Bus; local event bus.
- **Deviation from Streak:** OS notifications replace browser push / mobile push (no server, C-03).
- **Priority hint:** Important

### FR-ACCT-004 Log out
- **Source features:** ACCT-004
- **Requirement:** "Log out" (outline button with icon) shall ask for confirmation; if deployments are running, the dialog
  states that live deployments will stop receiving data and signals until the next login. On confirm the system invalidates
  the Kite access token, deletes it from the OS keyring, closes ticker connections, pauses all deployments and scanners
  (state preserved for resume after login), and shows the login screen. Local data (strategies, backtests, candles) remains.
  The app also enforces a single running instance: a second launch focuses the existing window.
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
  secret only in the OS keyring (NFR-04), never in files or logs; and verifies the credentials by completing a first login
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
- **Source features:** ACCT-005, ACCT-004
- **Requirement:** "Log in with Kite" shall open the Kite login page in the system browser and start a loopback HTTP
  listener on the configured port; on redirect with `request_token` the app exchanges it for an access token (checksum
  SHA-256 of api_key + request_token + api_secret), stores the token in the keyring with expiry at the next 06:00 IST,
  shows "Logged in — you can close this browser tab" in the browser, and focuses the app. Fallback if the loopback
  redirect cannot be used: the user pastes the full redirect URL or the request_token into a field. The login is always
  manual (no stored passwords/TOTP automation). At 06:00 IST the session is treated as expired: live features stop,
  the status indicator shows "Logged out" (FR-SHELL-005), and a banner "Kite session expired — log in to continue" with a
  Log in button is shown. On start with a still-valid token, no login is needed. Any Kite `TokenException` response
  triggers the same expired state.
- **Acceptance criteria:**
  1. Given valid credentials, when the user completes Kite login in the browser, then within 3 s the app shows Home and the
     status indicator shows "Live" (Connect plan).
  2. Given the loopback listener is blocked, when the user pastes the redirect URL, then login completes.
  3. Given the clock passes 06:00 IST, then within 5 s the expired banner appears and no Kite calls are made until login.
  4. Given the app restarts at 14:00 after a 09:00 login, then no login is requested.
  5. Given a `TokenException` from any endpoint, then the expired state is entered.
- **Data/Kite dependency:** Kite login URL, `POST /session/token` (gokiteconnect `GenerateSession`), keyring.
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
  explaining why. A global kill switch ("Stop all trading") is reachable from the status panel (NFR-05; behaviour in ORD/DEP).
- **Acceptance criteria:**
  1. Given a fresh install, then trading is disabled and no B/S buttons are shown anywhere.
  2. Given trading is enabled, then B/S buttons appear in watchlists, charts and option chains.
  3. Given an IP-related rejection from Kite, then trading becomes disabled and the toast names the cause.
- **Data/Kite dependency:** order endpoints (rejection messages); local settings.
- **Deviation from Streak:** Static-IP gate and signal-only default are new (C-04, NFR-05).
- **Priority hint:** Core

### FR-ACCT-009 Limits and usage view
- **Source features:** ACCT-006
- **Requirement:** Settings → Limits shall list the limits that apply to the app and the current usage: Kite limits —
  orders per second (10), per minute (400), per day (5,000) with today's count; modifications per order (25); quote API
  1 req/s; historical API 3 req/s with current queue length; WebSocket subscriptions used / 9,000 (3 × 3,000) and
  connections used / 3; per-request historical windows (Appendix I). The view states that the app imposes no limits of its
  own on strategies, scanners, deployments, backtests, watchlist sizes or custom baskets (NFR-13), and lists app
  configuration values that act as budgets (e.g. option-chain subscription budget, recorder strike window) with links to
  change them. When a Kite limit is hit anywhere, the toast links to this view.
- **Acceptance criteria:**
  1. Given 12 orders placed today, then the view shows "12 / 5,000 orders today".
  2. Given 2,400 instruments subscribed, then it shows "2,400 / 9,000".
  3. Given an HTTP 429 from Kite, then a WARNING toast with a "View limits" link appears.
- **Data/Kite dependency:** local counters from the order throttle, rate limiter and subscription manager.
- **Deviation from Streak:** Streak's own caps (100 deployments, 25 live scanners, 50 instruments, backtests/day,
  lookback windows, 10 × 50 baskets) are not applied (NFR-13); only Kite/SEBI limits.
- **Priority hint:** Important

### FR-ACCT-010 Built-in help
- **Source features:** ACCT-007
- **Requirement:** The app shall bundle an offline help centre (our own content) opened from the profile menu, (i) icons
  and view help menus, as an in-app view/side panel with a search box (full-text, results as you type), category cards
  (Dashboard & charts, Scalper, Strategies, Scanners, Backtesting, Deployments & orders, Kite setup, FAQ, Advanced
  conditions), articles with a category tree and "Updated for v<x>" stamp, and a "Keyboard shortcuts" article listing all
  shortcuts. It shall document backtest assumptions, square-off rules, limits and deviations from Streak behaviour.
  An "Open logs folder" link is provided for support (NFR-10).
- **Acceptance criteria:**
  1. Given no network, when help is opened and "square-off" searched, then matching articles are listed.
  2. Given the (i) icon of the Advanced conditions header, then the Advanced conditions article opens.
  3. Given the Keyboard shortcuts article, then it lists every shortcut defined in the FRs (e.g. Alt+T, Alt+H, Shift+T, F11).
- **Data/Kite dependency:** none (bundled content).
- **Deviation from Streak:** Offline, bundled, own content instead of an external help site; adds a shortcuts list and
  logs access (absent in Streak).
- **Priority hint:** Important

### FR-ACCT-011 Mobile companion (closest equivalent)
- **Source features:** ACCT-008
- **Requirement:** The system shall not provide a mobile app. Alerts that Streak delivers to its mobile app shall be
  delivered as desktop notifications (FR-ACCT-003) and recorded in the Notifications view. No data shall leave the machine
  for relay to phones (NFR-04). Any future remote-alert channel is out of scope for this requirements version.
- **Acceptance criteria:**
  1. Given a scanner alert, then it is recorded in Notifications and (if enabled) raised as a desktop notification, and
     network monitoring shows no outbound request other than to Kite.
- **Data/Kite dependency:** none.
- **Deviation from Streak:** No mobile companion (single-user local desktop scope; C-03, NFR-04).
- **Priority hint:** Nice-to-have

### FR-ACCT-012 Settings view
- **Source features:** ACCT-001, ACCT-005, ACCT-006
- **Requirement:** A Settings view (profile menu → Settings) shall group: Kite connection (FR-ACCT-005/006/007), Trading
  (FR-ACCT-008, order defaults: default product per segment, default order type), Appearance (theme, follow system,
  sidebar collapsed), Chart (show Buy/Sell buttons, show orders on chart), Universes & baskets (FR-WATCH-010/012), Data
  (data directory path read-only, candle-store size, "Clear cached candles", backfill status, analytics retention days,
  instrument master date with "Refresh now"), Notifications (FR-ACCT-003), Analytics (risk-free rate, dividend yield,
  recorder strike window, option-chain budget), Limits (FR-ACCT-009), Tours ("Reset all tours"), About (version, build,
  licences of bundled libraries). Every setting has a default, validates input inline and applies immediately.
- **Acceptance criteria:**
  1. Given "Clear cached candles" → confirm, then the candle store size becomes ~0 and charts re-fetch on next open.
  2. Given risk-free rate set to "abc", then an inline error is shown and the old value is kept.
  3. Given About, then the version equals the profile-menu version and third-party licences are listed.
- **Data/Kite dependency:** local only (SQLite settings, keyring).
- **Deviation from Streak:** Streak has no settings page (its /settings is a 404); a local desktop app needs one.
- **Priority hint:** Core

---
## Traceability — feature ID to FR IDs

Covers all 53 feature IDs of modules SHELL (12), HOME (5), CHART (13), ANLY (7), WATCH (8), ACCT (8); generated from the
"Source features" lines and checked against `docs/research/FEATURES.md` (0 unmapped).

| Feature ID | FR IDs |
|---|---|
| SHELL-001 | FR-SHELL-001, FR-SHELL-002, FR-SHELL-014, FR-SHELL-015, FR-SHELL-016 |
| SHELL-002 | FR-SHELL-004 |
| SHELL-003 | FR-SHELL-005 |
| SHELL-004 | FR-SHELL-006 |
| SHELL-005 | FR-SHELL-002, FR-SHELL-003 |
| SHELL-006 | FR-SHELL-007 |
| SHELL-007 | FR-SHELL-008 |
| SHELL-008 | FR-SHELL-009 |
| SHELL-009 | FR-SHELL-010 |
| SHELL-010 | FR-SHELL-011 |
| SHELL-011 | FR-SHELL-012 |
| SHELL-012 | FR-SHELL-013, FR-SHELL-015 |
| HOME-001 | FR-HOME-001 |
| HOME-002 | FR-HOME-002 |
| HOME-003 | FR-HOME-003 |
| HOME-004 | FR-HOME-004, FR-HOME-005 |
| HOME-005 | FR-HOME-006, FR-HOME-007, FR-HOME-008 |
| CHART-001 | FR-CHART-001, FR-CHART-002, FR-CHART-003 |
| CHART-002 | FR-CHART-003, FR-CHART-004 |
| CHART-003 | FR-CHART-005 |
| CHART-004 | FR-CHART-006 |
| CHART-005 | FR-CHART-007 |
| CHART-006 | FR-CHART-008 |
| CHART-007 | FR-CHART-009 |
| CHART-008 | FR-CHART-010 |
| CHART-009 | FR-CHART-011 |
| CHART-010 | FR-CHART-012 |
| CHART-011 | FR-CHART-013, FR-CHART-014 |
| CHART-012 | FR-CHART-015 |
| CHART-013 | FR-CHART-016, FR-CHART-017 |
| ANLY-001 | FR-ANLY-001, FR-ANLY-002, FR-ANLY-013 |
| ANLY-002 | FR-ANLY-003, FR-ANLY-004, FR-ANLY-005, FR-ANLY-013 |
| ANLY-003 | FR-ANLY-006, FR-ANLY-007, FR-ANLY-013 |
| ANLY-004 | FR-ANLY-008, FR-ANLY-010 |
| ANLY-005 | FR-ANLY-009, FR-ANLY-010 |
| ANLY-006 | FR-ANLY-010, FR-ANLY-011 |
| ANLY-007 | FR-ANLY-010, FR-ANLY-012 |
| WATCH-001 | FR-WATCH-001, FR-WATCH-002, FR-WATCH-006 |
| WATCH-002 | FR-WATCH-004, FR-WATCH-008 |
| WATCH-003 | FR-WATCH-005, FR-WATCH-006 |
| WATCH-004 | FR-WATCH-003 |
| WATCH-005 | FR-WATCH-007, FR-WATCH-008 |
| WATCH-006 | FR-WATCH-009, FR-WATCH-010, FR-WATCH-011 |
| WATCH-007 | FR-WATCH-012 |
| WATCH-008 | FR-SHELL-016, FR-WATCH-008, FR-WATCH-013, FR-WATCH-014 |
| ACCT-001 | FR-ACCT-001, FR-ACCT-012 |
| ACCT-002 | FR-ACCT-002 |
| ACCT-003 | FR-ACCT-003 |
| ACCT-004 | FR-ACCT-004, FR-ACCT-006 |
| ACCT-005 | FR-ACCT-005, FR-ACCT-006, FR-ACCT-007, FR-ACCT-008, FR-ACCT-012 |
| ACCT-006 | FR-ACCT-008, FR-ACCT-009, FR-ACCT-012 |
| ACCT-007 | FR-ACCT-010 |
| ACCT-008 | FR-ACCT-011 |

## Open questions (not resolvable from the catalogue)

1. **Chart library (C-12).** Licensed TradingView Charting Library vs open library + own implementation decides the effort
   of FR-CHART-007/008/009/010/013/014/015 (107 studies, ~80 drawing tools, dialogs, order lines). Needs a design decision.
2. **Drawing-tool variant list.** Streak's full TV flyout lists were not transcribed; FR-CHART-010 proposes a minimum set.
   Confirm the required set (especially if the open-library route is chosen).
3. **Technicals vote composition.** Streak's 27-vote set and verdict rules are unpublished; FR-ANLY-003/004 define our own
   (incl. pivot period per timeframe and the "—" verdict meaning). Confirm acceptable.
4. **IV model parameters.** Default risk-free rate and dividend yield, and Black-Scholes on spot vs Black-76 on futures for
   FR-ANLY-012.
5. **PCR scope.** Whole-chain PCR needs OI for every strike; FR-ANLY-009 uses ATM ±20 strikes. Is a strike-window PCR
   acceptable, or should the recorder subscribe the full chain (subscription budget impact)?
6. **Holdings card P&L.** Overall (chosen) vs day P&L as the headline; inclusion of T1 and collateral quantities.
7. **"Closed" deployments** definition for FR-HOME-003 must match the DEP module's status model.
8. **Featured strategies and scanners** on Home (FR-HOME-004/006): keep Streak's observed selection order or let the user
   choose from the start? Also confirm Live Backtest daily-return definition (P&L ÷ capital used).
9. **Watchlist/basket sizes.** NFR-13 removes caps; confirm no soft cap is wanted for watchlists (Streak 50), instrument
   picker (Streak 50) and custom baskets (Streak 10 × 50), given the 9,000-subscription limit.
10. **BSE coverage.** Streak restricts BSE to BSE 1000 stocks; FR-WATCH-008 shows all BSE equities. Confirm.
11. **Kite loopback redirect.** Whether Kite accepts `http://127.0.0.1:<port>` as the app redirect URL (A07 UNVERIFIED);
    the paste fallback covers it but the primary flow depends on it.
12. **Exchange holiday calendar source.** Kite has no holiday endpoint; FR-SHELL-005 assumes a shipped, user-editable list.
    Confirm the update process.
13. **Public-IP check.** Verifying the machine's egress IP against the registered static IP needs an external IP-echo
    service, which conflicts with "no telemetry" (NFR-04). FR-ACCT-008 relies on Kite rejections instead — acceptable?
14. **Constituent snapshot redistribution.** Shipping index constituent lists in the package (FR-WATCH-010) — confirm this
    is acceptable under the providers' terms (C-09), else ship empty and require import.
15. **Sector universes.** Mapping of index-file "Industry" values to the 11 sector universes (FR-WATCH-009/010) is our own;
    Streak's membership source is unknown.
16. **"Indices" and "Futures" universes** exact membership (all indices in the instrument master? current-month only?).
17. **Remember F&O quantity** key (per underlying + type, in lots) is our design; Streak semantics unknown.
18. **Chart type.** Streak is candlestick-only (§3.3); confirm no Heikin-Ashi/line charts are wanted.
19. **Chart layout persistence model** (studies per context, drawings per context + symbol, explicit Save) — confirm vs an
    autosave model.
20. **Tour content.** Streak's full backtest-page tour text was not captured; our tours need copy written from scratch.
