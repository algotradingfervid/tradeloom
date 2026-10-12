# Streak (streak.tech) — Feature & UI/UX Inventory

Clean-room research for a local Go + Wails desktop equivalent. Observed from the
logged-in web app via the browser on 2026-10-11. Observation only: nothing was saved,
deployed, ordered or changed in the account. No Streak code or assets are copied.

Status legend: [x] explored · [ ] pending

## Global shell
- Top bar: index tickers (Nifty 50, Bank Nifty, Sensex: LTP, abs change, % change),
  "Live" market-status dot, Order Book, Positions, notifications bell, profile menu.
- Left sidebar (icon + label): Home `/`, Scalper `/markets`, Strategies `/discover-strategies`,
  Equity Trader `/strategy?defaultType=equity`, Options Trader `/strategy?defaultType=options&isNew=true`,
  Live Strategies `/live-trades`, Scanners `/discover-scanners`, Create Scanner `/scanner`,
  Scanner Alerts `/live-alerts`.
- Onboarding: per-page guided tours ("N of M" popovers with Next / ×).
- Unsaved-change guard: the strategy builder blocks navigation with a "Leave site?" prompt.

## Home / Dashboard `/dashboard`
- Holdings card: Current Value, Investment, P&L (abs + %).
- Live Strategy Overview: Active / Closed / Total strategy counts.
- Main chart (TradingView-style): symbol search, timeframe (e.g. 1 Hour), Save, delete, ƒx Indicators,
  Buy/Sell quick buttons with bid/ask, OHLC + change readout, volume, crosshair.
- Tabs under chart: Technicals, Option Chain, OI, PCR, Advance/Decline, ATM IV/Straddle; Collapse.
  - Technicals: Bearish–Neutral–Bullish gauge with counts; pivot levels S3..R3; Short Term (5 & 20 SMA
    crossover), Long Term (20 & 50 SMA crossover), RSI(14) verdicts; Event list of detected patterns
    (e.g. Heikin Ashi Bearish Reversal, Bullish Engulfing, Bullish CCI Crossover, Bullish Inside bar) with timestamps.
- Watchlist panel: multiple watchlists (Watchlist 1, 2), Create Watchlist, symbol search, delete;
  rows: symbol + exchange tag, LTP / change, B / S buttons.
- Live Scanners panel: timeframe (1 Day), universe (Nifty 50), tabs Top Gainers / Top Losers /
  Opening Range Breakout / Opening Range Breakdown / …; rows with LTP/change, B/S; "See Full Results".
- Live Backtest panel: tabs of prebuilt strategies (Bollinger Band Mean Reversion, Falling Momentum,
  Volatility Contraction, …) with Backtest P&L, Max Gain, Max Loss, Win vs Loss bar, Traded count, Capital Used.

## Equity strategy builder `/strategy?defaultType=equity`
- Header: strategy name (Untitled Strategy, editable), My Strategies button.
- Entry card: Buy/Sell toggle; Instruments (avatars + Add Instruments); Chart Settings: chart type
  (Candlestick) and timeframe {1m, 3m, 5m, 10m, 15m, 30m, 1h, 1d}.
- Conditions: Basic / Advance toggle; Assist (AI helper); rows `If <indicator> <comparator> <indicator|value>`
  with delete; Add Another Condition; Pre built Setups carousel (SMA, RSI, RSI MA, EMA, WMA, DEMA, TEMA, MA, …).
  - Picking SMA pre-fills `SMA(close, 9, 0) crosses above SMA(close, 21, 0)` → indicator syntax is
    `NAME(source, period, offset)`.
  - Comparators: Equal to, Crosses above, Crosses below, Lower than, Higher than equal to,
    Lower than equal to, Higher than, Up by, Down by.
  - Indicator catalogue (dropdown, "See All"): SMA, RSI, RSI Moving Average, EMA, WMA, DEMA, TEMA,
    Moving Average, ADX, +DI, -DI, ADX Moving Average, Supertrend, Williams %R, Upper/Lower/Middle
    Bollinger Bands, VWAP, VWAP Moving Average, Volume, Volume Oscillator, Vortex Indicator,
    Money Flow Index, MFI Moving Average, Stochastic, Stochastic RSI, CCI, ATR, Awesome Oscillator,
    Choppiness Index, Ultimate Oscillator, Chaikin Money Flow, Chande Momentum Oscillator,
    Bollinger Bandwidth, Alligator, Aroon Oscillator, Donchian Channel, Elder Force Index, High,
    Ichimoku Cloud, Keltner Channel, Low, Median Price, Median Price Moving Average, Nth Candle, Open,
    Opening Range, Pivot Points, Previous N, Close, MACD, Parabolic SAR, Momentum Indicator,
    Trend Intensity Index, Bollinger %B, OBV, Standard Deviation, Fractal Chaos Bands,
    Linear Regression Forecast, Aroon Up, Aroon Down, Swing Index, Twiggs Money Flow,
    OBV Moving Average, True Range, Price Rate Of Change, Ehler Fisher, Time Series Forecast,
    Coppock Curve, ATR Bands, Central Pivot Range.
- Exit card (per instrument): Stop loss, Target profit, Trailing stop loss; unit radio
  %Percentage / Points / PNL; Add Exit Conditions (optional indicator-based exits). Save.
- Right pane: chart with tabs for each selected instrument, timeframe, Save, ƒx Indicators.

## Discover Strategies `/discover-strategies`
- Disclaimer: prebuilt strategies are educational, not recommendations. Create Strategy, My Strategies.
- Left list: filter tabs All / Equity Swing / Equity Intraday; search; filter icon; rows with name,
  complexity badge (Basic), timeframe (min / 15min / hour / day).
  Strategies seen: Volatility Contraction, Falling Momentum, Bollinger Band Mean Reversion, Volatility
  Expansion, Strong Breakout Continuation, Volatility Burst, Volatility Expansion Trend Breakout,
  RSI Trend Momentum Breakout, VWAP Mean Reversion, CPR Momentum Breakout, Pivot Breakdown,
  TEMA Momentum Shift, Accumulation Breakout With Volume, Stochastic Reversal, Stochastic RSI Breakdown,
  Williams With Supertrend, EMA Reversal, Bollinger Band Squeeze Breakout, Mean Reversion With MACD,
  CCI Recovery Signal, …
- Detail pane: name, Copy, View Strategy, plain-English description; "Backtest Results" with period
  (e.g. 13/07/2026 to 11/10/2026, ~3 months); View Full Backtest Results; instrument chips
  (Cumulative + each symbol with LTP/%).
  - Summary metrics: Total P&L, W/L Count, Trades, W/L Streak, Max Gain, Max Loss, Winning Avg., Losing Avg.
  - Hypothetical Daily Summary: per weekday Avg. Return, Max Profit, Max Loss.
  - Hypothetical Monthly Performance: year grid of months with % return, coloured green/red.
  - Footer actions: Copy, Notification Based Trading (deploy).

## Full backtest results `/backtest/<strategy-slug>` (e.g. equity-volatility-contraction-146555-sap)
- Header: strategy name, backtest period (13 Jul 2026 to 11 Oct 2026); actions View (strategy definition),
  Copy Strategy, Notification Based Trading (deploy). Has its own 2-step tour.
- Left column: Combined Results card (total P&L, Wins, Losses, Trades) + per-instrument table
  (Instrument with logo, P&L, coloured). Clicking an instrument switches the right pane.
- Right pane, per selected instrument (symbol, exchange, LTP, % change):
  - Trading Charges toggle: when ON, P&L is shown net of brokerage/taxes
    (APOLLOHOSP −523.79 → −1,254.00). Default OFF.
  - Metrics row: Total P&L, W/L Count, Trades, W/L Streak (max consecutive wins/losses), Max Gain,
    Max Loss, Winning Avg., Losing Avg., Max DD.
  - Hypothetical Daily Summary (weekday × Avg. Return / Max Profit / Max Loss) and
    Hypothetical Monthly Performance (year grid of month tiles, colour intensity by % return).
  - "Chart / Trade Log": TradingView-style candle chart of the instrument at the strategy timeframe with
    trade markers; "See All Transactions" scrolls to the log.
  - "Hypothetical cumulative Profit and Loss (P&L)" calendar heatmap: one mini-grid per month
    (Jul 2026…Oct 2026), each cell a trading day, green/red by day P&L.
  - Trade Log: columns Date & Time, Symbol, P&L, Price, Buy/Sell, Qty (signed), Trigger;
    paginated (Prev / 1 2 3 / Next); Download Trade Log (file export).
    Trigger codes seen: EN = entry, SO = square-off (intraday positions closed at 15:15).
    Expected others: SL (stop loss), TP (target), TSL (trailing SL), EX (exit condition) — to verify.
    P&L column on an exit row = realised P&L of that round trip; on entry rows it carries the running value.
  - P&L Chart: line chart of cumulative P&L over dates, segments green above 0 / red below 0.
- Observed engine behaviour: fixed qty per instrument (e.g. 2), one position at a time per instrument,
  intraday square-off at 15:15, entry fills at the candle close price of the signal candle.

## Strategy view (read-only, prebuilt) `/strategy/<slug>`
- Same layout as the builder but locked; "Discover" badge; description under the title.
- Instruments: avatar stack + "+10"; chart tabs across the right for every instrument (scrollable).
- Conditions in Advance mode show:
  - per-condition timeframe chip (15min) → multi-timeframe conditions supported;
  - arithmetic expressions with brackets: `( High(-2) - Low(-2) ) lower than ( High(-6) - Low(-6) )`;
  - candle offset in parentheses: `High(-2)` = high 2 candles ago, `Volume(0)` = current candle;
  - constant multipliers: `Volume(0) crosses above Volume(-3) * 3.6`;
  - logical joiners between rows ("and"; "or" expected — verify).
- Exit: Stop loss 4 %, Target profit 6 %, no trailing SL (example values).
- Footer: Copy (clone into My Strategies), Notification Based Trading (deploy).

## Add Instruments dialog ("Add Stocks", from the equity builder)
- Search box ("Search eg: SBIN, TCS etc.").
- Universe chips: NIFTY50, NIFTY100, NIFTY200, NIFTY500, BANKNIFTY; watchlist chips: Watchlist 1, Watchlist 2.
- Left list: select-all checkbox; rows = checkbox, logo, symbol, exchange tag (NSE), Company Name.
- Right list: "Symbol (4 / 50)" → max 50 instruments per strategy; each with Live Price + % change; remove (−).
- Footer: "+ Add to Watchlist" (save current selection as a watchlist), Done.

## Copy Strategy dialog (from Copy / first step of Notification Based Trading)
![Copy Strategy dialog](screenshots/00-core/06-copy-strategy-dialog.jpg)
- Feature: clones a prebuilt or own strategy into My Strategies so it can be edited, backtested and deployed.
- Fields: Name (required, "Enter a name to save"); Category chips Equity Intraday / Equity Swing (prefilled from source);
  Tags (free text, chips with ×, counter 3/5 → max 5); Description (multi-line, prefilled). Buttons Cancel / Copy.
- After Copy: navigates to `/strategy/equity-<name-slug>` in editable mode. (Research copy created with user approval:
  `zz-research-copy` — delete when research is done.)

## Own strategy — editable Advance mode
![Editable strategy](screenshots/00-core/07-own-strategy-editable-advance.jpg)
- Header adds ☆ (favourite) and ⋮ (more menu) next to My Strategies.
- Per condition row: timeframe dropdown (15min…), expression boxes with inline ⊕ (append a term/operator),
  comparator box, right-side controls: enable/disable toggle, duplicate, delete.
- Below rows: "+ and / or" (append a row; joiner shown between rows, clickable), "+ Group ( )" (bracketed group of
  conditions), "Copy All" / "Paste All" (copy conditions between strategies/sections).
- Empty row placeholders: "Add expression" · "Add comparator" · "Add expression".
  ![New row](screenshots/00-core/08-and-or-new-row.jpg)
- "Select Expression" popover: tabs All / Indicators / Math Functions; search box "Search or enter a number…"
  (numeric constants typed directly); list includes indicators (MFI, VWAP MA, Ulcer, -DI, TSF, T Range, MFI MA, ADX,
  Stochastic, WMA, TII, …). ![Select expression](screenshots/00-core/09-select-expression-popover.jpg)
  Math functions per docs: + − × ÷, Period Min/Max, Min/Max, Ceil, Floor (see areas/06-public-docs.md).
- Footer: Save, Execute. Execute → Execution panel with modes Notification-Based Trading and Backtest; Backtest
  Parameters (initial capital, quantity, date range, strategy cycle, entry start/stop time, product) per docs —
  NOT yet observed in UI (click was blocked by the permission classifier; awaiting user).

## Scanner copy + alerts (observed with user approval; copy `zz-research-scanner` created, alerts NOT enabled)
- "Set Alerts" on a prebuilt scanner → info toast "Copy the Scanner in order to set alerts" + Copy Scanner dialog:
  Name (required), Tags (prefilled, max 5), Description. ![Copy scanner](screenshots/00-core/10-copy-scanner-dialog.jpg)
- After Copy → `/scanner/<slug>?openAlert=true`: editable scanner (Scan On universe, Chart Settings, Conditions,
  Assist, Pre built Setups; footer Set Alerts / Save / Run Scan; results header has download icon, search, full-screen)
  with modal "Get Scanner Alert!": "This action will activate alerts for the scanner, providing you with notifications
  every 1 Day" (frequency = scanner timeframe), "I accept the trading terms" checkbox (link), Get Alerts button
  (disabled until accepted). No channel choice (push/app only). ![Get alert](screenshots/00-core/11-get-scanner-alert-dialog.jpg)
- My Scanners `/my-scanners`: "← My Scanners — View, manage, and monitor all the scanners you have created";
  Create Scanner, Discover Scanners; search; filter icon (tooltip "Filter options coming soon"); timeframe tabs
  All/1min/5min/15min/1hour/1day; rows: name, ☆ favourite, universe chip, timeframe chip, ⋮ menu (Delete, Edit, Copy).
  Right pane: name, Basic/Advance badge, Edit Conditions, description, "Scanner Results (N) Last Scan: <ts>" table
  (Symbol, LTP, Change %, Volume, B / S / chart), footer Re-Scan, Set Alerts.
  ![My scanners](screenshots/00-core/12-my-scanners.jpg) ![Row menu](screenshots/00-core/13-my-scanners-row-menu.png)

## My Strategies `/my-strategies`
- "← My Strategies — View and manage all the strategies you have created across equity and options";
  Create Strategy, Discover Strategies; tabs All / Equity Swing / Equity Intraday / Option Buying / Option Selling;
  search; filter; rows: icon, name, ☆, Basic/Advance badge, timeframe, ⋮.
- Right pane: name, Copy, Edit Strategy, description, Backtest Results + "Run Backtest →", instrument chips;
  empty state "No backtest results available." + Run Backtest; footer Run Backtest → / Notification Based Trading.
  ![My strategies](screenshots/00-core/14-my-strategies-no-backtest.jpg)

## Execute → Backtest parameters dialog (observed)
![Backtest params](screenshots/00-core/15-execute-backtest-params.jpg)
- Title "Execute · Backtest". Fields: Initial Capital (default 25000); Start Date (default = base-TF lookback start,
  13/07/2026 for 15-min ≈ 90 days); End Date (today); Start Time (hh dropdown : mm dropdown, AM/PM, default 9:15 AM);
  End Time (default 3:30 PM) — entry window; Holding Type radio Intraday (MIS) / Longterm (CNC);
  Strategy Cycle dropdown {–, 1 Cycle … 5 Cycles} (max entries per day; disabled when CNC).
  ![Cycles](screenshots/00-core/16-strategy-cycle-options.jpg) ![CNC](screenshots/00-core/17-backtest-params-cnc.jpg)
- Buttons Cancel / Run Backtest. No quantity field: quantity = floor(Initial Capital ÷ entry price) per instrument
  (verified: 25000 / 2651.80 → qty 9; 25000 / 8835.5 → 2).
- On run: toast "Strategy Saved Successfully" (running also saves), navigates to `/backtest/<slug>`, results stream in
  per instrument with spinners. ![Loading](screenshots/00-core/18-backtest-progressive-loading.jpg)
- Own-strategy backtest header: Edit ✏, Run Backtest, Notification Based Trading. Trade log paginates (1–5).
  Observed fills: signal on 15:00 candle → EN; SO at 15:25 (non-CAS stock). ![Results](screenshots/00-core/19-own-backtest-results.jpg)

## Execute → Notification Based Trading dialog (observed, NOT deployed)
![Deploy dialog](screenshots/00-core/20-deploy-nbt-dialog.jpg)
- Title "Execute · Notification Based Trading". Left: "Instruments (15 / 50)" master checkbox, "* Capital" (required),
  per-instrument checkbox + Qty input (default 1). Right "Options": Strategy Cycle dropdown, Order Type (Market — fixed),
  Holding Type Intraday MIS / Longterm CNC. Footer: "I accept the trading terms" checkbox, Cancel, Execute (disabled
  until terms accepted).
