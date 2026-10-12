# Functional Requirements — Part 02: Equity builder (EQB), Options builder (OPT), Discover (DISC), My Strategies / My Scanners (MYS)

Status: DRAFT · Date: 2026-10-11 · Inputs: `docs/requirements/REQUIREMENTS.md` §1–5 (scope, C-01..C-12, NFR-01..14),
`docs/research/FEATURES.md` (modules EQB, OPT, DISC, MYS; Appendices A, B, C, D, E, F, G, H, I, K, M, N; §3 cross-cutting; §6 conflicts).

Conventions
- "The system" = the local Streak-equivalent desktop app (Go + Wails v2, React UI, Kite Connect via `github.com/zerodha/gokiteconnect/v4`).
  Kite dependencies are named by REST endpoint (Appendix L); the wrapper method used is a design decision.
- All UI copy in quotes is **our own** wording and is illustrative; final copy is set in the UX spec. No Streak branding or text is reused.
- Times are IST. Dates are shown DD-MM-YYYY, money with ₹ and Indian digit grouping (NFR-11).
- "Library" = the app's own bundled, versioned catalogue of ready-made strategies, scanners, suggestions and option templates, authored
  by us from generic, well-known trading setups. The FEATURES.md lists are coverage references, not content to copy.
- Feature IDs from other modules (WATCH-, BT-, DEP-, SCAN-, SHELL-, CHART-, ORD-) are referenced by feature ID; their FRs live in other parts.
- Acceptance criteria use Given/When/Then and are written to be automatable (Go unit/integration tests, or UI tests via the Wails frontend).
- "Deviation from Streak" records where behaviour intentionally differs (platform constraint C-xx, NFR, or a §3.2 bug not replicated).

---

## FR module EQB — Equity strategy builder

### FR-EQB-001 Equity builder workspace layout
- **Source features:** EQB-001
- **Requirement:** The system shall provide an Equity Strategy Builder view, reachable from the sidebar ("Equity strategy") and from
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
- **Requirement:** The system shall persist every strategy (equity and options) and every scanner condition set in a single,
  documented, versioned JSON model, published as a JSON Schema at `docs/spec/strategy-schema-v1.json` and described in
  `docs/spec/strategy-dsl.md`. The model shall contain at minimum:
  ```
  StrategyDoc  { schema: "sd.strategy", version: "<semver>", id: uuid, kind: "equity"|"options",
                 meta: { name, category, tags[], description, favourite, createdAt, updatedAt,
                         origin: { type: "user"|"library"|"import", libraryId?, libraryVersion? } },
                 equity?: EquityBody, options?: OptionsBody (FR-OPT-002) }
  EquityBody   { direction: "buy"|"sell", instruments: [InstrumentRef],
                 chart: { type: "candlestick", timeframe: TF },
                 entry: ConditionBlock, exit: { risk: Risk, conditions?: ConditionBlock } }
  InstrumentRef{ exchange, tradingsymbol, instrumentToken? }      // token is a cache; re-resolved from the instruments dump
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
  1. Given the JSON Schema, when every library strategy, scanner and template shipped with the app is validated, then all validate with zero errors (CI test).
  2. Given 1,000 randomly generated valid docs (property test), when each is formatted to text and parsed back, then the result is deep-equal to the original.
  3. Given a doc with `version` "1.x" saved by an earlier build, when loaded by a build with schema 2.0 and a registered 1→2 migration, then it loads, and re-saving writes version 2.0.
  4. Given a doc with version "9.0.0", when imported, then import fails with a message stating the file's version and the highest supported version, and nothing is written to the store.
  5. Given a condition with `offset: 1`, when validated, then validation fails with error code `OFFSET_POSITIVE` pointing at that operand.
- **Data/Kite dependency:** Local only (SQLite; NFR-03 versioned migrations).
- **Deviation from Streak:** Streak's internal format is not observable; ours is documented, versioned and round-trippable (clean-room design).
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
  were never explicitly changed), and determines the default backtest lookback (BT-006). Candles align to the 09:15 IST session start.
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
  condition"); picking into a row that already has a comparator/RHS replaces only the LHS. Incomplete rows are shown with an
  "incomplete" marker; they are excluded from evaluation and block Execute (FR-EQB-037).
- **Acceptance criteria:**
  1. Given an empty row, when "Simple Moving Average" is picked as LHS, then the row reads `SMA(close, 9)[0] crosses above SMA(close, 21)[0]` (canonical form of the Appendix A template).
  2. Given a row `Supertrend(7, 3)[0] crosses below Close[0]`, when the LHS is changed to RSI, then the row reads `RSI(14)[0] crosses below Close[0]`.
  3. Given a row with no LHS, then the comparator and RHS controls are disabled and announce "Choose an indicator first" to assistive tech.
  4. Given three rows where row 2 is incomplete, when Execute is pressed, then Execute is blocked and row 2 is highlighted with "Incomplete condition".
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
- **Requirement:** The system shall offer 9 comparators (Appendix C) in both modes with identical semantics, evaluated at evaluation
  time t on the row's timeframe, where L and R are the evaluated LHS/RHS values:
  | Comparator | Rule |
  |---|---|
  | equal to | \|L − R\| ≤ 1e-9 × max(1, \|L\|, \|R\|) |
  | higher than | L > R |
  | higher than or equal to | L ≥ R |
  | lower than | L < R |
  | lower than or equal to | L ≤ R |
  | crosses above | L[t−1] ≤ R[t−1] and L[t] > R[t] |
  | crosses below | L[t−1] ≥ R[t−1] and L[t] < R[t] |
  | up by v (unit) from | pct: L ≥ R × (1 + v/100); pts: L ≥ R + v |
  | down by v (unit) from | pct: L ≤ R × (1 − v/100); pts: L ≤ R − v |
  If any required value (including the t−1 values for crosses) is undefined (insufficient history, division by zero, missing OI),
  the condition is false. One ordering is used everywhere: equal to, crosses above, crosses below, higher than, higher than or equal to,
  lower than, lower than or equal to, up by, down by. The comparator control is disabled until an LHS exists.
- **Acceptance criteria:**
  1. Given golden fixture series L=[1,2,3], R=[2,2,2], then crosses above is false at t=1 (L=2=R) and true at t=2; higher than is false at t=1.
  2. Given L=[3,2], R=[2,2], then crosses below is false at t=1 (L=R is not "below").
  3. Given L=101, R=100, up by 1 pct, then true; given L=100.99, then false.
  4. Given L undefined at t−1, then crosses above is false at t regardless of L[t], R[t].
  5. Given the Basic and Advanced comparator lists, then both show the same 9 items in the same order.
- **Data/Kite dependency:** Local rule engine.
- **Deviation from Streak:** Streak's docs say "Higher than" counts equality as true; we make "higher than" strict and keep a separate "higher than or equal to" so the two are distinguishable (see open question Q-03). Ordering is unified across modes.
- **Priority hint:** Core

### FR-EQB-013 "Up by" / "Down by" parameters
- **Source features:** EQB-011
- **Requirement:** Choosing "up by"/"down by" shall open a dialog with Value (decimal > 0, required) and Unit (Percent default, Points).
  The comparator cell renders inline, e.g. `up by 1% from` / `down by 12 pts from`. Clicking the cell reopens the dialog. Save is
  enabled only for valid values; Escape cancels.
- **Acceptance criteria:**
  1. Given a row with comparator "up by", when value 1 and unit Percent are saved, then the row renders `Close[0] up by 1% from Close[-1]` and JSON has `{op:"up_by", value:1, unit:"pct"}`.
  2. Given value 0, then Save is disabled with "Must be greater than 0".
  3. Given unit Points and value 5, then evaluation uses L ≥ R + 5 (unit test).
- **Data/Kite dependency:** Local only.
- **Deviation from Streak:** Adds a Points unit (Streak showed only % — EQB-011 open question).
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
  4. Given the Indicators tab, then the count equals the operand catalogue size (single source of truth; CI test).
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
- **Requirement:** Each condition carries a timeframe from {1m, 3m, 5m, 10m, 15m, 30m, 1h, 1d, 1w, 1M} (default = base timeframe).
  The chip is disabled for conditions whose operands are all time-only (Candle Time). Evaluation rules: (1) the strategy is evaluated
  at evaluation instants E; (2) a condition on timeframe T at instant e uses the most recent T-candle whose close time ≤ e (never a
  forming candle); (3) weekly candles span Monday–Friday trading days and close when the week's last trading session closes; monthly
  likewise per calendar month; both are resampled locally from daily candles; (4) E = every base-timeframe candle close, except when
  some condition timeframe (intraday, < 1d) is neither a multiple nor a divisor of the base timeframe, in which case E = closes aligned
  to LCM(base, all such timeframes) measured from 09:15; the builder shows an inline note stating the effective evaluation interval;
  (5) warm-up for higher timeframes is loaded automatically (FR-EQB-041); when unavailable, the condition is undefined (false) and the
  backtest report lists a warm-up warning.
- **Acceptance criteria:**
  1. Given base 5m and a 1h condition, when evaluating at 10:20, then the 1h value is from the 09:15–10:15 candle.
  2. Given base 3m and a 5m condition, then E = 09:30, 09:45, … (every 15 minutes) and the builder shows "Evaluated every 15 minutes".
  3. Given a 1w condition evaluated on Wednesday, then it uses the previous completed week's candle.
  4. Given a Candle Time-only condition, then its timeframe chip is disabled.
  5. Given EMA(50) on 1M with only 3 years of daily data, then the condition is undefined until 50 monthly candles exist and the backtest report shows a warm-up warning.
- **Data/Kite dependency:** Kite historical candles via local store; local resampling to week/month; local scheduler for live evaluation (DEP module).
- **Deviation from Streak:** none intended (parity with documented MTF/LCM rules; Streak bug §3.2 #2 not replicated).
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
- **Deviation from Streak:** Precedence documented explicitly (Streak's is unknown — open question Q-04).
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
  target (e.g. option-only operands in an equity strategy) are flagged after paste.
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

### FR-EQB-032 Footer actions: Save and Execute
- **Source features:** EQB-030
- **Requirement:** The footer shall show **Save** (new: opens FR-EQB-004; saved: saves in place) and, once saved, **Execute**, which
  opens the execution panel with two modes: **Backtest** (BT-001) and **Signals / Notification-based trading** (DEP-001). Execute is
  disabled with a reason list when the strategy is incomplete (FR-EQB-037). Starting a backtest from a dirty strategy saves it first
  (success toast), then runs.
- **Acceptance criteria:**
  1. Given a new draft, then only Save is shown.
  2. Given a saved, complete strategy, when Execute is pressed, then a panel offering Backtest and Signals opens.
  3. Given unsaved edits, when a backtest is started, then the strategy's `updatedAt` changes before the backtest record's `createdAt`.
- **Data/Kite dependency:** Local; downstream BT/DEP modules.
- **Deviation from Streak:** none.
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
  reference); (2) MA-type enumerations (Appendix A "MA11" and the 9-type list) map to defined algorithms (simple, exponential, weighted,
  triple-exponential, triangular, double-exponential, Hull, MESA adaptive, variable, Kaufman adaptive, VIDYA) — see Q-05;
  (3) session-anchored operands (VWAP and its MA/StdDev, Opening Range, Nth Candle, Day High Low, Previous N with candle=day, Pivot
  Points, Central Pivot Range, Intraday Momentum Index) reset/anchor at the 09:15 IST session start using the previous session's
  daily OHLC where applicable; (4) Candle Time returns minute-of-hour (0–59), hour-of-day (0–23) or day-of-week (1 = Monday … 7 =
  Sunday) of the candle's start time; (5) Open interest operands require candles fetched with `oi=1` and are undefined elsewhere;
  (6) **golden test vectors (NFR-06):** a fixture set (≥ 3 instruments, ≥ 1,000 candles each across 1m, 15m and 1d, including gaps, a
  zero-volume candle, a flat run and a limit move) is committed with expected outputs for every operand × every enumerated option
  value (each MA type, each line/band/signal selector, each field) at default and at one non-default parameter set; expected outputs are
  produced by an independent reference implementation (a pinned-version open-source TA library where the indicator exists there, or an
  independently written spreadsheet/Python implementation reviewed against the formula spec otherwise); CI fails if any output differs
  by more than max(1e-6 absolute, 1e-6 relative) after the declared warm-up; (7) computation is incremental-capable (append one candle
  without full recompute) and produces identical results to full recompute.
- **Acceptance criteria:**
  1. Given the fixture set, when the golden-vector suite runs in CI, then 100% of operand × option cases pass within tolerance, and the suite fails if any Appendix A/B operand lacks a vector (coverage check against the catalogue).
  2. Given SMA(close, 9) on fixture F1 daily, then values match the reference at every index ≥ 8 within tolerance.
  3. Given VWAP on 1m fixture spanning two sessions, then VWAP resets at the first candle of the second session.
  4. Given incremental append of 500 candles one by one, then outputs equal the full-recompute outputs exactly (bitwise for deterministic float order).
  5. Given Candle Time(day of week) on a Wednesday candle, then 3.
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
  1. Given the catalogue, when a CI check runs, then every Appendix A operand (71) and every Appendix B operand is present with a parameter schema whose defaults equal the appendix defaults.
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
  false conditions; be deterministic (same inputs → same outputs, independent of goroutine scheduling); evaluate one instrument ×
  one instant in < 1 ms for a 10-condition block on a 4-core laptop (supports NFR-07/NFR-08); expose a per-condition trace (values of
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
  focused token/row (with undo); "+"/"-"/"*"/"/" after a focused token appends an operator; Ctrl+Z/Ctrl+Shift+Z undo/redo all builder
  edits within the session.
- **Acceptance criteria:**
  1. Given focus on token `Low[0]`, when Delete is pressed, then the token is removed; Ctrl+Z restores it.
  2. Given focus on `High[0]`, when "-" is typed, then the operator is appended and the expression picker opens.
  3. Given an automated accessibility scan of the builder, then no interactive element is reachable only by mouse hover.
- **Data/Kite dependency:** Local only.
- **Deviation from Streak:** Adds keyboard alternatives (NFR-11; §3.1 hover-only affordances).
- **Priority hint:** Important

---

## FR module OPT — Options strategy builder

### FR-OPT-001 Options builder layout and new-draft defaults
- **Source features:** OPT-001
- **Requirement:** The system shall provide an Options Strategy Builder (sidebar "Options strategy") with: header (inline-editable
  title, "My Strategies" button); left scrolling column with cards **Entry**, **Execution** (legs), **Exit**; sticky footer **Save** and
  **Deploy signals** (FR-OPT-032); right column with Payoff graph / Payoff table tabs, "Option chain" link, legend, target-price
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

### FR-OPT-002 Options strategy model extension
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

### FR-OPT-003 Entry mode selector
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

### FR-OPT-004 Underlying picker
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

### FR-OPT-005 Underlying timeframe
- **Source features:** OPT-004
- **Requirement:** Each underlying block has a timeframe dropdown with the 8 base timeframes (default 1 minute). Up to two distinct
  timeframes may exist across underlying blocks (one per block); evaluation timing follows FR-OPT-034.
- **Acceptance criteria:**
  1. Given the dropdown, then exactly 1m, 3m, 5m, 10m, 15m, 30m, 1h, 1d are offered.
  2. Given block 1 = 3m and block 2 = 5m, then the builder shows "Evaluated every 15 minutes".
- **Data/Kite dependency:** Kite historical intervals.
- **Deviation from Streak:** none.
- **Priority hint:** Core

### FR-OPT-006 Underlying conditions block
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

### FR-OPT-007 Second underlying and block combination
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

### FR-OPT-008 Time-based entry
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

### FR-OPT-009 Execution card — leg table
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

### FR-OPT-010 Include-in-payoff checkbox
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

### FR-OPT-011 Leg quantity in lots
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

### FR-OPT-012 Per-leg SL / TP / TSL
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

### FR-OPT-013 Premium and total premium readout
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

### FR-OPT-014 Clear all, delete leg and empty state
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

### FR-OPT-015 Leg-picker drawer
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

### FR-OPT-016 Drawer — option chain
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

### FR-OPT-017 Greeks and implied volatility computation
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

### FR-OPT-018 Rule-based strike: ATM offset
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

### FR-OPT-019 Rule-based strike: premium-based
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

### FR-OPT-020 Expiry selection
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

### FR-OPT-021 Option template library
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

### FR-OPT-022 Combined SL / TP (strategy-level ₹)
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

### FR-OPT-023 Exit conditions on an underlying
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

### FR-OPT-024 Time-based exit
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

### FR-OPT-025 Payoff engine and graph
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

### FR-OPT-026 Payoff table
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

### FR-OPT-027 Target price and target date
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

### FR-OPT-028 Payoff statistics
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

### FR-OPT-029 Funds and margins
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

### FR-OPT-030 Underlying chart drawer
- **Source features:** OPT-027
- **Requirement:** A collapsible drawer "Underlying chart" (toggle on the column divider; keyboard shortcut) shows the shared chart
  component (CHART-001) for the entry underlying at the entry timeframe, with its standard controls.
- **Acceptance criteria:**
  1. Given the drawer is opened, then the chart shows the entry underlying at the entry timeframe.
  2. Given the underlying changes, then the drawer chart switches symbol.
- **Data/Kite dependency:** Historical candles; ticker.
- **Deviation from Streak:** Chart library per C-12.
- **Priority hint:** Nice-to-have

### FR-OPT-031 Option charts drawer (tab per leg)
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

### FR-OPT-032 Footer: Save and Deploy signals; options save dialog
- **Source features:** OPT-029
- **Requirement:** Footer actions **Save** and **Deploy signals** (opens the deployment dialog, DEP-001). Save for a new options strategy
  opens a Save dialog with Name, Category **Option buying** / **Option selling** (pre-selected by the classification rule of FR-OPT-035;
  user may override), Tags (≤ 5), Description. Deploy requires a saved, valid strategy (FR-OPT-002 validation). Unsaved-change guard
  applies (SHELL-007).
- **Acceptance criteria:**
  1. Given a new short straddle, when Save is pressed, then the dialog pre-selects "Option selling".
  2. Given an invalid strategy (no legs), then Deploy is disabled with the reason list.
- **Data/Kite dependency:** Local; DEP module.
- **Deviation from Streak:** Category auto-suggestion explicit.
- **Priority hint:** Core

### FR-OPT-033 Leg execution order and hedge-first
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

### FR-OPT-034 Options multi-timeframe evaluation
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

### FR-OPT-035 Options strategy classification and surfacing
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

### FR-OPT-036 Option contract master data
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
- **Requirement:** The app shall ship a versioned library of ready-made strategies authored by us from generic, well-known setups,
  each a valid StrategyDoc (FR-EQB-002) with: generic descriptive name (unique), plain-language description, category (Equity
  intraday / Equity swing; Option buying / Option selling optional), complexity (Basic/Advanced, derived from the condition mode), base
  timeframe, default instrument set (from the bundled universe snapshot), entry conditions, exit risk and/or exit conditions, trend tag
  (Bullish/Bearish) and 1–4 style tags from the filter vocabulary (FR-DISC-005). Coverage: ≥ 45 strategies spanning the setup families in
  Appendix E (volatility contraction/expansion, Bollinger mean reversion and squeeze, VWAP reversion, CPR and pivot breakouts/breakdowns,
  moving-average crossovers (SMA/EMA/WMA/TEMA), MACD momentum/shift, RSI/stochastic/CCI/Williams reversals, Supertrend combos, ADX/Aroon
  trend strength, previous-high/low breaks, volume-surge confirmations) across timeframes 1m/5m/15m/1h/1d and both intraday and swing.
  Entries are authored independently; the reference names list is not reproduced as-is.
- **Acceptance criteria:**
  1. Given the library file, when CI runs, then ≥ 45 strategies validate, names are unique, every strategy has a trend tag and ≥ 1 style tag from the vocabulary, and each listed setup family has ≥ 1 entry.
  2. Given each library strategy, when backtested on the golden candle fixture, then it runs without validation errors (smoke test).
- **Data/Kite dependency:** Local only.
- **Deviation from Streak:** Own content (clean-room); may include advanced-mode and options examples (Streak's are all Basic equity).
- **Priority hint:** Important

### FR-DISC-003 Strategy category tabs
- **Source features:** DISC-002
- **Requirement:** Tabs **All · Equity swing · Equity intraday**, plus **Option buying · Option selling** shown only when the library
  contains options strategies. Each tab shows its item count. The selected tab is remembered for the session.
- **Acceptance criteria:**
  1. Given a library with 30 intraday and 20 swing, then counts read All 50, Equity swing 20, Equity intraday 30.
  2. Given no options items, then option tabs are hidden.
- **Data/Kite dependency:** Local only.
- **Deviation from Streak:** Counts shown; option tabs conditional.
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
- **Requirement:** Each row shows a category icon (intraday vs swing vs options), full name (wrapped, never truncated without tooltip),
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
  in Settings) using the library's default instruments and default backtest parameters: period (DD-MM-YYYY to DD-MM-YYYY), "View full
  results" (→ BT-008), instrument chips (Combined + each instrument with LTP and % change), metrics Total P&L, Win/Loss count, Trades,
  Win/Loss streak, Max gain, Max loss, Avg win, Avg loss (Appendix M definitions; BT-011), weekday summary (avg return, max profit, max
  loss per weekday), monthly performance grid (% return per month, coloured). Results are cached with data range, parameters and app
  version (NFR-12) and show "Computed <timestamp>" with a **Recompute** action. If candles are missing, the pane offers "Download data"
  with backfill progress (C-08) instead of results. Footer: **Copy**, **Deploy signals** (requires copying first, FR-DISC-009).
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
  "Read-only — copy to edit". Footer: **Copy**, **Deploy signals** (via copy). Users can switch the display between Basic and Advanced
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
  and opened in the editable builder. When invoked from "Deploy signals" on a library item, the button reads "Copy and continue" and
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
- **Requirement:** The app shall ship a versioned library of ready-made scanners authored by us, each with a unique full name,
  description, timeframe, complexity, tags and a valid condition set. Coverage ≥ 37 scanners spanning the families of Appendix F: daily
  gainers/losers by % move, opening-range breakout/breakdown, consecutive higher-highs/lower-lows, short-term MA stack bullish/bearish,
  intraday momentum surge/fade, candlestick patterns (engulfing, harami, hammer, inverted hammer, doji, marubozu), volume
  breakout/breakdown, RSI zones and falling/rising RSI, 52-week high/low breaks, price near 200-day SMA (both sides), open = high /
  open = low, range breakouts, Supertrend crossovers, narrow CPR. Each entry is expressible in the DSL (Advanced where needed).
- **Acceptance criteria:**
  1. Given the library, when CI runs, then ≥ 37 scanners validate, names are unique, and each family above has ≥ 1 scanner.
  2. Given the golden fixture, then each scanner evaluates without error and pattern scanners match hand-labelled fixture candles (e.g. a known engulfing pair).
- **Data/Kite dependency:** Local only.
- **Deviation from Streak:** Own content; unique, untruncated names (§3.2 #12).
- **Priority hint:** Important

### FR-DISC-012 Universe selector
- **Source features:** DISC-010
- **Requirement:** A dropdown of universes per WATCH-006 (index baskets, Indices, Futures, F&O stocks, option-chain baskets per
  underlying and expiry bucket, sectors, user baskets/watchlists), default "NIFTY 50". The choice persists across restarts and is shared
  with the dashboard scanner widget (HOME-004). Changing it relabels row universe pills (option universes show resolved expiry, e.g.
  "NIFTY · current weekly (13-10-2026)") and re-runs the selected scanner.
- **Acceptance criteria:**
  1. Given "NIFTY BANK" selected, when the app restarts, then "NIFTY BANK" is still selected here and in the dashboard widget.
  2. Given an option universe, then pills show the resolved expiry date.
- **Data/Kite dependency:** Universe lists (bundled snapshot, user-importable, C-09); instruments dump for option/futures baskets.
- **Deviation from Streak:** Universe list is data-driven and user-editable (C-09).
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
- **Requirement:** The detail pane shows name, complexity badge, "View conditions" (→ FR-DISC-017), description, options filters when an
  option universe is active (FR-DISC-016), "Results (N)" and "Last scan: DD-MM-YYYY HH:mm:ss" (single format app-wide), a "Scan now"
  action, and an "Auto-refresh while open" toggle (default on during market hours: re-scan at each close of the scanner's timeframe
  while the view is visible). Results use the shared results table (SCAN-007: Symbol with exchange tag, LTP, Change %, Volume, Buy/Sell
  buttons, Chart action; sortable, searchable, CSV export). Buy/Sell open the order window (ORD-004), disabled when trading is not enabled
  (C-04) or for indices. Empty state: "No instruments match right now — try another universe" with the universe selector focused.
  Results are computed by the local scanner engine (identical engine to FR-EQB-041).
- **Acceptance criteria:**
  1. Given a scan completes at 15:29:32 on 09-10-2026, then the header reads "Last scan: 09-10-2026 15:29:32".
  2. Given trading not enabled, then Buy/Sell are disabled with a tooltip linking to the trading setting.
  3. Given auto-refresh on and a 5m scanner, then scans occur at each 5-minute candle close while visible and stop when hidden.
  4. Given 0 results, then the empty state is shown with "(0)".
- **Data/Kite dependency:** Local candle store; live candles from ticker; quotes for LTP/Change/Volume (`GET /quote` batches or ticker).
- **Deviation from Streak:** One timestamp format (§3.2 #14); sorting/search/export added; on-demand rather than server-continuous.
- **Priority hint:** Important

### FR-DISC-016 Options-universe filters
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
- **Requirement:** Users can **Export** a strategy or scanner (overflow menus, My Strategies/My Scanners rows) to a `.sdstrategy.json` /
  `.sdscanner.json` file containing the versioned doc (no credentials, no account data, no backtest results unless "Include results" is
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
- **Requirement:** A "My Strategies" master/detail view with header subtitle (own copy), actions **New strategy** (choice Equity /
  Options), **Import** (FR-DISC-019) and **Discover strategies**; left list (FR-MYS-002/003), right detail (FR-MYS-004). Default sort:
  favourites first, then last modified descending; sort menu offers Name A–Z, Last modified, Created, Last backtest P&L. Empty state:
  "No strategies yet" with **New strategy** and **Browse library**. Lists of ≥ 1,000 strategies scroll smoothly (virtualised).
- **Acceptance criteria:**
  1. Given no strategies, then the empty state with both actions is shown.
  2. Given strategies modified at different times, then the default order is favourites first, then most recently modified.
  3. Given 1,000 strategies (fixture), then scrolling stays at ≥ 50 fps on the reference laptop and search returns in ≤ 100 ms.
- **Data/Kite dependency:** Local store (SQLite).
- **Deviation from Streak:** Sorting and import added; no list caps (NFR-13).
- **Priority hint:** Core

### FR-MYS-002 My Strategies tabs, search and filters
- **Source features:** MYS-002
- **Requirement:** Tabs **All · Equity swing · Equity intraday · Option buying · Option selling** with counts; search (name, description,
  tags); filters popover identical to FR-DISC-005 plus **Status** (Complete, Incomplete, Deployed) and **Favourites**. Options strategies
  appear under their category (FR-OPT-035 rule). Active filters show a badge.
- **Acceptance criteria:**
  1. Given a short strangle saved, then it appears under Option selling and All.
  2. Given filter Status = Incomplete, then only strategies failing Execute validation are shown.
- **Data/Kite dependency:** Local only.
- **Deviation from Streak:** Status and favourites filters added; classification rule documented.
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
  listing past runs (date, period, P&L); sticky footer **Run backtest** (equity/futures only) and **Deploy signals**. Options strategies
  show the payoff summary (max profit/loss, break-evens at last save) instead of backtest sections.
- **Acceptance criteria:**
  1. Given a strategy without backtests, then "No backtest yet" and Run backtest are shown.
  2. Given 3 past backtests, then history lists 3 runs newest first and the pane shows the newest.
  3. Given an options strategy, then no Run backtest action is present.
- **Data/Kite dependency:** Local backtest store; quotes for chips.
- **Deviation from Streak:** Backtest history added (local storage, no caps).
- **Priority hint:** Core

### FR-MYS-005 My Scanners view
- **Source features:** MYS-005
- **Requirement:** A "My Scanners" master/detail view with actions **New scanner**, **Import**, **Discover scanners**; left: search,
  working filter popover (Universe, Timeframe, Alert status Live/Paused/Off, Favourites), timeframe chips (All + present base timeframes),
  list; right: detail or "Select a scanner". Empty state "No scanners yet" with **New scanner** and **Browse library**. Default sort
  favourites first, then last modified.
- **Acceptance criteria:**
  1. Given no scanners, then the empty state is shown.
  2. Given filter Alert status = Live, then only scanners with active alerts are listed.
- **Data/Kite dependency:** Local store; live-scanner registry (SCAN module).
- **Deviation from Streak:** Filters implemented (Streak: "coming soon", §3.2 #31); no scanner count cap (NFR-13).
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
- **Requirement:** User strategies and scanners are stored in the local SQLite database (NFR-03) as StrategyDoc JSON plus indexed columns
  (name, kind, category, tags, favourite, timestamps, status). Writes are transactional; an autosave draft copy of an open, dirty
  editor is written every 10 s to a recovery table and offered on next launch after a crash (NFR-09). Schema migrations are versioned.
- **Acceptance criteria:**
  1. Given the app is killed during editing, when restarted, then a "Recover unsaved changes?" prompt offers the draft (≤ 10 s old).
  2. Given a write failure (disk full simulated), then the user sees an error and the previous saved version remains intact.
- **Data/Kite dependency:** Local only.
- **Deviation from Streak:** Local storage with crash recovery instead of server storage (§1 G3).
- **Priority hint:** Core

---

## Traceability: Feature ID → FR IDs

| Feature ID | FR IDs |
|---|---|
| EQB-001 | FR-EQB-001 |
| EQB-002 | FR-EQB-003, FR-EQB-004 |
| EQB-003 | FR-EQB-005 |
| EQB-004 | FR-EQB-006 |
| EQB-005 | FR-EQB-007 |
| EQB-006 | FR-EQB-002, FR-EQB-008, FR-EQB-041 |
| EQB-007 | FR-EQB-009 |
| EQB-008 | FR-EQB-010 |
| EQB-009 | FR-EQB-011 |
| EQB-010 | FR-EQB-012 |
| EQB-011 | FR-EQB-013 |
| EQB-012 | FR-EQB-014 |
| EQB-013 | FR-EQB-015 |
| EQB-014 | FR-EQB-016, FR-EQB-017 |
| EQB-015 | FR-EQB-018 |
| EQB-016 | FR-EQB-002, FR-EQB-019, FR-EQB-041, FR-EQB-042 |
| EQB-017 | FR-EQB-020 |
| EQB-018 | FR-EQB-021 |
| EQB-019 | FR-EQB-022, FR-EQB-042 |
| EQB-020 | FR-EQB-023 |
| EQB-021 | FR-EQB-012, FR-EQB-024 |
| EQB-022 | FR-EQB-002, FR-EQB-025, FR-EQB-041 |
| EQB-023 | FR-EQB-002, FR-EQB-026 |
| EQB-024 | FR-EQB-002, FR-EQB-027 |
| EQB-025 | FR-EQB-028, FR-EQB-042 |
| EQB-026 | FR-EQB-029 |
| EQB-027 | FR-EQB-002, FR-EQB-030 |
| EQB-028 | FR-EQB-002, FR-EQB-031 |
| EQB-029 | FR-EQB-004 |
| EQB-030 | FR-EQB-032 |
| EQB-031 | FR-EQB-033 |
| EQB-032 | FR-EQB-034 |
| EQB-033 | FR-EQB-035 |
| EQB-034 | FR-EQB-002, FR-EQB-036 |
| EQB-035 | FR-EQB-010, FR-EQB-037 |
| EQB-036 | FR-EQB-017, FR-EQB-038, FR-EQB-039 |
| EQB-037 | FR-EQB-038, FR-EQB-039 |
| EQB-038 | FR-EQB-002, FR-EQB-040 |
| OPT-001 | FR-OPT-001 |
| OPT-002 | FR-OPT-002, FR-OPT-003 |
| OPT-003 | FR-OPT-004, FR-OPT-036 |
| OPT-004 | FR-OPT-005 |
| OPT-005 | FR-OPT-006 |
| OPT-006 | FR-OPT-002, FR-OPT-007 |
| OPT-007 | FR-OPT-002, FR-OPT-008 |
| OPT-008 | FR-OPT-002, FR-OPT-009 |
| OPT-009 | FR-OPT-010 |
| OPT-010 | FR-OPT-011, FR-OPT-036 |
| OPT-011 | FR-OPT-002, FR-OPT-012 |
| OPT-012 | FR-OPT-013 |
| OPT-013 | FR-OPT-014 |
| OPT-014 | FR-OPT-015 |
| OPT-015 | FR-OPT-016, FR-OPT-017, FR-OPT-020, FR-OPT-036 |
| OPT-016 | FR-OPT-002, FR-OPT-018, FR-OPT-020 |
| OPT-017 | FR-OPT-002, FR-OPT-019 |
| OPT-018 | FR-OPT-021 |
| OPT-019 | FR-OPT-002, FR-OPT-022 |
| OPT-020 | FR-OPT-002, FR-OPT-023 |
| OPT-021 | FR-OPT-002, FR-OPT-024 |
| OPT-022 | FR-OPT-025 |
| OPT-023 | FR-OPT-026 |
| OPT-024 | FR-OPT-027 |
| OPT-025 | FR-OPT-028 |
| OPT-026 | FR-OPT-029 |
| OPT-027 | FR-OPT-030 |
| OPT-028 | FR-OPT-031 |
| OPT-029 | FR-OPT-032 |
| OPT-030 | FR-OPT-033 |
| OPT-031 | FR-OPT-034 |
| OPT-032 | FR-OPT-035 |
| DISC-001 | FR-DISC-001, FR-DISC-002, FR-DISC-020 |
| DISC-002 | FR-DISC-003 |
| DISC-003 | FR-DISC-004 |
| DISC-004 | FR-DISC-005 |
| DISC-005 | FR-DISC-002, FR-DISC-006 |
| DISC-006 | FR-DISC-007 |
| DISC-007 | FR-DISC-002, FR-DISC-008 |
| DISC-008 | FR-DISC-009 |
| DISC-009 | FR-DISC-010, FR-DISC-011, FR-DISC-020 |
| DISC-010 | FR-DISC-012 |
| DISC-011 | FR-DISC-013 |
| DISC-012 | FR-DISC-011, FR-DISC-014 |
| DISC-013 | FR-DISC-015 |
| DISC-014 | FR-DISC-016 |
| DISC-015 | FR-DISC-017 |
| DISC-016 | FR-DISC-018 |
| DISC-017 | FR-DISC-019 |
| MYS-001 | FR-MYS-001, FR-MYS-008 |
| MYS-002 | FR-MYS-002 |
| MYS-003 | FR-MYS-003, FR-MYS-007 |
| MYS-004 | FR-MYS-004 |
| MYS-005 | FR-MYS-005, FR-MYS-008 |
| MYS-006 | FR-MYS-006, FR-MYS-007 |

Summary: EQB 42 FRs · OPT 36 FRs · DISC 20 FRs · MYS 8 FRs = 106 FRs; 93 feature IDs (EQB 38, OPT 32, DISC 17, MYS 6), none unmapped.

## Open questions

- **Q-01** MA-type naming: Streak's "triple" (TEMA vs T3) and "variable" vs "vidya" (both appear in the list) — confirm which algorithms we bind to each label before golden vectors are frozen (FR-EQB-038).
- **Q-02** Pivot Points "continuous = yes" meaning (rolling vs session-anchored pivots) — define in `indicators.md` (FR-EQB-038).
- **Q-03** "Higher than" equality: Streak docs say equality counts as true; we made it strict (FR-EQB-012). Confirm the user prefers strict semantics over doc parity.
- **Q-04** AND-before-OR precedence for flat mixed chains (FR-EQB-026) — Streak's actual precedence is unknown; confirm or switch to left-to-right.
- **Q-05** Exact formulas for less-standard operands (Trend Intensity Index, Twiggs MF, Fractal Chaos Bands, Center of Gravity, Swing Index, Schaff, Intraday Momentum Index, ATR Trailing Stoploss parameter semantics) — choose a reference for each in the indicator spec.
- **Q-06** Up by/Down by "from" semantics when R is negative (e.g. oscillators) — percent of |R| or signed R? (FR-EQB-012/013).
- **Q-07** Default instrument set for new drafts (FR-EQB-001): keep a pre-seeded set, or start empty?
- **Q-08** Removing condition/instrument/lot caps (NFR-13) — confirm no soft limits are wanted for very large Basic blocks or instrument lists beyond the warnings specified.
- **Q-09** Options: should the two underlying blocks default to AND (FR-OPT-007) — confirm; and whether more than 2 underlyings should ever be allowed.
- **Q-10** Greeks model inputs: risk-free-rate default (6.5%) and use of Black-76 on the synthetic forward vs Black-Scholes on spot (FR-OPT-017).
- **Q-11** Time-based entry default 09:20 vs Streak's 09:15 (FR-OPT-008) — confirm.
- **Q-12** Should options strategies get a backtest in a later phase (Streak has none; expired-contract intraday history availability from Kite is unverified)?
- **Q-13** Library content: should the Discover library include options strategies and Advanced-mode examples (FR-DISC-002), given Streak's library is equity/Basic only?
- **Q-14** Summary-backtest default window (3 months) and whether library summaries should be precomputed in the background after daily backfill (FR-DISC-007).
- **Q-15** Auto-refresh of library scanners while visible (FR-DISC-015) — acceptable load on Kite quote/ticker limits for broad universes (NIFTY 500, option baskets)?
- **Q-16** Soft-delete retention period (30 days) and whether backtests should be restorable (FR-MYS-007).
- **Q-17** Holiday calendar source for time-based entries/exits (bundled + user-editable vs import from exchange circulars) (FR-OPT-008, FR-OPT-024).
- **Q-18** Freeze-quantity table source and update process (FR-OPT-011).
