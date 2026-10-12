> **P0-2 Time & Market Data Implementation Plan — Task 18.** Read [`../MANIFEST.md`](../MANIFEST.md) first: its Global Constraints,
> Review Focus and Consumes/Produces apply to this task. Other tasks live beside this file in `tasks/`.

<!-- task body (verbatim from the plan) -->
### Task 18: KiteDialer over gokiteconnect ticker, proven against the mock server

**Files:**
- Create: `internal/kite/ticker/kitedial.go`
- Test: `internal/kite/ticker/kite_mock_test.go`

**Interfaces:**
- Consumes: Task 8: `mock.NewServer`, `TickerURL`, `Messages`, `Connects`, `PushTicks`, `Packet`, `DropConnections`. Task 17: `Manager`, `Options`, `Events`, `Conn`, `Dialer`.
- Produces (exact Go signatures; later tasks and P0-3/P0-5 copy them):

```go
func KiteDialer(apiKey, accessToken string, root *url.URL) Dialer
```

Wires the real `ticker.Ticker` with `SetReconnectMaxRetries(0)` so the library hands every failure back to the Manager (see "Verified library facts"). The integration test proves the subscribe/mode frames, binary decoding of a full-mode packet, and drop → redial → resubscribe through the real library (it waits for the library's 5 s data timeout, so it takes about 6 s).

- [ ] **Step 1: Write the failing tests**

`internal/kite/ticker/kite_mock_test.go`:

```go
package ticker_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"tradeloom/internal/bus"
	"tradeloom/internal/clock"
	"tradeloom/internal/kite/mock"
	"tradeloom/internal/kite/ticker"
)

func waitFor(t *testing.T, within time.Duration, what string, f func() bool) {
	t.Helper()
	deadline := time.Now().Add(within)
	for !f() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// Drives the real gokiteconnect ticker against the mock server: subscribe/mode frames, binary
// tick decoding, and — with SetReconnectMaxRetries(0) — the Manager's own reconnect.
func TestManagerOverRealTickerAndMockServer(t *testing.T) {
	srv := mock.NewServer()
	defer srv.Close()
	root := srv.TickerURL()
	m := ticker.New(ticker.Options{
		Dial:      ticker.KiteDialer("key", "token", &root),
		Clock:     clock.New(clock.Options{}),
		Bus:       bus.New(nil),
		BaseDelay: 100 * time.Millisecond,
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := m.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer m.Stop()
	waitFor(t, 3*time.Second, "connect", func() bool { return srv.Connects() == 1 })
	if err := m.Subscribe(ticker.ConsumerChart, "c", []uint32{738561}, ticker.ModeFull); err != nil {
		t.Fatal(err)
	}
	waitFor(t, 3*time.Second, "subscribe frames", func() bool { return len(srv.Messages()) == 2 })
	msgs := srv.Messages()
	if msgs[0] != `{"a":"subscribe","v":[738561]}` || msgs[1] != `{"a":"mode","v":["full",[738561]]}` {
		t.Fatalf("frames %v", msgs)
	}
	raw, stop := m.Raw(8)
	defer stop()
	ts := time.Date(2026, 10, 12, 10, 0, 1, 0, clock.IST)
	srv.PushTicks(mock.Packet{Token: 738561, Mode: "full", LastPrice: 2450.05, Volume: 12345, Open: 2440, High: 2455, Low: 2431.5, Close: 2438, Timestamp: ts, LastTradeTime: ts})
	select {
	case tk := <-raw:
		if tk.LTP != 2450.05 || tk.Volume != 12345 || !tk.ExchangeTS.Equal(ts) || tk.Mode != "full" || tk.Close != 2438 {
			t.Fatalf("tick %+v", tk)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("no tick")
	}

	// Drop the socket: the library notices within its 5 s data timeout, returns (max retries 0),
	// and the Manager redials and resubscribes.
	srv.DropConnections()
	waitFor(t, 15*time.Second, "reconnect", func() bool { return srv.Connects() == 2 })
	waitFor(t, 3*time.Second, "resubscribe", func() bool { return len(srv.Messages()) == 4 })
	if got := strings.Join(srv.Messages()[2:], " "); got != `{"a":"subscribe","v":[738561]} {"a":"mode","v":["full",[738561]]}` {
		t.Fatalf("resubscribe frames %s", got)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `GOWORK=off go test -tags webkit2_41,noorders -race -short ./internal/kite/ticker/`
Expected: the build fails — output contains `undefined: ticker.KiteDialer` and `[build failed]`.

- [ ] **Step 3: Write the implementation**

`internal/kite/ticker/kitedial.go`:

```go
package ticker

import (
	"context"
	"net/url"
	"sync"
	"time"

	kt "github.com/zerodha/gokiteconnect/v4/ticker"
)

// KiteDialer returns a Dialer over gokiteconnect's ticker. Each Conn is a fresh ticker.Ticker
// with SetReconnectMaxRetries(0): the library then returns from ServeWithContext after the first
// failure instead of reconnecting itself, so the Manager owns reconnects and resubscribes. This
// avoids two library hazards (v4.4.3 ticker.go): Resubscribe mutates the unsynchronised
// subscribedTokens map on the serve goroutine and prints the token list to stdout; and with
// SetAutoReconnect(false) a failed dial dereferences a nil Conn.
func KiteDialer(apiKey, accessToken string, root *url.URL) Dialer {
	return func(ev Events) Conn {
		t := kt.New(apiKey, accessToken)
		if root != nil {
			t.SetRootURL(*root)
		}
		t.SetAutoReconnect(true)
		t.SetReconnectMaxRetries(0)
		t.SetConnectTimeout(7 * time.Second)
		k := &kiteConn{t: t}
		t.OnConnect(func() {
			k.mu.Lock()
			k.connected = true
			k.mu.Unlock()
			if ev.OnConnect != nil {
				ev.OnConnect()
			}
		})
		if ev.OnTick != nil {
			t.OnTick(ev.OnTick)
		}
		if ev.OnOrderUpdate != nil {
			t.OnOrderUpdate(ev.OnOrderUpdate)
		}
		if ev.OnError != nil {
			t.OnError(ev.OnError)
		}
		return k
	}
}

type kiteConn struct {
	t         *kt.Ticker
	mu        sync.Mutex
	connected bool
}

func (k *kiteConn) Serve(ctx context.Context)             { k.t.ServeWithContext(ctx) }
func (k *kiteConn) Subscribe(tokens []uint32) error       { return k.t.Subscribe(tokens) }
func (k *kiteConn) Unsubscribe(tokens []uint32) error     { return k.t.Unsubscribe(tokens) }
func (k *kiteConn) SetMode(m Mode, tokens []uint32) error { return k.t.SetMode(m.kite(), tokens) }

// Stop cancels the serve loop and closes the socket so a blocked read returns.
func (k *kiteConn) Stop() {
	k.t.Stop()
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.connected && k.t.Conn != nil {
		_ = k.t.Conn.Close()
	}
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `GOWORK=off go test -tags webkit2_41,noorders -race -short ./internal/kite/ticker/`
Expected: `ok  	tradeloom/internal/kite/ticker`; no `DATA RACE`.

- [ ] **Step 5: Commit**

```bash
git add internal/kite/ticker/kitedial.go \
  internal/kite/ticker/kite_mock_test.go
git commit -m "$(cat <<'EOF'
feat(kite/ticker): gokiteconnect dialer with manager-owned reconnects

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
)"
```
