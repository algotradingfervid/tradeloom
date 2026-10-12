# P0-4 Frontend Shell, Account & Data UI Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build Tradeloom's React frontend shell (layout, phase-gated sidebar, top bar, status indicator, toasts, theme, Indian formats, shortcut registry and window state, dialogs, list template, unknown-view fallback, tooltips/help) and the P0 account and data UI: setup / login / risk-disclosure views, read-only Holdings, Positions and Order Book, the Home layout with the Holdings summary card, Profile, Settings (Kite connection, Appearance, Chart default, Market calendar, Data with backup/restore, Shortcuts, Diagnostics, About), the built-in help centre and the legal pages. Everything talks to the P0-3 Wails bindings through one typed adapter and is tested against an in-memory mock of that adapter.

**Architecture:** `frontend/src/api/backend.ts` declares a `Backend` interface (one property per bound Go struct plus `on(event)`); `api/wails.ts` implements it over the generated `frontend/wailsjs` modules (the only importer of `wailsjs`), `api/mock/` implements it in memory for Vitest and for Playwright (Vite `--mode mock`). Domain state lives in zustand stores fed by an event bridge (`api/events.ts`) and by view-mount fetches; routing is a `createMemoryRouter` (react-router-dom 7) whose root layout hosts the session gate, toasts and the shortcut provider, and whose shell layout hosts the top bar, sidebar and views. Colours are TypeScript tokens written to CSS variables at runtime, so the chart (P0-5) reads the same tokens via `useTheme()`.

**Tech Stack:** React 19.3.0 / react-dom 19.3.0, react-router-dom 7.18.4, zustand 5.0.15, Vite 8.3.4 + @vitejs/plugin-react 6.1.2, TypeScript as pinned by P0-1 (ARCHITECTURE §10.1), Vitest 5.0.4 + jsdom 30.1.2, @testing-library/react 16.3.3 + /dom 10.4.2 + /user-event 14.6.7 + /jest-dom 7.0.1, @playwright/test 1.64.0 (WebKit project) + @axe-core/playwright 4.13.0, @types/node 22.20.5, Wails v2.16.0 JS runtime (`EventsOn(name, cb): () => void`, verified in `wails/v2@v2.16.0/internal/frontend/runtime/wrapper/runtime.d.ts:41`). All npm versions checked with `npm view` on 2026-10-12; `createMemoryRouter(routes, {initialEntries})`, `RouterProvider` (re-exported by `react-router-dom` from `react-router/dom`), `NavLink`, `useNavigationType` checked in the 7.18.4 tarball type definitions; `zustand` `create` checked in the 5.0.15 tarball; `@testing-library/jest-dom/vitest` export checked in 7.0.1 `package.json`.

**Spec:** `docs/requirements/REQUIREMENTS.md` v2.2 — FR-SHELL-001/002/005/006/009/010/011/012/013/014/015/016/019/020, FR-ACCT-001 (profile view, index scope), FR-ACCT-004 (logout UI), FR-ACCT-005/006 (setup/login views), FR-ACCT-010/012/013 (UI)/015, FR-HOME-001/002, FR-ORD-001/002/003 (UI), FR-DATA-001 (Settings → Market calendar UI, AC4/AC6), FR-DATA-004 (status tooltip offset), FR-DATA-005 (UI parts), FR-SHELL-018 (Settings → Diagnostics and crash notice UI), NFR-11, §3.2 UI glossary; `docs/architecture/ARCHITECTURE.md` §8 (all), §11 (UI testing), §12 rows 5a/5d/6; `docs/plan/ROADMAP.md` §0 (phase-gated UI rule), §P0, §7 (GATE table); `docs/superpowers/plans/2026-10-12-p0-index.md`. UX reference (clean-room, our own copy and visuals): `docs/research/FEATURES.md` SHELL/HOME/ORD/ACCT blocks.

## Global Constraints

- Window: minimum supported size 1280×720 px; no horizontal frame scrollbar at ≥ 1280 px (NFR-11, FR-SHELL-001).
- Theme: light (default) and dark colour tokens live only in `src/design/tokens.ts` and are applied as CSS variables (`--bg`, `--surface`, `--surface-2`, `--text`, `--text-muted`, `--accent`, `--on-accent`, `--up`, `--down`, `--warn`, `--flat`, `--border`, `--focus`, `--overlay`, `--shadow`); every foreground/background pair ≥ 4.5:1 (WCAG 2.1 AA) in both themes; no colour literal (`#rgb`, `#rrggbb`, `rgb(`, `hsl(`) anywhere in `src/` outside `src/design/`.
- Numbers: Indian grouping via `Intl.NumberFormat('en-IN')`; money prefix `₹` (`₹12,34,567.80`); compact `L` for ≥ 1,00,000 and `Cr` for ≥ 1,00,00,000 with 2 decimals and no space (`2.54L`, `1.32Cr`); percentages signed with 2 decimals (`+0.44%`); missing or non-finite values render `—` (never `0.00`); negative zero never renders (`-0.00` is forbidden).
- Dates: compact `DD-MM-YYYY` (`09-10-2026 15:15:32`) and long `09 Oct 2026, 15:15`; 24-hour; always IST (`Asia/Kolkata`) regardless of the OS time zone; month abbreviations come from our own table (ICU `en-IN` prints `Sept`).
- Keyboard: every hover-revealed action is also visible on keyboard focus, reachable by Tab, and listed in a Shift+F10 / Menu-key / right-click context menu; every dialog closes on Escape (= Cancel), traps focus, restores focus on close and shows a visible × button.
- No Buy/Sell (B/S) order buttons in P0 builds: `isEnabled('orderButtons')` is false below phase 4 and no component in this plan renders one.
- Phase gating: controls whose FR ships in a later phase are hidden (never disabled, never "coming soon"); phase comes from `VITE_PHASE` (default `0`).
- Copy: our own wording only — no Streak name, logo, text, help-centre or legal sentences; UI strings live in `src/i18n/copy.ts`; long-form help and legal text in `src/help/articles.ts` and `src/legal/content.ts`.
- `src/api/wails.ts` is the only module that imports from `frontend/wailsjs`.
- Toolchain floor: Node ≥ 22.22.2 (jsdom 30 `engines`).

## Review Focus

1. **1280 px window with very long instrument names** (≥ 60 characters) in Holdings, Positions and Order Book — expected: the name cell truncates with an ellipsis, the full name is in the cell's `title`, the frame has no horizontal scrollbar and the table does not overflow its container. Test: Task 11, `e2e/portfolio-layout.spec.ts`.
2. **Very large, negative and non-finite numbers in Indian format** — expected: `-₹1,23,45,67,890.50`, `1,00,00,00,00,00,00,000.00`, rollover at rounding boundaries (`9,999,999.999` → `1.00Cr`, `99,999.996` → `1.00L`), `-0.004` → `+₹0.00`, `NaN`/`±Infinity`/`null` → `—`. Test: Task 1, `src/i18n/format.test.ts` "review focus" block.
3. **Backend event storm or disconnect while a dialog is open** — expected: the open dialog stays open with focus inside, the status indicator shows the final state, the panel gains "Log in" when the session expires, and the bridge holds exactly one listener per event. Test: Task 7, `src/app/StatusIndicator.test.tsx` "review focus: an event storm…".
4. **Keyboard-only user** — expected: sidebar via arrows + Enter, table rows via arrows, row actions via Tab from the focused row and via Shift+F10 menu, the whole setup → login → risk → Holdings path without a mouse. Tests: Task 5, `src/components/DataTable.test.tsx` "review focus: keyboard-only user"; Task 19, `e2e/keyboard.spec.ts` (first run → Holdings; row actions on focus and Shift+F10).
5. **Restore while settings are being edited or written** — expected: once a restore starts no settings write reaches the backend (a stale write must not land on the restored database); if the restore fails, writes resume and settings are reloaded from the backend. Test: Task 16, `src/views/settings/DataGroup.test.tsx` "restore suspends settings writes".

---

## Interfaces with other sub-plans

### Consumes (from P0-1)

- `frontend/` scaffold from P0-1 Task 1 ("repo skeleton"): Vite + React + TypeScript (`typescript` 5.9.3) with `package.json` scripts `dev`, `build`, `typecheck` (`tsc --noEmit`), `test` (`vitest run`) — there is **no** `lint` or `dev:mock` script; Task 1 Step 1 adds `"lint": "eslint src e2e"` (with the ESLint devDependencies it installs and `eslint.config.js`), `"dev:mock": "vite --mode mock"` and `"e2e": "playwright test"`. `index.html` contains `<div id="root"></div>`; `src/main.tsx`; `tsconfig.json` with `"strict": true`, `"jsx": "react-jsx"` (Task 1 Step 1 adds `"e2e"` and `"playwright.config.ts"` to `include` if missing). This plan overwrites `vite.config.ts` and `src/main.tsx` and adds dependencies; it does not change the TypeScript version P0-1 pinned.
- CI (`.github/workflows/ci.yml`, P0-1): the `frontend` job (ubuntu-24.04, `actions/setup-node@v7` with `node-version: ${{ env.NODE_VERSION }}`, `cache: npm`, `cache-dependency-path: frontend/package-lock.json`, `working-directory: frontend`) runs `npm ci`, `npm run typecheck`, `npm test`, `npm run build`. P0-1 has **no** `ui-e2e` job: Task 19 adds it (Playwright chromium + WebKit against `VITE_BACKEND=mock`).
- Wails window options `Width: 1440, Height: 900, MinWidth: 1280, MinHeight: 720` (P0-1 `internal/app` options, confirmed in the canonical contract §A).

### Consumes (from P0-3)

Generated TypeScript bindings (Wails `wails generate module`, ARCHITECTURE §8.4): one module per bound struct at `frontend/wailsjs/go/api/<Struct>.js` + `.d.ts` (each function calls `window.go.api.<Struct>.<Method>`); model classes in `frontend/wailsjs/go/models.ts` under the Go package namespaces `service`, `kite`, `orders`, `ticker`, `clock`, `settings`. Every DTO has explicit camelCase `json` tags that `src/api/backend.ts` mirrors field-for-field; times are Unix-ms `number` fields named `…Ms`, except the P0-2 payloads that carry `time.Time` as RFC3339 strings (`ticker.FeedState.nextRetry` / `lastTick`, `clock.Offset.updatedAt`) and `service.BackupInfo.createdAt` (manifest string). Money in portfolio snapshots is integer paise (`…Paise`); ratios are basis points (`…Bp`). Names below are the canonical contract (§C, §D, §E) and are final.

| Binding (Go struct → TS module) | Returns | Used in |
|---|---|---|
| `Session.Status()` | `service.SessionStatus` | Task 2 bootstrap |
| `Session.Credentials()` | `kite.CredentialsInfo` `{configured, apiKeyMasked, port, redirectUrl, secretBackend}` | Task 14 |
| `Session.SaveCredentials(arg1: service.CredentialsInput{apiKey, apiSecret})` | `service.SessionStatus` | Tasks 9, 14 |
| `Session.RemoveCredentials()` | `service.SessionStatus` | Task 14 |
| `Session.SetRedirectPort(arg1: port)` | `kite.CredentialsInfo` (refusal code `invalid_port`) | Task 14 |
| `Session.Login()` | `kite.LoginStart` `{loginUrl, redirectUrl, listenerOk, listenerError}` (opens the system browser; the result arrives as `session.state`) | Task 9 |
| `Session.CompleteLogin(arg1: pasted)` / `Session.CancelLogin()` | `service.SessionStatus` | Task 9 |
| `Session.Logout()` | `kite.LogoutResult` `{remoteInvalidated, message}` | Task 13 |
| `Session.AckRiskDisclosure()` | `service.SessionStatus` | Task 9 |
| `Session.Profile()` | `kite.Profile` | Tasks 8, 13 |
| `Session.SecretStorage()` | `service.SecretStorageInfo` `{mode, backend, keyringAvailable, fileExists, choices}` | Tasks 9, 14 |
| `Session.UseEncryptedFile(arg1: passphrase)` / `Session.RefuseSecretStorage()` / `Session.Unlock(arg1: passphrase)` | `service.SessionStatus` (codes `passphrase_too_short`, `bad_passphrase`) | Task 9 |
| `Session.MigrateToKeyring()` | `service.SecretStorageInfo` (code `keyring_unavailable`) | Task 14 |
| `Session.ReconnectFeed()` | `void` | Task 7 |
| `Portfolio.OrderBook()` / `Portfolio.Positions()` / `Portfolio.Holdings()` | `orders.BookSnapshot` / `orders.PositionsSnapshot` / `orders.HoldingsSnapshot` | Tasks 10–12 |
| `Portfolio.Refresh(arg1: scope)` (`"orders"`, `"positions"`, `"holdings"`, `"all"`) | `void` | Tasks 10–12 |
| `Settings.All()` | `service.SettingValue[]` `{name, valueJson}` (`kite.*` keys hidden) | Task 2 |
| `Settings.Set(arg1: name, arg2: valueJSON)` | `service.SettingValue` (code `invalid`; `kite.*` → `use_credentials_flow`) | Task 2 |
| `Settings.Shortcuts()` / `Settings.Rebind(arg1: actionId, arg2: binding)` / `Settings.ResetShortcuts()` | `settings.ShortcutAction[]` / `settings.ShortcutAction` (codes `conflict`, `invalid`) / `settings.ShortcutAction[]` | Tasks 6, 15, 18 |
| `Settings.GetUiState(arg1: key)` / `Settings.SetUiState(arg1: key, arg2: value)` | `string` (`""` when absent) / `void` | Task 2 (`Backend` surface for P0-5) |
| `Data.Stats()` / `Data.ClearCandles()` / `Data.RefreshInstruments()` | `service.DataStats` / `void` (code `runs_referenced`) / `void` | Task 16 |
| `Data.ChooseFolder()` / `Data.ChooseBackupFile()` | `string` (`""` when cancelled) | Task 16 |
| `Data.BackupNow(arg1: folder, arg2: includeCandles)` / `Data.InspectBackup(arg1: path)` / `Data.Restore(arg1: path, arg2: confirmation)` | job id / `service.BackupInfo` / job id (codes `not_confirmed`, `bad_archive`, `schema_too_new`, `deployments_active`) | Task 16 |
| `Calendar.Info()` / `Calendar.ChooseImportFile()` / `Calendar.Import(arg1: path)` / `Calendar.Upsert(arg1: service.CalendarEntryInput)` / `Calendar.Remove(arg1: exchange, arg2: date)` | `service.CalendarInfo` / `string` / `CalendarInfo` / `CalendarInfo` / `CalendarInfo` | Task 17 |
| `Diagnostics.OpenLogsFolder()` / `Diagnostics.CreateSupportBundle()` / `Diagnostics.LastCrash()` / `Diagnostics.OpenCrashReport()` / `Diagnostics.DismissCrash()` | `void` / `string` path / `service.CrashInfo` `{path, atMs}` (empty `path` = none) / `void` / `void` | Task 15 |
| `System.Info()` | `service.AppInfo` `{version, build, schemaVersion, dataDir, licences}` | Tasks 2, 14 |
| `System.Status()` | `service.StatusSnapshot` `{feed, market, clock}` | Task 2 bootstrap |

Events (`runtime.EventsOn(name, cb)`; P0-3 also emits names and payload types into `frontend/src/api/events.gen.ts` — `src/api/backend.ts` `EventMap` holds exactly these entries of it, and P0-5 adds the chart/watchlist ones):

| Event | Payload | Use |
|---|---|---|
| `session.state` | `service.SessionStatus` | gate, status indicator, banners |
| `feed.state` | `ticker.FeedState` `{state: stopped\|connecting\|live\|reconnecting\|offline, attempt, nextRetry (RFC3339), lastTick (RFC3339), connections, tokens}` | status indicator |
| `clock.offset` | `clock.Offset` `{millis, source: none\|ticks\|http-date, samples, warn, skewed, updatedAt (RFC3339)}` | status tooltip (FR-DATA-004) |
| `market.status` | `service.MarketStatus` `{exchange, phase: pre-open\|continuous\|closing\|post-closing\|closed, dayType: trading\|holiday\|special\|weekend\|inferred-non-trading, label, tooltip, atMs}` | status indicator |
| `tick.batch` | `service.TickDTO[]` `{token, ltp, close, change, changePct, volume, tsMs, polled}` | `useTicksStore` (live LTP overlay) |
| `order.update` | `kite.Order` (gokiteconnect snake_case JSON) | order toast + version bump |
| `orders.changed` / `positions.changed` / `holdings.changed` | `orders.BookSnapshot` / `orders.PositionsSnapshot` / `orders.HoldingsSnapshot` | portfolio store |
| `job.progress` | `service.JobProgress` `{jobId, kind: backup\|restore\|support_bundle, stage, done, total, finished, error, resultPath}` | jobs store (Task 16) |
| `settings.changed` | `settings.Changed` `{key, value}` (`value` is the JSON value) | settings store |
| `app.navigate` | `service.Navigate` `{route}` | router |

Canonical facts this plan relies on (they replace the earlier assumptions A1–A8):

- **A1 Errors.** A rejected binding promise carries the JSON string `{"code","stage","reason"}` (`service.ErrRefused` through `app.ErrorFormatter`); plain Go errors are wrapped as code `internal`. Codes used here: `invalid`, `conflict`, `invalid_port`, `passphrase_too_short`, `bad_passphrase`, `keyring_unavailable`, `use_credentials_flow`, `not_confirmed`, `bad_archive`, `schema_too_new`, `deployments_active`, `runs_referenced`, `internal`.
- **A2 Session states.** `SessionStatus.state` ∈ `no_credentials` (no key/secret stored), `locked` (encrypted-file backend not yet unlocked this start), `logged_out` (credentials present, no valid token — also the startup state when the stored token is past 06:00 IST), `logging_in` (between `Login()` and the redirect), `ready`, `expired` (only while running: 06:00 IST on the app clock or a `TokenException`). `riskAckRequired: true` means the FR-SHELL-020 disclosure must be acknowledged for this `sessionId`. `dataPlan` ∈ `unknown|connect|none`. `secrets.mode` ∈ `keyring|file|choose|none` and `secrets.backend` ∈ `keyring|file|none|locked`; `secrets.choices` (mode `choose` only) = `["encrypted_file", "no_storage"]`. "Logged out" and "no data plan" are derived from `state` / `dataPlan`, never from `feed.state`.
- **A3 Login.** `Session.Login()` starts the loopback listener on `credentials.port`, opens the system browser and returns `LoginStart`; when `listenerOk` is false the UI shows `listenerError` and a paste box whose value goes to `Session.CompleteLogin(pasted)` (redirect URL or bare request token). Success is a `session.state` with `state: "ready"`, `riskAckRequired: true`.
- **A4 Portfolio.** `Portfolio.*` return the Go `orders_cache` snapshots (no Kite call per invocation, §8.2); Go publishes fresh snapshots on `orders.changed` / `positions.changed` / `holdings.changed` and re-prices rows from its own LTP map; the UI overlays the newest `tick.batch` `ltp` per row `token`. `Portfolio.Refresh(scope)` forces a Kite fetch.
- **A5 Restore.** `Data.Restore` re-validates (schema ≤ supported, all deployments flat) and runs as a job; on success the final `job.progress` has `stage: "restart"`, `finished: true` and the app quits with exit code 3 so `main` re-executes itself; a failure is a final `job.progress` with non-empty `error`.
- **A6 Window state.** Size, position and maximised state are saved by Go (`BeforeClose` → UI state key `window.geometry`); the frontend persists only `ui.last_route` and `appearance.sidebar_collapsed` (FR-SHELL-019 "window state").
- **A7 Settings keys** (contract §E; JSON values validated in Go): `appearance.theme` (`"light"`\|`"dark"`, `"light"`), `appearance.follow_system` (bool, `false`), `appearance.sidebar_collapsed` (bool, `false`), `ui.last_route` (string, `"/home"`), `chart.default_timeframe` (one of `minute,3minute,5minute,10minute,15minute,30minute,60minute,day`, `"60minute"` — P0-5 reads it), `log.level` (`"info"`\|`"debug"`, `"info"`), `backup.schedule.enabled` (bool, `false`), `backup.schedule.folder` (absolute path or `""`), `backup.schedule.keep` (int 1–365, `7`), `backup.include_candles` (bool, `false`), `calendar.stale_toast_day` (`YYYY-MM-DD` or `""`). `kite.redirect_port` (1024–65535, `7420`) and `kite.api_key` are hidden from `Settings.All` and changed only through `Session.SetRedirectPort` / `Session.SaveCredentials`. Renamed from the earlier P0-4 draft: `appearance.followSystem`, `appearance.sidebarCollapsed`, `ui.lastRoute`, `chart.defaultTimeframe`, `kite.loopbackPort`, `diagnostics.logLevel`, `data.backup.scheduled/folder/keep`, `calendar.staleToastDay`.
- **A8 Shortcut registry** (P0-3 `internal/settings/shortcuts.go`, §8.8; `settings.ShortcutAction{id, label, scope, defaultBinding, binding}`): `nav.1`…`nav.8` (`Alt+1`…`Alt+8`, global, labels `Go to Home`, `Go to Scalper`, `Go to Strategies`, `Go to Equity Builder`, `Go to Live Strategies`, `Go to Scanners`, `Go to New Scanner`, `Go to Scanner Alerts`), `toasts.focus` (`Alt+T`, global), `search.open` (`Ctrl+K`, global), `help.open` (`F1`, global), `kill.switch` (`Ctrl+Shift+K`, global), `chart.fullView` (`F11`, global), `chart.trendLine` (`Alt+T`, chart), `order.new` (`Shift+T`, chart), `builder.undo` (`Ctrl+Z`, builder), `builder.redo` (`Ctrl+Shift+Z`, builder), `watchlist.moveUp` / `watchlist.moveDown` (`Alt+ArrowUp` / `Alt+ArrowDown`, view). Binding grammar: `+`-joined `Ctrl`, `Alt`, `Shift`, `Meta` and a key. Conflict = same binding in the same scope or in `global`; `Settings.Rebind` refuses with `{"code":"conflict","reason":"<label of the conflicting action>"}` and a malformed binding with code `invalid`.

### Produces (for P0-5 and later phases)

| Produced by | Name | Contract |
|---|---|---|
| Task 1 | `AppShell` | `({ topBar, sidebar, banner?, children }: AppShellProps)` — persistent frame |
| Task 1 | `TopBar` | `({ ticker?, status?, links?, profile? }: TopBarProps)` — P0-5 passes `<IndexTickerStrip/>` as `ticker` in `ShellLayout.tsx` |
| Task 1 | `ShellLayout` | route element; P0-5 edits only its `ticker={null}` prop |
| Task 1 | `HomeLayout` | `({ topRow?, main, below?, right }: HomeLayoutProps)` — two independently scrolling columns |
| Task 1 | `ThemeProvider`, `useTheme()`, `applyTheme(name)` | `useTheme(): { theme: ThemeName; tokens: ColourTokens }`; chart adapter calls `setTheme(useTheme().tokens)` |
| Task 1 | `colourTokens`, `ColourTokens`, `ThemeName` | `src/design/tokens.ts` |
| Task 1 | formatters | `formatNumber`, `formatPrice(v, tickDecimals)`, `formatMoney`, `formatSignedMoney`, `formatSigned`, `formatPercent`, `formatChange(abs, pct)`, `formatCompact`, `formatDate`, `formatDateLong`, `formatDateTimeLong`, `formatDateTimeCompact`, `formatTime(ms, withSeconds?)`, `formatExpiry`, `toneOf`, `DASH` in `src/i18n/format.ts` |
| Task 1 | `isEnabled(feature)`, `visibleNavEntries()`, `BUILD_PHASE` | `src/app/features.ts` |
| Task 1 | `shellChildren` | `src/app/routes.tsx` — P0-5 appends `{ path: 'chart-full', element: <ChartFullView /> }` |
| Task 2 | `Backend`, `EventMap`, model types (canonical DTOs: `SessionStatus`, `TickDTO`, `FeedState`, `MarketStatus`, `ClockOffset`, `ShortcutAction`, …) | `src/api/backend.ts` — P0-5 adds `chart`, `instruments`, `quotes`, `watchlists` domains here and in `api/wails.ts` + `api/mock/index.ts` |
| Task 2 | `Backend.on(name, cb): () => void` | the only event subscription API (Wails `EventsOn` / mock listeners); P0-5 receives the backend via `useBackend()` |
| Task 2 | `parseBackendError(e): BackendError{code, stage, reason}` | `src/api/errors.ts` |
| Task 2 | `useBackend()`, `createMockBackend(seed)`, `renderApp()` | backend context, mock, test helper |
| Task 2 | `seedByName(name)` incl. `personalSeed` (`?seed=personal`: ready session, `dataPlan: 'none'`), `window.__tradeloomMock.emit(name, payload)` | mock seeds for Vitest and Playwright |
| Task 1 | npm scripts `dev:mock` (`vite --mode mock`), `lint` (`eslint src e2e`), `e2e` | `frontend/package.json` |
| Task 2 | `useTicksStore`, `useLtp(token)` | latest `TickDTO` per token (`ltp`, `close`, `change`, `changePct`, `tsMs`, `polled`); unchanged tokens keep object identity |
| Task 2 | `useStatusStore`, `useSessionStore`, `useSetting(key, fallback)`, `writeSetting` | stores (`useSessionStore.session: SessionStatus \| null`) |
| Task 3 | `Button`, `IconButton`, `Tooltip`, `Dialog`, `ConfirmDialog`, `MenuButton`, `Toggle`, `TextField`, `SelectField`, `Skeleton`, `Icon` | `src/components/` |
| Task 4 | `pushToast({ level, message, action? })`, `focusNewestToast()` | `src/store/toasts.ts` |
| Task 6 | `useShortcut(actionId, handler)`, attribute `data-shortcut-scope="chart"` | `src/shortcuts/` |
| Task 5 | `ListPage`, `ListBody`, `DataTable<T>`, `filterRows`, `NotFoundView` | `src/components/`, `src/views/` |
| Task 12 | `HomeChartArea`, `HomeWatchlistArea` | `src/views/home/slots.tsx` — P0-5 replaces these two component bodies |
| Task 18 | `InfoButton({ articleId, label })`, `ARTICLES`, `HelpArticle` | `src/help/` — later phases add articles and place (i) buttons |
| Task 19 | `P0_VIEWS`, `openApp`, `go`, `tabTo` | `e2e/helpers.ts` — P0-5 appends its views so axe, overflow and no-B/S guards cover them |
| Task 19 | CI job `ui-e2e` | `.github/workflows/ci.yml` — Playwright chromium + WebKit against `VITE_BACKEND=mock` |

P0-5 tasks that need Dialog/Tooltip/toasts/shortcuts start after P0-4 Tasks 3–6; the index's "P0-4 Task 1 lands first" rule covers layout and theme only.

---

## File Structure

```
.github/workflows/ci.yml                 (modify, Task 19: add the ui-e2e job)
frontend/
├── package.json                         (modify: deps + scripts lint, e2e, dev:mock)
├── eslint.config.js                     (create: flat config for `npm run lint`)
├── vite.config.ts                       (overwrite: react plugin + vitest config)
├── playwright.config.ts                 (create: chromium + WebKit, 1280×720, mock dev server)
├── .env.mock                            (create: VITE_BACKEND=mock)
├── e2e/                                 Playwright specs (Tasks 1, 8, 9, 11, 12, 16, 19)
└── src/
    ├── main.tsx                         (overwrite) entry: applyTheme + <App backend>
    ├── vite-env.d.ts                    (overwrite) ImportMetaEnv typing
    ├── test/                            setup.ts, matchMedia.ts, render.tsx (Vitest helpers)
    ├── design/                          tokens.ts (colours), contrast.ts, tokens.css (spacing/type), base.css
    ├── i18n/                            format.ts (FR-SHELL-016), copy.ts (all UI strings)
    ├── api/                             backend.ts (types + interface), wails.ts, mock/{index,fixtures}.ts,
    │                                    BackendContext.tsx, errors.ts, events.ts, index.ts
    ├── store/                           reset.ts, session.ts, status.ts, ticks.ts, settings.ts, app.ts,
    │                                    toasts.ts, portfolio.ts, jobs.ts, nav.ts
    ├── shortcuts/                       registry.ts, store.ts, ShortcutProvider.tsx, GlobalShortcuts.tsx
    ├── components/                      Icon, Button, IconButton, Tooltip, Dialog, ConfirmDialog, MenuButton,
    │                                    Toggle, TextField, SelectField, Skeleton, ToastRegion, ListPage, ListBody,
    │                                    DataTable, filterRows.ts, *.css
    ├── app/                             App, RootLayout, ShellLayout, AppShell, TopBar, Sidebar, HomeLayout,
    │                                    routes.tsx, router.ts, features.ts, theme.tsx, useNow.ts, lastRoute.ts,
    │                                    StatusIndicator, describeStatus.ts, QuickLinks, ProfileMenu, useProfile.ts,
    │                                    SessionGate, gate.ts, ShellBanners, shell.css
    ├── portfolio/calc.ts                holdings/positions/orders arithmetic
    ├── views/
    │   ├── onboarding/                  SetupView, LoginView, RiskDisclosureView, OnboardingFrame, onboarding.css
    │   ├── holdings/HoldingsView.tsx    positions/PositionsView.tsx   orders/OrderBookView.tsx
    │   ├── home/                        HomeView, HoldingsSummaryCard, slots.tsx, home.css
    │   ├── profile/ProfileView.tsx
    │   ├── settings/                    SettingsView, groups.ts, SettingRow, KiteGroup, AppearanceGroup, ChartGroup,
    │   │                                AboutGroup, ShortcutsGroup, DiagnosticsGroup, DataGroup, CalendarGroup, settings.css
    │   ├── legal/LegalView.tsx
    │   └── NotFoundView.tsx
    ├── help/                            articles.ts, renderArticle.tsx, search.ts, HelpView.tsx, InfoButton.tsx, help.css
    └── legal/content.ts                 terms, privacy, risk disclosure (own text)
```

---

<!-- generated task table: begin (plan_split.py) -->
## Task files

Each implementer reads this manifest plus its one task file. Sizes are approximate (characters / 4).
"Consumes tasks" lists earlier tasks named in the task's Consumes lines. Dispatch in number order
unless Global Constraints or the P0 index state a different execution order.

| # | Task | File | ~tokens | Consumes tasks |
|---|---|---|---|---|
| 1 | Foundation — tooling, theme tokens, formatters, app layout shell (reused by P0-5) | [`tasks/01-foundation-tooling-theme-tokens-formatters-app.md`](tasks/01-foundation-tooling-theme-tokens-formatters-app.md) | 12.6k | — |
| 2 | Backend adapter, mock backend, stores and event bridge | [`tasks/02-backend-adapter-mock-backend-stores-and.md`](tasks/02-backend-adapter-mock-backend-stores-and.md) | 19.6k | 1 |
| 3 | Design-system primitives — buttons, tooltips, dialogs, menus, fields | [`tasks/03-design-system-primitives-buttons-tooltips-dialogs.md`](tasks/03-design-system-primitives-buttons-tooltips-dialogs.md) | 9.4k | 1 |
| 4 | Toast notifications | [`tasks/04-toast-notifications.md`](tasks/04-toast-notifications.md) | 3.0k | 1, 3 |
| 5 | List-page template, data table, empty/loading/error states, unknown-view fallback | [`tasks/05-list-page-template-data-table-empty.md`](tasks/05-list-page-template-data-table-empty.md) | 6.9k | 2, 3 |
| 6 | Shortcut registry client, global shortcuts and window state (last view) | [`tasks/06-shortcut-registry-client-global-shortcuts-and.md`](tasks/06-shortcut-registry-client-global-shortcuts-and.md) | 5.3k | 1, 2, 4 |
| 7 | Connection, session and market status indicator | [`tasks/07-connection-session-and-market-status-indicator.md`](tasks/07-connection-session-and-market-status-indicator.md) | 5.0k | 1, 2, 3, 4 |
| 8 | Top-bar quick links and profile menu (with theme toggle) | [`tasks/08-top-bar-quick-links-and-profile.md`](tasks/08-top-bar-quick-links-and-profile.md) | 3.8k | 1, 2, 3, 4 |
| 9 | Onboarding — session gate, setup (incl. secret-storage fallback), login, risk disclosure, expiry banner, last-view restore | [`tasks/09-onboarding-session-gate-setup-login-risk.md`](tasks/09-onboarding-session-gate-setup-login-risk.md) | 10.0k | 2, 3, 4, 6 |
| 10 | Portfolio state, arithmetic and the Holdings view | [`tasks/10-portfolio-state-arithmetic-and-the-holdings.md`](tasks/10-portfolio-state-arithmetic-and-the-holdings.md) | 6.4k | 1, 2, 4, 5 |
| 11 | Positions and Order Book views (read-only) + 1280 px long-name layout check | [`tasks/11-positions-and-order-book-views-1280.md`](tasks/11-positions-and-order-book-views-1280.md) | 5.2k | 2, 5, 10 |
| 12 | Home layout with the Holdings summary card (P0-5 fills the chart and watchlist slots) | [`tasks/12-home-layout-with-the-holdings-summary.md`](tasks/12-home-layout-with-the-holdings-summary.md) | 2.8k | 1, 2, 3, 10 |
| 13 | Profile view and log out | [`tasks/13-profile-view-and-log-out.md`](tasks/13-profile-view-and-log-out.md) | 2.5k | 1, 2, 3, 4, 5, 7, 8, 11 |
| 14 | Settings view — shell, Kite connection, Appearance, Chart, About | [`tasks/14-settings-view-shell-kite-connection-appearance.md`](tasks/14-settings-view-shell-kite-connection-appearance.md) | 7.6k | 1, 2, 3, 5 |
| 15 | Settings → Shortcuts and Diagnostics; crash notice | [`tasks/15-settings-shortcuts-and-diagnostics-crash-notice.md`](tasks/15-settings-shortcuts-and-diagnostics-crash-notice.md) | 4.1k | 2, 6, 14 |
| 16 | Settings → Data — storage facts, clear candles, backup, scheduled backup, restore | [`tasks/16-settings-data-storage-facts-clear-candles.md`](tasks/16-settings-data-storage-facts-clear-candles.md) | 6.1k | 2, 3, 14 |
| 17 | Settings → Market calendar (import, add/edit/remove, stale warning) | [`tasks/17-settings-market-calendar.md`](tasks/17-settings-market-calendar.md) | 5.0k | 1, 2, 5 |
| 18 | Built-in help centre, contextual help panel and legal pages | [`tasks/18-built-in-help-centre-contextual-help.md`](tasks/18-built-in-help-centre-contextual-help.md) | 7.6k | 2, 3, 5, 6, 9, 15 |
| 19 | End-to-end sweep — accessibility, dialogs, keyboard-only path, theme, P0 guards | [`tasks/19-end-to-end-sweep-accessibility-dialogs.md`](tasks/19-end-to-end-sweep-accessibility-dialogs.md) | 3.7k | 2 |

Coverage and self-review: [`APPENDIX.md`](APPENDIX.md).
<!-- generated task table: end -->