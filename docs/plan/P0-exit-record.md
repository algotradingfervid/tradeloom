# P0 exit record

Release commit: PENDING (user), not cut yet · Packages: PENDING (user): `tradeloom_<v>_amd64_noorders.deb`, `Tradeloom-<v>-x86_64_noorders.AppImage` (SHA256SUMS attached) · Date: 2026-10-13 (headless part) · Operator: Claude (P0-5 Task 19, headless part); the user does the rest
CI run: PENDING (user). Needs: go-unit, ta, arch-lint, frontend, licence, linux-package, install-smoke, ui-e2e, all success. `.github/workflows/ci.yml` on this branch has seven of the eight jobs. `ui-e2e` comes with P0-4 Task 19.

**Status: NOT SIGNED OFF.** Only the headless frontend part ran (macOS arm64, mock backend, Playwright Chromium and
WebKit). Every row marked **PENDING (user)** needs the prerequisites below. A step without recorded evidence counts as
not done (Task 19).

## Prerequisites for the pending steps

| Needed | For | Notes |
|---|---|---|
| Kite Connect app with a **Connect-plan** API key + secret | Step 4, Step 6, Step 7, Step 8 | live login, historical data, ticks |
| Kite app with a **Personal-plan** key | Step 5 | second demo ("No data plan") |
| **VM A**: clean Ubuntu 24.04 desktop (GNOME keyring, `nmcli`, `xdotool`, `secret-tool`, `tcpdump`/`tshark`, `sqlite3`) | Steps 4–8 | also the `wails dev` S0-5 run, or any Linux desktop with WebKit2GTK 4.1 |
| **VM B**: clean Ubuntu 24.04 | Step 4 #14 | restore check, keyring stays empty |
| **Docker** on an amd64 build host | Step 2 (`make docker-release`), Step 3 (distro matrix) | no Docker on this Mac |
| A **weekday market session** (09:15–15:30 IST, NSE trading day) | Step 4 #5, #7, #8, #10 | live candles and ticks |
| A **24-hour capture window** starting before 09:00 IST on a trading day | Step 7 | VM A logged in for the whole window |
| `gh` authenticated on the repo, branch pushed | Step 1 CI line | green run on the release commit |

## Step 1: Full automated suite on the release commit

| Command | Result | Evidence |
|---|---|---|
| `GOWORK=off go vet -tags webkit2_41,noorders ./...` | PENDING (user): Linux, release commit | |
| `GOWORK=off go test -race -count=1 -tags webkit2_41,noorders ./...` | PENDING (user): Linux, release commit | |
| `GOWORK=off go test -count=1 -tags webkit2_41,orders,noorders ./internal/kite/orderwrite/...` | PENDING (user): Linux, release commit | |
| `(cd ta && go vet ./... && go test -count=1 ./...)` | PENDING (user): release commit | |
| `npm ci` | PENDING (user): not run here (no installs allowed in this session; `node_modules` already present) | |
| `npm run typecheck` (`tsc --noEmit`) | **exit 0** (pre-release, branch `p0/p05-t19`) | 2026-10-13 run |
| `npm run lint` | **exit 0** (pre-release) | `eslint src e2e spike` |
| `npm test` (`vitest run`) | **exit 0**: 98 files, 1096 tests passed (pre-release) | 2026-10-13 run |
| `npm run build` | PENDING (user): release commit (not run here; it rewrites the embedded `frontend/dist`) | |
| `npx playwright test` (with `S05=1`, `E2E_PORT=5292`) | **exit 1**: 68 passed, 2 failed (pre-release). The 2 failures are `home-layout.spec.ts` in both engines, a known P0-4 Task 19 item (the test's 3000 px filler shrinks in the flex column, so `scrollTop` stays 0; fix: `flexShrink: 0` on the filler). Everything else passes, including the S0-5 spike spec and NFR-07. | see "Headless runs" below |
| `gh run list --workflow ci.yml …` | PENDING (user): needs the release commit and a green 8-job run | |

## Step 2: Build the packages
PENDING (user): Docker build host. `make docker-release VERSION=<v>` must print `g0: ok — no order-write methods in build/bin/tradeloom-g0`.

## Step 3: Install checks on the distro matrix (NFR-01, NFR-02)
PENDING (user): Docker. Needs five lines: `ubuntu:22.04 deb exit=0`, `ubuntu:24.04 deb exit=0`, `debian:12 deb exit=0`, `fedora:39 appimage exit=0` and `ubuntu:24.04 appimage exit=0`.

## ROADMAP §P0 demo (Connect plan): Step 4
All rows: **PENDING (user)**. Needs VM A, the Connect-plan key and a weekday market session (VM B for #14).

| # | Expected | Observed | Evidence |
|---|---|---|---|
| 1 | Installs without errors; window opens in < 1 s and shows first-run setup (FR-ACCT-005) | PENDING (user) | screenshots/p0-exit/01.png |
| 2 | Settings → Kite connection shows backend "Keyring"; `secret-tool lookup service tradeloom username kite.api_secret` prints the secret (it is in the keyring, not on disk) | PENDING (user) | screenshots/p0-exit/02.png |
| 3 | System browser opens Kite; after login + 2FA the tab shows "Logged in — you can close this tab"; the app window comes to front (FR-ACCT-006, loopback) | PENDING (user) | screenshots/p0-exit/03.png |
| 4 | Home opens (FR-SHELL-020) | PENDING (user) | screenshots/p0-exit/04.png |
| 5 | Chart header `NIFTY 50 INDICES <LTP> (<pct>%)`, timeframe `1 Hour`; within 60 s the last candle's close moves with ticks; three pills NIFTY 50 / NIFTY BANK / SENSEX with LTP and change; status dot green "Live" | PENDING (user). Mock-backend equivalent passes: `chart-watchlists.spec.ts` "Home: NIFTY 50 at 1 Hour, three pills…", "FR-SHELL-004 AC1", "FR-CHART-003 AC2 (UI)" | screenshots/p0-exit/05.png |
| 6 | Header `RELIANCE NSE …` within 1 s for cached, ≤ 5 s on first fetch; volume histogram visible | PENDING (user). Mock-backend equivalent passes: "FR-CHART-005 AC1/AC4/AC5: search RELIANCE (≤ 1 s)" | screenshots/p0-exit/06.png |
| 7 | Header shows `NIFTY OCT FUT` with F badge and `NFO-FUT`; candles load | PENDING (user) | screenshots/p0-exit/07.png |
| 8 | Both rows appended to "Watchlist 1"; LTP/% change on both within a few seconds, tick by tick | PENDING (user). Mock-backend equivalent passes: "P0 demo path: NIFTY OCT FUT + RELIANCE into a watchlist…" | screenshots/p0-exit/08.png |
| 9 | Each lists the account's data (or its empty state if the account has none) | PENDING (user) | screenshots/p0-exit/09.png |
| 10 | Dot turns amber "Reconnecting" (tooltip shows attempt and next retry); after reconnect it turns green "Live" with no user action; pills, both watchlist rows and the chart's last candle tick again (NFR-09) | PENDING (user) | screenshots/p0-exit/10.png |
| 11 | Every surface including the chart canvas, grid and scales turns dark at once; restart → still dark (FR-SHELL-011) | PENDING (user). The headless S0-5 G4 repaint is 28.1 ms (Chromium) and 24 ms (WebKit) | screenshots/p0-exit/11.png |
| 12 | Candle store size shows a non-zero value (MB) | PENDING (user) | screenshots/p0-exit/12.png |
| 13 | `~/tl-backups/tradeloom-backup-YYYYMMDD-HHMMSS.tar.zst` exists | PENDING (user) | screenshots/p0-exit/13.png |
| 14 | After the relaunch, "Watchlist 1" holds `NIFTY OCT FUT` and `RELIANCE`; dark theme is set; Settings → Kite connection asks for credentials; `secret-tool search --all service tradeloom` on VM B prints nothing (keyring empty) | PENDING (user): VM B | screenshots/p0-exit/14.png |
| 15 | No match | PENDING (user): see Step 6 | |
| 16 | No horizontal scrollbar. The 3 pills, status indicator, Order Book, Positions, bell and avatar are fully visible. Tab moves through symbol → refresh → timeframe with a visible focus ring. Repeat in light theme (NFR-11) | PENDING (user): real window via `xdotool`. Mock-backend equivalent passes in Chromium and WebKit: `layout-1280.spec.ts` (light and dark), `shell.spec.ts` (pill right edges inside the ticker container) | screenshots/p0-exit/16.png |

## Second demo (Personal plan): Step 5
All rows: **PENDING (user)**. Needs VM A and the Personal-plan key.

| # | Expected | Observed | Evidence |
|---|---|---|---|
| 1 | Status indicator grey "No data plan" | PENDING (user) | screenshots/p0-exit/s2-01.png |
| 2 | Chart area shows "Charts need a Kite Connect data plan"; pills show "—"; `grep -c 'instruments/historical' …/tradeloom.log` unchanged across opening Home | PENDING (user). Mock-backend equivalent passes: "FR-CHART-003 AC4 (no data plan, ?seed=personal)" (0 `chart.candles` calls) | screenshots/p0-exit/s2-02.png |
| 3 | Holdings load (FR-ACCT-007 AC1) | PENDING (user) | screenshots/p0-exit/s2-03.png |

## NFR runs

| NFR | Check | Result | Evidence |
|---|---|---|---|
| NFR-01/02 | Step 3 matrix: 5 × exit=0 | PENDING (user): Docker | console log |
| NFR-03 | P0-1 migration tests in CI + VM B restore re-ran migrations | PENDING (user): CI run + VM B | |
| NFR-04 | Step 6 grep: no match; Step 7 capture: api.kite.trade, ws.kite.trade only | PENDING (user): VM A, Connect key, 24 h capture | tl-sni.txt, tl-ips.txt |
| NFR-07 | e2e "tick storm" worst latency < 100 ms | **PASS (headless, mock backend)**: worst 15.5 ms in Chromium and 21.0 ms in WebKit (full run). Over `--repeat-each 5`, worst per run 8.2–16.7 ms (Chromium) and 9.0–16.0 ms (WebKit). All 10 timeframe changes per run reached the backend. | `frontend/e2e/chart-watchlists.spec.ts` "NFR-07: timeframe change paints < 100 ms during a 10 Hz × 250-token tick storm"; console lines `NFR-07 <engine>: worst … ms` |
| NFR-09 | Step 4 #10 reconnect + resubscribe | PENDING (user): VM A, market hours | |
| NFR-11 | Step 4 #16 + e2e layout-1280 (light, dark) | e2e part **PASS** in Chromium and WebKit (`layout-1280.spec.ts` light and dark, `shell.spec.ts`). Step 4 #16 PENDING (user) | Playwright run 2026-10-13 |
| NFR-15 | Step 8 counts | PENDING (user): VM A | |

### Headless runs (2026-10-13, this record's evidence)

Machine: macOS 26.5.2 arm64, Node 24.13.0, Playwright 1.64.0 (Chromium 156.0.8078.4, WebKit 27.2), branch `p0/p05-t19`
(base 3ec6a2c), mock backend (`vite --mode mock` started by Playwright's `webServer` on 127.0.0.1:5292).

| Command (in `frontend/`) | Exit | Result |
|---|---|---|
| `npx --no-install vitest run` | 0 | 98 files, 1096 tests passed |
| `npx --no-install tsc --noEmit` | 0 | |
| `npm run lint` | 0 | |
| `E2E_PORT=5292 S05=1 npx --no-install playwright test --reporter=line` | 1 | 70 tests: 68 passed, 2 failed (`home-layout.spec.ts` ×2, P0-4 Task 19's known filler issue; not in this task's scope) |
| `E2E_PORT=5292 S05=1 npx --no-install playwright test e2e/spike-s05.spec.ts e2e/chart-watchlists.spec.ts --repeat-each 5` | 0 | 230 passed |
| `lsof -nP -iTCP:5292 -sTCP:LISTEN` before and after every run | 1 (nothing listening) | port free before and after |

## Acceptance criteria (ROADMAP §P0 list)
The automated half on this branch: P0-5's Coverage table rows (CHART, WATCH, SHELL-004) are covered by the vitest suite
(1096 passed) and by the e2e specs `chart-watchlists.spec.ts`, `layout-1280.spec.ts` and `naming-consistency.spec.ts`
(all pass in Chromium and WebKit). The P0-1…P0-4 coverage tables and the green CI run on the release commit are
**PENDING (user)**. `home-layout.spec.ts` (FR-SHELL-001 AC4 / FR-HOME-001 AC3) fails both engines until P0-4 Task 19's
fix is merged. That is a red cell, and it blocks the exit.

## Spikes

| Spike | Document | Decision |
|---|---|---|
| S0-1 (P0-3) | not on this branch | PENDING: document missing here |
| S0-2 (P0-2) | [S0-2-candle-store.md](../architecture/spikes/S0-2-candle-store.md) | provisional; final needs the Linux reference-hardware run |
| S0-3 (P0-1) | not on this branch | PENDING: document missing here |
| S0-4 (P0-1) | [S0-4-keyring.md](../architecture/spikes/S0-4-keyring.md) | PENDING: Linux desktop matrix |
| S0-5 (P0-5) | [S0-5-chart.md](../architecture/spikes/S0-5-chart.md) | **Chromium: final GO. WebKit: final GO.** `wails dev` (WebKit2GTK 4.1): PENDING (user), needs a Linux desktop. OQ-01 not escalated for these two engines; it is final when the `wails dev` column is filled |
| S0-6, S0-7 (P0-2) | not on this branch | PENDING: documents missing here |
| S0-8 (P0-3) | not on this branch | PENDING: document missing here |

S0-5 detail. Task 18 saw G3 fail in WebKit only. The cause was the harness's single-row pixel sample: in WebKit an
opaque grid line falls on the sampled row, over the `bottom`-zOrder band. It was not a rendering bug. The check now
compares every row: ≥ 90 % of the in-band rows must change, nothing outside the band may change, and a negative
control must fail. G5's one-frame timing is confirmed by pixels: the first frame after a prepend is repainted and
already settled, and a negative control with a wrong intermediate frame is caught. See S0-5-chart.md.

Sign-off: P0 exit criteria met: PENDING (user)
