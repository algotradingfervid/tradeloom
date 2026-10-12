# CHANGELOG — REQUIREMENTS.md v1 → v2 (Tradeloom)

Date: 2026-10-12. Inputs: `REVIEW-1-completeness.md` (RV-001…RV-049, §3 outlines, §4 answers), `REVIEW-2-live-sweep.md`
(N1–N10, claim checks #1–#40), user decisions of 2026-10-12. Output: `REQUIREMENTS.md` v2 (single source of truth), updated
`../research/FEATURES.md`, `fr/README.md`. Verification script: `verify-v2.py` (results at the end).

## 1. User decisions applied

| # | Decision | Where |
|---|---|---|
| 1 | Name **Tradeloom**; binary `tradeloom`; data dir `~/.local/share/tradeloom`; exports `.tlstrategy.json` / `.tlscanner.json`; schema ids `tl.strategy` / `tl.scanner`; "Streak" only for the reference product | Title, §1, NFR-02, FR-DATA-005, FR-EQB-002, FR-SCAN-016, FR-DISC-019; RV-018 closed |
| 2 | Scope = cash equities + index/stock futures; indices non-tradable; options, MCX, currency out | §1.1/1.2, NFR-15, FR-WATCH-008 (dump filtered), FR-BT-038/039, whole OPT module + 9 FRs + 25 "-opt" fragments → §10 |
| 3 | Scalper kept, adapted to equity/futures | FR-SCALP-001/002/003/005/008–013 rewritten; FR-SCALP-004/006/007 → §10 |
| 4 | Enhancements in scope: paper trading, opt-in auto-execution, GTT-protected exits, Telegram alerts; options backtesting out | FR-DEP-017, FR-DEP-018 (completed), FR-DEP-009, new FR-ACCT-014; FR-BT-038(b) → §10 BT-038-opt |
| 5 | RV-021 documented parity; RV-009 re-arm ON; RV-017 live SO table 15:10 + OQ-31; RV-043 UNVERIFIED; RV-018 by name | FR-EQB-012; FR-DEP-004/012; FR-DEP-021; FR-DEP-020 + C-13; title |

## 2. REVIEW-1 findings (all 49)

| RV | Applied? | What changed |
|---|---|---|
| RV-001 | Yes (by scope) | Options backtesting removed: FR-BT-038(b) → §10 BT-038-opt; FR-OPT-001/035 and FR-MYS-004 options text → §10; the contradiction no longer exists in §6. |
| RV-002 | Yes (deferred) | FR-OPT-037 "Options deployment" drafted in §10 (never in §6). FR-DEP-001/004/008 no longer mention options. |
| RV-003 | Yes | New FR-DATA-001 market calendar/session model (pre-open, continuous, closing, post-closing 15:40–16:00, special sessions incl. Muhurat/half-days, import, staleness, inference). FR-SHELL-005, FR-BT-031, FR-DEP-004/006/010/012/021, FR-SCAN-010, FR-ORD-005/015, FR-HOME-007/008, FR-EQB-007/025/038, FR-BT-025/034/044, FR-DATA-002/004 cite it. §9 #12 / Q-17 / OQ-21 merged into OQ-08. |
| RV-004 | Yes | New FR-DATA-002 candle store (native intervals, resample on read, coverage map, checksums, budgets) and FR-DATA-003 Kite client/limiter/backfill (buckets, priority classes, error taxonomy, resumable jobs). FR-CHART-003, FR-BT-009, FR-ANLY-010, FR-ACCT-009, FR-WATCH-008, FR-CHART-006, FR-BT-008, FR-SCAN-003/006, FR-ACCT-006, FR-DEP-006/019 are consumers. FR-OPT-036 extras folded into FR-WATCH-008. |
| RV-005 | Yes | New FR-DATA-004 app clock (tick/HTTP offset, IST, 1 s warning, 5 s suspension). NFR-08 extended; FR-DEP-004 SUSPENDED reason; FR-DATA-001/FR-SCAN-010/FR-DEP-006/FR-ACCT-006 use it. |
| RV-006 | Yes | New FR-ORD-015 (pre-open, post-closing equity-cash MARKET, refusals, AMO only if enabled); FR-DEP-006 keeps late signals actionable in post-closing; FR-ORD-004/005 reference the phases; AMO stays the single enhancement in FR-ORD-004. §7 row added. |
| RV-007 | Yes (deferred) | FR-OPT-038 pricing engine drafted in §10; FR-ANLY-012, FR-SCALP-004/007 moved to §10 (options). One open question kept (OQ-§10-2). |
| RV-008 | Yes | FR-BT-034 is the single MTF definition: weekly complete only when the next week begins; LCM only among intraday TFs anchored at session start; daily/weekly/monthly rows never change instants. FR-EQB-025 cites it; FR-OPT-034 → §10; FR-EQB-041 and FR-SCAN-010 reference it. |
| RV-009 | Yes | "Re-arm daily" default ON (FR-DEP-001/012); DAY_DONE is the parity state; EXPIRED only at validity end or re-arm off (FR-DEP-004). |
| RV-010 | Yes | FR-ACCT-003 is the single OS-channel definition (default off, per-type filter, sound, click targets); FR-DEP-016 is a one-line reference; FR-SCAN-009 cites it. |
| RV-011 | Yes | New FR-BT-046 Capital used + return denominators (C, C × n); FR-BT-014 and FR-DISC-007 show it; FR-HOME-006 consumes FR-DISC-007's cached run and window; FR-HOME-007 uses ÷ (C × n). |
| RV-012 | Yes | FR-SCALP-011 is a delta over FR-CHART-012/013/014 (shared "—" label, one confirmation flow); only one-click and presets are Scalper-specific. |
| RV-013 | Yes | New FR-DATA-005 backup/restore/data directory/migrations (pre-migration backup, newer-schema refusal, secrets excluded, scheduled backup). G3, NFR-02/03/09 updated. |
| RV-014 | Yes | New FR-SHELL-017 application update (opt-in check, no auto-download, "What changed", migration path). |
| RV-015 | Yes | New FR-SHELL-018 logging/diagnostics (JSON, rotation 20 MB × 10, redaction filter unit-tested, crash report, support bundle). NFR-04/10 reference it; FR-ACCT-010 links the bundle. |
| RV-016 | Yes | New FR-SCAN-016 ScannerDoc schema (universe ref, timeframe, conditions, alerts, frozenSymbols); option filters → §10 SCAN-016-opt. FR-EQB-002, FR-SCAN-008, FR-MYS-008, FR-DISC-019, FR-DISC-011 reference it. |
| RV-017 | Yes | New FR-DEP-021 live SO table (default 15:10 cash and futures, reconcile-before-SO, "closed externally"); FR-DEP-008 uses it; FR-BT-031 is backtest-only; FR-DEP-009 GTT deletion at live SO; OQ-31 (Zerodha RMS timings). |
| RV-018 | Yes | Codename Tradeloom everywhere (decision 1). |
| RV-019 | Yes | §7 regenerated from full Deviation fields with columns FR / Deviation / Capability dropped? / Replacement; v1-missing items (RV-006/027/028/034) now have rows. |
| RV-020 | Yes | FR-HOME-004/006 use library `featured` tags (Settings → Home to change); Streak's name lists moved to coverage checklists in FR-DISC-002 (family taxonomy) and FR-DISC-011 (rule table); CI checks that no library name equals an Appendix E/F name. |
| RV-021 | Yes (decision 5) | FR-EQB-012: higher/lower than count equality true; crosses above = L[t−1] < R[t−1] and L[t] ≥ R[t]; ACs rewritten; equality tolerance → OQ-18; §7 row records parity. |
| RV-022 | Yes | §3.1 defines empty vs incomplete rows; FR-EQB-008, FR-SCAN-004, FR-SHELL-009 AC4 aligned (empty → INFO toast, incomplete → blocking ERROR). |
| RV-023 | Yes | Single instance = focus existing window (FR-ACCT-004 only); sentence removed from FR-DEP-012. |
| RV-024 | Yes | FR-WATCH-013 keeps one own convention (futures), no superscript/ordinal; option naming (own convention `NIFTY 13 OCT (W) 22500 CE`) recorded in §10 WATCH-013-opt. |
| RV-025 | Yes | FR-DISC-011 acceptance checklist: 20 families with the Appendix F parameters each library entry must implement; CI parameter check. |
| RV-026 | Yes | FR-BT-013 fixture states the rounding rule (half-up ₹0.01; STT/stamp half-up ₹1; GST on rounded components) and is recomputed: sell leg 15.12, total 25.43, net −491.63. |
| RV-027 | Yes | FR-BT-019 records the daily-delta deviation from Streak's "against the buy price"; OQ-26 covers it. |
| RV-028 | Yes | FR-BT-031 allows MIS on a 1-day base (next-open fill, 1-minute Intrabar SL/TP, SO at the 1-minute candle containing T); recorded in assumptions. |
| RV-029 | Yes | §3.2 UI glossary (Equity Builder, Scanner builder, Execute/execution panel, Backtest, Deploy/Deployment, execution modes, Live Strategies, order ticket + review mode, Buy/Sell buttons, instrument picker/symbol search, universe/custom basket, library, alerts, notifications, Settings/Profile, kill switch, status indicator); global normalisation (order window → order ticket; Deploy signals → Deploy; Live Deployments view → Live Strategies view; "order review" → ticket in review mode). |
| RV-030 | Yes | OQ-18 added (equality tolerance; tick-size alternative). |
| RV-031 | Yes | FR-EQB-038(2) names all three MA-type lists; FR-EQB-039 AC1 checks defaults **and enumeration values**. |
| RV-032 | Yes | FR-EQB-038(6): reference implementation named and version-pinned per operand in `indicators.md` (TA-Lib default), seeding rule stated; OQ-13. |
| RV-033 | Yes | New FR-ORD-016 price alerts via Kite alerts API (Nice-to-have); FR-CHART-011 "+" offers order or alert. |
| RV-034 | Yes (deferred) | Payoff defaults aligned in §10 FR-OPT-038 (10-point step, stepper step 10); FR-OPT-026/027/SCALP-007 live in §10. |
| RV-035 | Yes | FR-SCAN-014 → "opens the full-view chart (FR-CHART-017)". |
| RV-036 | Yes | FR-DEP-015 → unread count on the bell (FR-SHELL-003). |
| RV-037 | Yes | CYCLE_DONE is MIS-only; CNC never enters it (FR-DEP-004 table + AC3). |
| RV-038 | Yes | FR-WATCH-009 holds the one ordered list (21 built-ins, separator, custom baskets A–Z, separator, watchlists; "Custom symbols" last in the builder); FR-WATCH-012, FR-SCAN-002, FR-DISC-012, FR-HOME-004 cite it. |
| RV-039 | Yes | Periods use the long style everywhere (FR-DISC-007 fixed; §6.0a convention). |
| RV-040 | Yes | FR-ANLY-003 rule table normative with AO and Bull/Bear Power rules inline; "reference machine" defined in §5 and used by FR-WATCH-008, FR-MYS-001, FR-EQB-041, FR-BT-010, FR-DATA-002; FR-SCALP-008 AC measurable (320 px, 300 ms, 100 ms); FR-DISC-002 family taxonomy in data; FR-CHART-003 "≥ 740 daily candles within 5 s"; FR-SHELL-001 AC2 enumerates the controls. FR-ANLY-012 and FR-OPT-006 are in §10. |
| RV-041 | Yes | §9 is one merged list OQ-01…OQ-44 with Owner / Blocks phase? / Default taken; duplicates merged (calendar, IV, IP check, caps, sectors, freeze qty, expired contracts); answered questions removed (list in §4 below); options-only questions in §10.0. |
| RV-042 | Yes (decision 4) | FR-ACCT-014 Telegram bot alerts (own token/chat id in keyring, off by default, per-type filter, send-only, 20/min merge) replaces the generic hook; FR-ACCT-011 and FR-DEP-006 reference it. |
| RV-043 | Yes (decision 5) | FR-DEP-020 tray option marked UNVERIFIED (C-13, OQ-38). |
| RV-044 | Yes | New FR-ACCT-013 encrypted secrets file (Argon2id + XChaCha20-Poly1305) or refuse; never plaintext; FR-ACCT-005 and NFR-04 reference it. |
| RV-045 | Yes | New FR-SHELL-019 shortcut registry (defaults, scopes, remapping, conflict detection, generated help article) + window state; all shortcut mentions (FR-SHELL-009, FR-CHART-010/013/017, FR-WATCH-001, FR-EQB-042, FR-ORD-009, FR-SCALP-013) cite it. |
| RV-046 | Yes (deferred) | Time-based exit code `TE` recorded in §10 FR-OPT-037 and OQ-§10-8 (no time exits exist for equity strategies in §6). |
| RV-047 | Yes | FR-HOME-004: widget subscribes at lowest priority, falls back to polled evaluation when over budget, never refuses. |
| RV-048 | Yes | FR-ORD-005 step 9 submits `algo_id` when set (Settings → Trading field reserved, empty by default); OQ-37 (test on first live order). |
| RV-049 | Yes | FR-BT-042 body replaced by a reference to FR-SHELL-007; FR-OPT-036's manual refresh and lot-size rule folded into FR-WATCH-008. |

REVIEW-1 §3 outlines: FR-DATA-001…005, FR-SHELL-017/018/019, FR-SCAN-016, FR-BT-046, FR-ORD-015, FR-DEP-021, FR-ACCT-013 added
to §6 (expanded to the full FR format); FR-OPT-037/038 added to §10 only.

## 3. REVIEW-2 items

| Item | Applied? | REQUIREMENTS.md | FEATURES.md |
|---|---|---|---|
| N1 Technicals event tooltips | Yes | FR-ANLY-005 tooltip per event row (own text), AC4 | ANLY-002 corrected + screenshot 01 |
| N2 Execute step 1 chooser | Yes | FR-EQB-032 two-step execution panel (Deploy default, Backtest; Cancel/Next; Back on step 2); FR-BT-001 and FR-DEP-001 are step 2 | New EQB-039 + screenshot 02 |
| N3 Back button, trading-terms link, cycle default 1 | Yes | FR-BT-001 Back / Run backtest; FR-DEP-001 Back / Deploy, Strategy Cycle default 1, terms link → FR-ACCT-015 | BT-001, DEP-001 corrected + screenshot 03; Conflicts §6 #31 |
| N4 Up By type % / abs, trash icon | Yes | FR-EQB-013: Points = abs now parity (deviation removed), Remove action | EQB-011 corrected (OBSERVED, screenshot 04); §5.2 #2 resolved |
| N5 Scanner alert dialog extras | Yes | FR-SCAN-009 trigger-frequency note; dialog closable by Escape/×/Cancel recorded as deviation | SCAN-009 extended + screenshot 05 |
| N6 /terms page | Yes | New FR-ACCT-015 legal text pages (own text), linked from deploy and alert dialogs | New ACCT-009 + screenshot 06 |
| N7 /privacy page | Yes | FR-ACCT-015 (privacy notice) | New ACCT-010 + screenshot 07 |
| N8 /scanners alias | Yes (noted) | FR-MYS-005 deviation: no alias needed (one view); traceability MYS-007 → FR-MYS-005 | New MYS-007 + screenshot 08 |
| N9 Risk disclosure per session | Yes | New FR-SHELL-020 (after each login, gates Deploy/arming); FR-ACCT-006, FR-DEP-001/019, FR-EQB-032 reference it | New SHELL-013 (INFERRED, no screenshot) |
| N10 /login route | Yes | FR-ACCT-006 (own login screen) — traceability ACCT-011 | New ACCT-011 (status only) |
| Claim #14 (Cancel/Run Backtest) | Yes | FR-BT-001 | BT-001 |
| Claim #15 (NBT footer, cycle default) | Yes | FR-DEP-001 | DEP-001 |
| Claim #17 (trade-log P&L running total) | Yes | FR-BT-020: P&L column = cumulative running P&L on every row; Trade P&L added as an Enhancement column | BT-016 corrected; Conflicts §6 #30 |
| Claim #25 (107 Select Expression entries) | Yes | FR-EQB-020 AC4 reference count 107 = 99 + 8 | SCAN-004, Appendix B; §5.2 #45 |
| Claim #27 (Up By type) | Yes | see N4 | EQB-011 |
| Claim #28 (leave-site guard) | Yes | FR-SHELL-008 guards in-app navigation; deviation records Streak's gap | SHELL-007 corrected |
| Claim #29 (default 1 Hour, cannot verify) | Partly | Default timeframe made a setting (shipped 1 Hour) in FR-HOME-001, FR-CHART-004, FR-SCALP-002; OQ-23 | CHART-002, Appendix I noted; Conflicts §6 #29; §5.2 #46 |
| Claim #30 (Technicals tooltips) | Yes | see N1 | ANLY-002 |
| 32 confirmed claims | No change needed | — | Evidence lines reference R2 where entries were touched |
| Code-only hints ({live, auto, paper}; 2min/4hour) | Noted | — | §3.3 note |

## 4. Open questions closed (answered by REVIEW-1 §4 or by the decisions)

OQ-01 old (capital per instrument → confirmed, FR-BT-005 deviation "none"); OQ-10 old (MIS expiry = daily pause → decision 5, FR-DEP-012);
OQ-03 old part (End Time does not force exit → FR-BT-003; boundary inclusivity stays as OQ-24); OQ-13 old (freeze qty → Kite autoslice,
OQ-34 keeps the optional table); OQ-14 old (AMO → optional enhancement FR-ORD-004; post-closing is the parity path FR-ORD-015); Q-03
(comparator equality → decision 5); Q-07 (default instrument set → FR-EQB-001 configurable, 4-stock default); Q-11 (time-based entry
default → §10); Q-12/OQ-20 options part (options backtest → out of scope); Q-13 (options in library → §10 OQ-§10-5); §9 #14 (constituent
snapshots → do not ship, C-09 and FR-SCAN-002/FR-DISC-002 ship empty lists with import); §9 #18 (chart types → candlestick only);
§9 #7 ("Closed" = STOPPED ∪ EXPIRED per FR-DEP-004); §9 #8 (featured lists → library tags, RV-020); §9 #20 (tour copy → written fresh,
FR-SHELL-007); OQ-19 old (registered IP not readable via API → FR-ORD-013 rejection detection; OQ-09 keeps the user choice); OQ-12 old
part (GTT and static IP → UNVERIFIED, OQ-33); OQ-05 old part (return base → recorded deviation, OQ-26); §9 #4 / Q-10 / OQ-18 old (IV
parameters → §10 OQ-§10-2); §9 #17 (remember F&O qty key → FR-ACCT-002 defines it); §9 #9 / Q-08 merged into OQ-05.

## 5. Counts

- §6: 265 FRs in scope — SHELL 20 · HOME 8 · CHART 17 · ANLY 8 · WATCH 14 · ACCT 15 · DATA 5 · EQB 42 · DISC 19 · MYS 8 · BT 46 · DEP 21 · SCAN 16 · SCALP 10 · ORD 16.
- §10: 72 entries — 47 whole FRs (36 OPT, 5 ANLY, 1 DISC, 3 SCALP, 2 deferred FR-OPT-037/038) and 25 "-opt" fragments.
- New §6 FR IDs (17): FR-DATA-001…005, FR-SHELL-017/018/019/020, FR-ACCT-013/014/015, FR-SCAN-016, FR-BT-046, FR-ORD-015/016, FR-DEP-021.
- v1 FRs removed from §6: 36 OPT + FR-ANLY-006/007/008/009/012 + FR-DISC-016 + FR-SCALP-004/006/007 = 45 (all preserved in §10). 293 − 45 + 17 = 265.
- Catalogue: 222 → 228 features (SHELL-013, EQB-039, MYS-007, ACCT-009/010/011); 7 claims corrected; module map, ledger (R2), gaps #43–#46 and conflicts #29–#31 updated.
- §7: 244 rows (every §6 FR whose Deviation field is not "none"). §8: 228 rows. §9: 44 questions (5 block Phase 1: OQ-01, OQ-07, OQ-13, OQ-16, OQ-41; 4 block Live: OQ-09, OQ-31, OQ-33, OQ-37) + 9 options-only questions in §10.0.

## 6. Verification (script `verify-v2.py`, run on the installed files)

```
$ python3 verify-v2.py REQUIREMENTS.md ../research/FEATURES.md
[PASS] FR ids unique — 265 in §6, 72 in §10; duplicates: []
[PASS] every FR referenced exists — missing: []
[PASS] no §6 FR references an FR that lives only in §10 — {}
[PASS] every FEATURES.md feature ID appears in §8 — 228 features; missing []; not in catalogue []
[PASS] every §6 FR has all fields — {}
[PASS] no "Streak Desktop"/"streak-desktop"/".sdstrategy" remains — []
[PASS] §6 free of options feature text (manual review list) — 2 lines to eyeball
       The Scalper is adapted to equity/futures (§1.3): the option-chain drawer, option templates
       - **Deviation from Streak:** Streak's Scalper is an options workspace (underlying + ATM CE
COUNT §6 = 265 :: SHELL 20, HOME 8, CHART 17, ANLY 8, WATCH 14, ACCT 15, DATA 5, EQB 42, DISC 19, MYS 8, BT 46, DEP 21, SCAN 16, SCALP 10, ORD 16
COUNT §10 = 72 (47 whole FRs, 25 -opt fragments)
[PASS] every OQ reference exists in §9/§10.0 and no old Q-xx ids remain — bad []; old []
§7 rows: 244; §8 rows: 228; §9 questions: 53
```

The two "eyeball" lines are the SCALP module intro and FR-SCALP-001's deviation, both of which legitimately point at §10. All other checks pass.
