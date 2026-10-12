> **P0-2 Time & Market Data Implementation Plan — Task 21.** Read [`../MANIFEST.md`](../MANIFEST.md) first: its Global Constraints,
> Review Focus and Consumes/Produces apply to this task. Other tasks live beside this file in `tasks/`.

<!-- task body (verbatim from the plan) -->
### Task 21: Spike S0-7: app clock against a live account

**Files:**
- Test: `internal/kite/ticker/s07_test.go`
- Create: `docs/architecture/spikes/S0-7-clock.md` (generated in this task)

**Interfaces:**
- Consumes: Task 2: `clock.New` with a skewed `Options.System`, `Offset`, `SourceTicks`. Task 20: `liveEnv`, `equityTokens`. Task 17/18: `ticker.New`, `KiteDialer`, `ClockRef`, `Observer`.
- Produces: test harnesses and the spike record only.

ARCHITECTURE §12.1 S0-7 / FR-DATA-004 AC1 and AC4 on real data: with the system clock simulated 3 s fast the tick estimate must be −3 s ± 0.5 s within 60 s, and the HTTP `Date` fallback error is recorded (1 s header resolution → documented ±0.5 s). Output: `docs/architecture/spikes/S0-7-clock.md` confirming the estimator parameters.

- [ ] **Step 1: Write the S0-7 harness (build tag `kitelive`)**

`internal/kite/ticker/s07_test.go`:

```go
//go:build kitelive

package ticker_test

import (
	"context"
	"testing"
	"time"

	"tradeloom/internal/bus"
	"tradeloom/internal/clock"
	"tradeloom/internal/kite/ticker"
)

// TestS07 is spike S0-7 (ARCHITECTURE §12.1, FR-DATA-004 AC1/AC4). It simulates a system clock
// 3 s fast by skewing the System source, so it needs no root access. Run once during market
// hours (tick source) and once after 16:00 (HTTP Date source):
//
//	KITE_API_KEY=… KITE_ACCESS_TOKEN=… go test -tags kitelive,webkit2_41,noorders -run TestS07 -v ./internal/kite/ticker/
func TestS07(t *testing.T) {
	skew := 3 * time.Second
	clk := clock.New(clock.Options{System: func() time.Time { return time.Now().Add(skew) }})
	c, key, tok := liveEnv(t, clk)
	_, nifty := equityTokens(t, c, 0)

	m := ticker.New(ticker.Options{Dial: ticker.KiteDialer(key, tok, nil), Clock: clk, Bus: bus.New(nil), ClockRef: nifty, Observer: clk})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := m.Start(ctx); err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	var within time.Duration = -1
	for time.Since(start) < 90*time.Second {
		time.Sleep(time.Second)
		o := clk.Offset()
		t.Logf("S0-7 t+%2.0fs offset %+6dms source %-9s samples %d", time.Since(start).Seconds(), o.Millis, o.Source, o.Samples)
		if within < 0 && o.Source == clock.SourceTicks && abs(o.Value+skew) <= 500*time.Millisecond {
			within = time.Since(start)
		}
	}
	m.Stop()
	if within < 0 {
		t.Logf("S0-7 tick estimate never within −3 s ± 0.5 s (market closed? run again during market hours)")
	} else {
		t.Logf("S0-7 tick estimate within −3 s ± 0.5 s after %v (gate ≤ 60 s)", within.Round(time.Second))
	}

	// HTTP Date fallback: wait out the tick window, then five read calls.
	time.Sleep(61 * time.Second)
	for i := 0; i < 5; i++ {
		if _, err := c.Profile(ctx); err != nil {
			t.Fatal(err)
		}
		time.Sleep(700 * time.Millisecond) // spread samples across the 1 s Date resolution
	}
	o := clk.Offset()
	t.Logf("S0-7 HTTP Date estimate: %+dms (source %s, samples %d); error vs true −3000ms: %+dms",
		o.Millis, o.Source, o.Samples, o.Millis+3000)
}

func abs(d time.Duration) time.Duration {
	if d < 0 {
		return -d
	}
	return d
}
```

- [ ] **Step 2: Check it compiles and skips without credentials**

Run: `GOWORK=off go test -tags kitelive,webkit2_41,noorders -run TestS07 -v ./internal/kite/ticker/`
Expected: `--- SKIP: TestS07`.

- [ ] **Step 3: Confirm the system clock is NTP-synchronised**

The harness simulates the 3 s skew itself, so the true offset must be about zero.

Run: `timedatectl show -p NTPSynchronized --value`
Expected: `yes`

- [ ] **Step 4: Run S0-7 during market hours, then again after 16:00 IST**

With `KITE_API_KEY` and `KITE_ACCESS_TOKEN` exported (as in Task 20):

Run:
```bash
GOWORK=off go test -tags kitelive,webkit2_41,noorders \
  -run TestS07 -timeout 10m -v ./internal/kite/ticker/ 2>&1 | tee -a /tmp/s07.log
```
Expected (market hours): a line `S0-7 tick estimate within −3 s ± 0.5 s after <N>s (gate ≤ 60 s)` with N ≤ 60, and a line `S0-7 HTTP Date estimate: …`. After 16:00 only the HTTP line is meaningful.

- [ ] **Step 5: Write the spike record from the runs**

Run:
```bash
mkdir -p docs/architecture/spikes
{
  printf '# S0-7 App clock\n\n'
  printf 'ARCHITECTURE §12.1 S0-7 / FR-DATA-004 AC1, AC4: offset estimator accuracy with a system clock 3 s fast (simulated by\n'
  printf '`clock.Options.System`), and the HTTP Date fallback (1 s header resolution). Harness: `internal/kite/ticker/s07_test.go`.\n\n'
  printf '## Measurements\n\n'
  grep -E 'S0-7 (tick estimate|HTTP Date)' /tmp/s07.log | sed -E 's/^[[:space:]]*s07_test\.go:[0-9]+: /- /'
  printf '\n## Parameters\n\nTick window 60 s (median of `exchange_ts + 0.5 s - arrival` for the NIFTY 50 index in full mode); HTTP Date: median of the\n'
  printf 'last 5 samples of `date + 0.5 s - request midpoint`; warn > 1 s, skewed > 5 s, cleared < 2 s; timers re-armed on moves > 100 ms.\n\n## Decision\n\n'
  if grep -q 'S0-7 tick estimate within' /tmp/s07.log; then
    printf 'The estimator parameters above are confirmed for FR-DATA-004. HTTP Date fallback accuracy is documented as +/-0.5 s.\n'
  else
    printf 'The tick estimate did not converge within 60 s: the window/median parameters are revisited before P4 (live evaluation).\n'
  fi
} > docs/architecture/spikes/S0-7-clock.md
cat docs/architecture/spikes/S0-7-clock.md
```
Expected: the file contains the measurement lines, the parameters and one decision sentence.

- [ ] **Step 6: Commit**

```bash
git add internal/kite/ticker/s07_test.go \
  docs/architecture/spikes/S0-7-clock.md
git commit -m "$(cat <<'EOF'
test(kite/ticker): S0-7 live clock spike harness and record

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
)"
```
