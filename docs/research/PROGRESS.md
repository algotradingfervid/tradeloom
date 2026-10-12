# Research progress (resume from here)

Goal: map every Streak feature (screenshots + descriptions) → requirements doc → completeness review →
phase-wise implementation plan for a Go + Wails local Linux desktop app using Zerodha Kite Connect.
Decisions so far: purpose = avoid subscription limits; data/broker = Kite Connect; stack = Go + Wails v2
(official gokiteconnect SDK); v1 scope = strategy builder + backtest (later phases add scanners, paper, live).
Clean-room: observe UI only; never copy Streak code/assets; no cookie export (user's session stays in Chrome).
Safety: read-only exploration — never save/copy/deploy/order/delete/change settings.

## Status
- [x] Core inventory: streak-feature-inventory.md (+ screenshots/00-core)
- [~] Parallel area agents (reports in areas/, screenshots in screenshots/<area>/):
  [x] 01 equity builder · [x] 02 options · [x] 03 scanners · [x] 04 dashboard/charts/scalper ·
  [x] 05 live/orders/account · [x] 06 public help docs · [x] 07 Kite/SEBI/Wails platform
- [x] Approved account changes (user, 2026-10-11): copied strategy `zz-research-copy` (backtest run on it) and
  scanner `zz-research-scanner` (alerts NOT enabled). Delete both when research is done (ask user first).
- [x] Observed Execute→Backtest params and Execute→NBT deploy dialog (not deployed) — see inventory.
- Known unobservable: order window (places real orders), live-market behaviour (weekend), populated live
  deployments/orders/positions (account empty) — specify from help docs + Kite API instead.
- Tab hygiene: close every tab when done; user accidentally closed the Claude tab group once.
- [x] Verified agent reports (lead spot-checks; live sweep now part of review)
- [x] Consolidated feature catalogue docs/research/FEATURES.md (222 features, 15 modules, appendices A–N, coverage ledger, 42 gap items, 28 conflicts)
- [x] Requirements v1 merged: docs/requirements/REQUIREMENTS.md — 293 FRs (§6), deviations (§7), traceability (§8, all 222
  features covered, verified by script), open questions (§9). Source parts in docs/requirements/fr/.
- [x] Independent reviews: REVIEW-1-completeness.md (49 findings: 5 Blocker/16 Major/28 Minor) + REVIEW-2-live-sweep.md
  (10 new items, 7 wrong claims; 32/40 claims confirmed)
- [x] USER DECISIONS 2026-10-12: name **Tradeloom**; scope = equity only (NSE/BSE cash + index/stock futures; indices
  non-tradable as instruments; NO options/MCX/currency, no options analytics); Scalper kept, adapted to equity/futures;
  enhancements IN: paper trading, opt-in auto-execution, GTT-protected exits, Telegram bot (inspired by freqtrade's
  Telegram RPC — feature design only, GPL code not copied); options backtesting OUT.
- [x] REQUIREMENTS v2 (reviews + scope applied; FEATURES.md 228 features; CHANGELOG-v2.md; verify-v2.py all PASS)
- [x] freqtrade Telegram research → research/areas/08-freqtrade-telegram.md (42 draft FR-TG, lib github.com/go-telegram/bot MIT)
- [x] USER DECISION 2026-10-12: indicators use github.com/markcheno/go-talib (MIT, pure-Go TA-Lib port) for every indicator it
  covers; only non-TA-Lib indicators (Supertrend, VWAP, Ichimoku, pivots/CPR, …) are written in-house. Avoid cinar/indicator (AGPL).
- [x] REQUIREMENTS v2.1: + FR module TG (42), FR-ACCT-014 → pointer, FR-EQB-038 go-talib clause, NFR-05 Telegram confirmation,
  OQ-45…49. 307 FRs in §6, 72 in §10. verify-v2.py: 0 FAIL.
- [x] ROADMAP docs/plan/ROADMAP.md: P0 55 · P1 61 · P2 58 · P3 35 · P4 36 · P5 44 · P6 18 = 307 (verify_roadmap.py PASS)
- [x] USER APPROVED 2026-10-12: REQUIREMENTS v2.2 + ROADMAP. OQ-09 → Kite rejections only; OQ-45 → kill switch keeps GTT/SL.
- [x] Indicator engine = in-house `ta/` (TA-Lib v0.8.1 port made in another session; verified here: 1,824 oracle tests pass);
  go-talib superseded. Git repo initialised (local main): commits docs baseline + ta + TA-Lib submodule (v0.8.1).
- [x] Architecture spec docs/architecture/ARCHITECTURE.md (committed; versions verified on registries; Q-A1..3 decided by lead)
- [~] Phase-0 plans (writing-plans): index docs/superpowers/plans/2026-10-12-p0-index.md; 5 sub-plans being written in
  parallel (p0-1 platform core, p0-2 market data, p0-3 session/service, p0-4 frontend shell, p0-5 chart/watchlists+exit demo)
- [ ] Cross-plan consistency review (interfaces Produces/Consumes) → fix → user review + execution method choice
