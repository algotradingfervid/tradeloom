# Functional Requirements — BT, DEP, SCAN, SCALP, ORD

Status: DRAFT · Date: 2026-10-11 · Parent: `docs/requirements/REQUIREMENTS.md` (§1–5: scope, C-01..C-12, NFR-01..14)
Input catalogue: `docs/research/FEATURES.md` (modules BT, DEP, SCAN, SCALP, ORD; appendices F, G, H, I, K, L, M, N; §3, §6)
Platform facts: `docs/research/areas/07-platform-kite-wails.md` §A2–A12, §B, §C.

## 0. Conventions for this file

- "The system" = the local desktop application (Go + Wails v2 backend, React frontend) using the user's own Kite
  Connect key through `github.com/zerodha/gokiteconnect/v4`. gokiteconnect method names are given where an FR depends on
  Kite (e.g. `GetHistoricalData`, `PlaceOrder`, `GetBasketMargins`, `GetOrderCharges`, `PlaceGTT`, ticker `OnOrderUpdate`).
- Every FR traces to ≥1 feature ID. "**Enhancement (beyond Streak)**" marks behaviour Streak does not have; such FRs are
  separate from the parity FRs so phasing can drop or defer them independently.
- "Deviation from Streak" states what differs and why (usually C-02 daily login, C-03 no server-side runtime,
  C-04 static IP, C-05 rate limits, NFR-13 no artificial caps, or a Streak bug from FEATURES §3.2 not replicated).
- Where FEATURES §6 lists a conflict, the "chosen interpretation" of that table is used and the conflict number is cited.
- Clean-room: no Streak names, copy, logos or illustrations are reused. UI strings quoted below are functional
  placeholders; final copy is written fresh in the design spec. Trigger codes (EN/EX/SL/TP/TSL/SO/EXP) and indicator
  names are generic industry terms.
- Time is exchange time (IST). Default NSE session 09:15–15:30. "Base TF" = strategy/scanner base timeframe.
- Monetary examples use ₹ and are computed at full precision, displayed to 2 dp; acceptance tolerance ±0.01 unless stated.
- Numbering: `FR-<MODULE>-NNN`. The traceability table at the end maps every feature ID to its FRs.

### 0.1 Engine glossary (used by BT and DEP)

| Term | Meaning |
|---|---|
| Signal candle | The completed base-TF candle on whose close a condition evaluated to true |
| Fill candle | The candle immediately after the signal candle; fills occur at its open |
| Position | At most one open position per (strategy, instrument) at a time; direction = strategy side (Buy → long, Sell → short) |
| Capital C | Backtest "Initial Capital", applied per instrument |
| Realised P&L R | Sum of gross P&L of all closed round trips of that instrument before the current trade |
| Path | Assumed intra-candle price sequence: long O→L→H→C, short O→H→L→C |

---

## FR module BT — Backtesting

### FR-BT-001 Backtest parameters dialog
- **Source features:** BT-001
- **Requirement:** The system shall present, from the builder "Execute" action (mode Backtest), from My Strategies
  "Run Backtest" and from the results page "Run Backtest", a backtest-parameters dialog with: Initial Capital (number,
  default 25000), Start Date, End Date (default today), Entry Start Time and Entry End Time (hh:mm with AM/PM, defaults
  09:15 AM and 03:30 PM), Holding Type (Intraday MIS / Longterm CNC; futures additionally NRML — see FR-BT-004), Strategy
  Cycle ({–, 1…5}, disabled when CNC), and Cancel / Run actions. There is no quantity field (quantity is derived,
  FR-BT-006). The dialog shall close on Escape and on Cancel (Streak bug §3.2-8 not replicated).
- **Acceptance criteria:**
  1. Given a saved equity strategy on a 15-min base TF, when the dialog opens, then Initial Capital = 25000, End Date =
     today, Start Date = today − 90 calendar days (FR-BT-008 default), times = 09:15 AM / 03:30 PM, Holding = MIS, Cycle = "–".
  2. Given Holding Type = CNC, then the Strategy Cycle control is disabled and its value is ignored by the engine.
  3. Given Start Date > End Date, or Entry Start Time ≥ Entry End Time, or Initial Capital ≤ 0, when Run is pressed, then
     the run is blocked and an inline message names the offending field.
  4. Given the dialog is open, when Escape is pressed, then it closes without running or saving.
- **Data/Kite dependency:** Local strategy store (SQLite); local candle store.
- **Deviation from Streak:** Escape closes the dialog; inline validation messages (Streak behaviour unobserved).
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
- **Deviation from Streak:** "–" = unlimited is an interpretation (Streak does not document it; see Open Questions OQ-02).
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
- **Deviation from Streak:** none intended (exact Streak boundary semantics unverified — OQ-03).
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
- **Deviation from Streak:** Per-instrument is the documented legacy behaviour; current Streak behaviour unconfirmed (OQ-01).
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
- **Deviation from Streak:** Lot rounding for futures is an interpretation (Streak undocumented; OQ-04).
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
- **Data/Kite dependency:** Local candle store; Kite historical (`GetHistoricalData`; minute data from ~2015).
- **Deviation from Streak:** **Enhancement (beyond Streak):** lookback limited only by local data (NFR-13); Streak caps at
  30 d / 90 d / 365 d / 3 y.
- **Priority hint:** Core

### FR-BT-009 Data backfill for backtests
- **Source features:** BT-006, BT-007
- **Requirement:** Before a run the system shall backfill missing candles through the shared historical-data job:
  ≤ 3 requests/s, per-interval request windows (minute 60 d; 3/5/10-min 100 d; 15/30-min 200 d; 60-min 400 d; day 2000 d,
  adapting if Kite returns a window error), resumable after interruption, with a progress UI (instruments done / total,
  ETA). Higher TFs not offered by Kite (week, month, and any non-native resample) are built locally from finer candles.
- **Acceptance criteria:**
  1. Given 50 instruments missing 90 days of 15-min data, then exactly 50 historical requests are issued (one 200-day window
     each) at ≤ 3/s and progress reaches 50/50.
  2. Given the app is closed mid-backfill, when it restarts, then the job resumes from the last completed (instrument,
     window) without refetching completed windows.
  3. Given an HTTP 429, then the job backs off and retries without exceeding 3 req/s.
- **Data/Kite dependency:** `GetHistoricalData(token, interval, from, to, continuous, oi)`; local candle store.
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
  2. Given 50 instruments × 90 days × 15-min with cached data on a 4-core machine, then all results are shown in < 10 s
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
     on (brokerage + transaction + SEBI), and a long MIS round trip buy 9 @ 2651.80, sell 9 @ 2600.00, then buy-leg charges =
     10.31 (brokerage 7.16, txn 0.71, SEBI 0.02, stamp 1.00, GST 1.42), sell-leg charges = 15.13 (brokerage 7.02, STT 6.00,
     txn 0.69, SEBI 0.02, GST 1.39), total 25.44; gross P&L −466.20; net P&L −491.64.
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
  max(E₀…Eᵢ), shown as a positive percentage with the ₹ amount (Peak − Trough) alongside; 0 when no drawdown. Trade P&L: long =
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
- **Deviation from Streak:** Exact Streak averaging base unverified (OQ-05); definition made explicit.
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
- **Deviation from Streak:** none.
- **Priority hint:** Important

### FR-BT-020 Trade log table
- **Source features:** BT-016
- **Requirement:** The trade log shall list every fill with columns Date & Time, Symbol, P&L, Price, Buy/Sell, Qty (signed:
  + buy, − sell), Trigger code, and (Enhancement) Fill Time and Reason/diagnostic. Date & Time shall be the signal candle's
  open time for EN/EX (fill at the next candle's open), and the open time of the candle in which the event occurred for
  SL/TP/TSL/SO/EXP. Exit rows carry the realised P&L of that round trip; entry rows carry the running cumulative P&L. The table
  shall paginate (default 50 rows/page, configurable) with Prev / page numbers / Next.
- **Acceptance criteria:**
  1. Given a 5-min strategy whose condition becomes true on the 09:20–09:25 candle and the 09:25 candle opens at 1002.50, then
     the EN row shows time 09:20, price 1002.50, Buy, +qty, and (Enhancement column) fill time 09:25.
  2. Given a short round trip, then the entry row shows Sell with negative qty and the exit row shows Buy with positive qty.
  3. Given 230 rows and 50 per page, then 5 pages exist.
- **Data/Kite dependency:** none.
- **Deviation from Streak:** Fill-time and reason columns added (Enhancement) to remove the ambiguity of signal-time display.
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
- **Data/Kite dependency:** none.
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
- **Deviation from Streak:** Level vs transition triggering for exits is unverified in Streak (OQ-06).
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
  is an **Enhancement (beyond Streak)** for users who want stricter realism (OQ-07).
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
- **Deviation from Streak:** none intended; exact Streak trail start reference unverified (OQ-08).
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

### FR-BT-031 Engine: MIS square-off per instrument class
- **Source features:** BT-020
- **Requirement:** Open MIS positions shall be squared off (code SO) at the open of the base-TF candle that contains the
  square-off time T (candle start ≤ T < candle end), with T by instrument class (FEATURES §6-2 chosen interpretation, from a
  user-editable table): F&O stocks under the closing call-auction list: 15:10; indices: 15:10; non-F&O equities: 15:25;
  futures: 15:25; fallback (class unknown): 15:15. With a 1-hour base TF on an F&O stock this yields the 14:15 candle. New MIS
  entries whose fill candle starts at or after the SO candle are not taken. MIS with a 1-day base TF is rejected at run time
  ("intraday product requires an intraday timeframe"). Instrument class is derived from the instruments dump (equity with an
  NFO futures underlying → F&O stock) and the user-importable call-auction list.
- **Acceptance criteria:**
  1. Given a non-F&O stock on 5-min, then SO occurs at the 15:25 candle's open.
  2. Given an F&O stock in the call-auction list on 5-min, then SO occurs at the 15:10 candle's open.
  3. Given an F&O stock on 1-hour candles (09:15, 10:15 … 14:15, 15:15), then SO occurs at the 14:15 candle's open.
  4. Given an F&O stock on 15-min, then SO occurs at the 15:00 candle's open (contains 15:10).
  5. Given an MIS strategy on a 1-day base TF, then the run is refused with the message above.
- **Data/Kite dependency:** Instruments dump (`GetInstruments`, NFO underlyings); user-importable call-auction list (C-09).
- **Deviation from Streak:** The "containing candle" rule is an interpretation that reproduces Streak's documented 14:15
  hourly case; Streak's exact rule for 15-min F&O stocks is unverified (OQ-09).
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
- **Deviation from Streak:** Continuous-futures mode is an Enhancement; intraday history of expired contracts is UNVERIFIED (07 §A4).
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

### FR-BT-034 Engine: multi-timeframe evaluation
- **Source features:** BT-023
- **Requirement:** The strategy shall be evaluated at every base-TF candle close. A condition row on a higher TF uses that TF's
  last fully closed candle at the evaluation time. If a row's TF is not an integer multiple of the base TF, evaluation occurs
  only at times that are closes of both (their LCM, anchored at session open). Intraday higher-TF candles are resampled from
  finer local candles anchored at 09:15; week candles (Mon–Fri) and month candles are built locally and are complete only when
  the next week/month begins; week and month may be used only as MTF rows, never as base TF. Indicator warm-up on MTF rows uses
  the same unlimited local history (FR-BT-008).
- **Acceptance criteria:**
  1. Given base 5-min and a 15-min row, at the 10:00 evaluation (5-min candle 09:55–10:00), then the 15-min value used is the
     09:45–10:00 candle; at 09:55 it is the 09:30–09:45 candle.
  2. Given base 3-min and a 5-min row, then evaluations occur only at 09:30, 09:45, 10:00 … (every 15 min).
  3. Given a weekly row evaluated on Wednesday, then the previous week's candle is used; on the next Monday's first evaluation
     the just-finished week is used.
  4. Given a 1-hour row, then hourly candles are 09:15, 10:15, … 15:15 (last candle 15:15–15:30).
- **Data/Kite dependency:** Local resampling; LCM scheduler.
- **Deviation from Streak:** None in rules; warm-up not limited by Streak's lookback (Enhancement via FR-BT-008).
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
- **Requirement:** Backtests shall support NSE and BSE equities (cash) and NSE/BSE index and stock futures, long and short
  (intraday short-selling for MIS; CNC short is refused at run time). Candlestick charts only. **Enhancement (beyond Streak):**
  (a) any BSE stock with data, not only a BSE 1000 subset; (b) options backtesting for strategies built in the options builder,
  limited to contracts whose historical candles are locally available or fetchable (live contracts; expired-contract intraday
  data is UNVERIFIED), with the same engine rules applied per leg and combined SL/TP on summed leg P&L.
- **Acceptance criteria:**
  1. Given a Sell strategy with CNC, then the run is refused: "Short delivery positions are not allowed".
  2. Given an options strategy whose contracts lack history, then the run reports which legs lack data instead of silently
     producing no trades.
- **Data/Kite dependency:** `GetHistoricalData` for NSE/BSE/NFO/BFO; instruments dump.
- **Deviation from Streak:** Streak supports equities and futures only; options backtesting and full BSE are enhancements.
- **Priority hint:** Core (equities/futures) · Nice-to-have (options)

### FR-BT-039 MCX, currency, AMO, MTF exclusions
- **Source features:** BT-026
- **Requirement:** MCX, currency and MTF-product backtests are out of scope for parity; the instrument picker for backtests
  shall exclude those segments and say why.
- **Acceptance criteria:**
  1. Given an MCX instrument, then it cannot be added to a strategy that will be backtested and a tooltip explains "segment not supported".
- **Data/Kite dependency:** Instruments dump segment field.
- **Deviation from Streak:** none.
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
     (Streak bug §3.2-7 not replicated).
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

### FR-BT-042 Backtest page onboarding tour
- **Source features:** BT-029
- **Requirement:** On first visit to a results view, the system shall offer a two-step dismissible tour (step 1: per-instrument
  table and combined results; step 2: metrics and trade log) with Next/Back/Done/Close, keyboard operable; seen state persisted
  locally; re-launchable from Help.
- **Acceptance criteria:**
  1. Given the tour was completed once, then it does not reappear on later visits unless relaunched.
- **Data/Kite dependency:** Local settings.
- **Deviation from Streak:** Tour copy written fresh (clean-room).
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
  checksum of candles used, fee-table version, slippage, SL/TP evaluation mode, square-off table version, adjustment setting,
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

---
## FR module DEP — Deployment and notification-based trading

### FR-DEP-001 Deploy dialog (own strategy)
- **Source features:** DEP-001
- **Requirement:** From the builder "Execute" (mode Deploy), My Strategies and the results view, the system shall open a deploy
  dialog with: instrument list with master checkbox and count ("n / total"), per-instrument checkbox and Qty (default 1; lots for
  F&O); Capital (required; used for the capital check in FR-DEP-019 and for P&L % display); Strategy Cycle {–, 1…5} (disabled for
  CNC); Order Type (Market default; Limit as Enhancement with offset from LTP); Holding Type MIS / CNC (NRML for futures/options);
  Execution Mode (FR-DEP-003); Validity end date (default today + 29 calendar days); and a mandatory risk acknowledgement
  checkbox; Execute is disabled until the acknowledgement is ticked and all fields are valid. Each selected instrument becomes one
  deployment-instrument (DI) under one deployment.
- **Acceptance criteria:**
  1. Given 15 instruments selected of 50, then the header shows "15 / 50" and Execute creates 1 deployment with 15 DIs in state
     ARMED (FR-DEP-004).
  2. Given the acknowledgement is unticked, then Execute is disabled.
  3. Given Qty 0 or a non-lot-multiple for an F&O instrument, then Execute is disabled and the row is highlighted.
  4. Given today = 11 Oct 2026, then default validity = 09 Nov 2026 (T+29).
  5. Given Escape or Cancel, then the dialog closes and nothing is deployed.
- **Data/Kite dependency:** Local deployment store; instruments dump (lot size).
- **Deviation from Streak:** Mode selector, validity date, Limit option and Escape-to-close added; Streak's Market-only
  order type is the default.
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
- **Deviation from Streak:** Cancel/×/Escape and inline validation added (Streak bug §3.2-16 not replicated).
- **Priority hint:** Important

### FR-DEP-003 Execution modes
- **Source features:** DEP-011, DEP-001
- **Requirement:** Each deployment shall run in exactly one mode:
  (1) **Notify & confirm (default; parity with Streak):** signals raise alerts; an order is placed only when the user confirms it.
  (2) **Signal-only:** alerts only, no order capability; tracks a notional position (as if every entry were taken) so exit alerts
  are meaningful; available without static IP or trading enabled.
  (3) **Paper — Enhancement (beyond Streak):** simulated orders and positions (FR-DEP-017).
  (4) **Armed auto — Enhancement (beyond Streak):** orders placed automatically through the order safety pipeline, only under
  FR-DEP-018 gating.
  Mode changes on a live deployment are allowed only when all its DIs are flat.
- **Acceptance criteria:**
  1. Given trading is not enabled (FR-ORD-013), then modes 1 and 4 are disabled in the dialog with the reason shown; modes 2 and
     3 remain available.
  2. Given a DI is IN_POSITION, then the mode selector is read-only for that deployment.
- **Data/Kite dependency:** none (mode 1/4 depend on ORD pipeline).
- **Deviation from Streak:** Streak offers notification-based trading only; signal-only, paper and armed-auto are additions
  (paper and auto are explicitly absent in Streak, DEP-011).
- **Priority hint:** Core (1, 2) · Important (3) · Nice-to-have (4)

### FR-DEP-004 Deployment state machine
- **Source features:** DEP-003, DEP-004, DEP-005, DEP-006, DEP-007
- **Requirement:** Each deployment-instrument (DI) shall follow this state machine; the deployment's status is derived
  (ACTIVE if any DI non-terminal, else STOPPED/EXPIRED/COMPLETED).

  | State | Meaning |
  |---|---|
  | ARMED | Flat, evaluating entry conditions |
  | ENTRY_PENDING | Entry signal raised, awaiting user confirmation (mode 1) |
  | ENTRY_ORDER_OPEN | Entry order submitted, awaiting fill/rejection |
  | IN_POSITION | Position open (real, paper or notional); exits monitored |
  | EXIT_PENDING | Exit signal raised, awaiting confirmation (mode 1) |
  | EXIT_ORDER_OPEN | Exit order submitted, awaiting fill |
  | CYCLE_DONE | Flat; cycle limit reached for the day (MIS) or deployment (CNC) |
  | DAY_DONE | MIS after 15:21 for the day; resumes next trading day if re-arm is on |
  | SUSPENDED | Overlay: app offline, session expired, feed down or kill switch; stores `resume_state` |
  | ATTENTION | Needs user decision: rejection, partial fill, position mismatch, unknown order outcome |
  | STOPPED / EXPIRED | Terminal (user stop / validity reached or MIS daily expiry without re-arm) |

  | From | Event / guard | To | Action |
  |---|---|---|---|
  | ARMED | entry transition at candle close, within cycle/time rules, mode 1 | ENTRY_PENDING | Alert (FR-DEP-006) |
  | ARMED | same, mode 4 armed | ENTRY_ORDER_OPEN | Submit via ORD pipeline |
  | ARMED | same, mode 2 / 3 | IN_POSITION | Notional / simulated fill |
  | ENTRY_PENDING | user confirms, pipeline accepts | ENTRY_ORDER_OPEN | Order submitted |
  | ENTRY_PENDING | user ignores, or signal expires (FR-DEP-006) | ARMED | Notification status Ignored/Expired; cycle not consumed |
  | ENTRY_ORDER_OPEN | fully filled | IN_POSITION | Compute SL/TP/TSL from avg fill price; optional GTT (FR-DEP-009); cycle +1 |
  | ENTRY_ORDER_OPEN | rejected / cancelled with 0 fill | ARMED | Alert with reason |
  | ENTRY_ORDER_OPEN | partially filled then closed | ATTENTION | Track filled qty; user chooses keep/exit |
  | ENTRY_ORDER_OPEN | submit outcome unknown (timeout) | ATTENTION | Resolve by order lookup by tag (FR-ORD-008) |
  | IN_POSITION | EX/SL/TP/TSL/SO trigger, mode 1 | EXIT_PENDING | Exit alert |
  | IN_POSITION | same, mode 4 / 3 / 2 | EXIT_ORDER_OPEN / ARMED or CYCLE_DONE | Auto / simulated / notional exit |
  | IN_POSITION | position found closed at broker (reconcile) | ARMED or CYCLE_DONE | Event "closed externally"; cancel GTT |
  | IN_POSITION | GTT triggered and filled | ARMED or CYCLE_DONE | Record exit from GTT |
  | EXIT_PENDING | user confirms | EXIT_ORDER_OPEN | Order submitted |
  | EXIT_PENDING | user dismisses | IN_POSITION | Alert stays visible as "exit not taken"; monitoring continues |
  | EXIT_ORDER_OPEN | filled | ARMED, or CYCLE_DONE if cycles exhausted | Cancel GTT; P&L recorded |
  | EXIT_ORDER_OPEN | rejected | ATTENTION | Alert with reason |
  | any non-terminal | 15:20 (MIS) | (no new entries) | Entry evaluation disabled |
  | ARMED / CYCLE_DONE / ENTRY_PENDING (MIS) | 15:21 | DAY_DONE or EXPIRED | Pending entry alerts expire |
  | DAY_DONE / CYCLE_DONE (MIS) | next trading day first candle, re-arm on, validity not reached | ARMED | Cycle counter reset |
  | any non-terminal | validity date passed and flat | EXPIRED | — |
  | IN_POSITION | validity date passed | IN_POSITION (no new entries) | Exit alerts continue until flat, then EXPIRED |
  | any non-terminal | app offline / session expired / feed down / kill switch | SUSPENDED (resume_state saved) | — |
  | SUSPENDED | session valid + feed up + not killed | resume_state, after reconciliation (FR-DEP-010) | — |
  | ATTENTION | user resolves | ARMED / IN_POSITION / STOPPED | Per choice |
  | any | user Stop | STOPPED | Optional exit of open position (FR-DEP-013) |

- **Acceptance criteria:**
  1. Given a state-transition test table covering every row above, then each event moves the DI to the listed state and performs
     the listed action, and no other transition is possible (illegal events are logged and ignored).
  2. Given mode 1 and an entry alert ignored, then the DI returns to ARMED and the cycle counter is unchanged.
  3. Given MIS with cycle 1 and a completed round trip, then the DI is CYCLE_DONE until the next trading day.
- **Data/Kite dependency:** SQLite deployment store; ORD pipeline; ticker.
- **Deviation from Streak:** States are explicit and visible; SUSPENDED exists because the app may be closed (C-03).
- **Priority hint:** Core

### FR-DEP-005 Persistence and restart
- **Source features:** DEP-004, DEP-003
- **Requirement:** Every DI field needed to resume (state, resume_state, cycle count and day, position qty and average price,
  SL/TP levels, trail reference, last evaluated candle time, pending signal, linked order IDs and tags, GTT IDs, mode, validity)
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
  "deployment", C-07), finalise each candle at its boundary plus a 1 s grace for late ticks, evaluate with the shared engine
  (FR-BT-023/024/034), and raise any signal within 2 s of candle close (NFR-08). An alert shall be delivered to the in-app
  notification inbox (FR-DEP-015), as an OS desktop notification (FR-DEP-016) and as an in-app toast with "Review order" and
  "Ignore" actions. An entry alert expires at the next base-TF candle close (configurable) and then becomes "Expired". Signals
  after 15:30 shall not be actionable as regular orders; the alert explains the 15:40–16:00 post-closing session and AMO are not
  offered for parity (AMO: OQ-14).
- **Acceptance criteria:**
  1. Given a 5-min crossover on the 09:20–09:25 candle, then the alert appears by 09:25:02 (measured from candle end).
  2. Given the alert is not acted on by 09:30 close, then its status is Expired and the DI is ARMED.
  3. Given the same fixture candles, then live signals equal backtest signals.
  4. Given the app is minimised, then an OS notification is shown; clicking it focuses the app on the order review.
- **Data/Kite dependency:** Ticker (`ticker.Subscribe`, `SetMode` quote), local candle builder, warm-up via `GetHistoricalData`.
- **Deviation from Streak:** Latency target < 2 s (Streak 8–14 s); toast added; channels are local (no web/mobile push).
- **Priority hint:** Core

### FR-DEP-007 Notify & confirm order flow
- **Source features:** DEP-005, DEP-009
- **Requirement:** Acting on an alert shall open an order review pre-filled with instrument, side, qty, product, order type
  (Market with market protection), the signal price (signal candle close), current LTP, estimated margin and charges, and a
  short condition summary. Confirming sends it through the order safety pipeline (FR-ORD-005); the result updates the
  notification status (Placed / Rejected / Failed) and the DI state.
- **Acceptance criteria:**
  1. Given an entry alert for 5 × RELIANCE MIS Buy, then the review shows those values and Place submits exactly one order tagged
     with the deployment's tag.
  2. Given margin is insufficient, then the pipeline blocks placement and the notification shows "Blocked: insufficient margin".
- **Data/Kite dependency:** `GetOrderMargins`, `GetOrderCharges`, `PlaceOrder`.
- **Deviation from Streak:** none in flow; preview of margin/charges added.
- **Priority hint:** Core

### FR-DEP-008 Live SL / TP / TSL / combined monitoring
- **Source features:** DEP-006
- **Requirement:** For IN_POSITION DIs the system shall compute SL/TP/TSL levels from the actual average fill price and filled
  quantity (FR-BT-027 formulas), monitor them on every tick (LTP), and raise an exit trigger (SL/TP/TSL) the first time a level is
  crossed; exit-condition (EX) triggers are evaluated on candle close; SO triggers at the instrument's square-off time
  (FR-BT-031 times). For options strategies, combined SL/TP on the summed leg P&L raises one exit trigger for all legs; on exit, short
  legs are closed before long hedge legs so a hedge is never removed while its short leg is open. Partially filled entries are
  monitored on the filled quantity (Enhancement).
- **Acceptance criteria:**
  1. Given a long filled at avg 1000.40 with SL 2 %, then SL level = 980.39 and an exit alert fires on the first tick ≤ 980.39.
  2. Given a partial fill of 3 of 5, then levels use qty 3 for ₹-based exits and the DI shows "partial (3/5)".
  3. Given a combined SL of ₹2,000 on a 2-leg strategy, then one exit trigger fires when summed MTM ≤ −2,000.
- **Data/Kite dependency:** Ticker LTP; `GetPositions` for average price reconciliation.
- **Deviation from Streak:** Partial fills monitored (Streak does not monitor them).
- **Priority hint:** Core

### FR-DEP-009 Kite GTT mirroring of SL/TP — Enhancement (beyond Streak)
- **Source features:** DEP-006, DEP-004
- **Requirement:** Per deployment (opt-in, requires trading enabled), on entering IN_POSITION the system shall place a Kite GTT:
  two-leg OCO with SL and TP when both exist, single-leg otherwise, with LIMIT orders priced at the trigger ± a configurable
  buffer (default 0.5 %) in the exit direction, product and qty matching the position. When the TSL moves, the GTT SL leg is
  modified no more often than every 60 s and only if the move ≥ 1 tick-buffer; when the position is exited by the app, the GTT is
  deleted; when the GTT triggers, the DI records the exit from order updates. GTT IDs are persisted. The UI shall show per DI
  whether exits are "protected while app is closed".
- **Acceptance criteria:**
  1. Given a long 10 @ 1000 with SL 980 and TP 1030, then one OCO GTT exists with triggers 980/1030 and sell LIMIT prices
     975.10/1024.85 (0.5 % buffer below each trigger, rounded to tick 0.05).
  2. Given the app exits the position, then the GTT is deleted within 5 s; failure to delete raises ATTENTION.
  3. Given the app is closed and the GTT triggers, then on next start the DI shows the GTT exit and is ARMED/CYCLE_DONE.
  4. Given an MIS position, then the GTT is deleted at the DI's square-off time.
- **Data/Kite dependency:** `PlaceGTT`, `ModifyGTT`, `DeleteGTT`, `GetGTTs`; static IP requirement for GTT placement UNVERIFIED (OQ-12).
- **Deviation from Streak:** Streak has no GTT usage; GTT LIMIT orders may not fill on gaps (documented in UI).
- **Priority hint:** Important

### FR-DEP-010 Startup / reconnect reconciliation and missed signals
- **Source features:** DEP-005, DEP-003, DEP-004
- **Requirement:** On app start, after daily login, and after a feed outage > 1 candle, for each non-terminal DI the system shall:
  (1) apply time rules (validity, MIS 15:21) first; (2) fetch candles from `last_evaluated_candle` to the latest closed candle via
  historical API and replay the engine over the gap; (3) record entry signals in the gap as "Missed" notifications that are
  informational only and never auto-executed; (4) if IN_POSITION and an exit trigger occurred in the gap, raise a high-priority
  "Missed exit" alert (auto-executed only in armed mode with the separate opt-in "auto-execute missed exits", default off);
  (5) reconcile positions (`GetPositions`), orders (`GetOrders` by tag) and GTTs (`GetGTTs`) and resolve mismatches to ATTENTION
  with a description; (6) then leave SUSPENDED to the resume state.
- **Acceptance criteria:**
  1. Given the app was closed 10:00–11:00 on a 5-min deployment and an entry transition occurred at 10:25, then a "Missed entry
     10:25" notification exists and no order was placed.
  2. Given an open long whose SL level was crossed at 10:40 while closed, then a "Missed exit (SL)" alert is shown on start.
  3. Given the broker shows the position closed (net qty 0), then the DI moves to ARMED with event "closed externally".
  4. Given an MIS DI and the app opened at 16:00, then it is DAY_DONE/EXPIRED and no missed entries are actionable.
- **Data/Kite dependency:** `GetHistoricalData`, `GetPositions`, `GetOrders`, `GetGTTs`.
- **Deviation from Streak:** Needed because the local app is not always running (C-03).
- **Priority hint:** Core

### FR-DEP-011 Live deployments page
- **Source features:** DEP-003
- **Requirement:** The system shall provide a Live Deployments view with mode tabs (All, Notify & confirm, Signal-only, Paper,
  Armed auto), a table of deployments (parent rows) with expandable instruments (child rows) and columns: Strategy / Instrument,
  Avg Price, LTP, Day Change, TP, SL (absolute levels), P&L, Net Change, Status / Cycle (e.g. "In position · 1/3"), GTT-protected
  indicator, Log; row actions (FR-DEP-013); search and status filter; an empty state with a call to action to the strategy
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
- **Requirement:** The system shall enforce: MIS — no new entry signals after 15:20, MIS daily expiry at 15:21 (DI → DAY_DONE if
  "re-arm daily" is on, else EXPIRED; default re-arm OFF for parity); validity end date (default T+29, editable, no maximum);
  no artificial cap on concurrent deployments (NFR-13) — the only cap is live-data capacity: a deployment that would push total
  subscriptions above 9,000 instruments (3 × 3,000) is refused with the reason; order quantity above the exchange freeze limit is
  blocked or autosliced per FR-ORD-005; one app instance per data directory (second instance refuses to start).
- **Acceptance criteria:**
  1. Given 150 DIs, then all are accepted (no 100 cap) if subscriptions ≤ 9,000.
  2. Given an MIS entry transition at 15:20 close, then no alert is raised.
  3. Given a second app launch, then it shows "already running" and exits.
- **Data/Kite dependency:** Ticker capacity (07 §A6); local scheduler.
- **Deviation from Streak:** No 100-deployment cap; editable validity; optional daily re-arm (Enhancement).
- **Priority hint:** Core

### FR-DEP-013 Stop, re-execute, edit, delete
- **Source features:** DEP-007
- **Requirement:** Row actions shall include: Stop (deployment or single DI; if IN_POSITION, ask "Stop and exit position" /
  "Stop and keep position (untracked)"); Re-execute (create a new deployment with the same parameters and a fresh validity from a
  STOPPED/EXPIRED one); Edit quantity/validity (only when flat); View strategy; View log; Delete (only terminal; logs retained
  in the audit store).
- **Acceptance criteria:**
  1. Given Stop and exit on an IN_POSITION DI in mode 1, then an exit order review opens; on confirmation the DI ends STOPPED.
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
  Ignored, Expired, Missed, Info) and actions (Review order for Pending, View log). An unread count badge shall appear on the
  navigation entry.
- **Acceptance criteria:**
  1. Given 3 strategy alerts and 2 scanner alerts, then All shows 5, Strategy 3, Scanner 2.
  2. Given a Pending entry alert, then "Review order" opens FR-DEP-007; after expiry the action is disabled and status Expired.
- **Data/Kite dependency:** Local store.
- **Deviation from Streak:** Unread badge added (absent in Streak, FEATURES §3.3).
- **Priority hint:** Core

### FR-DEP-016 Desktop notification channel
- **Source features:** DEP-010
- **Requirement:** The system shall deliver alerts as OS desktop notifications (freedesktop notifications on Linux) with a profile
  toggle (default ON), optional sound, and a per-type filter (strategy entries, exits, scanner alerts). If the OS denies
  notifications, the inbox and toasts still work and the profile shows the channel unavailable.
- **Acceptance criteria:**
  1. Given the toggle OFF, then no OS notification is shown but inbox entries are still created.
- **Data/Kite dependency:** Wails runtime / OS notification service.
- **Deviation from Streak:** OS notifications replace browser/mobile push (no server).
- **Priority hint:** Core

### FR-DEP-017 Paper trading mode — Enhancement (beyond Streak)
- **Source features:** DEP-011
- **Requirement:** Paper deployments shall use the same engine, state machine and order pipeline validations, with a simulated
  executor: MARKET fills at the first LTP received after confirmation/auto-trigger (plus configured slippage); LIMIT fills when
  LTP crosses the limit; charges from the fee table; a separate paper positions/orders book; no Kite order endpoint is called.
  Paper results are labelled "Paper" everywhere.
- **Acceptance criteria:**
  1. Given a paper entry and next LTP 1000.20, then the paper fill is 1000.20 and no `PlaceOrder` call occurs (verified by a
     mock client).
  2. Given trading is disabled, then paper mode still works.
- **Data/Kite dependency:** Ticker; local store.
- **Deviation from Streak:** Streak has no paper trading.
- **Priority hint:** Important

### FR-DEP-018 Armed auto-execution — Enhancement (beyond Streak)
- **Source features:** DEP-011
- **Requirement:** Armed auto mode shall be available only when all hold: trading enabled with static IP confirmed
  (FR-ORD-013); the user turned on "Allow automatic execution" in settings with a typed confirmation; the deployment has per-
  deployment risk limits (max orders/day, max loss/day ₹, max qty per order); and the deployment was armed in the current session
  (arming lapses at session expiry 06:00 and must be renewed after daily login). Every auto order passes the full pipeline;
  stale or missed signals are never auto-executed (except FR-DEP-010 (4) opt-in); the kill switch disarms all. Breaching a risk
  limit disarms the deployment (to mode 1 behaviour) and alerts.
- **Acceptance criteria:**
  1. Given static IP not confirmed, then Armed auto cannot be selected.
  2. Given yesterday's armed deployment, after today's login, then it is in mode-1 behaviour until re-armed.
  3. Given max loss/day ₹1,000 and realised −1,050, then the deployment disarms and no further auto orders are sent.
- **Data/Kite dependency:** ORD pipeline; `PlaceOrder` with `tag`.
- **Deviation from Streak:** Streak does not auto-trade; allowed here only under NFR-05 explicit opt-in.
- **Priority hint:** Nice-to-have

### FR-DEP-019 Deploy-time checks
- **Source features:** DEP-001, DEP-004
- **Requirement:** On Execute the system shall check and report: Kite session valid; candle warm-up available for each DI
  (backfill if not); subscriptions capacity; for mode 1/4: basket margin for the first entry of all DIs vs available funds
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
  dialog listing them and whether each open position is GTT-protected, with options "Keep running in tray" (Enhancement), "Quit"
  and "Cancel".
- **Acceptance criteria:**
  1. Given the access token expired, then all DIs are SUSPENDED and a banner says "Deployments paused — log in to resume".
  2. Given 2 IN_POSITION DIs without GTT, when quitting, then the dialog lists both as "unprotected".
- **Data/Kite dependency:** Session state (C-02).
- **Deviation from Streak:** Streak runs server-side; this compensates (C-03).
- **Priority hint:** Core

---
## FR module SCAN — Scanners

### FR-SCAN-001 Scanner builder layout
- **Source features:** SCAN-001
- **Requirement:** The system shall provide a scanner builder view: editable title (default "Untitled scanner", inline rename),
  link to My Scanners, a Conditions card (universe, timeframe, conditions) with sticky footer Save / Run Scan / Set Alerts (Set
  Alerts enabled once saved), and a Results card with match count, last-run timestamp, export, search and expand. Leaving with
  unsaved changes shall prompt (Save / Discard / Cancel).
- **Acceptance criteria:**
  1. Given a new scanner, then the results card shows an empty state suggesting another universe or timeframe.
  2. Given unsaved edits, when navigating away, then the unsaved-changes prompt appears (Streak bug §3.2-15 not replicated).
- **Data/Kite dependency:** Local store.
- **Deviation from Streak:** Unsaved-change guard and inline rename added.
- **Priority hint:** Core

### FR-SCAN-002 Universe selector
- **Source features:** SCAN-002
- **Requirement:** "Scan on" shall offer (default Nifty 50) the 29 universes of FEATURES Appendix H in that order, plus Custom
  Symbols after Futures, plus the user's watchlists and custom baskets. Index and sector universes come from user-importable
  constituent lists shipped with a dated snapshot (C-09); Indices, Futures and NFO Equities are derived from the instruments dump;
  option universes (Nifty/Sensex weekly, next-week, monthly; Banknifty/Finnifty monthly) resolve at run time to the contracts of
  the nearest / second-nearest weekly or the current monthly expiry, filtered by strike range around ATM (default −6…+6 strikes,
  options 1…30 and Show All) and by CALL / PUT / VOLUME>0 (all on by default). The universe shows its instrument count and the
  constituent snapshot date.
- **Acceptance criteria:**
  1. Given "Nifty Weekly Options", spot 22,520, strike step 50, range −6…+6, CALL+PUT, then 26 contracts (13 strikes × 2) of the
     nearest weekly expiry are scanned before the VOLUME>0 filter.
  2. Given an imported Nifty 500 CSV (columns Company Name, Industry, Symbol, Series, ISIN Code), then the universe has 500
     instruments mapped to NSE tokens, and unmapped symbols are listed.
  3. Given Custom Symbols, then the instrument picker opens and the basket can be saved for reuse.
- **Data/Kite dependency:** `GetInstruments` (daily dump); user-imported CSVs; spot LTP for ATM.
- **Deviation from Streak:** Constituent lists are user-imported/snapshotted, not live (C-09; survivorship bias documented).
- **Priority hint:** Core

### FR-SCAN-003 Scanner timeframe and chart type
- **Source features:** SCAN-003
- **Requirement:** Base timeframe choices shall be 1, 3, 5, 10, 15, 30 min, 1 hour, 1 day (default 1 min; FEATURES §6-7). Chart
  type is candlestick only and shall be shown as a non-interactive label (not a button). Week/month are allowed only on Advance
  condition rows (MTF).
- **Acceptance criteria:**
  1. Given the timeframe dropdown, then exactly 8 options appear.
- **Data/Kite dependency:** Kite intervals; local resampling for 1-hour alignment.
- **Deviation from Streak:** Inert chart-type button not replicated (§3.2-9).
- **Priority hint:** Core

### FR-SCAN-004 Scanner conditions (shared editor)
- **Source features:** SCAN-004
- **Requirement:** The scanner shall reuse the strategy condition editor (Basic and Advance modes, indicator catalogue,
  comparators, AND/OR, groups, per-row timeframe in Advance, row tools, copy/paste, suggestions/"Assist", prebuilt setups) without
  side or exits. Empty Basic rows are ignored by Run Scan. No artificial condition-count limit (NFR-13).
- **Acceptance criteria:**
  1. Given 12 Advance rows, then the scanner saves and runs (no 10-row cap).
  2. Given 2 filled and 1 empty Basic row, then Run Scan evaluates the 2 filled rows.
- **Data/Kite dependency:** none.
- **Deviation from Streak:** No 5/10 condition caps.
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
- **Deviation from Streak:** Silent no-op (§3.2-13) replaced by explicit error.
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
- **Data/Kite dependency:** Ticker candle builder; `GetHistoricalData` (3 req/s); `GetQuote` (≤500 per call, 1 req/s).
- **Deviation from Streak:** Large uncached universes may take longer than Streak's server-side scan (bounded by 3 req/s);
  progress is shown. Whether Kite historical returns today's partial candle is UNVERIFIED (OQ-15).
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
- **Requirement:** Save shall open a dialog (Name required, Tags up to 5, Description), persist the scanner (universe, timeframe,
  conditions, mode, option filters) to My Scanners, and keep the builder open on the saved scanner. Saving again updates it;
  "Save as copy" creates a new one.
- **Acceptance criteria:**
  1. Given a saved scanner reopened, then universe, timeframe, conditions and option filters are identical.
- **Data/Kite dependency:** SQLite.
- **Deviation from Streak:** none (Streak dialog unobserved).
- **Priority hint:** Core

### FR-SCAN-009 Set Alerts (take a scanner live)
- **Source features:** SCAN-009
- **Requirement:** Set Alerts shall open a dialog stating the alert frequency (= scanner base timeframe), notification channels
  (in-app + OS), alert content option ("all matches" default / "only new matches since last alert"), optional end date, and a
  required acknowledgement; "Start alerts" activates the live scanner. For a prebuilt scanner, the user first copies it
  (DISC flow) then activates the copy.
- **Acceptance criteria:**
  1. Given a 1-day scanner, then the dialog states "Runs at every 1 day candle close".
  2. Given the acknowledgement unticked, then Start alerts is disabled.
- **Data/Kite dependency:** Local scheduler.
- **Deviation from Streak:** Content option and end date added (Enhancement); no 25-scanner cap.
- **Priority hint:** Core

### FR-SCAN-010 Live scanner engine
- **Source features:** SCAN-010
- **Requirement:** While the app runs and the session is valid, each active scanner shall be evaluated at every base-TF candle
  close during market hours (5-min → 09:20, 09:25, …), using completed base candles and last closed higher-TF candles, and shall
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
- **Requirement:** The Chart action on a result row shall open a full-size chart view (new window or tab within the app) for the
  symbol at the scanner timeframe.
- **Acceptance criteria:**
  1. Given a 1-day scanner result ADANIENT, then the chart opens on ADANIENT, 1 day.
- **Data/Kite dependency:** Local candles / `GetHistoricalData`.
- **Deviation from Streak:** In-app window instead of a browser tab.
- **Priority hint:** Important

### FR-SCAN-015 Scanner vs strategy builder scope
- **Source features:** SCAN-015
- **Requirement:** The scanner builder shall: have no side and no exits; use universe selection instead of an instrument list;
  offer the same 8 base timeframes and week/month on Advance rows; share comparators, indicators and suggestions with the
  strategy builder; and add scanner-only features (Run Scan, option filters, results sort/search/export/expand). **Enhancement
  (beyond Streak):** "Create strategy from scanner" copies conditions into a new strategy draft.
- **Acceptance criteria:**
  1. Given the scanner builder, then no Buy/Sell toggle or exit card is present.
  2. Given "Create strategy from scanner", then a strategy draft with the same conditions and timeframe opens.
- **Data/Kite dependency:** none.
- **Deviation from Streak:** Scanner→strategy linking added.
- **Priority hint:** Core (scope) · Nice-to-have (enhancement)

---

## FR module SCALP — Scalper workspace

### FR-SCALP-001 Scalper layout
- **Source features:** SCALP-001
- **Requirement:** The system shall provide a scalper workspace: top half — left chart = underlying (default NIFTY 50 index),
  right chart = ATM call of the nearest expiry; edge tabs Option Chain, Watchlist (left) and Add/Hide Chart (right); bottom half —
  left analytics panel for the left chart's symbol, right Positions / Open Orders / Order Book panel with refresh and collapse.
  The primary (left) chart accepts only an index or stock. Layout, symbols and timeframes persist locally across restarts.
- **Acceptance criteria:**
  1. Given NIFTY spot 22,520 and strike step 50, then the right chart defaults to the nearest-expiry 22500 CE.
  2. Given the user changes symbols and restarts, then the same symbols reappear.
  3. Given an option symbol chosen for the left chart, then it is refused with "Choose an index or stock".
- **Data/Kite dependency:** Ticker; instruments dump (expiries, strikes); historical candles.
- **Deviation from Streak:** Local persistence (Streak's unclear).
- **Priority hint:** Important

### FR-SCALP-002 Chart headers and CE/PE flip
- **Source features:** SCALP-002
- **Requirement:** Each scalper chart shall have its own header: symbol search, CE⇄PE flip for options (same strike and expiry),
  LTP with % change, refresh, timeframe (8 options, default 1 hour), save/clear layout, indicators, and its own Buy/Sell overlay
  and drawing tools. Changing chart 1's timeframe sets all charts to it; each can then be changed individually.
- **Acceptance criteria:**
  1. Given right chart 22500 CE, when flipped, then it shows 22500 PE of the same expiry.
  2. Given chart 1 changed to 5 min, then charts 2 and 3 switch to 5 min; changing chart 2 afterwards does not change chart 1.
- **Data/Kite dependency:** Ticker; instruments dump.
- **Deviation from Streak:** none.
- **Priority hint:** Important

### FR-SCALP-003 Add / hide third chart
- **Source features:** SCALP-003
- **Requirement:** Add Chart shall insert a third chart showing the ATM put of the same expiry as the right chart; Hide removes it.
  When the watchlist drawer is open the workspace shows at most 2 charts (FEATURES §6-26) and the UI states this.
- **Acceptance criteria:**
  1. Given Add Chart with ATM 22500, then the third chart shows the 22500 PE.
  2. Given 3 charts and the watchlist opened, then the third chart is hidden with a visible note, and restored when the watchlist closes.
- **Data/Kite dependency:** Instruments dump.
- **Deviation from Streak:** Layout constraint surfaced in UI (§3.2-32).
- **Priority hint:** Important

### FR-SCALP-004 Option chain drawer
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

### FR-SCALP-005 Basket order panel
- **Source features:** SCALP-005
- **Requirement:** The basket panel shall list legs with select checkbox (and select-all), drag handle, B/S, instrument, LTP,
  product (MIS/NRML), order type (Market/Limit, with price field for Limit), qty (lot-stepped spinner), required margin per leg and
  status; footer shows total margin required and margin blocked after execution (basket margin with spread benefit, recalc
  button) and Place Basket Order. Placing shows a confirmation summary, then submits the selected legs **sequentially in the shown
  order** (user is advised to place hedges first) through the order pipeline; each leg's status updates live. If a leg is
  rejected, remaining legs are held and the user chooses Continue / Cancel remaining.
- **Acceptance criteria:**
  1. Given 4 legs, then 4 orders are placed in displayed order, each respecting the throttle, and statuses show Complete/Rejected.
  2. Given leg 2 rejected, then legs 3–4 are not sent until the user chooses.
  3. Given qty spinner on NIFTY (lot 65), then values step 65, 130, …
- **Data/Kite dependency:** `GetBasketMargins` (initial/final), `PlaceOrder` (market_protection for Market), ticker order updates.
- **Deviation from Streak:** Rejection hold behaviour defined (Streak's unobserved).
- **Priority hint:** Important

### FR-SCALP-006 Option strategy templates in the basket
- **Source features:** SCALP-006
- **Requirement:** The basket panel shall offer the 34 templates of FEATURES Appendix G with filters All / Bullish / Bearish /
  Neutral; selecting a template fills legs around ATM of the selected expiry with one lot each. If legs exist, the user confirms
  replacement first.
- **Acceptance criteria:**
  1. Given NIFTY spot 22,520 and "Short Iron Condor", then legs are B 22100 PE, S 22300 PE, S 22700 CE, B 22900 CE (Appendix G).
  2. Given existing legs, then a confirmation appears before replacement (§3.2-23 not replicated).
- **Data/Kite dependency:** Instruments dump; LTP.
- **Deviation from Streak:** Replacement confirmation.
- **Priority hint:** Important

### FR-SCALP-007 Basket payoff graph and table
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

### FR-SCALP-008 Docked watchlist drawer
- **Source features:** SCALP-008
- **Requirement:** The watchlist drawer shall dock beside the charts (not overlay) and reuse the standard watchlist component
  (lists, create, search, delete, rows with B/S and chart); selecting a row loads it into the left chart.
- **Acceptance criteria:**
  1. Given the drawer opens, then the charts narrow and remain interactive.
- **Data/Kite dependency:** see watchlist module.
- **Deviation from Streak:** none.
- **Priority hint:** Nice-to-have

### FR-SCALP-009 Positions / Open orders / Order book panel
- **Source features:** SCALP-009
- **Requirement:** The docked panel shall show tabs Positions (Product, Symbol, Qty, Avg, LTP, P&L, Net change, Total P&L),
  Open Orders (Time, Type, Symbol, Qty, LTP, Price, Status) and Order Book (Time, Type, Symbol, Qty, LTP, Avg price, Status), live-
  updated from ticker order updates and LTP, with manual refresh, and row actions per FR-ORD-011.
- **Acceptance criteria:**
  1. Given an order filled, then it moves from Open Orders to Order Book and the position appears within 1 s of the order update.
- **Data/Kite dependency:** `GetPositions`, `GetOrders`, ticker `OnOrderUpdate`.
- **Deviation from Streak:** none.
- **Priority hint:** Important

### FR-SCALP-010 Collapse bottom panels
- **Source features:** SCALP-010
- **Requirement:** A collapse control shall collapse both bottom panels to their tab bars so charts take full height; Expand
  restores them; state persists.
- **Acceptance criteria:**
  1. Given Collapse, then chart height increases to the window height minus header and tab bars.
- **Data/Kite dependency:** none.
- **Deviation from Streak:** none.
- **Priority hint:** Nice-to-have

### FR-SCALP-011 Trade from chart
- **Source features:** SCALP-011
- **Requirement:** Each tradable chart shall show Buy (best ask) and Sell (best bid) buttons with required margin; clicking opens a
  compact confirmation (Market default, qty, product). Hovering a price level and pressing "+" places a Limit or SL order at that
  level (after confirmation). Open orders and positions are drawn as lines; dragging an order line modifies its price (showing
  remaining modifications of 25); × cancels an order or exits a position (after confirmation). **Enhancement (beyond Streak):**
  opt-in "one-click trading" (per session, with max qty) skips the compact confirmation; all orders still pass the pipeline.
  Outside market hours, buttons show "closed" instead of 0.00.
- **Acceptance criteria:**
  1. Given a drag of a limit order line from 120 to 118, then one modify call is sent and the counter shows 24 remaining.
  2. Given market closed, then Buy/Sell are disabled and labelled "Market closed" (§3.2-19 not replicated).
- **Data/Kite dependency:** Ticker full mode (best bid/ask), `PlaceOrder`, `ModifyOrder`, `CancelOrder`, `GetOrderMargins`.
- **Deviation from Streak:** One-click mode optional; closed-market labelling.
- **Priority hint:** Important

### FR-SCALP-012 Indicators on scalper charts
- **Source features:** SCALP-012
- **Requirement:** Every scalper chart (including the third) shall support indicators without a fixed count limit; a soft
  warning appears above 10 indicators per chart for performance.
- **Acceptance criteria:**
  1. Given 6 indicators on chart 3, then all render.
- **Data/Kite dependency:** none.
- **Deviation from Streak:** Streak allows 5 per chart on charts 1–2 only; removed per NFR-13.
- **Priority hint:** Nice-to-have

### FR-SCALP-013 Scalper additions not in Streak — Enhancement (beyond Streak)
- **Source features:** SCALP-013
- **Requirement:** The parity scalper has no market depth, hotkey settings or quantity presets. As enhancements the system shall
  offer: (a) 5-level market depth panel for the right chart (ticker full mode); (b) configurable keyboard shortcuts for buy, sell,
  exit-all-for-symbol, cancel-all, kill switch, with a shortcut legend; (c) quantity presets (e.g. 1/2/5 lots).
- **Acceptance criteria:**
  1. Given depth enabled, then 5 bid/ask levels with quantities update live.
  2. Given a shortcut for "Buy", then it opens the same confirmation as the Buy button (never bypasses confirmation unless one-click mode is on).
- **Data/Kite dependency:** Ticker `ModeFull`.
- **Deviation from Streak:** All three are absent in Streak.
- **Priority hint:** Nice-to-have

---

## FR module ORD — Orders, positions, holdings and order safety

### FR-ORD-001 Order book
- **Source features:** ORD-001
- **Requirement:** The order book view shall show sections Open, Executed, Rejected, Cancelled with counts; columns Time, Type
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
- **Requirement:** The positions view shall show Net and Day tabs, open and closed positions with Product, Symbol, Qty, Avg, LTP,
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
  1. Given no holdings, then the empty state reads "No holdings" (§3.2-25 not replicated).
  2. Given 10 @ 100 and LTP 110, then Invested 1,000, Current 1,100, P&L 100, Net change +10.00 %.
- **Data/Kite dependency:** `GetHoldings`, `GetAuctionInstruments`.
- **Deviation from Streak:** Correct empty-state copy, refresh added.
- **Priority hint:** Core

### FR-ORD-004 Order ticket
- **Source features:** ORD-004
- **Requirement:** All B/S entry points (watchlists, scanner results, option chains, chart buttons/context menu, notifications,
  shortcuts) shall open one order ticket with: side, instrument, exchange, qty (lots for F&O; remembered per F&O instrument when the
  profile option is on), product (CNC / MIS / NRML as valid for the segment), order type (Market / Limit / SL / SL-M), price and
  trigger price as applicable, validity (Day / IOC), required margin and charges preview, and (from scanner results) a technical
  summary of the condition values. **Enhancement (beyond Streak):** variety AMO outside market hours; iceberg; TTL validity.
  Indices are not orderable (B/S disabled).
- **Acceptance criteria:**
  1. Given an NFO option with lot 65 and qty 100, then the ticket blocks with "quantity must be a multiple of 65".
  2. Given SL type, then trigger price is required; given SL-M, then price is hidden and trigger required.
  3. Given a scanner result B, then the ticket shows the conditions and their current values above the form.
- **Data/Kite dependency:** `GetOrderMargins`, `GetOrderCharges`, instruments dump; `PlaceOrder`.
- **Deviation from Streak:** AMO/iceberg/TTL optional enhancements.
- **Priority hint:** Core

### FR-ORD-005 Order safety pipeline
- **Source features:** ORD-004, ORD-006, ORD-007
- **Requirement:** Every order action (place, modify, cancel, exit, basket leg, GTT) from any source shall pass, in order:
  (1) **Gate:** trading enabled (FR-ORD-013), Kite session valid, kill switch not engaged; (2) **Validation:** instrument tradable
  and not expired; qty > 0 and lot multiple; qty ≤ freeze quantity (else block, or autoslice if the user enabled it); price/trigger
  multiples of tick size; Limit price within the day's circuit limits; SL trigger on the correct side of LTP; CNC sell ≤ holdings
  qty; product valid for segment; inside market hours for regular variety; (3) **Risk limits:** user-configurable max order value,
  max qty per order, max orders per day, max daily loss; a price deviation warning when a Limit price is > X % (default 5 %) from
  LTP; (4) **Margin:** `GetOrderMargins` / `GetBasketMargins` vs `GetUserMargins` available — block if insufficient; (5) **Duplicate
  guard:** an identical order (instrument, side, qty, product, type, price) from a manual source within 3 s requires re-confirmation;
  (6) **Confirmation:** explicit user confirmation, or armed-auto authorisation (FR-DEP-018); (7) **Journal intent** (FR-ORD-008);
  (8) **Throttle** (FR-ORD-006); (9) **Submit** with `market_protection` (FR-ORD-007) and a unique `tag`; (10) **Journal outcome**
  and track updates. A failure at any stage stops the order and shows the stage and reason.
- **Acceptance criteria:**
  1. Given trading disabled, then no Kite order endpoint is called (verified with a mock client) and the UI says why.
  2. Given qty above the freeze limit and autoslice off, then the order is blocked at stage 2.
  3. Given available margin 10,000 and required 12,000, then blocked at stage 4.
  4. Given every stage passes, then exactly one `PlaceOrder` call carries market_protection (if Market/SL-M) and the tag.
- **Data/Kite dependency:** `GetQuote` (circuit limits), `GetOrderMargins`, `GetBasketMargins`, `GetUserMargins`, `GetHoldings`,
  `PlaceOrder`/`ModifyOrder`/`CancelOrder`; freeze-quantity table (source: OQ-13).
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
- **Requirement:** An always-visible kill switch (button and keyboard shortcut) shall, after one confirmation: clear the order
  queue; cancel all open orders placed by the app (option: all open orders in the account); disarm and suspend all deployments
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
- **Requirement:** Every instrument row (watchlist, scanner results, alerts snapshots, option chains, dashboard widgets) shall have B
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
  modes never require trading to be enabled.
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

---

## Traceability — feature ID → FR IDs

| Feature ID | FR IDs |
|---|---|
| BT-001 | FR-BT-001 |
| BT-002 | FR-BT-002 |
| BT-003 | FR-BT-003 |
| BT-004 | FR-BT-004 |
| BT-005 | FR-BT-005, FR-BT-006, FR-BT-007 |
| BT-006 | FR-BT-008, FR-BT-009 |
| BT-007 | FR-BT-009, FR-BT-010, FR-BT-045 |
| BT-008 | FR-BT-011, FR-BT-012, FR-BT-044, FR-BT-045 |
| BT-009 | FR-BT-012 |
| BT-010 | FR-BT-013 |
| BT-011 | FR-BT-014, FR-BT-033 |
| BT-012 | FR-BT-015 |
| BT-013 | FR-BT-016 |
| BT-014 | FR-BT-017, FR-BT-018 |
| BT-015 | FR-BT-019, FR-BT-033 |
| BT-016 | FR-BT-020 |
| BT-017 | FR-BT-021 |
| BT-018 | FR-BT-022 |
| BT-019 | FR-BT-023, FR-BT-024, FR-BT-025, FR-BT-026, FR-BT-030 |
| BT-020 | FR-BT-030, FR-BT-031 |
| BT-021 | FR-BT-027, FR-BT-028, FR-BT-029, FR-BT-030 |
| BT-022 | FR-BT-007 |
| BT-023 | FR-BT-034 |
| BT-024 | FR-BT-035, FR-BT-036, FR-BT-044 |
| BT-025 | FR-BT-037, FR-BT-044 |
| BT-026 | FR-BT-032, FR-BT-038, FR-BT-039 |
| BT-027 | FR-BT-040 |
| BT-028 | FR-BT-041 |
| BT-029 | FR-BT-042 |
| BT-030 | FR-BT-026, FR-BT-032, FR-BT-043 |
| DEP-001 | FR-DEP-001, FR-DEP-003, FR-DEP-019 |
| DEP-002 | FR-DEP-002 |
| DEP-003 | FR-DEP-004, FR-DEP-005, FR-DEP-010, FR-DEP-011 |
| DEP-004 | FR-DEP-004, FR-DEP-005, FR-DEP-009, FR-DEP-010, FR-DEP-012, FR-DEP-019, FR-DEP-020 |
| DEP-005 | FR-DEP-004, FR-DEP-006, FR-DEP-007, FR-DEP-010, FR-DEP-020 |
| DEP-006 | FR-DEP-004, FR-DEP-008, FR-DEP-009 |
| DEP-007 | FR-DEP-004, FR-DEP-013 |
| DEP-008 | FR-DEP-014 |
| DEP-009 | FR-DEP-007, FR-DEP-015 |
| DEP-010 | FR-DEP-016 |
| DEP-011 | FR-DEP-003, FR-DEP-017, FR-DEP-018 |
| SCAN-001 | FR-SCAN-001 |
| SCAN-002 | FR-SCAN-002 |
| SCAN-003 | FR-SCAN-003 |
| SCAN-004 | FR-SCAN-004 |
| SCAN-005 | FR-SCAN-005 |
| SCAN-006 | FR-SCAN-006 |
| SCAN-007 | FR-SCAN-007 |
| SCAN-008 | FR-SCAN-008 |
| SCAN-009 | FR-SCAN-009 |
| SCAN-010 | FR-SCAN-010 |
| SCAN-011 | FR-SCAN-011 |
| SCAN-012 | FR-SCAN-012 |
| SCAN-013 | FR-SCAN-013 |
| SCAN-014 | FR-SCAN-014 |
| SCAN-015 | FR-SCAN-015 |
| SCALP-001 | FR-SCALP-001 |
| SCALP-002 | FR-SCALP-002 |
| SCALP-003 | FR-SCALP-003 |
| SCALP-004 | FR-SCALP-004 |
| SCALP-005 | FR-SCALP-005 |
| SCALP-006 | FR-SCALP-006 |
| SCALP-007 | FR-SCALP-007 |
| SCALP-008 | FR-SCALP-008 |
| SCALP-009 | FR-SCALP-009 |
| SCALP-010 | FR-SCALP-010 |
| SCALP-011 | FR-SCALP-011 |
| SCALP-012 | FR-SCALP-012 |
| SCALP-013 | FR-SCALP-013 |
| ORD-001 | FR-ORD-001, FR-ORD-008, FR-ORD-014 |
| ORD-002 | FR-ORD-002, FR-ORD-014 |
| ORD-003 | FR-ORD-003 |
| ORD-004 | FR-ORD-004, FR-ORD-005, FR-ORD-006, FR-ORD-007, FR-ORD-013 |
| ORD-005 | FR-ORD-008, FR-ORD-010 |
| ORD-006 | FR-ORD-005, FR-ORD-006, FR-ORD-008, FR-ORD-009, FR-ORD-011 |
| ORD-007 | FR-ORD-005, FR-ORD-012 |

## Open questions

| ID | Question | FRs affected | Default taken until resolved |
|---|---|---|---|
| OQ-01 | Is backtest Initial Capital per instrument or shared across instruments on the current platform? | FR-BT-005, FR-BT-012 | Per instrument (legacy docs) |
| OQ-02 | Does cycle "–" mean unlimited, and what is the maximum cycle count in live mode? | FR-BT-002, FR-DEP-001 | Unlimited; 1–5 offered |
| OQ-03 | Entry window boundaries: is the test on signal time or fill time, inclusive or exclusive of End Time? Does End Time force exit? | FR-BT-003 | Fill time, inclusive; no forced exit |
| OQ-04 | Futures quantity in backtests: lot-rounded or raw floor(capital/price)? | FR-BT-006 | Lot-rounded |
| OQ-05 | Base of day/month return % (initial capital, capital + running P&L, or capital used)? Are no-trade days included in weekday averages? | FR-BT-015, FR-BT-016, FR-BT-019 | Initial capital; active days only |
| OQ-06 | Are exit conditions level-triggered or transition-triggered? | FR-BT-026 | Level-triggered |
| OQ-07 | SL/TP evaluation: close-based vs high/low after the entry candle (FEATURES §6-15). Verify with a controlled backtest comparison. | FR-BT-028 | Hybrid default + Intrabar option |
| OQ-08 | TSL reference start (entry price vs first candle high) and fill price on TSL hit (level vs close). | FR-BT-029 | Entry price; fill at level |
| OQ-09 | Square-off rule when T is not a candle boundary (e.g. 15:10 on 15-min); source of the call-auction (CAS) stock list. | FR-BT-031 | Containing-candle open; user-importable list |
| OQ-10 | Is MIS deployment expiry at 15:21 a full deployment end (redeploy daily) or a daily pause? | FR-DEP-004, FR-DEP-012 | Ends (EXPIRED); optional daily re-arm |
| OQ-11 | Entry alert expiry window in Streak (if any). | FR-DEP-006 | Next base-TF candle close |
| OQ-12 | Does GTT placement require the registered static IP? Is there a cap on active GTTs or GTT modifications? | FR-DEP-009 | Treat as requiring trading enabled; rate-limit modifications |
| OQ-13 | Authoritative source of exchange freeze quantities usable offline (instruments dump lacks it). | FR-ORD-005, FR-DEP-012 | User-maintained table + autoslice option |
| OQ-14 | Should AMO orders be offered for late signals (Streak does not support AMO)? | FR-DEP-006, FR-ORD-004 | Not for parity; AMO as optional enhancement |
| OQ-15 | Does Kite historical API return today's partial (forming) candle intraday? | FR-SCAN-006, FR-BT-041 | Use ticker-built candle; verify |
| OQ-16 | Live scanner alert content: all matches each run or only new matches? | FR-SCAN-009, FR-SCAN-010 | All matches, option for new only |
| OQ-17 | Sector universes (Energy, Financials, …): which classification source and licence? | FR-SCAN-002 | User-imported lists |
| OQ-18 | Risk-free rate and dividend assumptions for Greeks/IV; spot vs futures as underlying. | FR-SCALP-004, FR-SCALP-007 | User-configurable; spot |
| OQ-19 | Can the public egress IP be checked without a third-party service, or does Kite expose registered IPs via API? | FR-ORD-013 | Rejection detection + optional user endpoint |
| OQ-20 | Expired-contract intraday history availability for options/futures backtests. | FR-BT-032, FR-BT-038 | Live contracts only |
| OQ-21 | Trading-holiday calendar source for schedulers (MIS expiry, scanner runs, reconciliation). | FR-DEP-004, FR-SCAN-010 | User-importable holiday list; infer from missing candles |
