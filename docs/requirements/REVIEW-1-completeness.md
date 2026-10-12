# REVIEW-1 — Completeness and correctness review of REQUIREMENTS.md (DRAFT v1)

Reviewer: independent, sceptical pass. Date: 2026-10-11.
Inputs read in full: `docs/research/FEATURES.md` (222 features, appendices A–N, §3, §5, §6), `docs/requirements/REQUIREMENTS.md`
(§1–9, 293 FRs), `docs/research/areas/06-public-docs.md`, `docs/research/areas/07-platform-kite-wails.md`.
Nothing in REQUIREMENTS.md or FEATURES.md was edited.

## 1. Summary verdict

**Not ready to freeze.** The FR set is broad (every one of the 222 feature IDs maps to ≥1 FR — verified: 53 + 93 + 76 = 222;
293 FR headings; no dangling `FR-` cross-references; Appendix J really has 107 studies) and the engine rules largely follow the
chosen conflict interpretations. But the document fails the user's bar ("capture ALL features, no half-baked solution") in
five places that change architecture or scope, and in sixteen places where two FRs contradict each other or a documented
Streak capability is silently dropped. The §7 deviations table — the one place a reader can audit what was dropped — is
truncated in 46 of its 229 rows and is unusable as delivered.

| Severity | Count | Meaning used here |
|---|---|---|
| Blocker | 5 | Must be resolved before phase-1 design starts: missing subsystem, or two FRs that cannot both be built |
| Major | 16 | Wrong or missing requirement that would produce a half-baked feature, a safety gap, or an undocumented drop |
| Minor | 28 | Detail, testability, duplication, terminology or document-quality defect |
| **Total** | **49** | |

Mechanical checks run (bash over both files): FR heading count 293 ✓; defined-vs-referenced FR ids: 0 dangling ✓;
Appendix J study count 107 ✓; §7 rows not ending in a full stop: 46 ✗.

## 2. Findings

Severity: B = Blocker, Mj = Major, Mn = Minor. "R" = REQUIREMENTS.md, "F" = FEATURES.md, "A06"/"A07" = research areas 06/07.

| ID | Sev | Area / FR / Feature | Problem (with source) | Proposed fix |
|---|---|---|---|---|
| RV-001 | B | FR-OPT-001, FR-OPT-035, FR-MYS-004 vs FR-BT-038(b); BT-026, OPT-001 | Direct contradiction. FR-OPT-001: "No Backtest action is offered for options strategies" (AC3: "no Backtest control exists in footer or Execute"); FR-OPT-035/FR-MYS-004: "options strategies have no backtest". FR-BT-038(b) requires "options backtesting for strategies built in the options builder … with the same engine rules applied per leg and combined SL/TP". Both cannot be true; Q-12 asks the question while FR-BT-038 already answers it. | Decide once. Either delete FR-BT-038(b) and keep Q-12 as a later-phase item, or keep it and rewrite FR-OPT-001/035 and FR-MYS-004 to offer "Run backtest" for options with the data-availability caveat (A07 §A4 expired-contract intraday UNVERIFIED). Record the choice in §7. |
| RV-002 | B | DEP module (FR-DEP-001/004/007/008) for options; OPT-029 open question "Deploy dialog for options (not observed)" | Deployment is specified only for per-instrument equity/futures strategies. FR-DEP-001 has "instrument list … per-instrument checkbox and Qty"; an options strategy has legs with their own lots (FR-OPT-011), dynamic strikes resolved at entry (FR-OPT-018), relative expiries (FR-OPT-020) and hedge-first ordering (FR-OPT-033). Nothing defines: what a DI is for a multi-leg strategy; the entry sequence of N legs through the throttle; what state the DI is in when leg 1 fills and leg 2 is rejected (naked short risk); how "Qty" in the deploy dialog relates to leg lots; whether Strategy Cycle applies (Streak has none for options, F §3.3); how an options DI shows TP/SL columns on FR-DEP-011. FR-DEP-008 mentions only "short legs are closed before long hedge legs" on exit. | Add FR-OPT-037 "Options deployment" (outline in §3). Amend FR-DEP-001 to branch on strategy kind; amend FR-DEP-004 with a `LEGS_PARTIAL` sub-state or an ATTENTION trigger for partial leg sets; define the DI as (deployment, leg-set) with per-leg order ids. |
| RV-003 | B | Cross-cutting: FR-SHELL-005, FR-OPT-008/024, FR-BT-031, FR-DEP-004/012, FR-SCAN-010, FR-ORD-005; §9 #12, Q-17, OQ-21 | No FR owns the exchange calendar/session model, yet 9 FRs depend on it and three open questions (§9 #12, Q-17, OQ-21) ask the same thing. FR-SHELL-005 hard-codes `Pre-open 09:00–09:15, Open 09:15–15:30, Closed otherwise, Holiday` and a "local, user-editable list shipped with the current year's holidays"; FR-OPT-008 says "local, user-updatable holiday calendar"; FR-ORD-005 validates "inside market hours for regular variety" (undefined). Missing entirely: the post-closing session 15:40–16:00 that Streak supports (A06 §3 "A trigger after 3:30 PM can be placed in the post-market window, 3:40–4:00 PM"), half-day/special sessions (e.g. the annual Muhurat evening session, which breaks the 09:15-anchored candle alignment in FR-CHART-003/FR-BT-034 and every MIS cut-off), weekly-expiry day handling, and how the calendar is updated year to year. | Add FR-DATA-001 "Market calendar and session model" (outline in §3) and make FR-SHELL-005, FR-OPT-008/024, FR-BT-031, FR-DEP-004/012, FR-SCAN-010, FR-ORD-005 reference it. Collapse §9 #12 / Q-17 / OQ-21 into one question. |
| RV-004 | B | Cross-cutting: FR-CHART-003, FR-BT-009, FR-ANLY-010, FR-ACCT-009, FR-ACCT-012 (Data), FR-WATCH-008, FR-OPT-036; NFR-03, C-08 | The local candle store, the backfill job and the shared Kite rate limiter are described piecemeal in at least six FRs, each with its own numbers (FR-CHART-003 "initial window 1m → 5 trading days…"; FR-BT-009 "≤ 3 requests/s, per-interval windows …, resumable"; FR-ANLY-010 "background priority; user-visible fetches take precedence"; FR-ACCT-009 "historical API 3 req/s with current queue length"). No FR defines: which intervals are stored natively vs resampled; de-duplication when the same range is fetched by two consumers; storage of today's partial candles vs completed candles; store size management (A07 §D: ≈470 M minute rows for 500 stocks × 10 y); candle integrity/checksums that FR-BT-044 depends on; the priority classes of the limiter across all endpoint classes (quote 1/s, historical 3/s, orders 10/s, other 10/s — A07 §A3); error taxonomy and back-off for `NetworkException`/429/`TokenException` (only FR-ACCT-006 and FR-BT-009 touch it). FR-WATCH-008 and FR-OPT-036 duplicate the instrument-master refresh rule. | Add FR-DATA-002 "Candle store" and FR-DATA-003 "Kite client, rate limiter and backfill service" (outlines in §3); turn the six FRs into consumers that cite them. Merge FR-OPT-036 into FR-WATCH-008. |
| RV-005 | B | Cross-cutting: NFR-08, FR-DEP-006, FR-ACCT-006, FR-BT-034, FR-SCAN-010, FR-DEP-012 | Every time rule (candle boundaries "aligned to 09:15 IST", 06:00 token expiry, 15:20/15:21 cut-offs, "<2 s after candle close", "1 s grace for late ticks") assumes the machine clock is correct and in IST. No requirement detects clock skew against exchange time (Kite ticks carry `exchange_timestamp`/`last_trade_time`; historical candles carry exchange timestamps), handles a system timezone other than IST, or defines behaviour when skew exceeds the grace window (a 3 s fast clock would close candles early and evaluate on incomplete data). | Add FR-DATA-004 "Time synchronisation and clock integrity" (outline in §3). |
| RV-006 | Mj | FR-DEP-006, FR-ORD-005 stage 2; DEP-005; A06 §3 Late triggers | Documented Streak capability dropped without a §7 entry. FR-DEP-006: "Signals after 15:30 shall not be actionable as regular orders; the alert explains the 15:40–16:00 post-closing session and AMO are not offered for parity". A06 §3: "A trigger after 3:30 PM can be placed in the post-market window, 3:40–4:00 PM" — Streak *does* allow placement in that window. FR-ORD-005 "inside market hours for regular variety" would block it too. FR-ORD-004 then re-adds AMO as an enhancement — three FRs, three positions. | Add FR-ORD-015 "Order placement in the post-closing session" (equity cash only, 15:40–16:00, MARKET at closing price, flagged in the ticket) and state in FR-DEP-006 that a late signal remains actionable in that window. Keep AMO as the single optional enhancement in FR-ORD-004 (OQ-14). Record the behaviour in §7. |
| RV-007 | Mj | FR-ANLY-012, FR-OPT-017, FR-SCALP-004, FR-SCALP-007; §9 #4, Q-10, OQ-18 | Three incompatible option-pricing specifications for what must be one engine: FR-ANLY-012 "Black-Scholes … risk-free rate and dividend yield taken from Settings … precision 0.01 vol-point"; FR-OPT-017 "Black-76 model on the synthetic forward (derived from put-call parity …) … risk-free rate default 6.5% … tolerance 1e-6, bounds 0.01%–500%"; FR-SCALP-004 "Black-Scholes on spot, time to 15:30 of expiry day, configurable risk-free rate and dividend yield … match to 4 decimals"; FR-SCALP-007 "Black-Scholes value". The same question is asked three times (§9 #4, Q-10, OQ-18). The Scalper chain and the options-builder chain would show different Greeks for the same contract. | Replace the model text in all four FRs with a reference to one new FR-OPT-038 "Options pricing engine" (model, forward construction, day-count, rate/dividend settings, solver bounds and tolerance, golden vectors, Theta/Vega units). Keep one open question. |
| RV-008 | Mj | FR-EQB-025 vs FR-BT-034 vs FR-OPT-034; EQB-022, BT-023, OPT-031; A06 §1 Weekly candles | The MTF rule is written three times with conflicting details. (a) Weekly completion: FR-EQB-025(3) "weekly candles … close when the week's last trading session closes" and rule (2) "most recent T-candle whose close time ≤ e" → the just-finished week is usable at Friday's last evaluation; FR-BT-034 "week candles … are complete only when the next week/month begins" (matches A06 "A weekly candle counts as complete only when the next week begins"). (b) LCM scope: FR-EQB-025(4) applies LCM only to "intraday, < 1d" timeframes; FR-OPT-034 to "condition timeframes ≤ 1h"; FR-BT-034 to any "row's TF not an integer multiple of the base TF" — so a 1d row on a 1h base (375 min vs 60 min, not a multiple) gives LCM(60, 375) = 1500 min under FR-BT-034 and plain base-close evaluation under FR-EQB-025. | Keep the rule in one place (FR-BT-034 is the engine; make FR-EQB-025 and FR-OPT-034 cite it). Fix (a) to the A06 wording; fix (b) by stating: daily/weekly/monthly rows never change the evaluation instants; LCM applies only among intraday timeframes, anchored at 09:15. |
| RV-009 | Mj | FR-DEP-012, FR-DEP-004 (DAY_DONE/EXPIRED); DEP-004; A06 §3 | FR-DEP-012: "MIS daily expiry at 15:21 (DI → DAY_DONE if 're-arm daily' is on, else EXPIRED; default re-arm OFF for parity)". Parity is inverted. A06 §3 documents both "A deployment tracks entry/TP/SL/exit for T+29 days" and "MIS deployments expire daily at 3:21 PM", and the legacy cycle semantics "an MIS cycle per day". A T+29-day validity on an MIS deployment is only meaningful if it re-arms each trading day; with the chosen default every MIS deployment dies on day one and the validity field (FR-DEP-001 default "today + 29") is dead UI. OQ-10 is answerable from the sources (see §4). | Default "re-arm daily" ON for MIS; DAY_DONE is the parity state; EXPIRED only at validity end. Update FR-DEP-004 row "ARMED / CYCLE_DONE / ENTRY_PENDING (MIS) | 15:21 | DAY_DONE". |
| RV-010 | Mj | FR-ACCT-003 vs FR-DEP-016; ACCT-003, DEP-010 | Two FRs for the same toggle with opposite defaults: FR-ACCT-003 "A toggle 'Desktop notifications' … default off" (AC: "Given the toggle off, then no OS notification"); FR-DEP-016 "OS desktop notifications … with a profile toggle (default ON), optional sound, and a per-type filter". FR-ACCT-003 also lists event types FR-DEP-016 does not (session expiry, feed offline > 60 s). | Keep FR-ACCT-003 as the single definition (merge sound + per-type filter into it), delete the duplicate text from FR-DEP-016 and make it a one-line reference. Pick the default (Streak default off per ACCT-003). |
| RV-011 | Mj | FR-HOME-006/007 vs FR-BT-014/015/019; FR-DISC-007; HOME-005, Appendix M "Capital Used … INFERRED" | "Capital used" is displayed by FR-HOME-006 but defined nowhere (FR-BT-014 omits it). The daily-return denominator differs: FR-HOME-007 "Daily return % = that day's realised P&L ÷ capital used of the run"; FR-BT-015/019 use "÷ C" (initial capital). The summary window differs for the same library run: FR-HOME-006 "last 90 calendar days ending the last completed trading day"; FR-DISC-007 "last 3 calendar months (configurable 1–36 months)". FR-HOME-006 "See full results" opens "that run" — which run, with which window? | Add "Capital used" to FR-BT-014 (proposed: Σ over instruments of max(P_fill × qty) across that instrument's trades; combined = Σ instruments). Use one denominator everywhere (C per instrument; combined C × n). Make FR-HOME-006 consume FR-DISC-007's cached summary run and window. |
| RV-012 | Mj | FR-CHART-012/013/014 vs FR-SCALP-011; CHART-010, SCALP-011 | The same chart component (FR-CHART-001 "one reusable chart component used in … each Scalper chart") has two specifications for its Buy/Sell buttons. FR-CHART-012: "Clicking opens the order window (ORD-004) prefilled … it never places an order directly"; "—" when no bid/ask. FR-SCALP-011: "clicking opens a compact confirmation (Market default, qty, product)"; "+" at a price level "places a Limit or SL order at that level (after confirmation)"; opt-in "one-click trading … skips the compact confirmation"; buttons "show 'closed'" / "labelled 'Market closed'" when closed. Drag-to-modify is specified in both FR-CHART-014 and FR-SCALP-011 with different confirmation text. | Make FR-SCALP-011 a delta over FR-CHART-012/013/014 (only the one-click enhancement and quantity presets are scalper-specific). One closed-market label. One confirmation flow (NFR-05). |
| RV-013 | Mj | Missing cross-cutting; NFR-02/03, G3 "data and strategies stored locally", FR-DISC-019 | There is no backup/restore requirement. Goal G3 moves everything that Streak kept server-side (strategies, scanners, watchlists, all backtest runs, deployment/order journals, settings, analytics recordings) onto one machine; FR-DISC-019 exports single strategies/scanners only. A disk failure or reinstall loses the lot. No FR defines the data-directory layout, what is restorable, or how a restore interacts with keyring secrets and schema migrations (NFR-03 "schema migrations versioned" has no FR either). | Add FR-DATA-005 "Backup, restore and data directory" (outline in §3). |
| RV-014 | Mj | Missing cross-cutting; NFR-02 packaging | No requirement for application update: checking for a new version, notifying, migrating data forward (and refusing to run an older binary on a newer schema — FR-EQB-002(5) covers strategy docs only, not the SQLite schema), or what happens to running deployments during an update. Streak updates itself; a .deb/AppImage does not. | Add FR-SHELL-017 "Application update" (outline in §3). |
| RV-015 | Mj | Missing cross-cutting; NFR-10, FR-ACCT-010 "Open logs folder", FR-ORD-008 | NFR-10 ("structured local logs") has no FR. Nothing defines log levels, rotation/size cap, redaction (FR-ORD-008 covers the order journal only; the Kite access token appears in every HTTP `Authorization` header and in the login redirect URL — a debug HTTP log would leak it), crash reports, or a support bundle. | Add FR-SHELL-018 "Application logging and diagnostics" (outline in §3). |
| RV-016 | Mj | FR-EQB-002, FR-SCAN-008, FR-DISC-019, FR-MYS-008; SCAN-008 | FR-EQB-002 says the model covers "every scanner condition set" but defines no `ScannerDoc`: universe (29 kinds + custom basket + watchlist refs), base timeframe, option filters (CALL/PUT/VOLUME>0, strike range), alert settings (frequency, content option, end date — FR-SCAN-009), tags/description. FR-SCAN-008 lists these fields in prose; FR-DISC-019 exports `.sdscanner.json` "containing the versioned doc"; FR-MYS-008 stores "StrategyDoc JSON" for scanners too. Without a schema the export/import, library validation (FR-DISC-011 "validate") and migration promises cannot be tested. | Add a `ScannerDoc` block to FR-EQB-002 (outline in §3, FR-SCAN-016). |
| RV-017 | Mj | FR-DEP-008 (reuses FR-BT-031 times), FR-DEP-004 "SO trigger"; BT-020, DEP-004 | FR-DEP-008: "SO triggers at the instrument's square-off time (FR-BT-031 times)" — i.e. 15:25 for non-F&O equities and futures. Those times are Streak's *backtest simulation* rule (A06 §1). In live mode Streak does not square off at all (it only alerts, and "MIS deployments expire at 3:21 PM"); the broker's own RMS auto-square-off of MIS positions happens independently and earlier than 15:25 for cash equities. An SO *alert* at 15:25 that the user must confirm (mode 1 → EXIT_PENDING) may arrive after the broker has already closed the position, producing a double exit or an ATTENTION state at the worst moment. I could not verify Zerodha's RMS timings from A06/A07 — they are not in the sources. | Make live SO time a separate, user-editable table (default earlier than any broker auto-square-off, e.g. 15:10 for all MIS cash positions) distinct from the backtest table; add an open question to confirm Zerodha RMS timings; in FR-DEP-010 reconcile "closed externally" before raising SO. |
| RV-018 | Mj | §1 "Clean-room … No Streak … branding", NFR-02 `~/.local/share/streak-desktop`, document title, FR-DISC-019 `.sdstrategy.json`/`.sdscanner.json` | The product is named "Streak Desktop", the data directory `streak-desktop`, the file extensions `sd*`, while §1 states no Streak branding is reused. "Streak" is Zerodha's product name. For personal use the risk is low; §1 also anticipates possible later distribution ("needs legal review before any distribution"), at which point the name is the first thing a reviewer will object to. | Pick a neutral codename now (cheap) and use it for the binary, data directory, file extensions and doc titles; keep "Streak" only in research references. |
| RV-019 | Mj | §7 Deliberate deviations table | 46 of 229 rows are cut mid-sentence (first line of the FR's Deviation field only), e.g. FR-SHELL-001 "…Streak's ticker hiding below the `lg` breakpoint is", FR-ACCT-009 "Streak's own caps (100 deployments, 25 live scanners, 50 instruments, backtests/day,", FR-BT-014 "… shown as a positive %, because". §7 is the artefact meant to answer "what was dropped and why"; as delivered it cannot. It also omits deviations that exist only in FR bodies (RV-006, RV-027, RV-028, RV-034). | Regenerate §7 from the full Deviation field; add a second column "Capability dropped? (Y/N) + replacement"; include the §7-missing items listed in this review. |
| RV-020 | Mj | FR-HOME-004 (20 scanner names), FR-HOME-006 (10 strategy names) vs FR-DISC-002 "the reference names list is not reproduced as-is", FR-DISC-011 "authored by us … unique full name"; 6.0a "FEATURES.md lists are coverage references, not content to copy" | FR-HOME-004 requires the widget tab strip to show "in this order: Top Gainers, Top Losers, Opening Range Breakout, … Hammer" and FR-HOME-006 "Bollinger Band Mean Reversion, Falling Momentum, …" — Streak's catalogue names verbatim — while the library FRs forbid reproducing those names. Either the library will contain items with these names (contradicting FR-DISC-002/011 and 6.0a) or the widget cannot show them. | Make FR-HOME-004/006 reference library ids/tags ("the first 20 scanners flagged `featured` in the library, in library order") and move the Streak lists to a coverage checklist in FR-DISC-002/011. |
| RV-021 | Mj | FR-EQB-012 vs A06 §1 "Crosses above vs Higher than"; EQB-010; Q-03 | A06 (current help): "Crosses above means false on the previous candle and true now. Higher than is the current state only, and counts equal as true." FR-EQB-012 makes "higher than" strict (L > R) and defines "crosses above" as `L[t−1] ≤ R[t−1] and L[t] > R[t]`. If Streak's "true" in the crosses definition is its own "higher than" (equality true), its crosses-above is `L[t−1] < R[t−1] and L[t] ≥ R[t]` — the FR flips both boundary cases. This changes signals for every ported suggestion/library strategy whenever L = R (common with integer thresholds such as `RSI(14) crosses above 30`, `Volume(0) higher than 0`, `Open(0) equal to High(0)`). The FR also silently fixes "equal to" to a 1e-9 relative tolerance, which is not in any open question. | Resolve Q-03 before golden vectors: either implement doc parity (and note that "higher than" ≡ "higher than or equal to" in Streak) or keep strict semantics and state the consequence in §7. Add the "equal to" tolerance to the open questions (price equality for the Open=High / Open=Low scanners works; oscillator equality effectively never fires). |
| RV-022 | Mn | FR-SCAN-004 AC2 vs FR-SHELL-009 AC4 vs FR-EQB-008 | FR-SCAN-004: "Given 2 filled and 1 empty Basic row, then Run Scan evaluates the 2 filled rows" (replicates Streak's silent ignore, F EQB-006). FR-SHELL-009 AC4: "Given a scanner run with an incomplete row, then an ERROR toast appears and the incomplete row is outlined". FR-EQB-008 blocks Execute on incomplete rows. "Empty" vs "incomplete" is never defined. | Define: fully empty row (no LHS) = removed at run with an INFO toast; partially filled row = incomplete = blocking error in both builders. |
| RV-023 | Mn | FR-ACCT-004 vs FR-DEP-012 | FR-ACCT-004: "a second launch focuses the existing window"; FR-DEP-012: "one app instance per data directory (second instance refuses to start)", AC3 "shows 'already running' and exits". | Choose one (focus existing is the usual desktop behaviour); delete the other sentence. |
| RV-024 | Mn | FR-WATCH-013 vs FR-OPT-009; WATCH-008 | FR-WATCH-013 ("One formatter shall render instrument names everywhere"): weekly `NIFTY 13ᵗʰ ʷ OCT 22500 CE`, monthly `BANKNIFTY OCT 55300 CE`, dynamic `UNDERLYING DD MON LTP@<price> CE|PE`. FR-OPT-009: "one naming format: `<UNDERLYING> <DD MMM> <W|M> <STRIKE> <CE|PE>` for fixed legs and `<UNDERLYING> <DD MMM> <rule> <CE|PE>` … (e.g. `ATM+2`, `Premium≈75`)". Two formats, and the WATCH one copies Streak's distinctive superscript-"w" convention (F WATCH-008) — a clean-room smell. | Keep one format in FR-WATCH-013 (own convention, e.g. `NIFTY 13 OCT (W) 22500 CE`), make FR-OPT-009 cite it. |
| RV-025 | Mn | FR-DISC-011 vs Appendix F "What it detects" | Appendix F records the actual rule of most prebuilt scanners (Top Gainers `Close(0) up by 1% from Close(-1)`; ORB on the 30-min range; Short Term Bullish `Close > SMA21 > SMA50 > SMA100`; Momentum Surge 0.5 % close-vs-open; Volume Breakout vs 20-period average; 52-week = 252 sessions; Falling RSI 5 candles; Narrow CPR `Abs(tc − bc) lower than pp × 0.001`). FR-DISC-011 carries only family names, so a library with "a breakout scanner" of any parameters passes CI. Generic rules with public parameters are not Streak content. | Add an acceptance checklist to FR-DISC-011 listing each of the 37 families with the parameter(s) from Appendix F that the library entry must implement (own name, own description). |
| RV-026 | Mn | FR-BT-013 AC1 | The fixture totals do not reconcile under one rounding rule: sell-leg transaction charge is listed as 0.69 (23,400 × 0.00297 % = 0.69498) but the stated leg total 15.13 only holds with 0.70 (7.02 + 6.00 + 0.70 + 0.02 + 1.39). The "rounding rule" column of the fee table is named but its values are not given for the fixture. | State the rounding rule per component in the fixture (e.g. half-up to 0.01 for all except STT/stamp to ₹1) and recompute; golden numbers must be unambiguous. |
| RV-027 | Mn | FR-BT-019 vs A06 §2 | A06: "Open CNC positions are marked to that day's close **against the buy price**" (the heading is "Hypothetical **cumulative** P&L calendar"). FR-BT-019 marks "against the previous day's mark (first day: against entry price)" — a daily-delta model. The FR's model is more useful (day P&Ls sum to realised), but it is a deviation and §7 says "Deviation from Streak: none". | Keep the daily-delta model, record the deviation, and let OQ-05 cover the ambiguity explicitly. |
| RV-028 | Mn | FR-BT-031 "MIS with a 1-day base TF is rejected at run time" | A restriction Streak does not document (A06 lists 1 day as a base timeframe and MIS as a product with no exclusion). Not listed in §7 (the FR's Deviation names only the containing-candle rule). May block a legitimate Streak workflow (day-TF signal, intraday hold). | Either allow MIS on 1d (fill at next day's open, SO same day at the table time using intraday data) or list it in §7 as a dropped capability with the reason. |
| RV-029 | Mn | Terminology across modules | The same thing has several names: deploy action = "Notification Based Trading" (F), "Signals / Notification-based trading" (FR-EQB-032), "Deploy signals" (FR-OPT-032, FR-DISC-007, FR-MYS-004), "Deploy" (FR-BT-011), "Execute (mode Deploy)" (FR-DEP-001), "Notify & confirm" (FR-DEP-003). Sidebar: "Equity Builder/Options Builder/New Scanner" (FR-SHELL-002) vs "Equity strategy" (FR-EQB-001) vs "Options strategy" (FR-OPT-001); "Live Strategies" (FR-SHELL-002/003) vs "Live Deployments view" (FR-DEP-011). Order UI: "order window" (CHART/WATCH/HOME), "order ticket" (FR-ORD-004), "order review" (FR-DEP-007), "compact confirmation" (FR-SCALP-011). | Add a UI glossary to §3 and normalise. |
| RV-030 | Mn | FR-EQB-012 "equal to" | See RV-021: tolerance chosen silently (`|L − R| ≤ 1e-9 × max(1, |L|, |R|)`), not in §9. | Add to open questions; consider tick-size tolerance for price operands. |
| RV-031 | Mn | FR-EQB-038(2), FR-EQB-039 AC1; Appendix A row 8, Appendix B Schaff | Three MA-type enumerations exist in the catalogue: MA11; Moving Average's 9-list (…hull, **variable**, vidya — no mesa/kaufman); Schaff's 9-list (…hull, **kaufman**, vidya — no mesa/variable). FR-EQB-038(2) mentions only "MA11 and the 9-type list". FR-EQB-039's CI check compares *defaults* to the appendix, not enumerations, so a wrong option list passes. | Name all three lists in FR-EQB-038(2); extend FR-EQB-039 AC1 to "defaults **and enumeration values** equal the appendix". |
| RV-032 | Mn | FR-EQB-038(6), NFR-06 | Golden vectors come from "a pinned-version open-source TA library where the indicator exists there, or an independently written spreadsheet/Python implementation". The library is unnamed; TA libraries differ on seeding (e.g. EMA/RSI Wilder vs simple seed), so the golden set is undefined until it is named. | Name the reference per operand in `indicators.md` (part of Q-05) and pin versions in the FR. |
| RV-033 | Mn | FR-CHART-011 vs CHART-009 "⊕ (add alert/order)"; A07 §A8b | Streak's chart crosshair "+" offers alert or order; FR-CHART-011 keeps only "Create order at <price>". Kite's alerts API (500 active, `simple`/`ato`, A07 §A8b — "the only built-in way to act on a price when the desktop app is off") is used by no FR, although FEATURES cites it as the server-side fallback alongside GTT. | Add a Nice-to-have FR "Price alerts (Kite alerts API)" reachable from the crosshair "+" and instrument rows; or record the drop in §7. |
| RV-034 | Mn | FR-OPT-026 vs FR-SCALP-007; FR-OPT-027 vs OPT-024 | Payoff table default step: FR-OPT-026 "rows every strike step by default"; FR-SCALP-007 "default 10 points" (Streak: 10 points, F OPT-023/SCALP-007). Target-price stepper: Streak step 10 (F OPT-024); FR-OPT-027 "step = strike step" — not listed as a deviation. | One payoff component, one default (10 points for parity, step selectable); list the stepper change in §7 or revert. |
| RV-035 | Mn | FR-SCAN-014 vs FR-CHART-017 | FR-SCAN-014: "open a full-size chart view (new window or tab within the app)"; FR-CHART-017: "Wails v2 supports a single window, so the chart opens as an in-app view with back navigation". | FR-SCAN-014 → "opens the full-view chart (FR-CHART-017)". |
| RV-036 | Mn | FR-SHELL-003 vs FR-DEP-015 | FR-SHELL-003 puts the unread *count* on the bell and red *dots* on Live Strategies / Scanner Alerts; FR-DEP-015: "An unread count badge shall appear on the navigation entry" (Notifications is not a sidebar entry). | FR-DEP-015 → "on the bell (FR-SHELL-003)". |
| RV-037 | Mn | FR-DEP-004 CYCLE_DONE "cycle limit reached for the day (MIS) or deployment (CNC)" vs FR-DEP-001/FR-BT-002 "Strategy Cycle … disabled when CNC" | A CNC DI can never reach CYCLE_DONE under FR-DEP-001; the state table row "EXIT_ORDER_OPEN | filled | ARMED, or CYCLE_DONE if cycles exhausted" is unreachable for CNC. | Remove "(CNC)" from CYCLE_DONE or re-enable a per-deployment cycle for CNC (Streak legacy "CNC cycle counted per deployment", A06 §1) — decide and align the three FRs. |
| RV-038 | Mn | FR-WATCH-009 vs FR-WATCH-012 vs FR-SCAN-002 | Placement of custom baskets/watchlists in universe lists: "after 'Futures'" (FR-WATCH-009), "after the built-in universes" (FR-WATCH-012), "plus Custom Symbols after Futures, plus the user's watchlists and custom baskets" (FR-SCAN-002, position unspecified). | One ordered list definition in FR-WATCH-009; others cite it. |
| RV-039 | Mn | FR-DISC-007 "period (DD-MM-YYYY to DD-MM-YYYY)" vs FR-BT-011 "backtest period (DD Mon YYYY to DD Mon YYYY)" vs FR-SHELL-016 "Each view uses one style per field consistently" | The same field (backtest period) is specified in two styles. | Pick the long style for periods everywhere. |
| RV-040 | Mn | Testability — vague or deferred ACs | FR-ANLY-003 "The exact rule table shall be fixed in the design spec" (AC2 then tests an undefined table); FR-ANLY-003 "Awesome Oscillator (saucer/zero-cross rules)", "Bull/Bear Power 13 (bull/bear power rules)"; FR-ANLY-012 "defaults set in the design spec"; FR-WATCH-008 AC3 / FR-MYS-001 AC3 "on the reference laptop" (undefined; NFR-07 says "4-core laptop"); FR-SCALP-008 AC1 "charts narrow and remain interactive"; FR-OPT-006 AC1 "behaves per its FR (shared component test)"; FR-DISC-002 AC1 "each listed setup family has ≥ 1 entry" with no family taxonomy in data; FR-CHART-003 AC1 "about 3 years"; FR-SHELL-001 AC2 "all top-bar controls remain visible". | Fix numbers in the FRs (rule tables inline or in a referenced spec file that exists), define "reference machine" once in §5, replace "about"/"interactive" with measurable values. |
| RV-041 | Mn | §9 open questions | Duplicates across the three lists: holiday calendar (§9 #12, Q-17, OQ-21); IV/Greeks parameters (§9 #4, Q-10, OQ-18); public-IP check (§9 #13, OQ-19); caps (§9 #9, Q-08); sector classification (§9 #15, OQ-17); freeze-quantity table (Q-18, OQ-13); expired-contract history (Q-12, OQ-20); "Closed" definition (§9 #7) already answered by FR-HOME-003/FR-DEP-004. Several are answerable from the sources (see §4). | Merge into one numbered list with owner, blocking-phase flag and "default taken". |
| RV-042 | Mn | FR-ACCT-011; ACCT-008, A06 §3 Notification channels | Streak delivers every alert to the mobile app; the only replacement is a desktop notification on the same machine. The drop is recorded, but the compensation is weaker than any Streak channel and no optional user-configured relay (local script hook / self-hosted webhook) is offered, even as an enhancement. | Add an optional "Alert hook" enhancement (run a user script / POST to a user URL on alert, off by default, NFR-04-compliant) or state explicitly that away-from-desk alerting is out of scope. |
| RV-043 | Mn | FR-DEP-020 "Keep running in tray (Enhancement)" | A07 §D does not establish that Wails v2 offers a system tray; the v2 API has no tray support (only v3 beta does). The enhancement may be unbuildable on the chosen stack. | Mark UNVERIFIED and add to the platform open questions, or drop. |
| RV-044 | Mn | NFR-04, FR-ACCT-005 "stores the secret only in the OS keyring (libsecret)" | No behaviour when no Secret Service daemon is running (minimal window managers, headless/SSH sessions): the app can neither store nor read the secret. | Specify: detect absence at setup; offer an encrypted-file fallback protected by a user passphrase (never plaintext), or refuse to run with a clear message. |
| RV-045 | Mn | Cross-cutting keyboard/window state | Shortcuts are scattered (Alt+T FR-SHELL-009, Alt+H/J/V/C FR-CHART-010, Shift+T FR-CHART-013, F11 FR-CHART-017, Ctrl+Z FR-EQB-042, Alt+↑/↓ FR-WATCH-001/FR-OPT-033, kill-switch "keyboard shortcut" FR-ORD-009 unassigned, FR-SCALP-013 "configurable"). No FR owns the global shortcut map, conflict rules (Alt+T vs. GTK menu mnemonics) or remapping. Window size/position/maximised state persistence is not required anywhere. | Add FR-SHELL-019 "Global shortcuts and window state" (outline in §3). |
| RV-046 | Mn | FR-OPT-024 "(trigger SO-style time exit)", FR-BT-043 | A time-based options exit needs a trigger code; FR-BT-043 defines exactly seven and "SO-style" is not one of them. | Use SO for time exits (document) or add a code `TE` to FR-BT-043 and Appendix M mapping. |
| RV-047 | Mn | FR-HOME-004 vs FR-SCAN-013/FR-WATCH-009 AC5 | The widget evaluates the active scanner live on any of the 29 universes (e.g. Nifty 500, option baskets) but no budget check/refusal is specified for it, unlike live scanners (FR-SCAN-013) and deployments (FR-DEP-012). | State that the widget uses the subscription manager at the lowest priority and falls back to polled evaluation when over budget. |
| RV-048 | Mn | FR-ORD-005 step 9, A07 §C `AlgoID`, §B.2 "orders … tagged with a generic algo ID provided by the Exchange" | The `algo_id` order parameter (gokiteconnect v4.4.1+) is "unverified" in A07 and not mentioned in the submit stage; if Zerodha requires it for API orders after 1 Apr 2026, every order is rejected. | Add an open question with a test on the first live order; reserve a settings field. |
| RV-049 | Mn | FR-SHELL-007 vs FR-BT-042; FR-WATCH-008 vs FR-OPT-036 | The backtest-page tour is specified twice (consistently); the instrument-master refresh is specified twice (FR-WATCH-008 "once per trading day after 08:30 IST", FR-OPT-036 "refreshed automatically at first launch after 08:30 IST each trading day and manually on demand" — the second adds a manual refresh and the lot-size-change rule the first lacks). | Delete FR-BT-042's body (cite FR-SHELL-007); fold FR-OPT-036's extra rules into FR-WATCH-008. |

### 2.1 Checks that passed (so they are not re-litigated)

- Engine rules vs A06 and the FEATURES §6 chosen interpretations: next-open fills, fresh-transition entry, no pyramiding,
  signal-time display, SL-first same-candle, O→L→H→C / O→H→L→C paths, per-instrument square-off table incl. the 14:15 hourly
  case, LCM for 3/5-min, capital gating `P × qty ≤ C + R`, qty = floor(C ÷ P) (the two observed examples reproduce), charges on
  both legs, metric definitions, trigger codes, T+29 default, 15:20/15:21 — all correct. Arithmetic in FR-BT-014, FR-BT-015,
  FR-BT-019, FR-BT-006, FR-HOME-002, FR-OPT-013, FR-OPT-025, FR-OPT-028, FR-SCALP-007, FR-ANLY-004, FR-DEP-009 and FR-OPT-021
  AC1 (W = 4 × 50 = ±200/±400 matches the observed Short Iron Condor) was re-computed and is right (only FR-BT-013 fails, RV-026).
- Kite/SEBI facts vs A07: plan split, 06:00 expiry, checksum, rate limits (1/3/10 req/s; 400/min; 5,000/day; 25 mods),
  3 × 3,000 subscriptions, `market_protection` (−1 or 0 < x ≤ 100; 0 rejected), static IP, historical windows
  60/100/200/400/2000, `continuous=1` day-only, `oi=1`, instruments dump ~08:30, `/quote` 500 and `/quote/ltp` 1000 per call,
  GTT single/OCO LIMIT, postbacks unusable — all correctly carried.
- Appendix coverage: 71 Basic + 28 Advance-only operands with parameters/defaults/enumerations are required by FR-EQB-038/039;
  9 comparators (FR-EQB-012); 8 math functions (FR-EQB-022); timeframes and MTF list (FR-EQB-025); 29 universes in order
  (FR-WATCH-009); 34 option templates fully specified (FR-OPT-021); suggestion scale ≥300/≥300 (FR-EQB-017); 107 chart studies
  by reference (FR-CHART-008) and the drawing-tool groups (FR-CHART-010); metrics and trigger codes (FR-BT-014/043);
  Appendix N limits all removed and listed (FR-ACCT-009). 45 strategies and 37 scanners are covered as families only
  (RV-025 for scanners; the strategy catalogue has no definitions to carry).
- §3.2 Streak bugs: all 33 are either fixed by an FR or not replicated; the only partial replication is RV-022.

## 3. Missing FRs to add (draft outlines)

Each outline follows the document's FR format. Source = the catalogue/constraint that motivates it.

### FR-DATA-001 Market calendar and session model  (Blocker, RV-003)
- **Source:** SHELL-003, BT-020, DEP-004/005, OPT-007/021, SCAN-010; A06 §3 Late triggers; C-03.
- **Requirement:** One calendar service shall define, per exchange (NSE, BSE, NFO, BFO), per date: trading day / holiday / special
  session, and the session phases with times — pre-open (09:00–09:08 order entry, 09:08–09:12 matching, 09:12–09:15 buffer),
  continuous (09:15–15:30), closing-price computation (15:30–15:40), post-closing (15:40–16:00, equity cash only, MARKET at
  closing price), and any special session (date, phases, times; e.g. an evening Muhurat session or an exchange-announced
  half-day). The app ships the current year's NSE/BSE list; the user can import an updated list (CSV: date, exchange, type,
  phases) and edit entries; lists older than the current year show "calendar may be stale". Every time-based rule in the app
  (candle alignment, MTF LCM anchor, MIS square-off, 15:20/15:21 cut-offs, time-based entries/exits, scanner runs, alert expiry,
  reconciliation, "Market closed" label, order-placement validity per variety) shall obtain session boundaries from this
  service, never from literals. A special session's candles are aligned to that session's start, and MIS square-off for it is
  `session end − configurable minutes`.
- **Acceptance criteria:** (1) Given a shipped list and a date marked holiday, then FR-SHELL-005 shows "Holiday", no scanner/
  deployment runs, and no missed-signal replay is attempted. (2) Given a special session 18:15–19:15 configured for date D, then
  5-min candles on D start at 18:15 and the MIS SO candle is the one containing 19:15 − N. (3) Given 15:45 on a trading day and an
  equity MARKET order, then FR-ORD-005 accepts it as a post-closing order; given an NFO order at 15:45, then it is refused with
  "post-closing session is equity cash only". (4) Given an imported calendar with an invalid row, then import is refused naming
  the row. (5) Given a date absent from the list and no candles for that date on NIFTY 50, then the app treats it as a
  non-trading day and flags "inferred" in the status tooltip.
- **Data/Kite dependency:** none (Kite has no calendar endpoint); candle presence for inference.

### FR-DATA-002 Candle store  (Blocker, RV-004)
- **Source:** CHART-001, BT-006, HOME-005, ANLY-005; NFR-03, NFR-12, C-08; A07 §A4, §D.
- **Requirement:** One local store shall hold candles per (exchange, token, interval) for the eight Kite intervals natively;
  week/month and any other interval are resampled on read (anchored per FR-DATA-001) and never stored. Each candle row: start
  time (exchange time), O, H, L, C, V, OI (nullable), `source` (historical | tick-built | manual-refresh), `complete` flag.
  Today's tick-built candles are stored as incomplete and replaced by historical candles when fetched after the session.
  Writes are idempotent (same (token, interval, start) → upsert). Per (token, interval) the store keeps a coverage map of fetched
  ranges so consumers ask "what is missing between A and B" without scanning. A checksum per (token, interval, calendar month)
  is maintained for FR-BT-044. Settings show size per interval and total; the user can set a size budget and purge rules (e.g.
  "keep 1-min for 2 years"); purge never touches ranges referenced by a stored backtest unless the user confirms. Storage
  engine choice (SQLite vs DuckDB/Parquet) is a design decision recorded against the 470 M-row estimate in A07 §D.
- **Acceptance criteria:** (1) Given the same 200-day window fetched by two consumers, then only one historical request is made.
  (2) Given a tick-built 10:00 candle and a later historical fetch for the day, then the historical values replace it and
  `complete` = true. (3) Given purge of 1-min data older than N days while a backtest references it, then a confirmation names
  the runs affected. (4) Given 500 instruments × 2 years × 1-min loaded, then a point query for one instrument-day returns in
  < 50 ms. (5) Given a resample request for 1 week from daily candles, then the week candle for Mon–Fri equals the aggregate of
  its daily candles (golden test).

### FR-DATA-003 Kite client, shared rate limiter and backfill service  (Blocker, RV-004)
- **Source:** C-05, C-08; A07 §A3, §A4, §C; FR-BT-009, FR-ANLY-010, FR-ACCT-009.
- **Requirement:** All Kite HTTP calls pass through one client with per-class token buckets (quote 1/s, historical 3/s, orders
  per FR-ORD-006, other 10/s), priority classes (interactive > deployment > live scanner > backtest > analytics recorder >
  library precompute), fair queuing within a class, and visible queue depth (FR-ACCT-009). Error taxonomy: `TokenException` →
  FR-ACCT-006 expired state; `PermissionException` → FR-ACCT-007 degraded mode; HTTP 429 → exponential back-off (base 1 s, max
  30 s) on that class only, counter shown; `NetworkException` → retry idempotent GETs up to 3× with jitter, never retry order
  POSTs blindly (FR-ORD-006). The backfill service takes (token, interval, from, to) jobs, splits them into Kite window sizes
  (adapting on window errors), persists job state (queued / running / done per window), resumes after restart, reports progress
  (done/total, ETA) to any subscriber, and de-duplicates overlapping jobs.
- **Acceptance criteria:** (1) Given 10 backtest jobs and one interactive chart load queued, then the chart's requests are served
  first and no second contains > 3 historical requests. (2) Given an HTTP 429 on historical, then quote traffic is unaffected and
  the next historical call waits ≥ 1 s. (3) Given a kill during backfill, then on restart completed windows are not re-fetched.
  (4) Given a `TokenException` from any endpoint, then all queues pause and the expired banner shows within 5 s.

### FR-DATA-004 Time synchronisation and clock integrity  (Blocker, RV-005)
- **Source:** NFR-08; FR-DEP-006; FR-ACCT-006; FR-BT-034; A07 §A6 (tick timestamps).
- **Requirement:** All scheduling uses an app clock = system clock + measured offset, where the offset is estimated continuously
  from Kite tick `exchange_timestamp` arrival deltas (median over the last 60 s) and, outside market hours, from the `Date`
  header of Kite HTTP responses. All internal times are IST regardless of the system timezone. If |offset| > 1 s a WARNING is
  shown; if > 5 s live evaluation, scanners and time-based entries are suspended with reason "clock out of sync" until the
  offset returns below 2 s or the user acknowledges. Candle finalisation uses the app clock; the grace window (FR-DEP-006) is
  measured from the exchange-time boundary.
- **Acceptance criteria:** (1) Given the system clock set 3 s fast, then within 60 s of ticks the offset is ≈ −3 s and candles
  close at exchange time ± 0.5 s. (2) Given a system timezone of UTC, then every displayed time and every schedule is IST.
  (3) Given offset 7 s, then deployments show SUSPENDED (clock) and no order is sent.

### FR-DATA-005 Backup, restore and data directory  (Major, RV-013)
- **Source:** G3, NFR-02/03/09; FR-DISC-019.
- **Requirement:** Define the data directory layout (app DB, candle store, logs, exports) and schema migration policy (forward
  migrations run on start inside a transaction; a backup of the DB is taken before each migration; the app refuses to open a
  DB with a newer schema and names both versions). "Backup now" produces one archive containing the app DB (strategies,
  scanners, watchlists, settings, backtest runs, deployment/order journals, analytics recordings) with an option to include the
  candle store; secrets are never included (keyring). "Restore" validates the archive, shows what will be replaced, and restores
  into a fresh data directory; optional scheduled backup (daily after 16:00) to a user folder.
- **Acceptance criteria:** (1) Given a backup and a clean install, when restored, then every strategy, scanner, run and journal
  entry is present and the keyring is empty (re-login required). (2) Given a schema 3 DB and a schema-2 binary, then the app
  refuses with both versions named. (3) Given an archive grep for the API secret/token, then no match.

### FR-SHELL-017 Application update  (Major, RV-014)
- **Requirement:** On start (and daily) the app may check a user-configured or default release feed (off by default, NFR-04) and
  show "Update available vX" with release notes; no automatic download. Updating is manual (.deb/AppImage); on first start of a
  new version the migration path of FR-DATA-005 runs and a "What changed" page lists library-version changes (FR-DISC-020).
  Running deployments are not affected until the user quits (FR-DEP-020 dialog).
- **Acceptance criteria:** (1) Given the check is off, then no network request other than Kite is made (network capture).
  (2) Given a newer version on the feed, then the banner appears with the version and notes.

### FR-SHELL-018 Application logging and diagnostics  (Major, RV-015)
- **Requirement:** Structured JSON logs (level, time, component, event, fields) at INFO by default, DEBUG switchable at runtime;
  rotation at 20 MB × 10 files; redaction filter that removes `Authorization` headers, `request_token`, `access_token`,
  `api_secret`, `checksum` and any value from the keyring (unit-tested); crash handler writes a report with stack and app state
  summary; "Create support bundle" zips logs, settings (secrets excluded), schema version and recent journal excerpts.
- **Acceptance criteria:** (1) Given DEBUG HTTP logging, then a grep of all logs for the access token returns nothing. (2) Given
  a forced panic, then a crash report exists and the next start offers to view it.

### FR-SHELL-019 Global keyboard shortcuts and window state  (Minor, RV-045)
- **Requirement:** One shortcut registry (navigation to each sidebar entry, kill switch, new order, search, help, F11, toasts
  Alt+T, chart tools) with conflict detection and user remapping in Settings; the Help "Keyboard shortcuts" article is generated
  from the registry. Window size, position, maximised state and the last view are persisted and restored.
- **Acceptance criteria:** (1) Given a remap of kill switch to Ctrl+Shift+K, then the old binding does nothing and the article
  shows the new one. (2) Given the app is closed maximised on the Scalper view, then it reopens maximised on the Scalper view.

### FR-OPT-037 Options deployment  (Blocker, RV-002)
- **Source:** OPT-029 open question; DEP-001; OPT-030; A06 §5 Leg order, §3 Combined SL/TP.
- **Requirement:** Deploying an options strategy creates one deployment with one DI per leg set (not per instrument). Dialog
  fields: legs (read-only summary with lots editable per leg), Capital, Holding type NRML (MIS optional for intraday), mode
  (FR-DEP-003), validity, acknowledgement; no instrument list, no Strategy Cycle (one entry per entry signal; Time-based entries
  once per selected day). On an entry signal, dynamic legs are resolved (FR-OPT-018/019/020), the leg set is shown in one order
  review (mode 1) listing legs in execution order with per-leg margin and basket margin; on confirm, legs are submitted
  sequentially in order through the throttle, each tagged with the deployment tag + leg index. State: `ENTRY_ORDER_OPEN`
  until all legs are COMPLETE; if any leg is rejected or partially filled after the others filled, the DI goes to ATTENTION with
  actions "Exit filled legs" (default, pre-filled exit orders, short legs first) / "Keep". Exits (combined SL/TP, per-leg
  SL/TP/TSL, EX, time exit, expiry EXP on the leg's expiry day at `session end − N minutes`) exit all open legs, shorts before
  longs. FR-DEP-011 shows the leg set as a parent row with legs as children; TP/SL columns show the combined ₹ levels.
- **Acceptance criteria:** (1) Given a 4-leg iron condor in mode 1, when confirmed, then 4 orders are placed in saved order
  within the throttle and the DI is IN_POSITION only after the 4th COMPLETE. (2) Given leg 3 rejected after legs 1–2 filled,
  then the DI is ATTENTION and "Exit filled legs" pre-fills two opposite orders with the short leg first. (3) Given expiry day
  15:15 with open legs, then an EXP exit signal for all legs is raised.

### FR-OPT-038 Options pricing engine  (Major, RV-007)
- **Requirement:** One engine for IV and Greeks used by ANLY, OPT and SCALP: model (Black-76 on a forward derived from put-call
  parity at ATM, fallback spot × e^{(r−q)T}), day count (calendar days / 365 to 15:30 IST of expiry), inputs r and q from
  Settings (defaults recorded here, not "in the design spec"), solver (bracketed, tolerance 1e-6, bounds 0.01 %–500 %), units
  (Theta per calendar day, Vega per 1 vol point, IV in %), golden vectors (NFR-06), "—" when unsolvable.
- **Acceptance criteria:** (1) Given the golden set, then IV/Δ/Γ/Θ/Vega match within 1e-6 relative in all three UIs (same code
  path, verified by a shared test). (2) Given the same contract in the Scalper chain and the options-builder chain, then the
  displayed Greeks are identical.

### FR-SCAN-016 Scanner document schema  (Major, RV-016)
- **Requirement:** Extend FR-EQB-002 with
  `ScannerDoc { schema: "sd.scanner", version, id, meta{…same as StrategyDoc…}, universe: UniverseRef, timeframe: TF(1m..1d),
  conditions: ConditionBlock, optionFilters?: { calls: bool, puts: bool, volumeGt0: bool, strikeRange: int(1..30)|"all" },
  alerts?: { enabled, content: "all"|"new", endDate?, frozenSymbols?: [InstrumentRef] } }`;
  `UniverseRef = { builtin: <one of 29 ids> } | { basket: id } | { watchlist: id } | { custom: [InstrumentRef] }`.
  Same round-trip, migration and export rules as StrategyDoc; library scanners validate against it in CI.
- **Acceptance criteria:** as FR-EQB-002 AC1–4 applied to scanners; FR-DISC-019 import of a `.sdscanner.json` restores option
  filters and alert settings.

### FR-BT-046 "Capital used" metric  (Major, RV-011)
- **Requirement:** Per instrument, Capital used = max over trades of (P_fill × qty) (the largest notional ever deployed);
  Combined = Σ instruments. Shown in FR-HOME-006 and FR-BT-014. Daily return % everywhere = day P&L ÷ C (per instrument) or
  ÷ (C × n) (combined).
- **Acceptance criteria:** Given trades of notional 23,866.20 and 23,400.00 on one instrument, then Capital used = ₹23,866.20.

### FR-ORD-015 Order placement outside the continuous session  (Major, RV-006)
- **Requirement:** Per FR-DATA-001 phases: pre-open (09:00–09:08) regular LIMIT/MARKET orders allowed for equities with a ticket
  banner; post-closing (15:40–16:00) equity cash MARKET orders allowed at closing price with a banner; all other times regular
  orders are refused with the next allowed phase named; AMO (variety `amo`) only if FR-ORD-004's enhancement is enabled.
  Late deployment signals (FR-DEP-006) stay actionable during the post-closing session.
- **Acceptance criteria:** (1) Given 15:45 and an equity signal from 15:25, then "Review order" is enabled and the ticket shows
  "post-closing session — market order at closing price". (2) Given 16:05, then the action is disabled with "next session
  09:00 tomorrow".

### FR-DEP-021 Live square-off timing  (Major, RV-017)
- **Requirement:** Live SO triggers use a separate user-editable table (per instrument class) defaulting to times earlier than
  the broker's MIS auto-square-off (default proposal: 15:10 cash, 15:10 F&O; to be confirmed — open question), independent of
  the backtest table (FR-BT-031). Before raising SO, the DI reconciles the broker position; if already flat, record "closed
  externally" instead of an exit alert.
- **Acceptance criteria:** (1) Given a live MIS long on a non-F&O stock and the live table at 15:10, then the SO alert fires at
  15:10:00 ± 1 s, not 15:25. (2) Given the broker shows net qty 0 at that time, then no SO alert is raised.

### FR-ACCT-013 Secret storage fallback  (Minor, RV-044)
- **Requirement:** If no Secret Service is available, the app offers an encrypted secrets file (Argon2id-derived key from a user
  passphrase, asked once per app start) or refuses to run; plaintext storage is never offered.
- **Acceptance criteria:** Given no keyring daemon, then the setup flow shows the choice and a grep of the data directory for the
  secret returns nothing.

## 4. Open questions answerable from the sources

| Question | Answer | Citation |
|---|---|---|
| OQ-01 Initial Capital per instrument or shared? | **Per instrument** on the current platform. The current Backtest Assumptions page (Mar 2026) states the capital rule "for every instrument", and the observed run derived qty 9 and qty 2 for two instruments from the same ₹25,000. Remove the "legacy" caveat from FR-BT-005. | A06 §1 "Initial Capital must be ≥ Quantity × Close for every instrument"; F BT-005 "25000 / 2651.80 → qty 9; 25000 / 8835.5 → 2" |
| OQ-10 Is MIS expiry at 15:21 a deployment end or a daily pause? | **Daily pause within the T+29 validity** (re-arm next trading day). Validity T+29 and a daily 15:21 expiry are only consistent if the deployment survives the day; legacy semantics say "an MIS cycle per day". Flip FR-DEP-012's default (RV-009). | A06 §3 "A deployment tracks … for T+29 days"; "MIS deployments expire daily at 3:21 PM"; A06 §1 [LEGACY] "A CNC cycle counted per deployment and an MIS cycle per day" |
| OQ-03 (part) Does End Time force an exit? | **No** — the window governs entries only; the parameters are named "Entry Start Time"/"Entry Stop Time". Boundary inclusivity and signal-vs-fill time remain open. | A06 §1 "'Strategy cycle', 'Entry Start Time' and 'Entry Stop Time' … set how many entries are allowed per day and the time window for them" |
| OQ-13 Freeze-quantity source | Kite applies the freeze quantity server-side and `autoslice=true` splits an order into up to 10 slices; a local table is therefore optional (useful only for the pre-flight warning in FR-OPT-011). Streak itself just rejects above-freeze orders. | A07 §A7 "`autoslice=true`: splits orders above the freeze quantity, up to 10 slices"; A06 §3 "Orders above the freeze limit are rejected" |
| OQ-14 Offer AMO? | For parity: **no**; Streak lists AMO among unsupported items. Kite supports variety `amo`, so it is a legitimate enhancement (FR-ORD-004) — but see RV-006: the post-closing session is the parity path for late signals. | A06 §1 "No MCX, currency, AMO or MTF"; A07 §A7 varieties "regular, amo, co, iceberg, auction" |
| Q-03 "Higher than" with equality | Fact: Streak counts equality as true (so its "Higher than" and "Higher than equal to" coincide); FR-EQB-012's strict rule is a deviation (RV-021). Decision remains the user's. | A06 §1 "Higher than is the current state only, and counts equal as true" |
| Q-07 Default instrument set | Fact: Streak pre-seeds four (SBIN, HDFCBANK, ADANIENT, APOLLOHOSP). The FR's configurable set with a 4-stock default already matches; close the question. | F EQB-001 "pre-seeded with 4 instruments (SBIN, HDFCBANK, ADANIENT, APOLLOHOSP)" |
| Q-11 Time-based entry default | Fact: Streak defaults to 9:15 AM (and accepts out-of-session times — bug §3.2 #22). 09:20 is a deliberate deviation; keep it listed in §7. | F OPT-007 "hour dropdown 01–12 (default 9); minute 00–59 (default 15); AM" |
| Q-12 / OQ-20 Options backtest and expired contracts | Facts: Streak has none (confirmed 3 Sep 2026); Kite expired-contract intraday history is UNVERIFIED; expired tokens drop out of the instruments dump. Scope decision pending (RV-001). | A06 §1 "Options backtesting is not available … confirmed as of 3 Sep 2026"; A07 §A4 "Expired F&O contracts: UNVERIFIED … Expired tokens drop out of the instruments dump" |
| Q-13 Options in the Discover library | Fact: Streak's Discover has no options strategies and all 45 are "Basic" equity; the My Strategies tabs do have Option Buying/Selling. Including options/Advanced examples is an enhancement, not parity. | F OPT-032, DISC-002, Appendix E "all 45 are badge 'Basic'" |
| §9 #14 Ship constituent snapshots? | **Do not ship** copies. Both providers' terms prohibit systematic collection and storing content "in an electronic retrieval system" without permission; bundling a snapshot in a distributed package is storage/copying. Ship empty lists with a one-click "Import the CSV you downloaded" flow (A07's options 1–2). | A07 §E Terms of use (NSE: "may not be copied or stored in an electronic retrieval system without permission"; niftyindices: "personal, non-commercial use only … no systematic or automated data collection") |
| §9 #18 Chart types | **Candlestick only**; Heikin-Ashi and Renko are unsupported on the new platform. No HA/line chart is required for parity. | A06 §1 "Chart types: candlestick only. Heikin-Ashi and Renko are not supported." |
| §9 #7 "Closed strategies" definition | Already determined by the DEP state model: Closed = DIs in STOPPED ∪ EXPIRED (∪ COMPLETED); Active = any non-terminal. FR-HOME-003 says this; close the question and cite FR-DEP-004. | R FR-HOME-003, FR-DEP-004 |
| §9 #8 Featured lists | Fact: the observed sets are the 20 widget tabs and the 10 Live Backtest tabs in Appendix F/E order — but they cannot be used verbatim (RV-020). | F HOME-004, HOME-005, Appendix E "Live Backtest widget rotation (10)" |
| §9 #20 Tour copy | Fact: Streak's backtest-tour texts were not transcribed (and must not be copied anyway); all tour copy is to be written fresh. Close as a design task. | F SHELL-006 "its own 2-step tour (steps not transcribed)"; §5.2 #42 |
| OQ-19 Can the registered static IP be read via API? | No API is documented: IPs are configured at developer-account level on the console. Rejection detection is the only in-band signal; an external IP-echo is the only out-of-band one. | A07 §A12 "They are set at developer-account level and apply to all apps" |
| OQ-12 (part) Does GTT placement need the static IP? | Zerodha's statement scopes the mandate to "order placement"; GTT is a trigger endpoint, not an order endpoint, so the literal reading is "no" — but a triggered GTT does place an order server-side. Treat as UNVERIFIED and test on the first GTT. | A07 §A12 "Static IP is mandatory for order placement … Other endpoints work from any IP" |
| OQ-05 (part) Return base and open-position marking | Documented only as "marked to that day's close against the buy price" and "average return, max profit and max loss per weekday"; the base is not given. FR-BT-015/019 are a reasonable choice — record as a deviation (RV-027). | A06 §2 Hypothetical cumulative P&L calendar / Daily Summary |

Not answerable from the sources (must be observed or decided): §9 #1 (library choice), #2 (drawing-tool list), #5 (PCR scope),
#11 (loopback redirect), #16 (Indices/Futures membership), Q-01 (MA bindings), Q-02 (pivot "continuous"), Q-04 (precedence),
Q-05 (formulas), Q-06 (negative R), OQ-02 ("–" cycle), OQ-06, OQ-07, OQ-08, OQ-09 (CAS list), OQ-11, OQ-15, OQ-16, OQ-21.

### 4.1 Questions that block phase-1 work

1. RV-001 options backtest scope (changes the BT engine's data model: per-leg positions).
2. RV-004 candle store / Kite client (everything else is built on it).
3. RV-003 calendar/session model (candle alignment and square-off cannot be tested without it).
4. RV-005 clock integrity (NFR-08 is untestable without it).
5. Q-01 MA-type bindings and RV-032 reference library — golden vectors (NFR-06) cannot be frozen before these.
6. C-12 / §9 #1 chart library — decides whether FR-CHART-008/009/010/014 are weeks or months of work.
7. RV-021 / Q-03 comparator semantics — affects every suggestion and library strategy before authoring starts.
8. RV-009 / OQ-10 MIS re-arm default — affects the DEP state machine design.

---
*End of review.*
