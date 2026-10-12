> **P0-2 Time & Market Data Implementation Plan — Task 20.** Read [`../MANIFEST.md`](../MANIFEST.md) first: its Global Constraints,
> Review Focus and Consumes/Produces apply to this task. Other tasks live beside this file in `tasks/`.

<!-- task body (verbatim from the plan) -->
### Task 20: Spike S0-6: ticker manager against a live account

**Files:**
- Test: `internal/kite/ticker/live_helpers_test.go`
- Test: `internal/kite/ticker/s06_test.go`
- Create: `docs/architecture/spikes/S0-6-ticker.md` (generated in this task)

**Interfaces:**
- Consumes: Tasks 2, 6, 7, 9, 15, 17, 18, 19: `clock.New`, `ratelimit.New`, `kite.New`, `(*kite.Client).Instruments/Historical`, `instruments.Filter`, `ticker.New`, `KiteDialer`, `Manager.Subscribe/Release/Raw/Usage/Stop`, `builder.New/Track/Run`, `builder.TopicClosed`.
- Produces: test harnesses and the spike record only.

ARCHITECTURE §12.1 S0-6: 3 × 3000 handling and the 9001st refusal, tick rate, mode-switch cost, exchange → receive latency, reconnect + resubscribe time, and builder 1-minute candles compared with Kite historical candles for the same minutes. Output: `docs/architecture/spikes/S0-6-ticker.md` and the coalescing decision.

- [ ] **Step 1: Write the live helpers (build tag `kitelive`)**

`internal/kite/ticker/live_helpers_test.go`:

```go
//go:build kitelive

package ticker_test

import (
	"context"
	"os"
	"sort"
	"testing"
	"time"

	"tradeloom/internal/calendar"
	"tradeloom/internal/clock"
	"tradeloom/internal/instruments"
	"tradeloom/internal/kite"
	"tradeloom/internal/kite/ratelimit"
)

// liveEnv builds a real Kite client from KITE_API_KEY / KITE_ACCESS_TOKEN (a developer's own
// session, read-only calls only). It never runs in CI: the kitelive tag is not set there.
func liveEnv(t *testing.T, clk *clock.Real) (*kite.Client, string, string) {
	t.Helper()
	key, tok := os.Getenv("KITE_API_KEY"), os.Getenv("KITE_ACCESS_TOKEN")
	if key == "" || tok == "" {
		t.Skip("KITE_API_KEY and KITE_ACCESS_TOKEN are required")
	}
	lim := ratelimit.New(clk, ratelimit.Options{})
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	lim.Run(ctx)
	c := kite.New(kite.Options{APIKey: key, Limiter: lim, Clock: clk, DateSink: clk.ObserveHTTPDate})
	c.SetAccessToken(tok)
	return c, key, tok
}

// equityTokens returns up to n NSE then BSE cash-equity tokens from today's dump, and the NIFTY 50 token.
func equityTokens(t *testing.T, c *kite.Client, n int) ([]uint32, uint32) {
	t.Helper()
	dump, err := c.Instruments(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	all := instruments.Filter(dump, calendar.DateOf(time.Now()))
	var out []uint32
	var nifty uint32
	for _, in := range all {
		if in.ID() == "NSE:NIFTY 50" {
			nifty = in.Token
		}
		if in.Tag == instruments.TagNSE || in.Tag == instruments.TagBSE {
			out = append(out, in.Token)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	if len(out) > n {
		out = out[:n]
	}
	return out, nifty
}

func pct(ds []time.Duration, p float64) time.Duration {
	if len(ds) == 0 {
		return 0
	}
	s := append([]time.Duration(nil), ds...)
	sort.Slice(s, func(i, j int) bool { return s[i] < s[j] })
	return s[int(float64(len(s)-1)*p)]
}
```

- [ ] **Step 2: Write the S0-6 harness**

`internal/kite/ticker/s06_test.go`:

```go
//go:build kitelive

package ticker_test

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"

	"tradeloom/internal/bus"
	"tradeloom/internal/calendar"
	"tradeloom/internal/candles"
	"tradeloom/internal/candles/builder"
	"tradeloom/internal/clock"
	"tradeloom/internal/kite/ticker"
)

// TestS06 is spike S0-6 (ARCHITECTURE §12.1). Run during market hours on the reference machine:
//
//	KITE_API_KEY=… KITE_ACCESS_TOKEN=… S06_MINUTES=30 go test -tags kitelive,webkit2_41,noorders \
//	  -run TestS06 -timeout 8h -v ./internal/kite/ticker/
func TestS06(t *testing.T) {
	clk := clock.New(clock.Options{})
	c, key, tok := liveEnv(t, clk)
	minutes := 30
	if v := os.Getenv("S06_MINUTES"); v != "" {
		d, err := time.ParseDuration(v + "m")
		if err != nil {
			t.Fatal(err)
		}
		minutes = int(d.Minutes())
	}
	tokens, nifty := equityTokens(t, c, 9000)
	t.Logf("S0-6 tokens available: %d", len(tokens))

	var connMu sync.Mutex
	var conns []ticker.Conn
	base := ticker.KiteDialer(key, tok, nil)
	dial := func(ev ticker.Events) ticker.Conn {
		cn := base(ev)
		connMu.Lock()
		conns = append(conns, cn)
		connMu.Unlock()
		return cn
	}
	b := bus.New(nil)
	feed := &feedWatch{s: bus.Subscribe(b, ticker.TopicFeed, bus.SubOpts[ticker.FeedState]{Buffer: 256})}
	m := ticker.New(ticker.Options{Dial: dial, Clock: clk, Bus: b, ClockRef: nifty, Observer: clk})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	raw, stopRaw := m.Raw(1 << 16)
	defer stopRaw()

	// 1. Fill 3 × 3000 in ltp mode; time until all three connections are live.
	t0 := time.Now()
	if err := m.Start(ctx); err != nil {
		t.Fatal(err)
	}
	if err := m.Subscribe(ticker.ConsumerRecorder, "s06", tokens, ticker.ModeLTP); err != nil {
		t.Fatalf("budget: %v", err)
	}
	for m.Usage().Connections < 3 || feed.state() != "live" {
		if time.Since(t0) > time.Minute {
			t.Fatalf("not live after 1 min: %+v", m.Usage())
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Logf("S0-6 3×3000 live after %v; usage %+v", time.Since(t0).Round(time.Millisecond), m.Usage())
	if err := m.Subscribe(ticker.ConsumerRecorder, "over", []uint32{1}, ticker.ModeLTP); err == nil {
		t.Error("9001st token accepted")
	}

	// 2. Tick rate and coverage for 60 s.
	seen := map[uint32]bool{}
	count := 0
	for end := time.Now().Add(time.Minute); time.Now().Before(end); {
		select {
		case tk := <-raw:
			seen[tk.Token] = true
			count++
		case <-time.After(time.Second):
		}
	}
	t.Logf("S0-6 ticks in 60 s: %d (%.0f/s), distinct tokens %d, raw drops %d", count, float64(count)/60, len(seen), m.Usage().RawDrops)

	// 3. Mode switch cost: 3000 tokens ltp → full; time until 90 % have sent a full tick.
	first := tokens[:3000]
	want := map[uint32]bool{}
	for _, x := range first {
		want[x] = true
	}
	t1 := time.Now()
	if err := m.Subscribe(ticker.ConsumerChart, "s06-full", first, ticker.ModeFull); err != nil {
		t.Fatal(err)
	}
	full := map[uint32]bool{}
	var lat []time.Duration
	for len(full) < 2700 && time.Since(t1) < 2*time.Minute {
		select {
		case tk := <-raw:
			if want[tk.Token] && tk.Mode == "full" {
				full[tk.Token] = true
				if !tk.ExchangeTS.IsZero() {
					lat = append(lat, tk.Received.Sub(tk.ExchangeTS))
				}
			}
		case <-time.After(time.Second):
		}
	}
	t.Logf("S0-6 mode switch: %d/3000 full after %v", len(full), time.Since(t1).Round(time.Millisecond))
	t.Logf("S0-6 exchange→receive latency (app clock, offset %+v): p50 %v p95 %v p99 %v",
		clk.Offset(), pct(lat, .5), pct(lat, .95), pct(lat, .99))

	// 4. Reconnect: drop connection 2; time until live again with its tokens resubscribed.
	connMu.Lock()
	victim := conns[1]
	connMu.Unlock()
	t2 := time.Now()
	victim.Stop()
	for feed.state() != "reconnecting" && time.Since(t2) < 30*time.Second {
		time.Sleep(10 * time.Millisecond)
	}
	for feed.state() != "live" && time.Since(t2) < 2*time.Minute {
		time.Sleep(10 * time.Millisecond)
	}
	t.Logf("S0-6 reconnect + resubscribe after drop: %v (state %s)", time.Since(t2).Round(time.Millisecond), feed.state())

	// 5. Builder correctness: 1-minute candles from ticks vs Kite historical for S06_MINUTES.
	m.Release(ticker.ConsumerRecorder, "s06")
	m.Release(ticker.ConsumerChart, "s06-full")
	sample := tokens[:5]
	_ = m.Subscribe(ticker.ConsumerChart, "s06-builder", sample, ticker.ModeFull)
	cal, _ := calendar.NewDefault()
	bb := bus.New(nil)
	closed := bus.Subscribe(bb, builder.TopicClosed, bus.SubOpts[builder.Event]{Buffer: 1 << 14})
	bld := builder.New(builder.Options{Clock: clk, Calendar: cal, Bus: bb})
	for _, x := range sample {
		bld.Track(x, candles.Minute)
	}
	go bld.Run(ctx, raw)
	time.Sleep(time.Duration(minutes) * time.Minute)
	built := map[uint32]map[int64]candles.Candle{}
	for len(closed.C()) > 0 {
		e := <-closed.C()
		if e.Partial {
			continue
		}
		if built[e.Token] == nil {
			built[e.Token] = map[int64]candles.Candle{}
		}
		built[e.Token][e.Candle.Start.Unix()] = e.Candle
	}
	time.Sleep(2 * time.Minute) // let Kite finalise the last minute
	var compared, ohlcDiff, volDiff int
	for _, x := range sample {
		from := time.Now().Add(-time.Duration(minutes+3) * time.Minute)
		hist, err := c.Historical(ctx, int(x), "minute", from, time.Now(), false, false)
		if err != nil {
			t.Fatal(err)
		}
		for _, h := range hist {
			bc, ok := built[x][h.Date.Unix()]
			if !ok {
				continue
			}
			compared++
			if bc.Open != h.Open || bc.High != h.High || bc.Low != h.Low || bc.Close != h.Close {
				ohlcDiff++
			}
			if bc.Volume != int64(h.Volume) {
				volDiff++
			}
		}
	}
	t.Logf("S0-6 builder vs historical: %d candles compared, %d OHLC mismatches, %d volume mismatches, late ticks %d",
		compared, ohlcDiff, volDiff, bld.LateTicks())
	m.Stop()
}

// feedWatch tracks the latest feed.state.
type feedWatch struct {
	s    *bus.Sub[ticker.FeedState]
	last string
}

func (f *feedWatch) state() string {
	for len(f.s.C()) > 0 {
		f.last = (<-f.s.C()).State
	}
	return f.last
}
```

- [ ] **Step 3: Check it compiles and skips without credentials**

Run: `GOWORK=off go test -tags kitelive,webkit2_41,noorders -run TestS06 -v ./internal/kite/ticker/`
Expected: `--- SKIP: TestS06` with "KITE_API_KEY and KITE_ACCESS_TOKEN are required".

- [ ] **Step 4: Run S0-6 on the reference machine during market hours**

Log in once with the developer's own Kite Connect app (P0-3's login is not needed: take the access token from the Kite developer console session or a one-off `GenerateSession`), start before 14:30 IST on a trading day so the builder comparison finishes before 15:30. Export `KITE_API_KEY` and `KITE_ACCESS_TOKEN` in the shell first.

Run:
```bash
S06_MINUTES=30 GOWORK=off go test -tags kitelive,webkit2_41,noorders \
  -run TestS06 -timeout 2h -v ./internal/kite/ticker/ 2>&1 | tee /tmp/s06.log
```
Expected: lines starting `S0-6 tokens available`, `S0-6 3×3000 live`, `S0-6 ticks in 60 s`, `S0-6 mode switch`, `S0-6 exchange→receive latency`, `S0-6 reconnect + resubscribe`, `S0-6 builder vs historical`, and no "9001st token accepted" error.

- [ ] **Step 5: Write the spike record from the run**

Run:
```bash
mkdir -p docs/architecture/spikes
{
  printf '# S0-6 Ticker manager\n\n'
  printf 'ARCHITECTURE §12.1 S0-6: 3 x 3000 handling, reconnect/resubscribe, mode-switch cost, tick latency, and candle-builder\n'
  printf 'correctness against Kite historical candles. Harness: `internal/kite/ticker/s06_test.go` (tag kitelive).\n\n'
  printf '## Run\n\n- Date: %s\n- Machine: %s\n\n## Measurements\n\n' "$(date '+%d-%m-%Y %H:%M %Z')" "$(uname -srm)"
  grep -E 'S0-6 ' /tmp/s06.log | sed -E 's/^[[:space:]]*s06_test\.go:[0-9]+: /- /'
  printf '\n## Decision\n\n'
  if grep -qE 'raw drops 0' /tmp/s06.log && grep -qE ' 0 OHLC mismatches' /tmp/s06.log; then
    printf 'Coalescing interval stays 100 ms (`ticker.Options.Coalesce` default); raw stream buffer 65536 had no drops and builder\n'
    printf 'candles matched Kite OHLC for every compared minute. Volume differences, if any above, are recorded for FR-CHART-003 review.\n'
  else
    printf 'Coalescing interval stays 100 ms for the UI, but the raw stream dropped ticks or builder OHLC differed from Kite:\n'
    printf 'the measurements above are attached to a follow-up task in P0-5 before the exit demo.\n'
  fi
} > docs/architecture/spikes/S0-6-ticker.md
cat docs/architecture/spikes/S0-6-ticker.md
```
Expected: the file contains the seven measurement lines and one decision paragraph.

- [ ] **Step 6: Commit**

```bash
git add internal/kite/ticker/live_helpers_test.go \
  internal/kite/ticker/s06_test.go \
  docs/architecture/spikes/S0-6-ticker.md
git commit -m "$(cat <<'EOF'
test(kite/ticker): S0-6 live ticker spike harness and record

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
)"
```
