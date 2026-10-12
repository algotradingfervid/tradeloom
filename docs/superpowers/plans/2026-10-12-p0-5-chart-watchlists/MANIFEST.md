# P0-5 Chart, Watchlists & Ticker UI Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Give the Tradeloom P0 build a live candlestick chart (dashboard + full view), multiple watchlists with live prices, the shared instrument picker and the index ticker strip, then run the P0 exit demo and NFR checks that close Phase 0.

**Architecture:** Every chart sits behind the `ChartAdapter` TypeScript interface (ARCHITECTURE §8.5). `LightweightChartsAdapter` is the only file that imports `lightweight-charts`, and it is first built as the S0-5 spike. A framework-free `ChartDataController` handles history loading, lazy loading of older data, live candles from the Go builder (`candle.forming`/`candle.closed`) and refresh. React views (`ChartView`, `WatchlistPanel`, `IndexTickerStrip`, `InstrumentPicker`) read zustand stores (P0-4's `useTicksStore` through `useQuote`, plus `useInstruments`, `useWatchlists`, `useChartContexts`). They call Go only through thin `src/api/*` helpers over P0-4's `Backend` interface, which this plan extends with `chart`, `instruments`, `quotes` and `watchlists` domains (real Wails bindings from P0-3 in `api/wails.ts`, in-memory versions in `api/mock/`). One `SubscriptionCoordinator` merges every on-screen token set per ticker consumer (`chart`, `watchlist`, `widget`), keeps released tokens subscribed for 30 s, and tracks which tokens Go fell back to polling.

**Tech Stack:** React 19, TypeScript, Vite 8, zustand 5, react-router-dom 7, lightweight-charts **5.2.1** (Apache-2.0, sole dependency fancy-canvas 2.1.0), Vitest 5 + @testing-library/react 16 (jsdom), @playwright/test 1.64 (chromium + webkit projects) against the mock backend; Go 1.25 / Wails v2.16 (consumed, not written here).

**Spec:** `docs/requirements/REQUIREMENTS.md` v2.2: FR-CHART-001/002/003/004/005/006/011/016/017, FR-WATCH-001/002/003/004/005/006/007/013/014 (UI), FR-SHELL-004, NFR-07/08/11/15. `docs/architecture/ARCHITECTURE.md` §5.4, §5.6, §8 (especially §8.5), §11, §12 rows 5b/5c/7, §12.1 S0-5. `docs/plan/ROADMAP.md` §P0 exit criteria. Plan index: `docs/superpowers/plans/2026-10-12-p0-index.md`.

## Global Constraints

- Charting library: `lightweight-charts` exactly `5.2.1` (pinned, no caret). Only `frontend/src/chart/LightweightChartsAdapter.ts`, `frontend/src/chart/lwc.ts`, `frontend/src/chart/time.ts`, `frontend/src/chart/primitives/*` and the spike harness may import it. Host views use `ChartAdapter` only (FR-CHART-001). An ESLint `no-restricted-imports` rule enforces this (Task 3).
- Chart type: candlestick only, with a volume histogram (none for indices).
- Timeframes: exactly 8, in this order: `1 Minute, 3 Minutes, 5 Minutes, 10 Minutes, 15 Minutes, 30 Minutes, 1 Hour, 1 Day`, mapped 1:1 to Kite `minute, 3minute, 5minute, 10minute, 15minute, 30minute, 60minute, day`.
- Default timeframe for the dashboard comes from the Settings → Chart setting `chart.default_timeframe`. It ships as `1 Hour` (`"60minute"`, OQ-23). The dashboard's first-run symbol is `NIFTY 50` (`NSE:NIFTY 50`).
- Initial history windows (FR-CHART-003): 1m → 5 trading days; 3m/5m → 15; 10m/15m → 30; 30m/1h → 120; 1d → 750 trading days (at least 740 daily candles).
- Time axis, crosshair label and legend are always in IST (UTC+05:30, no DST), whatever the OS time zone. The crosshair time label format is `01 Oct '26 09:15`.
- Indian formats come only from `src/i18n/format.ts` (P0-4): prices `22,520.45` (en-IN grouping, 2 decimals by default), percents `+0.44%`, quantities `2.54L` / `1.32Cr`. Missing values show `—`, never `0.00`.
- Instrument names come only from `displayName()` / `<InstrumentLabel>` (FR-WATCH-013). Exchange tags come only from `exchangeTag()` and are one of `INDICES`, `NSE`, `BSE`, `NFO-FUT`, `BFO-FUT` (FR-WATCH-014). Identifiers use `EXCHANGE:TRADINGSYMBOL`.
- No B/S order actions in P0: no Buy/Sell buttons, no "Create order at <price>", no order lines. The build uses tag `noorders` (G-0, FR-ORD-013).
- NFR-15: no option, MCX or currency rows anywhere. The UI also drops any row whose `segment` is not one of the five tags (`isInScope`), even if the backend sends one.
- No literal colours in `src/` outside `src/design/` (ESLint rule from P0-4). The chart reads its colours from P0-4's theme tokens (`useTheme().tokens`) plus a few chart-only CSS custom properties in `src/design/tokens.css`.
- `attributionLogo: false`. Clicking the logo would navigate the single Wails webview to tradingview.com and break the outbound host allow-list (NFR-04). The Apache-2.0 attribution is shown on the About page instead (Task 3, `CHART_ATTRIBUTION`).
- Window: no horizontal scroll at ≥ 1280×720 (NFR-11). Every hover action has a keyboard equivalent.
- Frontend commands run from `frontend/`. Go commands run from the repo root with `GOWORK=off` and `-tags webkit2_41,noorders`.

## Review Focus

1. **Instrument with no history (newly listed, or Kite returns nothing).** The chart must show "No candles yet for this instrument", stop asking for older data, and start drawing as soon as the first live candle arrives. It must not spin forever or keep calling `Candles`, and a pending backfill job is waited for at most 30 s. Tests: Task 5, `"empty history shows empty state and never lazy-loads"` and `"a backfill that never reports is abandoned after 30 s"`.
2. **Switching symbols quickly while history is still loading (race).** Only the last request may reach the chart, and the live subscription of every abandoned symbol must be closed. Test: Task 5, `"out-of-order responses: only the latest load reaches the chart and stale live subs are closed"`.
3. **Market closed or holiday, with a stale last candle.** The chart shows the last session's candles without inventing a bar. The header and pills show the last price and change from the `Watch` snapshot (in-memory, this session), the loading skeleton clears, and nothing ever shows `0.00`. Test: Task 10, `"market closed: stale last candle stays, cached quote shown, no phantom bar"`.
4. **Tick or closed-candle event for a bar older than the last bar** (a late `candle.closed` for 10:00 after the 10:05 candle has started). lightweight-charts throws `Cannot update oldest data` for this. The adapter must update the bar in place with `historicalUpdate`, or ignore it if that bar is not loaded, and never throw. Test: Task 4, `"late close for an older loaded bar updates in place; unknown older bar is ignored without throwing"`.
5. **Watchlist of 200+ symbols that exceeds the live subscription budget.** Rows Go could not stream show a "polled" hint and still update. The coordinator sends one merged `Watch` call (not one per row) and does not retry the polled tokens as live in a loop. Tests: Task 6, `"250 tokens: one Watch call, budget overflow marked polled"`, and Task 14, `"rows beyond the budget show the polled hint"`.

---

## Consumes (from P0-3/P0-4)

These names are **canonical** (consistency review, 2026-10-12, contract §C/§D/§E/§F). P0-3 is authoritative for the Wails bindings and events and was extended with everything this plan needs; P0-4 Tasks 1–6, 12 and 19 are authoritative for the shell. Every name below is used verbatim. Only `src/api/backend.ts`, `src/api/wails.ts` and `src/api/mock/*` touch the generated bindings; every other file in this plan goes through the typed helpers of Task 2.

**P0-3 — Wails bindings** (generated into `frontend/wailsjs/go/api/<Struct>.{js,d.ts}`; models in `frontend/wailsjs/go/models.ts` under the Go package namespaces, here `service.*`; JSON fields camelCase; times are Unix ms `…Ms`; a rejected promise carries the JSON string `{"code","stage","reason"}`, read with P0-4's `parseBackendError`):

| Binding | Signature (TS as generated) | Behaviour |
|---|---|---|
| `Chart.Candles` | `Candles(arg1: service.CandlesRequest): Promise<service.CandlesResult>` | `CandlesRequest{token; interval /*Kite form*/; fromMs; toMs; tradingDays}`. History is asked as `tradingDays` with `toMs` (0 = now); Go resolves `fromMs` = start of the N-th trading day before `toMs` from the calendar. Returns `CandlesResult{candles: CandleDTO[]; complete: boolean; backfillJobId: string; needsDataPlan: boolean; offline: boolean}`. A non-empty `backfillJobId` means missing ranges were submitted to backfill at `Interactive` priority: the UI waits for `backfill.progress` with that `jobId` to reach `state: 'done' \| 'failed'` (at most 30 s) and calls `Candles` again. For an older-window request `reachedStart = complete && candles.length === 0`. `needsDataPlan: true` (DataPlan `none`) is a normal result, not a rejection: the chart shows the `no_data_plan` status and no historical call is made. `offline: true` = served from the candle store only. |
| `Chart.Refresh` | `Refresh(arg1: service.RefreshRequest): Promise<service.CandlesResult>` | `RefreshRequest{token; interval; fromMs; toMs}`. Re-fetches from Kite at `Interactive`, upserts with source `manual` (FR-CHART-006). Refusals `no_data_plan` / `offline`. |
| `Chart.OpenLive` / `Chart.CloseLive` | `OpenLive(token: number, interval: string): Promise<string>`; `CloseLive(id: string): Promise<void>` (Wails generates positional parameters) | `OpenLive(token, interval)` registers ticker consumer `ConsumerChart` (quote mode) and `builder.Track(token, iv)`, returns a subscription id. `CloseLive(id)` untracks when no other live subscription uses (token, interval). |
| `Instruments.Search` | `Search(arg1: string, arg2: string[], arg3: number): Promise<service.InstrumentDTO[]>` | `Search(q, tags, limit)`; `tags` empty = all, values = `INDICES`, `NSE`, `BSE`, `NFO-FUT`, `BFO-FUT`. **An empty query returns an empty list**: every search box shows "Type to search" until the user types. Ranking follows FR-WATCH-008. Never returns options, MCX or currency. |
| `Instruments.ByTokens` | `ByTokens(arg1: number[]): Promise<service.InstrumentDTO[]>` | Unknown tokens are omitted. |
| `Instruments.ByIdentifier` | `ByIdentifier(arg1: string): Promise<service.InstrumentDTO>` | `"NSE:NIFTY 50"` → the instrument. Rejects with code `not_found`. |
| `Quotes.Watch` | `Watch(arg1: string, arg2: number[]): Promise<service.WatchResult>` | `Watch(consumer, tokens)`; consumer `chart` \| `watchlist` \| `widget` (`invalid_consumer` otherwise). Replaces that consumer's whole set (30 s linger on removal, FR-WATCH-006). Tokens the feed cannot carry (`*ticker.ErrBudget`, `feed.ErrNotRunning`, DataPlan `none`) are polled by Go every 5 s and published on `tick.batch` with `polled: true`. `WatchResult{live: number[]; polled: number[]; snapshot: TickDTO[]}`; `snapshot` is the **in-memory** last quote per token for this session (not persisted across restarts). |
| `Watchlists.List` | `List(): Promise<service.WatchlistsState>` | `WatchlistsState{lists: Watchlist[]; activeId: string}`, `Watchlist{id; name; items: WatchlistItem[]}`, `WatchlistItem{token; exchange; symbol; displayName; name; isIndex; missing}` (`missing` = token no longer in the instrument master). A fresh install has one empty `"Watchlist 1"`. |
| `Watchlists.Create` | `Create(arg1: string): Promise<service.WatchlistsState>` | Name only. A seeded create (picker "Save as watchlist") is `Create(name)` followed by `Add(id, token, -1)` per token. Name errors `name_invalid`, `name_exists`. |
| `Watchlists.Rename` / `Delete` / `SetActive` | `Rename(arg1: string, arg2: string)`, `Delete(arg1: string)`, `SetActive(arg1: string)`, each `Promise<service.WatchlistsState>` | `Delete` of the last list leaves a new empty `"Watchlist 1"`. The active list persists in Go (`watchlist.active`). |
| `Watchlists.Add` | `Add(arg1: string, arg2: number, arg3: number): Promise<service.WatchlistsState>` | `Add(id, token, position)`; position −1 = append; duplicate → `already_in_list`. |
| `Watchlists.Remove` | `Remove(arg1: string, arg2: number): Promise<service.RemovedItem>` | `RemovedItem{listId; token; position}` (used by Undo). |
| `Watchlists.Reorder` / `ReorderLists` | `Reorder(arg1: string, arg2: number[])`, `ReorderLists(arg1: string[])`, each `Promise<service.WatchlistsState>` | Full new order. |
| `Settings.GetUiState` / `SetUiState` | `GetUiState(key): Promise<string>` (`""` when absent); `SetUiState(key, value): Promise<void>` | Reached through P0-4's `backend.settings.getUiState/setUiState`. This plan uses key `chart.dashboard` (JSON `{identifier, timeframe}`). |
| Setting | `chart.default_timeframe` (one of the 8 Kite intervals, default `"60minute"`) | Rendered by P0-4's Settings → Chart group; read here with `useSetting('chart.default_timeframe', '60minute')`. |

DTOs (`service.*`, JSON): `InstrumentDTO{token, exchange, segment, tag, tradingsymbol, name, displayName, instrumentType, expiryMs /*0 = none*/, tickSizePaise, lotSize, isIndex, isFuture, underlying}`; `TickDTO{token, ltp, close /*previous close*/, change, changePct, volume, tsMs, polled}`; `CandleDTO{t /*ms, candle start*/, o, h, l, c, v, oi, complete}`; `CandleEventDTO{token, interval, candle, partial}`.

**P0-3 — events** (`frontend/src/api/events.gen.ts`; P0-4's `Backend.on(name, cb): () => void` delivers them):

| Event | Payload | Emission |
|---|---|---|
| `tick.batch` | `TickDTO[]` | merged by token / 100 ms. P0-4's event bridge already applies it to `useTicksStore`. |
| `candle.forming` | `CandleEventDTO[]` (**array**) | latest per (token, interval) / 100 ms |
| `candle.closed` | `CandleEventDTO` | immediate |
| `backfill.progress` | `Progress[]` = `{jobId, token, interval, done, failed, total, etaNs, state: 'running' \| 'done' \| 'failed'}[]` | latest per job / 100 ms |
| `feed.state` | `FeedState{state: 'stopped' \| 'connecting' \| 'live' \| 'reconnecting' \| 'offline'; …}` | latest / 100 ms. "Logged out" and "no data plan" are never feed states: they come from `useSessionStore` (`session.state`, `session.dataPlan`). |

**P0-4 — frontend shell (frozen names, contract §F; all under `frontend/src/`):**

- `api/backend.ts`: `interface Backend` (domains + `on(name, cb): () => void`), `interface EventMap`, `type EventName`, model types incl. `TickDTO`, `SessionStatus`, `FeedState`. This plan **extends** `Backend` with `chart`, `instruments`, `quotes`, `watchlists` and `EventMap` with `candle.forming`, `candle.closed`, `backfill.progress` (Task 2). `settings.getUiState(key)` / `settings.setUiState(key, value)` already exist.
- `api/wails.ts` `createWailsBackend()`; `api/mock/index.ts` `createMockBackend(seed, opts?)`, `MockState`, `MockBackend` (`state`, `calls`, `emit(name, payload)`, `failNext(method, reason, code?)`, `callsTo(method)`, `listenerCount(name)`); `api/mock/fixtures.ts` `demoSeed()`, `personalSeed()` (ready session with `dataPlan: 'none'`), `readySession(overrides)`, `seedByName(name)`; `api/index.ts` `defaultBackend()` exposes the mock as `window.__tradeloomMock` and reads `?seed=<name>`; `api/BackendContext.tsx` `BackendProvider`, `useBackend()`; `api/errors.ts` `parseBackendError(e): BackendError{code, stage, reason}`.
- Stores: `store/ticks.ts` `useTicksStore` (`latest: Record<number, TickDTO>`, `applyBatch(ticks)`) + `useLtp(token)` — the **one** tick store; `store/session.ts` `useSessionStore` (`session: SessionStatus | null`, `profile`); `store/status.ts` `useStatusStore` (`feed`, `market`, `clock`, `setFeed`, …; zustand, so `.subscribe((s, prev) => …)` works); `store/settings.ts` `useSetting<T>(key, fallback)` → `[value, set]`, `writeSetting(backend, key, value)`, `useSettingsStore`; `store/toasts.ts` `pushToast({ level, message, action?: { label, onSelect } })`, `useToastStore`, `TOAST_DURATION_MS` (info = 5000 ms); `store/reset.ts` `registerReset(fn)` (the Vitest setup calls `resetAllStores()` after each test).
- `i18n/format.ts`: `formatPrice(v, tickDecimals = 2)` → `22,520.45`; `formatSigned(v, digits = 2)` → `+100.00`; `formatPercent(v)` → `+0.44%` (already signed); `formatCompact(v)` → `2.54L`, `1.32Cr`, `99,999`; `DASH` (`—`).
- `design/tokens.ts` `colourTokens`, `ColourTokens` (`bg, surface, surface2, border, text, textMuted, accent, onAccent, up, down, warn, flat, focus, overlay, shadow`), `ThemeName`; `app/theme.tsx` `applyTheme(name)` (writes the CSS variables `--bg`, `--up`, `--down`, `--border`, … on `<html>` and sets `data-theme`), `ThemeProvider({ mode, followSystem })`, `useTheme(): { theme; tokens }`. `design/tokens.css` holds spacing/type; `.visually-hidden` exists in `design/base.css`.
- Components: `Button({ variant?, icon?, …button props })`; `IconButton({ label, icon: IconName, tooltip?, size?, …button props })` (pressed state via `aria-pressed`); `Icon` names used here: `back`, `refresh`, `more`, `search`, `close`, `plus`, `trash`, `check`; `Dialog({ open, title, description?, onClose, children, footer?, size? })` (focus trap; Escape, × and backdrop call `onClose`); `ConfirmDialog({ open, title, message, confirmLabel, onConfirm, onCancel, tone? })`; `Skeleton({ rows?, label })`.
- `shortcuts/ShortcutProvider.tsx` `useShortcut(actionId, handler: () => void)`; the registry holds `chart.fullView` (F11, global), `watchlist.moveUp` (Alt+ArrowUp) and `watchlist.moveDown` (Alt+ArrowDown); the chart container sets `data-shortcut-scope="chart"`.
- `app/routes.tsx` `shellChildren`, `rootChildren`, `buildRoutes()`; `app/router.ts` `createAppRouter(initialEntries?)` (memory router); `app/ShellLayout.tsx` renders `<TopBar ticker={null} … />` (this plan edits only the `ticker` prop); `app/HomeLayout.tsx` `HomeLayout({ topRow?, main, below?, right })`; `views/home/slots.tsx` `HomeChartArea()` / `HomeWatchlistArea()` (this plan replaces both bodies, P0-4 Task 12).
- `test/render.tsx` `renderApp({ route?, backend? })`; `test/setup.ts` (jsdom + jest-dom + store reset).
- `e2e/helpers.ts` `P0_VIEWS`, `openApp(page, query?)`, `go(page, route)` (navigates through `app.navigate`; the router has no URL bar), `tabTo(page, locator)`; `playwright.config.ts` projects `chromium` + `webkit`, `webServer` on `npm run dev:mock` (= `vite --mode mock`, `VITE_BACKEND=mock` from `.env.mock`); CI job `ui-e2e` (P0-4 Task 19).

**P0-1:** make targets `deb`, `appimage`, `docker-release` (container build: binary, G-0 `go tool nm` check, `.deb`, AppImage, `SHA256SUMS`); artefacts `dist/tradeloom_<v>_amd64_noorders.deb` and `dist/Tradeloom-<v>-x86_64_noorders.AppImage`. CI jobs `go-unit`, `ta`, `arch-lint`, `frontend`, `licence`, `linux-package`, `install-smoke` (+ `ui-e2e` from P0-4). The binary accepts `--smoke`; `TRADELOOM_DATA_DIR` overrides the data directory. Secrets are stored with go-keyring (attributes `service=tradeloom`, `username=<key>`).

## Ordering (from the consistency review)

Contract §G rows 2–6 (P0-1 → P0-2 → P0-3 strictly before all of this):

| # | Must land first | Before these P0-5 tasks | Why |
|---|---|---|---|
| 2 | P0-4 Task 1 | every P0-5 task | theme tokens, `applyTheme`, `formatPrice` and the other formatters |
| 3 | P0-4 Task 2 | Tasks 2, 6, 7, 13 | `Backend`, mock backend, `useTicksStore`/`useSessionStore`/`useStatusStore`/`useSetting` |
| 4 | P0-4 Tasks 3, 4, 6 | Tasks 8–17 | primitives (`Button`, `IconButton`, `Dialog`, `ConfirmDialog`, `Skeleton`), `pushToast`, `useShortcut` |
| 5 | P0-4 Task 12 | Tasks 10, 16 | `src/views/home/slots.tsx` |
| 6 | P0-4 Task 19 | Tasks 18, 19 | `e2e/helpers.ts`, the `ui-e2e` CI job |

P0-5 Tasks 1, 3, 4 and 5 may run in parallel with P0-4 Tasks 2–19.

## Renamed in the consistency review (2026-10-12)

Renamed from the first draft of this plan (for reviewers diffing versions): `Chart.History`/`HistoryRequest` → `Chart.Candles`/`CandlesRequest`; `Watchlists.AddItem`/`InsertItem`/`RemoveItem`/`GetActive` and `WatchlistDTO` → `Add`/`Remove`/`List().activeId` and `WatchlistsState`; `api.InstrumentDTO`/`api.WatchlistDTO` → `service.*`; `onEvent(` → `backend.on(`; `parseBindingError` → `parseBackendError`; `loadUiState`/`saveUiState` → `readUiState`/`writeUiState` (`src/api/uiState.ts`); `useFeed` → `useStatusStore`; `useShell` → route outside the shell; `formatSignedPrice`/`formatSignedPercent`/`formatQuantity` → `formatSigned`/`formatPercent`/`formatCompact`; `toast.info` → `pushToast`; `Spinner` → `Skeleton`; `appRoutes` → `rootChildren`; TopBar `data-slot` → `ShellLayout` `ticker` prop; `prevClose` → `close`/`previousClose`; `tickSize` → `tickSizePaise`; `chart.defaultTimeframe` → `chart.default_timeframe`; `registerMockHandlers`/`emitMockEvent`/`window.__tlMock`/`?scenario=` → mock `Backend` domains, `window.__tradeloomMock.emit`, `?seed=`; `make package` → `make docker-release`.

## Produces (for P1+ and for P0-4's consumers)

`ChartAdapter`, `LightweightChartsAdapter`, `ChartDataController` (with the backfill wait), `TIMEFRAMES` / `TimeframeId` / `parseTimeframe`, `Instrument`, `exchangeTag` / `displayName` / `instrumentId` / `isInScope` / `tickRupees` / `itemTag` / `itemIdentifier` / `labelOfItem`; Backend domains `chart`, `instruments`, `quotes`, `watchlists` (real + mock) and the typed helpers `fetchHistory`, `refreshRange`, `openLive`, `closeLive`, `onCandle`, `onBackfillProgress`, `watchQuotes`, `searchInstruments`, `instrumentsByTokens`, `instrumentByIdentifier`, `watchlistsApi`, `setBackend` / `currentBackend`; `readUiState` / `writeUiState` (`src/api/uiState.ts`); `useQuote` / `quoteChange` / `applySnapshot` (selectors over P0-4's `useTicksStore`), `subscriptions` / `useSubscription` / `usePolled`; `useInstruments` / `useInstrument` / `resolveIdentifier`; `<InstrumentLabel>`, `<PriceChange>`, `<InstrumentRow>`, `<ChartView>`, `<DashboardChart>`, `useChartContexts` / `DASHBOARD_CONTEXT`, `<SymbolSearchDialog>`, `<WatchlistPanel>`, `useWatchlists`, `<InstrumentPicker>`, `<IndexTickerStrip>`, `<MarketDataBridge>`, route `/chart-full` (in `rootChildren`) with `openFullViewChart`, and `CHART_ATTRIBUTION`.

**Note for the consistency reviewer:** the holdings card (P0-4, FR-HOME-002) reads `useTicksStore` directly; for live LTPs on holdings it should register its tokens with `useSubscription('holdings-card', 'widget', tokens)` from this plan so the coordinator sends one merged `Quotes.Watch('widget', …)` (ARCHITECTURE §8.2: one tick store, one coordinator).

## Deliberately not in P0 (FR parts owned by later phases)

FR-CHART-002 Save/Clear/Indicators buttons (FR-CHART-007/008, P1). FR-CHART-004 AC2 (RSI kept across timeframes, P1 studies). FR-CHART-004 AC3 (Technicals tab, ANLY). FR-CHART-011 "+" crosshair button and gear (FR-CHART-014/015, FR-ORD-016, P4/P6). FR-CHART-012 Buy/Sell (P4). FR-CHART-016 AC3 (Scalper, P6). FR-CHART-017 AC1/AC2 scanner entry point (P3, which consumes `openFullViewChart`). FR-WATCH-005 B/S buttons and AC2 (P4; hidden while trading is not compiled). FR-WATCH-007 AC1 and universe chips (universe store, P1). FR-WATCH-003 AC3 (strategies copy instruments, P1). `ChartAdapter.addDrawing`/`setOrderLines`/`setMarkers` from §8.5 are added in P1/P4/P6. The P0 interface leaves them out (YAGNI).

---

## File Structure

```
frontend/
├── package.json                                  modify: add lightweight-charts 5.2.1 (exact)
├── eslint.config.js                              modify: no-restricted-imports for lightweight-charts
├── spike/s05.html, spike/s05.ts                  S0-5 harness (dev server only, not in the prod build)
├── e2e/
│   ├── spike-s05.spec.ts                         S0-5 go/no-go checks
│   ├── chart-watchlists.spec.ts                  P0 chart/watchlist/ticker flows on the mock backend
│   ├── layout-1280.spec.ts                       NFR-11 1280×720 check
│   ├── naming-consistency.spec.ts                FR-WATCH-013 AC3 / FR-WATCH-014 AC2
│   └── helpers.ts                                modify (P0-4 file): add /chart-full to P0_VIEWS
└── src/
    ├── chart/
    │   ├── types.ts                              Bar, InstrumentSpec, ChartTheme, StudySpec, PriceBandSpec, LegendData…
    │   ├── ChartAdapter.ts                       ChartAdapter interface + StudyLimitError
    │   ├── lwc.ts                                LwcLib injection seam (real lib or fake)
    │   ├── LightweightChartsAdapter.ts           the only implementation
    │   ├── primitives/PriceBandPrimitive.ts      custom ISeriesPrimitive (S0-5 "custom primitive")
    │   ├── time.ts                               IST shift, crosshair/tick-mark formatters
    │   ├── bars.ts                               dedupeSorted, mergeBars
    │   ├── format.ts                             precisionForTick, chart price formatter
    │   ├── theme.ts                              readChartTheme(tokens) from P0-4 colour tokens + chart CSS vars
    │   ├── timeframes.ts                         TIMEFRAMES, parseTimeframe
    │   ├── ChartDataController.ts                load / backfill wait / lazy-load / live / refresh / race guard
    │   ├── attribution.ts                        CHART_ATTRIBUTION
    │   ├── testing/fakeLwc.ts                    fake lightweight-charts for adapter tests
    │   ├── testing/FakeAdapter.ts                fake ChartAdapter for controller/view tests
    │   └── ui/ChartView.tsx, ChartHeader.tsx, TimeframeSelect.tsx, ChartLegend.tsx,
    │          ScaleToggles.tsx, SymbolSearchDialog.tsx, DashboardChart.tsx, chart.css
    ├── instruments/types.ts, naming.ts, fixtures.ts
    ├── api/
    │   ├── backend.ts, wails.ts                  modify (P0-4 files): chart/instruments/quotes/watchlists domains + events
    │   ├── mock/index.ts, mock/fixtures.ts       modify (P0-4 files): MockState market fields, wrapped domains
    │   ├── mock/market.ts                        in-memory Chart/Instruments/Quotes/Watchlists for Vitest, dev:mock and e2e
    │   ├── current.ts                            setBackend / currentBackend (registered by MarketDataBridge)
    │   ├── call.ts, chart.ts, instruments.ts, quotes.ts, watchlists.ts   typed helpers over Backend
    │   └── uiState.ts                            readUiState / writeUiState over settings.getUiState/setUiState
    ├── market/quotes.ts, subscriptions.ts, index.ts, hooks.ts, MarketDataBridge.tsx, testing.ts
    ├── store/instrumentsStore.ts, chartContextsStore.ts, watchlistsStore.ts
    ├── components/instrument/InstrumentLabel.tsx, PriceChange.tsx, InstrumentRow.tsx, instrument.css
    ├── app/IndexTickerStrip.tsx, tickerStrip.css   modify (P0-4 files): App.tsx, ShellLayout.tsx, routes.tsx
    ├── views/watchlists/WatchlistPanel.tsx, WatchlistTabs.tsx, WatchlistRows.tsx, WatchlistSearch.tsx,
    │                    InstrumentPicker.tsx, names.ts, watchlists.css
    ├── views/chart-full/ChartFullView.tsx
    ├── views/home/slots.tsx                      modify (P0-4 file): HomeChartArea / HomeWatchlistArea bodies
    └── design/tokens.css                         modify: chart-only CSS variables per data-theme
docs/architecture/spikes/S0-5-chart.md            S0-5 output (go/no-go record)
docs/plan/P0-exit-record.md                       P0 exit demo + NFR evidence
```

Responsibilities: `chart/` knows nothing about Wails or the Backend. `api/` is the only directory that touches the Backend implementations (`wails.ts` is the only importer of `wailsjs`). `market/` owns subscriptions and quote selectors over P0-4's tick store. `store/` holds the domain stores. `views/` and `components/` hold React UI only.

---

<!-- generated task table: begin (plan_split.py) -->
## Task files

Each implementer reads this manifest plus its one task file. Sizes are approximate (characters / 4).
"Consumes tasks" lists earlier tasks named in the task's Consumes lines. Dispatch in number order
unless Global Constraints or the P0 index state a different execution order.

| # | Task | File | ~tokens | Consumes tasks |
|---|---|---|---|---|
| 1 | Timeframes, instrument model, naming and scope guard | [`tasks/01-timeframes-instrument-model-naming-and-scope.md`](tasks/01-timeframes-instrument-model-naming-and-scope.md) | 3.9k | — |
| 2 | Backend market domains (real + mock) and typed helpers | [`tasks/02-backend-market-domains-and-typed-helpers.md`](tasks/02-backend-market-domains-and-typed-helpers.md) | 9.7k | 1 |
| 3 | Spike S0-5 — `ChartAdapter` prototype on lightweight-charts 5.2.1 (go/no-go for OQ-01) | [`tasks/03-spike-s0-5-chartadapter-prototype-on.md`](tasks/03-spike-s0-5-chartadapter-prototype-on.md) | 11.9k | 1, 2 |
| 4 | Adapter completion — live bars, legend data, scales, navigation, instrument spec, study limit | [`tasks/04-adapter-completion-live-bars-legend-data.md`](tasks/04-adapter-completion-live-bars-legend-data.md) | 2.6k | 3 |
| 5 | `ChartDataController` — history, backfill wait, lazy loading, live candles, refresh, race guard | [`tasks/05-chartdatacontroller-history-backfill-wait-lazy-loading.md`](tasks/05-chartdatacontroller-history-backfill-wait-lazy-loading.md) | 6.8k | 1, 2 |
| 6 | Quote selectors over the one tick store, subscription coordinator and market-data bridge | [`tasks/06-quote-selectors-over-the-one-tick.md`](tasks/06-quote-selectors-over-the-one-tick.md) | 4.6k | 2 |
| 7 | Instrument store and shared instrument UI (`InstrumentLabel`, `PriceChange`) | [`tasks/07-instrument-store-and-shared-instrument-ui.md`](tasks/07-instrument-store-and-shared-instrument-ui.md) | 2.9k | 1, 2, 6 |
| 8 | Chart header and timeframe selector | [`tasks/08-chart-header-and-timeframe-selector.md`](tasks/08-chart-header-and-timeframe-selector.md) | 2.8k | — |
| 9 | Chart symbol search dialog (FR-CHART-005) | [`tasks/09-chart-symbol-search-dialog.md`](tasks/09-chart-symbol-search-dialog.md) | 2.9k | — |
| 10 | `ChartView`, chart contexts, persistence and the dashboard chart | [`tasks/10-chartview-chart-contexts-persistence-and-the.md`](tasks/10-chartview-chart-contexts-persistence-and-the.md) | 8.1k | — |
| 11 | Full-view chart (FR-CHART-017) | [`tasks/11-full-view-chart.md`](tasks/11-full-view-chart.md) | 2.7k | — |
| 12 | Index ticker strip (FR-SHELL-004) | [`tasks/12-index-ticker-strip.md`](tasks/12-index-ticker-strip.md) | 2.3k | — |
| 13 | Watchlists store and naming rules | [`tasks/13-watchlists-store-and-naming-rules.md`](tasks/13-watchlists-store-and-naming-rules.md) | 4.1k | 2 |
| 14 | Instrument row and watchlist rows (live prices, polled hint, remove/undo, reorder) | [`tasks/14-instrument-row-and-watchlist-rows.md`](tasks/14-instrument-row-and-watchlist-rows.md) | 4.6k | — |
| 15 | Inline search and add to watchlist (FR-WATCH-004) | [`tasks/15-inline-search-and-add-to-watchlist.md`](tasks/15-inline-search-and-add-to-watchlist.md) | 2.3k | — |
| 16 | Watchlist panel — tabs, create, rename, delete; mount on Home | [`tasks/16-watchlist-panel-tabs-create-rename-delete.md`](tasks/16-watchlist-panel-tabs-create-rename-delete.md) | 3.5k | — |
| 17 | Instrument picker dialog (FR-WATCH-007) and "Add instruments…" on the panel | [`tasks/17-instrument-picker-dialog-and-add-instruments.md`](tasks/17-instrument-picker-dialog-and-add-instruments.md) | 4.6k | — |
| 18 | End-to-end flows on the mock backend (Playwright) | [`tasks/18-end-to-end-flows-on-the.md`](tasks/18-end-to-end-flows-on-the.md) | 3.9k | 2, 6 |
| 19 | P0 exit demo + NFR runs | [`tasks/19-p0-exit-demo-nfr-runs.md`](tasks/19-p0-exit-demo-nfr-runs.md) | 3.4k | — |

Coverage and self-review: [`APPENDIX.md`](APPENDIX.md).
<!-- generated task table: end -->