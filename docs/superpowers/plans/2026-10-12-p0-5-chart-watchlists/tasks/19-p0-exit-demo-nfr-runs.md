> **P0-5 Chart, Watchlists & Ticker UI Implementation Plan — Task 19.** Read [`../MANIFEST.md`](../MANIFEST.md) first: its Global Constraints,
> Review Focus and Consumes/Produces apply to this task. Other tasks live beside this file in `tasks/`.

<!-- task body (verbatim from the plan) -->
### Task 19: P0 exit demo + NFR runs

This task closes Phase 0 (index "Phase exit"). It needs P0-1 to P0-4 merged, a Kite Connect app with a **Connect-plan** key, and a **Personal-plan** key for the second demo. It also needs two clean Ubuntu 24.04 VMs (A and B), Docker on the build host, and a weekday market session for the live-bar checks. Record every result in `docs/plan/P0-exit-record.md` (Step 9). A step without recorded evidence counts as not done.

**Files:**
- Create: `docs/plan/P0-exit-record.md`

**Interfaces:**
- Consumes: all P0 deliverables. P0-1 make targets `deb`, `appimage`, `docker-release` (container build incl. the G-0 `go tool nm` check and `SHA256SUMS`), `--smoke`, `scripts/install-smoke.sh`, CI workflow `ci.yml` (jobs `go-unit`, `ta`, `arch-lint`, `frontend`, `licence`, `linux-package`, `install-smoke`, plus `ui-e2e` from P0-4 Task 19).
- Produces: the signed-off exit record that the index's "Phase exit" refers to.

- [ ] **Step 1: Full automated suite on the release commit**

```bash
GOWORK=off go vet -tags webkit2_41,noorders ./...
GOWORK=off go test -race -count=1 -tags webkit2_41,noorders ./...
GOWORK=off go test -count=1 -tags webkit2_41,orders,noorders ./internal/kite/orderwrite/...
(cd ta && go vet ./... && go test -count=1 ./...)
(cd frontend && npm ci && npm run typecheck && npm run lint && npm test && npm run build && npx playwright test --reporter=line)
gh run list --workflow ci.yml --branch "$(git branch --show-current)" --limit 1 --json conclusion,headSha --jq '.[0]'
```

Expected: each command exits 0. The `gh` line prints `{"conclusion":"success","headSha":"<HEAD sha>"}`, and the sha must equal `git rev-parse HEAD`. That green run includes all eight jobs: `go-unit` (archtest G-0 AST rules, redaction, the `orders,noorders` stub check), `ta`, `arch-lint`, `frontend`, `licence` (no GPL/AGPL/LGPL in the binary graph), `linux-package` (`make docker-release`, whose `g0-check` greps `go tool nm` for `gokiteconnect/v4.(*Client).{PlaceOrder…DeleteGTT}` and must find nothing), `install-smoke` and `ui-e2e`. Confirm with `gh run view <id> --json jobs --jq '.jobs[] | {name,conclusion}'`: every line must show `"success"`.

- [ ] **Step 2: Build the packages**

```bash
make docker-release VERSION=<v>
ls -1 dist/
cat dist/SHA256SUMS
```

Expected: `make docker-release` (P0-1: container build of the binary, the G-0 `g0-check`, then the `deb` and `appimage` targets) prints `g0: ok — no order-write methods in build/bin/tradeloom-g0` and exits 0. `dist/` contains `tradeloom_<v>_amd64_noorders.deb`, `Tradeloom-<v>-x86_64_noorders.AppImage` and `SHA256SUMS`, which lists both packages.

- [ ] **Step 3: Install checks on the distro matrix (NFR-01, NFR-02)**

Run from the repo root (this headless smoke test checks dependencies, migrations and window creation):

```bash
DEB=$(ls dist/tradeloom_*_amd64_noorders.deb); APP=$(ls dist/Tradeloom-*-x86_64_noorders.AppImage)
for img in ubuntu:22.04 ubuntu:24.04 debian:12; do
  docker run --rm -v "$PWD/dist:/dist:ro" "$img" bash -c "
    export DEBIAN_FRONTEND=noninteractive TRADELOOM_DATA_DIR=/tmp/tl
    apt-get update -qq && apt-get install -y -qq /dist/$(basename "$DEB") xvfb xauth >/dev/null &&
    xvfb-run -a tradeloom --smoke; echo \"$img deb exit=\$?\""
done
docker run --rm -v "$PWD/dist:/dist:ro" fedora:39 bash -c "
  export TRADELOOM_DATA_DIR=/tmp/tl
  dnf install -y -q webkit2gtk4.1 gtk3 xorg-x11-server-Xvfb xorg-x11-xauth >/dev/null &&
  cp /dist/$(basename "$APP") /tmp/tl.AppImage && chmod +x /tmp/tl.AppImage &&
  xvfb-run -a /tmp/tl.AppImage --appimage-extract-and-run --smoke; echo \"fedora:39 appimage exit=\$?\""
docker run --rm -v "$PWD/dist:/dist:ro" ubuntu:24.04 bash -c "
  export DEBIAN_FRONTEND=noninteractive TRADELOOM_DATA_DIR=/tmp/tl
  apt-get update -qq && apt-get install -y -qq libgtk-3-0t64 libwebkit2gtk-4.1-0 xvfb xauth >/dev/null &&
  cp /dist/$(basename "$APP") /tmp/tl.AppImage && chmod +x /tmp/tl.AppImage &&
  xvfb-run -a /tmp/tl.AppImage --appimage-extract-and-run --smoke; echo \"ubuntu:24.04 appimage exit=\$?\""
```

Expected: five lines, `ubuntu:22.04 deb exit=0`, `ubuntu:24.04 deb exit=0`, `debian:12 deb exit=0`, `fedora:39 appimage exit=0`, `ubuntu:24.04 appimage exit=0`. `apt-get install /dist/…deb` must resolve `libgtk-3-0` and `libwebkit2gtk-4.1-0` from the distro archive with no manual steps. A non-zero exit or an unresolved dependency fails NFR-01/02. Fix it in P0-1's `build/nfpm.yaml` or AppImage recipe, then repeat.

- [ ] **Step 4: Exit demo, Connect-plan key (VM A, clean Ubuntu 24.04, during market hours)**

Do these in order. For each step, write the observed result and a screenshot path in the record.

| # | Action | Expected |
|---|---|---|
| 1 | `sudo apt install ./tradeloom_<v>_amd64_noorders.deb`; start "Tradeloom" from the app menu | Installs without errors; window opens in < 1 s and shows first-run setup (FR-ACCT-005) |
| 2 | Enter API key and API secret; Save | Settings → Kite connection shows backend "Keyring"; `secret-tool lookup service tradeloom username kite.api_secret` prints the secret (it is in the keyring, not on disk) |
| 3 | "Log in with Kite" | System browser opens Kite; after login + 2FA the tab shows "Logged in — you can close this tab"; the app window comes to front (FR-ACCT-006, loopback) |
| 4 | Risk disclosure → "I understand" | Home opens (FR-SHELL-020) |
| 5 | Look at Home | Chart header `NIFTY 50 INDICES <LTP> (<pct>%)`, timeframe `1 Hour`; within 60 s the last candle's close moves with ticks; three pills NIFTY 50 / NIFTY BANK / SENSEX with LTP and change; status dot green "Live" |
| 6 | Chart header → search `RELIANCE` → Enter | Header `RELIANCE NSE …` within 1 s for cached, ≤ 5 s on first fetch; volume histogram visible |
| 7 | Search `nifty <current month> fut` (e.g. `nifty oct fut`) → Enter | Header shows `NIFTY OCT FUT` with F badge and `NFO-FUT`; candles load |
| 8 | Watchlist search: add `NIFTY OCT FUT` and `RELIANCE` | Both rows appended to "Watchlist 1"; LTP/% change on both within a few seconds, tick by tick |
| 9 | Open Holdings, Positions, Order Book | Each lists the account's data (or its empty state if the account has none) |
| 10 | `nmcli networking off` on VM A; wait 20 s; `nmcli networking on` | Dot turns amber "Reconnecting" (tooltip shows attempt and next retry); after reconnect it turns green "Live" with no user action; pills, both watchlist rows and the chart's last candle tick again (NFR-09) |
| 11 | Profile menu → "Dark theme" | Every surface including the chart canvas, grid and scales turns dark at once; restart → still dark (FR-SHELL-011) |
| 12 | Settings → Data | Candle store size shows a non-zero value (MB) |
| 13 | Settings → Data → "Backup now" → choose `~/tl-backups` | `~/tl-backups/tradeloom-backup-YYYYMMDD-HHMMSS.tar.zst` exists |
| 14 | Copy the backup to VM B (clean Ubuntu 24.04); install the same `.deb`; start; Settings → Data → Restore → choose the file → type the confirmation | After the relaunch, "Watchlist 1" holds `NIFTY OCT FUT` and `RELIANCE`; dark theme is set; Settings → Kite connection asks for credentials; `secret-tool search --all service tradeloom` on VM B prints nothing (keyring empty) |
| 15 | Run Step 6 (redaction grep) on VM A | No match |
| 16 | Window at 1280×720: `xdotool search --name '^Tradeloom$' windowsize 1280 720` | No horizontal scrollbar. The 3 pills, status indicator, Order Book, Positions, bell and avatar are fully visible. Tab moves through symbol → refresh → timeframe with a visible focus ring. Repeat in light theme (NFR-11) |

- [ ] **Step 5: Second demo, Personal-plan key (VM A, after Settings → Kite connection → replace the credentials and log in again)**

| # | Action | Expected |
|---|---|---|
| 1 | Log in with the Personal-plan app | Status indicator grey "No data plan" |
| 2 | Home | Chart area shows "Charts need a Kite Connect data plan"; pills show "—". Run `grep -c 'instruments/historical' ~/.local/share/tradeloom/logs/tradeloom.log` just before and just after opening Home: the count does not change. This assumes P0-1/P0-3 log each Kite request's route at the default level; if they do not, use Settings → Limits → historical request counter instead. |
| 3 | Open Holdings | Holdings load (FR-ACCT-007 AC1) |

- [ ] **Step 6: Redaction grep (NFR-04, FR-SHELL-018, FR-DATA-005 AC3)**

On VM A, while logged in with the Connect-plan key:

```bash
read -rs -p 'API secret: ' API_SECRET; echo
ACCESS_TOKEN=$(secret-tool lookup service tradeloom username kite.access_token)
[ -n "$API_SECRET" ] && [ -n "$ACCESS_TOKEN" ] && echo "values loaded (${#API_SECRET}/${#ACCESS_TOKEN} chars)"
D=${TRADELOOM_DATA_DIR:-$HOME/.local/share/tradeloom}
grep -r -a -l -F -e "$API_SECRET" -e "$ACCESS_TOKEN" "$D"; echo "datadir grep exit=$?"
for f in ~/tl-backups/tradeloom-backup-*.tar.zst; do echo "$f: $(zstd -dc "$f" | grep -a -c -F -e "$API_SECRET" -e "$ACCESS_TOKEN")"; done
journalctl --user --since today --no-pager | grep -c -F -e "$API_SECRET" -e "$ACCESS_TOKEN"; echo "journal count above"
grep -r -a -l -F -e "$API_SECRET" -e "$ACCESS_TOKEN" /tmp 2>/dev/null; echo "tmp grep exit=$?"
unset API_SECRET ACCESS_TOKEN
```

Expected: `values loaded (…)` is printed, then `datadir grep exit=1` with no file names before it, each backup line ends in `: 0`, the journal count is `0`, and `tmp grep exit=1`. Any file name or non-zero count fails NFR-04. Fix the redaction (P0-1 `logging.Redactor`), then repeat. If the encrypted-file backend is in use, `secrets.enc` is ciphertext and must not match. Read the access token from Settings → Kite connection → "Show token" only if P0-3 provides that; otherwise use the keyring command above.

- [ ] **Step 7: 24-hour network capture (NFR-04, ARCHITECTURE §9.2)**

On VM A, logged in, with Telegram, the update check and IP echo left disabled (defaults). Start before 09:00 IST on a trading day and stop 24 h later:

```bash
sudo apt install -y tcpdump tshark
PID=$(pgrep -x tradeloom); echo "pid=$PID"
sudo tcpdump -i any -n -w /var/tmp/tl-24h.pcap 'tcp or port 53' &
TCPDUMP=$!
( for i in $(seq 1 1440); do sudo ss -tnpH state established | grep "pid=$PID," | awk '{print $4}' >> /var/tmp/tl-peers.txt; sleep 60; done ) &
# … 24 hours later …
sudo kill $TCPDUMP; wait
sed -E 's/^\[?([^]]*)\]?:[0-9]+$/\1/' /var/tmp/tl-peers.txt | sort -u > /var/tmp/tl-ips.txt
tshark -r /var/tmp/tl-24h.pcap -Y 'tls.handshake.type == 1' -T fields -e ip.dst -e ipv6.dst -e tls.handshake.extensions_server_name \
  | awk 'NF{print ($1!=""?$1:$2), $NF}' | sort -u > /var/tmp/tl-sni.txt
awk 'NR==FNR{ip[$1]=1; next} ($1 in ip){print $2}' /var/tmp/tl-ips.txt /var/tmp/tl-sni.txt | sort -u
wc -l < /var/tmp/tl-ips.txt
```

Expected: the final `sort -u` prints exactly `api.kite.trade` and `ws.kite.trade`, and nothing else. The peer count is ≥ 1. `kite.zerodha.com` appears only for the browser, never for the tradeloom pid. If the process restarted during the 24 h, `pid=` is stale: restart the sampler with the new pid and record it. Any other host fails NFR-04. Find the client with `grep -n '<host>' -r internal/` and route it through `netguard`.

The CI test (ARCHITECTURE §9.2, mock-Kite build behind a failing proxy, owned by P0-3) is the automated companion. Confirm it ran in the Step 1 CI run.

- [ ] **Step 8: NFR-15 and the instrument master**

On VM A:

```bash
sqlite3 -readonly ~/.local/share/tradeloom/app.db "SELECT segment, COUNT(*) FROM instruments GROUP BY segment ORDER BY segment;"
sqlite3 -readonly ~/.local/share/tradeloom/app.db "SELECT COUNT(*) FROM instruments WHERE segment NOT IN ('INDICES','NSE','BSE','NFO-FUT','BFO-FUT') OR instrument_type IN ('CE','PE');"
```

Expected: the first query lists only `BFO-FUT`, `BSE`, `INDICES`, `NFO-FUT`, `NSE`. The second prints `0`. (`sqlite3` is read-only here. Install it with `sudo apt install -y sqlite3` if missing.)

- [ ] **Step 9: Write the exit record**

Create `docs/plan/P0-exit-record.md`:

```markdown
# P0 exit record

Release commit: <sha> · Packages: <deb name>, <AppImage name> (SHA256SUMS attached) · Date: <date> · Operator: <name>
CI run: <url> (go-unit, ta, arch-lint, frontend, licence, linux-package, install-smoke, ui-e2e — all success)

## ROADMAP §P0 demo (Connect plan) — Step 4
| # | Expected | Observed | Evidence |
|---|---|---|---|
| 1–16 | (copy each Expected cell from Task 19 Step 4) | | screenshots/p0-exit/NN.png |

## Second demo (Personal plan) — Step 5
| # | Expected | Observed | Evidence |

## NFR runs
| NFR | Check | Result | Evidence |
|---|---|---|---|
| NFR-01/02 | Step 3 matrix: 5 × exit=0 | | console log |
| NFR-03 | P0-1 migration tests in CI + VM B restore re-ran migrations | | |
| NFR-04 | Step 6 grep: no match; Step 7 capture: api.kite.trade, ws.kite.trade only | | tl-sni.txt, tl-ips.txt |
| NFR-07 | e2e "tick storm" worst latency < 100 ms | | Playwright report |
| NFR-09 | Step 4 #10 reconnect + resubscribe | | |
| NFR-11 | Step 4 #16 + e2e layout-1280 (light, dark) | | |
| NFR-15 | Step 8 counts | | |

## Acceptance criteria (ROADMAP §P0 list)
For each FR group, the test name(s) that pass on the release commit: this plan's Coverage table for CHART/WATCH/SHELL-004;
the P0-1…P0-4 coverage tables for the rest. Anything not green blocks the exit.

## Spikes
S0-1 … S0-8 output documents present and decisions recorded (link each).

Sign-off: P0 exit criteria met — <name>, <date>
```

Fill every cell with the observed values. Any red cell blocks the phase exit. Fix it in the owning sub-plan, then re-run only the affected steps.

- [ ] **Step 10: Commit**

```bash
git add docs/plan/P0-exit-record.md
git commit -m "$(cat <<'EOF'
docs(plan): P0 exit demo and NFR run record

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
)"
```

---
