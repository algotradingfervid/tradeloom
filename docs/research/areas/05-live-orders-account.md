# Area 05 — Live Trading / Deployment, Orders, Account & Settings

Clean-room observation of the logged-in Streak web app (https://www.streak.tech) on 2026-10-11.
Read-only: nothing was deployed, saved, ordered, toggled or changed. Personal data is written as
[redacted]. The account observed has **no deployments, no orders, no positions, no holdings and no
notifications**, so most screens below were seen in their empty state only.

Viewport used: 1624 x 949 (desktop).

---

## Global shell elements relevant to this area

Top bar (right side), left to right:
- Market status dot + "Live" label (green dot while market open; dot alone on some pages, amber dot
  seen while the profile menu was open — colour likely reflects connection/market state).
- **Order Book** link (`/order-book`) — icon + label; highlighted blue when active.
- **Positions** link (`/positions`) — icon + label; highlighted blue when active.
- **Notifications bell** — a plain link to `/notifications` (a full page, NOT a dropdown panel).
- **Profile** avatar button — opens a dropdown menu (see "Profile menu").

Sidebar: "Live Strategies" icon carries a red dot badge; "Scanner Alerts" also carries a red dot
(unread / attention indicator).

All account-area pages share a page-header pattern: back arrow (←) + H1 title, a one-line grey
description under it, and right-aligned utility controls (refresh icon, Search box). Tables use a
light header row with small grey column labels; empty state = a light-blue illustration
(empty folder/box with floating circles) + a centred grey message.

---

## 1. Live Strategies — `/live-trades`

![Live Strategies, empty state](../screenshots/05-live-orders-account/01-live-strategies-empty.jpg)

**Feature description:** One table that tracks every strategy you have deployed for notification-based
trading. For each strategy and instrument it shows live price, P&L, target and stop-loss levels and the
current status, and gives a way to open that deployment's log.

- **URL:** `/live-trades` (sidebar "Live Strategies"; the sidebar icon has a red dot badge).
- **Header:** H1 "Live Strategies", then the description: "Monitor and manage all strategies taken live
  via notification-based trading. View live status, order logs, re-execute or stop strategies, and
  track instrument-wise execution." The description confirms that rows support **re-execute** and
  **stop**.
- **Mode tabs:** one segmented tab, **"Notification Based Trading"** (filled blue, selected). It is the only
  deployment mode. No Auto, Paper or Virtual tab exists. The help FAQ confirms that "virtual trading /
  virtual deployment is not supported" and that full automation is pending regulatory approval.
- **Table columns** (in this order): Strategy / Instrument · Avg. Price · LTP · Day Change · TP · SL · P&L ·
  Net Change · Status / Cycle · Log.
  - The "Strategy / Instrument" header implies that the strategy is a parent row and its instruments are
    child rows. Each instrument counts as a separate deployment (see the limits section).
  - TP and SL are the absolute target and stop-loss prices derived from the strategy's % / points exit.
  - "Status / Cycle" combines a status and a cycle counter (for example, the number of entry/exit cycles
    done out of the maximum allowed).
  - "Log" is a per-row icon or link that opens the order/notification log.
- **Empty state:** an illustration, the bold line "No Deployed Strategies found", the grey line "You can
  deploy your strategies from the strategy page", and a primary button **"Go to Discover Strategies"**
  that links to `/discover-strategies`.
- **No** Search, Refresh, filter or sort controls are visible in the empty state.
- **Not observable (no deployments):** status values, row action menu, the per-deployment details page,
  the log dialog, and the format of the cycle counter.

### Behaviour documented in the official help FAQ (help.streak.tech)
- **Limit:** up to **100 concurrent deployments**. Each stock in a strategy counts as one deployment, so a
  50-stock strategy uses 50.
- **Signals use candle close only.** Conditions are evaluated on the OHLC of a *completed* candle, never on
  ticks or LTP. Example: a 5-minute crossover at 09:23 alerts after the 09:25 close. Typical alert latency
  is about 8–14 s after the candle closes.
- **TP/SL handling:** when the target or stop-loss level is reached, Streak sends an *alert*. The user
  clicks the alert to close the position. Streak does not exit automatically.
- **Combined SL / TP** (an options-strategy feature): one exit notification fires when the strategy-level
  ₹ profit target or ₹ loss limit is hit. The actual fill can overshoot the level.
- **Partially filled orders:** TP and SL are **not** monitored when the entry order was only partly filled.
- **MIS timing:** MIS deployments expire automatically at **15:21**. No MIS triggers occur after **15:20**,
  because the broker squares off MIS positions shortly after. Users who want triggers after 15:20 should
  use CNC.
- **After market close:** triggers can still be acted on in the post-market session (15:40–16:00).
  AMO orders are **not** supported.
- **Stale P&L:** if a position was closed directly at the broker, or deployed without SL/TP, the Live
  Strategies row can keep showing running P&L.
- **Backtest vs live mismatch:** both use the same trigger candles, but live fills differ. Backtest and
  trade details for the current day refresh after 21:00.
- **Chart trade markers:** EN = entry candle, EX = exit candle, SO = MIS square-off. In backtests, MIS
  positions are squared off at the open of the 15:15 candle.

### Open questions
- What are the exact status values (e.g. Waiting / Entered / Exited / Stopped / Expired)? What does a
  "Cycle" mean, and what is its maximum?
- What does the row action menu contain (Stop, Re-execute, View strategy, Delete)? Is there a details page
  per deployment, and is Log a modal or a page?
- Are deployments grouped by strategy with expandable instrument rows?

---

## 2. Deploy flow ("Notification Based Trading" button)

![Deploy step 1, Copy Strategy](../screenshots/05-live-orders-account/09-deploy-step1-copy-strategy.jpg)

**Feature description:** Starts a live, notification-based deployment of a strategy. For a prebuilt
(Discover) strategy, the user first has to save a personal copy (step 1). After that come the deployment
parameters (step 2, not observed).

- **Entry points:** the "Notification Based Trading" button (bell icon, full-width primary blue) in the
  Discover Strategies detail footer, on the full backtest page `/backtest/<slug>`, and in the read-only
  strategy view `/strategy/<slug>`.
- **Step 1 dialog, title "Copy Strategy".** The modal is centred over a dim backdrop. A two-dot step
  indicator sits bottom-left (step 1 of 2) and a **Next** button sits bottom-right. There is no × and no
  Cancel button. **Escape does not close it; clicking the backdrop does.**
  | Control | Type | Default / options | Constraints |
  |---|---|---|---|
  | Name | text input, placeholder "Enter a name to save" | empty, auto-focused | required. Pressing Next while empty turns the border red, shows a ⚠ icon, and hovering the icon shows the tooltip "Please enter a name…". |
  | Category | two pill toggles (single select) with icons | **Equity Intraday** (sun/clock icon, beige) / **Equity Swing** (zig-zag icon, purple). The prebuilt strategy's own category is pre-selected (Equity Swing here). | exactly one |
  | Tags | text input "Enter a tag" + counter "3/5" + hidden "Add" button; tags appear as chips with an orange × | pre-filled from the prebuilt (Breakout, Volatility, Range Break) | max **5** tags |
  | Description | resizable textarea, placeholder "Enter a description to save" | pre-filled with the prebuilt's description | — |
- **Step 2 was NOT observed.** It is not present in the DOM until Next is pressed, and pressing Next with
  a valid name may save a copy to the account, which the safety rules forbid. In the clean-room app,
  infer step 2 from what the Live Strategies columns show and from the FAQ: quantity per instrument,
  product (MIS / CNC), the TP/SL already defined in the strategy, a maximum-cycles or validity setting
  (the "Cycle" column), and a confirm (**Deploy**) button.
- The account has no saved strategies, so the "deploy your own strategy" path could not be seen either.

![Deploy step 1, name validation](../screenshots/05-live-orders-account/10-deploy-step1-copy-strategy-validation.jpg)

### My Strategies — `/my-strategies` (linked to deploy)

![My Strategies, empty state](../screenshots/05-live-orders-account/11-my-strategies-empty.jpg)

**Feature description:** Library of the user's own saved strategies. This is where a copy lands after
step 1 and where the user's own strategies are deployed from.
- Header: ← "My Strategies", "View and manage all the strategies you have created across equity and
  options.", and buttons **Create Strategy** (outline) and **Discover Strategies** (filled).
- Left list pane: tabs **All | Equity Swing | Equity Intraday | Option Buying | Option Selling**, the search
  box "Search for Strategies", and a filter icon. Right pane: details (blank when empty).
- Empty state: "No strategies found" + **Create Strategy** button.

### Open questions
- What fields does step 2 have, what are their defaults, and is there an expiry or validity date?
- Does deploying from your own strategy skip the Copy step?
- Is there any confirmation or risk disclaimer before going live?

---

## 3. Order Book — `/order-book`

![Order Book, empty state](../screenshots/05-live-orders-account/20-order-book-empty.jpg)

**Feature description:** Real-time list of every order placed through the linked broker account
(placed, pending, executed and cancelled). Use it to check fills and to see why a broker rejected an order.

- Header: ← back, H1 "Orderbook", description "Track all placed, pending, executed, and cancelled orders
  in real time."; right side has a **refresh** icon button (blue circular arrows) and a **Search** box.
- Section heading **"Executed Orders (0)"**, where the number is the row count. More sections (open or
  pending orders) probably appear when there are orders; they are hidden while empty (unverified).
- Columns: Time · Type (Buy/Sell) · Instrument · Product (MIS/CNC/NRML) · Quantity · Avg Price · Status.
- Empty state: illustration + "No orders found".
- From the FAQ: a rejected order shows an **"Order Rejected" icon**, and clicking it reveals the broker's
  rejection reason.
- The Scalper-mode docked panel has tabs **Positions / Open Orders / Order Book**, a **Total P&L** field,
  and a per-row ⋮ menu with Buy, Sell, Info, Cancel Order, Modify Order, Add More and Exit (covered by
  the Scalper area). The standalone `/order-book` page showed none of these while empty.

## 4. Positions — `/positions`

![Positions, empty state](../screenshots/05-live-orders-account/21-positions-empty.jpg)

**Feature description:** Shows today's open and closed positions at the broker, with live P&L.
- Header: ← "Positions", "View your open and closed trading positions along with real-time P&L.";
  refresh icon + Search box.
- Columns: Product · Symbol · Qty · Avg. · LTP · P&L · Net Change.
- Empty state: "No positions found". The top-bar "Positions" link is highlighted blue when active.

## 5. Holdings — `/holdings` (from the profile menu)

![Holdings, empty state](../screenshots/05-live-orders-account/22-holdings-empty.jpg)

**Feature description:** Long-term equity holdings synced from the linked broker, with invested amount,
current value and P&L.
- Header: ← "Holdings", "Check your long-term equity holdings synced from your linked Zerodha account.";
  Search box (no refresh button).
- Columns: Symbol · Qty · Avg. · LTP · Invested · Current Value · P&L · Net Change · Day Change.
- Empty state reuses the wrong copy: **"No positions found"** (a UX bug, since it should say holdings).
- The dashboard has a Holdings summary card (Current Value / Investment / P&L).

### Open questions (orders/positions/holdings)
- Can the table columns be sorted? What does Search match on (symbol only)?
- Do the refresh buttons poll, or do these views stream over a websocket?
- Does Positions have Open/Closed sub-tabs and a Total P&L footer when it has data?

---

## 6. Notifications — `/notifications` (the bell)

![Notifications, empty state](../screenshots/05-live-orders-account/30-notifications-empty.jpg)

**Feature description:** History of every alert generated by deployed strategies and live scanners. This
is where the user acts on entry, exit, SL and TP triggers in notification-based trading.

- The top-bar bell is a **link to a full page**, not a dropdown panel. No unread-count badge was seen.
- Header: ← "Notifications", "View all alerts and updates generated by strategies and scanners in one place."
- Filter tabs (segmented): **All** (default, filled blue) | **Notification Based** | **Scanner**; a
  Search box sits right-aligned on the same row.
- Columns: Trade Type · Instrument · Strategy/Scanner · Time · Status.
- Empty state: "No notifications found".
- Implied types: strategy entry/exit triggers (Trade Type = Buy/Sell, Status = e.g. pending / placed /
  ignored / expired) and scanner alerts. The FAQ says the user clicks a trigger alert to place or close
  the trade, so a row (or a toast) probably opens an order-confirmation dialog.
- Push delivery is controlled by the "Push Notifications" toggle on the Profile page.

### Open questions
- What are the Status values, and do triggers expire after N minutes?
- Is there a toast or pop-up when a trigger arrives in-session? (Likely, since this is the core of
  "notification based trading".)

---

## 7. Profile menu (avatar, top-right)

![Profile menu, name and ID cropped out](../screenshots/05-live-orders-account/40-profile-menu-cropped.png)

**Feature description:** Account dropdown with links to the profile and holdings pages, help, a theme
switch, and the app version.

A dropdown card anchored under the avatar, from top to bottom:
1. User name (bold) and broker client ID (grey): [redacted].
2. Divider.
3. **Profile** (person icon) → `/profile`.
4. **Holdings** (calendar-grid icon) → `/holdings`.
5. **Help** (paper-plane icon) → external `https://help.streak.tech`.
6. Divider.
7. **Dark Theme** label with a toggle switch (off by default; light theme is the default).
8. Divider.
9. **Version** "V1.0.0", right-aligned.

The menu has **no** Logout (that is on the Profile page), plans or billing, referral, broker connection,
settings, keyboard-shortcut or logs items. While the menu was open, the top-bar market-status dot turned
amber and its "Live" label disappeared (possibly a coincidence; per the FAQ, "Live" means connected to
the servers and "Offline" means disconnected).

## 8. My Profile — `/profile`

![Profile settings toggles (personal fields cropped out)](../screenshots/05-live-orders-account/41-profile-settings-toggles-cropped.png)

**Feature description:** Read-only account details from the linked broker, plus the app's only two
user preferences and Logout.

- Header: ← "My Profile", "View your account details and personal information as linked to your Zerodha
  account."
- Identity card: avatar placeholder, name, client ID with a broker logo chip: [redacted].
- Form rows (label left, input right): **Name**, **Email**, **Broker ID**, **Broker** (value "zerodha").
  All four inputs are **disabled** (not editable).
- Divider, then settings:
  - Toggle **"Remember F&O order quantity"** with the help text "Save quantity for orders (NFO-OPT,
    NFO-FUT, BFO-OPT, BFO-FUT)". Default **off**.
  - Toggle **"Push Notifications"** with the help text "Get real-time alerts for trades, scanners, and
    deployments". Default **off** (browser push).
- **Logout** button (orange outline, logout icon).
- No Save button: the toggles probably apply instantly (not tested).

## 9. Broker connection, plans/pricing, referral, settings
- **There is no broker-connection page.** Streak is Zerodha-only: login is through Kite credentials, no
  Kite Connect subscription is needed, and a user can have only one active session. Broker and ID are
  shown read-only on the Profile page.
- **There are no plans or pricing.** `/pricing`, `/plans`, `/subscription`, `/settings` and `/referral` all
  return the in-app 404 page. The FAQ confirms that **Streak is free for Zerodha users**; only normal
  brokerage and statutory charges apply. The page title is "Streak | Free Systematic Trading Platform".
- The help-centre logo and Login button link to `https://beta.streak.tech/`.

![In-app 404 page](../screenshots/05-live-orders-account/50-404-page.jpg)

The 404 page shows "404 | This page could not be found." in near-white text on a light grey background.
It is almost invisible: a contrast bug, possibly dark-theme text on the light theme.

### Limits table (no paid tiers; one free tier)
| Limit | Value | Source |
|---|---|---|
| Price | Free (Zerodha account required) | help FAQ "Are there any charges…" |
| Concurrent live deployments | 100 (each instrument counts as 1) | FAQ "How many positions I can deploy" |
| Live scanners at a time | 25 | FAQ "How many scanners can I take live" |
| Instruments per strategy | 50 | Add Instruments dialog "Symbol (n / 50)" |
| Tags per strategy | 5 | Copy Strategy dialog "3/5" |
| Order quantity | ≤ exchange freeze limit per order | FAQ |
| MIS deployment expiry | 15:21 daily; no MIS triggers after 15:20 | FAQ |
| Backtests/day | not stated anywhere observed | — |
| Sessions | 1 active login | FAQ |
| Automation / paper trading | not supported (notification-only) | FAQ |

## 10. Help centre — `https://help.streak.tech`

![Help centre home](../screenshots/05-live-orders-account/51-help-center-home.jpg)

**Feature description:** Public knowledge base. A hero with search ("Search for help topics & Articles…"),
six category cards (Home Page, Scalper Mode, Strategies, Scanners, Frequently Asked Questions, Advance
Create), and "Browse Help Topics" lists. Article pages have a left category tree and an "Updated on" date,
and end with "Still need help? Write to us at support@streak.tech". About 90 FAQs exist. There is no
in-app keyboard-shortcut list, support chat or logs page. A mobile app exists (Android and iOS).

---

## Cross-cutting UX observations (for the desktop rebuild)
- One consistent list-page template: back arrow, title, one-line description, right-aligned Refresh and
  Search, a flat table, and an illustrated empty state with one CTA. Reuse it for Live Strategies,
  Orders, Positions, Holdings and Notifications.
- The status of live connectivity ("Live" / "Offline") lives in the top bar. A desktop app should show
  both the market-data feed and the broker session state there.
- The notification-centric model (alerts that the user confirms) is central. The rebuild needs a
  first-class alert inbox, plus toasts or OS notifications with a one-click "place order" confirm.
- Bugs and rough edges to avoid: Holdings empty-state copy says "positions"; the 404 page is unreadable;
  the Copy Strategy modal has no close button and ignores Escape; the validation message is only in a
  hover tooltip.

## Gaps (not observed, by design or for lack of data)
- Deploy step 2 fields (blocked by the no-save rule).
- Any populated state: deployment rows and statuses, order and position rows, the rejected-order icon,
  notification rows, the log viewer, the action menus.
- What happens when a toggle is switched (not tested), dark-theme appearance.
