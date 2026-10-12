> **P0-2 Time & Market Data Implementation Plan — Task 22.** Read [`../MANIFEST.md`](../MANIFEST.md) first: its Global Constraints,
> Review Focus and Consumes/Produces apply to this task. Other tasks live beside this file in `tasks/`.

<!-- task body (verbatim from the plan) -->
### Task 22: Phase verification: full suite, rules, perf, coverage

**Files:**
- Modify: none (verification only; fix forward in the owning task's files if a check fails)

**Interfaces:**
- Consumes: Everything above; P0-1 `internal/archtest`.
- Produces: nothing new — the "Produces" blocks of Tasks 1–21 are the contract P0-3 and P0-5 copy.

Proves the whole sub-plan together before P0-3 starts: race-enabled unit suite, CI perf test, architecture rules, the G-0 check and the coverage table below.

- [ ] **Step 1: Race-enabled unit suite**

Run: `GOWORK=off go test -tags webkit2_41,noorders -race -short ./...`
Expected: every package line starts with `ok` (or `?` for packages without tests); no `FAIL`, no `DATA RACE`.

- [ ] **Step 2: CI-scale store performance (no race detector)**

Run: `GOWORK=off go test -tags webkit2_41,noorders -run TestPerf -v ./internal/candles/sqlite/`
Expected: `--- PASS: TestPerf_StoreQueriesAtCIScale` with logged read and missing-range times below 500 ms and 50 ms.

- [ ] **Step 3: Architecture rules (P0-1 archtest) and vet**

Run: `GOWORK=off go vet -tags webkit2_41,noorders ./... && GOWORK=off go test -tags webkit2_41,noorders ./internal/archtest/`
Expected: no vet output; `ok  	tradeloom/internal/archtest`.

- [ ] **Step 4: Rule greps the linters also enforce (fast local signal)**

Run:
```bash
grep -rnE 'time\.(Now|Since|Until|Sleep|After|AfterFunc|Tick|NewTimer|NewTicker)\(|^[[:space:]]*go ' internal assets | grep '\.go:' | grep -v '_test.go' \
  | grep -vE '^internal/(clock|bus|logging)/' ; \
grep -rlE '"github.com/zerodha/gokiteconnect' internal | grep -v '^internal/kite/' ; \
grep -rlE '"(database/sql|modernc.org/sqlite)"' internal | grep -vE '^internal/(store|candles/sqlite)/' ; \
grep -rnE '"(09:15|15:30|15:20|08:30|16:00)"' internal | grep -v '_test.go' | grep -v '^internal/calendar/' ; \
echo rules-checked
```
Expected: only the line `rules-checked`.

- [ ] **Step 5: Coverage of the new packages**

Run: `GOWORK=off go test -tags webkit2_41,noorders -short -cover ./internal/clock/ ./internal/calendar/ ./internal/kite/... ./internal/candles/... ./internal/instruments/`
Expected: every listed package reports `coverage:` ≥ 65 % (verification run: clock 87 %, calendar 89 %, candles 88 %, candles/sqlite 75 %, builder 89 %, instruments 87 %, kite 69 %, kite/ticker 95 %).

- [ ] **Step 6: Walk the coverage table**

For each row of "Acceptance-criteria coverage" below, run the named test with `-run '<TestName>$' -v` in its package and confirm `--- PASS`. Rows marked "later sub-plan" are not run here.


---
