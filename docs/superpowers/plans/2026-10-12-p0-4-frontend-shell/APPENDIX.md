## Acceptance-criteria coverage

"GATE Pn" = the AC exercises a feature hidden in a P0 build (ROADMAP §0); its test lands with that phase. "P0-5" = owned by the chart/watchlist plan. "Backend" = asserted in Go by P0-1/P0-3; this plan covers only the UI half named.

| FR / AC | Test (task, file → test name) |
|---|---|
| SHELL-001 AC1 | T1 `src/app/AppShell.test.tsx` → "AC1 (P0 form)…" (8-entry form: GATE P6, `src/app/features.test.ts`) |
| SHELL-001 AC2 | T1 `e2e/shell.spec.ts`; T8 `src/app/TopBar.test.tsx` "FR-SHELL-001 AC2"; T19 `e2e/guards.spec.ts` "every P0 view fits 1280×720" (ticker pills: P0-5; bell: GATE P3) |
| SHELL-001 AC3 | T8 `src/app/TopBar.test.tsx` → "AC1 + FR-SHELL-001 AC3" |
| SHELL-001 AC4 | T12 `e2e/home-layout.spec.ts` |
| SHELL-002 AC1 | T1 `src/app/features.test.ts` (P0 entries now; full eight: GATE P6) |
| SHELL-002 AC2 | GATE P1 (Equity Builder) |
| SHELL-002 AC3 | T1 `src/app/Sidebar.test.tsx` "AC3"; T19 `e2e/keyboard.spec.ts` "sidebar arrows and Enter" |
| SHELL-002 AC4 | T2 `src/app/persistence.test.tsx` "FR-SHELL-002 AC4" |
| SHELL-005 AC1–AC5 | T7 `src/app/describeStatus.test.ts` + `src/app/StatusIndicator.test.tsx` (AC1–AC5 named); 1 s timing of AC2 is the backend's emit (P0-3) |
| SHELL-006 AC1 | T8 `src/app/TopBar.test.tsx` "AC1 + FR-SHELL-001 AC3" |
| SHELL-006 AC2, AC3 | GATE P3 (bell / Notifications) |
| SHELL-009 AC1–AC3 | T4 `src/components/ToastRegion.test.tsx` |
| SHELL-009 AC4 | GATE P2 (scanner); toast mechanics in T4 |
| SHELL-009 AC5 | T6 `src/shortcuts/ShortcutProvider.test.tsx` "FR-SHELL-009 AC5 + FR-SHELL-019 AC4" |
| SHELL-010 AC1–AC4 | T8 `src/app/TopBar.test.tsx` "FR-SHELL-010" block |
| SHELL-011 AC1, AC2 | T1 `src/app/theme.test.tsx`; T8 `TopBar.test.tsx` theme toggle; T19 `e2e/guards.spec.ts` "within 300 ms… survives a restart" (chart canvas: P0-5) |
| SHELL-011 AC3 | T1 `src/design/tokens.test.ts`; T19 `e2e/a11y.spec.ts` (both themes) |
| SHELL-012 AC1, AC2 | T5 `src/views/NotFoundView.test.tsx`; T18 help/legal unknown pages |
| SHELL-013 AC1 | T3 `src/components/Tooltip.test.tsx` "AC1"; T19 `e2e/a11y.spec.ts` |
| SHELL-013 AC2 | T3 `Tooltip.test.tsx` "AC2" (primitive); refresh button itself: P0-5 |
| SHELL-013 AC3 | T18 `src/help/help.test.tsx` "(i) opens the matching article"; placing it on the header: GATE P1 |
| SHELL-014 AC1–AC4 | T5 `src/components/ListPage.test.tsx`; T10 `HoldingsView.test.tsx` (AC1, AC3) |
| SHELL-015 AC1 | T5 `src/components/DataTable.test.tsx` "review focus"; T19 `e2e/keyboard.spec.ts` row actions (watchlist row: P0-5; B/S: GATE P1) |
| SHELL-015 AC2, AC3 | T3 `src/components/Dialog.test.tsx`; T19 `e2e/dialogs.spec.ts` (every P0 dialog) |
| SHELL-016 AC1, AC2 | T1 `src/i18n/format.test.ts` |
| SHELL-016 AC3 | GATE P2 (scanner timestamps; both use `formatDateTimeLong` from T1) |
| SHELL-019 AC1 | T6 `ShortcutProvider.test.tsx` "AC1 (client part)"; T15 `ShortcutsGroup.test.tsx`; T18 `help.test.tsx` (help part); kill-switch dialog: GATE P1 |
| SHELL-019 AC2 | T2 `src/api/mock/mock.test.ts` (contract); T15 `ShortcutsGroup.test.tsx` "AC2" |
| SHELL-019 AC3 | T9 `onboarding.test.tsx` "AC3 (frontend part)"; maximised state: Backend (A6) |
| SHELL-019 AC4 | T6 `ShortcutProvider.test.tsx` (trend-line tool body: P0-5) |
| SHELL-020 AC1, AC3 | T9 `src/app/gate.test.ts` + `onboarding.test.tsx`; T9 `e2e/onboarding.spec.ts` |
| SHELL-020 AC2 | GATE P1 (deploy dialog) |
| ACCT-001 AC1, AC3 (UI) | T13 `src/views/profile/ProfileView.test.tsx` "AC1", "AC3" (`Profile` fields of P0-3 Task 11; `loggedInAtMs` / `expiresAtMs`) |
| ACCT-004 AC1 (UI) | T13 `ProfileView.test.tsx` "FR-ACCT-004 AC1 (UI part)" and the `LogoutResult.remoteInvalidated: false` case; revocation itself: Backend |
| ACCT-005 AC1, AC3 (UI) | T9 `onboarding.test.tsx` "FR-ACCT-005" block (`Session.SaveCredentials({apiKey, apiSecret})`); T14 `SettingsView.test.tsx` "AC3: Remove credentials", "Change saves new credentials" |
| ACCT-006 AC1–AC3 (UI) | T9 `onboarding.test.tsx` "FR-ACCT-006" block (`Login()` → `LoginStart`, `listenerOk: false` paste fallback, `CompleteLogin`, `CancelLogin`, expiry banner); T9 `e2e/onboarding.spec.ts`; T19 `guards.spec.ts` "06:00 expiry" |
| ACCT-007 AC1 (UI) | T7 `describeStatus.test.ts` "FR-ACCT-007 AC1" (`dataPlan: 'none'`); T14 `SettingsView.test.tsx` "Personal-plan key"; T19 `guards.spec.ts` "?seed=personal" |
| ACCT-013 AC1, AC3 (UI) | T9 `onboarding.test.tsx` "FR-ACCT-013" block (`SecretStorage().choices`, `UseEncryptedFile`, `RefuseSecretStorage`, `Unlock`); T14 `SettingsView.test.tsx` "FR-ACCT-013 AC3 (UI part)" (`MigrateToKeyring`) |
| DATA-004 AC4 (UI) | T7 `describeStatus.test.ts` "FR-DATA-004 AC4" (`clock.offset.millis`, `source`); T7 `StatusIndicator.test.tsx` "warns once" (`warn`) |
| SHELL-018 AC2 (UI) | T15 `DiagnosticsGroup.test.tsx` (`log.level` `info`/`debug`, support bundle, crash notice from `LastCrash().path`) |
| ACCT-010 AC1–AC3 | T18 `src/help/help.test.tsx` |
| ACCT-012 AC1 | T16 `src/views/settings/DataGroup.test.tsx` "AC1 (UI part)"; re-fetch on open: P0-5 |
| ACCT-012 AC2 | T14 `SettingsView.test.tsx` "AC2 pattern" (port); live square-off field itself: GATE P1 |
| ACCT-012 AC3 | T14 `SettingsView.test.tsx` "AC3" |
| ACCT-012 AC4 | GATE P6 (whole-registry check once all groups exist; P0 group order fixed by `SETTINGS_GROUPS` in T14) |
| ACCT-015 AC1 | T18 `LegalView.test.tsx` "back arrow returns there" (back behaviour); deploy-dialog link: GATE P1 |
| ACCT-015 AC2 | T18 `src/views/legal/LegalView.test.tsx` "AC2", "AC2 (navigation)" |
| ACCT-015 AC3 | T18 `src/legal/clean-room.test.ts` |
| HOME-001 AC1 | P0-5 (chart default); `chart.default_timeframe` = `60minute` from T14 |
| HOME-001 AC2 | T12 `src/views/home/HomeView.test.tsx` "AC2" |
| HOME-001 AC3 | T12 `e2e/home-layout.spec.ts`; T19 `guards.spec.ts` |
| HOME-002 AC1–AC4 | T10 `src/portfolio/calc.test.ts` (AC1 from the Go `HoldingsSummary`, AC2 live-LTP overlay); T12 `HomeView.test.tsx` (AC1–AC4) |
| ORD-001 AC1, AC2 (UI) | T11 `src/views/orders/OrderBookView.test.tsx` (`BookSnapshot` sections/counts from Go, `orders.changed` push, 30 s cache re-read while the feed is down) |
| ORD-002 AC1 | T10 `calc.test.ts`; T11 `PositionsView.test.tsx` |
| ORD-002 AC2 | GATE P1 (Convert; P0 views are read-only) |
| ORD-003 AC1, AC2 | T10 `src/views/holdings/HoldingsView.test.tsx`, `calc.test.ts` |
| DATA-005 AC2 (UI) | T16 `DataGroup.test.tsx` "AC2 (UI part)" |
| DATA-005 AC1, AC3, AC4, AC5 | Backend (P0-1); UI path T16 `e2e/settings-data.spec.ts`, scheduled-backup controls in `DataGroup.test.tsx` |
| DATA-001 AC4, AC6 (UI) | T17 `src/views/settings/CalendarGroup.test.tsx` |
| NFR-11 | T19 `e2e/a11y.spec.ts`, `e2e/guards.spec.ts`; T1 `tokens.test.ts` |
| Global: no B/S in P0 | T19 `e2e/guards.spec.ts` "no Buy/Sell order buttons" |
| Global: own copy only | T18 `src/legal/clean-room.test.ts` |
| Review Focus 1–5 | T11 `e2e/portfolio-layout.spec.ts`; T1 `format.test.ts`; T7 `StatusIndicator.test.tsx`; T5 `DataTable.test.tsx` + T19 `keyboard.spec.ts`; T16 `DataGroup.test.tsx` |

## Self-review

1. **Spec coverage.** Every FR the index assigns to P0-4 maps to a task above. Gaps found and fixed: FR-DATA-001 calendar UI (AC4/AC6) was not assigned in the index but Settings → Market calendar is a P0 group, so Task 17 was added. Help (FR-ACCT-010) and legal pages (FR-ACCT-015) were added as Task 18. The cross-view guards (axe, overflow, no B/S, dialog dismissal) were added as Task 19.
2. **Placeholder scan.** No unfinished markers or deferred steps. "P0-5 replaces" is a stated hand-off (`slots.tsx`), not a placeholder.
3. **Type and name consistency.** Fixed inline:
   - Review Focus items 3 and 4 pointed to the wrong tasks; they now point to Task 7 and Task 5.
   - The Produces table listed `ListPage`/`DataTable` under Task 9 and shortcuts under Task 5; these are now Task 5 and Task 6.
   - The bindings "Used in" column pointed to Task 8 for onboarding and Task 6 for Reconnect; these are now Task 9 and Task 7.
   - Checked against their definitions: `Dialog` `size` (extended in T18), `ShortcutAction.binding: string`, `formatBinding`, `reloadShortcuts`, `callsTo`, `ListPage` search label (`Search ${title}` → "Search help"), every dialog title and opener name in `e2e/dialogs.spec.ts`, and the `?persist` handling in `defaultBackend`.
4. **Contract alignment (canonical contract §C–§G, 2026-10-12).** Every binding, event, DTO field and settings key in this plan now uses the canonical names; the earlier assumed names (renamed from `Session.State`, `Session.LoginWithRedirect`, `Session.AckRisk`, `Session.Reconnect`, `Portfolio.Orders`, flat `Holding`/`Position`/`Order` rows, `phase`/`riskAcked`/`secretsBackend`, `offsetMs`, `nextRetryAt`/`lastTickAt`, camelCase settings keys) are gone. Portfolio money is paise from Go, converted with `rupees()` in the view layer; Go owns the arithmetic and the UI only overlays the newest tick. `market.status`, `job.progress`, `orders.changed`/`positions.changed`/`holdings.changed`, `settings.changed` and the `Session.SecretStorage`/`UseEncryptedFile`/`RefuseSecretStorage`/`Unlock`/`MigrateToKeyring`/`ReconnectFeed`, `Settings.Shortcuts`/`Rebind`/`ResetShortcuts`/`GetUiState`/`SetUiState`, `Data.*`, `Calendar.*`, `Diagnostics.*`, `System.*` bindings are P0-3 additions required by the contract.
5. **Open items for reviewers (not fixable in this plan):**
   - `kite.Order` (`order.update`) is typed here as the gokiteconnect snake_case subset the UI reads (`KiteOrder`); if P0-3 wraps it in a camelCase DTO, only `KiteOrder`, `kiteOrder()` and `applyOrderUpdate` change.
   - `PositionRow.multiplier` (added to P0-3 Task 12 by the consistency review) drives the live P&L overlay; Go still re-prices on every `positions.changed`.
   - The FR-ACCT-010 behaviour notes are written as our own behaviour. Comparisons with the researched product are deliberately left out (clean-room), so the "deviations" article states our choices without naming a source.
   - The compact form follows the FR (`1.32Cr`) rather than ARCHITECTURE's `1.23 L`.
   - `keyboard.spec.ts` is skipped on macOS WebKit (Tab skips links); it runs in the Linux CI `ui-e2e` job.
6. **Review Focus.** All five lines have tests in their owning tasks, as listed in the coverage table.
