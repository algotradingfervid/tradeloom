# Area 01 — Equity Strategy Builder & My Strategies

Observed 2026-10-11 in the logged-in web app (own browser tab), read-only. Nothing was saved,
copied, deployed or ordered. Strategy-level "Save" was never pressed; parameter dialogs were
closed with Cancel / overlay click. Complements `../streak-feature-inventory.md`.

## 0. Coverage summary
- Basic & Advance condition editors: fully explored (operators, functions, groups, joiners, per-row timeframe).
- Parameter dialogs: all 71 Basic-dropdown entries + 33 Advance-only operands/functions read programmatically
  (labels, defaults, every dropdown option). Comparator list verified for all 70 selectable indicators.
- Prebuilt-suggestion modal (See All / Assist): 335 bullish + 331 bearish suggestions enumerated (counts + samples).
- Exit card, Chart settings, Buy/Sell, Save-Strategy dialog (opened & cancelled), My Strategies list,
  row menu, filters, own-strategy backtest page and saved-strategy builder view.
- Not done (by rule): pressing Save / Execute / Run Backtest / Copy / Deploy; so post-save screens and
  save-time validation messages are unverified.

## 1. Equity builder — `/strategy?defaultType=equity`

**Feature description:** The no-code editor where the user defines an equity strategy — which stocks, the
chart timeframe, entry conditions, and exits (SL / target / trailing SL or indicator-based exit) — while
seeing a live chart of the selected instruments.

### 1.1 Layout
- Two columns. Left (~48%): strategy form (Entry card, Exit card, bottom-right primary **Save**).
  Right (~52%): live chart with one tab per selected instrument (SBIN, HDFCBANK, ADANIENT, APOLLOHOSP
  appear by default even on a fresh "Untitled Strategy" — the draft is pre-seeded with 4 instruments,
  avatar stack shown in "Instruments").
- Header: editable title "Untitled Strategy", top-right "My Strategies" outline button.

### 1.2 Condition row mechanics (Basic mode)
- Row = `If|And <LHS operand> <comparator select> <RHS operand> [trash]`. First row label "If",
  later rows labelled "And" (see joiners section for Or).
- LHS "Select Indicator" is a **searchable combobox**: typing filters by full name
  (e.g. "Super" → "Supertrend"); list always ends with a "See All" link.
- Picking an indicator into an EMPTY row auto-fills a complete default condition
  (comparator + RHS). Examples:
  - SMA → `SMA(close, 9, 0) crosses above SMA(close, 21, 0)`
  - Supertrend → `Supertrend(7,3,0) crosses below Close(0)`
  - If the row already had a comparator/RHS, only the LHS changes (RSI replaced Supertrend and kept
    `crosses below Close(0)`).
- Comparator select is disabled (hatched) until an LHS operand exists.
- Clicking a filled operand opens the **parameter dialog** (modal, centred, dimmed backdrop):
  title = short indicator name, red trash icon top-right (clears that operand back to
  "Select Indicator" and disables the comparator, RHS kept), one labelled field per parameter,
  footer Cancel / Save (Save = apply to draft only). Escape does NOT close it; clicking the backdrop does.
- Parameter fields: numeric inputs (`type=number`, placeholder "Enter <param>", spinner arrows) or
  select dropdowns (e.g. `field`: close / open / high / low / volume).
- Validation: none inline — period could be cleared or set to -5 without any message
  (validation, if any, presumably happens on dialog Save / strategy Save, which were not pressed).
- Row trash (red, right) deletes the row. "+ Add Another Condition" appends a row.


### 1.3 Basic-mode details (additional)

![Builder default state](../screenshots/01-equity-builder/01-builder-default.jpg)

**Feature description (Basic conditions):** Build entry rules as a list of rows "indicator – comparator –
indicator/number"; all rows are combined with AND. Fast for beginners, with auto-filled sensible defaults.

- Indicator dropdown (LHS and RHS): first ~9 items visible, scrollable, ends with "See All".
  The **RHS dropdown starts with a "Number" item**; typing a number in the RHS search and blurring/Escape
  creates a constant operand (e.g. `45`). Clicking a number operand opens a **"Number"** dialog
  (single field "number", Cancel/Save, trash).
- Comparator list (identical for all 70 selectable indicators, verified programmatically):
  Equal to, Crosses above, Crosses below, Lower than, Higher than equal to, Lower than equal to,
  Higher than, Up by, Down by. (In Basic mode the selected value is shown lower-case, e.g. "crosses above".)
- Joiner in Basic mode is a static "And" label for rows 2+; no OR in Basic mode (OR requires Advance).
- "Pre built Setups" carousel: horizontally scrolling tiles (mini chart icon + short name) with a
  » button; tiles are the same 71 indicators as the dropdown (SMA, RSI, RSI Moving Average, EMA, WMA, DEMA,
  TEMA, Moving Average, ADX, +DI, -DI, Adx Moving Average, Supertrend, Williams %R, Upper/Lower/Middle
  Bollinger Bands, VWAP, Vwap Moving Average, Volume, Volume Oscillator, Vortex Indicator, Money Flow Index,
  MFI Moving Average, Stochastic, Stochastic RSI, CCI, ATR, Awesome Oscillator, Choppiness Index,
  Ultimate Oscillator, Chaikin Money Flow, Chande Momentum Oscillator, Bollinger Bandwidth, Alligator,
  Aroon Oscillator, Donchian Channel, Elder Force Index, High, Ichimoku Cloud, Keltner Channel, Low,
  Median Price, Median Price Moving Average, Nth Candle, Open, Opening Range, Pivot Points, Previous N,
  Close, MACD, Parabolic SAR, Momentum Indicator, Trend Intensity Index, Bollinger %B, OBV,
  Standard Deviation, Fractal Chaos Bands, Linear Regression Forecast, Aroon Up, Aroon Down, Swing Index,
  Twiggs Money Flow, OBV Moving Average, True Range, Price Rate Of Change, Ehler Fisher, Time Series
  Forecast, Coppock Curve, ATR Bands, Central Pivot Range). Clicking a tile fills the first empty row
  with that indicator's default condition (same template as section 2, e.g. SMA tile →
  `SMA(close, 9, 0) crosses above SMA(close, 21, 0)`).

![Indicator dropdown](../screenshots/01-equity-builder/02-indicator-dropdown.jpg)
![Comparator dropdown](../screenshots/01-equity-builder/03-comparator-dropdown.jpg)
![Parameter dialog – SMA (RHS operand)](../screenshots/01-equity-builder/04-parameter-dialog-sma.jpg)
![Parameter dialog – MACD](../screenshots/01-equity-builder/05-parameter-dialog-macd.jpg)
![Number operand dialog](../screenshots/01-equity-builder/06-number-operand-dialog.jpg)
![RHS dropdown with Number option](../screenshots/01-equity-builder/07-rhs-dropdown-number-option.jpg)

### 1.4 Switching Basic → Advance
- If conditions exist, toggling to Advance shows modal **"Switch Mode"**: "Switching to advance mode will
  reset your current conditions. Are you sure?" [Cancel] [Confirm]. With no conditions it switches silently.
  (Conditions are NOT converted — a rebuild could do better by converting basic rows to expressions.)

![Switch Mode confirm](../screenshots/01-equity-builder/08-switch-mode-confirm.jpg)

### 1.5 Advance mode — expression editor

**Feature description:** A free-form logic builder: each condition row is `expression comparator expression`
where expressions can combine indicators, numbers and math functions with + − × ÷, rows can be joined with
AND/OR, nested in groups, have their own timeframe, and be disabled/copied/pasted.

![Advance mode, empty](../screenshots/01-equity-builder/20-advance-mode-empty.jpg)

- Header: "Conditions  Basic [toggle] Advance (i)". Hovering (i) shows tooltip "Learn about Advance Create —
  Click here" (link to help). "Pre built Setups" and "Assist" are hidden in Advance mode? — Assist button
  is not shown in Advance (observed: header has only the toggle + info icon).
- Row layout: `[timeframe chip ▾] [Add expression] [Add comparator] [Add expression]` + right-aligned
  row tools: enable/disable switch (tooltip "Disable"/"Enable"), copy icon ("Copy logic"),
  clipboard icon ("Paste logic after"), red trash.
  - Disabled row renders greyed out (kept but ignored).
- Timeframe chip per row: 1min, 3min, 5min, 10min, 15min, 30min, hour, day, week, month (default = chart
  timeframe "1min"). Greyed/disabled for time-only operands (e.g. Candle Time).
- Below rows: `+ and / or` (append a new row with joiner), `+ Group ( )` (append a nested group),
  `Copy All`, `Paste All`.
- Joiner pill between rows: "and" (blue) — click toggles to "or" (orange). Per-pair joiner, so mixed
  AND/OR chains are possible; grouping gives precedence.
- Group: indented block with a left border containing its own rows plus its own `+ and / or`,
  `+ Group ( )` (nesting) and its own enable/copy/paste/delete tools.
- "Add expression" opens popover **"Select Expression"** with × close, tabs **All / Indicators /
  Math Functions**, search "Search or enter a number...". Typing a number shows "Use number: 2.5".
  - The Advance catalogue is larger (~99 indicators + 8 math functions) — see 1.6.
  - Math Functions: Period min, Period max, Min, Max, Floor, Ceil, Abs, Brackets.
- After an operand is placed, hovering it shows a red × (remove operand) and a ⊕; clicking the ⊕ / blank
  area opens popover **"Math Operator"** with 4 tiles: + ADD, − SUBTRACT, × MULTIPLY, ÷ DIVIDE; choosing one
  immediately opens Select Expression for the next operand → `High(0) - Low(0)`.
- Operand tokens render as blue name + grey args, e.g. `High (0)`; clicking the name opens the same
  parameter dialog as Basic mode (High → only "offset"). Offset is how candle offsets like `High(-2)` are
  produced (offset −2).
- Function tokens render with slots: `Min( ⊕ ⊕ )`, `Period min( 14 , ⊕ )` (period number + expression),
  `Period max( 14 , Trange(0) ⊕ )`. Clicking a number inside a function opens a small inline number
  popover with spinner and "OK" button.
- "Add comparator" opens **"Select Comparator"** popover with search and the same 9 comparators,
  lower-case, in a different order: up by, equal to, crosses above, crosses below, lower than,
  higher than equal to, lower than equal to, higher than, down by. Chosen comparator renders orange.
- Numbers render blue (`2.5`).

![Select Expression – All](../screenshots/01-equity-builder/21-advance-select-expression-all.jpg)
![Math functions tab](../screenshots/01-equity-builder/22-advance-math-functions.jpg)
![Math operator popover](../screenshots/01-equity-builder/23-advance-math-operator-popover.png)
![Per-condition timeframe](../screenshots/01-equity-builder/24-advance-timeframe-chip.jpg)
![Select comparator](../screenshots/01-equity-builder/25-advance-select-comparator.jpg)
![Expression with OR joiner](../screenshots/01-equity-builder/26-advance-expression-and-or.png)
![Nested group](../screenshots/01-equity-builder/27-advance-group.png)
![Period min / Period max functions](../screenshots/01-equity-builder/28-advance-period-min-max.png)
![Inline number editor](../screenshots/01-equity-builder/29-advance-inline-number-editor.jpg)

### 1.6 Advance-only operands (not in the Basic dropdown) — parameters & defaults
| Operand | Token | Parameters (default; allowed) |
|---|---|---|
| Ulcer | Ulcer(14,Close,0) | period 14; field Close (Open/High/Low/Close); offset 0 |
| Choppiness Index Moving Average | Choppiness Index MA(14,simple,20,0) | period 14; type simple (MA11); ma period 20; offset |
| Open Interest Moving Average | OI MA(simple,20,0) | type simple (MA11); ma period 20; offset |
| Open interest | Open interest(0) | offset |
| Negative Volume Index | Negative Volume Index(close,100,simple,index,0) | field close (close/high/low/open); period 100; type simple (MA11); line index (index/ma); offset |
| Pvi | Positive Volume Index(close,100,simple,index,0) | same as NVI |
| Klinger Volume Oscillator | Klinger(13,34,55,klinger,0) | signal_period 13; short_cycle 34; long_cycle 55; type klinger (klinger/signal/histogram); offset |
| True Range Moving Average | TRange MA(14,true range,simple,20,0) | period 14; type "true range" (only option); ma type simple (MA11); ma period 20; offset |
| Central pivot range | Central pivot range(pp) | signal pp (pp/bc/tc) — works in Advance, invalid in Basic |
| Vortex Moving Average | Vortex MA(14,+vi,simple,20,0) | period 14; signal +vi (+vi/-vi); type simple (MA11); ma period 20; offset |
| Trade Volume Index | TVI(0.5,0) | min tick value 0.5; offset |
| Moving average volume | Moving average(volume,20,simple,0) | field volume (OHLCV); period 20; type simple (9-type list); offset |
| Cci Moving Average | Cci MA(20,simple,20,0) | period 20; type simple (MA11); ma period 20; offset |
| Stochastic Momentum Index | Stochastic Momentum Index(%k,10,3,3,10,0) | type %k (%k/%d); %k period 10; smoothing period 3; double smoothing period 3; %d period 10; offset |
| Moving Average Deviation | MA Dev(close,12,simple,point,0) | field close (OHLCV); period 12; type simple (MA11); pop point (point/percent); offset |
| Schaff Trend Cycle | Schaff(10,close,23,50,exponential,0) | period 10; field close (o/h/l/c); short_cycle 23; long_cycle 50; ma_type exponential (simple, exponential, weighted, triple, triangular, double, hull, kaufman, vidya); offset |
| Intraday Momentum Index | Imi(20,0) | period 20; offset |
| Intraday Momentum Index Moving Average | IMI MA(20,simple,20,0) | period 20; type simple (MA11); ma period 20; offset |
| Center Of Gravity | COG(10,Close,0) | period 10; field Close (o/h/l/c); offset |
| Bollinger %B Moving Average | Bollinger %B MA(Close,20,2,simple,simple,20,0) | field Close (C/O/H/L/V); Bb period 20; deviation 2; Bb type simple (MA11); ma type simple (MA11); ma period 20; offset |
| ATR Trailing Stoploss | ATR Trailing Stoploss(3, 21, 0) | multiplier 3; period 21; offset |
| Vwma | VWMA(close,20,0) | field close (o/h/l/c); period 20; offset |
| Elder Ray Index | Elder Ray(13,Bull,0) | period 13; Elder Power Bull (Bull/Bear); offset |
| Day High Low | DayHighLow(High) | option High (High/Low) — no offset |
| Vwap Standard deviation | Vwap StdDev(14,2,simple,0) | period 14; stddev 2; ma type simple (MA11); offset |
| Price Rate Of Change Moving Average | Proc MA(close,14,simple,20,0) | field close (o/h/l/c); period 14; type simple (MA11); ma period 20; offset |
| TriMA | TriMA(close,20,0) | field close (OHLCV); period 20; offset |
| Candle Time | Candle Time(minuteofcandle) | type minuteofcandle (minuteofcandle/hourofcandle/dayofweek); timeframe chip disabled |
| Period min / Period max | Period min(14, expr) | period 14 (inline number) + one expression slot |
| Min / Max | Min(expr, expr) | two expression slots |
| Floor / Ceil / Abs | Abs(expr) | one expression slot (assumed — not opened) |
| Brackets | ( expr ) | grouping parentheses for arithmetic precedence (assumed — not opened) |

Advance "All" list names (97 entries + 8 math functions) also include the Basic indicators under
slightly different labels (e.g. "T Range", "MOM", "LBB", "UBB", "Aroon-Up", "Aroon-Down", "Commodity
Channel Index", "Coppock curve", "Linear regression forecast", "Pivot points", "Nth candle").

## 2. Indicator parameter table (all 71 entries of the inline "Select Indicator" dropdown)

Method: each indicator was picked into a fresh, empty condition row (draft only), the auto-filled default
condition recorded, then the parameter dialog opened and every field / dropdown read, then Cancel.
Legend: **MA11** = MA-type list `simple, exponential, weighted, triple, triangular, double, hull, mesa,
variable, kaufman, vidya`; **OHLCV** = `close, open, high, low, volume` (order as shown).
Numeric fields are `<input type=number step=1>` with placeholder "Enter <label>"; no min/max attributes
and no inline validation (negative / empty accepted in the field). Every indicator except
Nth Candle / Opening Range / Pivot Points has a trailing **offset** (default 0; negative = N candles ago).

| # | Menu name | Token in row | Parameters (label: default; allowed) | Default condition when picked into an empty row |
|---|---|---|---|---|
| 1 | Simple Moving Average (SMA) | SMA | field: close (OHLCV); period: 9; offset: 0 | `SMA(close, 9, 0) crosses above SMA(close, 21, 0)` |
| 2 | Relative Strength Index (RSI) | RSI | period: 14; offset: 0 | `RSI(14,0) crosses above 30` |
| 3 | RSI Moving Average | RSI MA | period: 14; type: simple (MA11); ma period: 20; offset: 0 | `RSI MA(14,simple,20,0) crosses above RSI MA(14,simple,40,0)` |
| 4 | Exponential Moving Average (EMA) | EMA | field: close (OHLCV); period: 9; offset: 0 | `EMA(close, 9, 0) crosses above EMA(close, 21, 0)` |
| 5 | Weighted Moving Average (WMA) | WMA | field; period: 9; offset | `WMA(close, 9, 0) crosses above WMA(close, 21, 0)` |
| 6 | Double Exponential Moving Average (DEMA) | DEMA | field; period: 9; offset | `DEMA(close, 9, 0) crosses above DEMA(close, 21, 0)` |
| 7 | Triple Exponential Moving Average (TEMA) | TEMA | field; period: 9; offset | `TEMA(close, 9, 0) crosses above TEMA(close, 21, 0)` |
| 8 | Moving Average | Moving average | field: close (OHLCV); period: 9; type: hull (simple, exponential, weighted, triple, triangular, double, hull, variable, vidya — 9 options, no mesa/kaufman); offset: 0 | `Moving average(close,9,hull,0) crosses above Moving average(close,21,hull,0)` |
| 9 | Average Directional Index (ADX) | ADX | period: 14; offset: 0 | `ADX(14,0) crosses above 25` |
| 10 | Plus Directional Index (+DI) | Plus DI | period: 14; offset | `Plus DI(14,0) crosses above Minus DI(14,0)` |
| 11 | Minus Directional Index (-DI) | Minus DI | period: 14; offset | `Minus DI(14,0) crosses below Plus DI(14,0)` |
| 12 | ADX Moving Average | Adx MA | period: 14; type: adx (adx, plus di, minus di); ma type: simple (MA11); ma period: 9; offset | `Adx MA(14,adx,simple,9,0) crosses above Adx MA(14,adx,simple,21,0)` |
| 13 | Supertrend | Supertrend | period: 7; multiplier: 3; offset | `Supertrend(7,3,0) crosses below Close(0)` |
| 14 | Williams %R | Will%r | period: 14; offset | `Will%r(14,0) crosses above -80` |
| 15 | Upper Bollinger Bands | UBB | field: Close (OHLCV); period: 20; standard_deviations: 2; type: simple (MA11); offset | `UBB(Close,20,2,simple,0) crosses above Close(0)` |
| 16 | Lower Bollinger Bands | LBB | same as UBB | `LBB(Close,20,2,simple,0) crosses below Close(0)` |
| 17 | Middle Bollinger Bands | MBB | same as UBB | `MBB(Close,20,2,simple,0) crosses below Close(0)` |
| 18 | Volume Weighted Average Price (VWAP) | VWAP | offset: 0 | `VWAP(0) crosses below Close(0)` |
| 19 | VWAP Moving Average | VWAP MA | type: simple (MA11); ma period: 9; offset | `VWAP MA(simple,9,0) crosses above VWAP MA(simple,21,0)` |
| 20 | Volume | Volume | offset: 0 | `Volume(0) higher than 0` |
| 21 | Volume Oscillator | Vo | short: 12; long: 26; pop: pct (pct, point); offset | `Vo(12,26,pct,0) crosses above 0` |
| 22 | Vortex Indicator | Vortex | period: 14; signal: +vi (+vi, -vi); offset | `Vortex(14,+vi,0) crosses above Vortex(14,-vi,0)` |
| 23 | Money Flow Index (MFI) | MFI | period: 20; offset | `MFI(20,0) crosses above 20` |
| 24 | MFI Moving Average | Mfi MA | period: 20; type: simple (MA11); ma period: 9; offset | `Mfi MA(20,simple,9,0) crosses above Mfi MA(20,simple,21,0)` |
| 25 | Stochastic | Stochastic | period: 14; type: %k (%k, %d); smoothen: Yes (Yes, No); offset | `Stochastic(14,%k,Yes,0) crosses above Stochastic(14,%d,Yes,0)` |
| 26 | Stochastic RSI | Stochastic RSI | period: 14; type: %k (%k, %d); smoothen: Yes (Yes, No); offset | `Stochastic RSI(14,%k,Yes,0) crosses above 20` |
| 27 | Commodity Channel Index (CCI) | CCI | period: 20; offset | `CCI(20,0) crosses above 0` |
| 28 | Average True Range (ATR) | ATR | period: 14; offset | `ATR(14,0) higher than 5` |
| 29 | Awesome Oscillator | Awesome oscillator | offset | `Awesome oscillator(0) crosses above 0` |
| 30 | Choppiness Index | Choppiness Index | period: 14; offset | `Choppiness Index(14,0) crosses below 38` |
| 31 | Ultimate Oscillator | Ultimate Oscillator | length 1: 7; length 2: 14; length 3: 28; offset | `Ultimate Oscillator(7,14,28,0) crosses above 30` |
| 32 | Chaikin Money Flow Index | Chaikin MF | period: 20; offset (dialog title "Chaikin Money Flow") | `Chaikin MF(20,0) higher than 0` |
| 33 | Chande Momentum Oscillator | CMO | period: 9; offset | `CMO(9,0) higher than 0` |
| 34 | Bollinger Bandwidth | Bollinger Bandwidth | field: Close (OHLCV); period: 20; deviation: 2; type: simple (MA11); offset | `Bollinger Bandwidth(Close,20,2,simple,0) higher than 5` |
| 35 | Alligator | Alligator | signal: lips (jaw, teeth, lips); jaw period: 13; jaw offset: 8; teeth period: 8; teeth offset: 5; lips period: 5; lips offset: 3; offset: 0 | `Alligator(lips,13,8,8,5,5,3,0) higher than Alligator(teeth,13,8,8,5,5,3,0)` |
| 36 | Aroon Oscillator | Aroon oscillator | period: 14; offset | `Aroon oscillator(14,0) higher than 0` |
| 37 | Donchian Channel | Donchian channel | channel: upper (upper, lower, median); high_period: 20; low_period: 20; offset | `Donchian channel(upper,20,20,0) crosses below Close(0)` |
| 38 | Elder Force Index | Efi | period: 13; offset | `Efi(13,0) higher than 0` |
| 39 | High | High | offset: 0 | `High(0) crosses above High(-1)` |
| 40 | Ichimoku Cloud | Ichimoku | conversion: 9; base: 26; leading span b: 52; lagging span: 26; line: conversion (conversion, base, leading span a, leading span b); cloud shift: yes (yes, no); offset | `Ichimoku(9,26,52,26,conversion,yes,0) crosses below Close(0)` |
| 41 | Keltner Channel | Keltner | period: 50; band: top (top, median, bottom); ma type: exponential (MA11); shift: 5; offset | `Keltner(50,top,exponential,5,0) crosses below Close(0)` |
| 42 | Low | Low | offset | `Low(0) crosses above Low(-1)` |
| 43 | Median Price | Median Price | period: 14; offset | `Median Price(14,0) lower than Close(0)` |
| 44 | Median Price MA | Median Price MA | period: 14; type: simple (MA11); ma period: 20; offset | `Median Price MA(14,simple,20,0) crosses below Median price(14,0)` |
| 45 | Nth Candle | Nth candle | field: High (Open, High, Low, Close, Volume); range: 15min (min, 3min, 5min, 10min, 15min, 30min, hour); num: 1 — **no offset** | `Nth candle(High,15min,1) crosses below Close(0)` |
| 46 | Open | Open | offset | `Open(0) lower than Close(0)` |
| 47 | Opening Range | Opening Range | field: High (Open, High, Low, Close, Volume); range: 15min (min, 3min, 5min, 10min, 15min, 30min, hour) — **no offset** | `Opening Range(High,15min) crosses below Close(0)` |
| 48 | Pivot Points | Pivot point | signal: pp (r3, r2, r1, pp, s1, s2, s3); type: standard (standard, fibonacci); continous [sic]: No (no, yes) — **no offset** | `Pivot point(pp,standard,No) crosses below Close(0)` |
| 49 | Previous N | Prev N | field: Close (Open, High, Low, Close, Volume); offset: -1; candle: day (min, 3min, 5min, 10min, 15min, 30min, hour, day) | `Prev N(Close,-1,day) crosses below Close(0)` |
| 50 | Close | Close | offset | `Close(0) crosses above Close(-1)` |
| 51 | Moving Average Convergence/Divergence (MACD) | MACD | Fast_MA_period: 12; Slow_MA_period: 26; Signal_period: 9; type: macd (macd, signal, histogram); offset | `MACD(12,26,9,macd,0) crosses above MACD(12,26,9,signal,0)` |
| 52 | Parabolic SAR | Parabolic SAR | Minimum_AF: 0.02; Maximum_AF: 0.2; offset | `Parabolic SAR(0.02,0.2,0) crosses below Close(0)` |
| 53 | Momentum Indicator | Mom | period: 14; offset | `Mom(14,0) crosses above 0` |
| 54 | Trend Intensity Index | TII | field: Close (Open, High, Low, Close, Volume); type: TII (TII, Signal); period: 14; signal period: 9; offset | `TII(Close,TII,14,9,0) crosses above 20` |
| 55 | Bollinger %B | Bollinger %B | field: Close (Close, Open, High, Low, Volume); period: 20; deviation: 2; type: simple (MA11); offset | `Bollinger %B(Close,20,2,simple,0) higher than 50` |
| 56 | On balance Volume | OBV | offset | `OBV(0) higher than OBV(-1)` |
| 57 | Standard Deviation | StdDev | period: 14; field: close (open, high, low, close, volume); stddev: 2; ma type: simple (MA11); offset | `StdDev(14,close,2,simple,0) higher than 0` |
| 58 | Fractal Chaos Bands | Fractal chaos bands | type: upper (upper, lower); offset | `Fractal chaos bands(upper,0) lower than close(0)` |
| 59 | Linear Regression Forecast | Lrf | field: close (open, high, low, close — no volume); period: 14; offset | `Lrf(close,14, 0) crosses below close(0)` |
| 60 | Aroon Up | Aroon Up | period: 14; offset | `Aroon Up(14,0) higher than 70` |
| 61 | Aroon Down | Aroon Down | period: 14; offset | `Aroon Down(14,0) crosses above Aroon Up(14,0)` |
| 62 | Swing Index | Swing | limit move: 0.5; offset | `Swing(0.5,0) higher than 0` |
| 63 | Twiggs Money Flow | Twiggs | period: 21; offset | `Twiggs(21,0) higher than 0.2` |
| 64 | OBV Moving Average | Obv Ma | ma type: simple (MA11); ma period: 20; offset | `Obv Ma(simple,20,0) crosses below Obv(0)` |
| 65 | True Range | Trange | offset | `Trange(0) crosses above 20` |
| 66 | Price Rate Of Change | Proc | field: close (open, high, low, close); period: 14; offset | `Proc(close,14,0) crosses above 0` |
| 67 | Ehler Fisher | Ehler Fisher | period: 10; line: ef (ef, trigger); offset | `Ehler Fisher(10,ef,0) crosses above Ehler Fisher(10,trigger,0)` |
| 68 | Time Series Forecast (TSF) | TSF | period: 14; field: Close (Open, High, Low, Close); offset | `TSF(14,Close,0) crosses below Close(0)` |
| 69 | Coppock Curve | Coppock curve | field: close (open, high, low, close); period: 10; sroc: 11; lroc: 14; offset | `Coppock curve(close,10,11,14,0) crosses above 0` |
| 70 | ATR Bands | ATR Bands | period: 5; shift: 2.5; field: Close (Open, High, Low, Close); band: top (top, bottom); offset | `ATR Bands(5,2.5,Close,top,0) crosses below Close(0)` |
| 71 | Central Pivot Range | — | **Not selectable in equity builder**: picking it shows a red toast "ERROR · Invalid indicator" and the row stays empty (search text left in the box). Probably options-only / broken. | — |

Additional entry only in the See All modal: **ATR Trailing Stoploss** — `ATR Trailing Stoploss(3, 21, 0)` (likely multiplier 3, period 21, offset 0 — dialog not opened).

Observations for the rebuild:
- Token naming is inconsistent (mixed case, abbreviations: Will%r, Efi, Proc, Trange, Vo, Obv Ma) — a
  desktop clone should keep a display name, a short token, and a canonical parameter schema per indicator.
- Parameter labels are raw identifiers (Fast_MA_period, standard_deviations, Minimum_AF, "continous").
- Field enumerations vary (some include volume, some not; capitalisation differs per indicator).
- Default condition templates are indicator-specific (threshold for oscillators, crossover of two periods
  for MAs, "crosses below Close(0)" for bands/levels).

![Central Pivot Range invalid](../screenshots/01-equity-builder/13-cpr-invalid-indicator.png)

## 3. "See All" → "Add Prebuild Suggestions" modal

**Feature description:** A large browsing dialog that lets the user pick ready-made single conditions
(e.g. "RSI(14,0) crosses above 30") grouped by indicator, instead of composing each operand by hand.
Ticking suggestions and pressing Done adds them as condition rows.

![See All modal (Bullish, SMA selected)](../screenshots/01-equity-builder/10-see-all-prebuild-suggestions-modal.jpg)

- Trigger: "See All" link at the bottom of any "Select Indicator" dropdown.
- Header: title "Add Prebuild Suggestions"; A–Z letter strip (filters indicators by first letter;
  selected letter bold).
- Search box "Search for conditions..." — filters across both indicator list and suggestion text
  (e.g. "cross" → Indicators (37), Suggestions (78)).
- Bullish / Bearish segmented toggle (single-select; Bullish default). Bullish set = 71 indicators /
  335 suggestions; Bearish set = 69 indicators / 331 suggestions (Lower Bollinger Bands and High have
  no bearish suggestions).
- Left pane "Indicators (N)": category chip row with horizontal scroll (») —
  All, overlap studies, oscillators, volume indicators, volatility indicators, price action,
  volatility indicator, momentum indicators, volume & momentum, volume trend, momentum, trend,
  volatility (note duplicated/inconsistent categories in source data). Each card: short name (bold)
  + long name; selected card highlighted with primary border.
- Right pane "Prebuild Suggestions (N)" + "Tags" column: rows = checkbox + condition text + grey tag
  pill (e.g. Scalping, Trend Following, Longterm, Golden Cross, Oversold Reversal… ~139 distinct,
  free-text). Conditions already present in the strategy appear pre-ticked (row 1 SMA 9/21 was ticked).
- Footer: primary "Done". Clicking the backdrop closes without applying (observed).
- The modal list contains "ATR Trailing Stoploss" (not in the inline dropdown) and lacks
  "Central Pivot Range" (which is in the dropdown) → modal has 71 entries, dropdown has 71 different-ish entries.

![Letter filter M, Bearish](../screenshots/01-equity-builder/11-see-all-letter-filter-bearish.jpg)
![Search "cross"](../screenshots/01-equity-builder/12-see-all-search-conditions.jpg)

### 3.1 Suggestions per indicator (Bullish set: count + first suggestion — reveals canonical syntax)
| Indicator | # | First suggestion |
|---|---|---|
| SMA | 11 | SMA(close, 9, 0) crosses above SMA(close, 21, 0) |
| RSI | 12 | RSI(14,0) crosses above 30 (also Stochastic RSI items listed under RSI) |
| RSI Moving Average | 1 | RSI MA(14,simple,20,0) crosses above RSI MA(14,simple,40,0) |
| EMA | 36 | EMA(close, 9, 0) crosses above EMA(close, 21, 0) |
| WMA | 12 | WMA(close, 9, 0) crosses above WMA(close, 21, 0) |
| DEMA | 12 | DEMA(close, 9, 0) crosses above DEMA(close, 21, 0) |
| TEMA | 12 | TEMA(close, 9, 0) crosses above TEMA(close, 21, 0) |
| Moving Average | 12 | Moving average(close,9,hull,0) crosses above Moving average(close,21,hull,0) |
| ADX | 5 | ADX(14,0) crosses above 25 |
| Adx Moving Average | 1 | Adx MA(14,adx,simple,9,0) crosses above Adx MA(14,adx,simple,21,0) |
| Supertrend | 2 | Supertrend(7,3,0) crosses below Close(0) |
| Williams %R | 3 | Will%r(14,0) crosses above -80 |
| Lower Bollinger Bands | 6 | LBB(Close,20,2,simple,0) crosses below Close(0) |
| Middle Bollinger Bands | 4 | MBB(Close,20,2,simple,0) crosses below Close(0) |
| Upper Bollinger Bands | 1 | UBB(Close,20,2,simple,0) crosses above Close(0) |
| Aroon Oscillator | 2 | Aroon oscillator(14,0) higher than 0 |
| VWAP | 4 | VWAP(0) crosses below Close(0) |
| Vwap Moving Average | 1 | VWAP MA(simple,9,0) crosses above VWAP MA(simple,21,0) |
| CCI | 3 | CCI(20,0) crosses above 0 |
| Stochastic | 11 | Stochastic(14,%k,Yes,0) crosses above Stochastic(14,%d,Yes,0) |
| Volume | 4 | Volume(0) higher than 0 |
| Vortex Indicator | 4 | Vortex(14,+vi,0) crosses above Vortex(14,-vi,0) |
| Money Flow Index | 6 | MFI(20,0) crosses above 20 |
| MFI Moving Average | 1 | Mfi MA(20,simple,9,0) crosses above Mfi MA(20,simple,21,0) |
| Previous N | 8 | Prev N(Close,-1,day) crosses below Close(0) (variants High/Volume; vs Close/High/Volume) |
| Opening Range | 6 | Opening Range(High,15min) crosses below Close(0) (15min and 30min variants) |
| Nth Candle | 5 | Nth candle(High,15min,1) crosses below Close(0) |
| Pivot Points | 8 | Pivot point(pp,standard,No) crosses below Close(0) (pp/r1/r2/r3 variants) |
| Median Price | 2 | Median Price(14,0) lower than Close(0) |
| Median Price Moving Average | 1 | Median Price MA(14,simple,20,0) crosses below Median price(14,0) |
| ATR Trailing Stoploss | 1 | ATR Trailing Stoploss(3, 21, 0) crosses below Close(0) |
| Close | 7 | Close(0) crosses above Close(-1) |
| High | 2 | High(0) crosses above High(-1) |
| Open | 7 | (data bug: shows Opening Range items) |
| Low | 2 | Low(0) crosses above Low(-1) |
| Alligator | 5 | Alligator(lips,13,8,8,5,5,3,0) higher than Alligator(teeth,13,8,8,5,5,3,0) |
| Donchian Channel | 4 | Donchian channel(upper,20,20,0) crosses below Close(0) |
| Keltner Channel | 4 | Keltner(50,top,exponential,5,0) crosses below Close(0) |
| Volume Oscillator | 18 | (shows Volume items — data bug) |
| Choppiness Index | 2 | Choppiness Index(14,0) crosses below 38 |
| Ichimoku Cloud | 4 | Ichimoku(9,26,52,26,conversion,yes,0) crosses below Close(0) |
| ATR | 7 | ATR Trailing Stoploss(3, 21, 0) crosses below Close(0) |
| Awesome Oscillator | 2 | Awesome oscillator(0) crosses above 0 |
| Bollinger Bandwidth | 2 | Bollinger Bandwidth(Close,20,2,simple,0) higher than 5 |
| Chaikin Money Flow | 2 | Chaikin MF(20,0) higher than 0 |
| Chande Momentum Oscillator | 3 | CMO(9,0) higher than 0 |
| Elder Force Index | 3 | Efi(13,0) higher than 0 |
| Ultimate Oscillator | 2 | Ultimate Oscillator(7,14,28,0) crosses above 30 |
| MACD | 5 | MACD(12,26,9,macd,0) crosses above MACD(12,26,9,signal,0); also histogram>0, macd>0 |
| +DI | 2 | Plus DI(14,0) crosses above Minus DI(14,0) |
| -DI | 2 | Minus DI(14,0) crosses below Plus DI(14,0) |
| OBV | 5 | OBV(0) higher than OBV(-1) |
| Stochastic RSI | 6 | Stochastic RSI(14,%k,Yes,0) crosses above 20 |
| Parabolic SAR | 2 | Parabolic SAR(0.02,0.2,0) crosses below Close(0) |
| True Range | 2 | Trange(0) crosses above 20 |
| Bollinger %B | 2 | Bollinger %B(Close,20,2,simple,0) higher than 50 |
| Aroon Up | 1 | Aroon Up(14,0) higher than 70 |
| Aroon Down | 1 | Aroon Down(14,0) crosses above Aroon Up(14,0) |
| Linear Regression Forecast | 2 | Lrf(close,14, 0) crosses below close(0) |
| Standard Deviation | 2 | StdDev(14,close,2,simple,0) higher than 0 |
| Trend Intensity Index | 4 | TII(Close,TII,14,9,0) crosses above 20 |
| Momentum Indicator | 2 | Mom(14,0) crosses above 0 |
| Price Rate Of Change | 3 | Proc(close,14,0) crosses above 0 |
| Fractal Chaos Bands | 3 | Fractal chaos bands(upper,0) lower than close(0) |
| Swing Index | 2 | Swing(0.5,0) higher than 0 |
| Twiggs Money Flow | 2 | Twiggs(21,0) higher than 0.2 |
| OBV Moving Average | 4 | Obv Ma(simple,20,0) crosses below Obv(0) |
| Ehler Fisher | 2 | Ehler Fisher(10,ef,0) crosses above Ehler Fisher(10,trigger,0) |
| Coppock Curve | 2 | Coppock curve(close,10,11,14,0) crosses above 0 |
| Time Series Forecast | 2 | TSF(14,Close,0) crosses below Close(0) |
| ATR Bands | 4 | ATR Bands(5,2.5,Close,top,0) crosses below Close(0) |

Bearish set mirrors these with "crosses below"/"lower than" (e.g. `SMA(close, 9, 0) crosses below SMA(close, 21, 0)`, `RSI(14,0) crosses below 50`).
UX notes: inconsistent casing in generated text (close vs Close, "Moving average" vs "Moving Average"),
double spaces, a few wrong mappings (Open, Volume Oscillator, ATR) — a rebuild should generate suggestion
text from one canonical formatter.


## 4. Entry card controls

### 4.1 Buy / Sell toggle
**Feature description:** Chooses whether the entry signal opens a long (Buy) or short (Sell) position.
- Pill switch top-right of Entry card: blue "Buy" (knob right) ↔ orange-red "Sell" (knob left).
- Only the pill changes; conditions/exit card are unchanged (no automatic comparator inversion).

![Sell state](../screenshots/01-equity-builder/31-entry-sell-toggle.jpg)

### 4.2 Instruments
- Avatar stack of instrument logos (+N overflow, e.g. "+10") and "+ Add Instruments" button (dialog
  documented in the inventory: max 50, universes NIFTY50/100/200/500/BANKNIFTY, watchlists).
- A brand-new draft is pre-seeded with 4 instruments (SBIN, HDFCBANK, ADANIENT, APOLLOHOSP); the right-hand
  chart shows one tab per instrument (scrollable with » when many).

### 4.3 Chart Settings
**Feature description:** Sets the candle type and base timeframe the strategy is evaluated on.
- "Candlestick" — rendered as a button but clicking does nothing (no menu); only candlestick available.
- Timeframe dropdown: 1 Minute, 3 Minutes, 5 Minutes, 10 Minutes, 15 Minutes, 30 Minutes, 1 Hour, 1 Day
  (default 1 Minute). Changing it re-renders the chart (e.g. 1 Day shows ~6 months). Nothing else in the form
  changes (no intraday/positional switch appears).
- Advance rows have their own timeframe chip (adds week, month) which defaults to "1min".

![Timeframe dropdown](../screenshots/01-equity-builder/30-chart-timeframe-dropdown.png)

## 5. Assist
**Feature description:** Quick way to add ready-made conditions.
- Clicking "Assist ✨" (Basic mode only) opens the **same "Add Prebuild Suggestions" modal** as "See All"
  (section 3) — there is no free-text AI prompt UI in this build. Hidden in Advance mode.

![Assist opens suggestions modal](../screenshots/01-equity-builder/09-assist-opens-suggestions.jpg)

## 6. Exit card ("Exit (for individual instrument)")
**Feature description:** Defines how each open position is closed: fixed stop-loss, profit target and/or
trailing stop, optionally plus indicator-based exit conditions.
- Three numeric inputs (placeholder 0.00, spinner): Stop loss, Target profit, Trailing stop loss.
- Unit radio (applies to all three): ◉ %Percentage (suffix "%"), ○ Points (suffix "pts"), ○ PNL (suffix "₹").
  Default %Percentage.
- Typing "-5" into Stop loss is rejected silently (field stays empty) → negatives blocked; no inline messages.
- "Add Exit Conditions (Optional)" expands a second full Conditions block (Basic/Advance toggle, Assist,
  If-row, Pre built Setups, + Add Another Condition) and the button becomes red-outlined
  "Remove Exit Conditions". The left pane then scrolls internally.
- Saved example (prebuilt copy): Stop loss 4 %, Target 6 %, Trailing blank.

![Exit PNL units](../screenshots/01-equity-builder/32-exit-pnl-units.png)
![Exit conditions expanded](../screenshots/01-equity-builder/33-exit-conditions-added.jpg)

## 7. Save Strategy dialog (where intraday vs positional lives)
**Feature description:** Names and categorises the strategy before it is stored; the category decides
whether it is treated as an intraday (MIS-like) or swing/positional (CNC-like) strategy.
- Opened by clicking the strategy title ("Untitled Strategy") — and presumably by the bottom "Save" button
  (not pressed).
- Fields: name ("Enter a name to save"), **Category**: "Equity Intraday" (orange, default-selected) /
  "Equity Swing" (purple) chips; **Tags** input (0/5, "Press Enter to add the Tag*"); **Description** textarea.
- Footer: a 2-step progress indicator (dot + bar, step 2 of 2? — clicking does nothing), Cancel, Save.
- No quantity, capital, date-range, entry/exit time or charges fields exist anywhere in the builder or this
  dialog. These were not found on the backtest page either (only a "Trading Charges" toggle). Backtest
  period appears fixed (~3 months: 13 Jul 2026 – 11 Oct 2026 in all observed results) and quantity appears
  fixed by the platform (trade log shows qty, e.g. 2). Intraday square-off at 15:15 (trigger "SO").

![Save dialog – Intraday](../screenshots/01-equity-builder/34-save-strategy-dialog-intraday.jpg)
![Save dialog – Swing selected](../screenshots/01-equity-builder/35-save-strategy-dialog-swing.png)

## 8. Saved strategy in the builder — `/strategy/<slug>` (own strategy)
- Title shown lower-case slug-ish ("zz-research-copy") + description line; header right: My Strategies,
  ☆ favourite, ⋮ menu (Delete, Copy).
- Basic rows from a copied prebuilt strategy show expression operands (`(High(-2) - Low(-2))`,
  `Volume(-3) * 3.6`) plus a **4th column per row with the timeframe ("15min")** — i.e. Basic mode can carry
  per-row timeframe for imported logic. Clicking that chip turned it into an editable token and raised the
  "ERROR · Invalid indicator" toast (bug).
- Footer buttons: "Save" (outline) and "Execute" (primary; presumably saves + runs backtest — not pressed).

![Saved strategy in builder](../screenshots/01-equity-builder/45-saved-strategy-in-builder.jpg)
![Overflow menu](../screenshots/01-equity-builder/46-saved-strategy-overflow-menu.png)

## 9. My Strategies — `/my-strategies`
**Feature description:** The user's library of saved strategies with a quick backtest summary of the
selected one and entry points to edit, copy, backtest or deploy it.

![My Strategies](../screenshots/01-equity-builder/40-my-strategies-list.jpg)

- Header: "← My Strategies", subtitle "View and manage all the strategies you have created across equity and
  options."; right: "Create Strategy" (outline), "Discover Strategies" (primary).
- Left pane: segmented filter tabs **All | Equity Swing | Equity Intraday | Option Buying | Option Selling**;
  search "Search for Strategies"; filter icon (≡).
  - Row: strategy icon, name, ☆ favourite toggle, complexity badge ("Basic"), timeframe badge ("15m…",
    truncated), ⋮ menu → **Delete, Edit, Copy**. Selected row highlighted light blue. No columns/sorting UI.
- Filter popover "Filters": left categories **Trend** (Bullish, Bearish) and **Technicals** (Momentum,
  Trend Following, Reversal, Breakout, Breakdown, Trend Confirmation, Uptrend, Downtrend, Momentum Shift,
  Crossover, Price Action); checkboxes; "Apply" button (disabled until a change).
- Right pane (selected strategy): name, "Copy", "✎ Edit Strategy", description; "Backtest Results"
  (date range) + "View Full Backtest Results →"; instrument chip carousel (Cumulative + each symbol LTP/%);
  metrics row (Total P&L, W/L Count, Trades, W/L Streak, Max Gain, Max Loss, Winning Avg., Losing Avg.);
  Hypothetical Daily Summary table; Hypothetical Monthly Performance grid; sticky footer
  "Run Backtest →" (outline) and "🔔 Notification Based Trading" (primary).
- Own-strategy full backtest page `/backtest/<slug>`: same as the prebuilt one documented in the inventory,
  but header actions are "Edit ✎", "Run Backtest", "Notification Based Trading" (no Copy). Per-instrument
  metrics include Max DD; "Trading Charges" toggle top-right.

![Row menu](../screenshots/01-equity-builder/41-my-strategies-row-menu.jpg)
![Filters – Trend](../screenshots/01-equity-builder/42-my-strategies-filter-trend.jpg)
![Filters – Technicals](../screenshots/01-equity-builder/43-my-strategies-filter-technicals.png)
![Own backtest results](../screenshots/01-equity-builder/44-own-backtest-results.jpg)

## 10. Validation / error states observed
- "ERROR · Invalid indicator" red toast (top-centre) when choosing Central Pivot Range in Basic mode, and when
  clicking the per-row timeframe chip of a saved strategy.
- Parameter dialogs: no inline validation; empty and negative numbers accepted in the field.
- Exit inputs: negative values silently rejected.
- Switch Mode confirm dialogs (Basic↔Advance) warn conditions will be reset.
- Not observed (would require pressing Save): messages for empty conditions / no instruments / missing name.

## 11. UX observations
Good:
- Picking an indicator yields a complete, sensible default condition — zero-to-valid in one click.
- Advance mode is a genuinely expressive builder (arithmetic, functions, nested groups, AND/OR, per-row
  timeframe, disable/copy/paste rows) while keeping Basic simple.
- Suggestion modal with A–Z, category chips, Bullish/Bearish and full-text search is a fast discovery tool.
Bad:
- Mode switch destroys conditions instead of converting them.
- "Assist" label promises AI but opens the static suggestions modal.
- Inconsistent naming/casing of tokens and parameter labels (Fast_MA_period, continous, Will%r, Efi).
- Central Pivot Range listed but invalid in Basic; suggestion data bugs (Open, Volume Oscillator, ATR).
- No visible quantity / capital / backtest-period / trading-hours controls → users cannot tune backtests.
- Escape doesn't close parameter dialogs; Candlestick looks like a button but is inert.
- Clicking the title opens the Save dialog (surprising; expected inline rename).

## 12. Open questions
- What the bottom "Save" does beyond the Save Strategy dialog (second step? auto-backtest?), and its
  validation messages — not pressed by rule.
- What "Execute" and "Run Backtest" open (parameter dialog for date range/qty/charges, or immediate run).
- Whether quantity/capital can be configured anywhere (maybe in deploy flow) — not explored (deploy forbidden).
- Exact semantics of "Up by / Down by" comparators (percent or absolute?) and of Pivot Points "continous".
- Params of Floor/Ceil/Abs/Brackets/Min/Max beyond slot structure; ATR Trailing Stoploss param semantics.
- Whether Basic rows can be switched to OR (no UI seen) and the meaning of the Basic 4th "15min" column.
- My Strategies sorting (none visible) and behaviour with many strategies (only 1 strategy existed).
- One of my tabs (builder draft) could not be closed: closing appears blocked by the browser's
  "Leave site?" prompt; the user may need to close it manually (draft unsaved, nothing persisted).
